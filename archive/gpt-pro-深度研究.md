# 基于 Karpathy LLM Wiki 的 Local-First 个人知识库产品方案

## 方案结论与产品定位

我建议把这个产品定义成**“由多个 Agent 共同维护的、本地优先的 Markdown Wiki 操作系统”**，而不是“又一个 RAG 应用”。Karpathy 在其 LLM Wiki 思路里，明确把系统拆成三层：不可变的 raw sources、由 LLM 持续维护的 wiki、以及约束 Agent 行为的 schema / instructions；同时把工作流分成 ingest、query、lint，并要求 `index.md` 先于正文页面被读取，`log.md` 作为时间序列账本持续记录演化过程。这套思路的本质不是“查询时临时检索”，而是“把一次次查询和摄取转化成可复用、可生长、可审计的知识工件”。citeturn31view0turn31view3turn32view0turn32view1turn32view2

因此，产品定位应当非常明确：**服务于高强度知识工作者和多 Agent 编排用户**，例如研究者、独立开发者、创作者、投资/咨询从业者、以及已经在使用 Claude Code、Codex、Cursor、OpenCode、Hermes、命令行代理和 MCP 客户端的人。Codex CLI 明确支持在本地终端读取、修改和运行代码；Claude Code 是终端中的 agentic coding tool，并支持 MCP；Cursor 既支持 AGENTS.md 也支持 Project / User / Team Rules；OpenCode 支持 AGENTS.md、指令文件、watcher 和 MCP；Hermes 则原生围绕可持续学习的 skills 和 memory 工作。这意味着“同一套本地知识库被多个 Agent 共用”不是想象出来的未来能力，而是现在就可以围绕现有接口实现的工程目标。citeturn26view15turn26view11turn25view0turn23view4turn26view0turn23view14turn26view4turn23view5

和普通 RAG 相比，这个方向的优势有三点。第一，**知识会沉淀**：Karpathy 明确把 LLM Wiki 视为从“查询即丢失”的 stateless RAG，转到“持续更新持久图谱”的 stateful artifact。第二，**知识结构可维护**：`index.md`、实体页、概念页、claims、sources、log 让 Agent 不只是找片段，而是维护结构。第三，**一次高质量查询可以回写成新的页面**，从而让系统越用越值钱。它的代价也同样明确：会有写入污染风险、结构漂移风险、重复概念和同义词问题、以及多 Agent 并发修改冲突。也因此，它不能做成“允许所有 Agent 直接任意写 vault”的自由模式，而必须做成**“统一协作协议 + 受控写入网关”**。citeturn32view3turn32view4turn32view2

产品形态上，我建议你的**MVP 只做四件事**：一个 Git 仓库形态的知识库、一个本地 daemon、一个 CLI、一个 MCP server。桌面 App 和 Obsidian 插件都放到后面。原因很简单：你要兼容的主战场本来就已经是终端、IDE、MCP host 和本地 agent；MCP 本身就是为“把本地文件、数据库、工具和工作流标准化暴露给 AI 应用”而设计，协议还要求显式用户同意和明确的工具授权，这正好符合你强调的“用户本人授权的本地文件访问能力”。citeturn23view5turn23view6turn33view0

MVP 的核心用户故事应该只有三条。其一，**我把文件放进一个授权目录，系统能可靠、快速地把它 ingest 成 source-backed wiki**。其二，**任意 Agent 都能先读 index，再读相关页面，再在需要时回溯原始 source，并把高价值答案沉淀回 wiki**。其三，**多个 Agent 可以在同一知识库上协作，但只有统一写入协议能真正落盘，避免互相踩文件**。这三条做好了，你的产品就已经和“传统检索问答”拉开代差。citeturn32view1turn32view0turn31view0turn33view0

非目标也要尽早写死。MVP **不做**：云端中心化存储、默认全盘扫描、绕过系统权限、自动读取未授权目录、强依赖 embedding/向量库、富实时协同编辑、以及“不经审查直接把每次问答写成事实页面”。这些要么违背 local-first 与用户授权原则，要么会在早期把系统复杂度抬到不必要的程度。MCP 规范本身就强调用户同意、数据最小暴露和工具安全；Apple 也把 Full Disk Access 视为显式用户授予的权限，而不是应用默认能力。citeturn33view0turn27view2turn27view3

## 信息架构与数据模型

