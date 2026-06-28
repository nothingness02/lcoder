package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lcoder/lcoder/pkg/contextmgr"
	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/observability"
	"github.com/lcoder/lcoder/pkg/permissions"
	"github.com/lcoder/lcoder/pkg/tools"
)

// Config controls agent behavior.
type Config struct {
	SystemPrompt      string
	Model             models.ModelRef
	MaxTurns          int
	ToolExecutionMode models.ExecutionMode
	ContextManager    *contextmgr.Manager
	TransformContext  TransformContext
	BeforeToolCall    BeforeToolCallHook
	AfterToolCall     AfterToolCallHook
	ShouldStop        ShouldStopFunc
	Mode              string
	ModeManager       *ModeManager
}

// Agent runs the LLM tool loop.
type Agent struct {
	cfg        Config
	mgr        *contextmgr.Manager
	llm        *llm.Client
	registry   *tools.Registry
	permissions *permissions.Engine
	bus        *events.Bus
	obsCollector *observability.Collector

	mu            sync.Mutex
	state         State
	steeringQueue []models.AgentMessage
	followUpQueue []models.AgentMessage

	// Internal loop control.
	abortCh     chan struct{}
	abortOnce   sync.Once
	streamAbort context.CancelFunc
}

// State describes the agent runtime state.
type State int

const (
	StateIdle State = iota
	StateStreaming
	StateExecutingTools
)

// TransformContext transforms messages before sending to the LLM.
// It can be used for compaction, pruning, or injecting context.
type TransformContext func(ctx context.Context, messages []models.AgentMessage) ([]models.AgentMessage, error)

// BeforeToolCallHook runs after argument validation and may block execution.
type BeforeToolCallHook func(ctx context.Context, info ToolCallInfo) (*BeforeToolCallResult, error)

// ToolCallInfo is provided to hooks.
type ToolCallInfo struct {
	AssistantMessage models.AgentMessage
	ToolCall         models.ToolCallContent
	Args             map[string]any
	Context          []models.AgentMessage
}

// BeforeToolCallResult indicates whether a tool call should be blocked.
type BeforeToolCallResult struct {
	Block  bool
	Reason string
}

// AfterToolCallHook runs after a tool finishes and may modify its result.
type AfterToolCallHook func(ctx context.Context, info ToolCallResultInfo) (*AfterToolCallResult, error)

// ToolCallResultInfo is provided to the after hook.
type ToolCallResultInfo struct {
	AssistantMessage models.AgentMessage
	ToolCall         models.ToolCallContent
	Args             map[string]any
	Result           models.ToolResult
	IsError          bool
	Context          []models.AgentMessage
}

// AfterToolCallResult allows hooks to override result fields.
type AfterToolCallResult struct {
	Content   []models.ContentPart
	Details   map[string]any
	IsError   *bool
	Terminate bool
}

// ShouldStopFunc decides whether the loop should stop after a turn.
type ShouldStopFunc func(ctx context.Context, turn TurnSummary) (bool, error)

// TurnSummary provides context for a stop decision.
type TurnSummary struct {
	Message     models.AgentMessage
	ToolResults []models.AgentMessage
	Context     []models.AgentMessage
}

// New creates an agent.
func New(cfg Config, llmClient *llm.Client, registry *tools.Registry, perms *permissions.Engine, bus *events.Bus) *Agent {
	mgr := cfg.ContextManager
	if mgr == nil {
		mgr = contextmgr.NewManager(contextmgr.TokenBudget{
			MaxTotal:      128000,
			TargetTotal:   120000,
			ReserveOutput: 8192,
		})
		mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockSystem, "system", contextmgr.StabilityStatic, 100,
			models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: cfg.SystemPrompt})))
	}
	return &Agent{
		cfg:         cfg,
		mgr:         mgr,
		llm:         llmClient,
		registry:    registry,
		permissions: perms,
		bus:         bus,
		state:       StateIdle,
		abortCh:     make(chan struct{}),
	}
}

// NewWithObservability creates an agent with an observability collector.
func NewWithObservability(cfg Config, llmClient *llm.Client, registry *tools.Registry, perms *permissions.Engine, bus *events.Bus, obs *observability.Collector) *Agent {
	ag := New(cfg, llmClient, registry, perms, bus)
	ag.obsCollector = obs
	return ag
}

