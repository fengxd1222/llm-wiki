# LLM Wiki：local-first 个人知识库系统 — 完整产品方案

> 基于 Andrej Karpathy 的 *LLM Wiki* 思路（[gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)），
> 延展为一个工程可落地、跨 macOS/Windows、可与多 agent 联动的个人知识库系统。
>
> 本文是主报告。详细的技术架构、跨平台访问、agent 协议、MCP 工具、风险、30 天计划、研究问答见 `docs/`；
> 可直接复制使用的 agent 指令模板见 `templates/`；目录结构样例见 `examples/`。

---

## 0. 一句话定位

**LLM Wiki 是一个 local-first 的个人/小团队知识库。原始资料保持只读、不可变；agent 在一个由 Markdown
组成的 wiki 层上持续读写、交叉引用、定期 lint，把"一次性的对话"沉淀为"可累积、可演化、可追溯的知识图谱"。**

它不是 RAG，因为 wiki 是一个**长期演化的产物**，而不是查询时临时拼接的碎片。它不是 Notion / Obsidian 的替代品，
而是把 Obsidian 当作"IDE"、把 agent 当作"程序员"、把 wiki 当作"代码库"的协作模式的**协议 + 工具链**。

---

## 1. 设计哲学

延续 Karpathy 原文的核心观点，并明确我们的工程化补充：

1. **Wiki 是 compounding artifact，不是 cache。** 每一次 ingest、每一次 query、每一次 lint，都应该让 wiki
   **更值钱**——不能让"高质量的对话答案"消失在 chat history 里。
2. **三层结构是硬约束：**
   - `raw/` 只读、不可变、URI 唯一、内容 hash 可验证；
   - `wiki/` 完全由 agent 维护、可重写、可 git diff；
   - `schema/` 是合同（`AGENTS.md`、`CLAUDE.md`、`HERMES.md` 等），定义"agent 怎么改 wiki"。
3. **Claim 是一等公民。** Wiki 里每一个非平凡断言都应该可追溯到 `raw/` 的具体段落（文件 + 锚点 + 内容 hash）。
   无源 claim 必须被显式标记 `unverified` 或 `speculation`，agent 不得隐瞒不确定性。
4. **Local-first，用户授权前提。** 所有读写都在用户授权目录内，通过用户自己的终端能力（zsh / PowerShell /
   shell command / MCP server / local daemon）进行。**不绕过任何系统权限、加密机制或企业管控策略；不读取他人数据；
   不规避任何审计。** 这是产品边界。
5. **Boring + small steps。** 优先用 Markdown + SQLite + ripgrep + git；只有当文件规模真的逼上来再上 vector DB。
6. **Multi-agent 是协作不是竞赛。** Claude Code、Codex、Hermes、Cursor、OpenCode、CLI agent、MCP client 等
   都通过同一个 protocol（lock + change log + review queue）改 wiki，schema 文件是它们共同的 SLA。
7. **可逆 > 智能。** 任何 agent 操作必须是可 diff、可 revert、可解释的。宁可慢一点、宁可多一步 review，也不要
   让"看起来很聪明的自动写入"污染知识库。

---

## 2. 产品定位与目标用户

### 2.1 定位

| 维度 | 取值 |
|---|---|
| 形态 | 本地优先的 Markdown 知识库 + agent 协议 + CLI/MCP 工具链 |
| 安装 | 单机部署（macOS / Windows / Linux）；可选 git remote 同步 |
| 对话入口 | 任意支持文件系统访问或 MCP 的 agent；非锁定单一 LLM 厂商 |
| 数据所有权 | 100% 在用户磁盘上；不强制云存储 |
| 商业化（远期） | 开源 core + 可选的同步/索引云服务，但 core 永远能 100% 本地跑 |

### 2.2 目标用户画像

1. **研究型工作者**（PM / 分析师 / 投资人 / 学者 / 法务）：长期跟踪一个或多个话题，需要把数百篇文章、报告、
   会议纪要、聊天截图整合成可推理的知识库。