我建议你的知识库把**“页面”与“断言”分开建模**。页面负责人类可读和主题聚合；claim 负责事实最小单元、来源追溯和冲突检测。Karpathy 的做法已经把 `index.md` 看成内容目录，把 `log.md` 看成按时间追加的演化账本；你需要在这之上再显式引入 `claims/`，让“每个事实都能追到 source”成为强约束，而不是写作风格。这样做的好处是：实体页、概念页、专题页会保持可读；Claim 页则可以做 lint、冲突比对、过期判断、引用缺失检查和关系补全。citeturn32view1turn32view0turn32view2

推荐目录结构如下：

```text
knowledge-vault/
  AGENTS.md
  CLAUDE.md
  HERMES.md
  kb.yaml
  .gitignore
  .gitattributes
  .codex/
    config.toml
    agents/
  .claude/
    settings.local.json
    agents/
  .cursor/
    rules/
  .opencode/
    opencode.json
  raw/
    inbox/
    imported/
    attachments/
    manifests/
  wiki/
    index.md
    log.md
    entities/
    concepts/
    claims/
    sources/
    queries/
    reviews/
    relations/
    aliases/
  state/
    index.sqlite
    queue/
    locks/
    cache/
    watcher/
    audit/
  tools/
    mcp/
    bridge/
    extract/
  scripts/
```

这套结构背后的取舍是：`raw/` 永远只增不改；`wiki/` 是可演化层；`state/` 是可重建状态；`AGENTS.md` 是跨 Agent 的规范主文件；其余工具生态对它做适配。Git 负责版本历史和审查，`git worktree` 用于多 Agent 隔离工作区，`git diff` 用于 review 和 patch 可视化。citeturn31view0turn29view0turn29view1

推荐统一 frontmatter 基础 schema：

```yaml
---
id: ent.person.andrej_karpathy
type: entity            # source | entity | concept | claim | query | relation
title: Andrej Karpathy
aliases: ["Andrej", "Karpathy"]
status: active          # draft | active | contested | superseded | archived
canonical: true
created_at: 2026-05-20T10:00:00Z
updated_at: 2026-05-20T10:00:00Z
source_ids: ["src.web.karpathy_llm_wiki"]
claim_ids: ["clm.llmwiki.three_layers"]
tags: ["llm", "knowledge-base"]
review:
  state: reviewed       # draft | machine_checked | reviewed | contested
  owner: agent.wiki_curator
---
```

Claim 页应该更严格：

```yaml
---
id: clm.llmwiki.three_layers
type: claim
statement: "LLM Wiki 架构包含 raw sources、wiki、schema/instructions 三层。"
subject: con.llm_wiki
predicate: has_architecture
object: con.three_layer_model
status: supported       # supported | contested | superseded | uncertain
confidence: high
source_spans:
  - source_id: src.web.karpathy_llm_wiki
    locator:
      kind: web_lines
      ref: "turn31view0"
      detail: "L88-L94"
evidence_strength: direct
last_verified_at: 2026-05-20T10:00:00Z
---
```

这个设计的重点不是“把 YAML 写得很漂亮”，而是确保后续 lint 能直接回答这些问题：有没有 claim 没 source、有没有 source 被引用但 claim 未抽取、有没有 claim 状态过期、有没有同一 subject/predicate 的冲突对象、有没有同义实体被分裂成多个 canonical 页面。Obsidian 本身就把笔记存成 Markdown 纯文本文件，且支持 frontmatter 属性；这让“人可读 + 机可管”成为现实，而不是抽象设想。citeturn23view7turn12search12turn32view2

页面组织建议如下：

- `wiki/entities/`：稳定对象，如人、公司、项目、代码库、产品、协议。
- `wiki/concepts/`：抽象概念，如 LLM Wiki、MCP、BM25、事务、增量索引。
- `wiki/claims/`：原子断言，一页一 claim，一定带 source locator。
- `wiki/sources/`：原始资料的派生页，记录来源、摘要、抽取结果和状态。
- `wiki/queries/`：高质量问答沉淀页，不直接当事实源，必须链接 claims。
- `wiki/relations/`：显式关系页，便于关系类型治理。
- `wiki/aliases/`：同义词、重定向、规范名映射。

`wiki/index.md` 只做内容目录，不堆事实；`wiki/log.md` 只做追加账本，不做结论。Karpathy 原文对这两个文件的职责区分非常清楚，而且这恰好适合用简单 Unix/CLI 工具和 Agent 自动维护。citeturn32view1turn32view0

## 本地文件访问方案与跨平台架构

