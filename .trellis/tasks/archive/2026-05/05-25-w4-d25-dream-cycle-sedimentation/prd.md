# W4 D25: Dream Cycle 基础版 + Query Sedimentation 基础版

## Goal

实现两个"主动智能"功能基础版：
- **Dream Cycle**：daemon 闲时跑 audit + report（consolidate / evolve 留 v0.2）
- **Query Sedimentation**：user query 评分 + topic 沉淀（命中 N 次的 query
  自动升 topic 进 review queue）

需求来源：
- `spec-v2/docs/dream-cycle.md`
- `spec-v2/docs/query-sedimentation.md`
- `spec-v2/docs/roadmap-30d.md` W4 D25

## Requirements

### A. Dream Cycle audit + report

`internal/dream/` 新包：
- `audit.go`：扫全 vault → 统计 health metrics（pages count by status / age
  distribution / drift count / orphan count）
- `report.go`：生成 `.wikimind/dream-reports/YYYY-MM-DD.md` 含 audit 结果 +
  3 suggested actions
- daemon 闲时（last_user_action > 30 min）触发；W4 D25 简化 = 每天 03:00 cron

CLI: `wikimind dream run` 手动触发

### B. Query Sedimentation

`internal/sediment/` 新包：
- `score.go`：记 user query 到 `.wikimind/queries.jsonl`（每次 query 1 行）
- `topic.go`：每周扫 queries.jsonl，cluster 相似 query（简单：normalize +
  hash），命中 ≥ 5 次的 query → propose topic page (wiki/topics/<slug>.md)
  通过 review queue（user accept 才入 main）

CLI: `wikimind sediment scan`（manual 触发）

### C. wiki/topics/ 目录

`wikimind init` 已建 wiki/{claims,entities,concepts,sources}/；D25 加 topics/。
D11 propose_page type='topic' 路径校验加入。

### D. 测试

- Audit metrics 准确
- Report Markdown 渲染
- Sediment cluster 相似 query
- topic propose 通过 review queue
- daemon 闲时触发 mock

目标 ≥ 555（D24 后 530 + 25）。

## Acceptance Criteria

- [ ] `wikimind dream run` 生成 report
- [ ] `wikimind sediment scan` 生成 topic propose
- [ ] daemon 闲时 cron triggers
- [ ] wiki/topics/ 支持
- [ ] CI 5 OS 全绿；测试 ≥ 555

## Out of Scope

- Dream Cycle consolidate / evolve（v0.2）
- Embedding-based clustering（v0.2）
- 主动 chat agent ("research mode")

## Decision (ADR-lite)

**Decision**：D25 dream + sediment 是 daemon 后台 worker 的雏形；输出都
通过 review queue（不直绕 single-writer）。W5+ 加 LLM consolidation。

## Technical Notes

- queries.jsonl 每行 `{ts, query, source_agent, result_count}`
- topic slug from query normalize（lower + strip stopword）
- audit cron 用 `github.com/robfig/cron/v3` 或 daemon 主循环自实现
