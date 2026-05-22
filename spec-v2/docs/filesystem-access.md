# 文件访问模型

> **整合补遗**：方案 A 原有 `local-file-access.md` 一篇讲"从 agent 到磁盘的 5 条路径"，
> spec-v2 第一轮整合时**漏掉了这一篇**，导致"读"被错误地窄化成只能走 MCP。本文档恢复它。
>
> 核心原则：**读多通道（宽松），写单通道（严格），`raw/` 只读**。

---

## 1. 设计前提（硬约束）

1. 文件访问**仅在用户对本人授权目录内**进行。
2. **不绕过任何系统加密、ACL、企业 MDM、DLP**。这些是合法的系统 / 雇主控制，WikiMind 不挑战、
   不规避。
3. 当某个进程（IDE / agent 自身）无法读取某目录，但**用户的标准系统终端**（Terminal / PowerShell）
   拥有合法的本人授权访问——可以**通过用户授权的终端能力作为访问入口**。这等同于"用户自己输入
   `cat foo.md`"，是**合规调用，不是绕过**。
4. 不读其他用户数据；不抓剪贴板 / 钥匙串；不外发原始资料（除非用户显式配 git remote / embedding）。
5. 受控通道（MCP / CLI / bridge）的访问进 `change-log` audit。

> 第 3 条是本文档的关键——**终端级文件读写是 WikiMind 的功能能力，不是要防范的威胁**。

---

## 2. 读 vs 写：根本区别

| | 读 | 写 |
|---|---|---|
| 破坏性 | 无——读不改变任何状态 | 有——可污染知识库 |
| 通道 | **多通道，宽松** | **单通道，严格** |
| 约束 | 几乎不限（唯一例外：`quote_hash`，见 §4.3） | 必经 worktree → `propose_*` → review queue |
| 需要 handshake | 否（纯文件读不需要） | 是 |

**为什么这样分**：WikiMind 的单一闸门（review queue）保护的是**写**——防止未经审查的内容污染 wiki。
读再多也不破坏任何东西。把读也锁死成"只能走 MCP"只会让 agent 难用、且无任何安全收益。

---

## 3. 五条访问路径

```
                  ┌────────────────────────────────┐
                  │  Agent (Claude Code / Codex …)  │
                  └─┬───────┬───────┬───────┬──────┘
              (A)   │   (B) │   (C) │   (D) │
          ┌─────────▼┐ ┌────▼───┐ ┌─▼────┐ ┌▼──────────────┐
          │ MCP stdio│ │ CLI    │ │direct│ │ shell bridge  │
          │ tools    │ │ subproc│ │  FS  │ │(zsh/PowerShell)│
          └────────┬─┘ └───┬────┘ └─┬────┘ └───────┬───────┘
                   │       │        │              │
                   ▼       ▼        │(只读)         ▼
              ┌──────────────────┐  │      ┌──────────────────┐
              │   wikimindd      │◀─┘      │ 用户登录态终端     │
              │ (path norm/audit)│         │ 的合法访问能力     │
              └────────┬─────────┘         └──────────────────┘
                       ▼
              ┌──────────────────┐
              │ Vault (raw/wiki/)│
              └──────────────────┘
```

| 路径 | 何时用 | 优点 | 缺点 | MVP |
|---|---|---|---|---|
| **A. MCP stdio 工具** | 首选 | 结构化、带 `quote_hash`、进 audit、跨平台 | 需 client 支持 MCP | ✅ 默认 |
| **B. CLI 子进程** | MCP 不可用（部分 agent） | 兼容所有 agent | 性能略差 | ✅ 默认 |
| **C. 直接读 FS** | agent 仅需读 | 零依赖、最快 | 跳过 daemon → 无 audit；**仅限读** | ✅ 默认 |
| **D. Shell bridge** | agent 进程本身无 vault 权限时 | 合规代理访问 | 复杂、需显式启用 | 🟡 可选 |
| **E. 长驻 file-bridge daemon** | 多 agent 高频 | 单点 audit | 多一个进程 | ⏳ v0.2 |

**MVP 默认 A + B + C；D 可选；E 留 v0.2。**

---

## 4. 读路径详解

### 4.1 直接读（路径 C）

Agent **可以直接** `cat` / `grep` / `ripgrep` / 用自己的文件工具读 `raw/` 和 `wiki/`：