你的关键约束不是“如何黑进去”，而是**如何把“用户本人已获授权的系统级本地读取能力”变成稳定、可审计、可复用的 Agent 能力**。因此我建议的核心组件不是“神奇 reader”，而是一个**File Bridge**：由用户在本机启动、只暴露授权目录、同时提供 CLI 与 MCP 接口的本地桥接层。MCP 规范要求用户显式同意数据访问和工具调用，并强调工具本质上是任意代码执行，应谨慎授权；这和你的安全边界完全一致。citeturn33view0turn23view5turn23view6

File Bridge 的实现建议是：

```text
Agent  ──> MCP server / CLI ──> kb-daemon ──> file-bridge ──> authorized roots
                                   │
                                   ├─ indexer
                                   ├─ extractor workers
                                   ├─ write coordinator
                                   └─ audit log
```

其中 `file-bridge` 只做四类事：枚举目录、读取文件、调用受控提取器、产生变更事件；它**不默认开放任意 shell**，也不越权读取未授权路径。Agent 通过 MCP 调用的是稳定工具，而不是直接拿到一个全权终端。这样做的好处是：Claude、Codex、Cursor、OpenCode、Hermes 都能接入，但审计和权限边界仍掌握在你的本地 daemon 手里。citeturn33view0turn26view11turn26view15turn24search0turn26view1turn26view4

在 macOS 上，MVP 应坚持**“用户启动的非沙盒 daemon + CLI”**。Apple 官方文档明确表示：Full Disk Access 需要用户显式添加；桌面、文稿、下载等目录也属于 Privacy & Security 管辖；普通文件 API 并不会替你穿透权限边界。与此同时，APFS 在 macOS 默认是大小写不敏感，但又可能被配置成大小写敏感；它还对 Unicode 规范化做了特殊处理。因此，你的 bridge 必须做路径规范化、大小写冲突检测、Unicode 归一比较和明确的权限错误分类。对于未来桌面 App 版本，如果进 App Sandbox，则应使用 security-scoped bookmark 保持用户授予的持久文件访问；但这不应该是 MVP 的起点。守护进程层面，macOS 侧建议用用户级 `launchd`/LaunchAgent 托管 watcher 和索引器。citeturn27view2turn27view3turn27view4turn27view7turn28view0turn28view2turn5search2turn5search6turn27view1

在 Windows 上，MVP 应坚持**“用户态 PowerShell/可执行程序 + FileSystemWatcher + USN Journal reconcile”**。`FileSystemWatcher` 适合实时近线变更通知；USN Journal 则是 NTFS 的持久变化日志，官方文档甚至直接说明它比轮询时间戳或纯文件通知更高效，适合索引服务使用。Windows 还要额外处理 ACL、路径长度、编码、大小写和 Defender 影响：ACL 默认从父目录继承；Win32 默认路径上限仍有 MAX_PATH 260 的历史约束；NTFS 默认并不区分大小写；PowerShell 7 与 Windows PowerShell 的编码默认值并不完全一致，而 `Select-String` 在无 BOM 时按 UTF-8 处理。后台任务建议用登录时启动的 scheduled task，而不是系统服务优先。若遇到 Defender 性能问题，只能在**确认有具体性能或兼容性问题**时配置最小化排除项，不能默认让用户大面积关掉保护。citeturn27view8turn36view0turn27view9turn27view11turn27view12turn27view16turn27view17turn27view14turn27view15turn37search1turn36view3turn36view4

推荐 `kb.yaml` 如下：

```yaml
version: 1

vault:
  root: ~/knowledge-vault
  authorized_roots:
    - path: ~/knowledge-vault
      read: true
      write: true
    - path: ~/Documents/Research
      read: true
      write: false

bridge:
  mode: daemon
  listen:
    mcp: stdio
    local_api: 127.0.0.1:4318
  deny_globs:
    - "**/.git/**"
    - "**/node_modules/**"
    - "**/.DS_Store"
  audit_log: state/audit/bridge.jsonl

index:
  sqlite: state/index.sqlite
  content_hash: blake3
  detect:
    - mtime
    - size
    - hash_on_change
  watcher:
    enabled: true
    debounce_ms: 1500
  tokenizer:
    latin: "porter unicode61"
    cjk: "trigram"

ingest:
  max_workers: 4
  batch_size: 64
  extractors:
    markdown: native
    html: pandoc
    docx: pandoc
    pdf: tika
    pdf_fallback: pdfbox+ocr
    image_ocr: tesseract
    audio: whisper
```

