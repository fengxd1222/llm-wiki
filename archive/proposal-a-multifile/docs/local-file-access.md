# 跨平台本地文件访问方案（macOS + Windows）

> **设计前提（硬约束，必读）**：
>
> 1. 所有文件访问仅在**用户对本人授权目录**内进行。
> 2. **不绕过任何系统加密、ACL、企业 MDM、DLP 或类似安全机制**。任何"进程级加密"或"沙箱限制"
>    都是合法的系统/雇主控制，本方案**不挑战、不绕过、不规避**。
> 3. 当某个进程（如 IDE、Agent 自身）无法读取被加密/隔离的目录，但用户的标准系统终端（Terminal /
>    PowerShell）拥有合法的本人授权访问能力时，我们**通过用户授权的终端能力作为读取入口**——
>    这等同于"用户自己输入 `cat foo.md` 的合规调用"，不是绕过。
> 4. **不读取其他用户的数据**；不抓取剪贴板/钥匙串；不对外发送任何原始资料（除非用户显式配置了
>    git remote 或 embedding API）。
> 5. 任何文件读取路径都进 `change-log` audit；可被用户随时关停。

---

## 1. 总体架构：从 agent 到磁盘的 5 条路径

```
                      ┌────────────────────────────────┐
                      │  Agent (Claude Code / Codex …) │
                      └─┬────────┬────────┬────────┬──┘
                        │        │        │        │
              ┌─────────▼─┐   ┌──▼─────┐ ┌▼─────┐ ┌▼────────────────┐
       (A)    │ MCP stdio │   │ CLI    │ │direct│ │ shell bridge     │
              │ tools     │   │ subproc│ │  FS  │ │ (zsh/PowerShell) │
              └─────────┬─┘   └──┬─────┘ └┬─────┘ └────────┬────────┘
                        │        │        │                │
                        ▼        ▼        ▼                ▼
                    ┌──────────────────────────────────────────┐
                    │            llmwiki daemon                │
                    │  (path normalize, perm check, audit)     │
                    └────────────────────┬─────────────────────┘
                                         ▼
                              ┌───────────────────────┐
                              │  Vault (raw/, wiki/)  │
                              └───────────────────────┘
```

5 条访问路径的取舍：

| 路径 | 何时用 | 优点 | 缺点 |
|---|---|---|---|
| **A. MCP stdio** | 首选；Claude Code / Cursor / Cline / Continue | 结构化、可审计、跨平台 | 需 client 支持 MCP |
| **B. CLI 子进程** | MCP 不可用时（Codex、Hermes、自研） | 兼容所有 agent | 性能略差 |
| **C. 直接读 markdown** | Agent 仅需读 | 零依赖、最快 | 跳过 daemon → 没 audit；写文件危险 |
| **D. Shell bridge（zsh / PowerShell）** | **用户终端拥有访问能力、其他进程不能**时 | 合规绕过进程级限制 | 复杂、需用户显式启用 |
| **E. 长驻 daemon (file bridge mode)** | 多 agent 同时工作 | 单点 audit | 多一个进程 |

**MVP 默认 A + B + C；D 作为可选模块（详见 §6）；E 是 v0.2 推荐。**

---

## 2. macOS 平台细节

### 2.1 权限模型概览

macOS 11+ 的相关权限层：

| 层 | 影响 |
|---|---|
| **Full Disk Access (FDA)** | 决定进程能否读 `~/Library/Mail`、`~/Library/Containers/xxx` 等保护目录 |
| **App Sandbox** | Mac App Store 应用受限；CLI 一般不受 |
| **TCC（透明同意控制）** | 桌面/文档/下载文件夹访问需用户同意 |
| **File Permissions** | 标准 POSIX；用户家目录默认可写 |
| **Quarantine attribute** | 互联网下载文件带 `com.apple.quarantine` xattr |
| **Spotlight metadata** | `mds` 索引，含 `kMDItemContentType` 等 |

### 2.2 推荐配置

1. **Vault 放在 `~/Documents/<vault>` 或 `~/Sites/<vault>`**。
   - 第一次启动 daemon 时会触发 TCC "Documents Folder" 同意框，用户点允许即可。
   - **不要**放 `~/Library` 子目录（FDA 必需，体验差）。

