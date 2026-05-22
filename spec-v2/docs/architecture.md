# 架构

> 系统全貌、进程模型、数据流、存储模型。SPEC 描述"是什么"，本文档描述"怎么搭"。

---

## 1. 系统全貌

```
┌──────────────────────────────────────────────────────────────────────┐
│                            User Surface                                │
│  ┌───────────┐  ┌───────────┐  ┌────────────┐  ┌────────────────┐    │
│  │  CLI      │  │ MCP host  │  │ Editor     │  │ Browser        │    │
│  │ wikimind  │  │  (Claude  │  │ (Obsidian/ │  │ (read-only     │    │
│  │           │  │   Code)   │  │  Cursor)   │  │  v0.2)         │    │
│  └─────┬─────┘  └─────┬─────┘  └─────┬──────┘  └────────────────┘    │
│        │              │              │                                 │
└────────┼──────────────┼──────────────┼─────────────────────────────────┘
         │              │              │
         │ exec         │ stdio MCP    │ file read (only wiki/, schema/)
         │              │              │
┌────────┼──────────────┼──────────────┼─────────────────────────────────┐
│        ▼              ▼              ▼          WikiMind Daemon (Go)   │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │  wikimindd  (single process per vault, 1 writer rule)            │    │
│  │                                                                  │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │    │
│  │  │ MCP Server   │  │ CLI Bridge   │  │ HTTP Bridge (v0.2)   │  │    │
│  │  │ (stdio)      │  │ (unix sock /  │  │ (localhost only)     │  │    │
│  │  │              │  │  named pipe) │  │                      │  │    │
│  │  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │    │
│  │         │                 │                     │              │    │
│  │         ▼                 ▼                     ▼              │    │
│  │  ┌────────────────────────────────────────────────────────┐   │    │
│  │  │              Request Router + Auth + Rate-Limit         │   │    │
│  │  └──────────────────────┬─────────────────────────────────┘   │    │
│  │                         ▼                                       │    │
│  │  ┌─────────────────────────────────────────────────────────┐   │    │
│  │  │                    Service Layer                         │   │    │
│  │  │  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐ │   │    │
│  │  │  │ Read    │ │ Propose  │ │ Review   │ │ Lint /      │ │   │    │
│  │  │  │ Service │ │ Service  │ │ Service  │ │ Dream Cycle │ │   │    │
│  │  │  └─────────┘ └──────────┘ └──────────┘ └─────────────┘ │   │    │
│  │  └────────────────┬────────────────────────────────────────┘   │    │
│  │                   ▼                                              │    │
│  │  ┌─────────────────────────────────────────────────────────┐   │    │
│  │  │           Single-Writer Commit Loop                      │   │    │
│  │  │  (serializes ALL wiki/ writes; enforces change_log seq)  │   │    │
│  │  └────┬─────────────────┬──────────────────┬────────────────┘   │    │
│  │       │                 │                  │                     │    │
│  │       ▼                 ▼                  ▼                     │    │
│  │  ┌─────────┐    ┌──────────────┐   ┌───────────────────┐        │    │
│  │  │  Git    │    │  Lock Mgr    │   │  Change Log /     │        │    │
│  │  │  Worker │    │ (.wikimind/  │   │  log.md /         │        │    │
│  │  │         │    │   locks/)    │   │  audit            │        │    │
│  │  └────┬────┘    └──────────────┘   └───────────────────┘        │    │
│  │       │                                                          │    │
│  │       ▼                                                          │    │
│  │  ┌──────────────────────────────────────────────────────┐       │    │
│  │  │            File-System Watcher                        │       │    │
│  │  │  (FSEvents / RDCW / inotify)                          │       │    │
│  │  └──────────────────────────────────────────────────────┘       │    │
│  └──────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │             Ingest Workers (Python, fork per job)                 │    │
│  │  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌─────────────────┐    │    │
│  │  │ md/html │  │ PDF     │  │ image    │  │ audio (whisper) │    │    │
│  │  │ parser  │  │ pdftotxt│  │ OCR      │  │                 │    │    │
│  │  └─────────┘  └─────────┘  └──────────┘  └─────────────────┘    │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────┐
│                           Filesystem                                   │
│  vault/                                                                │
│  ├── raw/         ← read-only (daemon never writes)                    │
│  ├── wiki/        ← daemon owns writes; agents work in worktrees       │
│  │   ├── _review/ ← stage area for pending proposes                    │
│  │   └── _worktrees/.gitignored                                        │
│  ├── schema/      ← user owns; daemon read-only                        │
│  └── .wikimind/   ← daemon internal (locks, db, change_log, audit)     │
│                                                                        │
│  ~/.config/wikimind/  (Linux/macOS) or %APPDATA%\WikiMind\ (Win)      │
│     ← global config, keychain refs, telemetry                         │
└──────────────────────────────────────────────────────────────────────┘
```

