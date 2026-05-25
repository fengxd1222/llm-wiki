# W3 D21: W3 出口 demo + Codex/Claude 并发剧本

## Goal

W3 出口验收：Claude Code（ingest 文章）+ Codex CLI（跑 lint 命中 broken_link）
同时工作 → user accept 部分 → git log + change log 一一对应。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D21
- `spec-v2/docs/conflict-scenarios.md` 全部剧本

## Requirements

### A. Demo flow

`docs/demo/w3-walkthrough.md`：

1. 启 wikimindd daemon
2. 终端 1：Claude Desktop 接入 → ingest 1 篇论文 → propose 3 claim
3. 终端 2：Codex CLI 接入 → lint_run → 发现 broken_link → propose_edit 修
4. user `wikimind review today` 看 4 个 pending
5. user accept 3 个，reject 1 个 (reason: "wrong claim")
6. Codex 收到 rejection memory 通知 (next handshake)
7. 验证：git log 5 commit (1 ingest + 3 accept + 1 reject 不创 commit) +
   change-log.jsonl 4 行
8. CJK 子串 search 命中（中文论文测试）

### B. CI 端到端 smoke

`cmd/wikimind/w3_exit_test.go`：
- 上面 7 步 mock 跑（mock 2 agent 通过 service.AcceptReview 直接调）
- 跨 5 OS CI

### C. W3 出口指标

- 两 agent 同时跑无冲突（lock + reviews superseded 触发正确）
- lint 全套 8 规则在测试 vault 无 false positive
- review queue 上限保护触发：mock 51 propose → 第 51 个 backlog

### D. 测试

W3 e2e 8 步 + 出口 3 指标 + edge cases。目标 ≥ 475（D20 后 450 + 25）。

## Acceptance Criteria

- [ ] docs/demo/w3-walkthrough.md 完整
- [ ] CI w3_exit_test 5 OS 全绿
- [ ] 3 出口指标达成
- [ ] CI 5 OS 全绿；测试 ≥ 475
- [ ] **W3 全部 7 天完成 (D15-D21)**

## Out of Scope

- 真 Claude Desktop 自动化（manual walkthrough）
- 性能压测（W4 D29）

## Decision (ADR-lite)

**Decision**：CI 用 mock agent 走 service layer 避开 MCP stdio 自动化复杂度。
Manual walkthrough doc 是 user 实际跑的 reference。

## Technical Notes

- mock agent helper：`testdata/mock_agent.go` 复用 D11/D12 测试 helper
- 出口指标自动收集到 `.wikimind/audit/w3-metrics.json`
