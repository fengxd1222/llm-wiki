package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateListRemoveWorktree(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)

	wt, err := CreateWorktree(ctx, root, "codex-cli", "sess-1")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if wt.Agent != "codex-cli" || wt.SessionID != "sess-1" ||
		wt.Branch != "wt-codex-cli-sess-1" {
		t.Fatalf("worktree metadata = %+v", wt)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, ".git")); err != nil {
		t.Fatalf("worktree .git missing: %v", err)
	}

	listed, err := ListWorktrees(ctx, root)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if len(listed) != 1 || listed[0].Branch != wt.Branch ||
		listed[0].Agent != "codex-cli" || listed[0].SessionID != "sess-1" {
		t.Fatalf("listed worktrees = %+v", listed)
	}

	if err := RemoveWorktree(ctx, root, "codex-cli", "sess-1"); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wt.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree path after remove err = %v, want not exist", err)
	}
	if branchExists(ctx, root, wt.Branch) {
		t.Fatalf("branch %s still exists after remove", wt.Branch)
	}
	if err := RemoveWorktree(ctx, root, "codex-cli", "sess-1"); err != nil {
		t.Fatalf("RemoveWorktree second call should be idempotent: %v", err)
	}
}

func TestCreateWorktreeDuplicate(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)

	if _, err := CreateWorktree(ctx, root, "claude-code", "sess-A"); err != nil {
		t.Fatalf("CreateWorktree first: %v", err)
	}
	if _, err := CreateWorktree(ctx, root, "claude-code", "sess-A"); !errors.Is(err, ErrWorktreeExists) {
		t.Fatalf("CreateWorktree duplicate err = %v, want ErrWorktreeExists", err)
	}
}

func TestRemoveWorktreeAfterDirectoryWasDeleted(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)
	wt, err := CreateWorktree(ctx, root, "codex-cli", "sess-gone")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if err := os.RemoveAll(wt.Path); err != nil {
		t.Fatalf("RemoveAll worktree path: %v", err)
	}
	if err := RemoveWorktree(ctx, root, "codex-cli", "sess-gone"); err != nil {
		t.Fatalf("RemoveWorktree after manual delete: %v", err)
	}
	if branchExists(ctx, root, wt.Branch) {
		t.Fatalf("branch %s still exists", wt.Branch)
	}
}

func TestCreateWorktreeRejectsUnsafeIDs(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)

	cases := []struct {
		agent string
		sess  string
	}{
		{"codex-cli", "../sess"},
		{"codex/cli", "sess"},
		{"codex-cli", ""},
		{strings.Repeat("a", 65), "sess"},
	}
	for _, tc := range cases {
		if _, err := CreateWorktree(ctx, root, tc.agent, tc.sess); !errors.Is(err, ErrInvalidSessionID) {
			t.Fatalf("CreateWorktree(%q,%q) err = %v, want ErrInvalidSessionID", tc.agent, tc.sess, err)
		}
	}
}

func TestCreateWorktreeEmptyRepo(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	mustRunGit(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "wiki", "_worktrees"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, err := CreateWorktree(ctx, root, "codex-cli", "sess-1"); !errors.Is(err, ErrEmptyRepo) {
		t.Fatalf("CreateWorktree empty repo err = %v, want ErrEmptyRepo", err)
	}
}

func TestIsWorktreeWriteAllowed(t *testing.T) {
	cases := []struct {
		path    string
		wantErr error
	}{
		{"wiki/claims/foo.md", nil},
		{"wiki/entities/foo.md", nil},
		{"raw/inbox/foo.md", ErrRawWriteForbidden},
		{"schema/AGENTS.md", ErrSchemaWriteForbidden},
		{"wiki/_worktrees/agent-x/y.md", ErrWorktreeWriteForbidden},
		{"../outside.md", ErrPathOutsideWorktree},
		{"notes/foo.md", ErrPathOutsideWorktree},
	}
	for _, tc := range cases {
		err := IsWorktreeWriteAllowed(tc.path)
		if tc.wantErr == nil && err != nil {
			t.Fatalf("%s err = %v, want nil", tc.path, err)
		}
		if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s err = %v, want %v", tc.path, err, tc.wantErr)
		}
	}
}

func committedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustRunGit(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "wiki", "_worktrees"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustRunGit(t, root, "add", ".")
	mustRunGit(t, root, "-c", "user.name=WikiMind Test", "-c", "user.email=test@example.com", "commit", "-m", "init")
	return root
}

func mustRunGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