**核心不变量**：
- 1 vault = 1 daemon = 1 SQLite writer = 1 commit serializer
- Daemon 是 vault 的"BIOS"。所有 user 和 agent 通过 daemon 而不绕过它。

---

## 2. 进程模型

### 2.1 进程清单

| 进程 | 数量 | 生命周期 | 写权限 |
|---|---|---|---|
| `wikimindd` daemon | 1 per vault | 用户登录后启动（launchd / Scheduled Task） | `wiki/`, `.wikimind/` |
| `wikimind` CLI | 多 instances | 短命，每次命令 fork 一个 | 无（通过 daemon） |
| `wikimind mcp serve` | 多 instances | 父进程（如 Claude Code）spawn | 无（通过 daemon） |
| Ingest worker | 多，按需 fork | 单 job 生命周期 | `wiki/_review/` 的草稿（待 daemon 收编） |
| Agent process | 多（user 主动起） | 由 host 管理 | 自己的 git worktree |

### 2.2 1 vault = 1 daemon 的强制

```
启动 daemon：
  - 尝试 acquire lock: vault/.wikimind/daemon.pid
  - 文件已存在 + PID 仍活 → exit "another instance running"
  - 否则写入自己的 PID，注册 atexit handler 删除
```

这保证了**没有任何机会**两个 daemon 同时写 `wiki/`。

### 2.3 单写者承诺（Single Writer）

```
所有写操作 → 经过 Service Layer → 进入 Single-Writer Commit Loop → 实际 commit

Commit Loop 是一个 channel-based 串行队列：
  - 任意 propose / accept / lint_fix / dream_cycle 写入操作进队列
  - 队列消费者是单 goroutine
  - 每次消费：begin tx → apply patch → git commit → write change_log → tx commit
  - 失败回滚（git reset + tx rollback + 删除 change_log row）
```

这是协议安全性的物理保证：**永远不存在两个写并发**。

### 2.4 Agent worktree

每个 agent session 启动时（`agent_handshake`）：

```
1. daemon 分配 worktree: vault/wiki/_worktrees/agent-{name}-{session-id}/
2. git worktree add 创建该目录
3. 返回 worktree 路径给 agent
4. agent 在自己的 worktree 里 read/write，daemon 不直接看
5. agent 调 propose_* 时：
   - daemon 用 git diff 取 worktree vs main 的差异
   - 把差异作为 patch 写入 vault/wiki/_review/{review-id}.patch
   - worktree 本身保留（agent 可继续编辑）
6. accept 后：
   - daemon 在 main 分支应用 patch
   - 同时清理 worktree（或保留让 agent 看到 conflict）
```

**worktree 的价值**：
- 物理隔离 → 两个 agent 同时写同一文件不互相覆盖
- Git-native → 用 git 原生工具看 / 调试 agent 编辑过程
- 廉价 → worktree 共享 `.git/`，磁盘开销极小

---

## 3. 数据流

### 3.1 Ingest 流程（5 阶段）

