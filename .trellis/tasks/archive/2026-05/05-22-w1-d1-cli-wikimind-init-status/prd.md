# W1 D1: CLI 框架与 wikimind init/status

## Goal

搭起 `wikimind` CLI 的 cobra 骨架，实现 `init` 与 `status` 两个命令——让用户能创建一个
结构合规的 vault 并查看其状态。这是 30 天 MVP roadmap W1 的第一天，是后续所有 CLI
命令的地基。

需求来源：`spec-v2/docs/roadmap-30d.md` 的 W1 D1 + `spec-v2/docs/engineering-decisions.md`
§4-5（Go 模块划分、技术选型）。需求已规划，brainstorm 仅确认少数 MVP 行为细节。

## What I already know

- 技术选型已锁定（engineering-decisions §4.3）：CLI 框架 `spf13/cobra`，git 操作直接 exec
- Go 模块骨架已就位（W0）：`cmd/wikimind/main.go` 当前是占位 stub，`internal/vault`
  `internal/schema` 等 14 个包已建（仅 `doc.go`）
- vault 三层结构定义见 `spec-v2/docs/architecture.md §4.1`
- schema 模板已存在：`spec-v2/templates/` 的 7 个文件（AGENTS / CLAUDE / CODEX /
  HERMES / CURSOR / page-schemas / lint-rules）
- `wikimind init` 应自动 git init（`spec-v2/docs/cross-platform.md §7.1`）
- `go build ./...` + CI 当前全绿

## Requirements

- **cobra CLI 框架**：`wikimind` 根命令 + 7 个子命令（init / status / ingest / query /
  review / lint / revert）。D1 只实现 `init`、`status`；其余为 stub（打印"D1 未实现"并退出）。
- **`wikimind init <vault>`**：
  - 创建三层目录：`raw/{inbox,imported,attachments,manifests}`、
    `wiki/{claims,entities,concepts,sources,topics,_review,_reports}`、`schema/`、
    `.wikimind/{audit,locks}`
  - 写入默认 schema 文件 —— 用 `go:embed` 嵌入 `spec-v2/templates/` 的 7 个文件，
    init 时落到新 vault 的 `schema/`
  - 生成 `.wikimind/config.toml`（最小字段：vault root、schema_version、created_at）
  - 生成初始 `wiki/index.md` 与 `wiki/log.md`
  - 自动 `git init`（若 vault 不在已有 git 仓库内）
  - 目标目录处理：不存在则创建；已存在且为空则使用；**已存在且非空则拒绝报错**
- **`wikimind status`**：输出 vault 路径、schema_version、文件计数（raw / wiki pages /
  claims）、git 状态（branch、clean/dirty）
- **涉及包**：`cmd/wikimind`（cobra 命令）、`internal/vault`（目录创建）、
  `internal/schema`（模板嵌入与写入）

## Acceptance Criteria

- [ ] `wikimind init <dir>` 创建完整三层目录结构
- [ ] init 后 `schema/` 含 7 个模板文件，内容与 `spec-v2/templates/` 一致
- [ ] init 后 `.wikimind/config.toml` 存在且为合法 TOML
- [ ] init 后 vault 是一个 git 仓库
- [ ] init 到已存在的非空目录时拒绝并报清晰错误
- [ ] init 后 `wiki/index.md`、`wiki/log.md` 存在
- [ ] `wikimind status`（vault 内运行）输出正确元信息
- [ ] 5 个未实现子命令的 stub 能运行并给出清晰提示
- [ ] 单测覆盖 init / status 核心路径
- [ ] `go build ./...` + `go vet ./...` + `go test ./...` 全绿

## Definition of Done

- 测试添加（init / status 的单测）
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/` 的 error-handling / logging / quality 规范
- commit 并 push（W1 D1 一个 commit）

## Out of Scope

- ingest / query / review / lint / revert 的实现（D1 仅 stub）
- 跨平台路径规范化的完整实现 + 100 路径单测（D2）
- SQLite schema / 索引 / FTS5（D3）
- daemon（`cmd/wikimindd` 本次不动）
- 安装包 / 服务注册（D7+ / W4）

## Decision (ADR-lite)

**Context**: `wikimind init` 可能指向一个已有内容的目录，需定行为。
**Decision**: 目标目录已存在且非空 → 直接拒绝报错（要求路径不存在、或存在但为空）。
**Consequences**: D1 实现最简、最安全；"在非空目录强制铺结构"的 `--force` 留作后续按需添加。

## Technical Notes

- schema 模板嵌入：与 engineering-decisions §3.2 的 migrations `//go:embed` 同手法
- git 状态获取：`exec.Command("git", ...)`，不引入 go-git（engineering-decisions §4.4）
- config.toml：W0 已确认无需第三方 TOML 写库即可手写最小文件；如需读，后续 D2 引入