// Subscribe registers an event handler.
func (a *Agent) Subscribe(handler events.Handler) func() {
	return a.bus.Subscribe(handler)
}

// State returns the current agent state.
func (a *Agent) State() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// setState updates the agent state.
func (a *Agent) setState(s State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = s
}

// Steer injects a user message during the next safe boundary.
func (a *Agent) Steer(msg models.AgentMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steeringQueue = append(a.steeringQueue, msg)
}

// FollowUp queues a message after the agent would otherwise stop.
func (a *Agent) FollowUp(msg models.AgentMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followUpQueue = append(a.followUpQueue, msg)
}

// Abort signals the current run to stop gracefully. Safe to call multiple times.
func (a *Agent) Abort() {
	a.mu.Lock()
	cancel := a.streamAbort
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.abortOnce.Do(func() {
		close(a.abortCh)
	})
}

// SwitchModel changes the model used for subsequent turns and re-sizes the
// context budget in place. Conversation history is preserved. Intended to be
// called from the TUI while the agent is idle (the provider overlay is modal).
func (a *Agent) SwitchModel(ref models.ModelRef, budget contextmgr.TokenBudget) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Model = ref
	if a.mgr != nil {
		a.mgr.SetBudget(budget)
	}
}

// SetMessages rebuilds the conversation from a flat message list.
func (a *Agent) SetMessages(msgs []models.AgentMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mgr.SetMessages(msgs)
}

// AllMessages returns the full conversation from the context manager.
func (a *Agent) AllMessages() []models.AgentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mgr.AllMessages()
}

// Prompt starts a new agent run with a user message.
func (a *Agent) Prompt(ctx context.Context, msg models.AgentMessage) error {
	return a.run(ctx, []models.AgentMessage{msg})
}

// Continue resumes from the current context without adding a new message.
func (a *Agent) Continue(ctx context.Context) error {
	return a.run(ctx, nil)
}

// Mode returns the active mode name.
func (a *Agent) Mode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.Mode
}

// WithMode returns a copy of the agent with a different mode set.
// It snapshots the current context manager so that mode-specific system prompts
// are applied consistently and not repeatedly appended.
func (a *Agent) WithMode(mode string) *Agent {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.cfg
	cfg.Mode = mode
	return &Agent{
		cfg:           cfg,
		mgr:           a.mgr.Clone(),
		llm:           a.llm,
		registry:      a.registry,
		permissions:   a.permissions,
		bus:           a.bus,
		obsCollector:  a.obsCollector,
		state:         a.state,
		steeringQueue: append([]models.AgentMessage(nil), a.steeringQueue...),
		followUpQueue: append([]models.AgentMessage(nil), a.followUpQueue...),
		abortCh:       a.abortCh,
	}
}

func (a *Agent) run(ctx context.Context, initialPrompts []models.AgentMessage) error {
	a.setState(StateStreaming)
	a.abortCh = make(chan struct{})
	a.abortOnce = sync.Once{}

	turn := 0
	for _, msg := range initialPrompts {
		a.appendMessage(msg)
	}

	_ = a.bus.Emit(ctx, events.AgentStartEvent{Base: events.Base{Type: events.AgentStart, Turn: turn}})

	for {
		pending := a.drainSteeringQueue()
		if len(pending) > 0 {
			for _, msg := range pending {
				a.appendMessage(msg)
			}
		}

		_ = a.bus.Emit(ctx, events.TurnStartEvent{Base: events.Base{Type: events.TurnStart, Turn: turn}})

		assistantMsg, err := a.streamAssistant(ctx, turn)
		if err != nil {
			_ = a.bus.Emit(ctx, events.ErrorEvent{Base: events.Base{Type: events.Error, Turn: turn}, Message: err.Error()})
			break
		}

		toolCalls := assistantMsg.ToolCalls()
		var toolResults []models.AgentMessage
		terminate := false
		if len(toolCalls) > 0 {
			a.setState(StateExecutingTools)
			toolResults, terminate = a.executeToolCalls(ctx, turn, assistantMsg, toolCalls)
			a.setState(StateStreaming)
		}

		_ = a.bus.Emit(ctx, events.TurnEndEvent{
			Base:        events.Base{Type: events.TurnEnd, Turn: turn},
			Message:     assistantMsg,
			ToolResults: toolResults,
		})

		turn++

		if a.maxTurnsReached(turn) {
			break
		}

		if terminate {
			break
		}

		if a.shouldStop(ctx, assistantMsg, toolResults) {
			followUps := a.drainFollowUpQueue()
			if len(followUps) == 0 {
				break
			}
			for _, msg := range followUps {
				a.appendMessage(msg)
			}
		}
	}

	_ = a.bus.Emit(ctx, events.AgentEndEvent{
		Base:     events.Base{Type: events.AgentEnd, Turn: turn},
		Messages: a.allMessages(),
	})
	a.setState(StateIdle)
	return nil
}

