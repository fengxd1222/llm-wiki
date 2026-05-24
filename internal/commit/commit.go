package commit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// commitMu 序列化 Commit 调用，作为 W1 MVP 的"单写者"占位实现。
// W2 daemon 上线后由 Single-Writer Commit Loop (architecture.md §2.3) 替换。
var commitMu sync.Mutex

// Commit 是 service-layer 唯一的写入闸门：把若干 vault-relative 路径作为本次
// 操作的源改动，加上自动生成的 wiki/log.md + .wikimind/change-log.jsonl 两个
// log 文件，一起放进**同一个** git commit。
//
// 流程（按 prd ADR-lite）：
//  1. EnsureRepo —— 确保 git 仓库（首次自动 git init）
//  2. NextSeq —— 读 jsonl 最后行 +1
//  3. Write log entries (git_sha="") —— append 到 log.md + jsonl
//  4. GitAdd source files + log files
//  5. GitCommit "<op>: <summary> (seq=<N>)" —— 拿 short sha
//  6. **不**回填 jsonl 的 git_sha；返回 entry 时填好供调用方使用
//
// op:     "ingest" / "revert" / 后续 "lint_fix" / "review_accept"...
// summary: 给人看的一行话；含 "|" / 换行会被 sanitize
// files:  vault-relative POSIX 路径（如 "raw/inbox/foo.md"）；空 slice 合法
//
// 返回的 LogEntry.GitSHA 是本次 commit 的 short sha；jsonl 中保留 ""。
func Commit(ctx context.Context, vaultRoot, op, summary string, files []string) (*LogEntry, error) {
	return CommitWithActor(ctx, vaultRoot, defaultActor, op, summary, files)
}

// CommitWithActor is Commit with an explicit actor for MCP-managed writes.
func CommitWithActor(ctx context.Context, vaultRoot, actor, op, summary string, files []string) (*LogEntry, error) {
	commitMu.Lock()
	defer commitMu.Unlock()

	if err := EnsureRepo(ctx, vaultRoot); err != nil {
		return nil, err
	}

	seq, err := NextSeq(vaultRoot)
	if err != nil {
		return nil, fmt.Errorf("compute next seq: %w", err)
	}

	entry := LogEntry{
		Seq:       seq,
		GitSHA:    "", // ADR-lite: 留空，revert 时用 commit message 反查
		Timestamp: time.Now().UTC().Format(timestampLayout),
		Actor:     actor,
		Op:        op,
		Summary:   summary,
	}

	// 1) 先把 log 落盘——commit 也把它们带走，保证 source + log 同一 commit。
	if err := AppendChangeLog(vaultRoot, entry); err != nil {
		return nil, err
	}
	if err := AppendLogMd(vaultRoot, entry); err != nil {
		return nil, err
	}

	// 2) GitAdd source + log 文件（一起进同一个 commit）。
	addPaths := make([]string, 0, len(files)+2)
	for _, f := range files {
		if f == "" {
			continue
		}
		addPaths = append(addPaths, f)
	}
	addPaths = append(addPaths, logMdRelPath, changeLogRelPath)
	if err := GitAdd(ctx, vaultRoot, addPaths...); err != nil {
		return nil, err
	}

	// 3) commit。message 嵌 seq 让 wikimind revert <seq> 可反查。
	message := fmt.Sprintf("%s: %s (seq=%d)", op, summary, seq)
	sha, err := GitCommit(ctx, vaultRoot, message)
	if err != nil {
		return nil, err
	}

	// 4) 返回 entry 含真实 sha（jsonl 中保留 ""）。
	entry.GitSHA = sha
	return &entry, nil
}
