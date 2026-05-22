# WikiMind — LLM Wiki 主 Spec (v2)

> **整合自三方案审查**：以多文件方案 A 为基底，吸收方案 B（单文件 WikiMind）的 Dream Cycle / Query
> Sedimentation / Day-by-day roadmap，吸收 GPT Pro 方案的 CJK tokenizer / git worktree per agent /
> 平台细节 / MCP `readOnlyHint`，并补三方共同盲点（claim 抽取算法、onboarding 剧本、review queue
> 上限保护、多 agent 冲突剧本）。
>
> **基于** Karpathy [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) 思路。

---

## 0. 一句话定位

**WikiMind 是一个 local-first 的个人 / 小团队知识库**。原始资料保持只读、不可变；多个 agent 在一个由
Markdown 组成的 wiki 层上持续读写、交叉引用、定期 lint，把"一次性对话"沉淀为"可累积、可演化、可追溯
的知识图谱"。

它不是 RAG（wiki 是长期演化的工件，不是查询时拼接的碎片），不是 Obsidian / Notion 的替代品（产品价值
在 agent 协作协议而非编辑器），是一个**协议 + 工具链**。

---

## 1. 设计哲学（七条硬约束）

1. **Wiki 是 compounding artifact，不是 cache。** 每次 ingest / query / lint 都让 wiki 更值钱；
   不能让高质量对话答案消失在 chat history 里。
2. **三层结构是硬约束。** `raw/` 只读、`wiki/` 由 agent 维护、`schema/` 是合同。任何打破这个边界的
   操作都被 daemon 拒绝。
3. **Claim 是一等公民。** Wiki 里每个非平凡断言必须可追溯到 `raw/` 的具体位置（文件 + 锚点 + 内容
   hash）。无源 claim 必须显式标 `speculation` 或 `unverified`。
4. **Local-first，用户授权前提。** 所有读写在用户授权目录内，不绕过任何系统权限、不读他人数据、不规避
   审计。**这是产品边界。**
5. **Boring + small steps。** 优先 Markdown + SQLite + ripgrep + git；只有文件规模真的逼上来再上向量。
6. **Multi-agent 是协作不是竞赛。** Claude Code / Codex / Hermes / Cursor / Cline / MCP client 都通过
   同一个协议（review queue + worktree + change log）改 wiki，schema 是共同 SLA。
7. **可逆 > 智能。** 任何 agent 操作必须可 diff、可 revert、可解释。宁可慢也不让"看起来很聪明"的自动
   写入污染知识库。

---

## 2. 三层架构

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: Schema 合同 (schema/)                              │
│  AGENTS.md / CLAUDE.md / CODEX.md / HERMES.md / CURSOR.md   │
│  page-schemas.md / lint-rules.md                            │
│  → 定义 agent 怎么改 wiki；版本化；不可绕过                  │
├─────────────────────────────────────────────────────────────┤
│  Layer 2: Wiki (wiki/)                                       │
│  由 agent 维护的 Markdown 知识库                              │
│  index.md / log.md / claims/ / entities/ / concepts/ /       │
│  sources/ / _review/                                          │
│  → 完全可 git diff、可重写、可重建；100% 可逆                 │
├─────────────────────────────────────────────────────────────┤
│  Layer 1: Raw (raw/)                                         │
│  原始资料，只读、不可变、URI 唯一、内容 hash 可验证            │
│  inbox/ / imported/ / attachments/ / manifests/              │
│  → daemon 进程边界永不写入此目录                              │
└─────────────────────────────────────────────────────────────┘
```

### 2.1 raw/

- 只读。**daemon 永不写入。**
- 每个文件三件套：`sha256` + `size` + `mtime`，存 `sources` 表。
- 大文件（PDF / 图片 / 音频）走 git LFS 或外置存储（raw/ 留 hash 引用）。
- 修改外部源 → daemon 检测 mtime / hash 变化 → 标 `needs_reverify`，**不自动改 wiki**。

### 2.2 wiki/

- 由 agent 维护的 Markdown 文件，每个文件 frontmatter + 正文 + outbound `[[id]]` 链接。
- **文件名强制 ASCII kebab-case + lowercase**（跨平台规范，详见 [`docs/cross-platform.md`](docs/cross-platform.md)）。
- 五类 page：`claims/` / `entities/` / `concepts/` / `sources/` / `topics/`，frontmatter schema 见
  [`templates/page-schemas.md`](templates/page-schemas.md)。
- `_review/` 是写入暂存区——所有 agent 提议先到这里，user/master agent 决议后才进正式 wiki。

### 2.3 schema/

- `AGENTS.md` 是**底线契约**，所有 agent 必须读、必须遵守。
- 各 agent 的专属文件（`CLAUDE.md` / `CODEX.md` / `HERMES.md` / `CURSOR.md`）**只能加严**，不能放宽。
- 顶部强制 `schema_version`；agent session 开始必须 `agent_handshake`，版本不兼容 → daemon 拒绝写工具。

---

## 3. Claim 是一等公民

详细算法见 [`docs/claim-extraction.md`](docs/claim-extraction.md)。本节只列核心约束。

### 3.1 Claim 的最小三件套

```yaml
---
id: cl-2026-05-21-001
type: claim
status: supported | unverified | disputed | refuted
confidence: 0.0 – 1.0
sources:
  - raw_id: raw/inbox/karpathy-llm-wiki.md
    anchor: "#section-1-philosophy"      # heading / paragraph / char span
    quote: "every ingest, every query…"   # < 30 words 原文
    quote_hash: a7f2e3c1                  # sha256(quote) 前 8
    span: [14, 19]                        # line range
