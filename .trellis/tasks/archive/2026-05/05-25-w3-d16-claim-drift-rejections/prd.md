# W3 D16: Claim quote_hash 反验证 + DRIFT 检测 + rejections memory

## Goal

Claim 的 source quote_hash 在每次 read_claim / search 时反向校验：raw 文件
若被改动 (mtime / sha256 变) → 标 DRIFT。同时建 `rejections.jsonl` 记 user
reject 历史给后续 agent 当避坑。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D16
- `spec-v2/docs/agent-protocol.md §9` rejections memory
- `spec-v2/docs/claim-extraction.md §6` claim 抽取算法

## What I already know

- D9 anchor.QuoteHash + ResolveAnchor 已实现
- D11 propose_claim ValidateClaimSources 在 propose 时验证；D16 加 read 时
  反验证
- D11 D9 read_claim 当前 sources 字段 staged 返空——D16 真填 drift_status

## Requirements

### A. `claim_sources` 表（migration 0005）

```sql
CREATE TABLE IF NOT EXISTS claim_sources (
    claim_id          TEXT NOT NULL,
    raw_id            TEXT NOT NULL,
    anchor            TEXT NOT NULL,
    stored_quote_hash TEXT NOT NULL,
    quote_preview     TEXT NOT NULL,  -- 前 200 chars 给 user 看
    span_start        INTEGER,
    span_end          INTEGER,
    PRIMARY KEY (claim_id, raw_id, anchor)
);
CREATE INDEX idx_claim_sources_raw ON claim_sources(raw_id);
```

D11 propose_claim 通过 review accept 时（D12 service.AcceptReview）→
INSERT INTO claim_sources rows from frontmatter。

### B. DRIFT 检测

`internal/service/drift.go` 新：
- `VerifyClaimSource(ctx, vault, row ClaimSourceRow) (status string, err error)`
  - 重算 quote_hash → 比对 stored
  - "verified" / "drift" / "anchor_missing" / "raw_missing"
- `ScanAllClaims(ctx, db, vault) (driftCount int, err error)` —— 跑全 vault

D9 read_claim 真填 sources 字段（call VerifyClaimSource per source）。
D14 wiki_info.health.drift_claims_count 真取数。

### C. propose_claim 严格化（已部分 D11 做）

- 严格拒绝 provenance_depth > 1（D11 已做）
- 严格拒绝无 source claim（除非 speculation: true）（D11 已做）
- D16 加：claim 必含 `speculation: false` 时校验所有 sources verify pass

### D. Rejections memory

`internal/service/rejections.go`：
- `RecordRejection(ctx, vault, ReviewID, Agent, Page, Reason) error` —— append
  to `.wikimind/rejections.jsonl`
- `LoadRecentRejections(ctx, vault, limit int) ([]Rejection, error)` —— 倒序读

D12 review reject CLI 加：reject 时调用 RecordRejection。

D10 agent_handshake response 加 `recent_rejections` 字段（最近 10 条），让
agent 知道 user 拒过什么。

### E. claim 抽取 templates

`templates/CLAUDE.md`（如果 D8 W2 init 阶段已建则改）补 claim 抽取算法
（claim-extraction.md §6 6 步：identify → locate quote → compute hash →
build frontmatter → propose → verify in review）。

### F. 测试

- migration 0005 claim_sources 建表 + index
- VerifyClaimSource 4 状态各覆盖
- read_claim sources 真填 + drift_status 准确
- agent_handshake 含 recent_rejections
- rejections.jsonl append-only

目标测试 ≥ 320（D15 后 290 + 30）。

## Acceptance Criteria

- [ ] migration 0005 claim_sources 表
- [ ] VerifyClaimSource 4 状态
- [ ] read_claim sources 不再 staged 占位
- [ ] rejections.jsonl + agent_handshake recent_rejections
- [ ] templates/CLAUDE.md 含 claim 抽取算法
- [ ] CI 5 OS 全绿；测试 ≥ 320

## Out of Scope

- DRIFT 自动 propose_fix（仅检测 + 标 status；fix 留 W4 dream cycle）
- 部分 anchor 重定位（raw 内容改但 anchor 仍能找到 → 自动迁移；W4+）
- rejections 智能搜索（W4+ embedding）

## Decision (ADR-lite)

**Context**：DRIFT 检测在什么时机？read_claim 时实时算（每次重做）vs lint 时
批量算（缓存）vs watcher 触发 raw 改时算？

**Decision**：3 路并行：
1. **read_claim 实时**（cost 小，单 claim）—— D16 主路径
2. **watcher 触发**（raw 改 → 找 affected claims，标 drift_status='drift'）
   —— D13 watcher 集成
3. **lint 全扫**（D17 加 drift_check 规则）

drift_status 状态本身 cached 在 claim_sources 表 (加 last_verified_at + cached_status)，read 时按 mtime 判断要不要重算。

## Technical Notes

- `.wikimind/rejections.jsonl` append-only（D6 commit 同 pattern）
- VerifyClaimSource cost：1 file open + 1 hash 计算 < 5ms per source
- recent_rejections JSON format: `{review_id, agent, page, reason, ts}`
