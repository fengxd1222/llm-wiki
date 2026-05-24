package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrWorktreeExists indicates that a worktree path or branch already exists.
	ErrWorktreeExists = errors.New("worktree already exists")
	// ErrWorktreeNotFound indicates that a requested worktree is absent.
	ErrWorktreeNotFound = errors.New("worktree not found")
	// ErrInvalidSessionID indicates that agent or session id is not path-safe.
	ErrInvalidSessionID = errors.New("invalid worktree session id")
	// ErrEmptyRepo indicates that git HEAD does not point at an initial commit.
	ErrEmptyRepo = errors.New("git repository has no commits")
)

const gitExe = "git"

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Worktree describes one agent-specific git worktree.
type Worktree struct {
	Path      string
	Branch    string
	Agent     string
	SessionID string
	CreatedAt time.Time
}

// CreateWorktree adds a git worktree under wiki/_worktrees for an agent session.
func CreateWorktree(ctx context.Context, vaultRoot, agent, sessionID string) (*Worktree, error) {
	spec, err := buildSpec(vaultRoot, agent, sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath(gitExe); err != nil {
		return nil, err
	}
	if err := ensureHasCommit(ctx, spec.root); err != nil {
		return nil, err
	}
	if pathExists(spec.path) || branchExists(ctx, spec.root, spec.branch) {
		return nil, fmt.Errorf("%w: %s/%s", ErrWorktreeExists, agent, sessionID)
	}
	if err := os.MkdirAll(filepath.Dir(spec.path), 0o755); err != nil {
		return nil, fmt.Errorf("create worktree parent: %w", err)
	}
	if _, err := runGit(ctx, spec.root,
		"worktree", "add", filepath.FromSlash(spec.relPath), "-b", spec.branch,
	); err != nil {
		if pathExists(spec.path) || strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("%w: %v", ErrWorktreeExists, err)
		}
		return nil, fmt.Errorf("git worktree add: %w", err)
	}
	return &Worktree{
		Path:      spec.path,
		Branch:    spec.branch,
		Agent:     agent,
		SessionID: sessionID,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// RemoveWorktree force-removes an agent worktree and deletes its branch.
// Missing worktree path or branch is treated as success.
func RemoveWorktree(ctx context.Context, vaultRoot, agent, sessionID string) error {
	spec, err := buildSpec(vaultRoot, agent, sessionID)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(gitExe); err != nil {
		return err
	}
	if pathExists(spec.path) {
		if _, err := runGit(ctx, spec.root,
			"worktree", "remove", "--force", filepath.FromSlash(spec.relPath),
		); err != nil {
			return fmt.Errorf("git worktree remove: %w", err)
		}
	} else {
		// If a user manually deleted the directory, git may still keep stale
		// worktree metadata that prevents branch deletion.
		_, _ = runGit(ctx, spec.root, "worktree", "prune")
	}
	if branchExists(ctx, spec.root, spec.branch) {
		if _, err := runGit(ctx, spec.root, "branch", "-D", spec.branch); err != nil {
			_, _ = runGit(ctx, spec.root, "worktree", "prune")
			if _, retryErr := runGit(ctx, spec.root, "branch", "-D", spec.branch); retryErr != nil {
				return fmt.Errorf("git branch delete %s: %w", spec.branch, retryErr)
			}
		}
	}
	return nil
}

// ListWorktrees parses `git worktree list --porcelain` and returns wt-* entries.
func ListWorktrees(ctx context.Context, vaultRoot string) ([]Worktree, error) {
	root, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	if _, err := exec.LookPath(gitExe); err != nil {
		return nil, err
	}
	out, err := runGit(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	var worktrees []Worktree
	for _, block := range strings.Split(strings.TrimSpace(out), "\n\n") {
		wt := parsePorcelainBlock(block)
		if wt.Branch == "" || !strings.HasPrefix(wt.Branch, "wt-") {
			continue
		}
		wt.Agent, wt.SessionID = inferAgentSession(filepath.Base(wt.Path), wt.Branch)
		worktrees = append(worktrees, wt)
	}
	return worktrees, nil
}

type worktreeSpec struct {
	root    string
	relPath string
	path    string
	branch  string
}

func buildSpec(vaultRoot, agent, sessionID string) (*worktreeSpec, error) {
	if !safeIDPattern.MatchString(agent) || !safeIDPattern.MatchString(sessionID) {
		return nil, fmt.Errorf("%w: agent/session must match %s", ErrInvalidSessionID, safeIDPattern.String())
	}
	root, err := filepath.Abs(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve vault root: %w", err)
	}
	name := "agent-" + agent + "-" + sessionID
	rel := filepath.ToSlash(filepath.Join("wiki", "_worktrees", name))
	return &worktreeSpec{
		root:    root,
		relPath: rel,
		path:    filepath.Join(root, filepath.FromSlash(rel)),
		branch:  "wt-" + agent + "-" + sessionID,
	}, nil
}

func ensureHasCommit(ctx context.Context, root string) error {
	if _, err := runGit(ctx, root, "rev-parse", "--verify", "HEAD"); err != nil {
		return fmt.Errorf("%w: %v", ErrEmptyRepo, err)
	}
	return nil
}

func branchExists(ctx context.Context, root, branch string) bool {
	_, err := runGit(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func parsePorcelainBlock(block string) Worktree {
	var wt Worktree
	for _, line := range strings.Split(block, "\n") {
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch key {
		case "worktree":
			wt.Path = value
		case "branch":
			wt.Branch = strings.TrimPrefix(value, "refs/heads/")
		}
	}
	return wt
}

func inferAgentSession(base, branch string) (string, string) {
	withoutPrefix := strings.TrimPrefix(base, "agent-")
	for _, agent := range []string{"claude-code", "codex-cli", "opencode", "cursor", "cline", "hermes", "custom"} {
		prefix := agent + "-"
		if strings.HasPrefix(withoutPrefix, prefix) {
			return agent, strings.TrimPrefix(withoutPrefix, prefix)
		}
	}
	branchRest := strings.TrimPrefix(branch, "wt-")
	if i := strings.LastIndex(branchRest, "-"); i > 0 {
		return branchRest[:i], branchRest[i+1:]
	}
	return "", ""
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runGit(ctx context.Context, root string, args ...string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("git: empty vault root")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("git: vault root not a directory: %s", root)
	}
	cmd := exec.CommandContext(ctx, gitExe, args...)
	cmd.Dir = root
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