schema_version: 1.0
created_by: claude-code @ 2026-05-21 10:14
last_verified: 2026-05-21 10:18
---

## Wiki 是一个 compounding artifact

Karpathy 在 LLM Wiki gist 中明确主张…
```

### 3.2 强约束

| 规则 | 检查时机 | 违反结果 |
|---|---|---|
| 必须有至少 1 个 source | propose 时 | 拒绝；除非 `status: speculation=true` |
| `quote_hash` 必须可校验 | propose / lint / reverify | DRIFT 错误，claim 标 `needs_reverify` |
| `provenance_depth ≤ 1` | lint | claim 必须直接挂 raw/，禁止"claim 引用 claim 的 source" |
| `confidence` 与文字限定一致 | lint | 0.55 conf 的 claim 不能用"必定""一定"等强限定词 |
| Source 文件存在且 anchor 解析成功 | propose / lint | 拒绝；user 看到错误原因 |

### 3.3 Claim 状态机

```
        propose
           ↓
       unverified ─────────────────┐
           │                       │
           │ user accept           │ lint 发现 quote_hash mismatch
           ↓                       ↓
       supported ←─── reverify ── disputed
           │                       │
           │ 反证                  │ 反证
           ↓                       ↓
        refuted ←──────────────────┘
```

每次状态迁移 → 写 `log.md` + change_log + git commit。

---

## 4. 多 Agent 协议

详细见 [`docs/agent-protocol.md`](docs/agent-protocol.md)。本节给核心机制。

### 4.1 三件套

1. **Schema 是合同** — `AGENTS.md` 底线 + 专属文件加严。
2. **Review queue 是单一闸门** — 所有 agent 写入先到 `wiki/_review/`，daemon 串行化 commit。详见
   [`docs/review-queue-policy.md`](docs/review-queue-policy.md)。
3. **Git worktree 是物理隔离** — 每个 agent 用自己的 worktree，避免并发改同一文件的物理冲突。

### 4.2 角色与边界

| 角色 | 身份 | 能力 | 限制 |
|---|---|---|---|
| **User** | 唯一最终决策者 | 任意 | — |
| **Master agent**（可选） | user 委托的"主 agent"，如 Claude Code | 被授权自动 accept 一部分低风险 review | 不可改 schema |
| **Worker agent** | Codex / Hermes / Cursor / Cline / 自研 | 读 + propose；**不能直接写正式 wiki** | rate limit + schema 校验 |
| **Daemon** | `wikimindd` 系统进程 | 唯一执行 git commit；唯一发锁；唯一维护 `.wikimind/` 状态 | 不主动产生内容 |
| **Bridge** | `wikimind-bridge` 文件访问通道 | 读 raw / wiki；不参与决策 | 路径白名单 |

### 4.3 协议关键不变量

> 1. **只有 daemon 能 `git commit` 正式 wiki**（worker agent 在自己 worktree 里随便改，merge 必经 daemon）。
> 2. **任何 agent 写入都先到 `wiki/_review/`**，等 daemon 把 review 应用到正式 wiki。
> 3. **任何 agent session 开始必须 `agent_handshake`**，否则写工具拒绝。
> 4. **每条 change log 与 git commit 一一对应**；不存在"未记账"的写入。
> 5. **冲突剧本提前定义**——5 个典型冲突场景见 [`docs/conflict-scenarios.md`](docs/conflict-scenarios.md)。

### 4.4 写入流程（单条 propose 生命周期）

```
T0   agent 在自己 worktree 编辑 → 调 propose_*  → daemon 创建 review_id, 状态 pending
T0+  agent 调 request_review(r-0001..r-N, title="…")  → daemon 创建 bundle b-0001
T1   user 看 bundle b-0001:
       wikimind review show b-0001               # 综合 diff
       wikimind review accept r-0001 r-0003
       wikimind review reject r-0002 "title typo"
