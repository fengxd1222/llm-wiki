# W4 D26: 完善 templates + revert-cascade

## Goal

完善所有 agent templates (AGENTS / CLAUDE / CODEX / HERMES / CURSOR /
page-schemas) + `wikimind revert-cascade` 命令（依赖图反向影响分析）。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D26
- `spec-v2/docs/failure-playbook.md §3`

## Requirements

### A. Templates

`templates/` 完善（D8 init 已建骨架）：
- `AGENTS.md` —— 通用 agent 行为指南
- `CLAUDE.md` —— Claude Code 专属（D16 已加 claim 抽取算法 §6）
- `CODEX.md` —— Codex CLI 专属
- `HERMES.md` —— Hermes 专属
- `CURSOR.md` —— Cursor 专属
- `page-schemas/`:
  - `claim.yaml` —— frontmatter schema
  - `entity.yaml`
  - `concept.yaml`
  - `source.yaml`
  - `topic.yaml`

`wikimind init` 写入新 vault；`wikimind templates upgrade` 升级既有 vault
templates 到最新版本（不覆盖 user customization）。

### B. `wikimind revert-cascade <seq>` (failure-playbook §3)

D6 已有 `wikimind revert <seq>` 单 commit。D26 加 cascade：
- 找 commit 影响的 pages
- 找 backlinks (D14 page_links 表)
- 找 derived claims (claim_sources 引用)
- 反向遍历依赖图，提示 user 哪些 page / claim 受影响
- 选项 1：revert 整个影响子集
- 选项 2：revert 单 commit + 提示 user 手动修受影响项

CLI: `wikimind revert-cascade 42 [--apply] [--dry-run]`

### C. 测试

- templates upgrade 不覆盖 customization
- revert-cascade 依赖图正确（5+ test scenarios）

目标 ≥ 575（D25 后 555 + 20）。

## Acceptance Criteria

- [ ] 6 templates + 5 page-schemas
- [ ] `templates upgrade` 不丢 customization
- [ ] revert-cascade 依赖图正确
- [ ] CI 5 OS 全绿；测试 ≥ 575

## Out of Scope

- 通过 GUI 编辑 templates
- LLM 主动改 template

## Decision (ADR-lite)

**Decision**：templates embed in binary + `wikimind templates upgrade` 写文件。
Customization 检测：file hash 与 embed 版本不同 → 不动 + 建议 user 手动 diff。

## Technical Notes

- go:embed templates/* 编进 binary
- revert-cascade 用递归 BFS 遍历 page_links + claim_sources
