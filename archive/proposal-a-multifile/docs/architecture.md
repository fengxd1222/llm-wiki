# MVP 技术架构

> 配套 `REPORT.md` 的"高层方案"，这里给出**可直接动手实现**的架构、组件、数据流、性能方案、选型对比。

---

## 1. 系统视图

```
                ┌────────────────────────────────────────────────────────┐
                │                  User-facing surfaces                  │
                │                                                        │
                │  Claude Code   Codex CLI   Cursor   Continue   ...    │
                │  Hermes        OpenCode    Cline    自研 agent         │
                └─────────────┬──────────────┬───────────────┬──────────┘
                              │ MCP (stdio)  │ shell exec    │ direct FS
                              ▼              ▼               ▼
              ┌───────────────────────────────────────────────────────┐
              │                  llmwiki core daemon                  │
              │                                                       │
              │  ┌─────────────┐  ┌────────────┐  ┌────────────────┐  │
              │  │ MCP server  │  │  CLI (llm  │  │ Long-running   │  │
              │  │ (stdio/sse) │  │   wiki)    │  │ daemon (watch  │  │
              │  │             │  │            │  │  + lint cron)  │  │
              │  └──────┬──────┘  └──────┬─────┘  └────────┬───────┘  │
              │         │                │                 │          │
              │         └────────┬───────┴────────┬────────┘          │
              │                  ▼                ▼                   │
              │       ┌─────────────────────────────────────┐         │
              │       │           Core services             │         │
              │       │   ingest  search  lint  review      │         │
              │       │   change-log  lock-manager          │         │
              │       └────────┬────────────┬───────────────┘         │
              │                │            │                         │
              └────────────────┼────────────┼─────────────────────────┘
                               ▼            ▼
                    ┌─────────────────┐  ┌────────────────────┐
                    │  Index store    │  │   Vault on disk     │
                    │  (SQLite FTS5 + │  │   raw/  wiki/       │
                    │   sqlite-vec)   │  │   schema/  .git/    │
                    └─────────────────┘  └────────────────────┘
```

**关键边界**：

- 所有 agent 都是 client；wiki vault 是被服务的对象。
- `llmwiki core` 是唯一可以写 `.llmwiki/` 内部状态的进程；它也是唯一持有锁、唯一负责 git commit 的进程。
- Agent 可以通过 MCP / CLI / 直接读 markdown 文件。**写**强烈建议走 MCP/CLI（保证 review queue + change log
  + lock 三件套），但 MVP 也允许"直接 git 写 + 事后 reconcile"作为兜底。

---

## 2. 组件分解

### 2.1 Core daemon（`llmwikid`）

长期常驻进程，跨平台。职责：

| 子模块 | 职责 |
|---|---|
| watcher | macOS FSEvents / Windows ReadDirectoryChangesW / Linux inotify；polling 兜底 |
| indexer | 增量索引（mtime + sha256）；写 `.llmwiki/index.db` |
| lock-manager | 文件级 advisory lock（基于 `.llmwiki/locks/<pageid>.lock` + PID） |
| change-log | 追加 `.llmwiki/change-log.jsonl`；与 git commit 1:1 |
| review-runner | 处理 review queue 的 accept/reject |
| lint-runner | cron 调度（macOS launchd / Windows Task Scheduler） |
| git-bridge | 自动 commit（每条 change log → 一个 commit）；可选 push |

实现：建议 **Rust 或 Go**，理由：单文件二进制、跨平台、watcher 性能好、没有 runtime 依赖。
若团队更熟悉 Python，可用 Python + `watchdog` + PyInstaller，但要接受冷启动慢和打包痛苦。

### 2.2 CLI（`llmwiki`）

薄客户端，通过 IPC / RPC 调用 daemon。命令清单：

```
llmwiki init <vault>              # 初始化 vault，写默认 schema
llmwiki status                    # 健康度、索引大小、待 review 数量
llmwiki ingest <file>             # 触发 agent 处理一个 raw 文件
llmwiki query "<question>"        # 查询 wiki（CLI 模式，调本地 agent）
llmwiki lint [--fix]              # 跑 lint
llmwiki review list / accept / reject / diff <id>
llmwiki log [--tail N]            # 看 log.md / change-log.jsonl
llmwiki rebuild-index             # 从 markdown 完全重建索引
llmwiki agent-context <agent>     # 输出某 agent 需要的"运行时上下文"
llmwiki claim list / show / verify <id>
llmwiki page show / new / move / merge <id>
llmwiki mcp serve                 # 启动 MCP server（stdio 模式）
llmwiki doctor                    # 自检：路径、权限、git、watcher
```

