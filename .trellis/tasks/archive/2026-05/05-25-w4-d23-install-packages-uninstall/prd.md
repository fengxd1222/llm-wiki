# W4 D23: Windows MSI + macOS Homebrew + uninstall

## Goal

发布 release-ready 安装包：Windows MSI（含 Scheduled Task 自动注册）+
macOS Homebrew tap（含 launchd plist）+ `wikimind uninstall --purge`。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D23
- `spec-v2/docs/cross-platform.md §2.3` launchd / Scheduled Task

## Requirements

### A. macOS Homebrew tap

`packaging/homebrew/wikimind.rb`：Formula 定义
- depends_on git + python (option)
- launchd plist install `~/Library/LaunchAgents/com.wikimind.daemon.plist`
- `brew install fengxd1222/tap/wikimind`

`packaging/launchd/com.wikimind.daemon.plist`：定义 `wikimindd --vault $HOME/wikimind-vault` 自启

### B. Windows MSI (wix)

`packaging/wix/wikimind.wxs`：
- Bundle `wikimind.exe` + `wikimindd.exe` 安装到 `C:\Program Files\WikiMind\`
- Scheduled Task 创建：daily `wikimindd --vault %USERPROFILE%\wikimind-vault`
- PATH 加 `C:\Program Files\WikiMind\bin`
- Start Menu shortcut

### C. Linux .deb (bonus)

`packaging/deb/control` + systemd user unit。

### D. `wikimind uninstall --purge`

`cmd/wikimind/command.go` 加 newUninstallCommand：
- 关 daemon + 移除 launchd plist / Scheduled Task / systemd unit
- `--purge` 删 vault data (默认保留，user 数据 sacred)
- `--keep-config` 保留 .wikimind/config.toml

### E. CI release pipeline

`.github/workflows/release.yml` (新)：
- 触发 tag push `v*.*.*`
- Build matrix 4 OS × {wikimind, wikimindd} = 8 binaries
- 打 release notes + GH Releases 上传
- Homebrew formula bump (PR to tap repo)

### F. 测试

- Homebrew formula audit
- MSI install + uninstall E2E (Windows CI)
- Linux deb lintian
- uninstall --purge 不留 残文件

目标 ≥ 515（D22 后 495 + 20）。

## Acceptance Criteria

- [ ] Homebrew tap 可 install + 启 launchd
- [ ] MSI 安装 Windows + Scheduled Task 自动
- [ ] `wikimind uninstall --purge` 干净
- [ ] release.yml CI 自动出包
- [ ] CI 5 OS 全绿

## Out of Scope

- Linux .rpm（D23 仅 .deb；rpm W4+）
- macOS App Store / notarization（manual flow）
- Auto-update 机制

## Decision (ADR-lite)

**Decision**：Homebrew tap 用自己 fork repo（fengxd1222/homebrew-wikimind）；
MSI 用 wix v3（稳定 + 跨平台 build）；Linux 仅 deb（rpm 留 W5+）。

## Technical Notes

- Homebrew Formula 借用 ripgrep / fd 等成熟 example
- wix 学习曲线大；用 `dotnet tool install --global wix` 跨平台 build
- launchd plist 模板复用 `spec-v2/docs/cross-platform.md §2.3`
