# LLM Wiki 个人知识库产品方案

> 基于 Andrej Karpathy 的 LLM Wiki 模式，面向多 Agent 协作的 local-first 个人知识库系统

---

## 一、产品定位与背景

### 1.1 核心理念

Karpathy 于 2026 年 4 月提出 LLM Wiki 模式：用 LLM Agent 持续维护一个结构化 Markdown 知识库，替代传统 RAG 的"查询-遗忘"循环。核心洞察：

- **RAG 是无状态的**：每次查询独立，知识不积累
- **Wiki 是有状态的**：知识持续编译、交叉引用、演化
- **Plain text 胜过 vector DB**：对个人规模（数百到数千页），直接文件读取比 embedding 检索更可靠、更可调试

参考来源：
- [Karpathy LLM Wiki Gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
- [MindStudio: Build a Personal Knowledge Base 70x Faster Than RAG](https://www.mindstudio.ai/blog/karpathy-llm-wiki-pattern-personal-knowledge-base-without-rag)

### 1.2 三层架构（Karpathy 原始设计）

```
┌─────────────────────────────────────────────┐
│  Layer 3: Schema / Instructions             │
│  AGENTS.md, CLAUDE.md, HERMES.md            │
│  定义 agent 如何维护知识库                    │
├─────────────────────────────────────────────┤
│  Layer 2: Wiki                              │
│  由 Agent 维护的 Markdown 知识库              │
│  index.md, concepts/, entities/, claims/    │
├─────────────────────────────────────────────┤
│  Layer 1: Raw Sources                       │
│  原始资料，只读、不可变、source of truth      │
│  papers/, articles/, transcripts/, images/  │
└─────────────────────────────────────────────┘
```

### 1.3 产品定位

**WikiMind** — 一个 local-first、multi-agent 协作的个人知识编译系统。

- **目标用户**：重度知识工作者、独立研究者、技术写作者、AI 工程师
- **核心价值**：让多个 AI Agent 共同维护一个可积累、可演化、可追溯的知识库
- **非目标**：不是笔记应用、不是搜索引擎、不是团队协作工具（v1）

### 1.4 LLM Wiki vs RAG：优势、限制与风险

| 维度 | 传统 RAG | LLM Wiki |
|------|----------|----------|
| 知识积累 | 无（每次查询独立） | 有（持续编译） |
| 可调试性 | 低（向量空间不可读） | 高（纯文本，Git 可追溯） |
| 基础设施 | 重（向量DB、embedding pipeline） | 轻（文件系统 + Git） |
| 精确度 | 概率性（近似最近邻） | 确定性（文件级） |
| 适用规模 | 百万级文档 | 数百到数万页（hybrid 可扩展） |
| 幻觉风险 | 检索噪声导致 | 编译错误导致（但可追溯 source） |
| 维护成本 | 低（被动索引） | 中（需要 lint/dream cycle） |
| 知识质量 | 取决于原始文档 | 可通过 claim 验证持续提升 |

**LLM Wiki 的独特风险：**
1. **编译幻觉**：Agent 可能在编译时引入原文不存在的推断 → 解决：claim 必须附 source + quote
2. **知识污染**：错误信息一旦写入 wiki 会被后续查询引用 → 解决：confidence 分级 + periodic lint
3. **概念漂移**：同一概念在不同时间被不同 agent 以不同方式描述 → 解决：canonical ID + 同义词表
4. **规模瓶颈**：当 wiki 超过 context window 时 index 不够用 → 解决：分层 index + FTS5 + 未来 hybrid

---

## 二、核心用户故事

### 2.1 MVP 用户故事

1. **作为研究者**，我把一篇论文 PDF 放入 `raw/` 目录，Agent 自动提取关键概念、创建 wiki 页面、建立与已有知识的交叉引用
2. **作为开发者**，我向 Claude Code 提问时，它先读 wiki index，再读相关页面，给出基于我积累知识的回答
3. **作为知识管理者**，我运行 `wikimind lint`，系统报告孤立页面、断链、矛盾声明、缺失引用
4. **作为多 agent 用户**，Claude Code 和 Codex 都能读写同一个知识库，不会互相覆盖

### 2.2 进阶用户故事

5. **Dream Cycle**：每周 Agent 自动审查 wiki，合并重复概念、更新陈旧结论、发现知识空白
6. **Query Sedimentation**：一次高质量问答的结果自动沉淀为新的 wiki 页面
7. **Cross-reference Discovery**：Agent 发现两个看似无关的概念之间存在联系，创建关系页面

---

## 三、MVP 范围与非目标

### 3.1 MVP 范围（30 天）

- [x] 三层目录结构初始化
- [x] Agent instruction 文件（AGENTS.md, CLAUDE.md, CODEX.md, HERMES.md）
- [x] MCP server 提供文件读写 + 搜索能力
- [x] CLI 工具：init, ingest, query, lint, dream
- [x] Markdown frontmatter schema 定义
- [x] 基于 SQLite FTS5 的本地全文搜索
- [x] Git-based 版本控制与冲突处理
- [x] macOS + Windows 跨平台文件访问
- [x] Claim 追溯机制（每个 claim 必须有 source）
- [x] 基本 file watcher（检测 raw/ 新增文件）

### 3.2 非目标（暂不做）

- ❌ Web UI / Desktop App（用 Obsidian 或 VS Code 查看）
- ❌ 多用户协作
- ❌ 云同步（用户自行用 Git remote / Syncthing）
- ❌ Embedding / Vector DB（大规模时再引入）
- ❌ 实时协同编辑
- ❌ 自然语言查询界面（直接用 agent 对话）
- ❌ 移动端

---

## 四、信息架构与目录结构

### 4.1 推荐目录结构

```
~/wikimind/
├── .wikimind/                    # 系统配置与状态
│   ├── config.toml               # 全局配置
│   ├── index.db                  # SQLite FTS5 索引
│   ├── lock.json                 # 文件锁状态
│   └── changelog.jsonl           # 变更日志（机器可读）
├── raw/                          # Layer 1: 原始资料（只读）
│   ├── papers/
│   │   └── attention-is-all-you-need.pdf
│   ├── articles/
│   │   └── karpathy-llm-wiki.md
│   ├── transcripts/
│   │   └── podcast-ep42.txt
│   ├── images/
│   │   └── architecture-diagram.png
│   ├── code/
│   │   └── nanoGPT/
│   └── _manifest.jsonl           # 资料清单与 ingest 状态
├── wiki/                         # Layer 2: Agent 维护的知识库
│   ├── index.md                  # 总索引（Agent 入口点）
│   ├── log.md                    # 变更日志（人类可读）
│   ├── concepts/                 # 概念页面
│   │   ├── transformer.md
│   │   ├── attention-mechanism.md
│   │   └── llm-wiki-pattern.md
│   ├── entities/                 # 实体页面（人、组织、项目）
│   │   ├── andrej-karpathy.md
│   │   ├── openai.md
│   │   └── tesla-autopilot.md
│   ├── claims/                   # 声明页面（一等公民）
│   │   ├── claim-001-transformers-scale.md
│   │   └── claim-002-wiki-vs-rag.md
│   ├── sources/                  # 资料摘要页面
│   │   ├── attention-is-all-you-need.md
│   │   └── karpathy-llm-wiki.md
│   ├── relations/                # 关系页面
│   │   └── karpathy-cofounded-openai.md
│   ├── _synonyms.md              # 同义词映射表
│   └── _schema.md                # Wiki 页面 schema 说明
├── instructions/                 # Layer 3: Agent 指令
│   ├── AGENTS.md                 # 通用 agent 协议
│   ├── CLAUDE.md                 # Claude Code 专用指令
│   ├── CODEX.md                  # OpenAI Codex 专用指令
│   ├── HERMES.md                 # Hermes 专用指令
│   ├── OPENCODE.md               # OpenCode / Pi 专用指令
│   └── CURSOR.md                 # Cursor 专用指令
├── tools/                        # 本地工具脚本
│   ├── ingest.sh                 # 资料导入脚本
│   ├── lint.sh                   # 质量检查脚本
│   └── build-index.sh            # 索引构建脚本
├── .gitignore
└── .git/                         # 版本控制
```

### 4.2 Markdown Frontmatter Schema

#### 概念页面 (concepts/)

```yaml
---
id: concept-transformer
title: Transformer 架构
type: concept
created: 2026-05-20T10:30:00+08:00
updated: 2026-05-20T10:30:00+08:00
updated_by: claude-code
confidence: high          # high | medium | low | disputed
sources:
  - raw/papers/attention-is-all-you-need.pdf
  - raw/articles/karpathy-llm-wiki.md
related:
  - concept-attention-mechanism
  - entity-google-brain
tags: [deep-learning, architecture, nlp]
claims:
  - claim-001-transformers-scale
status: active            # active | archived | draft | needs-review
---

# Transformer 架构

一句话定义：基于自注意力机制的序列到序列模型架构。

## 核心要点

- 完全基于注意力机制，不使用循环或卷积
- 支持高度并行化训练
- 通过位置编码保留序列信息

## 详细说明

[详细内容...]

## 相关概念

- [[attention-mechanism]] — Transformer 的核心组件
- [[positional-encoding]] — 解决序列顺序问题

## 开放问题

- 对超长序列的效率问题仍在研究中

## 变更历史

- 2026-05-20: 初始创建 (claude-code)
```

#### 声明页面 (claims/)

```yaml
---
id: claim-001-transformers-scale
title: Transformer 架构的扩展性优于 RNN
type: claim
status: supported         # supported | disputed | refuted | unverified
confidence: high
created: 2026-05-20T10:30:00+08:00
updated: 2026-05-20T10:30:00+08:00
updated_by: claude-code
evidence:
  supporting:
    - source: raw/papers/attention-is-all-you-need.pdf
      page: 7
      quote: "The Transformer allows for significantly more parallelization"
      accessed: 2026-05-20
    - source: raw/papers/scaling-laws.pdf
      section: "3.2"
      quote: "Loss scales as a power-law with model size"
      accessed: 2026-05-20
  contradicting: []
related_claims:
  - claim-002-wiki-vs-rag
tags: [scaling, architecture, performance]
---

# Claim: Transformer 架构的扩展性优于 RNN

## 声明内容

Transformer 架构在参数规模扩展时，性能提升遵循幂律关系，且训练效率显著优于 RNN。

## 支持证据

1. **Vaswani et al. (2017)**: "The Transformer allows for significantly more parallelization and can reach a new state of the art in translation quality" (p.7)
2. **Kaplan et al. (2020)**: 损失函数与模型规模呈幂律关系 (Section 3.2)

## 反对证据

（暂无）

## 评估

置信度：高。多篇独立研究支持，无已知反驳。
```

#### 实体页面 (entities/)

```yaml
---
id: entity-andrej-karpathy
title: Andrej Karpathy
type: entity
subtype: person           # person | organization | project | tool | dataset
created: 2026-05-20T10:30:00+08:00
updated: 2026-05-20T10:30:00+08:00
updated_by: claude-code
sources:
  - raw/articles/karpathy-llm-wiki.md
related:
  - entity-openai
  - entity-tesla
  - concept-llm-wiki-pattern
aliases: ["Karpathy", "AK"]
tags: [ai-researcher, educator, entrepreneur]
status: active
---
```

---

## 五、数据模型

### 5.1 核心实体关系

```
┌──────────┐     cites      ┌──────────┐
│  Claim   │───────────────▶│  Source   │
└──────────┘                └──────────┘
     │                           │
     │ supports/refutes          │ summarizes
     ▼                           ▼
┌──────────┐    related     ┌──────────┐
│ Concept  │◀──────────────▶│  Entity  │
└──────────┘                └──────────┘
     │                           │
     └───────── related ─────────┘
            ┌──────────┐
            │ Relation │  (显式关系页面)
            └──────────┘
```

### 5.2 SQLite Schema（.wikimind/index.db）

```sql
-- 页面元数据索引
CREATE TABLE pages (
    id TEXT PRIMARY KEY,          -- e.g. "concept-transformer"
    title TEXT NOT NULL,
    type TEXT NOT NULL,           -- concept | entity | claim | source | relation
    subtype TEXT,                 -- person | organization | ...
    path TEXT NOT NULL UNIQUE,    -- 相对路径 wiki/concepts/transformer.md
    content_hash TEXT,            -- SHA-256 of file content
    mtime REAL,                  -- 文件修改时间
    updated_by TEXT,             -- 最后修改的 agent
    confidence TEXT,             -- high | medium | low | disputed
    status TEXT DEFAULT 'active', -- active | archived | draft
    created_at TEXT,
    updated_at TEXT
);

-- 全文搜索索引
CREATE VIRTUAL TABLE pages_fts USING fts5(
    id,
    title,
    content,
    tags,
    content='pages',
    content_rowid='rowid',
    tokenize='porter unicode61'
);

-- 关系图
CREATE TABLE edges (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    relation_type TEXT NOT NULL,  -- related | supports | refutes | cites | part_of | alias_of
    weight REAL DEFAULT 1.0,
    created_at TEXT,
    created_by TEXT,
    PRIMARY KEY (source_id, target_id, relation_type),
    FOREIGN KEY (source_id) REFERENCES pages(id),
    FOREIGN KEY (target_id) REFERENCES pages(id)
);

-- 变更日志
CREATE TABLE changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    agent TEXT NOT NULL,
    action TEXT NOT NULL,         -- create | update | delete | merge | lint-fix
    page_id TEXT,
    diff_summary TEXT,
    commit_hash TEXT,
    approved INTEGER DEFAULT 0   -- 0=pending, 1=approved, -1=rejected
);

-- 文件锁（乐观锁）
CREATE TABLE locks (
    page_path TEXT PRIMARY KEY,
    agent TEXT NOT NULL,
    acquired_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,     -- TTL 防止死锁
    operation TEXT               -- write | delete | merge
);

-- 原始资料清单
CREATE TABLE raw_manifest (
    path TEXT PRIMARY KEY,
    content_hash TEXT,
    mime_type TEXT,
    ingested INTEGER DEFAULT 0,
    ingested_at TEXT,
    ingested_by TEXT,
    size_bytes INTEGER
);

-- 索引：加速常用查询
CREATE INDEX idx_pages_type ON pages(type);
CREATE INDEX idx_pages_status ON pages(status);
CREATE INDEX idx_edges_source ON edges(source_id);
CREATE INDEX idx_edges_target ON edges(target_id);
CREATE INDEX idx_changelog_page ON changelog(page_id);
CREATE INDEX idx_changelog_time ON changelog(timestamp);
```

---

## 六、Agent 工作流

### 6.1 Ingest 工作流（资料导入）

```
用户放入 raw/ 文件 或 CLI: wikimind ingest <file>
       │
       ▼
┌─────────────────────────────────┐
│ 1. 检测文件类型                  │
│    PDF → pdftotext / pymupdf    │
│    DOCX → pandoc                │
│    HTML → readability + turndown│
│    图片 → tesseract OCR         │
│    音频 → whisper               │
│    Markdown → 直接使用           │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ 2. 写入 _manifest.jsonl         │
│    记录 hash、类型、时间          │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ 3. Agent 知识编译                │
│    • 获取文件锁                  │
│    • 读取转换后文本              │
│    • 提取概念 → concepts/        │
│    • 提取实体 → entities/        │
│    • 提取声明 → claims/          │
│    • 创建摘要 → sources/         │
│    • 发现关系 → relations/       │
│    • 更新已有页面交叉引用         │
│    • 更新 index.md              │
│    • 释放文件锁                  │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│ 4. 后处理                        │
│    • 更新 SQLite FTS5 索引       │
│    • Git add + commit            │
│    • 写入 changelog              │
│    • 更新 log.md                 │
└─────────────────────────────────┘
```

### 6.2 Query 工作流（知识查询）

```
用户向 Agent 提问
       │
       ▼
┌──────────────────────────────────┐
│ 1. 读取 wiki/index.md            │
│    了解知识库全貌和分类            │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ 2. FTS5 搜索                     │
│    wikimind search "关键词"       │
│    返回相关页面路径 + 摘要         │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ 3. 读取相关 wiki 页面             │
│    加载 concepts/ entities/ claims/│
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ 4. 验证（可选）                   │
│    对关键 claim 回溯 raw/ source  │
│    检查 quote 是否准确            │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ 5. 生成回答                       │
│    基于 wiki 知识 + source 验证    │
└────────────┬─────────────────────┘
             │
             ▼
┌──────────────────────────────────┐
│ 6. Query Sedimentation（沉淀）    │
│    如果回答产生了新的综合知识：     │
│    • 创建新 wiki 页面             │
│    • 或更新已有页面               │
│    • 标记 confidence: medium      │
│    • 标记 source: "query synthesis"│
└──────────────────────────────────┘
```

### 6.3 Lint 工作流（质量检查）

```bash
$ wikimind lint [--fix] [--report]
```

检查项：

| 检查类型 | 说明 | 自动修复 |
|---------|------|---------|
| orphan-pages | 无入链的页面 | 添加到 index.md |
| broken-links | 引用不存在的页面 | 标记为 TODO |
| contradictions | 同一主题相反 claim | 标记为 disputed |
| stale-pages | 超过 90 天未更新 | 标记为 needs-review |
| missing-sources | claim 无 source 引用 | 标记为 unverified |
| duplicate-entities | 同义词未合并 | 建议合并 |
| schema-violations | frontmatter 不合规 | 自动修复格式 |
| confidence-decay | 新证据可能反驳旧 claim | 降级 confidence |

### 6.4 Dream Cycle（周期性维护）

```bash
$ wikimind dream [--dry-run]
```

```
定时触发（推荐：每周一次）
     │
     ▼
┌────────────────────────────────────┐
│ Phase 1: Audit                     │
│ • 运行完整 lint                    │
│ • 统计知识库健康度指标              │
│ • 识别知识空白（有 entity 无 concept）│
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Phase 2: Consolidate               │
│ • 合并重复概念（基于 _synonyms.md） │
│ • 统一术语用法                      │
│ • 修复断链                          │
│ • 合并碎片化的 claim                │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Phase 3: Evolve                    │
│ • 对 low confidence claim 寻找新证据│
│ • 发现新的交叉引用                   │
│ • 生成 "本周知识变更摘要"            │
│ • 建议用户补充的 raw source          │
└────────────┬───────────────────────┘
             │
             ▼
┌────────────────────────────────────┐
│ Phase 4: Report                    │
│ • 更新 wiki/log.md                 │
│ • Git commit: "dream: weekly maintenance"│
│ • 输出人类可读报告                   │
└────────────────────────────────────┘
```

---

## 七、跨平台本地文件访问方案

### 7.1 设计原则

本方案设计为**用户本人授权的本地文件访问能力**。所有文件访问均限定在用户明确授权的目录内，通过标准操作系统权限机制实现。

### 7.2 macOS 方案

#### 文件访问层

| 方式 | 适用场景 | 性能 | 复杂度 |
|------|---------|------|--------|
| zsh / bash 命令 | Agent 直接执行 shell | 高 | 低 |
| MCP Filesystem Server | 标准化 agent 接口 | 高 | 中 |
| FSEvents (watchman) | 文件变更监听 | 极高 | 中 |
| Spotlight (mdfind) | 系统级全文搜索 | 高 | 低 |
| launchd daemon | 后台服务 | 高 | 中 |

#### macOS 关键注意事项

```toml
# .wikimind/config.toml - macOS 配置
[platform.macos]
# 确保 wikimind 目录在 Full Disk Access 授权范围内
# 或放在用户 home 目录下（无需额外授权）
wiki_root = "~/wikimind"

# 文件监听方式
watcher = "fsevents"  # fsevents | polling
# FSEvents 是 macOS 原生，性能最优，延迟 < 100ms

# Spotlight 集成（可选，利用系统索引加速搜索）
spotlight_enabled = true
# mdfind -onlyin ~/wikimind "transformer"

# 沙盒限制：如果 agent 运行在沙盒中
# 需要通过 MCP server 或 CLI bridge 访问
sandbox_bridge = "mcp"  # mcp | cli | direct
```

#### macOS 权限处理

```bash
# 检查目录权限
ls -la ~/wikimind/

# 确保 agent 进程有读写权限
chmod -R u+rw ~/wikimind/

# 如果使用 launchd 后台服务
# ~/Library/LaunchAgents/com.wikimind.daemon.plist
# 以当前用户身份运行，继承用户文件权限
```

#### FSEvents Watcher（推荐）

```javascript
// 使用 Node.js chokidar（跨平台，底层用 FSEvents）
const chokidar = require('chokidar');

const watcher = chokidar.watch('~/wikimind/raw/', {
  persistent: true,
  ignoreInitial: true,
  awaitWriteFinish: { stabilityThreshold: 2000 }
});

watcher.on('add', path => triggerIngest(path));
```

### 7.3 Windows 方案

#### 文件访问层

| 方式 | 适用场景 | 性能 | 复杂度 |
|------|---------|------|--------|
| PowerShell | Agent 直接执行命令 | 高 | 低 |
| MCP Filesystem Server | 标准化 agent 接口 | 高 | 中 |
| FileSystemWatcher (.NET) | 文件变更监听 | 高 | 中 |
| Windows Search (WDS) | 系统级搜索 | 中 | 低 |
| USN Journal | 高性能变更检测 | 极高 | 高 |

#### Windows 关键注意事项

```toml
# .wikimind/config.toml - Windows 配置
[platform.windows]
wiki_root = "C:\\Users\\<username>\\wikimind"
# 或使用 %USERPROFILE%\\wikimind

# 长路径支持（Windows 10 1607+）
long_paths_enabled = true
# 需要注册表：HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled = 1

# 文件监听
watcher = "filesystemwatcher"  # filesystemwatcher | polling | usn

# Windows Defender 排除（提升性能）
# 建议将 wikimind 目录加入 Defender 排除列表
# Set-MpPreference -ExclusionPath "C:\Users\<user>\wikimind"
defender_exclusion_recommended = true

# 编码处理
default_encoding = "utf-8"
# Windows 默认可能是 GBK/CP936，强制 UTF-8
bom_handling = "strip"  # strip | preserve | add

# NTFS 特性
case_sensitive = false  # NTFS 默认大小写不敏感
# 可通过 fsutil.exe file setCaseSensitiveInfo 启用目录级大小写敏感
```

#### Windows 权限与 ACL

```powershell
# 检查当前用户对目录的权限
Get-Acl "C:\Users\$env:USERNAME\wikimind" | Format-List

# 确保完全控制权限
$acl = Get-Acl "C:\Users\$env:USERNAME\wikimind"
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
    $env:USERNAME, "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
)
$acl.SetAccessRule($rule)
Set-Acl "C:\Users\$env:USERNAME\wikimind" $acl
```

#### FileSystemWatcher

```powershell
# PowerShell FileSystemWatcher
$watcher = New-Object System.IO.FileSystemWatcher
$watcher.Path = "$env:USERPROFILE\wikimind\raw"
$watcher.IncludeSubdirectories = $true
$watcher.EnableRaisingEvents = $true

Register-ObjectEvent $watcher "Created" -Action {
    $path = $Event.SourceEventArgs.FullPath
    wikimind ingest $path
}
```

### 7.4 跨平台统一方案

#### 路径处理

```typescript
// 跨平台路径规范化
import { resolve, normalize, sep } from 'path';
import { homedir } from 'os';

function getWikiRoot(): string {
  const configRoot = process.env.WIKIMIND_ROOT;
  if (configRoot) return resolve(configRoot);
  return resolve(homedir(), 'wikimind');
}

// 统一使用 POSIX 路径存储（在 frontmatter 和 index 中）
function toStoragePath(absolutePath: string): string {
  const root = getWikiRoot();
  const relative = path.relative(root, absolutePath);
  return relative.split(sep).join('/'); // 统一为 /
}
```

#### 编码与换行符

```toml
# .wikimind/config.toml - 跨平台设置
[files]
encoding = "utf-8"
line_ending = "lf"          # 统一使用 LF（Git 配合 .gitattributes）
max_filename_length = 200   # 安全值，兼容所有平台
```

```gitattributes
# .gitattributes
*.md text eol=lf
*.toml text eol=lf
*.json text eol=lf
*.jsonl text eol=lf
```

#### 文件锁处理

```typescript
// 跨平台文件锁（基于 SQLite，避免 OS 级锁的差异）
async function acquireLock(pagePath: string, agent: string): Promise<boolean> {
  const db = getIndexDb();
  const now = new Date().toISOString();
  const expires = new Date(Date.now() + 60_000).toISOString(); // 60s TTL
  
  try {
    db.run(`
      INSERT INTO locks (page_path, agent, acquired_at, expires_at)
      VALUES (?, ?, ?, ?)
      ON CONFLICT(page_path) DO UPDATE SET
        agent = excluded.agent,
        acquired_at = excluded.acquired_at,
        expires_at = excluded.expires_at
      WHERE expires_at < ?
    `, [pagePath, agent, now, expires, now]);
    return true;
  } catch {
    return false; // 锁被占用且未过期
  }
}

async function releaseLock(pagePath: string, agent: string): Promise<void> {
  const db = getIndexDb();
  db.run(`DELETE FROM locks WHERE page_path = ? AND agent = ?`, [pagePath, agent]);
}
```

#### 错误处理策略

| 错误类型 | macOS | Windows | 处理方式 |
|---------|-------|---------|---------|
| 权限拒绝 | EACCES | EPERM | 提示用户检查权限，跳过文件 |
| 文件被锁 | EBUSY | EBUSY/SHARING_VIOLATION | 等待重试（3次，指数退避） |
| 路径过长 | 极少 | ERROR_FILENAME_EXCED_RANGE | 截断或 hash 文件名 |
| 编码错误 | 少见 | 常见（GBK混入） | 检测编码，转换为 UTF-8 |
| 磁盘满 | ENOSPC | ERROR_DISK_FULL | 报错，不写入 |
| 文件不存在 | ENOENT | ERROR_FILE_NOT_FOUND | 从索引中移除，标记断链 |

### 7.5 Agent 文件访问 Bridge 设计

推荐的 Agent 访问知识库的方式优先级：

```
1. MCP Server（最推荐）
   ├── 标准化接口，所有支持 MCP 的 agent 通用
   ├── 内置权限控制（只允许访问 wikimind 目录）
   └── 支持 Claude Desktop, Cursor, 自定义 client

2. CLI 命令（通用性最强）
   ├── 任何能执行 shell 的 agent 都能用
   ├── wikimind search / wikimind read / wikimind write
   └── 适合 Claude Code, Codex, OpenCode, 命令行 agent

3. 直接文件系统访问
   ├── Agent 本身有文件读写能力时
   ├── 如 Claude Code 的 Read/Write 工具
   └── 需要 agent 遵守 AGENTS.md 协议
```

---

## 八、MCP Server 设计

### 8.1 MCP Tools 列表

```typescript
// wikimind-mcp-server tools

interface WikiMindTools {
  // === 读取类 ===
  
  /** 读取 wiki 索引，了解知识库全貌 */
  "wikimind.read_index": () => string;
  
  /** 读取指定 wiki 页面 */
  "wikimind.read_page": (params: {
    page_id: string;  // e.g. "concept-transformer"
  }) => string;
  
  /** 全文搜索 wiki */
  "wikimind.search": (params: {
    query: string;
    type?: "concept" | "entity" | "claim" | "source" | "all";
    limit?: number;  // default 10
  }) => SearchResult[];
  
  /** 获取页面的关系图（入链 + 出链） */
  "wikimind.get_relations": (params: {
    page_id: string;
    depth?: number;  // default 1
  }) => Relation[];
  
  /** 读取原始资料（只读） */
  "wikimind.read_raw": (params: {
    path: string;  // 相对于 raw/ 的路径
  }) => string;
  
  /** 获取知识库统计信息 */
  "wikimind.stats": () => WikiStats;

  // === 写入类 ===
  
  /** 创建新的 wiki 页面 */
  "wikimind.create_page": (params: {
    type: "concept" | "entity" | "claim" | "source" | "relation";
    id: string;
    title: string;
    content: string;       // 完整 Markdown（含 frontmatter）
    related?: string[];    // 关联页面 ID
  }) => { success: boolean; path: string };
  
  /** 更新已有 wiki 页面 */
  "wikimind.update_page": (params: {
    page_id: string;
    content: string;       // 完整新内容
    reason: string;        // 变更原因（写入 changelog）
  }) => { success: boolean; diff_summary: string };
  
  /** 追加内容到页面（不覆盖） */
  "wikimind.append_to_page": (params: {
    page_id: string;
    section: string;       // 追加到哪个 ## 标题下
    content: string;
  }) => { success: boolean };

  // === 维护类 ===
  
  /** 运行 lint 检查 */
  "wikimind.lint": (params: {
    fix?: boolean;         // 是否自动修复
    checks?: string[];     // 指定检查项
  }) => LintReport;
  
  /** 合并重复页面 */
  "wikimind.merge_pages": (params: {
    source_id: string;     // 被合并的页面
    target_id: string;     // 合并到的目标
    reason: string;
  }) => { success: boolean };
  
  /** 更新索引数据库 */
  "wikimind.rebuild_index": () => { pages_indexed: number };

  // === 锁管理 ===
  
  /** 获取文件锁 */
  "wikimind.acquire_lock": (params: {
    page_id: string;
    agent: string;
    ttl_seconds?: number;  // default 60
  }) => { acquired: boolean; holder?: string };
  
  /** 释放文件锁 */
  "wikimind.release_lock": (params: {
    page_id: string;
    agent: string;
  }) => { released: boolean };
}
```

### 8.2 MCP Server 配置

```json
{
  "mcpServers": {
    "wikimind": {
      "command": "node",
      "args": ["~/wikimind/tools/mcp-server/index.js"],
      "env": {
        "WIKIMIND_ROOT": "~/wikimind",
        "WIKIMIND_LOG_LEVEL": "info"
      },
      "disabled": false,
      "autoApprove": [
        "wikimind.read_index",
        "wikimind.read_page",
        "wikimind.search",
        "wikimind.get_relations",
        "wikimind.stats"
      ]
    }
  }
}
```

### 8.3 MCP Server 实现要点

```typescript
// mcp-server/index.ts 核心结构
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import Database from "better-sqlite3";

const WIKI_ROOT = process.env.WIKIMIND_ROOT || path.join(homedir(), 'wikimind');
const DB_PATH = path.join(WIKI_ROOT, '.wikimind', 'index.db');

// 安全：只允许访问 WIKI_ROOT 内的文件
function assertSafePath(requestedPath: string): string {
  const resolved = path.resolve(WIKI_ROOT, requestedPath);
  if (!resolved.startsWith(path.resolve(WIKI_ROOT))) {
    throw new Error("Access denied: path traversal detected");
  }
  return resolved;
}

// 读取页面时自动更新 "last accessed" 统计
// 写入页面时自动：获取锁 → 写文件 → 更新索引 → Git commit → 释放锁
```

---

## 九、多 Agent 协作协议

### 9.1 协作模型

```
┌─────────────────────────────────────────────────────┐
│                  WikiMind 知识库                      │
│                                                     │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
│  │ Claude  │  │  Codex  │  │ Hermes  │  ...        │
│  │  Code   │  │         │  │         │            │
│  └────┬────┘  └────┬────┘  └────┬────┘            │
│       │             │             │                 │
│       ▼             ▼             ▼                 │
│  ┌──────────────────────────────────────┐          │
│  │        MCP Server / CLI Bridge        │          │
│  │  • 文件锁管理                         │          │
│  │  • 变更日志                           │          │
│  │  • Schema 验证                        │          │
│  │  • 冲突检测                           │          │
│  └──────────────────────────────────────┘          │
│                      │                              │
│                      ▼                              │
│  ┌──────────────────────────────────────┐          │
│  │           Git Repository              │          │
│  │  • 每次写入自动 commit                 │          │
│  │  • 冲突时创建 branch                   │          │
│  │  • 人工 review 后 merge               │          │
│  └──────────────────────────────────────┘          │
└─────────────────────────────────────────────────────┘
```

### 9.2 冲突避免策略

#### 策略一：乐观锁 + TTL（推荐 MVP）

```
Agent A 要写 concept-transformer.md:
1. acquire_lock("concept-transformer", "claude-code", ttl=60s)
2. 如果成功 → 读取、修改、写入、release_lock
3. 如果失败 → 等待 5s 重试，最多 3 次
4. TTL 过期自动释放（防止 agent 崩溃导致死锁）
```

#### 策略二：Git Branch（大变更）

```
Agent 执行 dream cycle 或大规模重构时：
1. git checkout -b dream/2026-05-20
2. 在 branch 上执行所有变更
3. 完成后创建 PR-style 摘要
4. 用户 review 后 merge 到 main
```

#### 策略三：Append-Only Log（高并发场景）

```
多个 agent 同时产出时：
1. 写入 .wikimind/pending/<agent>-<timestamp>.jsonl
2. 定期由 coordinator 合并到 wiki/
3. 合并时检测冲突，人工裁决
```

### 9.3 Agent 身份标识

每个 agent 在写入时必须标识自己：

```yaml
# 在 frontmatter 中
updated_by: claude-code    # 最后修改者
# 在 changelog 中
agent: codex-cli           # 操作执行者
```

支持的 agent 标识：
- `claude-code` — Claude Code (Anthropic)
- `codex-cli` — OpenAI Codex CLI
- `hermes` — Hermes Agent
- `opencode` — OpenCode / Pi
- `cursor` — Cursor IDE Agent
- `wikimind-daemon` — 本地后台服务
- `user-manual` — 用户手动编辑

### 9.4 变更审核机制

```toml
# .wikimind/config.toml
[review]
# 哪些操作需要人工审核
require_review_for = [
  "delete",           # 删除页面
  "merge",            # 合并页面
  "refute_claim",     # 反驳已有 claim
  "dream_cycle",      # dream cycle 的变更
]

# 哪些操作自动批准
auto_approve = [
  "create",           # 创建新页面
  "update_minor",     # 小幅更新（< 20% 变更）
  "lint_fix",         # lint 自动修复
  "add_cross_ref",    # 添加交叉引用
]

# 审核队列
[review.queue]
path = ".wikimind/review-queue/"
# 待审核的变更以 JSON 文件存放
# 用户通过 CLI: wikimind review 来处理
```

---

## 十、CLI 工具设计

### 10.1 命令列表

```bash
# 初始化知识库
wikimind init [--path <dir>]

# 导入原始资料
wikimind ingest <file|dir> [--type paper|article|transcript|image|code]
wikimind ingest --watch          # 持续监听 raw/ 目录

# 搜索知识库
wikimind search <query> [--type concept|entity|claim|source]
wikimind search --related <page_id>

# 读取页面
wikimind read <page_id>
wikimind read --index            # 读取 index.md

# 质量检查
wikimind lint [--fix] [--check orphan|broken|stale|duplicate|schema]
wikimind lint --report           # 输出 JSON 报告

# 周期性维护
wikimind dream [--dry-run] [--phase audit|consolidate|evolve|report]

# 索引管理
wikimind index rebuild           # 重建 FTS5 索引
wikimind index stats             # 索引统计

# 变更管理
wikimind log [--last N]          # 查看变更日志
wikimind review                  # 审核待处理变更
wikimind review --approve <id>
wikimind review --reject <id>

# MCP Server
wikimind serve                   # 启动 MCP server（stdio 模式）
wikimind serve --port 3000       # HTTP 模式（开发用）

# 工具
wikimind export --format json|csv|graph  # 导出知识图谱
wikimind validate                        # 验证所有 frontmatter
wikimind gc                              # 清理过期锁、临时文件
```

### 10.2 配置文件

```toml
# .wikimind/config.toml

[general]
version = "0.1.0"
wiki_root = "."                    # 相对于 config 文件的位置
default_agent = "user-manual"      # 手动操作时的 agent 标识

[search]
engine = "fts5"                    # fts5 | ripgrep | tantivy
max_results = 20
snippet_length = 200

[ingest]
# 文件类型处理器
[ingest.handlers]
pdf = "pdftotext"                  # pdftotext | pymupdf | marker
docx = "pandoc"
html = "readability"
image = "tesseract"                # tesseract | gpt4-vision
audio = "whisper"                  # whisper | whisper-cpp
video = "whisper"                  # 提取音频后转录

[ingest.watch]
enabled = false
debounce_ms = 2000                 # 文件稳定后再处理
ignore_patterns = [".*", "_*", "*.tmp"]

[git]
auto_commit = true
commit_prefix = "wiki:"           # e.g. "wiki: create concept-transformer"
branch_for_dream = true           # dream cycle 在独立 branch 执行

[lint]
stale_threshold_days = 90
min_confidence_for_active = "medium"
auto_fix = ["schema-violations", "orphan-pages"]

[dream]
schedule = "weekly"               # weekly | daily | manual
max_pages_per_run = 50
require_review = true

[platform]
# 自动检测，也可手动覆盖
os = "auto"                       # auto | macos | windows | linux

[agents]
# 各 agent 的权限配置
[agents.claude-code]
can_create = true
can_update = true
can_delete = false
can_merge = false
max_pages_per_session = 20

[agents.codex-cli]
can_create = true
can_update = true
can_delete = false
can_merge = false
max_pages_per_session = 10

[agents.wikimind-daemon]
can_create = true
can_update = true
can_delete = true
can_merge = true
max_pages_per_session = 100       # dream cycle 需要更多
```

---

## 十一、性能方案

### 11.1 规模演进路线

| 规模 | 文件数 | 索引方案 | 搜索方案 | 预期延迟 |
|------|--------|---------|---------|---------|
| 小型 | < 1K | 无需索引，直接 ripgrep | ripgrep 全文 | < 100ms |
| 中型 | 1K-10K | SQLite FTS5 | FTS5 + ripgrep 备选 | < 200ms |
| 大型 | 10K-100K | SQLite FTS5 + 分层 index | FTS5 + tantivy | < 500ms |
| 超大 | 100K-1M | Tantivy + LanceDB hybrid | Tantivy + vector search | < 1s |

### 11.2 增量索引策略

```typescript
// 增量索引：只处理变更文件
async function incrementalIndex(): Promise<void> {
  const db = getIndexDb();
  const wikiDir = path.join(WIKI_ROOT, 'wiki');
  
  // 1. 扫描文件系统，获取当前 mtime
  const currentFiles = await scanDirectory(wikiDir);
  
  // 2. 与数据库中的 mtime 对比
  for (const file of currentFiles) {
    const dbRecord = db.get('SELECT mtime, content_hash FROM pages WHERE path = ?', file.path);
    
    if (!dbRecord || dbRecord.mtime < file.mtime) {
      // 文件已变更，重新索引
      const content = await readFile(file.absolutePath, 'utf-8');
      const hash = sha256(content);
      
      if (hash !== dbRecord?.content_hash) {
        // 内容确实变了（排除 mtime 误报）
        await updateIndex(file.path, content, hash, file.mtime);
      }
    }
  }
  
  // 3. 检测已删除文件
  const dbPaths = db.all('SELECT path FROM pages').map(r => r.path);
  const currentPaths = new Set(currentFiles.map(f => f.path));
  for (const dbPath of dbPaths) {
    if (!currentPaths.has(dbPath)) {
      db.run('DELETE FROM pages WHERE path = ?', dbPath);
      db.run('DELETE FROM pages_fts WHERE id = (SELECT id FROM pages WHERE path = ?)', dbPath);
    }
  }
}
```

### 11.3 搜索性能优化

```sql
-- FTS5 搜索优化
-- 使用 BM25 排名
SELECT id, title, path, 
       snippet(pages_fts, 2, '<mark>', '</mark>', '...', 32) as snippet,
       bm25(pages_fts, 5.0, 2.0, 1.0, 1.0) as rank
FROM pages_fts
WHERE pages_fts MATCH ?
ORDER BY rank
LIMIT 10;

-- 按类型过滤
SELECT p.id, p.title, p.path, p.type
FROM pages p
JOIN pages_fts f ON p.rowid = f.rowid
WHERE f.pages_fts MATCH ?
  AND p.type = ?
ORDER BY bm25(pages_fts)
LIMIT 10;
```

### 11.4 大文件处理

```toml
# 资料导入的分块策略
[ingest.chunking]
# 超过此大小的文件分块处理
chunk_threshold_bytes = 100_000   # 100KB
chunk_size = 50_000              # 50KB per chunk
chunk_overlap = 2_000            # 2KB overlap

# 并发控制
[performance]
max_concurrent_reads = 10
max_concurrent_writes = 1         # 写入串行化，避免冲突
index_batch_size = 100            # 批量索引
watcher_debounce_ms = 2000
```

### 11.5 缓存策略

```typescript
// LRU 缓存：热门页面内容
const pageCache = new LRUCache<string, string>({
  max: 100,                    // 最多缓存 100 个页面
  maxSize: 10 * 1024 * 1024,   // 最大 10MB
  sizeCalculation: (value) => Buffer.byteLength(value),
  ttl: 5 * 60 * 1000,         // 5 分钟 TTL
});

// index.md 常驻内存（最频繁读取）
let indexCache: string | null = null;
let indexMtime: number = 0;
```

---

## 十二、Claim 作为一等公民

### 12.1 设计哲学

传统 wiki 的问题：信息以"事实"形式呈现，但实际上很多是推断、观点或有条件的结论。WikiMind 将 **claim（声明）** 作为一等公民，每个知识断言都必须：

1. **可追溯**：指向具体 source + 位置（页码、段落、时间戳）
2. **可验证**：标注 confidence level 和 evidence 数量
3. **可反驳**：支持 disputed/refuted 状态
4. **可演化**：随新证据更新状态

### 12.2 Claim 生命周期

```
新信息进入 raw/
       │
       ▼
┌─────────────────┐
│ Agent 提取声明   │
│ status: unverified│
│ confidence: low  │
└────────┬────────┘
         │
         ▼ (找到支持证据)
┌─────────────────┐
│ status: supported│
│ confidence: medium│
└────────┬────────┘
         │
         ▼ (多个独立来源确认)
┌─────────────────┐
│ status: supported│
│ confidence: high │
└────────┬────────┘
         │
         ├──▶ (发现反驳证据)
         │    ┌─────────────────┐
         │    │ status: disputed │
         │    │ confidence: low  │
         │    └─────────────────┘
         │
         └──▶ (被完全推翻)
              ┌─────────────────┐
              │ status: refuted  │
              │ confidence: n/a  │
              └─────────────────┘
```

### 12.3 防幻觉机制

```markdown
## Agent 写入 Claim 的规则（写入 AGENTS.md）

1. **禁止无源声明**：每个 claim 必须至少有一个 source 引用
2. **区分事实与推断**：
   - 事实：直接引用原文 → confidence: high
   - 推断：基于多个事实的逻辑推导 → confidence: medium
   - 猜测：缺乏充分证据 → confidence: low, status: unverified
3. **引用必须精确**：
   - 必须包含 source 文件路径
   - 必须包含位置（page/section/timestamp）
   - 推荐包含原文 quote（< 30 words）
4. **不得修改已有 high-confidence claim 的核心内容**
   - 只能添加新证据或标记为 disputed
   - 降级需要明确的反驳证据
5. **综合性声明必须标注**：
   - 如果 claim 是从多个 source 综合得出
   - 必须列出所有 source 并说明推理过程
```

---

## 十三、安全、隐私与审计

### 13.1 安全设计

```
┌─────────────────────────────────────────┐
│           安全边界                        │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │  MCP Server / CLI                 │  │
│  │  • 路径遍历防护                    │  │
│  │  • 只允许访问 WIKIMIND_ROOT       │  │
│  │  • 写入速率限制                    │  │
│  │  • 文件大小限制                    │  │
│  └───────────────────────────────────┘  │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │  Agent 权限控制                    │  │
│  │  • 每个 agent 独立权限配置         │  │
│  │  • 不允许执行任意 shell 命令       │  │
│  │  • 不允许访问 raw/ 以外的文件      │  │
│  │  • 操作审计日志                    │  │
│  └───────────────────────────────────┘  │
│                                         │
│  ┌───────────────────────────────────┐  │
│  │  数据保护                          │  │
│  │  • 所有数据本地存储                 │  │
│  │  • 不发送数据到外部服务             │  │
│  │  • Git 历史提供完整审计轨迹         │  │
│  │  • 可选：.gitcrypt 加密敏感文件     │  │
│  └───────────────────────────────────┘  │
└─────────────────────────────────────────┘
```

### 13.2 审计日志

```jsonl
{"ts":"2026-05-20T10:30:00Z","agent":"claude-code","action":"create","page":"concept-transformer","commit":"a1b2c3d"}
{"ts":"2026-05-20T10:31:00Z","agent":"claude-code","action":"update","page":"index","commit":"e4f5g6h"}
{"ts":"2026-05-20T11:00:00Z","agent":"codex-cli","action":"create","page":"claim-001","commit":"i7j8k9l"}
```

### 13.3 隐私考量

- **本地优先**：所有数据存储在用户本地磁盘
- **Agent 调用时的隐私**：wiki 内容会发送给 LLM API（这是使用 agent 的固有特性）
- **敏感信息标记**：支持在 frontmatter 中标记 `sensitivity: private`，这类页面不会被自动发送给 agent
- **选择性上下文**：agent 查询时只加载相关页面，不会一次性发送整个 wiki

---

## 十四、失败模式与恢复策略

| 失败模式 | 检测方式 | 恢复策略 |
|---------|---------|---------|
| Agent 写入幻觉内容 | lint 检测无 source 的 claim | 标记为 unverified，等待人工审核 |
| 索引与文件不同步 | content_hash 校验 | `wikimind index rebuild` |
| 文件锁死锁 | TTL 过期检测 | 自动释放过期锁 |
| Git 冲突 | merge 失败 | 创建 conflict branch，人工解决 |
| 磁盘空间不足 | 写入前检查 | 报错 + 建议清理 |
| Agent 崩溃中途 | 不完整的 frontmatter | lint 检测 schema 违规，标记为 draft |
| 重复页面创建 | ID 冲突检测 | 拒绝创建，建议 merge |
| 编码损坏 | UTF-8 验证 | 尝试修复编码，失败则隔离文件 |
| 大文件导致 OOM | 文件大小预检 | 分块处理，限制单次加载量 |
| 循环引用 | 关系图环检测 | lint 报告，建议重构 |

### 恢复命令

```bash
# 重建索引（索引损坏时）
wikimind index rebuild

# 清理过期锁
wikimind gc --locks

# 验证所有文件完整性
wikimind validate --all

# 回滚到上一个 Git commit
wikimind rollback [--commit <hash>]

# 从 Git 历史恢复单个文件
wikimind restore <page_id> [--commit <hash>]
```

---

## 十五、与现有工具的集成

### 15.1 Obsidian 集成

```
WikiMind 目录结构天然兼容 Obsidian：
- wiki/ 目录可直接作为 Obsidian vault 打开
- [[wiki-links]] 语法兼容
- frontmatter 在 Obsidian 中正常显示
- 使用 Obsidian 作为人类阅读/浏览界面

注意事项：
- Obsidian 的 .obsidian/ 配置目录加入 .gitignore
- 避免在 Obsidian 中手动编辑 agent 维护的页面
- 可以用 Obsidian 的 graph view 可视化知识图谱
```

### 15.2 Git 集成

```bash
# .gitignore
.wikimind/index.db          # 索引可重建，不需要版本控制
.wikimind/lock.json         # 运行时状态
.wikimind/pending/          # 临时文件
.obsidian/                  # Obsidian 配置
node_modules/
*.tmp
```

### 15.3 ripgrep 集成

```bash
# 快速全文搜索（FTS5 的补充）
rg --type md "transformer" wiki/
rg --type md -l "confidence: low" wiki/  # 找所有低置信度页面
rg --type md "status: disputed" wiki/claims/  # 找所有争议 claim
```

### 15.4 技术选型对比

| 组件 | 选项 A | 选项 B | 选项 C | **推荐** | 理由 |
|------|--------|--------|--------|---------|------|
| 运行时 | Node.js | Python | Rust | **Node.js** | MCP SDK 原生支持，生态丰富 |
| 搜索引擎 | SQLite FTS5 | Tantivy | Elasticsearch | **SQLite FTS5** | 零依赖，嵌入式，性能够用 |
| 文件监听 | chokidar | watchman | 原生 API | **chokidar** | 跨平台，稳定，Node.js 原生 |
| PDF 解析 | pdftotext | PyMuPDF | pdf.js | **pdftotext** | CLI 工具，无运行时依赖 |
| OCR | Tesseract | GPT-4V | Apple Vision | **Tesseract** | 本地运行，免费，跨平台 |
| 数据库 | SQLite | DuckDB | LevelDB | **SQLite** | 最成熟，FTS5 内置 |
| 包管理 | npm | pnpm | bun | **pnpm** | 快速，磁盘效率高 |
| 构建工具 | tsup | esbuild | rollup | **tsup** | 简单，基于 esbuild |
| 测试 | vitest | jest | mocha | **vitest** | 快速，ESM 原生支持 |

---

## 十六、产品路线图

### MVP（第 1-30 天）

```
Week 1: 基础架构
├── 项目初始化（TypeScript + pnpm）
├── 目录结构生成器（wikimind init）
├── 配置文件解析
├── SQLite FTS5 索引模块
└── 基本 CLI 框架（commander.js）

Week 2: 核心功能
├── Ingest 工作流（Markdown 直接导入）
├── PDF/DOCX 转换（pdftotext + pandoc）
├── Search 命令（FTS5 查询）
├── Read 命令（页面读取）
└── Frontmatter 解析与验证

Week 3: Agent 集成
├── MCP Server 实现（stdio 模式）
├── AGENTS.md / CLAUDE.md 编写
├── 文件锁机制
├── Git 自动 commit
└── Changelog 记录

Week 4: 质量与维护
├── Lint 工具（所有检查项）
├── Dream cycle 基础版
├── 跨平台测试（macOS + Windows）
├── 文档编写
└── 发布 v0.1.0
```

### v1.0（第 31-90 天）

```
├── File watcher（chokidar）自动 ingest
├── 图片 OCR（Tesseract）
├── 音频转录（whisper.cpp）
├── Tantivy 搜索引擎（可选后端）
├── 更完善的 dream cycle
├── Review queue UI（CLI 交互式）
├── 多 agent 并发测试与优化
├── Obsidian 插件（可选）
├── 性能基准测试（10K 文件）
└── 发布 v1.0.0
```

### v2.0（第 91-180 天）

```
├── Hybrid search（FTS5 + embedding）
├── LanceDB / Qdrant 集成（可选）
├── 知识图谱可视化（D3.js / Obsidian graph）
├── 多知识库联邦查询
├── Agent 自主学习（主动搜索补充 source）
├── 语义去重（embedding 相似度）
├── 自然语言查询接口
├── Web UI（可选）
└── 发布 v2.0.0
```

---

## 十七、Agent Instructions 草案

### 17.1 AGENTS.md（通用协议）

见独立文件：`instructions/AGENTS.md`

### 17.2 CLAUDE.md（Claude Code 专用）

见独立文件：`instructions/CLAUDE.md`

### 17.3 HERMES.md（Hermes 专用）

见独立文件：`instructions/HERMES.md`

---

## 十八、风险清单

| # | 风险 | 概率 | 影响 | 缓解措施 |
|---|------|------|------|---------|
| 1 | Agent 编译幻觉污染 wiki | 高 | 高 | Claim 必须附 source；confidence 分级；periodic lint |
| 2 | 多 agent 写入冲突 | 中 | 中 | 乐观锁 + TTL；Git branch 隔离；串行化写入 |
| 3 | 知识库规模超出 context window | 中 | 高 | 分层 index；FTS5 精确检索；未来 hybrid search |
| 4 | 概念漂移与重复 | 高 | 中 | canonical ID；_synonyms.md；dream cycle 合并 |
| 5 | 跨平台兼容性问题 | 中 | 中 | 统一 UTF-8 + LF；路径规范化；CI 双平台测试 |
| 6 | PDF/图片解析质量差 | 中 | 低 | 多解析器备选；人工标注 fallback |
| 7 | Agent API 成本过高 | 低 | 中 | 批量处理；缓存；本地小模型做预处理 |
| 8 | 用户不审核导致错误积累 | 高 | 高 | 强制 review queue；confidence 自动降级 |
| 9 | Git 仓库过大 | 低 | 低 | .gitignore 排除大文件；Git LFS |
| 10 | MCP 协议变更 | 低 | 中 | 抽象层隔离；跟踪 MCP spec 更新 |
| 11 | 单点故障（SQLite 损坏） | 低 | 中 | 索引可重建；WAL 模式；定期备份 |
| 12 | 隐私泄露（敏感内容发送给 API） | 中 | 高 | sensitivity 标记；选择性上下文；本地模型选项 |

---

## 十九、30 天开发计划

### Week 1: 基础架构（Day 1-7）

| Day | 任务 | 产出 |
|-----|------|------|
| 1 | 项目初始化：pnpm + TypeScript + tsup + vitest | 可构建的空项目 |
| 1 | CLI 框架搭建（commander.js） | `wikimind --help` 可运行 |
| 2 | `wikimind init` 命令：生成目录结构 | 完整目录树 |
| 2 | 配置文件解析（config.toml → TOML parser） | 配置加载模块 |
| 3 | SQLite 模块：创建数据库 + schema | index.db 初始化 |
| 3 | FTS5 索引：创建 + 基本 CRUD | 索引读写测试通过 |
| 4 | Frontmatter 解析器（gray-matter） | 解析所有页面类型 |
| 4 | 页面 CRUD 模块：create/read/update/delete | 单元测试通过 |
| 5 | Git 集成：auto-commit on write | 写入自动提交 |
| 5 | Changelog 模块：记录所有操作 | changelog.jsonl 写入 |
| 6 | 文件锁模块：acquire/release/TTL | 并发测试通过 |
| 7 | 集成测试 + 代码审查 | Week 1 里程碑 |

### Week 2: 核心功能（Day 8-14）

| Day | 任务 | 产出 |
|-----|------|------|
| 8 | `wikimind ingest` 命令框架 | CLI 可调用 |
| 8 | Markdown 直接导入（最简路径） | .md 文件可 ingest |
| 9 | PDF 转文本（pdftotext wrapper） | PDF 可 ingest |
| 9 | DOCX 转 Markdown（pandoc wrapper） | DOCX 可 ingest |
| 10 | `wikimind search` 命令 | FTS5 搜索可用 |
| 10 | 搜索结果格式化（snippet + highlight） | 人类可读输出 |
| 11 | `wikimind read` 命令 | 页面内容输出 |
| 11 | Index.md 自动生成与更新 | index 保持最新 |
| 12 | 增量索引：mtime + content_hash | 只索引变更文件 |
| 12 | 跨平台路径处理模块 | macOS + Windows 测试 |
| 13 | `wikimind validate` 命令 | Schema 验证 |
| 14 | 集成测试 + 性能基准 | Week 2 里程碑 |

### Week 3: Agent 集成（Day 15-21）

| Day | 任务 | 产出 |
|-----|------|------|
| 15 | MCP Server 骨架（@modelcontextprotocol/sdk） | stdio server 启动 |
| 15 | 实现 read_index + read_page tools | 读取类 tools 可用 |
| 16 | 实现 search + get_relations tools | 搜索类 tools 可用 |
| 16 | 实现 create_page + update_page tools | 写入类 tools 可用 |
| 17 | 实现 lint + acquire_lock + release_lock tools | 维护类 tools 可用 |
| 17 | MCP Server 安全：路径遍历防护 | 安全测试通过 |
| 18 | 编写 AGENTS.md | 通用协议文档 |
| 18 | 编写 CLAUDE.md | Claude Code 指令 |
| 19 | 编写 CODEX.md + HERMES.md | 其他 agent 指令 |
| 19 | 端到端测试：Claude Code + MCP Server | 实际 agent 可用 |
| 20 | `wikimind serve` 命令 | MCP server 一键启动 |
| 21 | 集成测试 + 文档 | Week 3 里程碑 |

### Week 4: 质量与发布（Day 22-30）

| Day | 任务 | 产出 |
|-----|------|------|
| 22 | `wikimind lint` 实现所有检查项 | 完整 lint 报告 |
| 22 | Lint auto-fix 模式 | --fix 可用 |
| 23 | Dream cycle 基础版 | audit + consolidate |
| 23 | Dream cycle report 生成 | log.md 更新 |
| 24 | Windows 测试与修复 | 跨平台 CI 通过 |
| 25 | 性能优化：批量索引、缓存 | 1K 文件 < 5s 全量索引 |
| 26 | README.md + 使用文档 | 用户文档完整 |
| 27 | npm 包发布准备 | package.json 完善 |
| 28 | Beta 测试（自己使用） | 真实场景验证 |
| 29 | Bug 修复 + 边界情况处理 | 稳定性提升 |
| 30 | 发布 v0.1.0 | GitHub release |

---

## 二十、推荐 Repo 结构

```
wikimind/
├── packages/
│   ├── core/                     # 核心库
│   │   ├── src/
│   │   │   ├── index.ts
│   │   │   ├── config.ts         # 配置解析
│   │   │   ├── database.ts       # SQLite 操作
│   │   │   ├── search.ts         # FTS5 搜索
│   │   │   ├── pages.ts          # 页面 CRUD
│   │   │   ├── frontmatter.ts    # Frontmatter 解析
│   │   │   ├── lock.ts           # 文件锁
│   │   │   ├── git.ts            # Git 操作
│   │   │   ├── changelog.ts      # 变更日志
│   │   │   ├── ingest/           # 资料导入
│   │   │   │   ├── index.ts
│   │   │   │   ├── pdf.ts
│   │   │   │   ├── docx.ts
│   │   │   │   ├── html.ts
│   │   │   │   └── ocr.ts
│   │   │   ├── lint/             # 质量检查
│   │   │   │   ├── index.ts
│   │   │   │   ├── orphan.ts
│   │   │   │   ├── broken-links.ts
│   │   │   │   ├── contradictions.ts
│   │   │   │   └── schema.ts
│   │   │   └── types.ts          # 类型定义
│   │   ├── package.json
│   │   └── tsconfig.json
│   ├── cli/                      # CLI 工具
│   │   ├── src/
│   │   │   ├── index.ts          # 入口
│   │   │   ├── commands/
│   │   │   │   ├── init.ts
│   │   │   │   ├── ingest.ts
│   │   │   │   ├── search.ts
│   │   │   │   ├── read.ts
│   │   │   │   ├── lint.ts
│   │   │   │   ├── dream.ts
│   │   │   │   ├── review.ts
│   │   │   │   └── serve.ts
│   │   │   └── utils/
│   │   ├── package.json
│   │   └── tsconfig.json
│   └── mcp-server/              # MCP Server
│       ├── src/
│       │   ├── index.ts          # Server 入口
│       │   ├── tools/            # Tool 实现
│       │   │   ├── read.ts
│       │   │   ├── write.ts
│       │   │   ├── search.ts
│       │   │   ├── lint.ts
│       │   │   └── lock.ts
│       │   └── security.ts      # 安全检查
│       ├── package.json
│       └── tsconfig.json
├── templates/                    # 模板文件
│   ├── config.toml
│   ├── AGENTS.md
│   ├── CLAUDE.md
│   ├── CODEX.md
│   ├── HERMES.md
│   └── _schema.md
├── tests/
│   ├── unit/
│   ├── integration/
│   └── fixtures/
├── docs/
│   ├── getting-started.md
│   ├── architecture.md
│   ├── agent-guide.md
│   └── api-reference.md
├── package.json                  # Workspace root
├── pnpm-workspace.yaml
├── tsconfig.base.json
├── vitest.config.ts
├── .github/
│   └── workflows/
│       └── ci.yml               # macOS + Windows CI
└── README.md
```

---

## 二十一、关键设计决策与取舍

### Q1: 为什么不用 Vector DB？

**决策**：MVP 不用，v2 可选引入。

**理由**：
- 个人知识库通常 < 10K 页面，FTS5 完全够用
- Vector DB 引入额外依赖（Python runtime、模型下载、GPU）
- Plain text + FTS5 的可调试性远优于 embedding
- Karpathy 原始设计的核心洞察就是"简单胜过复杂"
- 当规模确实需要时，LanceDB（嵌入式）是最佳过渡方案

### Q2: 为什么选 SQLite 而不是纯文件？

**决策**：用 SQLite 做索引，文件做存储。

**理由**：
- 文件是 source of truth（人类可读、Git 可追踪）
- SQLite 只做加速层（可随时重建）
- FTS5 提供 BM25 排名，比 ripgrep 更适合语义搜索
- 关系图查询在 SQL 中比遍历文件高效得多
- SQLite 是零配置、跨平台、嵌入式的

### Q3: 为什么用 Node.js 而不是 Python/Rust？

**决策**：Node.js (TypeScript)

**理由**：
- MCP SDK 官方支持 TypeScript
- chokidar（文件监听）生态成熟
- better-sqlite3 性能优秀
- 跨平台打包简单（pkg / sea）
- Agent 开发者群体熟悉 TypeScript
- 如果性能瓶颈出现，可以用 Rust 写 native addon

### Q4: 为什么 Claim 是独立文件而不是嵌入 Concept？

**决策**：Claim 独立为 claims/ 目录下的文件。

**理由**：
- 一个 claim 可能被多个 concept 引用
- Claim 有独立的生命周期（unverified → supported → disputed）
- 独立文件便于 lint 检查和批量审核
- 避免 concept 页面过长
- 支持 claim 级别的 Git blame 追溯

### Q5: 如何处理 Agent 之间的"知识分歧"？

**决策**：以 source 为准，不以 agent 为准。

**规则**：
- 如果两个 agent 对同一事实有不同理解，创建 disputed claim
- 人工审核时以 raw/ source 原文为最终裁决
- 不存在"某个 agent 的观点更权威"的设定
- 所有 agent 平等，差异通过 evidence 解决

---

## 附录 A：参考资料

1. [Karpathy LLM Wiki Gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) — 原始设计
2. [LLM Wiki v2 (agentmemory 扩展)](https://gist.github.com/rohitg00/2067ab416f7bbe447c1977edaaa681e2) — 社区扩展
3. [MindStudio: 70x Faster Than RAG](https://www.mindstudio.ai/blog/karpathy-llm-wiki-pattern-personal-knowledge-base-without-rag) — 性能对比分析
4. [Augment: Git Worktrees for Parallel AI Agent Execution](https://www.augmentcode.com/guides/git-worktrees-parallel-ai-agent-execution) — 多 agent 隔离方案
5. [SQLite FTS5 官方文档](https://www.sqlite.org/fts5.html) — 全文搜索引擎
6. [MCP Filesystem Server](https://fast.io/resources/mcp-filesystem/) — MCP 文件访问标准
7. [Tantivy (Rust 全文搜索)](https://github.com/quickwit-oss/tantivy) — 大规模搜索备选

---

*文档版本：v0.1.0 | 最后更新：2026-05-20*