### 2.3 MCP server（`llmwiki mcp serve`）

stdio transport（首选，最大兼容 Claude Code / Cursor），可选 SSE。
工具集见 `docs/mcp-tools.md`，核心思路：

- **只读工具**：免确认、可被 agent 自由调用。
- **写工具**：所有"写"都是"提案到 review queue"，**不直接修改 wiki 文件**。
- **管理工具**：lock / unlock / log_append 等。

### 2.4 Vault 结构

见 `examples/directory-tree.md`，要点：

- `raw/`：**只读**（程序约定，不靠 fs 权限强制，否则跨平台麻烦）。
- `wiki/`：agent 可写区。
- `schema/`：手写，提交进 git，演化由用户主导。
- `.llmwiki/`：派生数据，大部分 git-ignore，但 `.llmwiki/change-log.jsonl` 进 git。

---

## 3. 数据流

### 3.1 Ingest 数据流（详细）

```
[1] raw/inbox/<file>          ← 用户 drag/clipper/CLI 投递
[2] watcher 触发 event        → 计算 sha256
[3] indexer:
      INSERT INTO sources(id, path, hash, mtime, type, status='pending')
[4] daemon 调用 notification hook:
      - 写一行到 .llmwiki/ingest-queue.jsonl
      - （可选）通过 MCP `notify_source_added` 唤起 agent
[5] agent（Claude Code / Codex / ...）:
      - 调 MCP tool `read_raw(<id>)`
      - 与 user 对话："要不要 ingest？"
      - 调 MCP tool `propose_page(...)`  → 写 wiki/_review/<draft-id>.md
      - 调 MCP tool `propose_edits(...)` → 批量提议 entity / concept 更新
      - 调 MCP tool `log_append("ingest | …")` → 追加到 log.md
[6] user 在 CLI / 即将到来的 UI 里:
      llmwiki review list
      llmwiki review diff <draft-id>
      llmwiki review accept <draft-id>
[7] daemon:
      - 移动 draft 到正式 wiki 路径
      - git add + commit "ingest: <title> (drafted by <agent>, accepted by <user>)"
      - INSERT INTO change_log
      - 更新 sources.status='ingested'
```

### 3.2 Query 数据流（详细）

```
[1] agent 收到用户问题
[2] agent 调 MCP `read_page("wiki/index.md")`        ← 强制必读
[3] agent 调 MCP `search(query, k=20, filter={...})`
      内部:
        - SQLite FTS5 BM25
        - 可选: embedding (sqlite-vec) rerank
        - 返回 page_id + 摘要片段
[4] agent 调 MCP `read_page(<top-k-ids>)`
[5] agent 调 MCP `read_claim_sources(<claim-id>)`   ← 回溯 source
[6] agent 调 MCP `read_raw_anchor(<raw-id>, <anchor>)`
[7] agent 综合答案，附 citation
[8] agent 调 MCP `propose_page("queries/<auto-id>")` 提议归档（用户决定）
```

### 3.3 Lint 数据流

```
[scheduler tick] (cron / launchd / Task Scheduler) →
  daemon:
    - 全表扫 wiki/
    - 检测：orphan / broken-link / contradiction / stale / unverified / dup-entity / schema-violation
    - 写 .llmwiki/lint-report-<date>.jsonl
    - 把待修复项推到 review queue
  agent（可选 / 用户触发）:
    - 读 lint report
    - 提议合并、补 source、补 claim
```

---

## 4. 数据模型（持久化层）

### 4.1 Markdown 文件（事实来源）

详见 `templates/page-schemas.md`。

### 4.2 SQLite schema（派生）

