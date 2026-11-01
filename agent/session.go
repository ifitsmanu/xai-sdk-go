package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/proto"
)

type sessionContextKey struct{}

// Session is the transcript and scratch state for one conversation.
// Set and Get are safe for concurrent use. The runner is the only writer of Messages.
type Session struct {
	ID       string
	Messages []*xaiapi.Message

	mu    sync.RWMutex
	state map[string]any
}

// NewSession returns a session with a random id and an empty state map.
func NewSession() *Session {
	return &Session{ID: newID(), state: map[string]any{}}
}

// Set stores a scratch value.
func (s *Session) Set(key string, value any) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		s.state = map[string]any{}
	}
	s.state[key] = value
}

// Get reads a scratch value.
func (s *Session) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	return v, ok
}

// FromContext returns the session the runner attached to ctx.
func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionContextKey{}).(*Session)
	return s, ok && s != nil
}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, s)
}

// Clone copies messages and state so parallel runs do not share a transcript.
func (s *Session) Clone() *Session {
	if s == nil {
		return NewSession()
	}
	out := &Session{ID: s.ID, state: cloneState(nil)}
	s.mu.RLock()
	out.state = cloneState(s.state)
	s.mu.RUnlock()
	out.Messages = cloneMessages(s.Messages)
	return out
}

func cloneMessages(in []*xaiapi.Message) []*xaiapi.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]*xaiapi.Message, len(in))
	for i, msg := range in {
		if msg == nil {
			continue
		}
		cloned, _ := proto.Clone(msg).(*xaiapi.Message)
		out[i] = cloned
	}
	return out
}

func cloneState(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneState(t)
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = cloneValue(t[i])
		}
		return out
	case proto.Message:
		return proto.Clone(t)
	default:
		return v
	}
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "session"
	}
	return hex.EncodeToString(b[:])
}

// Store persists sessions.
type Store interface {
	Load(ctx context.Context, id string) (*Session, error)
	Save(ctx context.Context, session *Session) error
}

// MemoryStore keeps sessions in process memory.
type MemoryStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: map[string]*Session{}}
}

// Load returns a copy of the session. A missing id returns an empty session with that id.
func (m *MemoryStore) Load(_ context.Context, id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		return s.Clone(), nil
	}
	s := NewSession()
	s.ID = id
	m.sessions[id] = s
	return s.Clone(), nil
}

// Save stores a copy of the session by id.
func (m *MemoryStore) Save(_ context.Context, session *Session) error {
	if session == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions == nil {
		m.sessions = map[string]*Session{}
	}
	m.sessions[session.ID] = session.Clone()
	return nil
}
