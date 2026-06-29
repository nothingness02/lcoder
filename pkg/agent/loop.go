package agent

import (
	"context"
	"strings"
	"sync"

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
		// Default "natural completion": keep looping while the model is still
		// calling tools; stop on the first turn that produces no tool calls
		// (its final natural-language answer). This lets the model observe tool
		// results and decide for itself when the task is done, rather than
		// halting after a single turn. terminate and MaxTurns, checked earlier
		// in run(), remain the hard backstops.
		return len(msg.ToolCalls()) == 0
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