```
[User drops file to raw/inbox/]
        ↓
[Watcher: FSEvents notifies daemon]
        ↓
┌─────────────────────────────────────────────────────────────────┐
│ Stage 1: Deliver                                                  │
│   - sha256 + size + mtime 三件套                                   │
│   - 写入 sources 表（status=pending）                              │
│   - 移动到 raw/imported/ 或保留在 inbox（按配置）                  │
│   - 触发对应 worker (md/html/pdf/image/audio)                      │
└────────────────┬────────────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────────────┐
│ Stage 2: Parse                                                    │
│   - Python worker fork 出来                                        │
│   - 解析为 (headings, paragraphs, char-spans, anchors)             │
│   - 产出 normalized.json，供 stage 3 用                            │
│   - sources.status = parsed                                        │
└────────────────┬────────────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────────────┐
│ Stage 3: Extract claims (LLM agent)                               │
│   - daemon 调 agent (通过 MCP) 执行 claim-extraction.md 算法       │
│   - agent 在自己的 worktree 里写 propose drafts                    │
│   - 4 步算法：扫描 → 合并 → 三件套 → 自检                          │
│   - 每条 claim 产出 propose draft                                  │
└────────────────┬────────────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────────────┐
│ Stage 4: Propose                                                  │
│   - daemon 把 worktree diff 转为 patches                          │
│   - 写入 wiki/_review/r-{seq}.patch                                │
│   - 写入 review 表（status=pending）                               │
│   - 多条 propose 自动归到一个 bundle（同一 ingest 任务）           │
└────────────────┬────────────────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────────────────────────────────┐
│ Stage 5: Review queue                                             │
│   - 触发 review queue 健康检查（pending 数、优先级排序）            │
│   - 优先级排序后插入 user 可见的 list                              │
│   - 通过 priority_score 决定 bundle 在 list 中位置                 │
│   - 推 OS 通知（如 critical 或队列满）                             │
└─────────────────────────────────────────────────────────────────┘

→ User: wikimind review accept b-xxxx
        ↓
[Single-Writer Commit Loop applies patches, git commit, update index]
        ↓
[Bundle cleared, sources.status = done]
```

任何 stage 失败：
- `sources.status = error`
- 错误写入 `.wikimind/audit/ingest-errors.jsonl`
- 不影响其它正在进行的 ingest
- User `wikimind ingest --retry <source-id>` 重试

### 3.2 Query 流程

```
[User: wikimind query "wiki vs RAG?"]
        ↓
[Read Service]
        ↓
1. FTS5 BM25 (CJK-aware tokenizer)
        ↓
2. (optional) embedding rerank (only if user enabled)
        ↓
3. Graph traversal: 命中 page → 找 backrefs → 找 outbound links
        ↓
4. Construct context bundle (claims + sources for verification)
        ↓
[Agent / CLI receives result]
        ↓
[If high-quality answer → Query Sedimentation triggers propose]
```

详见 [`query-sedimentation.md`](query-sedimentation.md)。

### 3.3 Review 流程

```
[User: wikimind review accept r-0245]
        ↓
[Review Service]
        ↓
1. Validate review_id exists, status = pending
2. Read patch from wiki/_review/r-0245.patch
3. Validate quote_hash (re-verify source still matches)
4. Validate schema (frontmatter complete, types match)
5. Build commit message (with bundle context)
        ↓
[Single-Writer Commit Loop]
        ↓
6. Begin tx (SQLite)
7. Apply patch to wiki/ via git
8. Verify (re-parse the modified file; ensure valid markdown + frontmatter)
9. git add + commit
10. Write change_log row (seq++)
11. Update index (FTS5 + relations)
12. Delete patch from _review/
13. Commit tx
        ↓
[On failure at any step → full rollback, return error to user]
```

### 3.4 Watcher 与 Sync 流程

```
[Filesystem changes (Obsidian saves, rsync, manual edit)]
        ↓
[Watcher (FSEvents / RDCW / inotify) emits event]
        ↓
[Event Debouncer (200ms batch)]
        ↓
[Reconciler]
        ↓
- raw/ change → sources 表更新 sha256，若已被 wiki claim 引用且 hash 变 → DRIFT 通知
- wiki/ change（user 手动改）→ index update + git auto-commit on schedule
- schema/ change → schema_version check; if breaking → blocking notification
- .wikimind/ change → 警告（不应被外部 touch）
```

Watcher 不可靠？参考方案 A 的 R-04：

- 启动时全扫一次
- 每小时 reconcile（对 sources 表全扫 mtime + size）
- 每日抽样 5% 重算 sha256
- `wikimind reconcile` 手动触发