```sql
-- 资料层
CREATE TABLE sources (
  id          TEXT PRIMARY KEY,          -- ULID, 与 wiki/sources/<id>.md 对应
  path        TEXT NOT NULL UNIQUE,      -- 相对 vault 根的 POSIX 路径
  hash_sha256 TEXT NOT NULL,
  mtime       INTEGER NOT NULL,
  size        INTEGER NOT NULL,
  mime        TEXT,
  status      TEXT NOT NULL,             -- pending | ingested | failed
  added_at    INTEGER NOT NULL,
  ingested_at INTEGER
);
CREATE INDEX idx_sources_status ON sources(status);

-- Wiki 页面层
CREATE TABLE pages (
  id        TEXT PRIMARY KEY,            -- ULID
  path      TEXT NOT NULL UNIQUE,
  type      TEXT NOT NULL,               -- source | claim | entity | concept | topic | query | misc
  title     TEXT NOT NULL,
  hash      TEXT NOT NULL,               -- content sha256 for change detection
  created   INTEGER NOT NULL,
  updated   INTEGER NOT NULL,
  status    TEXT NOT NULL                -- draft | published | stale | archived
);
CREATE INDEX idx_pages_type ON pages(type);
CREATE INDEX idx_pages_status ON pages(status);

-- 全文索引
CREATE VIRTUAL TABLE pages_fts USING fts5(
  id UNINDEXED,
  title,
  body,
  tokenize = 'unicode61 remove_diacritics 2'
);

-- Claim 一等公民
CREATE TABLE claims (
  id          TEXT PRIMARY KEY,
  page_id     TEXT NOT NULL,             -- → pages.id
  text        TEXT NOT NULL,
  confidence  REAL NOT NULL,             -- 0..1
  status      TEXT NOT NULL              -- unverified | verified | disputed | retracted
);
CREATE TABLE claim_sources (
  claim_id    TEXT NOT NULL,
  source_id   TEXT NOT NULL,
  anchor      TEXT,                      -- char offset / heading / quote hash
  quote_hash  TEXT,                      -- 引用文本的 sha256（防内容漂移）
  PRIMARY KEY (claim_id, source_id, anchor)
);
CREATE TABLE claim_relations (
  claim_id    TEXT,
  rel         TEXT,                      -- supports | contradicts | refines
  other_id    TEXT,
  PRIMARY KEY (claim_id, rel, other_id)
);

-- 关系图（entity ↔ entity / entity ↔ concept）
CREATE TABLE relations (
  src_id  TEXT NOT NULL,
  dst_id  TEXT NOT NULL,
  rel     TEXT NOT NULL,                 -- works_at | uses | extends | located_in | ...
  source_claim_id TEXT,                  -- 关系背后的 claim（强制可追溯）
  PRIMARY KEY (src_id, rel, dst_id)
);

-- 变更日志
CREATE TABLE change_log (
  seq        INTEGER PRIMARY KEY AUTOINCREMENT,
  ts         INTEGER NOT NULL,
  agent      TEXT NOT NULL,              -- claude-code | codex | hermes | user | system
  op         TEXT NOT NULL,              -- create | update | move | merge | delete | accept | reject
  page_id    TEXT,
  git_commit TEXT,                       -- 关联的 git hash
  payload    TEXT NOT NULL               -- JSON
);

-- Review queue
CREATE TABLE review_queue (
  id         TEXT PRIMARY KEY,
  kind       TEXT NOT NULL,              -- new_page | edit_page | merge | delete
  payload    TEXT NOT NULL,              -- JSON, 含 diff
  proposed_by TEXT NOT NULL,             -- 哪个 agent
  proposed_at INTEGER NOT NULL,
  status     TEXT NOT NULL,              -- pending | accepted | rejected
  decided_by TEXT,
  decided_at INTEGER
);

-- 可选向量索引（用 sqlite-vec 扩展）
CREATE VIRTUAL TABLE page_embeddings USING vec0(
  id TEXT PRIMARY KEY,
  embedding FLOAT[768]
);
```

### 4.3 Change log（机器可读）

`.llmwiki/change-log.jsonl`：

```jsonl
{"seq":1,"ts":1715200000,"agent":"claude-code","op":"propose_page","page_id":"01J5...","review_id":"r-001","git_commit":null}
{"seq":2,"ts":1715200200,"agent":"user","op":"accept","review_id":"r-001","page_id":"01J5...","git_commit":"abc123"}
```

每条 jsonl 行 = 一次原子操作。与 git commit 1:1。可以重放重建状态。

