package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"maps"
	"slices"
	"sync"

	"github.com/ifitsmanu/xai-sdk-go/responses"
)

// Session is one conversation: its items and scratch state.
// Get and Set are safe for concurrent use; only the runner writes Items.
type Session struct {
	ID    string
	Items []responses.Item

	mu    sync.RWMutex
	state map[string]any
}

// NewSession returns an empty session with a random id.
func NewSession() *Session {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return &Session{ID: hex.EncodeToString(b[:]), state: map[string]any{}}
}

// Set stores a state value. Instructions read it through {key} placeholders.
func (s *Session) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		s.state = map[string]any{}
	}
	s.state[key] = value
}

// Get reads a state value.
func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	return v, ok
}

// Clone copies the item list and the state map. Items themselves are immutable.
func (s *Session) Clone() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &Session{ID: s.ID, Items: slices.Clone(s.Items), state: maps.Clone(s.state)}
}

type sessionKey struct{}

// FromContext returns the session of the run that called a tool.
func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(*Session)
	return s, ok
}

// Store persists sessions between runs.
type Store interface {
	Load(ctx context.Context, id string) (*Session, error)
	Save(ctx context.Context, s *Session) error
}

// MemoryStore keeps sessions in process memory.
type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// Load returns a copy of the session, or a new one with that id.
func (m *MemoryStore) Load(_ context.Context, id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s.Clone(), nil
	}
	s := NewSession()
	s.ID = id
	return s, nil
}

// Save stores a copy of the session.
func (m *MemoryStore) Save(_ context.Context, s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		m.sessions = map[string]*Session{}
	}
	m.sessions[s.ID] = s.Clone()
	return nil
}
