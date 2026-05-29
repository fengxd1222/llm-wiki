# ST2 · Protocol Contract — log_append + Sentinels + Dynamic Counts

> 父任务：`05-28-v0-1-1-patch-13-fixes-from-v0-1-0-audit`
> 维度：契约稳定性。含 4 条 finding：F-027 / F-025 / F-048 / F-049。
> 执行顺序：先 helper/API（ToolCount/RuleCount/sentinel 提取）→ 后 caller。

## Goal

修复 4 条协议契约 finding：change-log op 字面量错位、inline error 无法比对、CLI banner 硬编码工具/规则数。**只改这 4 块。**

## Fixes（按依赖顺序）

### F-025 · MCP inline `errors.New` 提取为包级 sentinel【P1 / 契约】

- **位置**：`internal/mcp/tools.go:433, 439`（grep `errors.New("[A-Z_]"`）。
- **现状**：`errors.New("CROSS_SESSION_BUNDLE")` / `errors.New("REVIEW_ALREADY_BUNDLED")` 等 inline 创建，caller 无法 `errors.Is` 比对。
- **修复方向**：在 `tools.go` 顶部 `var (...)` 块加包级 sentinel（`ErrCrossSessionBundle` / `ErrReviewAlreadyBundled` 等），调用点改引用。保持错误码字面量不变（`CROSS_SESSION_BUNDLE` 等）。
- **测试**：`errors.Is` 断言单测。

### F-027 · change-log op `append_log` → `log_append`【P1 / 契约破坏】

- **位置**：`internal/mcp/tools.go:492`（`CommitWithActor(..., "append_log", ...)`）+ `internal/mcp/tools_test.go:605`（断言 `"append_log"`）。
- **现状**：spec 多处用 `log_append`，代码写 `append_log`；未来 dream-cycle / revert 按 spec 查为空集。
- **决策**：直接改成 `log_append`，不做向后兼容（v0.1.0 仅 dogfood）。
- **修复方向**：两处字面量改 `log_append`；若有 change-log fixture 同步更新。
- **测试**：tools_test.go 断言改为 `log_append`。

### F-048 · `command.go:108` 硬编码 "15 tools"【P1 / CLI 契约】

- **位置**：`cmd/wikimind/command.go:108`（mcp serve banner）+ `internal/mcp/server.go`。
- **现状**：banner 写"15 tools"，实际 17。
- **修复方向**：`internal/mcp/server.go` 加 `ToolCount() int`（或 `RegisteredTools() []string`）；`command.go` 改 `fmt.Sprintf("... %d tools", srv.ToolCount())`。
- **测试**：ToolCount 返回值单测 + 与实际注册数一致断言。

### F-049 · `command.go:821` 硬编码 "(8 rules)"【P1 / CLI 契约】

- **位置**：`cmd/wikimind/command.go:821`（lint banner）+ `internal/lint`。
- **现状**：banner 写"(8 rules)"，实际 5。
- **修复方向**：`internal/lint` 加 `RuleCount()` 或 `RuleNames() []string`；command.go 动态查询。
- **测试**：RuleCount 与实际规则数一致断言。

## Acceptance Criteria

- [ ] F-025：MCP 协议错误码全部为包级 sentinel，caller 用 `errors.Is`；字面量不变。
- [ ] F-027：代码 + 测试中 `append_log` 全部改为 `log_append`，无残留。
- [ ] F-048：mcp serve banner 工具数动态来自 `server.ToolCount()`。
- [ ] F-049：lint banner 规则数动态来自 lint 包 API。
- [ ] `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- [ ] 未触碰范围外 finding。

## Out of Scope

- ❌ 其他 9 条 finding（ST1/ST3/ST4/ST5）。
- ❌ Spec-Drift 修复（F-026 工具数 spec 更新走 trellis-update-spec）。
- ❌ 修改 `.trellis/spec/`。

## Technical Notes

- 审查报告：`.trellis/tasks/archive/2026-05/05-27-code-quality-audit-v0-1-0/audit-report.md`（F-025/027/048/049 详条 + W2 D11 `<spec-entry>`）。
- 标尺：`error-handling.md`（sentinel + 协议错误码）/ `logging-guidelines.md`（op 枚举含 `log_append`）/ `quality-guidelines.md`（CLI 输出契约）。
- 协议错误码字面量是契约：本子任务只改"如何创建 error"（inline → sentinel），不改字面量本身。F-027 是唯一改字面量的（spec 是权威方）。