func (a *Agent) streamAssistant(ctx context.Context, turn int) (models.AgentMessage, error) {
	streamCtx, streamCancel := context.WithCancel(ctx)
	a.mu.Lock()
	a.streamAbort = streamCancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.streamAbort = nil
		a.mu.Unlock()
		streamCancel()
	}()

	systemPrompt, tools, modelRef, execMode := a.applyMode()

	var req models.TurnRequest
	var err error
	if a.cfg.TransformContext != nil {
		messages := a.allMessages()
		transformed, terr := a.cfg.TransformContext(ctx, messages)
		if terr != nil {
			return models.AgentMessage{}, fmt.Errorf("transform context: %w", terr)
		}
		req = models.TurnRequest{
			Model:        modelRef,
			SystemPrompt: systemPrompt,
			Messages:     transformed,
			Tools:        tools,
			Generation: models.GenerationConfig{
				Temperature: 0.2,
				MaxTokens:   4096,
			},
			Cache: "auto",
		}
	} else {
		req, err = a.mgr.BuildTurnRequest(modelRef, tools)
		if err != nil {
			return models.AgentMessage{}, fmt.Errorf("build turn request: %w", err)
		}
	}

	_ = execMode

	turnStartTime := time.Now()
	stream, err := a.llm.StreamTurnRetry(ctx, req, llm.DefaultRetryConfig())
	if err != nil {
		return models.AgentMessage{}, err
	}
	defer stream.Close()

	var partial models.AgentMessage
	started := false
	ttftRecorded := false

	for {
		select {
		case <-streamCtx.Done():
			return partial, streamCtx.Err()
		default:
		}

		ev, ok, err := stream.Next(streamCtx)
		if err != nil {
			return partial, err
		}
		if !ok {
			break
		}

		switch ev.Type() {
		case "start":
			partial = models.NewAgentMessage(models.RoleAssistant)
			if !started {
				started = true
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
		case "text_delta", "thinking_delta":
			if !started {
				started = true
				partial = models.NewAgentMessage(models.RoleAssistant)
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
			if !ttftRecorded && a.obsCollector != nil {
				ttftRecorded = true
				_ = a.obsCollector.RecordTTFT(turn, time.Since(turnStartTime).Milliseconds())
			}
			delta, _ := ev.Payload["delta"].(string)
			if ev.Type() == "text_delta" {
				partial = updateText(partial, delta)
			} else {
				partial = updateThinking(partial, delta)
			}
			_ = a.bus.Emit(ctx, events.MessageUpdateEvent{
				Base:    events.Base{Type: events.MessageUpdate, Turn: turn},
				Delta:   delta,
				Message: partial,
			})
		case "toolcall_start", "toolcall_delta":
			if !started {
				started = true
				partial = models.NewAgentMessage(models.RoleAssistant)
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: partial,
				})
			}
			call, _ := ev.Payload["tool_call"].(map[string]any)
			partial = updateToolCall(partial, call, false)
			_ = a.bus.Emit(ctx, events.MessageUpdateEvent{
				Base:    events.Base{Type: events.MessageUpdate, Turn: turn},
				Delta:   formatToolCallDelta(call),
				Message: partial,
			})
		case "toolcall_end":
			if !started {
				started = true
			}
			msg, _ := ev.FinalMessage()
			partial = msg
		case "done":
			msg, err := ev.FinalMessage()
			if err != nil {
				return partial, err
			}
			if !started {
				_ = a.bus.Emit(ctx, events.MessageStartEvent{
					Base:    events.Base{Type: events.MessageStart, Turn: turn},
					Message: msg,
				})
			}
			_ = a.bus.Emit(ctx, events.MessageEndEvent{
				Base:    events.Base{Type: events.MessageEnd, Turn: turn},
				Message: msg,
			})
			if usage, ok := ev.Usage(); ok {
				usage.Provider = a.cfg.Model.Provider
				usage.Model = a.cfg.Model.ID
				_ = a.obsCollector.RecordLLMUsage(usage)
			}
			a.appendMessage(msg)
			return msg, nil
		case "error":
			if ge, ok := ev.Error(); ok {
				return models.AgentMessage{}, ge
			}
			return models.AgentMessage{}, fmt.Errorf("unknown gateway error")
		}
	}

	// Stream ended without a done event: use accumulated partial.
	if !started {
		_ = a.bus.Emit(ctx, events.MessageStartEvent{
			Base:    events.Base{Type: events.MessageStart, Turn: turn},
			Message: partial,
		})
	}
	_ = a.bus.Emit(ctx, events.MessageEndEvent{
		Base:    events.Base{Type: events.MessageEnd, Turn: turn},
		Message: partial,
	})
	a.appendMessage(partial)
	return partial, nil
}