2. **重度 LLM 使用者**：日常用 Claude Code、Codex 这类 agent 干活，已经在 Obsidian / git 里维护笔记，对
   "对话内容白白蒸发"感到不爽。
3. **个人成长型用户**：跟踪健康、心理、目标、读书笔记，希望 LLM 帮自己做长期画像，而不是每次都重新解释自己。
4. **小团队 wiki 维护者**：5–20 人团队，没有专职文档维护人员，希望 LLM 把 Slack / 会议 / 邮件沉淀为可查
   的内部 wiki（带 human review）。

非目标用户：

- 需要协作编辑、权限管理、SSO 的中大型企业知识库（用 Notion / Confluence）。
- 不接受 Markdown / 不接受 git 概念的轻度用户（用 NotebookLM / ChatGPT Projects）。

### 2.3 价值主张（与 RAG / NotebookLM / Obsidian 对比）

| 能力 | 传统 RAG / NotebookLM | 纯 Obsidian | **LLM Wiki（本方案）** |
|---|---|---|---|
| 资料保留 | ✓ | ✓ | ✓（`raw/` 不可变） |
| 实时问答 | ✓ | ✗ | ✓（多 agent 接入） |
| 跨文档综合 | △（每次重做） | 人工 | ✓（agent 持续维护） |
| 知识沉淀 | ✗（chat 易丢） | 人工 | ✓（query → 自动归档为 wiki page） |
| Claim 可追溯 | △（引用） | 人工 | ✓（claim 是一等公民，含 source anchor + hash） |
| 多 agent 协作 | ✗ | ✗ | ✓（lock / change log / review queue） |
| 数据所有权 | △（云） | ✓ | ✓ |
| 跨平台 | ✓ | ✓ | ✓（macOS + Windows + Linux） |
| Lint / 自检 | ✗ | 人工 | ✓（孤儿、断链、矛盾、陈旧） |

---

## 3. 核心用户故事（精选 10 条）

> 用 INVEST 原则写。`As a … I want … so that …` 风格。

1. **作为研究者**，我想把一篇 PDF 论文丢进 `raw/inbox/`，让 agent 自动产出 summary、entity、concept 页面
   并更新 `index.md` 和 `log.md`，这样我可以一边读、一边在 Obsidian 里跟着看交叉引用。
2. **作为重度 Claude Code 用户**，我想让 Claude Code 在写代码间隙顺手维护我个人的"决策日志 wiki"，
   把它跟我项目里的 `CLAUDE.md` 解耦，避免污染项目记忆。
3. **作为多模型用户**，我想 Claude Code 早上写的 wiki、Codex 下午能直接读懂，并按照同一个 schema 继续写，
   这样我不被任何单一 LLM 厂商锁定。
4. **作为不信任 agent 的人**，我想任何 wiki 写入都先进入 `wiki/_review/`，由我确认或一键 reject，
   而不是直接覆盖正式页面。
5. **作为 wiki 维护者**，我想每周让 agent 自动跑一次 `wiki lint`，输出孤儿页、断链、矛盾 claim、陈旧
   结论的清单，并把"需要补充的 query"作为新任务塞进 `wiki/_inbox/`。
6. **作为问问题的人**，我想问"X 和 Y 的关系"时 agent 先读 `index.md`，再读相关页面，再回溯 source，最后
   给出带 citation 的答案，**并问我"要不要把这个答案归档为新页面"**。
7. **作为 Mac 用户**，我想在 Windows 笔记本上接着工作时，wiki 完全可用，路径、编码、大小写不出问题。
8. **作为安全敏感用户**，我想所有 agent 写入都在 git 里、所有 source 都有内容 hash、所有 claim 都可追溯，
   即使 agent 撒谎，我也能在 30 分钟内定位并回滚。
