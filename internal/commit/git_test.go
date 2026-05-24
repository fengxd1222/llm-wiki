package commit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRepoInitializesGitRepository(t *testing.T) {
	requireGit(t)
	root := t.TempDir()

	if err := EnsureRepo(context.Background(), root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf(".git missing after EnsureRepo: %v", err)
	}
}

func TestEnsureRepoReportsMissingGit(t *testing.T) {
	t.Setenv("PATH", "")

	err := EnsureRepo(context.Background(), t.TempDir())
	if !errors.Is(err, ErrGitMissing) {
		t.Fatalf("EnsureRepo err = %v, want ErrGitMissing", err)
	}
}

func TestGitCommitCleanWorktreeReturnsError(t *testing.T) {
	requireGit(t)
	root := initializedRepo(t)

	_, err := GitCommit(context.Background(), root, "empty")
	if err == nil {
		t.Fatalf("GitCommit clean worktree should fail")
	}
}

func TestGitRevertCreatesReverseCommit(t *testing.T) {
	requireGit(t)
	root := initializedRepo(t)
	notePath := filepath.Join(root, "note.md")
	if err := os.WriteFile(notePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	if err := GitAdd(context.Background(), root, "note.md"); err != nil {
		t.Fatalf("GitAdd: %v", err)
	}
	sha, err := GitCommit(context.Background(), root, "ingest: note.md (seq=1)")
	if err != nil {
		t.Fatalf("GitCommit: %v", err)
	}

	revertSHA, err := GitRevert(context.Background(), root, sha)
	if err != nil {
		t.Fatalf("GitRevert: %v", err)
	}
	if strings.TrimSpace(revertSHA) == "" {
		t.Fatalf("GitRevert returned empty sha")
	}
	if _, err := os.Stat(notePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("note.md after revert err = %v, want not exist", err)
	}
}

func TestGitRevertNoCommitPreservesAppendOnlyLogs(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "raw", "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir raw/inbox: %v", err)
	}
	noteRel := "raw/inbox/note.md"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(noteRel)), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed note: %v", err)
	}
	entry, err := Commit(context.Background(), root, "ingest", noteRel, []string{noteRel})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	paths, err := GitRevertNoCommit(context.Background(), root, entry.GitSHA)
	if err != nil {
		t.Fatalf("GitRevertNoCommit: %v", err)
	}
	if len(paths) != 1 || paths[0] != noteRel {
		t.Fatalf("changed paths = %v, want [%s]", paths, noteRel)
	}

	logRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(changeLogRelPath)))
	if err != nil {
		t.Fatalf("read change log: %v", err)
	}
	if !strings.Contains(string(logRaw), `"seq":1`) {
		t.Fatalf("change log lost original seq after no-commit revert:\n%s", logRaw)
	}
}

func TestEnsureRepoCreatesMainBranch(t *testing.T) {
	requireGit(t)
	root := t.TempDir()

	if err := EnsureRepo(context.Background(), root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("symbolic-ref: %v", err)
	}
	if branch := strings.TrimSpace(string(out)); branch != "main" {
		t.Fatalf("expected branch 'main' after EnsureRepo, got %q", branch)
	}
}

func TestEnsureRepoRenamesMasterToMain(t *testing.T) {
	requireGit(t)
	root := t.TempDir()

	// Manually init with master branch (simulating old git default)
	cmd := exec.Command("git", "init", "--initial-branch=master", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --initial-branch=master: %v\n%s", err, out)
	}

	// EnsureRepo should rename master → main
	if err := EnsureRepo(context.Background(), root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("symbolic-ref: %v", err)
	}
	if branch := strings.TrimSpace(string(out)); branch != "main" {
		t.Fatalf("expected branch 'main' after EnsureRepo on master repo, got %q", branch)
	}
}

func TestEnsureRepoIdempotentOnMain(t *testing.T) {
	requireGit(t)
	root := t.TempDir()

	// Init with main branch
	cmd := exec.Command("git", "init", "--initial-branch=main", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --initial-branch=main: %v\n%s", err, out)
	}

	// EnsureRepo should be a no-op
	if err := EnsureRepo(context.Background(), root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("symbolic-ref: %v", err)
	}
	if branch := strings.TrimSpace(string(out)); branch != "main" {
		t.Fatalf("expected branch 'main' after idempotent EnsureRepo, got %q", branch)
	}
}

func initializedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := EnsureRepo(context.Background(), root); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	return root
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(gitExe); err != nil {
		t.Skip("git executable not available")
	}
}
