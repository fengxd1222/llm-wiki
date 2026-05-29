# ST4 · Concurrency & Resources — lockManager + Session Worktree Cleanup + Expire

> 父任务：`05-28-v0-1-1-patch-13-fixes-from-v0-1-0-audit`
> 维度：并发 + 资源管理。含 3 条 finding：F-028 / F-029 / F-030。
> 三条强相关（都围绕 session / daemon 长跑生命周期）。

## Goal

修复 3 条 daemon 长跑场景下的并发 / 资源 finding：lockManager 懒初始化 race、session 过期不清 worktree、Expire 死代码。**只改这 3 块。**

## Fixes

### F-028 · `vaultBackend.lockManager()` 懒初始化非线程安全【P1 / 并发】

- **位置**：`internal/mcp/`（`vaultBackend.lockManager()`；实现时 grep 定位精确文件/line）。
- **现状**：首次访问时多 goroutine 可能同时初始化 lockManager → data race。
- **修复方向**：用 `sync.Once` 包裹初始化，或在 `vaultBackend` 构造时 eager init（取决于 lockManager 是否依赖运行时参数）。优先 `sync.Once`。
- **测试**：`go test -race` 并发调用 `lockManager()` 的单测。

### F-029 · Session 过期未清理 worktree（资源泄漏）【P1 / 资源管理】

- **位置**：`internal/mcp/session.go`（session 过期路径 / `Expire`）。
- **现状**：session 过期被标记但 git worktree 留在磁盘，daemon 长跑下无限累积。
- **修复方向**：`Expire(id)` 内追加 `worktree.RemoveWorktree(ctx, vaultRoot, agent, sessionID)`；删 worktree 失败 log 但不阻塞 Expire（best-effort，遵循 worktree spec 的 prune-before-delete）。
- **测试**：验证 Expire 后 worktree 目录被删的单测。

### F-030 · `SessionStore.Expire` 在生产代码中从未被调用【P1 / 死代码 / 正确性】

- **位置**：`internal/mcp/session.go`（`Expire` 定义）+ `internal/daemon/loop.go`（daemon 主循环）。
- **现状**：`Expire` 是 dead code；session 即使过期也不清理。
- **修复方向**：daemon 启动注册一个 ticker（默认 5min，复用现有 lock reaper goroutine 模式 `loop.go:85`），周期遍历 SessionStore 调 `Expire` 清理过期 session。
- **依赖**：与 F-029 协同——Expire 被激活后会真正触发 worktree 清理，两者一起测才有意义。
- **测试**：ticker 周期触发 Expire 的单测（可用短 interval + fake clock 或直接调用周期函数）。

## 实施顺序建议

F-029（Expire 内补 worktree 清理）→ F-030（daemon 激活 Expire）→ F-028（独立，lockManager 加锁）。F-029/F-030 强耦合，建议同一轮实现 + 联合测试。

## Acceptance Criteria

- [ ] F-028：`lockManager()` 并发安全，`go test -race` 覆盖且通过。
- [ ] F-029：`Expire` 清理对应 worktree；删失败不阻塞、有 log。
- [ ] F-030：daemon 启动注册 session 过期 ticker，`Expire` 被周期调用（不再是死代码）。
- [ ] `go test ./... -race` 全绿（覆盖 mcp + daemon 并发路径）。
- [ ] `go build ./...` / `go vet ./...` 全绿。
- [ ] 未触碰范围外 finding。

## Out of Scope

- ❌ 其他 finding（ST1/ST2/ST3/ST5）。
- ❌ F-031 RateLimits 真正落地（属 v0.2，不在本 patch）。
- ❌ 修改 `.trellis/spec/`。

## Technical Notes

- 审查报告：F-028/029/030 详条 + W2 D10 handshake+worktree `<spec-entry>`。
- 标尺：`quality-guidelines.md`（并发 / 资源管理 / context 首参）/ W2 D10 spec-entry（worktree 创建/清理 + prune-before-delete）。
- daemon goroutine 模式参考 `internal/daemon/loop.go:85`（lock reaper ticker，wg.Add/Done + ctx.Done 退出）。
- worktree 清理 API：`internal/worktree.RemoveWorktree`（含 `git worktree prune`）。
