package agent

import (
	"context"
	"sync"

	"github.com/lcoder/lcoder/pkg/models"
)

// stateHolder owns the agent runtime state, steering/follow-up queues, and
// per-stream abort control. It exists so the top-level Agent can stay a
// coordinator rather than a God Object.
type stateHolder struct {
	mu            sync.Mutex
	state         State
	steeringQueue []models.AgentMessage
	followUpQueue []models.AgentMessage

	// Loop-level abort.
	abortCh   chan struct{}
	abortOnce sync.Once

	// In-flight stream abort (set by the current turn's streamer).
	streamAbort context.CancelFunc
}

func newStateHolder() *stateHolder {
	return &stateHolder{abortCh: make(chan struct{})}
}

func (s *stateHolder) clone(steeringQueue, followUpQueue []models.AgentMessage) *stateHolder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &stateHolder{
		state:         s.state,
		steeringQueue: append([]models.AgentMessage(nil), steeringQueue...),
		followUpQueue: append([]models.AgentMessage(nil), followUpQueue...),
		abortCh:       make(chan struct{}),
	}
}

// State returns the current agent state.
func (s *stateHolder) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// SetState updates the agent state.
func (s *stateHolder) SetState(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
}

// ResetAbort prepares a fresh abort channel for a new run.
func (s *stateHolder) ResetAbort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.abortCh = make(chan struct{})
	s.abortOnce = sync.Once{}
}

// Steer injects a user message during the next safe boundary.
func (s *stateHolder) Steer(msg models.AgentMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steeringQueue = append(s.steeringQueue, msg)
}

// FollowUp queues a message after the agent would otherwise stop.
func (s *stateHolder) FollowUp(msg models.AgentMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.followUpQueue = append(s.followUpQueue, msg)
}

// Abort signals the current run to stop gracefully. Safe to call multiple times.
func (s *stateHolder) Abort() {
	s.mu.Lock()
	cancel := s.streamAbort
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.abortOnce.Do(func() {
		close(s.abortCh)
	})
}

// SetStreamAbort registers the cancel function for the in-flight stream.
func (s *stateHolder) SetStreamAbort(cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamAbort = cancel
}

// ClearStreamAbort unregisters the in-flight stream cancel function.
func (s *stateHolder) ClearStreamAbort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamAbort = nil
}

// DrainSteeringQueue returns and clears the steering queue.
func (s *stateHolder) DrainSteeringQueue() []models.AgentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.steeringQueue
	s.steeringQueue = nil
	return msgs
}

// DrainFollowUpQueue returns and clears the follow-up queue.
func (s *stateHolder) DrainFollowUpQueue() []models.AgentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.followUpQueue
	s.followUpQueue = nil
	return msgs
}
