package commit

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

// ErrNotGitRepo 表示 vault root 不是 git 工作区，且 EnsureRepo 之外的命令禁止自动 init。
var ErrNotGitRepo = errors.New("not a git repository")

// ErrGitMissing 表示系统 PATH 中找不到 git 可执行文件——CLI 友好提示用。
var ErrGitMissing = errors.New("git executable not found in PATH")

// gitExe 是 git 可执行名。Windows 自动解析为 git.exe（exec.LookPath 行为）。
const gitExe = "git"

// 默认 commit 身份用于用户环境没有全局 git config 的场景。
const (
	defaultGitUserName  = "WikiMind"
	defaultGitUserEmail = "wikimind@localhost"
)

// EnsureRepo 在 vaultRoot 下确认（或建立）git 仓库。
//
// 已是 git work tree（含子目录情况）→ no-op。
// 不是仓库 → 自动 `git init`，与 vault.Init 行为一致。
// git 缺失 → ErrGitMissing。
func EnsureRepo(ctx context.Context, vaultRoot string) error {
	if _, err := exec.LookPath(gitExe); err != nil {
		return ErrGitMissing
	}
	ok, err := isInsideGitWorkTree(ctx, vaultRoot)
	if err == nil && ok {
		return nil
	}
	if _, err := runGit(ctx, vaultRoot, "init"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

// GitAdd 把若干 vault-relative POSIX 路径加入 index。
// 路径前已 filepath.FromSlash 规范化，跨平台一致。
func GitAdd(ctx context.Context, vaultRoot string, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	if _, err := exec.LookPath(gitExe); err != nil {
		return ErrGitMissing
	}
	args := []string{"add", "-A", "--"}
	for _, p := range paths {
		rel := filepath.FromSlash(p)
		if rel == "" {
			continue
		}
		if isMissingAndStagedForDeletion(ctx, vaultRoot, rel) {
			continue
		}
		args = append(args, rel)
	}
	if len(args) == 3 {
		return nil
	}
	if _, err := runGit(ctx, vaultRoot, args...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// GitCommit 创建一个新的 commit，返回 short SHA（git log -1 --format=%h）。
//
// "干净 worktree → nothing to commit" 透传 git 原始错误；调用方按需处理。
func GitCommit(ctx context.Context, vaultRoot, message string) (string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return "", ErrGitMissing
	}
	if _, err := runGitWithCommitIdentity(ctx, vaultRoot, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	out, err := runGit(ctx, vaultRoot, "log", "-1", "--format=%h")
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// GitRevert 创建一个反向 commit，返回新 commit 的 short SHA。
//
// 使用 --no-edit 避免 git 拉起编辑器；MVP 不处理 conflict（透传错误）。
func GitRevert(ctx context.Context, vaultRoot, sha string) (string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return "", ErrGitMissing
	}
	if strings.TrimSpace(sha) == "" {
		return "", errors.New("git revert: empty sha")
	}
	if _, err := runGitWithCommitIdentity(ctx, vaultRoot, "revert", "--no-edit", sha); err != nil {
		return "", fmt.Errorf("git revert %s: %w", sha, err)
	}
	out, err := runGit(ctx, vaultRoot, "log", "-1", "--format=%h")
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// GitRevertNoCommit 应用 sha 的反向 patch，但不创建 commit。
// 调用方负责 stage/commit 返回路径。`wikimind revert <seq>` 用它保证反向 patch
// 与 op=revert log 行进入同一个带 seq 的 commit。
func GitRevertNoCommit(ctx context.Context, vaultRoot, sha string) ([]string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return nil, ErrGitMissing
	}
	if strings.TrimSpace(sha) == "" {
		return nil, errors.New("git revert: empty sha")
	}
	if _, err := runGit(ctx, vaultRoot, "revert", "--no-commit", sha); err != nil {
		return nil, fmt.Errorf("git revert --no-commit %s: %w", sha, err)
	}
	// source commit 含 append-only log 文件；直接 revert 会删旧 log 行。
	// 写入新的 revert 行前先从 HEAD 恢复它们。
	if _, err := runGit(ctx, vaultRoot,
		"restore", "--source=HEAD", "--staged", "--worktree", "--",
		filepath.FromSlash(logMdRelPath),
		filepath.FromSlash(changeLogRelPath),
	); err != nil {
		return nil, fmt.Errorf("restore append-only logs after revert: %w", err)
	}
	paths, err := GitChangedPaths(ctx, vaultRoot)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, errors.New("git revert produced no changed paths")
	}
	return paths, nil
}

// GitStatus 返回 git status --porcelain 的行（trim 后），用于 dirty 检测。
//
// 仓库 clean → 空 slice；非仓库 → ErrNotGitRepo。
func GitStatus(ctx context.Context, vaultRoot string) ([]string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return nil, ErrGitMissing
	}
	if ok, err := isInsideGitWorkTree(ctx, vaultRoot); err != nil || !ok {
		return nil, ErrNotGitRepo
	}
	out, err := runGit(ctx, vaultRoot, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// GitChangedPaths 返回 index 或 worktree 中已变更的 vault-relative POSIX 路径。
// 它刻意忽略不属于当前 revert/apply 操作的 untracked 文件。
func GitChangedPaths(ctx context.Context, vaultRoot string) ([]string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return nil, ErrGitMissing
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, args := range [][]string{
		{"diff", "--name-only"},
		{"diff", "--cached", "--name-only"},
	} {
		out, err := runGit(ctx, vaultRoot, args...)
		if err != nil {
			return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		for _, line := range strings.Split(out, "\n") {
			path := filepath.ToSlash(strings.TrimSpace(line))
			if path == "" {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func isMissingAndStagedForDeletion(ctx context.Context, vaultRoot, rel string) bool {
	if _, err := os.Stat(filepath.Join(vaultRoot, rel)); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	out, err := runGit(ctx, vaultRoot,
		"diff", "--cached", "--name-only", "--diff-filter=D", "--", rel)
	return err == nil && strings.TrimSpace(out) != ""
}

// FindCommitBySeq 在 git history 中按 commit message 后缀 "(seq=<N>)" 查找
// 对应的 commit short SHA。
//
// 命中多个时返回最早（最久远）的——按 ADR-lite 决策，seq 应当唯一；多匹配
// 暗示历史被改写，调用方按需处理。
func FindCommitBySeq(ctx context.Context, vaultRoot string, seq int) (string, error) {
	if _, err := exec.LookPath(gitExe); err != nil {
		return "", ErrGitMissing
	}
	// 用 fixed-string 模式 + 字面量 "(seq=<N>)" 锚定避免 seq=1 误命中 seq=10。
	pattern := fmt.Sprintf("(seq=%d)", seq)
	out, err := runGit(ctx, vaultRoot,
		"log", "--all", "--fixed-strings", "--grep", pattern, "--format=%h")
	if err != nil {
		return "", fmt.Errorf("git log --grep: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("%w: no commit for seq=%d", ErrSeqNotFound, seq)
}

// isInsideGitWorkTree 复用 vault.isInsideGitWorkTree 的语义：调用
// `git rev-parse --is-inside-work-tree` 判断 vaultRoot 是否在 git 工作区。
func isInsideGitWorkTree(ctx context.Context, vaultRoot string) (bool, error) {
	out, err := runGit(ctx, vaultRoot, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// runGit 在 vaultRoot 下执行 git 命令。
//
// 用 cmd.Dir = vaultRoot 避免 chdir（保证并发安全）。
// 错误 stderr 优先；为空则用 err.Error() 兜底。
func runGit(ctx context.Context, vaultRoot string, args ...string) (string, error) {
	if strings.TrimSpace(vaultRoot) == "" {
		return "", errors.New("git: empty vault root")
	}
	if info, err := os.Stat(vaultRoot); err != nil || !info.IsDir() {
		return "", fmt.Errorf("git: vault root not a directory: %s", vaultRoot)
	}
	cmd := exec.CommandContext(ctx, gitExe, args...)
	cmd.Dir = vaultRoot
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

func runGitWithCommitIdentity(ctx context.Context, vaultRoot string, args ...string) (string, error) {
	withIdentity := append([]string{
		"-c", "user.name=" + defaultGitUserName,
		"-c", "user.email=" + defaultGitUserEmail,
	}, args...)
	return runGit(ctx, vaultRoot, withIdentity...)
}
