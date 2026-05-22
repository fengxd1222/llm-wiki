# 跨平台细节

> macOS / Windows / Linux 三个平台的实操约束 + 已知地雷 + 失败 playbook。
>
> **核心策略**：用**强制约定**（文件名 ASCII kebab-case 等）压平多数差异，剩下的平台特殊点做隔离适配。

---

## 0. 总体策略

| 维度 | 策略 |
|---|---|
| **文件名** | 强制 ASCII kebab-case + lowercase（lint 拒绝违规）；中文 / 大写在 frontmatter 的 `title` 字段 |
| **路径分隔** | 内部全 POSIX `/`；系统调用前转换；path traversal 拒绝符号链接逃逸 |
| **编码** | UTF-8 无 BOM；line endings 强制 LF（`.gitattributes` 锁） |
| **Watcher** | 平台原生 API（FSEvents / RDCW / inotify）+ debounce + 兜底 reconcile |
| **Daemon** | 跨平台单二进制（Go static link）；安装包用平台原生（pkg / msi / deb） |
| **凭证** | OS keychain（macOS Keychain / Windows Credential Manager / Linux libsecret） |
| **自动启动** | macOS launchd / Windows Scheduled Task / Linux systemd user |

---

## 1. 文件名约定（最重要的一条规则）

### 1.1 强制规则

```
✅ wiki/claims/wiki-is-compounding.md          # lower kebab，全 ASCII
✅ wiki/entities/karpathy.md
✅ wiki/concepts/source-of-truth.md

❌ wiki/Claims/Wiki-Is-Compounding.md          # 大写：拒绝
❌ wiki/claims/wiki是compounding.md             # 中文：拒绝
❌ wiki/claims/wiki_is_compounding.md           # 下划线：拒绝
❌ wiki/claims/wiki is compounding.md           # 空格：拒绝
❌ wiki/claims/wiki:compounding.md              # 特殊字符：拒绝
```

中文 / 标题 / 大小写 → 全放 frontmatter：

```yaml
---
id: cl-2026-05-21-001
title: "Wiki 是一个 compounding artifact"
aliases: ["compounding wiki", "知识工件"]
slug: wiki-is-compounding
---
```

### 1.2 为什么这么严

- APFS 默认大小写**不敏感但保留** → `Foo.md` 与 `foo.md` 视为同一文件，但显示原始大小写
- NTFS 默认大小写**不敏感** → 同上
- ext4 / APFS-case-sensitive → 严格区分
- macOS Mojave+ 默认还是 case-insensitive APFS

→ 同一 vault 在 Mac 上叫 `Karpathy.md`，clone 到 Linux 后还能用，但 user 一旦在 Mac 上 commit 的
`Karpathy.md` 在 Linux 上是不同文件，导致 git 显示"重命名"或链接全断。

**用 ASCII lowercase kebab-case 一条规则全 cover**。

### 1.3 Lint 规则强制

```
rule: filename_convention
severity: error
check: filename matches /^[a-z0-9][a-z0-9-]*\.md$/
fix_hint: rename file to lowercase kebab-case, add original to frontmatter `slug`
```

Daemon 启动时 `wikimind doctor` 全扫一遍；任何违规拒绝运行直到 `wikimind doctor --fix-names` 修完。

### 1.4 已有违规的迁移

```bash
wikimind doctor --fix-names --dry-run
# 输出预览
wikimind doctor --fix-names
# 实际执行：rename + 更新所有 [[link]] 引用 + 写一条 change_log
```

---

## 2. macOS 特殊

### 2.1 APFS

| 默认 | 处理 |
|---|---|
| Case-insensitive（默认） | 强制 ASCII lower kebab 规避 |
| Unicode normalization（NFD） | 文件名只用 ASCII → 不受影响 |
| Resource forks（已弃用） | 忽略 |
| Snapshot | 不依赖 |
| Time Machine | `.wikimind/` 中 `index.db` 应排除（避免半写状态被 backup） |

**配置**：在 macOS 安装后给 `~/Library/Preferences/com.apple.TimeMachine.plist` 加入：

```xml
<key>SkipPaths</key>
<array>
  <string>{vault}/.wikimind/index.db</string>
  <string>{vault}/.wikimind/index.db-wal</string>
  <string>{vault}/.wikimind/index.db-shm</string>
  <string>{vault}/wiki/_worktrees</string>
</array>
```