9. **作为 100 篇资料阶段的用户**，我不想跑任何 vector DB；当我攒到 5000 篇时，希望系统自动建议我打开
   embedding/向量索引并提供迁移命令。
10. **作为 MCP user**，我想任何符合 MCP 协议的 client（包括 Cursor / Continue / Cline / 自研 agent）
    都能通过同一套 `llm-wiki` MCP server 操作我的知识库。

---

## 4. MVP 范围（4 周）

**目标**：单用户、单平台先跑通；100–1000 篇资料级别；只覆盖 ingest / query / lint 三个核心 loop。

### 4.1 必须有（MUST）

| # | 功能 | 形态 |
|---|---|---|
| M1 | 三层目录骨架（`raw/`、`wiki/`、`schema/`、`index.md`、`log.md`） | 标准化目录 |
| M2 | `llmwiki` CLI（init / ingest / query / lint / status / log） | Python or Go 二进制 |
| M3 | MCP server（暴露 read_page / search / list_index / propose_edit / log_append 等只读+提案工具） | stdio MCP |
| M4 | 文件 watcher + 增量索引（mtime + sha256） | FSEvents (macOS) + ReadDirectoryChangesW (Windows) |
| M5 | 全文搜索（SQLite FTS5 + ripgrep 兜底） | 本地索引 |
| M6 | 跨平台 path/encoding 标准化（slash 统一、UTF-8、大小写敏感约定） | core util |
| M7 | claim 抽取与 source citation（frontmatter schema + lint 校验） | schema + linter |
| M8 | Review queue（所有写入先入 `wiki/_review/`，CLI/MCP 提交后才 merge） | git-based |
| M9 | `AGENTS.md` / `CLAUDE.md` / `HERMES.md` 三套指令模板 | docs |
| M10 | git 自动 commit + 每条 change log 与 commit 对齐 | hooks |

### 4.2 应该有（SHOULD）

- Obsidian vault 兼容（wiki/ 可被 Obsidian 直接打开）
- PDF → Markdown ingest（用 `pdftotext` 兜底）
- 图片 OCR ingest（macOS 用 `shortcuts`/Vision、Windows 用 PowerToys/Tesseract）
- 简易 dashboard（`llmwiki status` 输出健康度）

### 4.3 可以有（COULD）

- Embedding 索引（用 SQLite + sqlite-vec 或 LanceDB，**仅作可选**）
- Marp 幻灯片导出（query → 答案 → 幻灯片）
- Web clipper 集成（Obsidian Web Clipper / Wallabag）

### 4.4 暂不做（WON'T）

> 这一节是**产品边界**的硬声明，写在合同上。

- ❌ 多人协作 / 权限管理 / SSO（用 git remote 凑合）
- ❌ 任何形式的数据上传到第三方云（除非用户自己接 git remote）
- ❌ 绕过系统加密、绕过 FDA / ACL、读取非用户授权目录
- ❌ 试图自动"修复" agent 错误以隐藏失败（任何错误必须显式 log + 失败原因可见）
- ❌ 移动端 app（v2 才考虑）
- ❌ 内置 LLM 推理（永远调用外部 agent / API）
- ❌ 锁定单一 LLM 厂商或单一 IDE
- ❌ 自动 web 抓取（必须由用户显式触发或 Web Clipper 落地后才 ingest）

---

## 5. 信息架构与目录结构（高层）

详细样例见 `examples/directory-tree.md`，这里只给"分层与命名约定"。

