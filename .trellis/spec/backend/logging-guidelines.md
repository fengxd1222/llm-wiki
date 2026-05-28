# Logging Guidelines

> WikiMind 后端的"日志"约定。
> 本项目的日志哲学与典型服务有别——**审计日志是数据，不是 stdout**。

---

## Overview

WikiMind 是 local-first CLI/MCP，不是服务端。日志有三类，分别走三条路：

| 类别 | 目的 | 落点 | 谁读 |
|------|------|------|------|
| **审计日志** | 谁、什么时候、对 vault 做了什么写操作（不可丢） | `.wikimind/change-log.jsonl` + `wiki/log.md` + git commit message | 用户、Agent、未来 revert / dream cycle |
| **用户输出** | 命令结果、状态摘要 | stdout（命令结束打印） | 终端用户 |
| **错误输出** | 失败原因 | stderr（`error: <msg>`） | 终端用户 |
| ~~应用日志~~ | ~~debug / info / warn~~ | **不使用** | — |

**核心原则**：审计日志是 vault 数据的一部分（受 git 管理、可 revert），不是 ops 工具的副产品。
不要为了"知道发生了什么"而往 stdout/stderr 喷 `log.Printf`；要么写进 change-log（重要、需追溯），要么不写（debug 用 `go test -v` 或调试器）。

---

## What Logging Library

- **不使用** `log`、`slog`、`logrus`、`zap`、`zerolog` 等通用日志库。
- 用户输出：`fmt.Fprintln(out, ...)` / `fmt.Fprintf(out, ...)`，`out` 由调用方注入（CLI 入口传 `os.Stdout` / `os.Stderr`）。
- 审计写入：`internal/commit` + `internal/changelog` 包提供 `AppendChangeLog` / `AppendLogMd`。

当某天确实需要结构化 daemon 日志（W2+ daemon 长期运行）时，再引入 `log/slog`（标准库），日志走 stderr，**仍不应替代审计日志**。

---

## Audit Log Format

### `.wikimind/change-log.jsonl`（机器可读）

每行一个完整 JSON 对象，UTF-8，行尾固定 `"\n"`（跨平台一致）：

```jsonl
{"seq":1,"git_sha":"","timestamp":"2026-05-23T10:15:00Z","actor":"user","op":"ingest","summary":"raw/inbox/foo.md"}
{"seq":2,"git_sha":"","timestamp":"2026-05-23T10:16:30Z","actor":"claude-code/sess-1","op":"propose_edit","summary":"r-0007"}
{"seq":3,"git_sha":"","timestamp":"2026-05-23T10:18:00Z","actor":"user","op":"review_accept","summary":"r-0007"}
```

**字段约定**：

| 字段 | 类型 | 约束 |
|------|------|------|
| `seq` | int | 单调递增，从 1 开始，不复用、不跳号 |
| `git_sha` | string | **W1 留空 `""`**；用 `git log --grep "(seq=N)"` 反查 |
| `timestamp` | string | RFC 3339 UTC，秒级精度，例 `2026-05-23T10:15:00Z` |
| `actor` | string | `user` / `<agent-name>/<session-id>` / `system` |
| `op` | string | 见下方 op 枚举；不可改字面量（协议契约） |
| `summary` | string | 一行简述；含 `|` 或换行会被 sanitize |

**op 枚举（不完整，新增需同步 spec-v2/templates/change-log-format.md）**：

`ingest` / `revert` / `propose_page` / `propose_edit` / `propose_claim` / `request_review` /
`review_accept` / `review_reject` / `log_append` / `lint_fix` / `dream_consolidate`。

### `wiki/log.md`（人读 Markdown 表格）

```markdown
# WikiMind Log

| seq | time                 | actor              | op            | summary           |
|-----|----------------------|--------------------|---------------|-------------------|
| 1   | 2026-05-23T10:15:00Z | user               | ingest        | raw/inbox/foo.md  |
| 2   | 2026-05-23T10:16:30Z | claude-code/sess-1 | propose_edit  | r-0007            |
```

与 jsonl 一一对应，**同一 git commit 内同时追加**，保证两者不漂移。

---

## What to Log (审计)

**必须**追加 change-log 的事件——所有改变 vault 内容的写操作：

| 事件 | op | 触发位置 |
|------|----|---------|
| 文件导入 vault/raw/ | `ingest` | `internal/service/ingest.go` |
| 撤销某 seq | `revert` | `cmd/wikimind` revert 命令 |
| Agent 提议改动 | `propose_page` / `propose_edit` / `propose_claim` | `internal/mcp/tools.go` |
| 提交 review bundle | `request_review` | `internal/mcp/tools.go` |
| 接受 / 拒绝 review | `review_accept` / `review_reject` | review CLI / MCP |
| Agent 直接追加 log | `log_append` | MCP `log_append` 工具 |
| Lint 自动修复 | `lint_fix` | lint apply 路径 |

**写闸门是唯一入口**：所有 op 都通过 `commit.Commit(ctx, vaultRoot, op, summary, files)` 落地，
该函数会把 source 文件 + log.md + change-log.jsonl 三者放进**同一个** git commit。

```go
// internal/commit/commit.go
entry, err := commit.Commit(ctx, vaultRoot, "ingest", rawID, []string{rawID})
if err != nil {
    return fmt.Errorf("commit ingest: %w", err)
}
// entry.GitSHA 是本次 commit 的 short sha；change-log.jsonl 内的 git_sha 留空（ADR-lite）
```