### 3.5 Direct read（绕过 daemon 的轻量读）

Agent 不一定通过 daemon 读 vault——它可以直接 `cat` / `grep` / `ripgrep` 读 `raw/` 和 `wiki/`。
这是**有意支持**的能力（零依赖、最快），不是绕过：

- 读不改变任何状态——单一闸门保护的是写，不是读
- 代价：直接读不进 audit（可接受）
- 唯一约束：抽 claim 的 `quote_hash` 仍须经 `read_raw_anchor`，或在 `propose_claim` 时被 daemon 校验

完整的「5 条访问路径」模型 + 读写分离见 [`filesystem-access.md`](filesystem-access.md)。

---

## 4. 存储模型

### 4.1 Vault 目录全貌

```
vault/
├── raw/
│   ├── inbox/                       ← user drops files here
│   ├── imported/                    ← daemon moves after ingest
│   ├── attachments/                 ← user-curated additional sources
│   └── manifests/                   ← raw_id ↔ original-source URL mapping (LFS)
│
├── wiki/                            ← agent + user maintained
│   ├── index.md                     ← MUST be read first by agents
│   ├── log.md                       ← append-only event log
│   ├── claims/                      ← claim pages (one per claim)
│   ├── entities/                    ← person/org/product
│   ├── concepts/                    ← abstract ideas
│   ├── sources/                     ← per-source summary pages
│   ├── topics/                      ← user-curated grouping
│   ├── _review/                     ← pending proposes (drafts + patches)
│   │   ├── r-0245.patch
│   │   ├── r-0246.patch
│   │   ├── b-0042.meta.json         ← bundle metadata
│   │   └── ...
│   ├── _worktrees/                  ← git worktree per agent (gitignored)
│   │   ├── agent-claude-sess-A1/
│   │   └── agent-codex-sess-B2/
│   └── _reports/                    ← Dream Cycle output (weekly)
│
├── schema/                          ← user-owned contract
│   ├── AGENTS.md                    ← baseline rules (all agents must obey)
│   ├── CLAUDE.md                    ← Claude Code addendum (only stricter)
│   ├── CODEX.md
│   ├── HERMES.md
│   ├── CURSOR.md
│   ├── page-schemas.md              ← frontmatter schemas for each page type
│   └── lint-rules.md                ← lint rule definitions
│
├── .wikimind/                       ← daemon internal (DO NOT touch)
│   ├── config.toml                  ← vault config
│   ├── daemon.pid                   ← single-writer lock
│   ├── index.db                     ← SQLite (FTS5, relations, sources, reviews)
│   ├── change-log.jsonl             ← machine-readable change log (1:1 git commits)
│   ├── rejections.jsonl             ← rejected proposes (for agent memory)
│   ├── audit/
│   │   ├── ingest-errors.jsonl
│   │   ├── auth-events.jsonl
│   │   └── ...
│   └── locks/
│       ├── <page-id>.lock           ← advisory locks
│       └── ...
│
└── .git/                            ← git repo for wiki/ + schema/
                                       (raw/ may be tracked or in LFS or in .gitignore)
```

### 4.2 SQLite Schema 关键表

