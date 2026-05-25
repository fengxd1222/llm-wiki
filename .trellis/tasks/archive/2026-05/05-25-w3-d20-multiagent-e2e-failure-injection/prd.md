# W3 D20: Multi-agent E2E + 失败注入 + doctor 完善

## Goal

D15-D19 各部件单独 OK；D20 端到端跑：Claude + Codex 同时 ingest + lint，
验证 final vault 一致 + change-log 完整。同时注入 daemon 杀 / agent 杀
等失败，验证恢复机制。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D20
- `spec-v2/docs/failure-playbook.md §5` doctor 命令
- `spec-v2/docs/conflict-scenarios.md` 全部剧本

## Requirements

### A. Daemon 主循环 `cmd/wikimindd/` 真启动

W0 stub 终于上线：
- `wikimindd --vault <path>` 启动 daemon
- 内含：
  - mcp serve (D8) on stdio (subprocess)
  - bridge IPC server (D19)
  - watcher (D13) + reconcile (D19)
  - bundle merger goroutine (D18)
  - session reaper (D10)
  - lock manager reaper (D15)
- Graceful shutdown (SIGTERM)
- 日志到 `.wikimind/daemon.log`

`internal/daemon/loop.go` 新文件：
- Main loop 协调 5 个 goroutine 生命周期

### B. Multi-agent E2E test

`cmd/wikimind/e2e_multiagent_test.go`：
- 启 daemon (subprocess) → mock 2 agent session 同时调 propose_*
- 验证：reviews 表 2 个 row，bundle merge，无冲突；commit log 1:1
- 验证：critical_limit hit → 第 3 agent ErrQueueFull

### C. 失败注入测试

- 杀 daemon mid-accept → 重启后 reviews queue 状态恢复（patch 文件状态 + reviews.status 一致）
- 杀 agent (session 失活) → 60s 后 lock 自动释放
- vault dir rm → daemon 报错 + 退出（不 panic）
- SQLite db corrupt → doctor 推荐 fsck

### D. `wikimind doctor` 完善 (failure-playbook.md §5)

D14 加了基础 doctor；D20 加：
- git binary version ≥ 2.28
- python3 + pypdf 装好
- vault 三层目录存在
- SQLite db 完整性（PRAGMA integrity_check）
- session count / lock count / queue size 健康度
- daemon 进程状态（is daemon running? 通过 IPC ping）
- 输出 → green / yellow / red 三档

### E. 测试

- e2e_multiagent_test 包含 conflict-scenarios.md 全部 4 个剧本
- 失败注入：4 个剧本
- doctor 9 项检查

目标 ≥ 450（D19 后 415 + 35）。

## Acceptance Criteria

- [ ] wikimindd 真启动 + 5 goroutine
- [ ] E2E 2 agent 并发剧本通过
- [ ] 失败注入 4 类全恢复
- [ ] doctor 9 项检查（绿/黄/红）
- [ ] CI 5 OS 全绿；测试 ≥ 450

## Out of Scope

- Distributed daemon
- Hot reload config
- Telemetry / metrics endpoint（W4+）

## Decision (ADR-lite)

**Decision**：daemon 通过 IPC 服务多 CLI；CLI 仍可 daemon-less 模式（向后
兼容）。Auto-start daemon 留 W4 release packaging。

## Technical Notes

- daemon goroutine lifecycle 用 errgroup.Group
- failure injection 用 test helper signal subprocess SIGKILL
- doctor 输出借鉴 `brew doctor`
