package proposal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePatchAndWritePatchFile(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)
	worktree := addWorktree(t, root, "wt-test")

	rel := "wiki/claims/new.md"
	if err := os.MkdirAll(filepath.Join(worktree, "wiki", "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, filepath.FromSlash(rel)), []byte("# New\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := StagePath(ctx, worktree, rel); err != nil {
		t.Fatalf("StagePath: %v", err)
	}
	patch, err := GeneratePatch(ctx, worktree, "wt-test", rel)
	if err != nil {
		t.Fatalf("GeneratePatch: %v", err)
	}
	if !strings.Contains(string(patch), "wiki/claims/new.md") {
		t.Fatalf("patch missing path:\n%s", patch)
	}

	gotRel, err := WritePatchFile(ctx, root, "r-0001", patch)
	if err != nil {
		t.Fatalf("WritePatchFile: %v", err)
	}
	if gotRel != "wiki/_review/r-0001.patch" {
		t.Fatalf("patch rel = %q", gotRel)
	}
	if _, err := WritePatchFile(ctx, root, "r-0001", patch); !errors.Is(err, ErrPatchExists) {
		t.Fatalf("second WritePatchFile err = %v, want ErrPatchExists", err)
	}
}

func TestGeneratePatchNoChanges(t *testing.T) {
	ctx := context.Background()
	root := committedRepo(t)
	worktree := addWorktree(t, root, "wt-clean")
	_, err := GeneratePatch(ctx, worktree, "wt-clean", "README.md")
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("GeneratePatch err = %v, want ErrNoChanges", err)
	}
}

func committedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustGit(t, root, "init", "-b", "main")
	if err := os.MkdirAll(filepath.Join(root, "wiki", "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("init\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustGit(t, root, "add", ".")
	mustGit(t, root, "-c", "user.name=WikiMind Test", "-c", "user.email=test@example.com", "commit", "-m", "init")
	return root
}

func addWorktree(t *testing.T, root, branch string) string {
	t.Helper()
	path := filepath.Join(root, "wiki", "_worktrees", "agent-test-"+branch)
	mustGit(t, root, "worktree", "add", path, "-b", branch)
	return path
}

func mustGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