- 适合：大范围浏览、grep 找关键词、快速读 `index.md`、扫一个目录
- 零依赖、最快——不经 daemon、不需 handshake（纯文件读）
- 代价：daemon 看不到 → 这次读不进 audit（可接受，读无破坏性）

### 4.2 MCP 读（路径 A）

`read_page` / `read_raw` / `read_raw_anchor` / `read_claim` / `search` / `list_index` /
`graph_neighbors` / `get_history` / `wiki_info` 共 9 个 read 工具——它们是**结构化读取的优化通道**：

- 返回解析好的 frontmatter / 锚点 / 结构，agent 不用自己 parse
- 带 `quote_hash`、进 audit
- 跨平台一致（路径、编码差异由 daemon 抹平）

read 工具不是"唯一读法"，是"省事 + 权威"的读法。

### 4.3 quote_hash 特例（读的唯一约束）

抽 claim 需要 `quote_hash`，而 `quote_hash` 必须是 source 原文的权威哈希：

- **想要权威 `quote_hash`** → 必须经 `read_raw_anchor`（daemon 计算）
- **但即使 agent 直接读 raw 抽 claim、自己填 hash** → `propose_claim` 时 daemon 会**重算校验**，
  hash 对不上直接 `QUOTE_HASH_MISMATCH` 拒绝

所以"直接读 raw 抽 claim"也走得通——daemon 在 propose 关卡兜底。直接读不会让编造引用蒙混过关。

---

## 5. 写路径详解

### 5.1 唯一写路径

```
agent 在自己的 worktree 内编辑（cat / Write / Edit / sed —— 随便用终端）
        ↓
调 propose_*  → daemon 取 worktree diff → wiki/_review/
        ↓
review queue → user 决议 → daemon commit 到正式 wiki/
```

**关键澄清**：写**不是**"不能用终端"——agent 在自己的 **worktree** 里用任何终端工具自由编辑文件，
这完全 OK，worktree 就是 agent 的工作区。受控的是 **worktree → 正式 wiki** 这一步（必经 propose）。

### 5.2 不允许的写

| ❌ 操作 | 原因 |
|---|---|
| 直接写**正式 `wiki/`**（worktree 之外的 wiki 文件） | 绕过 review queue 单一闸门 |
| 写 `raw/` 任何文件 | raw 不可变 |
| 直接 `git commit` 正式分支 | 只有 daemon 是单写者 |
| 直接写 `schema/` | schema 由 user 维护 |

### 5.3 worktree 内 vs 正式 wiki —— 看目录

