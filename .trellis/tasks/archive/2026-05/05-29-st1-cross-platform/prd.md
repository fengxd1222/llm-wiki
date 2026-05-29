# ST1 · Cross-Platform — Bridge Windows IPC + Watcher Race

> 父任务：`05-28-v0-1-1-patch-13-fixes-from-v0-1-0-audit`
> 维度：跨平台 + 并发。含 2 条 finding：F-041 / F-042。

## Goal

修复 v0.1.0 审查中两条阻塞 W3+ daemon 上线的跨平台 / 并发 finding，并加 Windows CI matrix 防回归。**只改这两块，不碰其他 finding。**

## Fixes

### F-041 · Bridge SocketPath Windows 不可用【P0 边缘 / 跨平台硬阻塞】

- **位置**：`internal/bridge/bridge.go`（`SocketPath` + `Listen`）。
- **现状**：Windows 上 `SocketPath` 返回 Named Pipe namespace 路径 `\\.\pipe\wikimind-<base>`，但 `Listen` 用 `net.Listen("unix", ...)` 监听该路径——必然失败。W2 无 daemon 不爆；W3 daemon + IPC 上线 Windows boot 即崩。
- **修复方向**（实现时先读真实代码确认）：
  1. 统一 `SocketPath` 与 `Listen` / `Dial` 的协议假设：Windows 要么走真正的 named pipe（需引依赖如 `github.com/Microsoft/go-winio`，评估后再定），要么 fallback 到普通文件 socket（`os.TempDir()/wikimind-<base>.sock`）。
  2. v0.1.1 优先**最小依赖方案**：若 winio 引入成本高，先用文件路径 unix socket（macOS/Linux 已工作）+ Windows 用 TCP loopback 或文件路径，保证 Listen/Dial 一致。
  3. 决策点：是否引 winio 由实现者评估后在 research/ 记录，再选方案。
- **测试**：bridge 单测覆盖 SocketPath + Listen/Dial round-trip；用 `runtime.GOOS` 分支断言路径形态。

### F-042 · Watcher Close send-on-closed-channel race【P1 / 并发】

- **位置**：`internal/watcher/watcher.go`（`Close` + `time.AfterFunc` 回调）。
- **现状**：`Close()` 关闭 channel 后，已排程的 `time.AfterFunc` 回调仍可能向已关闭 channel send，daemon Ctrl-C 时窗 panic。
- **修复方向**：
  1. `Watcher` 加 `closed atomic.Bool`；AfterFunc 回调首行 `if w.closed.Load() { return }`。
  2. 或 `Close()` 先 `Stop()` 所有 pending timer 再关 channel。
  3. 选 atomic 守卫（更简单、无需追踪 timer 句柄）。
- **测试**：`go test -race` 模拟 Close 与 AfterFunc 回调竞争的时窗单测。

## CI 增强（F-041 配套）

- GitHub Actions workflow 加 `windows-latest` matrix entry，至少跑 `go test ./internal/bridge/... ./internal/watcher/...`。
- 若仓库当前无 CI workflow，则新建 `.github/workflows/ci.yml`（实现时先 `ls .github/workflows/` 确认现状）。

## Acceptance Criteria

- [ ] `internal/bridge` 在 `GOOS=windows` 下 SocketPath/Listen/Dial 协议一致（交叉编译 `GOOS=windows go build ./...` 通过）。
- [ ] bridge 单测覆盖两平台路径逻辑。
- [ ] `internal/watcher` 的 Close+AfterFunc race 由 `go test -race ./internal/watcher/` 覆盖且通过。
- [ ] GitHub Actions 加 windows-latest matrix，至少跑 bridge + watcher 单测并通过。
- [ ] `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- [ ] 未触碰本子任务范围外的 finding。

## Out of Scope

- ❌ 其他 11 条 finding（属 ST2~ST5）。
- ❌ Windows named pipe 的完整生产实现（若 v0.1.1 选文件 socket fallback，winio 方案推 v0.2）。
- ❌ 修改 `.trellis/spec/`。

## Technical Notes

- 审查报告：`.trellis/tasks/archive/2026-05/05-27-code-quality-audit-v0-1-0/audit-report.md`（F-041 / F-042 详条）。
- 标尺：`.trellis/spec/backend/quality-guidelines.md`（跨平台 git init `<spec-entry>` 同类思路）+ `directory-structure.md`。
- 跨平台路径必经 `internal/vault.NormalizePath`；行尾 `\n`；`runtime.GOOS` 分支。
- 引入新依赖（如 winio）需在 research/ 记录评估，并在本 PRD 更新 Decision。
