# W3 D17: Lint 全套 8 规则 + incremental + CLI

## Goal

实现 `wikimind lint` 全套 8 规则 + 增量 (`--since`)，让 user 一键定位
vault 健康问题。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D17
- `spec-v2/docs/mcp-tools.md §20` lint_run schema

## 8 规则

| Rule | 检查 | Severity |
|---|---|---|
| `orphan` | page 无 inbound link 也无 outbound link | warn |
| `broken_link` | `[[xxx]]` 指向不存在 page | error |
| `contradictions` | 两个 claim 互相矛盾（confidence + status 检测） | warn |
| `stale` | page updated_at > 90 天 + 无引用更新 | info |
| `unverified_claim` | claim status='unverified' > 7 天 | warn |
| `duplicate_entity` | 两个 entity 同 name 或同 alias | warn |
| `schema_violation` | frontmatter 缺必填字段 / 类型错 | error |
| `missing_index_entry` | page 不在 wiki/index.md (D14 自动维护后这条该没了，仍跑兜底) | info |

## Requirements

### A. `internal/lint/` 新包

- `rules.go`：`Rule interface { Name() string; Run(ctx, vault, db, scope) []Finding }`
- `findings.go`：`Finding struct { Rule, PageID, Severity, Detail, SuggestedAction }`
- 8 个 rule 实现各一文件：`orphan.go / broken_link.go / contradictions.go ...`
- `runner.go`：`RunRules(ctx, vault, db, rules []Rule, scope LintScope) ([]Finding, Summary, error)`
- `cache.go`：incremental 用 `last_lint_at` (meta 表) + git diff since

### B. CLI `wikimind lint`

- flags: `--rules <name,name,...>` (default all) / `--scope <glob>` / `--since <date>` / `--json`
- 输出表格：rule / page / severity / detail / suggested
- exit code: errors > 0 → 1; warnings only → 0
- `wikimind lint --auto-fix` 部分规则可自动 propose 修复（broken_link 创建空 page / missing_index_entry 加行）

### C. MCP `lint_run` (mcp-tools.md §20)

`internal/mcp/tools.go` 加 handleLintRun（已在 D8 总览，D17 真实现）。
ReadOnlyHint=true。总 tool 数：17 → 18。

### D. 测试

每个 rule 至少 3 测试（hit / no-hit / edge），= 24 + runner 8 + CLI 5 = 37 新测试。

目标 ≥ 360（D16 后 320 + 40）。

## Acceptance Criteria

- [ ] 8 rule 全实现 + 单测
- [ ] `wikimind lint` CLI + `--json` / `--scope` / `--since`
- [ ] `lint_run` MCP tool（总 18 tool）
- [ ] incremental scan：last_lint_at + 仅扫 changed
- [ ] CI 5 OS 全绿；测试 ≥ 360

## Out of Scope

- 自动修复所有 rule（仅 broken_link + missing_index_entry MVP）
- 大 vault 性能优化（10k pages，W4+）
- Custom rule plugin（W4+）

## Decision (ADR-lite)

**Decision**：rule pluggable interface + 内置 8 rule + scope/since filter。
auto-fix 通过 propose_* 走 review queue（不直接改 main，保持 single-writer）。

## Technical Notes

- broken_link 用 D14 page_links 表 LEFT JOIN pages 找 NULL
- orphan 同表 NOT EXISTS 两向
- contradictions 简单版：同 entity 的 claim 含 ✓ vs ✗ 关键词
- last_lint_at 存 `meta` 表（D3 已建）