2. **launchd 用 LaunchAgent（user-level），不要用 LaunchDaemon（system-level）**。
   `~/Library/LaunchAgents/io.llmwiki.daemon.plist`：

   ```xml
   <?xml version="1.0" encoding="UTF-8"?>
   <plist version="1.0">
   <dict>
     <key>Label</key><string>io.llmwiki.daemon</string>
     <key>ProgramArguments</key>
     <array>
       <string>/usr/local/bin/llmwikid</string>
       <string>--vault</string>
       <string>/Users/me/Documents/my-wiki</string>
     </array>
     <key>RunAtLoad</key><true/>
     <key>KeepAlive</key><true/>
     <key>StandardOutPath</key><string>/Users/me/Library/Logs/llmwiki.out.log</string>
     <key>StandardErrorPath</key><string>/Users/me/Library/Logs/llmwiki.err.log</string>
     <key>ProcessType</key><string>Interactive</string>
   </dict>
   </plist>
   ```

   加载：`launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.llmwiki.daemon.plist`。

3. **不要请求 FDA**，除非 vault 真的放在受保护目录。MVP 不需要 FDA。

### 2.3 文件 watcher：FSEvents

- Go 用 `fsnotify`（底层 FSEvents）；Rust 用 `notify`。
- FSEvents 的特点：**目录粒度**事件，不区分修改类型；要拿 `Modified | Created | Removed` 必须自己 diff。
- 大量 rename 会触发 `RenamedFromSet`+`RenamedToSet`，需配对处理。
- 网络盘 / 外置 NTFS：FSEvents 不支持，**降级为 polling**（2s 间隔）。

伪代码：

```go
ev := fsnotify.NewWatcher()
ev.Add(vaultRoot)  // recursive on macOS by default for FSEvents
for e := range ev.Events {
    if shouldIgnore(e.Name) { continue }
    debounce(e.Name, 200*time.Millisecond, func() {
        scheduleReindex(e.Name)
    })
}
```

### 2.4 Spotlight 与 mdfind

可选加速：

- 大 vault 启动时全扫太慢，可用 `mdfind` 拿"最近修改"列表来 prime 数据库：
  ```bash
  mdfind -onlyin "$VAULT/raw" "kMDItemFSContentChangeDate > $date '-1d'"
  ```
- 但**不依赖 Spotlight**，因为它可能被禁用、可能没索引新文件。

### 2.5 macOS 上的 shell bridge（zsh）

当 agent 自己不能读 vault 时（罕见，例如 agent 跑在 Docker、跑在另一个用户账户），通过 zsh 子进程读：

```bash
# 用户在自己的 shell 里启动 daemon；daemon 拥有 user's UID 的 FDA。
# agent 不直接读文件，而是通过 daemon CLI:
llmwiki cat-source <id>
llmwiki cat-page <id>
```

**重要**：daemon 必须在用户登录后由 launchd 启动，继承用户的 TCC 同意。不要尝试以 root 启动绕过 TCC。

### 2.6 macOS 性能与陷阱

- `Time Machine` 在 vault 上做快照会触发 watcher 风暴 → 检测 `.DocumentRevisions-V100` 路径直接忽略。
- iCloud Drive 同步 vault：可以，但 `.icloud` 占位符文件需要 `brctl download` 才能读到内容 → daemon 检测
  到 0 字节 `.icloud` 时按"未下载"处理。
- Spotlight 索引会争 IO；vault 大时建议 `mdutil -i off "$VAULT"`（用户自愿）。
- Unicode：HFS+/APFS 使用 NFD（分解形式），处理路径时统一 `unicodedata.normalize('NFC', path)`，与 Linux/Windows 对齐。

---

## 3. Windows 平台细节

### 3.1 权限模型概览

| 层 | 影响 |
|---|---|
| **NTFS ACL** | 用户对自己 `%USERPROFILE%` 子树拥有完全权限；其他用户目录默认拒绝 |
| **UAC** | 默认 standard user；不要请求 admin |
| **Controlled Folder Access (Defender)** | 默认保护 `Documents/`、`Pictures/`；可能拒绝未签名进程写入 |
| **BitLocker** | 加密整个驱动器；用户登录后 daemon 可读 |
| **EFS (NTFS Encrypted Files)** | 文件级加密；私钥绑定用户；其他进程读取受限 |
| **OneDrive on-demand** | 占位符文件，需触发 hydrate |

