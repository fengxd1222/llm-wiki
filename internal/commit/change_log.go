package commit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSeqNotFound 表示 change-log.jsonl 中没有匹配的 seq 行。
var ErrSeqNotFound = errors.New("change log seq not found")

// Relative paths inside the vault (POSIX, joined with filepath.Join at use sites).
const (
	logMdRelPath      = "wiki/log.md"
	changeLogRelPath  = ".wikimind/change-log.jsonl"
	logMdHeader       = "# Wiki Change Log\n\n| seq | ts | actor | op | summary |\n|-----|----|-------|-----|---------|\n"
	defaultActor      = "user"
	timestampLayout   = time.RFC3339
	changeLogModeFile = 0o644
)

// LogEntry 是 wiki/log.md 与 .wikimind/change-log.jsonl 共用的一行记录。
//
// 字段语义遵循 agent-protocol §7.2：
//   - GitSHA：W1 MVP 简化方案下，写入 jsonl 时保留空字符串；
//     ADR-lite 决策"用 commit message 的 seq 反查 sha"，
//     由 wikimind revert <seq> 在运行时 `git log --grep` 反查。
//   - Bundle / Reviews：W3 review pipeline 后才会填，MVP 留空。
//   - Actor：MVP 写 "user"；W2 daemon 加 agent 区分。
type LogEntry struct {
	Seq       int      `json:"seq"`
	GitSHA    string   `json:"git_sha"`
	Timestamp string   `json:"ts"`
	Actor     string   `json:"actor"`
	Op        string   `json:"op"`
	Bundle    string   `json:"bundle,omitempty"`
	Reviews   []string `json:"reviews,omitempty"`
	Summary   string   `json:"summary"`
}

// NextSeq 读 .wikimind/change-log.jsonl 最后一行，返回其 seq+1；
// 文件不存在或空文件返回 1。
//
// 失败：jsonl 格式损坏 / 系统读错误。
func NextSeq(vaultRoot string) (int, error) {
	path := filepath.Join(vaultRoot, filepath.FromSlash(changeLogRelPath))
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, nil
		}
		return 0, fmt.Errorf("open change log: %w", err)
	}
	defer f.Close()

	var lastSeq int
	scanner := bufio.NewScanner(f)
	// 单行 jsonl 可能很长（含完整 entry）——给到 1MB 缓冲足够。
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return 0, fmt.Errorf("parse change log line: %w", err)
		}
		if entry.Seq > lastSeq {
			lastSeq = entry.Seq
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan change log: %w", err)
	}
	return lastSeq + 1, nil
}

// AppendChangeLog 把 entry 以 JSON 一行追加到 .wikimind/change-log.jsonl。
//
// 文件不存在自动建；O_APPEND 保证多协程串行写时不互相覆盖（同进程 race 仍需上层
// 串行化，参考 architecture.md §2.3 Single-Writer Commit Loop 的 W2 实现）。
func AppendChangeLog(vaultRoot string, entry LogEntry) error {
	path := filepath.Join(vaultRoot, filepath.FromSlash(changeLogRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure change log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, changeLogModeFile)
	if err != nil {
		return fmt.Errorf("open change log: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	// json.Encoder.Encode 已自动加 "\n"，避免手工拼接。
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("encode change log entry: %w", err)
	}
	return nil
}

// AppendLogMd 把 entry 追加到 wiki/log.md 的表格末尾。
// 首次写（文件不存在或为空）自动建 header。
//
// log.md 格式遵循 agent-protocol §7.4，行尾固定 "\n"（跨平台一致，
// 不依赖 OS 行尾约定）。
func AppendLogMd(vaultRoot string, entry LogEntry) error {
	path := filepath.Join(vaultRoot, filepath.FromSlash(logMdRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure log.md dir: %w", err)
	}

	needHeader, err := logMdNeedsHeader(path)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, changeLogModeFile)
	if err != nil {
		return fmt.Errorf("open log.md: %w", err)
	}
	defer f.Close()

	if needHeader {
		if _, err := f.WriteString(logMdHeader); err != nil {
			return fmt.Errorf("write log.md header: %w", err)
		}
	}
	line := formatLogMdLine(entry)
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("append log.md line: %w", err)
	}
	return nil
}

// logMdNeedsHeader 检查 log.md 是否需要 header（不存在或文件为空 / 无 header 行）。
//
// vault.Init 写的 log.md 仅含 "# WikiMind Log\n\n"——本函数把它视为"需要 header"，
// 因为标准表头未出现；首次 append 时会覆盖性追加完整 header 行。
//
// 检查逻辑：含 "| seq | ts |" 关键串视为已带 header。
func logMdNeedsHeader(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("read log.md: %w", err)
	}
	if len(data) == 0 {
		return true, nil
	}
	return !strings.Contains(string(data), "| seq | ts |"), nil
}

// formatLogMdLine 生成 log.md 表格行（含末尾换行）。
//
// summary 中的 "|" / 换行会破坏 Markdown 表格 —— 替换为占位符。
func formatLogMdLine(entry LogEntry) string {
	summary := sanitizeMarkdownCell(entry.Summary)
	ts := entry.Timestamp
	if ts == "" {
		ts = time.Now().UTC().Format(timestampLayout)
	}
	actor := entry.Actor
	if actor == "" {
		actor = defaultActor
	}
	return fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
		entry.Seq, ts, actor, entry.Op, summary)
}

// sanitizeMarkdownCell 把竖线与换行替换成可见占位符，保持表格不破坏。
func sanitizeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

// ReadEntryBySeq 顺序扫 change-log.jsonl 找 seq 命中的行。
//
// 未命中返回 ErrSeqNotFound；jsonl 损坏 / IO 错误透传。
func ReadEntryBySeq(vaultRoot string, seq int) (*LogEntry, error) {
	path := filepath.Join(vaultRoot, filepath.FromSlash(changeLogRelPath))
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSeqNotFound
		}
		return nil, fmt.Errorf("open change log: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse change log line: %w", err)
		}
		if entry.Seq == seq {
			return &entry, nil
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan change log: %w", err)
	}
	return nil, ErrSeqNotFound
}