---

## 5. 性能方案

> 目标：**MVP 段 1k–10k 文件无延迟**；**v1 段 100k 文件可用**；**v2 段 1M 文件不崩**。

### 5.1 规模分段策略

| 规模 | 资料量 | 索引技术 | 搜索策略 | 关键风险 |
|---|---|---|---|---|
| S | 100–1k | 不需要索引，`index.md` + `ripgrep` 够 | grep + LLM rerank | 用户耐心 |
| M | 1k–10k | SQLite FTS5 | BM25 only | watcher 抖动 |
| L | 10k–100k | FTS5 + sqlite-vec | hybrid (BM25 + vec rerank) | embedding 成本 |
| XL | 100k–1M | 分片 FTS5 + LanceDB/Qdrant | hybrid + filter | watcher 风暴、git 性能 |

**关键洞察**：S 段不要上 embedding。`index.md` + ripgrep + LLM rerank 在 100 篇资料时甚至更准（因为 LLM 看
得到全貌而不是片段）。Karpathy 文中也明确说了"works surprisingly well at moderate scale (~100 sources, ~hundreds of pages)"。

### 5.2 增量索引算法

```
on_event(path):
  if ignored(path): return
  st = stat(path)
  current = db.get_source(path)
  if current and current.mtime == st.mtime and current.size == st.size:
      return                              # 快速短路
  hash = sha256(file)
  if current and current.hash == hash:
      db.update(path, mtime=st.mtime)     # 只更新 mtime
      return
  # 真的变了
  enqueue(reindex_job, path)
```

`reindex_job`：
1. 解析（md / pdf / html / docx → text）
2. 抽取 frontmatter + headings + claim
3. 更新 `pages`、`pages_fts`、`claims`、`relations`
4. 若启用 embedding，异步扔到 embedding 队列
5. 写 change_log

**并发**：Watcher event 进 lock-free queue；worker pool（默认 CPU/2）消费；每个 worker 持自己的 SQLite 连接，
WAL 模式开启。