```
my-wiki/                       # 一个 wiki vault = 一个 git repo
├── raw/                       # 第一层：只读 source of truth
│   ├── inbox/                 # 新进的、未处理的资料
│   ├── articles/              # 网页文章（含 obsidian web clipper 输出）
│   ├── papers/                # 论文 PDF
│   ├── transcripts/           # 会议/播客转录
│   ├── notes/                 # 个人手记（user 写的，不是 agent 写的）
│   ├── images/                # 图片
│   └── assets/                # 二进制、嵌入资源
├── wiki/                      # 第二层：agent 维护的 markdown
│   ├── index.md               # 全局索引（agent 每次 ingest 都更新）
│   ├── log.md                 # 时间线（append-only）
│   ├── entities/              # 人、组织、地点、产品
│   ├── concepts/              # 概念、术语、理论
│   ├── claims/                # 一等公民：每个 claim 一个文件
│   ├── sources/               # 每个 raw 文件对应一个 source page
│   ├── topics/                # 主题综述、思路 evolving
│   ├── queries/               # 重要 query 沉淀（user 决定归档的）
│   ├── _review/               # 待人工 review 的草稿
│   └── _inbox/                # agent 自己列的"该补的 query / source"
├── schema/                    # 第三层：合同
│   ├── AGENTS.md              # 通用指令（OpenAI Codex / Hermes 等）
│   ├── CLAUDE.md              # Claude Code 专用
│   ├── HERMES.md              # Hermes 专用
│   ├── page-schemas.md        # frontmatter & 页面模板
│   └── lint-rules.md          # lint 规则
├── .llmwiki/                  # 系统文件（git ignore 大部分）
│   ├── config.toml            # 全局配置
│   ├── index.db               # SQLite FTS5 索引
│   ├── locks/                 # 文件级 lock
│   ├── change-log.jsonl       # 机器可读 change log
│   ├── review-queue.jsonl     # review queue 状态
│   ├── embeddings.db          # 可选 vector 索引
│   └── cache/                 # 缓存
└── .gitignore
```

### 关键约定

- **文件命名**：`kebab-case`，全 ASCII（避免 Windows/macOS 大小写差异问题）。中文标题放 frontmatter。
- **frontmatter 必填**：`id`（短 ULID）、`type`、`created`、`updated`、`sources`、`status`。
- **wiki link 用 `[[id]]`** 而不是 `[[slug]]`（避免 rename 破坏链接）。
- **每个 `raw/` 文件**对应一个 `wiki/sources/<id>.md`，记录 hash + 元数据 + 第一手摘要。
- **claim 文件**含 `claim`、`evidence[]`、`source_anchors[]`、`confidence`、`contradicts[]` 字段。

---

## 6. 数据模型（高层）

详细字段见 `templates/page-schemas.md`，这里只列实体类型与关系：

```
SourcePage   1 ─── n  ClaimPage          # 一个 source 抽出多个 claim
ClaimPage    n ─── n  EntityPage         # claim 涉及多个 entity
ClaimPage    n ─── n  ConceptPage        # claim 涉及多个 concept
ClaimPage    1 ─── n  Contradiction      # 矛盾通过 contradicts 字段
EntityPage   n ─── n  EntityPage         # 通过 relations[] 表达
TopicPage    n ─── n  ClaimPage          # 主题聚合 claim
QueryPage    n ─── n  ClaimPage          # 沉淀的 query 引用 claim
```

数据存放方式：

- **真理来源**：Markdown 文件本身（git 管理）。
- **二级索引**：`.llmwiki/index.db`（SQLite FTS5）。这是**派生数据**，可随时从 markdown 重建。
- **change log**：`.llmwiki/change-log.jsonl`（追加，与 git commit 一一对应）。

---

## 7. 关键设计决策（取舍）

