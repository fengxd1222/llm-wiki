# Code Quality Audit v0.1.0

## Goal

对 v0.1.0 已发布代码（D1–D30 全量交付物）做一次**只读**全量深度审查，按 10 个 lens 全维度扫描，
产出单文件 audit 报告 + 三档严重度分级（P0/P1/P2），为后续决策（v0.1.1 patch / v0.2 / 现状冻结）提供依据。

**全程只读**——不修改任何源代码；P0 级问题如有发现，仅高亮在报告顶部"紧急警报"区，不中断审查流程。
所有修复由本任务结束后衍生的独立任务承担。

## What I already know

- v0.1.0 已发布，最后归档任务 `05-25-w4-d30-v010-release-retro`。
- 代码体量：Go 在 `cmd/{wikimind,wikimindd}` + `internal/{17 包}` + `verify/{fts5,ipc,mcp}`，Python worker 在 `worker/`。
- 审查标尺：`.trellis/spec/backend/{quality,directory,database,error,logging}-guidelines.md`（1359 行 + 7 个 `<spec-entry>` 场景契约）。
- 测试基础：`go test ./...` / `go vet ./...` / `verify/` 端到端 / `cmd/wikimind/*_test.go` E2E。
- 最近修复 `55fc534`（vault.Init 跨平台 default branch 一致）—— 提示跨平台层有遗留风险。
- 工作目录 19 个未提交变更，不在审查范围。

## Requirements

### 范围（已锁定：全量深度审查）

- ✅ `cmd/wikimind/`（CLI 主入口 + 所有子命令）
- ✅ `cmd/wikimindd/`（daemon 入口）
- ✅ `internal/` 全部 17 个包：`vault` / `index` / `mcp` / `service` / `commit` / `proposal` / `worktree` / `lock` / `lint` / `watcher` / `bridge` / `daemon` / `changelog` / `git` / `schema` / `model` / `worker`
- ✅ `verify/{fts5,ipc,mcp}` 三套集成测试本身的质量
- ✅ `worker/`（Python，用 Python 标尺）
- ✅ `go.mod` / `go.sum`（依赖安全扫描）

### 审查维度（已锁定：10 个 lens 全套餐）

| # | Lens | 重点 |
|---|------|------|
| 1 | 正确性 / 逻辑 | 边界条件、状态机错位、空值、未覆盖分支 |
| 2 | 错误处理 | sentinel + `%w` wrap、`errors.Is/As`、漏 wrap、吞错 |
| 3 | 安全 | path traversal、SQL 拼接、命令注入、敏感信息泄漏到 log/commit |
| 4 | 并发 | goroutine 泄漏、mutex 误用、context 漏传、race |
| 5 | 性能 | 明显的 O(n²)、热路径 IO、内存复制反模式 |
| 6 | 契约稳定性 | 导出 API、MCP 错误码字面量、CLI 输出格式、change-log 字段 |
| 7 | 测试质量 | 违反"不 mock fs/git/SQLite"约定、覆盖盲点、跨平台缺失 |
| 8 | 资源管理 | 文件句柄、DB 连接、git worktree、临时目录清理 |
| 9 | 跨平台 | 路径分隔符、行尾、`git init` 默认分支、文件权限位 |
| 10 | 可维护性 | 函数过长（> 80 行）、复杂度、命名、重复代码 |

Python worker 用 Python 标尺：PEP 8 / type hints / 异常处理 / 依赖安全。

### 加项（已锁定 4/4）

- ✅ **Python worker 同步审**：`worker/main.py` + `worker/pyproject.toml`，用 Python 思维（不套 Go sentinel error）。
- ✅ **依赖安全扫描**：`go.mod` / `go.sum` 用 `govulncheck`（如本机已装）或手工核对版本+CVE 公告；不安装新依赖到项目。
- ✅ **Spec 反向校准**：发现"代码 vs spec 不一致 但代码更优/spec 过时"时，问题打 `Spec-Drift` 标签，独立列出，供后续 trellis-update-spec。
- ✅ **P0 不中断 + 顶部高亮**：审查中遇到 P0 级问题不停，继续完成全貌；仅在报告顶部"🚨 紧急警报"区提前列出。

### 严重度分级（已锁定：三档）

| 档位 | 定义 | 处置建议 |
|------|------|---------|
| **P0** | 数据损坏 / 安全漏洞 / 跨平台彻底不可用 / 协议契约破坏 | 必须 v0.1.1 patch |
| **P1** | 行为偏离 spec、可观察 bug 但不致命；性能/可维护性显著问题 | 可延 v0.2 |
| **P2** | 风格、命名、注释、轻微重复；FYI 不必修 | 不必修 |

报告顶部需有"📊 严重度统计表" + "🎯 推荐结论"（三选一：v0.1.1 patch / v0.2 / 现状冻结）。

## Acceptance Criteria