### 3.2 推荐配置

1. **Vault 放在 `%USERPROFILE%\Documents\<vault>`**。
2. **Controlled Folder Access**：如果用户启用了，需要把 `llmwikid.exe` 添加到允许列表
   （`Windows Security → Virus & threat protection → Ransomware protection → Allow an app`）。
   安装器自动检测并提示。
3. **代码签名**：v1 必做，否则 Defender / SmartScreen 会拦。
4. **服务 vs Scheduled Task**：
   - 推荐**Scheduled Task at logon**（运行在用户 session 内，继承所有用户权限）。
   - 不推荐 Windows Service（跑在 SYSTEM 账户，反而读不到用户的 EFS、OneDrive、Personal cloud）。

   PowerShell 安装：

   ```powershell
   $action = New-ScheduledTaskAction -Execute "$env:LOCALAPPDATA\llmwiki\llmwikid.exe" `
                                     -Argument "--vault `"$env:USERPROFILE\Documents\my-wiki`""
   $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
   $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
   Register-ScheduledTask -TaskName "LLMWikiDaemon" -Action $action -Trigger $trigger -Settings $settings
   ```

### 3.3 文件 watcher：ReadDirectoryChangesW

- Go `fsnotify`、Rust `notify` 都封装好；MVP 直接用。
- 注意 **buffer overflow**：当变更速度超过 buffer，会丢事件 → 启动时和**每 N 分钟**做一次 reconcile 全扫（增量 hash）。
- 大型移动（move 整个文件夹）会 reorder 事件；要 debounce 500ms 才稳定。

### 3.4 USN Journal（NTFS 高级）

> 仅适用于 NTFS 且需要 admin 启用。v1+ 可选。

NTFS 自带 USN Journal，可拿到**绝对可靠**的变更流（即使 daemon 没运行）。
启用：

```powershell
fsutil usn createjournal m=1073741824 a=1048576 C:
```