func updateText(msg models.AgentMessage, delta string) models.AgentMessage {
	if len(msg.Content) == 0 {
		msg.Content = []models.ContentPart{models.TextContent{Text: delta}}
		return msg
	}
	if text, ok := msg.Content[0].(models.TextContent); ok {
		text.Text += delta
		msg.Content[0] = text
		return msg
	}
	msg.Content = append([]models.ContentPart{models.TextContent{Text: delta}}, msg.Content...)
	return msg
}

func updateThinking(msg models.AgentMessage, delta string) models.AgentMessage {
	if len(msg.Content) == 0 {
		msg.Content = []models.ContentPart{models.ThinkingContent{Text: delta}}
		return msg
	}
	if thinking, ok := msg.Content[0].(models.ThinkingContent); ok {
		thinking.Text += delta
		msg.Content[0] = thinking
		return msg
	}
	msg.Content = append([]models.ContentPart{models.ThinkingContent{Text: delta}}, msg.Content...)
	return msg
}

func updateToolCall(msg models.AgentMessage, call map[string]any, final bool) models.AgentMessage {
	if call == nil {
		return msg
	}
	id, _ := call["id"].(string)
	name, _ := call["name"].(string)
	args, _ := call["arguments"].(map[string]any)

	for i, part := range msg.Content {
		if tc, ok := part.(models.ToolCallContent); ok && tc.ID == id {
			if name != "" {
				tc.Name = name
			}
			if tc.Arguments == nil {
				tc.Arguments = make(map[string]any)
			}
			for k, v := range args {
				tc.Arguments[k] = v
			}
			msg.Content[i] = tc
			return msg
		}
	}

	if args == nil {
		args = make(map[string]any)
	}
	msg.Content = append(msg.Content, models.ToolCallContent{
		ID:        id,
		Name:      name,
		Arguments: args,
	})
	return msg
}

func formatToolCallDelta(call map[string]any) string {
	if call == nil {
		return ""
	}
	b, _ := json.Marshal(call)
	return string(b)
}

func (a *Agent) executeToolCalls(ctx context.Context, turn int, assistantMsg models.AgentMessage, calls []models.ToolCallContent) ([]models.AgentMessage, bool) {
	sequential := a.cfg.ToolExecutionMode == models.ExecutionSequential
	if a.cfg.ModeManager != nil {
		mode := a.cfg.ModeManager.Get(a.cfg.Mode)
		if mode.ExecutionMode == "sequential" {
			sequential = true
		}
	}
	if !sequential {
		for _, call := range calls {
			if exec, ok := a.registry.Get(call.Name); ok {
				if exec.Definition().ExecutionMode == models.ExecutionSequential {
					sequential = true
					break
				}
			}
		}
	}

	if sequential {
		return a.executeSequential(ctx, turn, assistantMsg, calls)
	}
	return a.executeParallel(ctx, turn, assistantMsg, calls)
}

func (a *Agent) executeSequential(ctx context.Context, turn int, assistantMsg models.AgentMessage, calls []models.ToolCallContent) ([]models.AgentMessage, bool) {
	var results []models.AgentMessage
	allTerminate := true
	for _, call := range calls {
		resultMsg := a.executeOneToolCall(ctx, turn, assistantMsg, call)
		results = append(results, resultMsg)
		a.appendMessage(resultMsg)
		if !isToolResultTerminate(resultMsg) {
			allTerminate = false
		}
	}
	return results, allTerminate && len(calls) > 0
}

