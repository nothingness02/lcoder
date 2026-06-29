package session

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lcoder/lcoder/pkg/models"
)

// DefaultDir returns the default session directory.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".lcoder", "sessions")
}

// hashCWD creates a stable directory name for a project path.
func hashCWD(cwd string) string {
	sum := sha256.Sum256([]byte(cwd))
	return fmt.Sprintf("%x", sum)[:16]
}

// Store persists session data.
type Store struct {
	Dir string
}

// NewStore creates a session store.
func NewStore(dir string) *Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Store{Dir: dir}
}

// Session is a persisted conversation.
type Session struct {
	ID           string
	Path         string
	CWD          string
	Messages     []models.AgentMessage
	ActiveBranch []string
	CreatedAt    int64
}

// Create initializes a new session for the given working directory.
func (s *Store) Create(cwd string) (*Session, error) {
	id := uuid.New().String()[:12]
	sess := &Session{
		ID:           id,
		Path:         s.sessionPath(cwd, id),
		CWD:          cwd,
		Messages:     []models.AgentMessage{},
		ActiveBranch: []string{},
		CreatedAt:    time.Now().Unix(),
	}
	if err := os.MkdirAll(filepath.Dir(sess.Path), 0o755); err != nil {
		return nil, err
	}
	if err := sess.Save(); err != nil {
		return nil, err
	}
	return sess, nil
}

// Load reads a session by its file path.
func (s *Store) Load(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sess := &Session{Path: path}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg models.AgentMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("invalid session line: %w", err)
		}
		sess.Messages = append(sess.Messages, msg)
		if sess.ID == "" && msg.Metadata != nil {
			if id, ok := msg.Metadata["session_id"].(string); ok {
				sess.ID = id
			}
		}
		if sess.CWD == "" && msg.Metadata != nil {
			if cwd, ok := msg.Metadata["cwd"].(string); ok {
				sess.CWD = cwd
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Rebuild active branch from root to last message without branching.
	if len(sess.Messages) > 0 {
		last := sess.Messages[len(sess.Messages)-1]
		sess.ActiveBranch = buildBranch(sess.Messages, last.ID)
	}

	return sess, nil
}

// LoadByID loads a session by project and session id.
func (s *Store) LoadByID(cwd, id string) (*Session, error) {
	return s.Load(s.sessionPath(cwd, id))
}

// List returns metadata for sessions in a project.
func (s *Store) List(cwd string) ([]Session, error) {
	dir := filepath.Join(s.Dir, hashCWD(cwd))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		sess, err := s.Load(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, *sess)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modifiedTime() > sessions[j].modifiedTime()
	})
	return sessions, nil
}

// MostRecent returns the most recently modified session for a project.
func (s *Store) MostRecent(cwd string) (*Session, error) {
	sessions, err := s.List(cwd)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}
	return &sessions[0], nil
}

// AppendMissing appends every message from msgs whose ID is not already present
// in the session, preserving order. The TUI and one-shot runner only Append the
// user message at submit time; the agent's assistant and tool_result messages
// live in its context window and must be mirrored in here after a turn so they
// actually reach disk. Dedup is by message ID, making repeated calls idempotent.
func (s *Session) AppendMissing(msgs []models.AgentMessage) error {
	have := make(map[string]bool, len(s.Messages))
	for _, m := range s.Messages {
		have[m.ID] = true
	}
	for _, m := range msgs {
		if m.ID == "" || have[m.ID] {
			continue
		}
		if err := s.Append(m); err != nil {
			return err
		}
		have[m.ID] = true
	}
	return nil
}

// Append adds a message to the session and persists it.
func (s *Session) Append(msg models.AgentMessage) error {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}
	msg.Metadata["session_id"] = s.ID
	msg.Metadata["cwd"] = s.CWD
	msg.Metadata["saved_at"] = time.Now().UnixMilli()

	if len(s.Messages) > 0 {
		last := s.Messages[len(s.Messages)-1]
		msg.ParentID = &last.ID
	}

	s.Messages = append(s.Messages, msg)
	s.ActiveBranch = buildBranch(s.Messages, msg.ID)

	return s.Save()
}

// Save writes all messages to the session file.
func (s *Session) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, msg := range s.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	return nil
}

// ActiveMessages returns messages on the active branch.
func (s *Session) ActiveMessages() []models.AgentMessage {
	set := make(map[string]struct{}, len(s.ActiveBranch))
	for _, id := range s.ActiveBranch {
		set[id] = struct{}{}
	}
	var out []models.AgentMessage
	for _, msg := range s.Messages {
		if _, ok := set[msg.ID]; ok {
			out = append(out, msg)
		}
	}
	return out
}

func (s *Store) sessionPath(cwd, id string) string {
	return filepath.Join(s.Dir, hashCWD(cwd), id+".jsonl")
}

func (s *Session) modifiedTime() int64 {
	info, err := os.Stat(s.Path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

func buildBranch(messages []models.AgentMessage, leafID string) []string {
	byID := make(map[string]models.AgentMessage, len(messages))
	for _, m := range messages {
		byID[m.ID] = m
	}

	var branch []string
	current := leafID
	for current != "" {
		branch = append([]string{current}, branch...)
		msg, ok := byID[current]
		if !ok {
			break
		}
		if msg.ParentID == nil {
			break
		}
		current = *msg.ParentID
	}
	return branch
}

// SessionID returns the session identifier.
func (s *Session) SessionID() string {
	return s.ID
}