**WAL 配置**：

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA wal_autocheckpoint = 1000;
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 268435456;       -- 256MB
PRAGMA cache_size = -200000;         -- 200MB cache
```

### 5.3 搜索性能预算

| 操作 | S 段 (1k) | M 段 (10k) | L 段 (100k) |
|---|---|---|---|
| 全文搜索 P95 | < 50ms | < 200ms | < 500ms |
| 单页读 | < 5ms | < 5ms | < 10ms |
| 增量索引 / 文件 | < 100ms | < 200ms | < 500ms |
| Lint 全扫 | < 5s | < 60s | < 10min（分批） |
| Embedding 1 page | 50–500ms（网络） | — | 同 |

L 段以上 lint 必须 incremental（只扫 dirty pages）。

### 5.4 各类资料的 ingest 策略

| 类型 | 转换工具 | 注意 |
|---|---|---|
| Markdown | 原样 | UTF-8 normalize NFC，统一 LF |
| PDF | `pdftotext -layout` 或 `pymupdf` | 多栏论文要 `-layout`；扫描件走 OCR |
| HTML / 网页 | Obsidian Web Clipper / `pandoc -t markdown_strict` / `readability-cli` | 优先 clipper |
| DOCX | `pandoc -t markdown_strict` | 嵌入图片单独抽出 |
| 图片 | macOS `shortcuts` + Vision / Windows PowerToys OCR / Tesseract | 把 OCR 文本写到 source page，原图留 `raw/images/` |
| 音频 | `whisper.cpp` 本地 | 长音频分段，每段独立 claim |
| 视频 | `ffmpeg` 抽音轨 → whisper | 加上字幕 SRT 时间戳作为 anchor |
| Slack / 聊天 | export 工具 → 拆 thread → 每 thread 一个 source | 注意脱敏 |
| 邮件 | `mbox` → 逐封 markdown | header 信息保留 |

### 5.5 缓存

- LLM 的回答缓存：query hash + wiki snapshot hash → 答案，TTL 24h。
- Embedding 缓存：page content hash → embedding，永久。
- PDF/OCR 结果缓存：source hash → 文本，永久（`.llmwiki/cache/`）。

---

## 6. 技术选型对比

### 6.1 实现语言

| 候选 | 优势 | 劣势 | 决议 |
|---|---|---|---|
| **Rust** | 零运行时、跨平台二进制、watcher 库成熟 | 开发成本高 | MVP 后期 / v1 推荐 |
| **Go** | 单二进制、并发模型友好、watcher OK | 文件解析生态弱（PDF/DOCX 要调外部） | 推荐 MVP（折衷最佳） |
| Python + watchdog | 生态最好（pandoc、pdf、tesseract 绑定） | 打包痛苦、冷启动慢 | MVP 快速原型可用，正式版换掉 |
| Node.js | MCP SDK 最成熟 | watcher 性能差、文件解析弱 | 不推荐 daemon；只用于写 MCP 包装 |

**MVP 推荐：Go 写 daemon + CLI；Python 写 ingest worker（pandoc/whisper/ocr 依赖最方便）。**
两者通过 stdin/stdout JSON 通信。`mcp server` 可以用 Go 直接写，也可以走 [official MCP SDK](https://modelcontextprotocol.io/)。

### 6.2 索引引擎

| 候选 | 适合规模 | 优势 | 劣势 |
|---|---|---|---|
| **SQLite FTS5** | < 100k | 单文件、零运维、跨平台 | 不支持向量（要加 sqlite-vec） |
| sqlite-vec | < 1M | 与 FTS5 同库 | 比专用 DB 慢 |
| LanceDB | < 10M | 列式、Arrow 兼容 | 多一个 store |
| Qdrant (local) | 任意 | 性能强 | 资源占用 |
| Tantivy | 任意 | Rust 原生、性能强 | 不是 SQL |
| ripgrep | 任意 | 零索引、极简 | 不支持 ranking |

**MVP 决议：FTS5（必）+ ripgrep（兜底）+ sqlite-vec（可选）**。
L 段以后再评估 LanceDB / Tantivy。

### 6.3 Watcher

| 平台 | 库 | 备注 |
|---|---|---|
| macOS | FSEvents（`fsnotify`/`notify-rs`） | 网络盘、外置盘退化为 polling |
| Windows | `ReadDirectoryChangesW` | 长路径要 `\\?\` 前缀；NTFS USN journal 可选高级方案 |
| Linux | inotify | 注意 `inotify_max_user_watches` |

跨平台库：Go 的 [fsnotify](https://github.com/fsnotify/fsnotify)、Rust 的 [notify](https://github.com/notify-rs/notify)。
两者 MVP 都够用。

### 6.4 Git 集成

- 自动 commit：每个 review accept = 一个 commit；message 模板 `<op>: <page-title> (drafted by <agent>, accepted by <user>) #<change-log-seq>`。
- LFS：图片/PDF/音频用 Git LFS 或干脆 git ignore，外部存储 + 内容 hash 引用。
- 大仓库优化：`git config core.fsmonitor true`（git ≥ 2.36），`git maintenance start`。

### 6.5 MCP 实现

- 选 stdio 而非 HTTP/SSE：兼容性最好、Claude Code 原生、零端口冲突。
- 工具数量保持在 12–20 个；避免过多工具拖累 LLM 选择性能。

### 6.6 与 qmd 的关系