`wikimind init` 安装时**询问** user 是否帮忙写（默认询问）。

### 2.2 FSEvents（Watcher）

```go
// 用 fsnotify 库（macOS 走 FSEvents）
watcher.Add(vaultRoot + "/raw")
watcher.Add(vaultRoot + "/wiki")
watcher.Add(vaultRoot + "/schema")
```

**已知问题**：

- FSEvents 在 sudden volume unmount / sleep / wake 可能 buffer overflow → 启动时全扫一次 + 每小时
  reconcile + 失败回退到 polling
- FSEvents 会上报"目录级"事件，需自己 diff 出文件级变化
- 高频写入（如 rsync 大批文件）可能合并事件 → debounce 200ms

### 2.3 launchd 自启

```xml
<!-- ~/Library/LaunchAgents/com.wikimind.daemon.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "...">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.wikimind.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/wikimindd</string>
        <string>--vault</string>
        <string>/Users/feng/karpathy-vault</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/Users/feng/Library/Logs/wikimindd.log</string>
</dict>
</plist>
```

`wikimind init --install-service` 自动生成 + `launchctl load`。
`wikimind uninstall --purge` 清理。

### 2.4 Full Disk Access (FDA) / Sandbox

**MVP 不申请 FDA**——daemon 仅访问 user 显式指定的 vault root（在 user home 下默认不需 FDA）。

如果 user 把 vault 放在受保护目录（如 `~/Library/`、`~/Pictures/`）：
- 启动时 daemon 检测 EACCES → 弹窗指引 user "授予 FDA 或换路径"
- 不主动申请 FDA（产品边界）

**Sandbox 化**（v1+）：评估打包成 sandboxed app（需 security-scoped bookmarks），MVP 不做。

### 2.5 安装

- 推荐 Homebrew：`brew install wikimind`
- 或独立 pkg installer（自动签名 + notarize）

---

## 3. Windows 特殊

### 3.1 NTFS

| 默认 | 处理 |
|---|---|
| Case-insensitive（默认） | 强制 ASCII lower kebab 规避 |
| 大小写敏感可启用（per directory） | 不依赖 |
| 长路径（> 260 字符） | 自动加 `\\?\` 前缀 |
| 文件名禁字符 | `< > : " / \ | ? *` 全部已被 kebab 规则禁用 |
| Reserved names | `CON, PRN, AUX, NUL, COM1-9, LPT1-9` → lint 加规则拒绝 |
| 路径分隔 | 内部 POSIX `/`，调系统 API 前转 `\` |

### 3.2 ReadDirectoryChangesW + USN Journal

```go
// fsnotify 在 Windows 走 ReadDirectoryChangesW
// 但缺点：buffer overflow 静默丢事件
```

**两层保险**：

1. **ReadDirectoryChangesW**：常规 watch，低延迟
2. **USN Journal 补漏**：daemon 每 5 分钟读取 USN journal 与上次 cursor 之间的 change record，补
   ReadDirectoryChangesW 漏掉的事件

USN journal 需要 admin？不需要，user 进程可读自己 home 下卷的 USN（Win10+）。

### 3.3 Scheduled Task 自启

```powershell
Register-ScheduledTask -Action (New-ScheduledTaskAction -Execute "C:\Program Files\WikiMind\wikimindd.exe" -Argument "--vault C:\Users\feng\karpathy-vault") `
                       -Trigger (New-ScheduledTaskTrigger -AtLogon) `
                       -TaskName "WikiMind Daemon" `
                       -User $env:USERNAME
```

`wikimind init --install-service` 自动注册。
`wikimind uninstall --purge` 反注册。

### 3.4 Windows Defender / Controlled Folder Access (CFA)

**最大的 Windows 11 地雷**——Defender CFA 默认开启，阻止未签名进程写 `Documents/`、`Desktop/` 等。

| 问题 | 缓解 |
|---|---|
| Defender 把未签名 daemon 当威胁 | v1 代码签名（OV/EV cert） |
| CFA 拒绝写 `Documents/` 下 vault | 安装器检测 + 引导 user 添加排除 |
| 兜底 | 安装时推荐 vault 放 `C:\Users\<name>\WikiMind\`（非保护路径） |

`wikimind doctor` 启动检测 CFA 状态，若 vault 在保护目录 → 弹窗指引：

```
WikiMind 检测到你的 vault 在 Controlled Folder 内。
Windows Defender 可能阻止 daemon 写入。

