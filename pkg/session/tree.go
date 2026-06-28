package session

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/lcoder/lcoder/pkg/models"
)

// Fork creates a new session starting from a specific message.
func (s *Store) Fork(cwd string, source *Session, atMessageID string) (*Session, error) {
	newID := uuid.New().String()[:12]
	newPath := s.sessionPath(cwd, newID)

	byID := make(map[string]models.AgentMessage, len(source.Messages))
	for _, m := range source.Messages {
		byID[m.ID] = m
	}

	var history []models.AgentMessage
	current := atMessageID
	for current != "" {
		msg, ok := byID[current]
		if !ok {
			return nil, fmt.Errorf("message %s not found", current)
		}
		history = append([]models.AgentMessage{msg}, history...)
		if msg.ParentID == nil {
			break
		}
		current = *msg.ParentID
	}

	sess := &Session{
		ID:           newID,
		Path:         newPath,
		CWD:          cwd,
		Messages:     history,
		ActiveBranch: buildBranch(history, atMessageID),
	}
	if err := sess.Save(); err != nil {
		return nil, err
	}
	return sess, nil
}

// Clone duplicates the current active branch into a new session.
func (s *Store) Clone(cwd string, source *Session) (*Session, error) {
	if len(source.ActiveBranch) == 0 {
		return s.Create(cwd)
	}
	leafID := source.ActiveBranch[len(source.ActiveBranch)-1]
	return s.Fork(cwd, source, leafID)
}

// Tree returns all messages reachable from the active branch leaves.
func (s *Session) Tree() []models.AgentMessage {
	return s.Messages
}
