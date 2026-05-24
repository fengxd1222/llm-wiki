package mcp

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSessionStoreRegisterLookupTouchExpire(t *testing.T) {
	store := NewSessionStore()
	now := time.Now().UTC().Add(-2 * time.Hour)
	sess := &Session{
		Token:       "sk-1",
		Agent:       "codex-cli",
		SessionID:   "sess-1",
		CreatedAt:   now,
		LastSeenAt:  now,
		IdleTimeout: time.Hour,
	}
	if err := store.Register(sess); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := store.Lookup("sk-1")
	if !ok || got.Agent != "codex-cli" {
		t.Fatalf("Lookup = %+v, %v", got, ok)
	}
	store.Touch("sk-1")
	touched, _ := store.Lookup("sk-1")
	if !touched.LastSeenAt.After(now) {
		t.Fatalf("Touch did not update LastSeenAt: %+v", touched)
	}

	touched.LastSeenAt = now
	expired := store.Expire(now.Add(2 * time.Hour))
	if len(expired) != 1 || expired[0].Token != "sk-1" {
		t.Fatalf("expired = %+v, want sk-1", expired)
	}
	if _, ok := store.Lookup("sk-1"); ok {
		t.Fatal("expired session still in store")
	}
}

func TestSessionStoreRejectsDuplicateAgentSession(t *testing.T) {
	store := NewSessionStore()
	first := &Session{Token: "sk-1", Agent: "claude-code", SessionID: "sess-A"}
	second := &Session{Token: "sk-2", Agent: "claude-code", SessionID: "sess-A"}
	if err := store.Register(first); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := store.Register(second); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("second Register err = %v, want ErrSessionExists", err)
	}
}

func TestSessionStoreConcurrentRegister(t *testing.T) {
	store := NewSessionStore()
	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- store.Register(&Session{
				Token:     fmt.Sprintf("sk-%d", i),
				Agent:     "codex-cli",
				SessionID: fmt.Sprintf("sess-%d", i),
			})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("Register concurrent err = %v", err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, ok := store.Lookup(fmt.Sprintf("sk-%d", i)); !ok {
			t.Fatalf("missing session sk-%d", i)
		}
	}
}

func TestNewSessionTokenShape(t *testing.T) {
	token, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	if len(token) != 35 || token[:3] != "sk-" {
		t.Fatalf("token = %q, want sk- + 32 hex chars", token)
	}
}
