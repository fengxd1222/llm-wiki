package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	defer d.Shutdown()

	lm := d.LockManager()
	if lm == nil {
		t.Fatalf("LockManager is nil")
	}

	if err := lm.Acquire("page-1", "sess-a", "agent-a", 300*time.Second); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
}