| 决策 | 选择 | 取舍 |
|---|---|---|
| 数据格式 | Markdown + YAML frontmatter | 取舍掉数据库结构化的查询能力，换 git diff、可读性、Obsidian 兼容 |
| 索引存储 | SQLite FTS5 / sqlite-vec | 单文件、零运维；不上 Postgres/Qdrant |
| 文件 watcher | macOS FSEvents / Windows ReadDirectoryChangesW；polling 兜底 | 平台原生 API 性能好；polling 用于挂载点、网络盘 |
| 搜索 | BM25 (SQLite FTS5) + ripgrep + 可选 embedding | 小规模时 FTS 够用；大规模再加 hybrid |
| Agent 通道 | MCP（主） + CLI（兜底） + 直接文件系统读写（最终兜底） | 不锁定单一 agent；最大兼容性 |
| 写入安全 | 全部进 review queue，git 单线程 commit | 牺牲一点速度，换可审计 |
| 跨平台 | 路径强制 POSIX 风格内部表示；文件名 ASCII + frontmatter 中文 | 牺牲文件名表达力，换零跨平台 bug |
| Claim | 一等公民、独立文件 | 牺牲 markdown 简洁性，换可推理、可审计 |
| 锁 | 文件级 advisory lock（`.lock` sentinel + git） | 不上分布式锁；单机够用 |
| LLM 推理 | 不内置；只调外部 agent / API | 不维护推理；让用户用最熟悉/最便宜的 agent |

---

## 8. 核心工作流（4 个 cycle）

### 8.1 Ingest cycle

1. 用户把文件丢进 `raw/inbox/`（或 Obsidian Web Clipper / drag-drop / `llmwiki ingest <file>`）。
2. Watcher 检测到新文件 → 计算 sha256 → 写入 `.llmwiki/index.db` 的 `sources` 表（status=pending）。
3. Agent 被触发（或用户在 Claude Code 里手动 `/ingest`）：
   - 读取 source，做"key takeaways"对话；
   - 在 `wiki/_review/` 生成：1 个 source page、N 个 claim、M 个 entity 更新提议、K 个 concept 更新提议；
   - 更新 `index.md` 候选 diff；
   - 在 `log.md` 写一条 `[<date>] ingest | <title>`。
4. 用户 review → CLI `llmwiki accept <review-id>` → merge 到正式 wiki、git commit。
5. 拒绝项进 `.llmwiki/rejects.jsonl` 供 agent 学习。

详细 ASCII 时序图见 `docs/architecture.md`。

### 8.2 Query cycle

```
user: "X 和 Y 的关系？"
  │
  ▼
agent reads wiki/index.md            (cheap, 必读)
  │
  ▼
agent shortlists pages by topic      (BM25 / vector)
  │
  ▼
agent reads selected wiki pages      (深度阅读)
  │
  ▼
agent reads source anchors (raw/)    (回溯)
  │
  ▼
agent synthesizes answer with citations
  │
  ▼
agent asks user: "归档这个回答为 wiki/queries/<id>.md ？"
  │ yes
  ▼
file back to wiki (review queue)
```

### 8.3 Lint cycle（每周/每月）

| Lint 项 | 检查内容 | 修复方式 |
|---|---|---|
| 孤儿页 | 无入链页面 | 标记 → 建议合并或删除 |
| 断链 | `[[id]]` 指向不存在页面 | 自动修复或建议新建 |
| 矛盾 claim | `contradicts[]` 字段或 NLI 检查 | 进入 `wiki/_review/contradictions/` |
| 陈旧结论 | 引用的 source 已被 newer source 覆盖 | 标 `stale: true` |
| 缺引用 | 非平凡断言但无 `sources[]` | 标 `unverified` |
| 重复实体 | 别名/同义词 | 进入 `wiki/_review/merge-suggestions/` |
| Schema 违规 | frontmatter 必填缺失 | 自动补全或拒绝 commit |

### 8.4 Dream cycle（可选，离线时跑）

灵感来自 Karpathy 文章评论里有人提到的"睡眠期巩固"：在用户离线时 agent 自动跑：

1. 重读最近 N 篇 source；
2. 寻找跨主题的隐含连接；
3. 在 `wiki/_inbox/dream-<date>.md` 生成"可探索的新 query"；
4. 不直接写正式 wiki，**只投递建议**。

这个 cycle 必须可关、可调度（cron / launchd / Task Scheduler）。

---

## 9. 多 agent 协作模型（一句话）

