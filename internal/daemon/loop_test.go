package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fengxd1222/llm-wiki/internal/mcp"
	"github.com/fengxd1222/llm-wiki/internal/vault"
)

func TestDaemonStartAndShutdown(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}

	// Ensure .wikimind dir exists for log file.
	_ = os.MkdirAll(filepath.Join(root, ".wikimind"), 0o755)

	cfg := Config{VaultRoot: root}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run in background.
	done := make(chan error, 1)
	go func() {
		done <- d.Run(ctx)
	}()

	// Give it a moment to start.
	time.Sleep(100 * time.Millisecond)

	// Shutdown.
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("daemon did not shut down in time")
	}
}

func TestDaemonLockManager(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	_ = os.MkdirAll(filepath.Join(root, ".wikimind"), 0o755)

	cfg := Config{VaultRoot: root}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown() })

	lm := d.LockManager()
	if lm == nil {
		t.Fatalf("LockManager is nil")
	}

	if err := lm.Acquire("page-1", "sess-a", "agent-a", 300*time.Second); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
}

// TestDaemonReapSessions exercises F-030: the daemon's session reaper actually
// expires idle sessions registered in its SessionStore (no longer dead code).
func TestDaemonReapSessions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	_ = os.MkdirAll(filepath.Join(root, ".wikimind"), 0o755)

	d, err := New(Config{VaultRoot: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown() })

	store := d.SessionStore()
	// Register an already-idle session (no worktree → cleanup is a no-op).
	sess := &mcp.Session{
		Token:       "sk-idle",
		Agent:       "claude-code",
		SessionID:   "sess-idle",
		IdleTimeout: 1 * time.Millisecond,
		LastSeenAt:  time.Now().Add(-1 * time.Hour),
		CreatedAt:   time.Now().Add(-1 * time.Hour),
	}
	if err := store.Register(sess); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Drive one reap cycle directly.
	d.reapSessions(context.Background(), time.Now())

	if _, ok := store.Lookup("sk-idle"); ok {
		t.Fatalf("idle session was not reaped by daemon")
	}
}
