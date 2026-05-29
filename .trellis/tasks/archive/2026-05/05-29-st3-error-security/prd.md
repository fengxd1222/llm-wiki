# ST3 · Error & Security — Claim Sentinel + LIKE Escape + Fail-Closed

> 父任务：`05-28-v0-1-1-patch-13-fixes-from-v0-1-0-audit`
> 维度：错误处理 + 安全。含 3 条 finding：F-009 / F-010 / F-012。
> 执行顺序：先 escapeLikePattern helper（F-010）→ 其余。

## Goal

修复 3 条错误处理 / 安全 finding：sql.ErrNoRows 泄漏、LIKE 元字符未转义、fail-open 绕过闸门。**只改这 3 块。**

## Fixes

### F-010 · LIKE 模式拼接未转义 `idempotency_key`【P1 / 安全 / CWE-89 风格】

- **位置**：`internal/index/`（`FindReviewByIdempotencyKey` 类查询；实现时 grep `LIKE` 定位精确 line）。
- **现状**：用户提供的 `idempotency_key` 含 `%` / `_` / `\` 时破坏幂等契约——可命中非目标行或错过自己的行。
- **修复方向**：
  1. 加 `escapeLikePattern(s string) string` helper：把 `\` → `\\`、`%` → `\%`、`_` → `\_`（顺序：先转义反斜杠）。
  2. SQL 改 `... LIKE ? ESCAPE '\'`，参数传 escaped 值。
  3. 检查同包内是否有其他 LIKE 查询需同样处理。
- **测试**：helper 单测覆盖 3 个元字符 + 端到端单测（含 `%` 的 key 不误命中）。

### F-009 · `claim_sources.go` 暴露 `sql.ErrNoRows`【P1 / 错误处理】

- **位置**：`internal/index/claim_sources.go`（`FindXxx` 返回 `(nil, sql.ErrNoRows)`）。
- **现状**：违反 error-handling spec"未找到行返回包级 sentinel，不暴露 sql.ErrNoRows"。
- **修复方向**：加包级 `ErrClaimSourceNotFound`；`Scan` 命中 `sql.ErrNoRows` 时 `return nil, ErrClaimSourceNotFound`；同时检查同文件其他查询（F-011 同模式的 `(nil,nil)` 若顺手可一并，但属 P2 不强制）。
- **测试**：`errors.Is(err, ErrClaimSourceNotFound)` 断言。

### F-012 · `CheckQueueForPropose` fail-open 静默吞错【P1 / 安全/正确性】

- **位置**：`internal/service/queue.go`（`CheckQueueForPropose`）。
- **现状**：DB 查询出错时返回"队列未满"，agent 可在队列实际已满时仍 propose（绕过配额闸门）。
- **修复方向**：改 fail-closed——DB 错误用 `fmt.Errorf("check queue: %w", err)` 返回；MCP 工具入口把它 surface 为错误码（如 `QUEUE_CHECK_FAILED`）而非静默放行。
- **测试**：注入 DB 错误的单测，断言返回 error 而非"未满"。

## Acceptance Criteria

- [ ] F-010：`escapeLikePattern` helper 存在 + 单测；LIKE 查询用 `ESCAPE '\'`；含元字符的 key 不误命中。
- [ ] F-009：`claim_sources.go` 不再向上暴露 `sql.ErrNoRows`，改包级 sentinel。
- [ ] F-012：`CheckQueueForPropose` DB 错误 fail-closed，不静默放行。
- [ ] `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- [ ] 未触碰范围外 finding。

## Out of Scope

- ❌ 其他 finding（ST1/ST2/ST4/ST5）。
- ❌ F-011（`(nil,nil)` 未命中，P2）——可顺手但不强制；若做则同 sentinel 模式。
- ❌ 修改 `.trellis/spec/`。

## Technical Notes

- 审查报告：F-009/010/012 详条。
- 标尺：`error-handling.md`（sentinel / 不暴露 sql.ErrNoRows / fail-fast 不吞错）/ `database-guidelines.md`（参数化 / LIKE 转义）。
- F-010 是安全相关，参考 CWE-89；FTS5 MATCH 转义同理（但本任务只管 LIKE）。