T2   daemon:
       - 把 r-0001, r-0003 patch 应用到正式 wiki/
       - 写 change_log（seq 严格单调递增）
       - git add + commit
       - update index.db
       - delete drafts from wiki/_review/
```

---

## 5. MVP 范围

详细 30 天 day-by-day roadmap 见 [`docs/roadmap-30d.md`](docs/roadmap-30d.md)。本节给边界。

### 5.1 MVP 必做（D1–D30）

- [x] `wikimind init` / `status` / `ingest` / `query` / `review` / `lint` / `revert` CLI
- [x] MCP server（stdio），20 个工具，read-only 工具带 `readOnlyHint`
- [x] SQLite FTS5 索引（**CJK 用 trigram 或 jieba tokenizer，不能用默认 unicode61**，详见 [`docs/cjk-tokenizer.md`](docs/cjk-tokenizer.md)）
- [x] Review queue 单一闸门 + bundle 归并 + 上限保护（>50 pending 停接新 propose）
- [x] Git worktree per agent + advisory lock 双重协调
- [x] `agent_handshake` + `schema_version` 强制版本检查
- [x] Claim quote_hash 反验证（DRIFT 错误）
- [x] Lint 规则：通用 8 条 + claim 专项 4 条 + 结构 1 条（13 条，完整定义见
      [`templates/lint-rules.md`](templates/lint-rules.md)）
- [x] Dream Cycle 周期性维护（audit → consolidate → evolve → report；默认 24h 一次）
- [x] Query Sedimentation（高质量问答自动回写为 wiki 草稿，进 review queue）
- [x] 跨平台：macOS + Windows，文件名 ASCII kebab-case 强制
- [x] `wikimind demo` 5 分钟 onboarding 剧本（详见 [`docs/onboarding.md`](docs/onboarding.md)）
- [x] 失败 playbook（9 类回滚命令，详见 [`docs/failure-playbook.md`](docs/failure-playbook.md)）

### 5.2 MVP 不做

- ❌ 云端中心化存储
- ❌ 默认全盘扫描
- ❌ 绕过系统权限 / 自动读未授权目录
- ❌ 默认 embedding / 向量库（MVP 关；W4 可选本地 `bge-small`）
- ❌ 实时协同编辑
- ❌ 不经审查直接把每次问答写成事实页
- ❌ Web dashboard（v0.2）
- ❌ 移动端（v1.5）
- ❌ 多用户协作（v1）
- ❌ 跨设备 CRDT 同步（v2.0）

### 5.3 MVP 接入面（MCP）

| Agent | MVP 支持 | 怎么接 |
|---|---|---|
| Claude Code | ✅ 优先 | MCP stdio + `templates/CLAUDE.md` |
| Codex CLI | ✅ 优先 | MCP stdio + `templates/CODEX.md` |
| Hermes | 🟡 适配指南 | `templates/HERMES.md` + CLI bridge |
| Cursor | 🟡 适配指南 | `templates/CURSOR.md` + AGENTS.md rules |
| Cline / OpenCode | 🟡 兼容 | 走通用 MCP，但 prompt 模板未深度调优 |

> "优先"意味着 D14 demo 必须跑通；"适配指南"意味着提供模板但不在 D14 demo 中强制验证。

---

## 6. 技术选型

| 维度 | 选择 | 理由 |
|---|---|---|
| **Daemon 语言** | Go | 跨平台单二进制、并发好、watcher 库成熟 |
| **Ingest worker** | Python | PDF/OCR/whisper 生态成熟，与 daemon 通过 JSON-RPC over Unix socket / named pipe 通信 |
| **存储** | Markdown + git | 可读、可 diff、Obsidian 兼容、100% 可逆 |
| **索引** | SQLite FTS5 + sqlite-vec（可选） | 零运维、单文件、100-10k 文件够用 |
| **CJK 分词** | trigram（保守）或 jieba（可选） | **不能用默认 unicode61**，详见 [`docs/cjk-tokenizer.md`](docs/cjk-tokenizer.md) |
| **搜索** | FTS5 BM25 + ripgrep 兜底 | 小规模快、可解释 |
| **并发控制** | Git worktree per agent + advisory lock + daemon 单线程 commit | 物理隔离 + 协调锁 + 串行化三层 |
| **跨平台** | 强制 POSIX 内部路径 + ASCII 文件名 + frontmatter 中文 | 一条 lint 规则压死大小写/编码问题 |
| **Watcher** | FSEvents (macOS) / ReadDirectoryChangesW (Windows) / inotify (Linux) | 原生系统级 |
| **MCP** | stdio transport，20 工具（9 读 / 6 写提案 / 3 管理 / 2 元） | 详见 [`docs/mcp-tools.md`](docs/mcp-tools.md) |
| **Git** | Auto-commit（每次 accept 一次 commit），revert 兜底，无强制 push | 最小化用户学习曲线 |
| **凭证** | OS keychain（macOS Keychain / Windows Credential Manager） | 不进配置文件明文 |

---

## 7. 与 RAG / Obsidian / Notion 的区别

| 维度 | 传统 RAG | Obsidian | Notion | **WikiMind** |
|---|---|---|---|---|
| 知识演化 | 无状态查询 | 纯人工维护 | 人工 + 简单 AI | **多 agent + 协议**编译 |
| Source 追溯 | embedding chunk | 双链 + 引用 | 评论 / 链接 | **Claim + quote_hash** 强校验 |
| 防幻觉 | 仅 retrieval | 无 | 弱（AI 写就写） | **五层防御**（claim 校验 / confidence / review queue / lint / git revert） |
| 多 agent 协作 | 不支持 | 无 | 不支持 | **协议 + worktree + handshake** |
| 跨平台 | 取决于 RAG 服务 | ✅ | 云端 | ✅ + 平台细节 playbook |
| 离线 | ❌ | ✅ | ❌ | ✅ |
| 可逆 | 部分 | git plugin | 历史版本（云） | **git 是真理源** + 完整 revert playbook |

**核心定位**：WikiMind 不是"更好的 Obsidian"，是"agent + 知识库的协作协议与工具链"。

---

## 8. 五层防幻觉防御

| 层 | 机制 | 责任方 |
|---|---|---|
| 1 | **Claim source 校验** — `quote_hash` 反验证；无源 propose 拒绝 | Agent → daemon |
| 2 | **显式信心度** — `confidence` + `status` + lint 检测文字限定与 conf 不一致 | Agent → Lint |
| 3 | **Review queue 单一闸门** — 默认所有写入进 queue，user 拍板 | User |
| 4 | **Lint 反幻觉规则** — orphan / unverified / contradictions / duplicate / drift | Lint → User |
| 5 | **Git revert 兜底** — 任何 commit 可一键回退；依赖图标记级联影响 | User |

详细的失败模式与回滚见 [`docs/failure-playbook.md`](docs/failure-playbook.md)。

---

## 9. 文档导航

| 文档 | 主题 | 批次 |
|---|---|---|
| **本文** | 主 spec | Wave 1 |
| [`README.md`](README.md) | 整合说明 + 怎么读 | Wave 1 |
| [`docs/claim-extraction.md`](docs/claim-extraction.md) | Claim 粒度判定算法 + 案例 | Wave 1 |
| [`docs/onboarding.md`](docs/onboarding.md) | `wikimind demo` 5 分钟剧本 | Wave 1 |
| [`docs/review-queue-policy.md`](docs/review-queue-policy.md) | Review queue 上限保护 + 归并 | Wave 1 |
| [`docs/architecture.md`](docs/architecture.md) | 详细架构图 + 数据流 | Wave 2 |
| [`docs/agent-protocol.md`](docs/agent-protocol.md) | 协议细节 + 握手 + worktree | Wave 2 |
| [`docs/mcp-tools.md`](docs/mcp-tools.md) | 20 个 MCP 工具 + JSON schema | Wave 2 |
| [`docs/cross-platform.md`](docs/cross-platform.md) | macOS / Windows 细节 | Wave 2 |
| [`docs/filesystem-access.md`](docs/filesystem-access.md) | 文件访问模型（5 路径 / 读写分离） | 整合补遗 |
| [`docs/cjk-tokenizer.md`](docs/cjk-tokenizer.md) | 中文分词选型与配置 | Wave 2 |
| [`docs/dream-cycle.md`](docs/dream-cycle.md) | 周期性维护 audit→consolidate→evolve→report | Wave 2 |
| [`docs/query-sedimentation.md`](docs/query-sedimentation.md) | 高质量问答回写 | Wave 2 |
| [`docs/conflict-scenarios.md`](docs/conflict-scenarios.md) | 多 agent 冲突 5 剧本 | Wave 3 |
| [`docs/failure-playbook.md`](docs/failure-playbook.md) | 9 类失败回滚 + 依赖图 | Wave 3 |
| [`docs/risks.md`](docs/risks.md) | 风险清单（26 条） | Wave 3 |
| [`docs/roadmap-30d.md`](docs/roadmap-30d.md) | D1–D30 day-by-day | Wave 3 |
| [`docs/engineering-decisions.md`](docs/engineering-decisions.md) | W1 前置工程决策（IPC / MCP / migration / Go 模块） | 工程准备 |
| `templates/*.md` | AGENTS / CLAUDE / CODEX / HERMES / CURSOR / page-schemas / lint-rules | Wave 3 |
| `examples/*.md` | directory-tree + demo-walkthrough | Wave 3 |

---

## 10. 验收（MVP 出口判据）

D30 时，下列**全部**为真，v0.1 才发布：

1. **本人每天用** — dogfooding 30 天，ingest ≥ 30 篇资料
2. **敢相信内容** — 任一 claim 都能 30 秒内回到 source 原文段落
3. **多 agent 不打架** — Claude Code + Codex CLI 同时跑 demo 无冲突，change_log 1:1 git commit
4. **跨平台跑通** — macOS + Windows 各装一遍，全套 CLI + MCP demo 通过
5. **冷启动 5 分钟** — `wikimind demo` 命令完整跑通，新用户 5 分钟内看到第一个 ingest → review → wiki 循环
6. **review 不堆积** — review queue 上限保护机制启用，user 每周 review 时间 ≤ 30 分钟
7. **失败可恢复** — failure-playbook 9 类命令全部测试通过
8. **CI 全绿** — macOS + Windows runner，单测 + e2e 全绿

如果其中任何一条不满足 → MVP 不发布，延期或缩范围。

---

## 11. 三方案血统标注

为了承认知识来源、便于回溯：

| WikiMind v2 设计点 | 主要来源 |
|---|---|
| 三层架构 + Claim 一等公民 + Schema 合同 | 方案 A、B、GPT Pro 共识（追溯 Karpathy gist） |
| Review queue + advisory lock + change log + agent_handshake + quote_hash | **方案 A** |
| Schema_version + breaking_changes + 21 条风险清单 + templates 可复用 | **方案 A** |
| **Git worktree per agent**（物理隔离） | **GPT Pro** |
| **CJK tokenizer 警告 + 选型** | **GPT Pro**（独家） |
| **MCP `readOnlyHint` 标签** | **GPT Pro** |
| 平台细节（APFS / NTFS / CFA / OneDrive 占位符 / Windows 长路径） | **GPT Pro** |
| **Dream Cycle**（周期性主动维护） | **方案 B**（独家） |
| **Query Sedimentation**（问答回写） | **方案 B**（独家） |
| **Day-by-day D1-D30 roadmap** | **方案 B**（A 是周维度） |
| **Claim 粒度判定算法**（10+ 案例） | **新增**（三方共同盲点） |
| **`wikimind demo` 5 分钟 onboarding 剧本** | **新增**（三方共同盲点） |
| **Review queue 上限保护 + bundle 归并** | **新增**（三方共同盲点） |
| **多 agent 冲突 5 剧本** | **新增**（三方共同盲点） |
| **依赖图级联回滚** | **新增**（扩展方案 A 的 revert） |

---

## 一句话总结

> WikiMind 把"agent 维护 wiki"从"容易污染和混乱"工程化成"可追溯、可审计、可协作"的协议 + 工具链。
> 它的产品价值不在功能多，而在"我每天都用、我敢信任、它越用越值钱"。
