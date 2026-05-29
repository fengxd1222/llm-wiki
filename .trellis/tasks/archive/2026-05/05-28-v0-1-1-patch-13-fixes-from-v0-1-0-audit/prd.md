# v0.1.1 Patch — 13 Fixes from v0.1.0 Audit

## Goal

把 v0.1.0 全量审查（`.trellis/tasks/archive/2026-05/05-27-code-quality-audit-v0-1-0/audit-report.md`，2530 行 / 75 entries / 0 P0 / 24 P1 / 48 P2 / 13 Spec-Drift）筛出的 **13 条高 ROI 低风险 P1** 在 v0.1.1 收尾。

预计代码改动 ≈ 150-180 行 + 1 个 CI step。

## What I already know

- v0.1.0 已发布（commit `5d88ffc` 审查报告归档）；零 P0，意味着可冻结发布、本 patch 是质量加固。
- 13 条 finding 全部已识别 `file:line` 锚点 + 修复方向；不需要重新调研。
- 审查标尺仍是 `.trellis/spec/backend/` 5 份 spec（patch 期间不再放宽）。
- Spec-Drift（13 条）走独立 `trellis-update-spec` 流程，**不在本 patch 范围**。
- v0.2 重构（F-002 空壳包 / F-005 command.go 拆分 / F-006 mcp 拆分 / F-008 git 包合并 / F-013 长函数等）**不在本 patch 范围**。

## Assumptions (temporary)

- 每条修复保留 v0.1.0 既有行为（不引入新 breaking change）。例外：F-027 把 `append_log` 改 `log_append` 是协议修正（但 v0.1.0 dogfood 阶段量极小，可接受）。
- 每条修复加最小回归测试（无须 80% 覆盖率，但触发原 finding 的路径必须可由测试触达）。
- 不引入新生产依赖；govulncheck 只在 CI（开发依赖）。

## Decision (ADR-lite)

**Context**：13 条 P1 来自 v0.1.0 审查，需要决定拆分粒度、顺序、跨平台验证强度、协议字面量兼容策略。

**Decisions**（2026-05-28 brainstorm 对齐）：

- **Q1 拆分粒度** → **按维度拆 5 个子任务**（见下方 Subtask Breakdown）。每组高内聚、可独立 commit + 测试。
- **Q2 执行顺序** → **按依赖：先共享 helper / API 后 caller**。先落地 `ToolCount()` / `RuleCount()` / `escapeLikePattern()` / 协议 sentinel 提取，再改调用点，避免重复改同一文件。
- **Q3 Windows 验证** → **修代码 + 加 Windows CI matrix**（GitHub Actions 加 windows-latest job 跑 bridge 单测）。
- **Q4 govulncheck** → **纳入本 patch CI step**（warning gate，不 hard fail；归入子任务 5）。
- **Q5 F-027 字面量** → **直接改 `append_log` → `log_append`，不做向后兼容**（v0.1.0 仅 dogfood，历史 change-log 量极小）。

**Consequences**：
- 5 子任务可并行/串行；子任务 1（CI 增强基础）和子任务内 helper 先行。
- Windows CI matrix 引入后，后续所有 PR 都受 windows-latest 约束（正向）。
- F-027 直接改意味着若有 dogfood vault 存在 `op=append_log` 历史行，revert 旧 seq 时查不到——可接受（dogfood 数据非生产）。

## Subtask Breakdown（按维度，5 组）

> 执行顺序：每组内部"先 helper/API 后 caller"。组间建议序：ST5（CI 基础设施）可先建好 → ST2（协议 helper）→ ST3（错误/安全 helper）→ ST1（跨平台）→ ST4（并发资源）。但各组可独立 commit。

| 子任务 | 维度 | 含 finding | 估算 |
|--------|------|-----------|------|
| **ST1** | 跨平台 | F-041（Bridge Windows IPC + CI matrix）/ F-042（Watcher race）| ~35 行 + CI matrix |
| **ST2** | 协议契约 | F-027（log_append 字面量）/ F-025（sentinel 提取）/ F-048（ToolCount）/ F-049（RuleCount）| ~25 行 |
| **ST3** | 错误与安全 | F-009（claim sentinel）/ F-010（LIKE 转义）/ F-012（fail-closed）| ~35 行 |
| **ST4** | 并发与资源 | F-028（lockManager sync.Once）/ F-029（session worktree 清理）/ F-030（Expire 激活）| ~45 行 |
| **ST5** | 用户契约 + CI | F-003+F-069（worker honesty + doctor）/ govulncheck CI step | ~15 行 + CI step |