---

## What NOT to Log

### 不要 spawn 应用日志

```go
// ❌ 错误：往 stderr / stdout 喷应用层 log
log.Printf("opening db at %s", path)
fmt.Println("[info] processing file", name)
slog.Info("ingest done", "raw_id", rawID)

// ✅ 正确：错误才 return；成功路径不喷日志
//        要追溯就追加 change-log，不要往日志库塞
```

### 不要把进度/debug 写进 change-log

change-log 是契约：每条对应一次"语义上完整"的写。不要：

```jsonl
{"seq":1,"op":"ingest_start", ...}   // ❌ 错误：op 不是状态机过程
{"seq":2,"op":"parse_chunk", ...}    // ❌ 错误：内部进度不该入 log
```

只在事件**最终完成**时写一行。

### 不要日志敏感信息

虽然项目是 local-first，但 vault 可能被 git push 到 GitHub，change-log.jsonl 也会被推上去。**禁止**写入：

- API key、access token（包括 MCP `session_token`，token 永远只在内存里）。
- `.env` / 系统路径 / 用户名（用 `~/...` 替换或截断）。
- 完整的 raw 内容（write `summary` 时只放 `raw_id` 或 short hash，不放正文）。
- 第三方 API 响应原文（如未来接 LLM API，response body 不入 log）。

### 不要在测试里 log 到全局

测试用 `t.Log(...)` / `t.Logf(...)`，不要 `fmt.Println` 或 `log.Printf` —— 测试输出由 `go test -v` 控制。

---

## User Output (stdout/stderr)

### 写命令结果到 stdout

```go
fmt.Fprintln(cmd.OutOrStdout(), "initialized:", absRoot)
fmt.Fprintf(out, "schema_version: %s\n", version)
```

- 用 `cobra.Command.OutOrStdout()` 取 writer，不要直接 `os.Stdout`（便于测试注入 buffer）。
- 输出格式稳定：CLI 测试会断言 `initialized: <abs-path>` 这类字面量；改前先看 spec-v2/templates/cli-contract.md。
- 多行输出每行一个事实，**不要 ASCII art / 颜色码**（除非用户开了 `--color`）；CI 解析友好优先。

### 错误到 stderr

```go
fmt.Fprintln(os.Stderr, "error:", err)
os.Exit(1)
```

- 仅在 `cmd/wikimind/main.go` 入口处统一打印；子命令内部 `return err`。
- 一行简短描述，不带 wrap chain 完整链路（用户不需要 6 层 `: : :`）。

### 进度提示

长任务（如 `wikimind ingest <large-pdf>`）的进度走 stderr 的 `\r` 重写，**完成后清空**：

```go
fmt.Fprintf(os.Stderr, "\ringesting: %d/%d", done, total)
// 完成时
fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 40))  // 清空
```

避免把进度行污染到 jsonl 输出或测试 stdout 捕获。

---

## Daemon / MCP Server Logging（未来）

当 `cmd/wikimindd/` 长期运行（W2+），MCP server 进程内的"哪个 session 调了哪个工具"信息：

- **审计部分** → 仍走 change-log（每次 `propose_*` / `log_append`）。
- **运维部分**（启动、关停、健康检查、内部状态）→ `log/slog` 写 stderr，结构化（JSON handler）；级别 INFO / ERROR 二选一，不引入 DEBUG / TRACE。
- **不**把审计放 slog；**不**把运维放 change-log。两条管道职责严格分开。

当前（W1/W2）daemon 尚未上线，本节作为前向约定占位。

---

## Forbidden / Required

### Forbidden

- ❌ 用 `log` / `slog` 替代审计——所有写操作必须经 `commit.Commit` 追加 change-log。
- ❌ 在 change-log 写中间状态、debug 信息、第三方 API 原文。
- ❌ 直接 `os.Stdout.WriteString(...)` 绕过 cobra writer 注入。
- ❌ 修改 jsonl 字段名 / op 枚举字符串（破坏 revert / dream cycle 反查）。
- ❌ 在审计 summary 里写 raw 文件正文、token、绝对路径含用户名。

### Required

- ✅ 所有改 vault 的写都通过 `commit.Commit`（或 `CommitWithActor` for MCP）。
- ✅ change-log.jsonl 一行一 JSON、UTF-8、行尾 `\n`、append-only。
- ✅ `wiki/log.md` 与 jsonl 同 commit 同步追加。
- ✅ 时间戳 RFC 3339 UTC 秒级。
- ✅ CLI 错误用 `fmt.Fprintln(os.Stderr, "error:", err)` + `os.Exit(1)`。
- ✅ CLI 成功输出经 `cobra.Command.OutOrStdout()`。

---

## Examples

参考实现：

- `internal/commit/commit.go:31` — `Commit` 写闸门：`source files + log.md + change-log.jsonl` 同一 commit。
- `internal/commit/change_log.go:85` — `AppendChangeLog` 写 jsonl 实现。
- `internal/commit/change_log.go:107` — `AppendLogMd` 写 Markdown 表格实现。
- `cmd/wikimind/main.go:10-16` — CLI 入口错误打印 + 退出码。
- `cmd/wikimind/command.go` — 子命令使用 `cmd.OutOrStdout()` 写结果。
- `spec-v2/templates/change-log-format.md` — change-log JSON schema 协议契约（未来修改前先读）。
