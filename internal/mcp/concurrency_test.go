package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	worktreepkg "github.com/fengxd1222/llm-wiki/internal/worktree"
)

// TestLockManagerConcurrentInit exercises F-028: concurrent first-access to
// lockManager() must not race. Run with `go test -race`.
func TestLockManagerConcurrentInit(t *testing.T) {
	b := &vaultBackend{root: t.TempDir()}

	const n = 50
	var wg sync.WaitGroup
	managers := make([]interface{}, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			managers[idx] = b.lockManager()
		}(i)
	}
	wg.Wait()

	// All goroutines must observe the same singleton instance.
	first := b.lockManager()
	for i := 0; i < n; i++ {
		if managers[i] != first {
			t.Fatalf("lockManager() returned different instances (idx %d)", i)
		}
	}
}

// TestSessionStoreConcurrentSessionInit exercises F-028 for sessionStore().
func TestSessionStoreConcurrentSessionInit(t *testing.T) {
	b := &vaultBackend{root: t.TempDir()}

	const n = 50
	var wg sync.WaitGroup
	stores := make([]*SessionStore, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			stores[idx] = b.sessionStore()
		}(i)
	}
	wg.Wait()

	first := b.sessionStore()
	for i := 0; i < n; i++ {
		if stores[i] != first {
			t.Fatalf("sessionStore() returned different instances (idx %d)", i)
		}
	}
}

// committedVaultWithWorktreeSupport builds a git repo with an initial commit
// and the wiki/_worktrees directory so CreateWorktree succeeds.
func committedVaultWithWorktreeSupport(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGitT(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "wiki", "_worktrees"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitT(t, root, "add", ".")
	runGitT(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "init")
	return root
}

func runGitT(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestExpireAndCleanupRemovesWorktree exercises F-029: expiring an idle session
// removes its git worktree from disk.
func TestExpireAndCleanupRemovesWorktree(t *testing.T) {
	ctx := context.Background()
	root := committedVaultWithWorktreeSupport(t)

	wt, err := worktreepkg.CreateWorktree(ctx, root, "codex-cli", "sess-expire")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if _, err := os.Stat(wt.Path); err != nil {
		t.Fatalf("worktree path missing pre-expire: %v", err)
	}

	store := NewSessionStore()
	sess := &Session{
		Token:        "sk-expire",
		Agent:        "codex-cli",
		SessionID:    "sess-expire",
		WorktreePath: wt.Path,
		Branch:       wt.Branch,
		IdleTimeout:  1 * time.Millisecond,
		LastSeenAt:   time.Now().Add(-1 * time.Hour), // already idle
		CreatedAt:    time.Now().Add(-1 * time.Hour),
	}
	if err := store.Register(sess); err != nil {
		t.Fatalf("Register: %v", err)
	}

	expired, errs := store.ExpireAndCleanup(ctx, time.Now(), root)
	if len(errs) != 0 {
		t.Fatalf("cleanup errors: %v", errs)
	}
	if len(expired) != 1 {
		t.Fatalf("expired = %d, want 1", len(expired))
	}

	// Worktree must be gone from disk.
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists after expire+cleanup: err=%v", err)
	}
	// Session must be removed from the store.
	if _, ok := store.Lookup("sk-expire"); ok {
		t.Fatalf("session still present after expire")
	}
}

// TestExpireAndCleanupSkipsActiveSessions ensures non-idle sessions survive.
func TestExpireAndCleanupSkipsActiveSessions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store := NewSessionStore()
	sess := &Session{
		Token:       "sk-active",
		Agent:       "claude-code",
		SessionID:   "sess-active",
		IdleTimeout: 1 * time.Hour,
		LastSeenAt:  time.Now(),
		CreatedAt:   time.Now(),
	}
	if err := store.Register(sess); err != nil {
		t.Fatalf("Register: %v", err)
	}

	expired, errs := store.ExpireAndCleanup(ctx, time.Now(), root)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(expired) != 0 {
		t.Fatalf("expired = %d, want 0 (session is active)", len(expired))
	}
	if _, ok := store.Lookup("sk-active"); !ok {
		t.Fatalf("active session was incorrectly expired")
	}
}