- [ ] `audit-report.md` 落在 `.trellis/tasks/05-27-code-quality-audit-v0-1-0/audit-report.md`，单文件。
- [ ] 报告顶部依次：🚨 紧急警报（P0 列表）→ 📊 严重度统计表 → 🎯 推荐结论。
- [ ] 报告主体按"严重度主排序，维度为次排序"组织 findings。
- [ ] 每个 finding 含：`file:line` 锚点、所属 lens、严重度、可复现描述、证据片段、修复方向（不写代码）。
- [ ] 至少抽查 3 个 P0/P1 finding，验证 `file:line` 锚点可被 `grep -n` 命中复现。
- [ ] 报告末尾列出"Spec-Drift"段，单独汇总代码与 spec 描述不一致但代码更优的条目。
- [ ] 全部 17 个 internal 包、cmd、verify、worker、go.mod 都在报告中至少出现一次（覆盖证明，可写"无 finding"）。
- [ ] 任务执行期间未修改任何源文件（`git status -- internal/ cmd/ verify/ worker/ go.mod go.sum` 必须无变化）。

## Definition of Done

- 上述 8 个 AC 全部勾选。
- 审查报告以 commit 落盘（`docs(audit): v0.1.0 code quality audit report` 或类似）。
- `task.py finish` + `archive` 完成；报告随归档目录保留。
- 不创建衍生修复任务（用户审完报告后自己决定下一步）。

## Technical Approach

### 工作流（5 阶段）

1. **预扫阶段**：跑 `go build ./... && go test ./... && go vet ./...`，记录工具层的初判（编译/测试/vet 输出 = 0 类 finding）。
2. **包级深扫**：按依赖底层→上层顺序逐包：`vault` → `commit` → `index` → `service` → `proposal` → `worktree` → `mcp` → `lock` / `lint` / `watcher` / `bridge` / `daemon` / `changelog` / `git` / `schema` / `model` / `worker`（Go）→ `cmd/*` → `verify/*`。
3. **横切扫描**：跨包维度（安全 path/SQL、并发 goroutine、跨平台路径、契约字面量）用 grep / find 全仓扫描。
4. **Python worker + 依赖**：`worker/` PEP 8 + 安全；`go.mod`/`go.sum` 依赖 CVE 核对（有 govulncheck 则用，否则手工）。
5. **综合**：合并、去重、分级、写报告。

### 工具使用

- 主要工具：`Read` + `Grep` + `Bash`（运行 `go vet` / `go test` / 文件统计）。
- 不引入新依赖到项目；本机若装了 `govulncheck` / `staticcheck` 可临时用一次但不写入 go.sum。
- 不调用网络（CVE 公告类信息可在依赖版本号确定后由用户后续检索）。

### 比对锚点

每条 finding 都需要对照下面之一作为"为什么是问题"的依据：
- `.trellis/spec/backend/*.md` 某一条款（引用 `file#section`）
- `quality-guidelines.md` 内 7 个 `<spec-entry>` 之一
- 一般 Go/Python 行业共识（PEP 8 / Effective Go / CWE）

## Decision (ADR-lite)

**Context**: v0.1.0 已发布但有未提交变更与一次紧急跨平台 fix，需要在开 v0.2 前对全量代码做一次系统性体检。

**Decision**:

- 走**全量深度 + 10 维度 + 单文件报告 + 三档分级**组合。
- 全程只读，发现的问题不立即修；P0 不中断审查。
- 扩展 4 个加项：Python worker、依赖扫描、Spec-Drift、P0 顶部高亮。

**Consequences**:

- 工作量大（17 包 × 10 维度），预计 1 个长会话或拆 2-3 个执行窗口完成。
- 报告体量可能 800-1500 行单文件；接受这个体量换"一处一眼看完"。
- 不自动拆修复子任务——决策权留给用户，避免审查没读完就被一堆任务淹没。

## Out of Scope (explicit)

- ❌ 修改源代码（任何 bug fix 都走独立任务）。
- ❌ 引入新 lint / 静态分析工具作为项目依赖（临时本机调用允许）。
- ❌ 性能压测 / 跑 benchmark（仅做反模式扫描）。
- ❌ 审查 `spec-v2/`、`prototypes/`、`archive/`、`docs/`、`README.md`。
- ❌ 修改 `.trellis/spec/`（Spec-Drift 仅记录不修）。
- ❌ 自动 `task.py create` 修复子任务（用户审完报告后自决）。
- ❌ 调用网络（CVE 数据库联网检索）—— 离线核对版本即可。

## Technical Notes

- 审查标尺主文件：`.trellis/spec/backend/{quality,directory,database,error,logging}-guidelines.md`。
- 必须比对的 7 个 `<spec-entry>` 契约场景：W1 D1 CLI / W1 D6 git log / W2 D9 MCP read / W2 D10 handshake+worktree / W2 D11 MCP propose / 跨平台 git init。
- 工具链：`go test ./... -race` / `go vet ./...` / `go build ./...` / `grep` / `find` / 手动 Read。
- 不可越界目录：`internal/` / `cmd/` / `verify/` / `worker/` / `go.mod` / `go.sum`（只读）。
- 报告路径：`.trellis/tasks/05-27-code-quality-audit-v0-1-0/audit-report.md`。

## Research References

(本任务为只读审查，无需外部技术调研；审查标尺就在 `.trellis/spec/backend/`。)