文档摄取上，我建议采用**“类型分层提取”**。Markdown 与纯文本直接读；HTML、DOCX 先走 Pandoc 做结构转换；PDF 先走 Tika/PDFBox 文本层抽取，再根据质量分数决定是否 OCR；图片 OCR 用 Tesseract；音频转录用 Whisper。原因很现实：Tika 的 Parser 接口就是为统一抽取不同文档格式的结构化文本和元数据而设计；PDFBox 虽然能抽文本，但官方也明确提醒 PDF 是图形格式，不保证文本顺序天然正确；Pandoc 能在 Markdown、HTML、LaTeX、docx 等格式间转换，但官方同时提醒不能期待所有格式都完美无损互转；Tesseract 原生支持命令行 OCR；Whisper 的 `turbo` 模型则以更快速度换来轻微精度损失，适合本地批处理。citeturn29view7turn29view8turn29view9turn29view10turn15search9turn23view12turn29view6turn23view13

CLI 命令建议统一到 `kb` 命名空间：

```bash
kb init
kb bridge serve
kb mcp serve
kb watch start
kb ingest add <path>
kb ingest run <source-id>
kb search text "<query>"
kb query "<question>"
kb lint run
kb dream run
kb patch create
kb patch apply <patch-id>
kb review list
kb doctor
```

本地搜索方案的取舍我建议分阶段处理。**MVP 不上向量库**，只用 `index.md + ripgrep + SQLite FTS5`。ripgrep 天生跨平台、尊重 `.gitignore` 且在 Windows/macOS/Linux 都有一等支持；SQLite FTS5 是内嵌式全文检索，FTS5 的 `unicode61`、`porter`、`trigram` 都是官方内置 tokenizer。对中文语料尤其重要的一点是：SQLite 官方明确说明 `porter` 是面向英文词干的包装 tokenizer，而 `trigram` 支持基于子串的更一般匹配。对中文知识库，你的默认配置应考虑 `trigram` 或独立 CJK tokenizer，而不是直接照搬英文的 `porter unicode61`。citeturn23view10turn23view9turn40view0turn40view2

如果你考虑 qmd，我的建议是：**先作为实验性查询引擎接入，不要把它作为中文语料的唯一主检索**。qmd 的定位非常契合你的目标：它是本地 on-device 搜索引擎，组合了 BM25、向量检索和 LLM reranking；Karpathy 的 gist 也把它点名为本地 markdown 搜索的候选。不过，qmd 仓库在 2026 年 4 月有过针对 CJK 的 FTS5/BM25 问题报告，指出用 `porter unicode61` 会让中文查询命中退化为几乎只有向量侧有效。因此，如果你的知识库主要是中文，你要么在试点时验证 tokenizer 配置，要么先把主索引固定在你自己可控的 SQLite FTS5 方案上。citeturn26view7turn32view1turn38view0

当规模从 10k 文件上升到 100k 甚至 1M 文件时，再逐步引入 Tantivy、LanceDB、Qdrant 或 Faiss：Tantivy 适合高性能本地倒排索引；LanceDB 适合把全文、向量和 reranker 放在一个较统一的工程面内；Qdrant 更适合多阶段 dense+sparse 融合和大规模 ANN；Faiss 适合非常明确的向量近邻检索。MVP 不该一开始全上。citeturn30view3turn30view0turn30view1turn30view2

## 多 Agent 协作协议与工作流

这套系统的关键不是“每个 Agent 都有自己的记忆”，而是**所有 Agent 共享一个 canonical contract**。为此，我建议把 `AGENTS.md` 设为**跨 Agent 主规范**，然后为各工具生成薄适配层。Codex 原生支持 `AGENTS.md`；Cursor 同时支持 `.cursor/rules` 和 `AGENTS.md`；OpenCode 明确支持 `AGENTS.md`；Claude Code 则明确说明它读取 `CLAUDE.md`，但官方建议在已有 `AGENTS.md` 的仓库里，用 `CLAUDE.md` 导入 `@AGENTS.md` 以避免重复维护。Hermes 官方文档重心在 skills，而不是 `HERMES.md`；所以对 Hermes 来说，真正的执行载体应是一个 `SKILL.md`，而 `HERMES.md` 可以作为给人和其他 agent 阅读的镜像规范。citeturn23view1turn25view0turn23view4turn41view0turn26view4turn26view5

建议你采用如下约定：

