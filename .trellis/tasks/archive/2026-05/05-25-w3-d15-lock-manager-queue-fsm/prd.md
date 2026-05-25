# W3 D15: Lock manager + review queue 状态机完善

## Goal

实现 advisory lock manager（防多 agent 并发改同 page）+ review 状态机加
`superseded` / `conflict` 状态。打通"两 agent 抢同页不互覆盖"剧本。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D15
- `spec-v2/docs/agent-protocol.md §5`（Lock 协议：TTL / stale / disconnected 60s 宽限 / force）
- `spec-v2/docs/conflict-scenarios.md`（剧本 1：两 agent 抢同页）

## What I already know

- D10 SessionStore in-memory（session token + agent + sessionID）
- D11 propose_edit base_hash 是 page-level concurrency control，但**不阻塞**
  同时编辑（只在 commit 时报 BASE_HASH_MISMATCH）
- Lock 是更细粒度："你正在 edit 这 page，别人 propose 时收 LOCK_HELD"
- reviews.status 当前 5 态：pending / accepted / rejected / superseded / conflict
  —— D10 schema 已含 superseded / conflict 字符串但代码未用

## Requirements

### A. `internal/lock/` 新包

- `lock.go`：
  - `Lock` struct: `PageID / Holder (session token) / Agent / AcquiredAt / TTL / LastSeenAt`
  - `LockManager` in-memory map + sync.RWMutex
  - `Acquire(ctx, pageID, sess, ttl) error` —— ErrLockHeld if 别人持有 + 未过 grace
  - `Release(ctx, pageID, sess) error`
  - `Touch(ctx, pageID, sess) error` —— 续 TTL
  - `Reap(now time.Time) []ReapedLock` —— 清 expired locks (TTL 过 + 60s grace)
  - `ForceRelease(ctx, pageID, by string) error` —— admin/CLI 强制
- `disconnected.go`：session 失活 60s 宽限（D10 SessionStore 配合）
- 错误：`ErrLockHeld / ErrLockNotHeld / ErrLockNotMine`

### B. 2 MCP tool（mcp-tools.md §17 §18）

- `acquire_lock(page_id, ttl_seconds)` → handler 调 LockManager.Acquire
- `release_lock(page_id)` → 调 Release
- 注册：ReadOnlyHint=false (mgmt 类)
- 总 tool 数：15 → 17

### C. propose_* wired lock 检查

D11 propose_edit / propose_page handler 加：
- 调用前 `LockManager.IsHeldByOther(pageID, sess)` → ErrLockHeldByOther + 返回
  当前 holder 信息（agent name + acquired_at）让 user 决策

### D. reviews 状态机 superseded / conflict

`internal/service/review.go` 加：
- `MarkSuperseded(ctx, db, reviewID, supersededBy string) error` —— 当后来的
  propose 替代既有 pending review (同 page_id + 同 agent) 时调
- `DetectConflict(ctx, db, reviewID) error` —— accept 时如发现 base_hash
  已变（main 上 page 被别人改）→ 不直接 fail，标 status='conflict' + 提示
  user 手动解决

D11 propose_* 加：propose 之前查同 page+同 agent 的 pending review，存在 →
mark superseded + insert new；保持只有 1 个 active pending per (page, agent)

### E. 测试

- `internal/lock/lock_test.go`：
  - Acquire / Release / Touch / Reap / ForceRelease
  - 并发：100 goroutine Acquire 同 page → 1 成功
  - TTL 过期自动可被 Acquire
  - disconnected session 60s 宽限
- `internal/mcp/tools_test.go`：acquire_lock / release_lock handler
- `internal/service/review_test.go`：MarkSuperseded / DetectConflict
- **剧本 1 集成测试**：两 session 同时 propose_edit 同 page → 一个 reject
  ErrLockHeldByOther（或后到的 superseded 前者）

目标测试 ≥ 290（D14 后 260 + 30）。

## Acceptance Criteria

- [ ] `internal/lock/` 包 + 4 操作 + 60s grace 清理
- [ ] acquire_lock / release_lock 2 MCP tool（总 17 tool）
- [ ] propose_edit / propose_page wired lock 检查
- [ ] reviews.status superseded / conflict 真使用
- [ ] 剧本 1 集成测试通过
- [ ] CI 5 OS 全绿；测试 ≥ 290

## Out of Scope

- 持久化 lock（W3 daemon 才需要）
- lock 跨 vault（单 vault 假设）
- distributed lock（远期）

## Decision (ADR-lite)

**Context**：lock 是 advisory（agent 自愿遵守）还是 mandatory（daemon 阻断）？

**Decision**：D15 advisory：propose handler 主动查 lock，不持锁 → 允许写但
返 warning meta 字段；持锁人 → ErrLockHeldByOther + 提示。这跟 git/SQL
advisory lock 一脉相承。Mandatory 留 W3 daemon single-writer commit loop。

## Technical Notes

- TTL 默认 300s；max 3600s
- `LockManager.Reap` 启动 goroutine 每 30s 跑
- ForceRelease 写 audit log 行 (D6 commit.Commit append_log op)
