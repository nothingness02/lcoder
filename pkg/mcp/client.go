package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Client connects to an MCP server over stdio.
type Client struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu       sync.Mutex
	nextID   int32
	pending  map[int]chan Response
	closed   bool
	stopErr  error

	serverInfo  Info
	serverCaps  ServerCapabilities
	tools       []Tool
}

// NewClient starts an MCP server command and returns a connected client.
func NewClient(name string, command []string, env map[string]string) (*Client, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("mcp server command is empty")
	}

	cmd := exec.Command(command[0], command[1:]...)
	for k, v := range env {
		cmd.Env = append(os.Environ(), k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp start: %w", err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		pending: make(map[int]chan Response),
	}

	go c.readLoop()
	go c.stderrLoop()

	if err := c.initialize(context.Background()); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}

	return c, nil
}

// Name returns the server display name.
func (c *Client) Name() string { return c.name }

// ServerInfo returns the server info from initialization.
func (c *Client) ServerInfo() Info { return c.serverInfo }

// Tools returns the cached list of tools from the MCP server.
func (c *Client) Tools() []Tool { return c.tools }

func (c *Client) initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: Info{
			Name:    "lcoder",
			Version: "0.1.0",
		},
	}
	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return err
	}
	c.serverInfo = result.ServerInfo
	c.serverCaps = result.Capabilities

	// Send initialized notification.
	_ = c.notify("notifications/initialized", struct{}{})

	// List tools if supported.
	if result.Capabilities.Tools != nil {
		var toolsResult ListToolsResult
		if err := c.call(ctx, "tools/list", struct{}{}, &toolsResult); err != nil {
			return err
		}
		c.tools = toolsResult.Tools
	}

	return nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := int(atomic.AddInt32(&c.nextID, 1))
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req := Request{JSONRPC: "2.0", ID: id, Method: method, Params: paramsBytes}

	respCh := make(chan Response, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("mcp client closed")
	}
	c.pending[id] = respCh
	c.mu.Unlock()

	if err := c.send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

func (c *Client) notify(method string, params any) error {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req := Request{JSONRPC: "2.0", Method: method, Params: paramsBytes}
	return c.send(req)
}

func (c *Client) send(req Request) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("mcp client closed")
	}
	c.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		c.mu.Lock()
		c.stopErr = err
		c.mu.Unlock()
	}
}

func (c *Client) stderrLoop() {
	// Drain stderr to avoid blocking the child process.
	_, _ = io.Copy(io.Discard, c.stderr)
}

// CallTool invokes an MCP tool by name.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{Name: name, Arguments: args}
	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close shuts down the MCP client.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[int]chan Response)
	c.mu.Unlock()

	_ = c.stdin.Close()

	done := make(chan struct{})
	go func() {
		_ = c.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = c.cmd.Process.Kill()
		<-done
	}
	return nil
}

// Healthy reports whether the client process is still running.
func (c *Client) Healthy() bool {
	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	return c.cmd.Process.Signal(os.Signal(nil)) == nil
}
