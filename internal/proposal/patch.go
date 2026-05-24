package proposal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNoChanges        = errors.New("NO_CHANGES")
	ErrPatchExists      = errors.New("PATCH_EXISTS")
	ErrPatchWriteFailed = errors.New("PATCH_WRITE_FAILED")
	ErrPatchApplyFailed = errors.New("PATCH_APPLY_FAILED")
)

// StagePath stages one vault-relative path in a worktree checkout.
func StagePath(ctx context.Context, worktreeRoot, path string) error {
	if _, err := runGit(ctx, worktreeRoot, "add", "-A", "--", filepath.FromSlash(path)); err != nil {
		return fmt.Errorf("git add %s: %w", path, err)
	}
	return nil
}

// GeneratePatch returns the staged diff from a worktree against the base ref.
//
// D11 handlers pass the agent worktree root here. The branch parameter is kept
// for the public contract and future branch-aware variants.
// The base ref is resolved at runtime via defaultBaseRef (main → master → HEAD~1)
// to handle cross-platform git init default branch differences.
func GeneratePatch(ctx context.Context, worktreeRoot, branch, path string) ([]byte, error) {
	_ = branch
	baseRef := defaultBaseRef(ctx, worktreeRoot)
	out, err := runGit(ctx, worktreeRoot,
		"diff", "--cached", "--binary", baseRef, "--", filepath.FromSlash(path))
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", path, err)
	}
	if strings.TrimSpace(out) == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoChanges, path)
	}
	return []byte(out), nil
}

// defaultBaseRef resolves the base ref for diff operations.
// Priority: main → master → HEAD~1.
// Uses `git rev-parse --verify <ref>` to check existence.
func defaultBaseRef(ctx context.Context, root string) string {
	for _, ref := range []string{"main", "master"} {
		if _, err := runGit(ctx, root, "rev-parse", "--verify", ref); err == nil {
			return ref
		}
	}
	return "HEAD~1"
}

// ApplyPatch applies a unified diff into a worktree and stages the result.
func ApplyPatch(ctx context.Context, worktreeRoot string, patch []byte) error {
	if len(bytes.TrimSpace(patch)) == 0 {
		return fmt.Errorf("%w: empty patch", ErrPatchApplyFailed)
	}
	if _, err := runGitWithInput(ctx, worktreeRoot, patch,
		"apply", "--index", "--whitespace=nowarn", "-"); err != nil {
		return fmt.Errorf("%w: %v", ErrPatchApplyFailed, err)
	}
	return nil
}

// WritePatchFile writes a review patch under wiki/_review using O_EXCL.
func WritePatchFile(ctx context.Context, vaultRoot, reviewID string, patch []byte) (string, error) {
	_ = ctx
	rel := filepath.ToSlash(filepath.Join("wiki", "_review", reviewID+".patch"))
	abs := filepath.Join(vaultRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("%w: mkdir _review: %v", ErrPatchWriteFailed, err)
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("%w: %s", ErrPatchExists, rel)
		}
		return "", fmt.Errorf("%w: open %s: %v", ErrPatchWriteFailed, rel, err)
	}
	defer f.Close()
	if _, err := f.Write(patch); err != nil {
		return "", fmt.Errorf("%w: write %s: %v", ErrPatchWriteFailed, rel, err)
	}
	return rel, nil
}

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	return runGitWithInput(ctx, root, nil, args...)
}

func runGitWithInput(ctx context.Context, root string, input []byte, args ...string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("git: empty root")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("git: root not a directory: %s", root)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(msg)
	}
	return stdout.String(), nil
}