func (a *Agent) executeParallel(ctx context.Context, turn int, assistantMsg models.AgentMessage, calls []models.ToolCallContent) ([]models.AgentMessage, bool) {
	type pair struct {
		call   models.ToolCallContent
		result models.AgentMessage
	}

	var wg sync.WaitGroup
	pairs := make([]pair, len(calls))
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c models.ToolCallContent) {
			defer wg.Done()
			pairs[idx] = pair{call: c, result: a.executeOneToolCall(ctx, turn, assistantMsg, c)}
		}(i, call)
	}
	wg.Wait()

	var results []models.AgentMessage
	allTerminate := true
	for _, p := range pairs {
		results = append(results, p.result)
		a.appendMessage(p.result)
		if !isToolResultTerminate(p.result) {
			allTerminate = false
		}
	}
	return results, allTerminate && len(calls) > 0
}

func (a *Agent) executeOneToolCall(ctx context.Context, turn int, assistantMsg models.AgentMessage, call models.ToolCallContent) models.AgentMessage {
	_ = a.bus.Emit(ctx, events.ToolExecutionStartEvent{
		Base:       events.Base{Type: events.ToolExecutionStart, Turn: turn},
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Args:       call.Arguments,
	})

	// Validate / prepare arguments.
	args := call.Arguments
	if args == nil {
		args = make(map[string]any)
	}

	// Permission check via hook.
	if a.cfg.BeforeToolCall != nil {
		beforeResult, err := a.cfg.BeforeToolCall(ctx, ToolCallInfo{
			AssistantMessage: assistantMsg,
			ToolCall:         call,
			Args:             args,
			Context:          a.mgr.AllMessages(),
		})
		if err != nil {
			return a.makeToolResultMessage(call, models.NewToolResultError(err.Error()), true)
		}
		if beforeResult != nil && beforeResult.Block {
			return a.makeToolResultMessage(call, models.NewToolResultError(beforeResult.Reason), true)
		}
	}

	result, isError := a.registry.Execute(ctx, call.ID, call.Name, args)

	// Run after hook.
	if a.cfg.AfterToolCall != nil {
		afterResult, err := a.cfg.AfterToolCall(ctx, ToolCallResultInfo{
			AssistantMessage: assistantMsg,
			ToolCall:         call,
			Args:             args,
			Result:           result,
			IsError:          isError,
			Context:          a.mgr.AllMessages(),
		})
		if err != nil {
			result = models.NewToolResultError(err.Error())
			isError = true
		} else if afterResult != nil {
			if len(afterResult.Content) > 0 {
				result.Content = afterResult.Content
			}
			if afterResult.Details != nil {
				result.Details = afterResult.Details
			}
			if afterResult.IsError != nil {
				isError = *afterResult.IsError
			}
			result.Terminate = afterResult.Terminate
		}
	}

	_ = a.bus.Emit(ctx, events.ToolExecutionEndEvent{
		Base:       events.Base{Type: events.ToolExecutionEnd, Turn: turn},
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Result:     result,
		IsError:    isError,
	})

	_ = a.bus.Emit(ctx, events.MessageStartEvent{
		Base:    events.Base{Type: events.MessageStart, Turn: turn},
		Message: a.makeToolResultMessage(call, result, isError),
	})
	_ = a.bus.Emit(ctx, events.MessageEndEvent{
		Base:    events.Base{Type: events.MessageEnd, Turn: turn},
		Message: a.makeToolResultMessage(call, result, isError),
	})

	return a.makeToolResultMessage(call, result, isError)
}

func (a *Agent) makeToolResultMessage(call models.ToolCallContent, result models.ToolResult, isError bool) models.AgentMessage {
	details := result.Details
	if details == nil {
		details = make(map[string]any)
	}
	details["terminate"] = result.Terminate
	return models.NewAgentMessage(models.RoleToolResult, models.ToolResultContent{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    result.Content,
		IsError:    isError,
		Details:    details,
	})
}

func isToolResultTerminate(msg models.AgentMessage) bool {
	if len(msg.Content) == 0 {
		return false
	}
	result, ok := msg.Content[0].(models.ToolResultContent)
	if !ok {
		return false
	}
	if result.Details == nil {
		return false
	}
	v, ok := result.Details["terminate"].(bool)
	return ok && v
}