选项：
  [1] 帮我把 wikimindd.exe 加入 Defender 排除（需 admin）
  [2] 帮我把 vault 移到 C:\Users\<name>\WikiMind\
  [3] 我自己处理，跳过此检查
```

### 3.5 路径长度

Windows 默认路径限制 260 字符。WikiMind 的 worktree 嵌套（`wiki/_worktrees/agent-claude-sess-A1/...`）
容易超限。

**对策**：

- 所有 system call 自动加 `\\?\` 前缀（取消 260 限制，扩到 32767）
- 但要求：vault 路径必须用绝对路径；相对路径转绝对
- `wikimind doctor` 检测 vault 路径长度 > 200 → 警告

### 3.6 编码 + 行尾

- 强制 UTF-8 无 BOM
- 强制 LF（`.gitattributes` 写 `* text eol=lf`）
- PowerShell 输出默认 UTF-16 → `wikimind` CLI 强制 set `[Console]::OutputEncoding = [Text.Encoding]::UTF8`

### 3.7 安装

- 推荐 MSI installer（用 wix 打包）+ 代码签名
- 或 Scoop bucket：`scoop install wikimind`
- 或 Winget：`winget install wikimind`（v0.2 提交）

---

## 4. Linux（一等公民但不在 MVP 重点）

### 4.1 inotify

```go
// fsnotify 默认走 inotify
// 限制：max_user_watches 系统默认 8192，大 vault 不够
```

**对策**：

- 启动时检测 `cat /proc/sys/fs/inotify/max_user_watches`
- 若 < 65536 → 输出建议 `sudo sysctl fs.inotify.max_user_watches=524288`
- 不强制改（user 决定）

### 4.2 systemd user

```ini
# ~/.config/systemd/user/wikimind.service
[Unit]
Description=WikiMind Daemon

[Service]
ExecStart=/usr/local/bin/wikimindd --vault %h/karpathy-vault
Restart=always

[Install]
WantedBy=default.target
```

`systemctl --user enable --now wikimind`。

### 4.3 凭证

- libsecret（GNOME Keyring / KWallet）
- fallback：`~/.config/wikimind/credentials.enc`（age 加密 + master password）

### 4.4 安装

- Deb / RPM 包
- AUR（Arch）
- 二进制 tarball + install.sh

---

## 5. 同步工具适配

### 5.1 OneDrive / iCloud 占位符

**问题**：文件本地不存在（云端 only），文件名可见，但 `read()` 触发下载 / 报错。

| 平台 | 检测 |
|---|---|
| macOS iCloud | `getxattr(file, "com.apple.fileprovider.icloud")` 判定 |
| Windows OneDrive | `GetFileAttributesW` 返回 `FILE_ATTRIBUTE_OFFLINE` |

**daemon 行为**：

```
读 raw 文件，发现是占位符：
  - 标 sources.status = needs_hydrate
  - 询问 user 是否触发下载（带 size 警告）
  - 跳过 ingest 直到 hydrate
