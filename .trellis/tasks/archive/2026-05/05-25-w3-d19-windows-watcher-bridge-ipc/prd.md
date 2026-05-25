# W3 D19: Windows watcher RDCW + 长路径 + CLI bridge IPC

## Goal

把 D13 macOS-only watcher 扩展到 Windows（RDCW + USN journal 补漏 +
`\\?\` 长路径 + CFA）。同时引入 CLI bridge：CLI 与 daemon 通过 named
pipe / unix socket 通讯（D8 mcp serve 进程内 + D20 daemon 分离过渡）。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W3 D19
- `spec-v2/docs/cross-platform.md §3.2` Windows watcher
- `spec-v2/docs/cross-platform.md §1.x` 长路径 `\\?\`
- `spec-v2/docs/engineering-decisions.md §2.4` IPC

## Requirements

### A. Windows watcher 兼容

D13 已用 fsnotify（跨平台抽象），但：
- macOS FSEvents 上报目录级 → fsnotify 抽得不全
- Windows RDCW 文件级 ok 但 buffer 易 overflow
- D19 加 **USN journal 补漏 reconcile**：每小时全扫 raw/ + wiki/ 对比
  pages/sources 表 mtime + size，drift 标记入 lint backlog

`internal/watcher/reconcile.go`：
- `Reconcile(ctx, vault, db) (drift []Drift, err error)`
- 跑在 watcher goroutine 旁边 cron

### B. Windows 长路径

`internal/vault/longpath_windows.go` (build tag `// +build windows`)：
- `EnableLongPaths()` 自动加 `\\?\` prefix（Windows API call）
- `internal/vault/path.go` 所有 abs path 通过 helper 包装

### C. CFA (Controlled Folder Access)

Windows Defender 把 raw/ wiki/ 加白名单。`wikimind doctor` 检测 +
推荐命令 (PowerShell `Add-MpPreference -ControlledFolderAccessAllowedApplications`)。
D19 加 detect only；fix 留 user manual。

### D. CLI bridge IPC

新包 `internal/bridge/`：
- `server.go`：daemon 起 IPC server (unix socket / named pipe)
- `client.go`：CLI 连接 send 命令
- 协议：JSON-RPC over stream
- 用于 D20 wikimind daemon 启 + wikimind <subcommand> 走 RPC

D19 阶段是骨架：CLI 仍可在 daemon-less 模式跑；如果 daemon 在跑 (`wikimind daemon start`) 则后续命令优先走 RPC。

### E. 测试

- Windows RDCW edge case mock
- Reconcile 检测 drift
- 长路径处理 ≤ 4096 chars
- CLI bridge round-trip
- IPC over uds + named pipe

目标 ≥ 415（D18 后 390 + 25）。

## Acceptance Criteria

- [ ] Windows CI watcher pass + RDCW reconcile
- [ ] 长路径处理（>260 chars Windows）
- [ ] doctor 检测 CFA
- [ ] internal/bridge/ JSON-RPC 骨架（uds + pipe）
- [ ] CI 5 OS 全绿；测试 ≥ 415

## Out of Scope

- daemon 主循环（D20）
- 远程 daemon (TCP)
- CFA 自动 fix
- USN journal native 集成（D19 用 polling reconcile 替代）

## Decision (ADR-lite)

**Decision**：跨平台 watcher 一层 fsnotify + 一层 cron reconcile（cross-platform.md §2.2 已定）。
IPC 走 net.Conn 抽象（uds on unix / named pipe on windows）。daemon 模式
D20 才启动 RPC，D19 是 IPC 接口准备。

## Technical Notes

- `golang.org/x/sys/windows/registry` 检测 CFA 状态
- 长路径：`\\?\C:\very\long\...`，仅 Windows API；Go 1.20+ 默认支持
- IPC payload 复用 mcp protocol JSON shape（兼容 future MCP over socket）