## Fix Checklist

> 顺序与 audit 报告"v0.1.1 必修清单"一致，按推荐严重度排序。每条都有源锚点 + 修复方向 + 估算。

### 🔴 阻塞 W3+ daemon 上线（必须先修）

#### F-041 · Bridge SocketPath Windows 不可用

- **位置**：`internal/bridge/bridge.go`（SocketPath 实现 + Listen 调用）。
- **现状**：Windows 上 `SocketPath` 返回 `\\.\pipe\wikimind-<base>` 但 `net.Listen("unix", ...)` 监听该路径会失败。W2 无 daemon 不爆，W3 daemon 上线即崩。
- **修复方向**：
  1. Windows: SocketPath 改回普通文件路径（例如 `os.TempDir()/wikimind-<base>.sock`），或保持 named pipe 但 Listen 用 `winio.ListenPipe`（依赖项需评估）。
  2. 统一 `Listen` 路径与 `Dial` 路径，避免协议错位。
  3. 加 Windows CI smoke：在 GitHub Actions matrix 里加 windows-latest job 跑 bridge unit test。
- **估算**：~20 行代码 + 1 个 CI matrix entry。

#### F-042 · Watcher Close 存在 send-on-closed-channel race

- **位置**：`internal/watcher/watcher.go`（Close + time.AfterFunc 回调）。
- **现状**：Close 之后 AfterFunc 仍可能触发回调向已关闭 channel send，daemon Ctrl-C 时窗 panic 风险。
- **修复方向**：
  1. `Watcher` 加 `closed atomic.Bool`，AfterFunc 回调首行 `if w.closed.Load() { return }`。
  2. 或者 `Close()` 改为先停 AfterFunc 再关 channel。
- **估算**：~15 行代码 + 1 个 race 单测（模拟 close 时窗）。

### 🟡 协议契约 / 行为修正（中优先）

#### F-027 · MCP 写入 change-log 时 `append_log` 应为 `log_append`

- **位置**：
  - `internal/mcp/tools.go:492` —— `commit.CommitWithActor(ctx, b.root, sess.Agent, "append_log", summary, nil)`
  - `internal/mcp/tools_test.go:605` —— 断言 `entry.Op != "append_log"`
- **现状**：spec 多处用 `log_append`，代码写成 `append_log`；未来 dream-cycle / revert 按 spec 查会失败。
- **修复方向**：直接把代码两处字面量改成 `log_append`（spec 是权威）。dogfood 阶段历史 commit 极少，不做 migration。
- **估算**：2 行代码 + 同步改测试断言；如有 historical change-log fixture 一并更新。

#### F-009 · `claim_sources.go` 暴露 `sql.ErrNoRows`

- **位置**：`internal/index/claim_sources.go`（`FindXxx` 返回 `(nil, sql.ErrNoRows)`）。
- **现状**：违反 error-handling spec "未找到行返回包级 sentinel，不暴露 sql.ErrNoRows"。
- **修复方向**：定义包级 `ErrClaimSourceNotFound`，`Scan` 命中 `sql.ErrNoRows` 时转译；service 层用 `errors.Is`。
- **估算**：< 10 行代码 + 1 个 errors.Is 单测。

#### F-010 · LIKE 模式拼接未转义 `idempotency_key`

