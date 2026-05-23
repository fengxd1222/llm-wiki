# Fix Windows path cross-validate case sensitivity

## Goal

修复 D2（commit `d8a8958`）引入的 `internal/vault.LoadConfig` 在 Windows NTFS
（大小写不敏感）上 cross-validate `vault_root` 失效的 bug —— `TestLoadConfigVaultRootMismatch`
在 Windows CI 失败，导致 D2/D3 两次 push CI 红。修复后让最新 commit CI 全绿。

## What I already know

- D2 `validateConfig` 用 `filepath.EvalSymlinks` + **严格字符串相等**比对 declared vs
  actual `vault_root`
- Windows NTFS 大小写不敏感：`C:\Users\Foo` 与 `C:\users\foo` 是同一目录但字符串不等
- macOS APFS 默认也大小写不敏感，但 `EvalSymlinks` 返回的实际路径保留原大小写——
  所以 macOS 测试侥幸过了
- CI 失败具体：`--- FAIL: TestLoadConfigVaultRootMismatch (0.23s)` on `go (windows-2022)`
- D3 CI 因同一 bug 失败（test 包没动但仍跑）
- **非阻塞**：`go (ubuntu-24.04)` 的 `actions/checkout@v4` 间歇报
  `fatal: could not read Username for github.com` —— GitHub 平台问题，重跑通常解决，与代码无关

## Requirements

- 修 `internal/vault.LoadConfig`（按 Q1 决定的方向）
- `TestLoadConfigVaultRootMismatch` 跨 macOS / Linux / Windows 都 PASS
- D1/D2/D3 已有 AC + 测试不退化
- 重新触发 CI 让 main 最新 commit 5 OS 矩阵全绿

## Acceptance Criteria

- [ ] `validateConfig`：Windows 用 `strings.EqualFold`，macOS / Linux 保持严格相等
- [ ] 新增 `TestLoadConfigVaultRootCaseInsensitiveOnWindows`（仅 Windows 跑），证明
      大小写不同被识别为同一 vault
- [ ] `TestLoadConfigVaultRootMismatch` 在 Windows / macOS / Linux 都 PASS
- [ ] 所有 D2 / D3 现有测试不退化
- [ ] `go build ./...` + `go vet ./...` + `go test ./...` 全绿
- [ ] CI 5 OS 矩阵（macos-14 / 15 / windows-2022 / ubuntu-22.04 / 24.04）+ python 全绿

## Definition of Done

- LoadConfig 修复 + 跨平台测试覆盖
- lint / vet / CI 绿
- commit 并 push（fix 一个 commit）

## Out of Scope

- D4 开发（pages 表）—— 修完 fix 后再开
- `ubuntu-24.04` actions/checkout 间歇 auth fail —— GitHub 平台问题，单独处理
- `actions/checkout@v4` / `setup-go@v5` / `setup-python@v5` 升级以消 Node 20 deprecation
  warning —— 单独处理

## Decision (ADR-lite)

**Context**: D2 的 `validateConfig` 用严格字符串相等比对 vault_root，在 Windows NTFS
（大小写不敏感）下，`C:\Users\Foo` 与实际存储的 `C:\users\foo` 会被误判为不匹配；
`TestLoadConfigVaultRootMismatch` 在 Windows CI fail。

**Decision**: 治本——`validateConfig` 在 `runtime.GOOS == "windows"` 时用
`strings.EqualFold` 比较；macOS / Linux 保持严格字符串相等（D2 现状）。

**Consequences**:
- 修了"Windows config 该报 mismatch 但没报"的真 bug；
- 不破坏 Linux ext4 严格 case-sensitive 语义；
- 极少数 case-sensitive APFS / 启用 case-sensitive 的 Windows 目录的用户感知不到差异
  （他们 init 时会用一致大小写）；
- `TestLoadConfigVaultRootMismatch` 需调整测试构造，让 Windows 上也能造出真 mismatch
  （例如不同 drive letter / 不同上级目录），或 skip Windows 这一断言并新增专门的
  Windows case-insensitive 用例。

## Technical Notes

- `runtime.GOOS` 判平台
- 路径大小写不敏感比较：`strings.EqualFold`
- macOS APFS 大多 case-insensitive，但 case-sensitive APFS 也存在；Linux ext4 严格
  case-sensitive；Windows NTFS 默认 case-insensitive
- 最务实：Windows 用 `EqualFold`，macOS / Linux 保持严格相等（与 D2 现状一致）。
  极少数 case-sensitive Windows / case-sensitive APFS 用户感知不到差异（这种用户大概率
  自己 init 路径用一致大小写）