读 journal：用 [USNJournalReader](https://github.com/dmurfet/UsnJournal) 或自己 P/Invoke `DeviceIoControl(FSCTL_QUERY_USN_JOURNAL)`。

**取舍**：USN Journal 需要 admin、需要 NTFS、配置复杂。MVP 不上；只有当用户需要"长时间离线 + 回来重放"
时才提供。

### 3.5 长路径与编码

Windows 经典坑：

1. **MAX_PATH = 260** 默认；超过即失败。
   - 启用 Long Path Support：组策略 `Computer Configuration → Administrative Templates → System →
     Filesystem → Enable Win32 long paths`，或注册表 `HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled = 1`。
   - 应用 manifest 必须声明 `<longPathAware>true</longPathAware>`。
   - 内部对所有 path 加 `\\?\` 前缀（Go 的 `os` 包会自动处理，但 PowerShell 字符串不会）。
2. **路径分隔符**：内部表示统一 POSIX `/`；只在系统调用前转换为 `\`。
3. **UTF-8**：Windows 10 1903+ 支持 manifest 声明 `activeCodePage=UTF-8`；务必启用，否则中文文件名挂。
4. **大小写不敏感**：NTFS 默认 case-insensitive；vault 内部约定文件名**全小写 + kebab-case**，避免大小写
   冲突；linter 强制检查。
5. **保留字与非法字符**：`CON / PRN / AUX / NUL / COM1-9 / LPT1-9` 等不能用作文件名；linter 拒绝。
   非法字符 `< > : " | ? *` 同。
6. **行尾**：vault 强制 LF；git 配置 `core.autocrlf=false`，加 `.gitattributes` 锁定 `* text=auto eol=lf`。
7. **EFS 加密文件**：daemon 用用户身份运行才能读；EFS 文件复制到其他卷可能丢密钥 → 警告用户。
8. **OneDrive / Dropbox on-demand**：检测 `FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS` (0x00400000)；
   是的话先 hydrate（读一次就会拉下来），再 hash。注意会消耗流量。

### 3.6 Defender 性能影响

Defender 实时扫描会显著拖慢 PDF / 大文件 ingest：

- 建议用户把 vault 加入 Defender 排除列表（`Add-MpPreference -ExclusionPath`）：**只在用户主动同意时**。
- 安装器 **不要默认改 Defender 设置**；只提供命令和说明。

### 3.7 PowerShell shell bridge

当 agent 自身受沙箱限制、但用户登录态的 PowerShell 拥有访问权时：

```powershell
# daemon-less mode 的兜底：通过 PowerShell 子进程读取
llmwiki.exe cat-source <id>      # daemon CLI
# 或 agent 自己 spawn pwsh：
pwsh -NoProfile -Command "Get-Content -Encoding UTF8 -Raw $path"
```

PowerShell 一定加 `-Encoding UTF8` 和 `-NoProfile`，避免编码错乱和加载用户 profile 的副作用。

### 3.8 Windows shell bridge 安全注意

- **绝对不要把 agent 输入直接拼到 shell 命令里**。所有路径走 `--path-stdin` 之类的参数化通道。
- 路径 normalize 后再传：`Resolve-Path` + 严格白名单检查（必须 startswith vault root）。
- 启用 PowerShell ConstrainedLanguage 模式不可行（用户 profile 可能不允许）；改用最小命令集。

---

## 4. 跨平台规范化层（"normalize" 模块）

所有平台差异收敛到 daemon 的一个模块 `pathutil`：

### 4.1 路径规范化

```python
def normalize(p: str, vault_root: str) -> str:
    p = os.fspath(p)
    p = p.replace("\\", "/")
    p = unicodedata.normalize("NFC", p)
    p = posixpath.normpath(p)
    if not p.startswith(vault_root + "/"):
        raise PathTraversalError(p)
    return p
```

### 4.2 文件名约束

linter 强制：

- ASCII only；
- `[a-z0-9-_.]+`；
- 长度 ≤ 80；
- 不含 Windows 保留字；
- 不含 leading/trailing dot 或 space。

中文标题放 frontmatter `title:`，linter 不限制 frontmatter 字段值。

### 4.3 编码

- 文件**必须** UTF-8 (no BOM)；
- 行尾**必须** LF；
- 写入前 normalize NFC；
- watcher 启动时全扫一遍，违规文件 → 进 `wiki/_review/encoding-issues/`。

### 4.4 时间戳

- 内部全 UTC；显示时按用户本地时区。
- macOS APFS / NTFS / ext4 的 mtime 精度不同（nanosecond / 100ns / second）→ 比较时只看秒。

### 4.5 大小写

- vault 文件名永远小写；
- 程序内查找做 case-sensitive；
- 用户在 macOS / Windows 上保存的大小写不同的"同名文件"由 linter 拒绝。

### 4.6 文件锁

- macOS：`flock` advisory。
- Windows：`LockFileEx` / 用 `.lock` sentinel 文件 + PID 写入（更简单跨平台）。
- 推荐方案：**只用 `.llmwiki/locks/<page-id>.lock` sentinel 文件**，里面写 PID + agent name + acquired_at +
  ttl。daemon 启动时清理 stale lock（PID 不存在或 ttl 过期）。

---

## 5. 文件读取 bridge 设计

针对"进程级加密 / 沙箱限制"场景的合规读取通道。

### 5.1 设计要点

1. **用户授权前提**：bridge 只在 vault root 内工作；启动时显式校验 vault 在用户可读目录中。
2. **零长期凭证**：bridge 不要 cache 任何密钥；每次请求都靠继承的用户会话。
3. **请求最小化**：bridge 只接受 4 个动作：`stat / list / read / hash`。
4. **审计完整**：所有 bridge 调用都进 `change-log`，含调用 agent、目标路径、字节数。
5. **本地 socket / pipe**：不开 TCP 端口。

### 5.2 协议（JSON-RPC over named pipe / Unix socket）

```
macOS: /Users/me/.llmwiki/run/bridge.sock
Windows: \\.\pipe\llmwiki-bridge-<username>
```

Request:

```json
{ "id": 1, "method": "read", "params": { "path": "raw/papers/x.pdf", "offset": 0, "len": 1048576 } }
```

Response:

```json
{ "id": 1, "result": { "bytes_b64": "...", "hash_sha256": "...", "eof": true } }
```

Methods:

- `stat(path)` → `{ size, mtime, mode, exists }`
- `list(path)` → `{ entries: [...] }`
- `read(path, offset?, len?)` → `{ bytes_b64, hash_sha256, eof }`
- `hash(path, algo)` → `{ hash }`

错误码同 `docs/mcp-tools.md`。

### 5.3 启动方式

- macOS：launchd LaunchAgent 启动 bridge daemon，监听 socket。
- Windows：Scheduled Task at logon 启动 bridge，监听 named pipe。
- agent 通过 daemon 或自己直接连 socket/pipe；vault root 在 bridge 启动时锁定。

### 5.4 与 MCP server 的关系

Bridge 是 MCP server 的**底层数据通道**。
对外暴露 MCP（agent 友好），底层用 bridge 抽象"如何真正访问文件"。
在大多数 macOS / Windows 配置下，bridge 直接调系统 IO；只有当 agent 跑在沙箱进程里时，bridge 跑在用户
session 进程里、提供受信通道。

### 5.5 不做的事

- ❌ 不做"自动检测加密容器并尝试解密"。
- ❌ 不做"突破 MDM / 企业策略"。
- ❌ 不做"读取任意路径"——一切 path 必须 startswith vault root。
- ❌ 不做"读其他用户的文件"——bridge 进程 UID == 用户。

---

## 6. 容量与降级

| 场景 | 检测 | 降级策略 |
|---|---|---|
| Watcher 没事件（外置盘、SMB） | 启动时一次全扫；事件 < 阈值 + mtime 检测 | 切 polling |
| 占位符文件（iCloud / OneDrive） | size==0 + 占位符属性 | 跳过 ingest，标 `needs_hydrate`；或在用户同意时 hydrate |
| 大文件（>100MB） | size 检测 | 标 LFS；只 hash + 摘要，不全文 ingest |
| 编码异常 | UTF-8 decode fail | 进 `wiki/_review/encoding-issues/` |
| 路径过长（Windows） | path len + Long Path 未启用 | 拒绝创建，提示用户开 long path |
| 权限拒绝 | `errno=EACCES` / `WIN32 5` | 报错 + 不重试，写 audit 警告 |
| Defender 隔离 | 进程崩溃 / 文件消失 | 提示用户加排除列表 |

---

## 7. 测试矩阵（必须覆盖）

| 平台 | 文件系统 | 同步 | 必测 |
|---|---|---|---|
| macOS 14 | APFS（本地） | 无 | ✓ |
| macOS 14 | APFS | iCloud Drive | ✓ |
| macOS 14 | exFAT 外置 | 无 | ✓ |
| macOS 13 | SMB 网络盘 | 无 | △（polling） |
| Windows 11 | NTFS | 无 | ✓ |
| Windows 11 | NTFS | OneDrive | ✓ |
| Windows 11 | NTFS | 长路径 ≥ 260 | ✓ |
| Windows 10 | NTFS | EFS 加密 | △ |
| Linux | ext4 | 无 | ✓ |
| Linux | btrfs | 无 | △ |

E2E 必跑：

- 1k 文件初始化
- 1 个文件改 100 次（watcher 抖动）
- rename 整个目录
- 同一文件 macOS / Windows 互改（CRLF / NFC 漂移测试）
- vault 在睡眠/休眠/网络盘断开后恢复

---

## 8. 总结

跨平台核心规则就 6 条：

1. **POSIX 路径内部表示 + 系统调用前转换**。
2. **UTF-8 NFC + LF 强制**。
3. **文件名 ASCII kebab-case，中文进 frontmatter**。
4. **Watcher 用平台原生 + polling 兜底 + 定期 reconcile 全扫**。
5. **launchd LaunchAgent (macOS) / Scheduled Task at logon (Windows)，不要服务/system 账户**。
6. **Bridge 模块封装所有"如何读到文件"的细节；上层 MCP / CLI 只看抽象**。

把这 6 条做对，跨平台 99% 的坑都填了。
