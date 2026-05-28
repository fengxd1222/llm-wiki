# Error Handling

> WikiMind 后端的错误处理约定。
> 核心是：**用 sentinel error + wrap，把错误码留给协议边界**，绝不 panic、绝不静默吞错。

---

## Overview

- **语言模式**：Go 1.26+ 标准 `error` interface + `errors.Is/As` + `fmt.Errorf("%w", ...)`。
- **不引入第三方**：不用 `pkg/errors`、`zap` 等；标准库已足够。
- **不 panic**：业务路径全部返回 `error`；只有 `main` 函数入口或确实不可恢复的 init（`panic("driver not registered")`）允许 panic。
- **三个边界**：
  1. **包内错误流**：sentinel error + `fmt.Errorf("%w: context", ErrX, ...)` 包裹。
  2. **协议边界**（MCP / CLI）：把 sentinel 翻译成稳定的字符串错误码或 CLI 退出码。
  3. **用户边界**（CLI 输出）：`fmt.Fprintln(os.Stderr, "error:", err)` + `os.Exit(1)`，错误信息简洁、不带堆栈。

---

## Error Types

### 1. Sentinel Errors（包级 `var`）

包级 `var Err<X> = errors.New("...")`，固定不变的"错误身份"。

**命名**：`Err` 前缀 + 名词性短语，PascalCase。
**消息**：

- 内部错误（仅自己 `errors.Is` 比较，不直接暴露给用户）：用自然语言，例 `"vault config is missing"`。
- 协议错误码（要原样返回给 MCP / CLI）：用 `SCREAMING_SNAKE_CASE`，例 `"SESSION_REQUIRED"`、`"AGENT_NOT_WHITELISTED"`、`"CROSS_SESSION_BUNDLE"`。

```go
// internal/vault/config.go
var ErrConfigMissing = errors.New("vault config is missing")
var ErrInvalidConfig = errors.New("vault config is invalid")

// internal/vault/vault.go
var ErrNonEmptyDirectory = errors.New("vault directory already exists and is not empty")

// internal/index/index.go
var ErrIndexUnavailable = errors.New("index unavailable")

// internal/mcp/session.go  ← 跨越协议边界，名字即错误码
var ErrSessionRequired = errors.New("SESSION_REQUIRED")
```

### 2. Wrapped Errors（`fmt.Errorf` + `%w`）

带上下文的错误用 `fmt.Errorf("%w: <context>", ErrX, ...)`：

```go
if err := os.MkdirAll(dbDir, 0o755); err != nil {
    return nil, fmt.Errorf("%w: create .wikimind: %v", ErrIndexUnavailable, err)
}
```

- 链式 wrap：用 `%w`（保留可 `errors.Is`）；最底层的原始错误用 `%v`（避免双重 wrap）。
- 上下文格式：`"<verb> <object>: <detail>"`，例 `"resolve vault root: %w"`、`"compute next seq: %w"`。

### 3. Typed Errors（结构体实现 `error`）

当错误需要携带结构化字段（行号、文件名、键值）时才用结构体；否则优先 sentinel。

```go
type SchemaValidationError struct {
    Field   string
    Reason  string
}

func (e *SchemaValidationError) Error() string {
    return fmt.Sprintf("schema: %s %s", e.Field, e.Reason)
}
```

调用方用 `errors.As(err, &target)` 取字段。
目前项目里这种场景少；多数仍用 sentinel + wrap。

---

## Error Handling Patterns

### 包内传播：fail fast + wrap

每跨越一个语义层就 wrap 一层上下文，**不要丢错**：

```go
func Open(vaultRoot string) (*DB, error) {
    if vaultRoot == "" {
        return nil, fmt.Errorf("%w: vault root is empty", ErrIndexUnavailable)
    }
    absRoot, err := filepath.Abs(vaultRoot)
    if err != nil {
        return nil, fmt.Errorf("%w: resolve vault root: %v", ErrIndexUnavailable, err)
    }
    // ... 每步都 wrap 同一个 sentinel，让调用方可以 errors.Is(err, ErrIndexUnavailable)
}
```

### 比较错误：永远 `errors.Is` / `errors.As`

```go
// 正确
if errors.Is(err, os.ErrNotExist) { ... }
if errors.Is(err, ErrGitMissing) { ... }

// 错误：失去 wrap 后的可比较性
if err == ErrGitMissing { ... }
if err.Error() == "git missing" { ... }
```

### `defer` 清理 + 错误聚合

资源持有路径：失败时清理已分配资源，不要泄漏：

```go
sqlDB, err := sql.Open(...)
if err != nil {
    return nil, fmt.Errorf("%w: open sqlite: %v", ErrIndexUnavailable, err)
}
if err := sqlDB.Ping(); err != nil {
    _ = sqlDB.Close()  // 清理，但 ping 错误是主因
    return nil, fmt.Errorf("%w: ping sqlite: %v", ErrIndexUnavailable, err)
}
```

`_ = xxx.Close()` 显式标注"这里忽略 Close 错误是有意的"（主流程已有错误，Close 错误不该覆盖它）。

### 不要静默吞错

**禁止**：

```go
_, _ = doSomething()           // ❌ 把错误丢掉
if err := xxx(); err != nil { } // ❌ 空 if 块
defer file.Close()             // ⚠️ 写场景必须 check，读场景视情况
```

唯一允许 `_ =` 的场景：

1. 已知错误且文档化（`_ = sqlDB.Close()` 在已有主错误时）。
2. 清理性操作（`_, _ = runGit(ctx, root, "worktree", "prune")` 是 best-effort 兜底）。

### 没有结果就返回 sentinel，不返回 nil error

**未找到** != **错误的查询**，但仍是调用方需要分支处理的：

