package agent

import (
	"context"
	"sync"

	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/models"
)

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
