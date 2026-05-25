# W4 D29: bug 修复 + 性能微调

## Goal

修 D28 收集的 P0/P1 bug；性能微调达到 architecture §8 目标：
- query p95 < 100ms (10k pages vault)
- ingest single file < 5s (含 commit)
- daemon RSS < 100MB

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D29
- `spec-v2/docs/architecture.md §8` 性能目标

## Requirements

### A. Bug 修复

按 D28 `bugs-w4-d28.md` P0/P1 列表逐个修。每个 fix 单独 commit + regression
test。

### B. Performance benchmark

`internal/bench/` 新（test-only）：
- `bench_query_test.go`：1k / 5k / 10k page vault，query 100 次取 p95
- `bench_ingest_test.go`：1KB / 100KB / 10MB markdown ingest 时间
- `bench_daemon_test.go`：daemon RSS / goroutine count after 1k ingest

CI 跑 benchmark on macos-15 + ubuntu-24.04（不在 windows，性能 baseline 不严格）；
benchmark 结果上传到 GH artifact 让 user 看趋势。

### C. 关键优化点

D28 收集 + 已知热点：
- FTS5 query：加 `idx_pages_status` 实际使用率 verify
- ingest commit：D7 auto-reindex 全 vault；D29 改为 incremental（只 reindex
  改的文件）
- daemon goroutine：D20 5 goroutine + bundle merger；profile 验无 leak

### D. 测试

新增 + 修 bug 各带 regression test。目标 ≥ 640（D28 后 605 + 35，含 bench
test）。

## Acceptance Criteria

- [ ] D28 P0/P1 bug 全修
- [ ] 3 性能目标达成（query p95 / ingest / daemon RSS）
- [ ] benchmark CI 跑 + artifact upload
- [ ] 无 regression（既有测试不退）
- [ ] CI 5 OS 全绿；测试 ≥ 640

## Out of Scope

- P2 bug（推 V0.2）
- 性能优化超目标（10x）—— 仅达到 architecture §8 目标
- 大 vault > 100k pages（V0.2）

## Decision (ADR-lite)

**Decision**：性能目标硬截止；超过就 ship；优化空间 V0.2 再来。Benchmark
进 CI 防 regression。

## Technical Notes

- 用 `go test -bench` + `benchstat` 比较前后
- pprof 跑 daemon 找 leak
- SQLite EXPLAIN QUERY PLAN 验索引命中