func (a *Agent) appendMessage(msg models.AgentMessage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mgr.AppendRecent(msg)
}

// Stats returns context manager statistics if available.
func (a *Agent) Stats() map[string]int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mgr.Stats()
}

// LatestAssistantMessage returns the most recent assistant message in context.
func (a *Agent) LatestAssistantMessage() (models.AgentMessage, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := a.mgr.AllMessages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == models.RoleAssistant {
			return msgs[i], true
		}
	}
	return models.AgentMessage{}, false
}

// allMessages returns the full message list from the context manager.
func (a *Agent) allMessages() []models.AgentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mgr.AllMessages()
}

func (a *Agent) drainSteeringQueue() []models.AgentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := a.steeringQueue
	a.steeringQueue = nil
	return msgs
}

func (a *Agent) drainFollowUpQueue() []models.AgentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	msgs := a.followUpQueue
	a.followUpQueue = nil
	return msgs
}

func (a *Agent) maxTurnsReached(turn int) bool {
	maxTurns := a.cfg.MaxTurns
	if a.cfg.ModeManager != nil {
		maxTurns = a.cfg.ModeManager.Get(a.cfg.Mode).EffectiveMaxTurns(maxTurns)
	}
	if maxTurns <= 0 {
		return false
	}
	return turn >= maxTurns
}

func (a *Agent) applyMode() (string, []models.ToolDefinition, models.ModelRef, models.ExecutionMode) {
	var systemParts []string
	if a.mgr != nil {
		if b, ok := a.mgr.GetBlock(contextmgr.BlockSystem, "system"); ok {
			systemParts = []string{b.Text()}
		}
	}

	tools := a.registry.Definitions()
	modelRef := a.cfg.Model
	execMode := a.cfg.ToolExecutionMode

	if a.cfg.ModeManager == nil {
		return strings.Join(systemParts, "\n\n"), tools, modelRef, execMode
	}

	mode := a.cfg.ModeManager.Get(a.cfg.Mode)
	if mode.SystemPrompt != "" {
		modeBlock := contextmgr.NewBlock(contextmgr.BlockMode, "mode", contextmgr.StabilityStable, 90,
			models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: "# Mode: " + mode.Name + "\n\n" + mode.SystemPrompt}))
		a.mgr.SetBlock(modeBlock)
	}
	if mode.SystemPrompt != "" {
		systemParts = append(systemParts, "# Mode: "+mode.Name+"\n\n"+mode.SystemPrompt)
	}
	if len(mode.AllowedTools) > 0 {
		allowed := make(map[string]bool)
		for _, p := range mode.AllowedTools {
			allowed[p] = true
		}
		var filtered []models.ToolDefinition
		for _, t := range tools {
			if matchToolName(t.Name, allowed) {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}
	if len(mode.DeniedTools) > 0 {
		denied := make(map[string]bool)
		for _, p := range mode.DeniedTools {
			denied[p] = true
		}
		var filtered []models.ToolDefinition
		for _, t := range tools {
			if !matchToolName(t.Name, denied) {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}
	if mode.Model != "" {
		modelRef.ID = mode.Model
	}
	if mode.Provider != "" {
		modelRef.Provider = mode.Provider
	}
	if mode.ExecutionMode == "sequential" {
		execMode = models.ExecutionSequential
	} else if mode.ExecutionMode == "parallel" {
		execMode = models.ExecutionParallel
	}
	return strings.Join(systemParts, "\n\n"), tools, modelRef, execMode
}

func matchToolName(name string, patterns map[string]bool) bool {
	if patterns[name] {
		return true
	}
	for p := range patterns {
		if strings.HasSuffix(p, "*") && strings.HasPrefix(name, p[:len(p)-1]) {
			return true
		}
		if strings.HasPrefix(p, "*") && strings.HasSuffix(name, p[1:]) {
			return true
		}
	}
	return false
}

func (a *Agent) shouldStop(ctx context.Context, msg models.AgentMessage, toolResults []models.AgentMessage) bool {
	if a.cfg.ShouldStop == nil {
		return true
	}
	stop, err := a.cfg.ShouldStop(ctx, TurnSummary{
		Message:     msg,
		ToolResults: toolResults,
		Context:     a.mgr.AllMessages(),
	})
	if err != nil {
		return false
	}
	return stop
}