```go
// 正确
func FindReviewByID(ctx, db, id) (*Review, error) {
    row := db.QueryRowContext(...)
    if err := row.Scan(...); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrReviewNotFound  // 转译成包级 sentinel
        }
        return nil, fmt.Errorf("scan review: %w", err)
    }
    return &r, nil
}
```

不暴露 `sql.ErrNoRows` 给上层（实现细节泄漏）。

---

## Protocol Boundaries

### CLI 退出码

`cmd/wikimind/main.go` 的退出规则：

```go
func main() {
    cmd := newRootCommand(os.Stdout, os.Stderr)
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    }
}
```

- 正常退出：`0`。
- 任何业务/输入错误：`1`，错误信息走 stderr，**一行**简短描述（不带堆栈、不带 Go 风格 wrap chain）。
- cobra 校验错误（usage 错误）：由 cobra 处理；不要再额外打印。

### MCP 错误码

MCP 工具返回的错误必须可被 Agent 程序化匹配，约定：

- 错误码用 `SCREAMING_SNAKE_CASE`，定义在 sentinel 的 `errors.New(...)` 消息里。
- 工具实现里把内部错误翻译到稳定错误码：

```go
if !sessionStore.HasToken(token) {
    return nil, ErrSessionRequired  // "SESSION_REQUIRED"
}
if claim.Source.Depth > 1 {
    return ProposeResult{}, fmt.Errorf("%w: %s", proposal.ErrProvenanceDepthExceeded, path)
}
```

- 已定义的协议错误码（不完整清单，新增需同步 spec-v2/templates/mcp-tools.md）：
  - `SESSION_REQUIRED` — 写工具缺少 `session_token` 或 token 过期。
  - `AGENT_NOT_WHITELISTED` — `agent_handshake` 时 agent 不在 `allowed_agents`。
  - `SESSION_EXISTS` — 同 `(agent, session_id)` 已存在 worktree。
  - `WORKTREE_CREATE_FAILED` — git worktree 创建失败（含空仓 `ErrEmptyRepo`）。
  - `CROSS_SESSION_BUNDLE` — request_review 包含其他 session 的 review。
  - `REVIEW_ALREADY_BUNDLED` — 重复 bundle 同一 review。
  - `ErrBaseHashMismatch` / `ErrProvenanceDepthExceeded` / `ErrQuoteHashMismatch` / `ErrPatchExists` / `ErrPatchApplyFailed` / `ErrPathNotAllowed` / `ErrSchemaViolation` — 见 `internal/proposal`。

### 不可恢复 vs 可恢复

| 情形 | 处理 |
|------|------|
| 用户输入非法（路径错、schema 失败） | wrap sentinel 返回，CLI 退出 1 / MCP 返回错误码 |
| 文件系统瞬时错（permission） | 同上，**不重试**（写闸门由 daemon 串行化，瞬时错通常意味着配置问题） |
| 网络 / 外部进程（ripgrep、git） | wrap 错误返回上层；ripgrep 可降级到 LIKE，但需返回 `degraded=true` |
| 程序 bug（不应到达的分支） | 返回带前缀 `"BUG: ..."` 的错误，**不 panic** |
| 启动时不可恢复（驱动未注册） | `init` 函数里允许 `panic` |

---

## Common Mistakes

### ❌ 已踩过的坑

- **`err == ErrX` 直接比较**：wrap 之后会失效；统一用 `errors.Is`。
- **吞 Close 错误而不带主错误上下文**：`defer db.Close()` 写场景下 Close 错误也可能掩盖真实问题；写场景显式 check。
- **同时 `%w` 多个 error**：Go 1.20+ 允许 `errors.Join`，但项目里多数是线性链；不要嵌套 `%w` of `%w` 制造迷宫。
- **CLI 错误堆栈泄漏**：把 wrap chain 直接打给用户。用户只需要"error: vault path is required"，不是 6 层 `: : : :`。在 CLI 入口处用 `errors.Unwrap` 或 sentinel 翻译成人话。
- **`sql.ErrNoRows` 上漏到 service**：service 应只见 `ErrXxxNotFound`，不见标准库实现细节。
- **协议错误码乱改**：MCP 错误码字面量是协议契约。修改 `var ErrSessionRequired = errors.New("SESSION_REQUIRED")` 的字符串就是 breaking change，必须升 schema major。

### ✅ 检查清单

- [ ] 函数返回 `error` 而不是 panic。
- [ ] sentinel error 用 `errors.New` 在包级 `var` 声明，名字 `ErrXxx`。
- [ ] wrap 用 `fmt.Errorf("%w: ...", ErrX, ...)`；不重复 wrap 同一 sentinel 制造迷宫。
- [ ] 比较错误用 `errors.Is` / `errors.As`。
- [ ] 未找到行返回包级 sentinel，不暴露 `sql.ErrNoRows`。
- [ ] 协议错误码（MCP / 退出码）在 spec-v2/templates 里有定义。
- [ ] CLI 入口只打一行 "error: xxx"，stderr，退出码 1。
- [ ] 资源清理用 `_ = xxx.Close()` 显式标注。

---

## Examples

参考实现：

- `internal/vault/config.go:19-22` — sentinel + wrap pattern 样板。
- `internal/index/index.go:30,44` — `ErrIndexUnavailable` 多层 wrap。
- `internal/mcp/session.go:15` — 协议错误码型 sentinel（消息即错误码）。
- `internal/commit/git.go:233` — `errors.Is(err, os.ErrNotExist)` 比较标准库错误。
- `internal/worktree/worktree_test.go:59,96,110` — `errors.Is` 在测试里断言错误身份的样板。
- `cmd/wikimind/main.go:10-16` — CLI 入口的极简错误退出。