> **schema 是合同；review queue 是单一写入点；change log 是审计；git 是 source of truth；
> MCP / CLI 是接入方式；锁是临时排他。**

详见 `docs/agent-protocol.md`。

每个 agent 接入步骤：
1. 启动时读取 `schema/AGENTS.md`（+ 自己专属的 `CLAUDE.md` / `HERMES.md`）。
2. 通过 MCP 或 CLI 拿到当前 wiki 状态。
3. 提案写入 → review queue → 用户/master agent 决议。
4. 任何直接写文件的能力**默认关闭**；MVP 阶段所有写入都走 review。

---

## 10. 产品路线图

### v0.1 MVP（4 周）

> 见 `docs/roadmap-30d.md`，按周拆分。

- 三层骨架、CLI、MCP server、watcher、SQLite FTS5、review queue、git 集成、3 份 instruction 模板。
- 平台覆盖：macOS 主、Windows 次（CI 跑通即可）。
- 规模目标：1k source 文件、5k wiki page，索引 < 5s，全文搜索 P95 < 200ms。

### v0.2（再 4 周）

- Windows 一等公民（FileSystemWatcher、长路径 `\\?\` 兼容、PowerShell module）；
- Obsidian vault 兼容验证；
- PDF / 图片 OCR ingest；
- claim 矛盾检测（基于 NLI 模型，可选）；
- 简易 web dashboard（localhost, 只读）；
- 多 agent 同步演示（Claude Code + Codex 同时工作）。

### v1.0（3 个月）

- 规模：10k source 文件、100k wiki page。
- Hybrid search（BM25 + embedding rerank），可选 LanceDB / sqlite-vec / qmd 集成；
- Dream cycle 默认开启；
- Review queue UI（TUI or web）；
- Linux 一等公民；
- 性能 SLA：增量索引 P95 < 500ms / file，全文搜索 P95 < 500ms。

### v1.5（6 个月）

- 移动端只读 viewer（iOS / Android）；
- 多 vault 联合查询；
- 团队模式（git PR-based review，5–20 人）；
- 与 Notion / Linear / Slack 的可选 ingest 适配器（用户授权前提下）。

### v2.0（12 个月）

- 规模：100k source / 1M wiki page；
- 分片索引、并发 ingest；
- 跨设备同步（git + CRDT for active sessions）；
- "记忆模块"：长期画像（人格、偏好、目标）；
- 一个开源的、可被多 agent 共享的"个人 OS 入口"。

---

## 11. 与本仓库其他文档的关系

| 你想看 | 去哪 |
|---|---|
| 详细技术架构 + 性能方案 + 选型对比 | `docs/architecture.md` |
| macOS / Windows 跨平台文件访问 | `docs/local-file-access.md` |
| 多 agent 协作协议、锁、change log、review、防幻觉 | `docs/agent-protocol.md` |
| MCP tools 详细 schema | `docs/mcp-tools.md` |
| 风险清单 + 缓解措施 | `docs/risks.md` |
| 30 天逐周开发计划 | `docs/roadmap-30d.md` |
| 8 个重点研究问题的深度回答 | `docs/research-qa.md` |
| 完整目录结构样例 | `examples/directory-tree.md` |
| Agent 指令模板（可直接复制使用） | `templates/AGENTS.md`、`templates/CLAUDE.md`、`templates/HERMES.md` |
| frontmatter schema + 页面模板 | `templates/page-schemas.md` |

---

## 12. 终极判据

> 当用户 6 个月后回头看自己的 wiki 时——
>
> 如果他能在 30 秒内回到任何一个 claim 的 source；
> 如果他能从任何一个 entity 出发走到 5 跳之外的相关概念；
> 如果他能让一个新的 agent（昨天还没接触过这个 wiki）在 5 分钟内开始有用地工作；
> 如果他敢相信 wiki 里的每一句"事实"——
>
> 那么这个产品就成立了。

否则我们只是在做另一个会被遗忘的笔记软件。