```text
AGENTS.md      # canonical contract，所有 agent 的共同规则
CLAUDE.md      # 仅做 @AGENTS.md import + Claude-specific 补充
HERMES.md      # 人类可读版 Hermes 约定
skills/wiki-maintainer/SKILL.md   # Hermes 实际加载的 skill
.cursor/rules/wiki.mdc            # 从 AGENTS.md 生成/同步
.codex/agents/wiki_curator.toml   # Codex 专用子代理
.claude/agents/wiki-curator.md    # Claude 专用子代理
```

推荐的 `AGENTS.md` 草案：

```md
# Knowledge Vault Agent Contract

## Mission
Maintain a source-backed Markdown wiki.
Never edit raw sources.
Treat claims as first-class objects.

## Read order
1. Read `wiki/index.md`
2. Read relevant `wiki/entities/`, `wiki/concepts/`, `wiki/claims/`
3. If a claim is important, trace back to `wiki/sources/` and then raw source
4. Only then synthesize an answer

## Write rules
- Never write directly into `raw/`
- Never state unsupported information as fact
- Every factual addition must create or update at least one claim with source locator
- If evidence conflicts, mark `status: contested`, do not silently overwrite
- Use canonical IDs and alias mapping
- Prefer patch proposal over in-place edits when touching more than 3 pages

## Page rules
- Keep entity pages human-readable
- Keep claim pages atomic
- Update `wiki/index.md` and append `wiki/log.md` on every accepted change

## Maintenance
- Run lint for orphan pages, broken links, duplicate entities, missing citations, stale conclusions, contradictions
- Propose merges for synonyms; never auto-merge without evidence
```

推荐的 `CLAUDE.md` 草案：

```md
@AGENTS.md

## Claude Code specifics
- Use plan mode before modifications that affect more than one folder
- Use hooks to validate frontmatter and claim/source linkage before write
- Store project-local ephemeral learnings in auto memory, not in wiki, unless they are reusable knowledge artifacts
- Prefer creating a patch proposal when evidence is weak or contested
```

这和 Claude Code 官方推荐路径是一致的：`CLAUDE.md` 负责持久指令，且可导入 `AGENTS.md`；Claude 还支持项目级和用户级 subagents、hooks 以及 machine-local auto memory。你不需要和它对抗，而应该利用它。citeturn41view0turn41view1turn41view2turn26view12turn26view13turn26view14

推荐的 `HERMES.md` 草案：

```md
# Hermes Wiki Maintainer

## Role
You are a cautious curator, not a freeform assistant.

## Memory policy
- Reusable domain knowledge belongs in `wiki/`
- Temporary preferences belong in Hermes local memory/skills
- Do not convert conversation guesses into wiki facts

## Workflow
- Discover relevant skills
- Read index, then pages, then sources
- Emit structured patch proposals
- Ask for review when claims are contested or schema changes are required
```

但要强调：**Hermes 实际集成面应以 skills 为主**。Hermes 把 skills 作为按需加载的知识文档，保存在 `~/.hermes/skills/`，这比单独约定一个 `HERMES.md` 更符合它的原生工作方式。citeturn26view4turn26view5

多 Agent 写入时，我强烈建议做成**“单写者协调器”**。具体协议如下：

- 所有 Agent 都只能调用 `propose_patch`，不能直接改写 wiki 文件。
- `kb-daemon` 是唯一 `apply_patch` 执行者。
- Patch 带上 base revision、目标文件列表、变更摘要、涉及 claim/source IDs。
- 协调器先跑 schema 校验、frontmatter 校验、claim-source 链接校验、冲突检测，再决定自动应用或进入 review queue。
- 真正落盘时使用临时文件 + 原子 rename，并在同一事务里更新 SQLite 索引、日志和审计记录。

这样做是因为 SQLite 虽然很适合本地状态库，但多进程写入仍然要讲究事务策略。官方文档说明 `BEGIN IMMEDIATE` 可以在事务开始时就抢到写事务，从而避免中途升级写锁时的 `SQLITE_BUSY`；WAL 模式下读写并发更好，但 SQLite 官方在 2026 年还披露过一个罕见的 WAL race bug，并要求升级到修复版本。因此，你的实现应采用**单写多读 + `BEGIN IMMEDIATE` + patched SQLite 版本**，而不是“让每个 agent 都自己写 DB”。citeturn29view3turn29view4turn29view2