```

### 5.2 Dropbox / Drive 同步冲突

第三方同步把 `.wikimind/` 同步到云 → 多机同步时 index.db 状态分裂。

**对策**：
- `.wikimind/` 默认进 `.gitignore` + 同步排除清单
- `wikimind init` 安装时检测同步目录，给出指引

### 5.3 推荐 vault 放置

- macOS：`~/Documents/Vaults/<name>` 或 `~/Notes/<name>`，**不**放 iCloud Drive 路径
- Windows：`C:\Users\<name>\WikiMind\<name>`，**不**放 OneDrive 路径
- Linux：`~/Vaults/<name>`

---

## 6. Git LFS 跨平台

PDF / 图片 / 音频走 LFS。各平台需 `git lfs` 二进制：

- macOS：`brew install git-lfs`
- Windows：MSI installer
- Linux：包管理器

**WikiMind 安装时检测**：如果 vault 已有 LFS-tracked file 但当前机器没 git-lfs → 阻止 clone，指引安装。

---

## 7. 路径标准化（统一规则）

### 7.1 内部表示

- 所有路径**内部**用 POSIX `/` 分隔
- vault-relative 路径不带前缀（`wiki/claims/foo.md`，不要 `./wiki/...`）

### 7.2 系统调用前转换

```go
// Go
import "path/filepath"
osPath := filepath.FromSlash(internalPath)   // POSIX → 本地（Win 上变 \）
internalPath := filepath.ToSlash(osPath)
```

### 7.3 Path Traversal 防御

```go
func resolveAndCheck(vaultRoot, userPath string) (string, error) {
    abs := filepath.Join(vaultRoot, userPath)
    abs, err := filepath.EvalSymlinks(abs)  // 解析符号链接
    if err != nil { return "", err }
    if !strings.HasPrefix(abs, vaultRoot + string(filepath.Separator)) {
        return "", ErrPathEscape
    }
    return abs, nil
}
```

任何 path traversal 攻击（`../../etc/passwd`）或符号链接逃逸 → 拒绝。

---

## 8. 测试矩阵

CI 必跑组合：

| OS | filesystem | git | python | 备注 |
|---|---|---|---|---|
| macOS 14 (Sonoma) | APFS case-insensitive | 2.40+ | 3.11 | MVP |
| macOS 15 (Sequoia) | APFS case-insensitive | 2.42+ | 3.12 | MVP |
| Windows 11 | NTFS | 2.40+ | 3.11 | MVP |
| Windows 10 | NTFS | 2.30+ | 3.10 | 兼容 |
| Ubuntu 22.04 | ext4 | 2.34+ | 3.10 | 一等公民 |
| Ubuntu 24.04 | ext4 | 2.42+ | 3.12 | 一等公民 |
| Fedora 40 | btrfs | 2.42+ | 3.12 | 验证 |
| Alpine 3.19 (Docker) | ext4 | 2.42+ | 3.11 | server mode 验证 |

**未在矩阵的不保证**（如 FreeBSD、macOS HFS+ 老盘）。

---

## 9. 失败 Playbook（平台特定）

| 现象 | 原因 | 修复 |
|---|---|---|
| macOS 启动后 daemon 立即退出 | launchd plist 错误 | `launchctl list \| grep wikimind` + `wikimind doctor` |
| Windows daemon 写入失败 EACCES | CFA 阻止 | 加 Defender 排除或换 vault 路径 |
| 跨 mac/linux 拉 git，链接全断 | 大小写差异 | `wikimind doctor --fix-names`（重命名为 lowercase） |
| Watcher 漏事件 | FSEvents buffer overflow / RDCW miss | 等下次 reconcile（hourly）或 `wikimind reconcile` |
| 占位符文件 read 报错 | iCloud / OneDrive 未下载 | `wikimind ingest --hydrate` |
| Path too long Windows | vault 路径太深 | 移到更浅路径 + `wikimind doctor` 重设 |
| `index.db locked` Win | 杀进程不干净 | `wikimind doctor --kill-stale-locks` |

---

## 10. 平台 telemetry

默认关，user 显式开启后才发：

- OS + version + arch + locale
- 文件系统类型
- 同步工具（iCloud / OneDrive / Dropbox 是否检测到）
- Vault 路径深度（不发路径本身）
- Daemon uptime / crash count
- 性能指标（query latency、ingest throughput）

绝**不**发：vault 内容、用户名、文件名、claim 内容。
配置：`.wikimind/config.toml` 中 `[telemetry] enabled = false`（默认）。

---

## 11. 不在范围

- iOS / Android（v1.5+）
- FreeBSD / OpenBSD（社区贡献）
- 32-bit OS（不支持）
- ChromeOS（v2+，PWA）

---

## 12. 与其它文档的关系

- 文件名约定影响 [`claim-extraction.md`](claim-extraction.md)（page_id 命名）
- Watcher 影响 [`architecture.md §3.4`](architecture.md#34-watcher-与-sync-流程)
- CFA / FDA 是 [`risks.md`](risks.md) Wave 3 的 High 风险条目

---

## 一句话总结

> 强制 ASCII lower kebab 文件名 + UTF-8 LF + 内部 POSIX 路径 + 平台原生 watcher 三选一 +
> 同步占位符检测——五条铁律压平 macOS / Windows / Linux 的多数差异。剩下的平台地雷
> （CFA / 长路径 / launchd / inotify 上限）在 `wikimind doctor` 中显式检测并引导 user。