```sql
-- 资料追溯
CREATE TABLE sources (
    raw_id      TEXT PRIMARY KEY,    -- raw/inbox/karpathy-llm-wiki.md
    sha256      TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mtime       INTEGER NOT NULL,
    status      TEXT NOT NULL,        -- pending | parsed | done | error
    ingested_at INTEGER,
    parser      TEXT,
    metadata    JSON                  -- parser-specific
);

-- 页面元数据（rebuildable from markdown）
CREATE TABLE pages (
    id          TEXT PRIMARY KEY,    -- cl-2026-05-21-001
    type        TEXT NOT NULL,        -- claim | entity | concept | source | topic
    path        TEXT NOT NULL UNIQUE, -- wiki/claims/wiki-is-compounding.md
    title       TEXT NOT NULL,
    confidence  REAL,
    status      TEXT,                 -- supported | unverified | ...
    schema_ver  TEXT NOT NULL,
    created_by  TEXT,
    updated_by  TEXT,
    created_at  INTEGER,
    updated_at  INTEGER,
    frontmatter JSON                  -- full frontmatter for query
);

-- Full-text search (CJK-aware, see cjk-tokenizer.md)
CREATE VIRTUAL TABLE pages_fts USING fts5(
    id UNINDEXED,
    title,
    body,
    tokenize = 'trigram'              -- or jieba; not unicode61
);

-- Claim ↔ Source links (with quote_hash)
CREATE TABLE claim_sources (
    claim_id    TEXT NOT NULL,
    raw_id      TEXT NOT NULL,
    anchor      TEXT NOT NULL,
    quote       TEXT NOT NULL,
    quote_hash  TEXT NOT NULL,
    span_start  INTEGER,
    span_end    INTEGER,
    verified_at INTEGER,
    status      TEXT NOT NULL,        -- verified | drift | missing
    PRIMARY KEY (claim_id, raw_id, anchor)
);

-- Outbound links (wiki link graph)
CREATE TABLE page_links (
    from_id   TEXT NOT NULL,
    to_id     TEXT NOT NULL,
    link_type TEXT,                  -- mention | refers | related
    PRIMARY KEY (from_id, to_id)
);

-- Reviews
CREATE TABLE reviews (
    review_id     TEXT PRIMARY KEY,   -- r-0245
    bundle_id     TEXT,
    agent         TEXT NOT NULL,
    page_id       TEXT,
    operation     TEXT NOT NULL,      -- create | edit | delete | merge
    status        TEXT NOT NULL,      -- pending | accepted | rejected | superseded | merged
    priority_score INTEGER,
    patch_path    TEXT,
    created_at    INTEGER,
    decided_at    INTEGER,
    decided_by    TEXT,
    decision_reason TEXT
);

CREATE TABLE bundles (
    bundle_id   TEXT PRIMARY KEY,    -- b-0042
    title       TEXT,
    kind        TEXT,                 -- ingest | lint_fix | query_sediment | dream_cycle | custom
    agent       TEXT,
    status      TEXT,
    created_at  INTEGER
);

-- Change log (1:1 git commits)
CREATE TABLE change_log (
    seq         INTEGER PRIMARY KEY AUTOINCREMENT,
    git_sha     TEXT NOT NULL UNIQUE,
    timestamp   INTEGER NOT NULL,
    actor       TEXT,
    operation   TEXT,                 -- accept | revert | auto-accept | manual-edit
    bundle_id   TEXT,
    review_ids  JSON,
    summary     TEXT
);

-- Locks (advisory)
CREATE TABLE locks (
    target      TEXT PRIMARY KEY,    -- page id, file path
    holder      TEXT NOT NULL,        -- agent name + session id
    acquired_at INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    purpose     TEXT
);

-- Agent handshake history
CREATE TABLE agent_sessions (
    session_id  TEXT PRIMARY KEY,
    agent       TEXT NOT NULL,
    schema_ver  TEXT NOT NULL,
    started_at  INTEGER NOT NULL,
    ended_at    INTEGER,
    worktree    TEXT
);
```