Git 层面，推荐**每个 agent 一个 worktree / branch**。Git 官方说明一个仓库可以同时挂多个 working trees，这正适合让 Codex、Claude、Cursor cloud agent 或命令行 worker 在彼此隔离的目录里产出 patch。最终由 `kb-daemon` 或人工 review 把 patch merge 回主分支。你不需要真的搭一个 GitHub PR 系统，但应实现一个本地的 PR-style review：`review queue -> diff -> accept/reject -> apply -> log`。citeturn29view0turn29view1

工作流建议如下：

**ingest**
`raw/inbox` 放入文件 → 计算 hash / manifest → 抽取文本与元数据 → 生成 `wiki/sources/*.md` → 提取 claims → 定位关联 entities/concepts → 更新 `index.md` → 追加 `log.md` → 进入 review queue。Karpathy 原文明确建议一份 source 往往会触发 10–15 个页面修改，因此 ingest 不应该只是“生成摘要”，而应被设计成**图谱更新事务**。citeturn32view4turn32view1

**query**
先读 `wiki/index.md`，再读命中的 entities / concepts / claims，再回溯 source 页面与 raw source，最后生成答案。若答案质量高，落成 `wiki/queries/` 页面，但注意：该页面必须引用 claims，而不能反过来成为事实基础。Karpathy 明确把“好答案应沉淀回 wiki”作为核心价值之一。citeturn32view1turn32view3

**lint**
周期性检查：孤立页面、断链、缺失引用、重复实体、重复概念、矛盾 claims、被新 source 取代的旧结论、缺失关系类型、别名未收敛、source hash 漂移。Karpathy 的原始 lint 目标就包括 contradictions、stale claims、orphan pages、missing links 和 data gaps。citeturn32view2

**dream cycle**
这是我建议你引入的产品术语：夜间或空闲时跑低优先级维护，不直接写事实，只产出建议。它做五类事：同义词聚类、概念层级整理、关系补全建议、陈旧结论复检、查询沉淀候选。MVP 的 dream cycle 可以只输出 `reviews/` 里的提案，不自动合并。

推荐的 MCP tools 设计草案如下：

```text
kb_roots.list              # 列出授权根目录
kb_files.stat              # 路径元数据
kb_files.read_text         # 只读文本读取
kb_files.extract           # 受控提取（pdf/docx/html/image/audio）
kb_search.bm25             # SQLite FTS / ripgrep 搜索
kb_search.hybrid           # 可选，v1+ 引入
kb_wiki.get_index          # 读取 wiki/index.md
kb_wiki.get_page           # 读取指定页面
kb_claim.get               # 读取 claim
kb_claim.trace_sources     # claim -> source spans
kb_patch.propose           # 提交 patch 提案
kb_patch.validate          # schema / citation / conflict 校验
kb_patch.apply             # 仅协调器可调用
kb_lint.run                # 运行 lint
kb_review.list             # 审核队列
kb_review.resolve          # 审核通过/拒绝
kb_log.append              # 追加 log
kb_audit.tail              # 审计日志
```

这些 tool 应在元数据里明确标识 `readOnlyHint`、`destructive`、`idempotent` 和作用域，以符合 MCP 的工具安全实践。citeturn33view0turn23view6

## 性能、安全与恢复策略

性能路线我建议分成三档。**10k 文件以内**，保持极简：文件系统 watcher、内容 hash、SQLite FTS5、ripgrep、内存 LRU cache、批量 ingest 队列。**100k 文件级**，引入更严格的分层索引：SQLite 做元数据和 claims，Tantivy 或自定义倒排做高性能正文索引，抽取器进程池和分片队列开始发挥作用。**1M 文件级**，再引入 hybrid / vector 层，并把 dense、sparse、rerank 拆成可独立扩缩的模块。LanceDB、Qdrant、Faiss 都更适合放在这一层，而不是 MVP 初期。citeturn23view9turn23view10turn30view3turn30view0turn30view1turn30view2

增量索引策略应使用**mtime + size 快筛，hash 做最终确认**。Windows 侧 watcher 由 `FileSystemWatcher` 提供近实时感知，USN Journal 做漏事件补偿；macOS 侧 watcher 用 FSEvents，外加定时 reconcile。批处理上，建议用“小文件合批、大文件单独处理”的队列策略。对 PDF、图片、音频，抽取器必须做**质量评分和降级策略**：例如 PDF 文本层可读率低于阈值就自动进入 OCR；OCR 结果低置信度则只建 source 页，不自动生成事实 claims。PDFBox 官方已明确提醒 PDF 不是文本格式，所以提取顺序和可提取性都有限制。citeturn27view8turn36view0turn29view9