- **worktree**（`wiki/_worktrees/agent-xxx/`）= agent 的，随便用终端编辑
- **正式 wiki/**（main checkout）= daemon 的，只有 daemon commit

agent 的 `Write` / `Edit` 目标路径永远落在它握手时分配的 worktree 内——这是判断"合规写"vs"越界写"
的唯一标准。

---

## 6. Shell Bridge（路径 D）

### 6.1 何时需要

正常情况 agent 进程跑在用户账户下，对 vault 有完整文件权限——直接走路径 A/B/C 即可，**不需要 D**。

路径 D 用于少数场景：agent 进程**本身**无法访问 vault——

- agent 跑在 Docker 容器里
- agent 受 IDE 的 sandbox 限制
- agent 跑在另一个用户账户下

而**用户登录态的终端 / daemon** 拥有合法访问能力。此时通过 shell bridge 代理读取——这是
"用用户自己的授权能力"，符合 §1 第 3 条，不是绕过。

### 6.2 协议（只读）

Shell bridge **只接受只读动作**——`stat` / `list` / `read` / `hash` 四个。它不提供写能力（写永远
走 §5 的 propose 路径）。

- 传输：Unix domain socket / named pipe，**不开 TCP 端口**
- 路径：每个请求的 path 必须 `startswith` vault root（path traversal 拒绝）
- 安全：bridge 进程 UID == 用户；不把 agent 输入拼进 shell 命令（参数化传递）

### 6.3 MVP 状态

可选模块——MVP 实现但默认不启用。`wikimind doctor` 检测到"agent 无 vault 权限"时引导 user 开启。

---

## 7. `raw/` 的额外保护

`raw/` 不可变是最该硬化的一层。除协议约束（agent 不写 raw）外，提供**可选**的 OS 级硬化：

- macOS：`chflags uchg`（user immutable flag）
- Windows：只读 ACL

`wikimind init` 询问是否启用。注意：`chflags` 同用户可解除，不是绝对硬隔离——但能挡住"手滑直接
写 raw"，把误写变成"必须刻意解锁才能做"。

被动兜底：daemon watcher 检测 `raw/` 变化、引用它的 claim 出现 quote_hash mismatch → 触发
`claim_drift` lint（见 [`lint-rules.md §4`](../templates/lint-rules.md)）。

---

## 8. 带外写入的检测（写闸门的兜底）

直接**读**无害，无需检测。但若有进程——agent 失误、user 手动、第三方同步工具——直接写了
**正式 `wiki/`**：

- daemon 的 watcher + `git status` 一眼能检测到 working tree 有未登记的改动
- daemon 标记为 `unmanaged_change`，移入特殊 review，由 user 决定 accept / revert
- 把"带外写"从**静默污染**降级为**可见、可追溯、可回滚**的事件

这不是"防 agent"——是兜住**任何来源**的带外写（包括 user 自己手抖在 Obsidian 里改了正式 wiki）。
详见 [`failure-playbook.md §2.6`](failure-playbook.md)。

**可选硬隔离**（高要求用户）：daemon 以独立 user 运行、`wiki/` 对 agent user 只读——这是唯一的真
硬约束，但重。MVP 不默认，作为部署选项。

---

## 9. 平台细节

路径规范化、编码、watcher、launchd / Scheduled Task、占位符文件、长路径等平台差异，全部见
[`cross-platform.md`](cross-platform.md)。本文档只讲"访问路径模型"，不重复平台细节。

---

## 10. 审计

| 路径 | 是否进 audit |
|---|---|
| A. MCP 工具 | ✅ |
| B. CLI | ✅ |
| C. 直接读 FS | ❌ daemon 看不到 |
| D. Shell bridge | ✅ |
| E. file-bridge daemon | ✅ |

路径 C 不进 audit 是**有意的 trade-off**——读无破坏性，不值得为审计每次 `cat` 都强制走 daemon。
需要完整读 audit 的场景（如合规要求）用 A/B。**写永远有 audit**（写只能走受控路径）。

---

## 11. 决策汇总

| 问题 | 决策 |
|---|---|
| agent 能否直接 `cat`/`grep` 读 vault | ✅ 能（路径 C，MVP 默认） |
| 读必须走 MCP 吗 | ❌ 不必；MCP 是优化通道（结构化 + audit + quote_hash） |
| agent 能否用终端在 worktree 里编辑 | ✅ 能——worktree 是 agent 工作区 |
| agent 能否直接写正式 wiki/ | ❌ 不能——必经 propose → review queue |
| agent 能否写 raw/ | ❌ 永不 |
| quote_hash 怎么保证 | `read_raw_anchor` 给权威值；`propose_claim` 时 daemon 重算校验兜底 |
| shell bridge 何时用 | agent 进程本身无 vault 权限时（Docker/沙箱/别账户）；MVP 可选 |
| 带外写正式 wiki 怎么办 | watcher + git status 检测 → `unmanaged_change` → user 决议 |

---

## 12. 不在范围

- 突破 MDM / 企业策略 / 自动解密 —— 永不做（§1）
- 读其他用户的文件 —— 永不做
- 路径 E（长驻 file-bridge daemon）的完整设计 —— v0.2

---

## 13. 与其它文档的关系

- 平台差异（路径 / 编码 / watcher / 占位符）→ [`cross-platform.md`](cross-platform.md)
- 写路径的协议细节（worktree / propose / review）→ [`agent-protocol.md`](agent-protocol.md)
- read 工具的 JSON schema → [`mcp-tools.md`](mcp-tools.md)
- 带外写的回滚 → [`failure-playbook.md`](failure-playbook.md)
- agent 的读写规范 → [`templates/AGENTS.md`](../templates/AGENTS.md)

---

## 一句话总结

> 读和写是两套规则。**读**：raw / wiki 都可直接 `cat`/`grep`/MCP/CLI 多通道——宽松，因为读不破坏。
> **写**：agent 在自己 worktree 内随便用终端编辑，但进正式 wiki 只有一条路——`propose_*` → review
> queue → daemon commit。终端级读写是 WikiMind 的功能能力；单一闸门只约束"写进正式 wiki"这一步。
