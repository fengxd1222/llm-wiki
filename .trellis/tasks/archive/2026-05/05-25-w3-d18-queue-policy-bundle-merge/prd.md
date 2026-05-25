# W3 D18: Review queue 上限保护 + bundle 归并 + 优先级

## Goal

Review queue 不会无限膨胀。引入 soft/hard/critical 三级 limit + bundle
自动归并 + 优先级排序 + `wikimind review today` 一键看高优。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D18
- `spec-v2/docs/review-queue-policy.md §3 §4`

## Requirements

### A. Queue limit (review-queue-policy.md §3)

config 字段：
```toml
[review_queue]
soft_limit = 30      # 提示 user 该清理
hard_limit = 50      # 新 propose 进入 backlog (不入 active queue)
critical_limit = 100 # daemon 拒所有新 propose
```

`internal/service/queue.go`：
- `QueueState(ctx, db) (active int, backlog int, can_propose bool, ...)` 
- D10 agent_handshake.queue_state 真用这个（替代 D10 hardcode）
- D11 propose_* 在 hard 满时 → ErrQueueBacklog（patch + review row 仍写，
  但 status='backlog'）

### B. Bundle 自动归并 (§4)

`internal/service/bundle_merger.go`：
- 后台 goroutine 每 5 min 跑
- 同 agent + 同 kind + 1 小时内的多 review 自动 merge 进同 bundle
- 已 submitted bundle 不动；只整 open bundles
- 也可手动 `wikimind review merge r-N,r-M --kind=lint_fix`

### C. 优先级排序

D11 request_review 已计算 priority_score。D18 加：
- `wikimind review today` —— 按 priority desc 限 20，high-impact first
- `wikimind review list --priority critical|high|normal|low`
- bundle merge 时 priority = max(child priorities)

### D. accept-bundle 单命令

`wikimind review accept-bundle <bid> [--no-confirm]`：
- 复用 D12 review accept --bundle，但加 progress bar
- D12 partial 状态在 D18 加自动 retry 机制（subset succeed）

### E. 测试

- queue state 各 limit 边界
- propose_* 在 hard 满时 backlog
- bundle merger 5 min cron + edge (same agent / different kind 不 merge)
- priority sort
- accept-bundle progress + partial retry

目标 ≥ 390（D17 后 360 + 30）。

## Acceptance Criteria

- [ ] config review_queue 3 limit
- [ ] hard 满时 propose 进 backlog（不 reject）
- [ ] critical 满时 propose 真拒
- [ ] bundle 自动归并后台 goroutine
- [ ] `review today` / `review accept-bundle` CLI
- [ ] CI 5 OS 全绿；测试 ≥ 390

## Out of Scope

- 动态 limit 自适应（vault size based，W4+）
- LRU eviction stale propose（W4+）
- Cross-vault queue（远期）

## Decision (ADR-lite)

**Decision**：3 级 limit 静态配置（不动态）；backlog 是新状态值（reviews.status
扩展）；merge 后台 goroutine 跑在 daemon 主循环（W3 daemon 跑起来后接管）；
D18 仍 single-process CLI 模式，merger 在 `wikimind mcp serve` 进程内启动。

## Technical Notes

- soft 提示用 wiki_info.health.score 降级（lint_warnings + queue_pressure 两源）
- bundle merge 不会改变 review_id（review 仍各自 ID，只是 bundle_id 重指向）
- priority_score 公式 D11 已有；D18 加 age penalty (pending > 7d → +20)