安全与隐私上，我建议坚持四条。第一，**授权根目录白名单**，所有读取都在 roots 之内。第二，**最小工具暴露**，默认只开只读 tool，写入类 tool 由协调器持有。第三，**全量审计**，包括读取了哪些文件、谁提了什么 patch、哪些 claim 被新增或标记 contested。第四，**本地优先且状态可删**，缓存、索引和 memory 都应可重建。MCP 规范对 consent、data privacy、tool safety 的要求，基本就是这套设计的制度来源。citeturn33view0

失败恢复策略必须工程化，而不是“Git 会救你”。我建议至少做这些：

- 文件写入采用 `tmp -> fsync -> atomic rename`。
- `log.md` 和 `state/audit/*.jsonl` 都是 append-only。
- SQLite 用 WAL，但锁定到修复版本，并在写事务中使用 `BEGIN IMMEDIATE`。
- `state/` 视为可重建状态；真正不能丢的是 `raw/` 和 `wiki/`。
- 每日自动 Git commit，重要 patch 进入 review queue。
- 提供 `kb repair`：重扫 raw、重建索引、重放日志、校验 dangling claims。
- 对 Obsidian 用户，明确告知 File Recovery 只是意外修改保护，不是完整备份，而且它是设备本地、不会自动跨设备同步。citeturn29view2turn29view3turn29view4turn29view5

你特别关心的“wiki 不幻觉、不污染”问题，我建议用**四层防线**。第一层，写规则：未经 source locator 支撑的内容不得进入 supported claim。第二层，数据模型：claim 独立建模，状态分为 `supported / contested / superseded / uncertain`。第三层，lint：扫描缺失引用、冲突断言、陈旧结论和重复实体。第四层，review：低风险结构修复可自动应用，高风险事实修改必须入队审核。Karpathy 原文的 lint 思路和 source-backed wiki 已经给了基础框架；你的工程实现需要把它变成硬协议。citeturn31view0turn32view2

风险清单我建议写成下面这组：

- **事实幻觉风险**：Agent 把推测写成事实。  
  缓解：claim 必须有 source spans；无证据内容只允许进入 `query notes` 或 `uncertain`。 citeturn31view0turn32view2

- **知识污染风险**：低质量 query 结果回写污染 wiki。  
  缓解：`queries/` 与 `claims/` 分层；query 页面不能直接充当事实源。 citeturn32view3turn32view1

- **重复概念与同义词风险**：中文/英文混名、多别名导致知识分叉。  
  缓解：`aliases/` + canonical ID + nightly duplicate lint。  

- **并发写冲突风险**：多个 agent 同时改同页。  
  缓解：单写者协调器、patch queue、Git worktree、原子落盘。 citeturn29view0turn29view3

- **中文检索质量风险**：英文 tokenizer 套到中文语料。  
  缓解：中文默认 trigram 或专用 tokenizer；qmd 仅试点接入并做中文回归。 citeturn40view0turn40view2turn38view0

- **权限/隐私风险**：agent 读取超出用户预期目录。  
  缓解：roots 白名单、MCP consent、审计日志、按 tool 分级授权。 citeturn33view0

- **平台差异风险**：大小写、编码、路径长度、APFS 规范化、NTFS ACL 导致异常。  
  缓解：统一路径规范层、UTF-8 内部表示、Windows 长路径测试、大小写冲突 lint。 citeturn28view0turn27view12turn27view16turn27view14

## 路线图、技术选型与三十天开发计划

技术选型上，我的明确建议是：

- **MVP 主栈**：Rust 或 Go 写 `kb-daemon` 与 `file-bridge`；SQLite 做 state；Markdown 文件做 source of truth；抽取器用外部命令或 worker 子进程。
- **为什么不是 Electron 先做**：你当前核心价值在本地读取、索引、MCP 和多 Agent 协作，不在 UI。桌面壳会过早把权限、沙盒和跨平台 UI 复杂度拉高。
- **为什么不是向量数据库先行**：Karpathy 自己也把 `index.md` + proper search 视为随规模逐步演进的过程，MVP 最大敌人不是召回不足，而是写污染和工作流不稳定。citeturn32view1turn31view0

MVP 范围我建议控制在以下闭环内：

