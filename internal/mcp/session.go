package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrSessionExists indicates that an agent/session_id pair is already active.
var ErrSessionExists = errors.New("SESSION_EXISTS")

// ErrSessionRequired indicates that a write tool was called without an active session.
var ErrSessionRequired = errors.New("SESSION_REQUIRED")

const defaultIdleTimeout = 60 * time.Minute

// Session describes one active MCP agent session.
type Session struct {
	Token         string
	Agent         string
	Version       string
	SessionID     string
	Capabilities  []string
	SchemaVersion string
	WorktreePath  string
	Branch        string
	CreatedAt     time.Time
	LastSeenAt    time.Time
	IdleTimeout   time.Duration
}

// SessionStore tracks active sessions in memory for the current MCP process.
type SessionStore struct {
	mu      sync.RWMutex
	byToken map[string]*Session
	byKey   map[string]*Session
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		byToken: map[string]*Session{},
		byKey:   map[string]*Session{},
	}
}

// Register stores a session unless the agent/session_id pair already exists.
func (s *SessionStore) Register(sess *Session) error {
	if sess == nil {
		return errors.New("session is nil")
	}
	if sess.Token == "" || sess.Agent == "" || sess.SessionID == "" {
		return errors.New("session token, agent, and session_id are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(sess.Agent, sess.SessionID)
	if _, ok := s.byKey[key]; ok {
		return fmt.Errorf("%w: %s", ErrSessionExists, key)
	}
	if sess.IdleTimeout == 0 {
		sess.IdleTimeout = defaultIdleTimeout
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	if sess.LastSeenAt.IsZero() {
		sess.LastSeenAt = sess.CreatedAt
	}
	s.byToken[sess.Token] = sess
	s.byKey[key] = sess
	return nil
}

// Lookup returns a session by token.
func (s *SessionStore) Lookup(token string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byToken[token]
	return sess, ok
}

// Authenticate checks that a session token is present, active, and not expired.
func (s *SessionStore) Authenticate(token string) (*Session, error) {
	if token == "" {
		return nil, ErrSessionRequired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byToken[token]
	if !ok {
		return nil, ErrSessionRequired
	}
	timeout := sess.IdleTimeout
	if timeout == 0 {
		timeout = defaultIdleTimeout
	}
	if time.Since(sess.LastSeenAt) > timeout {
		delete(s.byToken, token)
		delete(s.byKey, sessionKey(sess.Agent, sess.SessionID))
		return nil, ErrSessionRequired
	}
	return sess, nil
}

// Touch updates LastSeenAt for a session token.
func (s *SessionStore) Touch(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.byToken[token]; ok {
		sess.LastSeenAt = time.Now().UTC()
	}
}

// Expire removes and returns sessions whose idle timeout has elapsed.
func (s *SessionStore) Expire(now time.Time) []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expired []*Session
	for token, sess := range s.byToken {
		timeout := sess.IdleTimeout
		if timeout == 0 {
			timeout = defaultIdleTimeout
		}
		if now.Sub(sess.LastSeenAt) <= timeout {
			continue
		}
		delete(s.byToken, token)
		delete(s.byKey, sessionKey(sess.Agent, sess.SessionID))
		expired = append(expired, sess)
	}
	return expired
}

func (s *SessionStore) remove(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byToken[token]
	if !ok {
		return
	}
	delete(s.byToken, token)
	delete(s.byKey, sessionKey(sess.Agent, sess.SessionID))
}

func sessionKey(agent, sessionID string) string {
	return agent + "/" + sessionID
}

func newSessionToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return "sk-" + hex.EncodeToString(b[:]), nil
}