[qmd](https://github.com/tobi/qmd) 提供 hybrid 搜索 + MCP server。两种集成方式：

1. **直接 wrap**：`llmwiki search` 在 L 段以后切到 qmd 后端。
2. **并存**：用户自由选择，配置文件 `search_backend = "fts5" | "qmd"`。

推荐 v0.2 之后做 qmd 适配器。

### 6.7 Obsidian 关系

LLM Wiki vault 同时是一个合法的 Obsidian vault：

- `wiki/` 是 Obsidian 默认的入口；
- frontmatter 字段与 Dataview 兼容；
- `[[id]]` 链接 Obsidian 原生支持；
- 图片放 `raw/images/`，frontmatter 指向；
- `_review/`、`_inbox/` 在 Obsidian 设置里 hide。

不做 Obsidian 插件（v2 再考虑），CLI/MCP 即够。

---

## 7. 部署模型

### 7.1 安装

```
# macOS
brew install llm-wiki/tap/llmwiki
llmwiki init ~/Documents/my-wiki

# Windows
winget install LLMWiki.llmwiki  # 或 scoop install llmwiki
llmwiki init %USERPROFILE%\Documents\my-wiki

# Linux
curl -fsSL https://llmwiki.io/install.sh | sh
llmwiki init ~/my-wiki
```

### 7.2 后台进程

| 平台 | 机制 | 文件 |
|---|---|---|
| macOS | launchd | `~/Library/LaunchAgents/io.llmwiki.daemon.plist` |
| Windows | Service / Scheduled Task | `Register-ScheduledTask` |
| Linux | systemd user unit | `~/.config/systemd/user/llmwiki.service` |

`llmwiki doctor` 检查后台状态。

### 7.3 配置文件 `.llmwiki/config.toml`

```toml
[vault]
root      = "/Users/me/Documents/my-wiki"
encoding  = "utf-8"
line_ending = "lf"

[watcher]
backend = "auto"       # auto | fsevents | rdcw | inotify | polling
poll_interval_ms = 2000

[index]
backend = "sqlite-fts5"
sqlite_path = ".llmwiki/index.db"
enable_vectors = false
embedding_model = "text-embedding-3-small"
embedding_dim = 1536

[review]
require_accept = true
auto_accept_agents = []      # 例: ["user"] 表示 user 直接写不进 review

[git]
enabled = true
auto_commit = true
auto_push = false
remote = ""

[agents]
[agents.claude-code]
instructions = "schema/CLAUDE.md"
[agents.codex]
instructions = "schema/AGENTS.md"
[agents.hermes]
instructions = "schema/HERMES.md"

[lint]
schedule = "weekly"          # off | daily | weekly | manual
fail_on = ["broken_link", "schema_violation"]

[mcp]
transport = "stdio"
allowed_clients = ["claude-code", "cursor", "codex", "*"]   # * = 允许所有
```

---

## 8. 失败模式与恢复（concise；详见 `docs/risks.md`）

| 失败 | 探测 | 恢复 |
|---|---|---|
| Watcher 漏事件 | 启动时全扫 + 周期性 reconcile | `llmwiki rebuild-index` |
| SQLite 损坏 | 启动校验 / `PRAGMA integrity_check` | 删 db + 全量重建（markdown 是真理） |
| Git 冲突 | merge 失败 | 自动 `git stash` + 报警；不擅自解 |
| Agent 写错文件 | change-log + git diff | 一键 `llmwiki revert <change-log-seq>` |
| Source 文件被改 | hash 不匹配 | flag affected claims as `needs-reverify` |
| Lock 死锁 | PID 不存在但 lock 在 | daemon 启动时清理 stale lock |
| 跨平台 case 冲突 | indexer 检测 | 拒绝 commit，提示重命名 |
| 长路径 (>260, Windows) | 创建文件失败 | `\\?\` 前缀 + frontmatter id |

---

## 9. 可观测性

- `llmwiki status` 输出：vault 大小、source 数、page 数、claim 数、pending review、最近 lint 时间、watcher
  health、git 状态。
- `.llmwiki/metrics.jsonl` 记录每分钟一个 sample（可选）。
- 不上 Prometheus / Grafana（单机产品过度设计）。

---

## 10. 安全模型（简要，详见 `docs/risks.md` 安全章节）

1. **进程边界**：daemon 仅访问 `[vault].root` 子树；硬性 path traversal 拦截。
2. **MCP 权限**：每个 client 在 `[mcp].allowed_clients` 中显式列出。
3. **写入审计**：所有写入都进 change-log + git；用户可 30 秒回滚。
4. **不向外发送数据**：core 完全本地；只有用户配置了 git remote 或 embedding API 时才发出网络请求。
5. **凭证**：embedding API key 等存 OS keychain（macOS Keychain / Windows DPAPI / Linux Secret Service）。

---

## 11. 总结

MVP 的核心是**让 Markdown 文件本身成为可被多 agent 协作的"小代码库"**：

- SQLite 是派生索引，不是真理；
- Git 是单一写入序列化点；
- Review queue 是质量门；
- MCP / CLI 是接入面；
- 跨平台靠纪律（POSIX 内部表示、UTF-8、kebab-case）而不是黑魔法。

所有"看起来高级"的能力（embedding、dream cycle、矛盾检测）都是可选模块，**默认关闭**。先把 boring 部分做对，
再谈智能。