- **位置**：`internal/index/<file>`（`FindReviewByIdempotencyKey` 类）—— 待补 line。
- **现状**：用户提供的 `idempotency_key` 含 `%` / `_` / `\` 时破坏幂等契约（可命中非目标行 / 错过自己的行）。CWE-89 风格。
- **修复方向**：
  1. 加 `escapeLikePattern(s string) string` helper：`replace %, _, \` 为 `\%, \_, \\`。
  2. SQL 用 `LIKE ? ESCAPE '\\'`。
  3. 单测覆盖 3 个元字符。
- **估算**：< 15 行代码 + 1 个 helper 单测 + 1 个端到端单测。

#### F-012 · `CheckQueueForPropose` fail-open 静默吞错

- **位置**：`internal/service/queue.go`（`CheckQueueForPropose`）。
- **现状**：DB 查询出错时返回"队列未满"，绕过 propose 闸门 → agent 可在队列实际已满时仍 propose。
- **修复方向**：把 fail-open 改成 fail-closed（DB 错误返回 wrap 后的 error；MCP 工具入口 surface 为 `QUEUE_CHECK_FAILED` 而非静默放行）。
- **估算**：< 10 行代码 + 1 个 DB-error 单测。

#### F-025 · MCP 工具内 inline `errors.New("CROSS_SESSION_BUNDLE")` 应为包级 sentinel

- **位置**：`internal/mcp/tools.go:433, 439` 等（grep `errors.New("[A-Z_]"`）。
- **现状**：inline 创建 error 无法被 caller `errors.Is` 比对，破坏协议错误码可比性。
- **修复方向**：把 inline `errors.New(...)` 提取到 `tools.go` 顶部包级 `var ErrCrossSessionBundle = errors.New("CROSS_SESSION_BUNDLE")` 等；调用点改用 sentinel。
- **估算**：~15 行（提取 4-5 个 sentinel）。

### 🟢 并发 / 资源管理（daemon 长跑必修）

#### F-028 · `vaultBackend.lockManager()` 懒初始化非线程安全

- **位置**：`internal/mcp/session.go` 或 `tools.go`（`vaultBackend.lockManager()`）。
- **现状**：首次访问时多 goroutine 可能同时初始化 → race。
- **修复方向**：用 `sync.Once` 包裹，或在 `vaultBackend` 构造时 eager init。
- **估算**：< 10 行代码 + 1 个 race 单测（`go test -race` 触发）。

#### F-029 · Session 过期未清理 worktree（资源泄漏）

- **位置**：`internal/mcp/session.go`（session 过期路径）。
- **现状**：session 被标记过期但 git worktree 留在磁盘 → daemon 长跑下 worktree 无限累积。
- **修复方向**：`Expire(id)` 内追加 `worktree.RemoveWorktree(ctx, vaultRoot, agent, sessionID)`；删 worktree 失败时 log 但不阻塞 Expire。
- **估算**：~20 行代码 + 1 个验证 worktree 被删的单测。

#### F-030 · `SessionStore.Expire` 在生产代码中从未被调用

- **位置**：`internal/mcp/session.go`（`Expire` 方法定义）+ `internal/daemon/loop.go`（daemon 主循环）。
- **现状**：`Expire` 是 dead code；session 即使过期也不会被清理。
- **修复方向**：daemon 启动注册 ticker（默认 5min），周期遍历 SessionStore 调 `Expire` 清理过期 session。
- **估算**：~15 行 daemon 改动 + 1 个 ticker 周期单测。

### 🔵 CLI 契约字面量动态化

#### F-048 · `command.go:108` 硬编码 "15 tools"

- **位置**：`cmd/wikimind/command.go:108`（mcp serve 输出 banner）。
- **现状**：banner 写"Registered 15 tools"，实际 17 个；用户感官与 spec 都不准。
- **修复方向**：
  1. 在 `internal/mcp/server.go` 加 `ToolCount() int`（返回当前注册数）。
  2. `command.go:108` 改成 `fmt.Sprintf("Registered %d tools", srv.ToolCount())`。
- **估算**：API 5 行 + caller 2 行 + 1 个单测。

#### F-049 · `command.go:821` 硬编码 "(8 rules)"

- **位置**：`cmd/wikimind/command.go:821`（lint 子命令输出）。
- **现状**：lint banner 写"(8 rules)"，实际 5 条；类同 F-048。
- **修复方向**：
  1. 类似 F-048：`internal/lint` 加 `RuleCount()` 或 `RuleNames() []string`。
  2. command.go 动态查询。
- **估算**：~5 行（API + caller）。

### 🟣 用户契约期望管理

#### F-003 + F-069 · Python worker 仍是 W0 skeleton 但 doctor 假装 ready

- **位置**：
  - `worker/main.py:1-37`（顶部 docstring 自承"W0 skeleton"）
  - `worker/pyproject.toml`（description / dependencies）
  - `cmd/wikimind/command.go:689`（doctor 检查 pypdf 并标 ✓）
- **现状**：doctor 输出"✓ pypdf"暗示系统能解析 PDF，实际 worker 只回 skeleton 事件 + Go 侧无人调 worker.py。
- **修复方向**：
  1. `worker/main.py` 顶部 docstring 改成"WikiMind ingest worker — W0 skeleton (PDF/image parsing deferred to v0.2)"。
  2. `worker/pyproject.toml` description 写 "skeleton"，`dependencies = []` 显式（不列 pypdf 假承诺）。
  3. `cmd/wikimind/command.go:689` 把 pypdf 检查改 warning：`⚠ pypdf: optional, not used by v0.1.x worker`，或直接删 pypdf 检查。
- **估算**：< 15 行（3 文件各几行）。

### ⚙️ CI 增强（来自 F-075 follow-up）

- **govulncheck** 加 GitHub Actions 步骤：`govulncheck ./...`，作为 CI gate（非阻塞 warning → 后续可升 hard fail）。
- **估算**：1 个 CI step yaml。

## Requirements

- 13 条 finding 全部修完（不删减、不"看情况"）。
- 每条修复有最小回归测试触达原 finding 路径。
- 不引入新生产依赖；govulncheck 是 dev 依赖。
- 不破坏现有 `go test ./...` / `go vet ./...` / `go build ./...`。
- 保持 v0.1.0 既有 CLI / MCP / change-log 行为（F-027 例外，spec 是权威）。

## Acceptance Criteria

- [ ] 13 条 finding 每条都有对应 commit / 代码改动 + 至少 1 个回归测试。
- [ ] `go test ./... -race` 全绿（覆盖 F-028 / F-042 的并发场景）。
- [ ] `go vet ./...` / `go build ./...` 无 warning。
- [ ] Windows CI smoke 在 GitHub Actions matrix 里加入并通过（F-041 配套）。
- [ ] govulncheck 在 CI 里跑出"no known vulnerabilities"（或仅 warning 列表）。
- [ ] `wikimind doctor` 输出对 pypdf 不再误导（F-003+F-069）。
- [ ] `wikimind mcp serve` banner / `lint` banner 不再硬编码数字（F-048 / F-049）。
- [ ] 全部改动总行数控制在 ~150-180 行代码（不含 CI yaml）。

## Definition of Done

- 全部 AC 勾选。
- 单独 1 个或多个 commit，commit message 引用 finding ID。
- 任务归档；归档 commit 由 `task.py archive` 自动产生。
- 不开衍生子任务；剩余 P1/P2 推 v0.2。

## Out of Scope (explicit)

- ❌ Spec-Drift 修复（13 条 `F-S001/F-S002/F-014/F-018/F-025/F-026/F-027/F-031/F-034/F-045/F-048/F-049/F-056`）—— 走独立 `trellis-update-spec`。
- ❌ v0.2 结构性重构（F-002 / F-005 / F-006 / F-008 / F-013 / F-031 / F-038 / F-039）。
- ❌ 23 条 P2 整理（命名 / 重复代码 / 错误处理统一化）—— v0.2 重构期顺手清。
- ❌ Python worker 的真实 PDF 解析（推 v0.2 milestone 或 deprecate）。
- ❌ govulncheck 升 hard fail（v0.1.1 仅 warning gate；v0.2 升 hard）。
- ❌ 修改 `.trellis/spec/` 任何文件（spec 改动走 trellis-update-spec）。

## Technical Notes

- 审查报告原文：`.trellis/tasks/archive/2026-05/05-27-code-quality-audit-v0-1-0/audit-report.md`。
- 审查标尺（patch 期间仍生效）：`.trellis/spec/backend/{quality,directory,database,error,logging}-guidelines.md` 5 份 + 7 个 `<spec-entry>` 场景契约。
- 工具链：`go test ./...` / `go test -race` / `go vet ./...` / `go build ./...` / `govulncheck ./...`（新增）。
- v0.1.0 baseline commit：`5d88ffc`（审查报告归档）/ `048c8ec`（daemon log 文件句柄 fix）。

## Research References

(本任务 finding 来源全部在审查报告内，无需外部技术调研。)