**关键设计**：所有 SQLite 数据**都是派生的**——可从 wiki/*.md + .git/ + .wikimind/change-log.jsonl 完全重建。
`wikimind rebuild-index` 是兜底命令，用 markdown 是真理源的承诺保证 100% 可重建。

### 4.3 Git 布局

```
.git/
├── HEAD                   → refs/heads/main
├── refs/heads/main        ← 正式 wiki 分支（only daemon writes）
├── refs/heads/wt-<sess>   ← agent worktree 分支
├── worktrees/             ← git worktree metadata
└── objects/               ← shared object store
```

- raw/ 默认进 git（小文件）+ LFS（PDF/图片/音频）
- wiki/ + schema/ 100% 进 git
- .wikimind/ 中只有 `change-log.jsonl` 进 git（审计基线），其它 gitignored

---

## 5. 状态机

### 5.1 Source 状态机

```
inbox → parsing → parsed → extracting → extracted → done
                    ↓                       ↓
                  error                   error
                    ↓                       ↓
            (user retry / 修 parser)   (agent 重试 / 修 prompt)
```

### 5.2 Claim 状态机

见 [`SPEC.md §3.3`](../SPEC.md#33-claim-状态机)。

### 5.3 Review 状态机

```
            ┌──────────────┐
            │   pending    │
            └──────┬───────┘
                   ↓
        ┌──────────┼──────────┐
        ▼          ▼          ▼
    accepted   rejected   superseded
        │
        ▼
     merged (in git, change_log written)
```

详见 [`review-queue-policy.md`](review-queue-policy.md)。

### 5.4 Lock 状态机

```
free → acquired(holder, TTL) → ┬─ released
                               ├─ expired (TTL passed)
                               └─ force-broken (admin)
```

Lock 是 advisory（agent 自觉遵守），daemon 不阻止物理写，只在 commit 时检查。

---

## 6. 索引模型

### 6.1 索引层级

```
真理源       ← markdown files + git
   ↓
派生层 1    ← SQLite (relations, fts5, sources, reviews)
   ↓
派生层 2    ← (optional) embedding store (LanceDB / sqlite-vec)
```

派生层 1 必须可由真理源完全重建。
派生层 2 是可选缓存，丢失只影响 search 召回率。

### 6.2 Reconcile

```
wikimind reconcile [--full]
```

- 扫描 vault/wiki/, vault/raw/
- 对比 SQLite 中元数据
- 修复差异（重新解析变更的文件、删除幽灵记录、补缺失的）
- 输出 diff 报告（user 决定是否接受）

### 6.3 Rebuild

```
wikimind rebuild-index
```

- 删除 SQLite db，从头扫描 wiki/ + raw/ 重建
- 用于 SQLite 损坏 / 升级 schema / 切换 tokenizer

---

## 7. 错误传播路径

```
任何错误首先记录到 .wikimind/audit/ → 然后按类型决定通知 user 程度

类别                     位置                          通知
─────                    ────                           ────
ingest error             ingest-errors.jsonl            review queue 显示 + log.md
schema violation         schema-violations.jsonl        阻断 propose + tip 显示
quote drift              drift-events.jsonl              banner + 触发 reverify
permission denied        auth-events.jsonl               立即弹窗
git conflict             git-conflicts.jsonl             review queue 显示，user 决议
lock timeout             locks-events.jsonl              warning + 自动 expire
config invalid           startup-errors.jsonl            daemon 启动失败 + stderr
```

所有错误**永不静默丢失**。审计文件保留 90 天，weekly Dream Cycle 输出总结。

---

## 8. 性能预算

| 操作 | 目标 | 测量条件 |
|---|---|---|
| `wikimind query "..."` | < 100ms p95 | 10k pages, CJK trigram FTS5 |
| Ingest 1 篇 markdown (10KB) | < 5s | claim extraction 包含在内 |
| Review accept (single) | < 200ms | 含 git commit + index update |
| Review accept (bundle of 10) | < 1s | |
| Watcher → reconcile (1 file change) | < 200ms | 含 hash + index update |
| Full reconcile (10k files) | < 30s | |
| Lint full (10k pages) | < 60s | incremental: < 5s |
| Daemon RSS | < 200 MB | 10k pages, 4 active agents |

性能不达标 → 优先级 P0 bug。

---

## 9. 与 SPEC 的关系

本文档详化 SPEC §2-§4 的"是什么"为"怎么实现"。
任何 SPEC 中提到的结构（三层、Claim、review queue、worktree）都在本文档有对应的物理实现描述。
新增功能必须先更 SPEC，再更本文档，再实现。

---

## 10. 不在范围

- 跨设备同步架构 → v2.0（CRDT / sync server）
- 移动端架构 → v1.5（read-only client + push notification）
- 多 user 团队架构 → v0.2+（permissions, roles, multi-writer 协调）
- Web dashboard 架构 → v0.2

---

## 一句话总结

> 1 vault = 1 daemon = 1 SQLite writer = 1 commit serializer。三层文件结构 + worktree 物理隔离 +
> 单写者串行化 commit + 5 阶段 ingest pipeline = WikiMind 的物理骨架。所有派生数据可重建；
> markdown + git 是永恒真理源。