- 授权目录管理
- 文件桥接读取
- Markdown / HTML / DOCX / PDF / 图片 OCR / 音频转录 ingest
- SQLite FTS5 + ripgrep 搜索
- `index.md` / `log.md` / `entities/` / `concepts/` / `claims/` / `sources/`
- patch queue + 单写者协调器
- Lint
- CLI + MCP server
- Claude / Codex / Cursor / OpenCode / Hermes 五种适配样例

v1 再补：工作流模板、review UI、Obsidian plugin、更多 extractors、混合检索。v2 再补：向量层、跨库 federation、更高级 dream cycle、自动关系学习、团队协作策略。

三十天开发计划建议如下。

**第一个阶段**  
先把仓库和协议固定住：定义目录结构、frontmatter schema、ID 规范、claim/source 规则、`AGENTS.md` canonical contract、`CLAUDE.md` 导入方式、`Cursor` / `OpenCode` / `Codex` / `Hermes` 的适配文件生成器。并且在这周就把“不能直接写 wiki，只能提 patch”的单写者原则定死。citeturn23view1turn41view0turn25view0turn23view4turn26view4

**第二个阶段**  
完成 `file-bridge` 与 `kb-daemon` 最小实现：授权 roots、目录遍历、文本读取、stat、watcher、审计日志、SQLite state、`kb init / kb watch / kb search / kb query / kb patch` 的命令面。Windows 侧优先打通 PowerShell + FileSystemWatcher + USN reconcile；macOS 侧打通 zsh/Terminal 启动、权限检查和 FSEvents watcher。citeturn27view8turn36view0turn27view2turn28view0

**第三个阶段**  
实现 ingest 管线：manifest、hash、抽取器 worker、source page 生成、claim 提取、entity/concept 链接、`index.md` 更新、`log.md` 追加、lint 初版。把 PDF、图片、音频三类的 fallback 路线跑通，允许质量差时只生成 source 页。citeturn29view7turn29view8turn23view12turn29view6turn23view13

**第四个阶段**  
实现 MCP server 与 agent 适配：至少暴露只读 search/read tools、claim trace、patch propose/validate，以及最小 lint tools。然后分别用 Claude Code、Codex、Cursor CLI、OpenCode、Hermes 跑通同一 vault 的读取与 patch 提交流程。MCP 的目标不是“万物远程化”，而是“任何 host 都走同一工具语义”。citeturn23view5turn23view6turn26view11turn26view15turn34search1turn26view1turn26view4

具体任务拆解可以直接落成 backlog：

- Core：schema、IDs、normalizer、path abstraction、audit
- Bridge：roots、stat、read、extract、watch、permissions
- Index：SQLite schema、FTS5、hash、incremental queue
- Wiki：source page writer、claim linker、entity/concept updater、index/log writer
- Coordination：patch model、validator、lock manager、review queue
- Integrations：Claude / Codex / Cursor / OpenCode / Hermes adapters
- Maintenance：lint、repair、dream cycle、doctor
- Packaging：macOS launch agent、Windows scheduled task、installer scripts
- QA：中文语料回归、长路径、Unicode、大小写冲突、权限拒绝、文件锁、损坏恢复

最后给出我的**最终取舍结论**：  
你要做的不是“local RAG”，而是**“一个以 Git 仓库为核心、以 Markdown Wiki 为主存、以 claims 为事实单元、以 CLI/daemon/MCP 为执行面、以单写者协调器为安全阀的个人知识基础设施”**。MVP 应该先把**可审计的写入协议、跨平台文件桥、claim/source 约束、以及多 Agent 共享 contract**做稳；embedding、vector DB、桌面 UI、花哨视图都应该后置。这个顺序最符合 Karpathy 的原始思想，也最符合你现在的工程约束。citeturn31view0turn32view1turn33view0turn23view7turn29view0

## 开放问题与限制

有三点我建议在动工前心里有数。第一，**Hermes 并没有官方标准化 `HERMES.md` 约定**，它更偏向 skills；所以上文给出的 `HERMES.md` 是产品约定，不是 Hermes 原生规范。第二，**qmd 对中文语料的最新实际状态需要你在试点仓库里再做一次回归**；我拿到的是 2026 年 4 月的主仓 issue 证据，足够说明它需要验证，但不应替代你的实测。第三，**如果未来你一定要做沙盒化桌面 App**，macOS 的 security-scoped bookmarks、Windows 的权限与企业策略兼容性都会明显增加开发复杂度，所以这件事更适合放到 v1 之后。citeturn26view4turn38view0turn27view2turn5search2