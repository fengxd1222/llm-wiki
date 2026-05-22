# 工程决策（W1 前置）

> `roadmap-30d.md` 给出 D1–D30，但 4 个技术点只点到未展开——它们在 W1 第一天就会撞上。
> 本文档在 W0 启动日把它们**定死**，避免 W1 反复返工。
>
> 4 个技术点：daemon↔worker IPC / MCP server 进程模型 / SQLite migration / Go 模块划分。

---

## 1. Daemon ↔ Worker IPC

### 1.1 问题

`wikimindd`（Go）需要调用 ingest worker（Python）做 parse / OCR / transcribe。两个进程怎么通信？

### 1.2 决策：stdin/stdout + NDJSON，不用 socket

```
daemon                              worker (Python, fork per job)
  │                                   │
  │ exec.Command("python3",           │
  │   "worker/main.py")               │
  │                                   │
  │ ──── 任务 JSON (1 行) ──────────▶ │ stdin 读一行
  │                                   │ parse / ocr / transcribe
  │ ◀──── NDJSON 进度流 ───────────── │ stdout 逐行写
  │       {"type":"progress",...}     │
  │       {"type":"progress",...}     │
  │ ◀──── 最终结果 ────────────────── │
  │       {"type":"result",...}       │
  │                                   │ exit 0
  │ ◀──── (失败时) stderr + exit≠0    │
```

**为什么不用 Unix socket / named pipe**：

- worker 是**短命**进程（architecture.md §2.1：fork per job），不是常驻服务——socket 的意义（多路复用、长连接）用不上
- daemon 是 worker 的**父进程**，天然持有 worker 的 stdin/stdout pipe，零额外设置
- stdin/stdout 跨平台**完全一致**（socket 在 Windows 要 named pipe，有差异）
- 调试简单：`echo '<task json>' | python3 worker/main.py` 就能单测 worker

### 1.3 协议

**任务（daemon → worker，stdin 一行 JSON）**：

```json
{
  "task_id": "ingest-7f3a91e4",
  "type": "parse",
  "raw_path": "/abs/path/raw/inbox/karpathy-llm-wiki.md",
  "raw_format": "markdown",
  "options": {}
}
```

`type`: `parse` | `ocr` | `transcribe` | `pdf_extract`

**输出（worker → daemon，stdout NDJSON，每行一个事件）**：

```jsonl
{"type":"progress","task_id":"ingest-7f3a91e4","stage":"parsing","pct":40}
{"type":"progress","task_id":"ingest-7f3a91e4","stage":"parsing","pct":100}
{"type":"result","task_id":"ingest-7f3a91e4","normalized":{"headings":[...],"paragraphs":[...],"anchors":[...]}}
```

- worker **只产出 normalized 中间结果**，不碰 `wiki/`、不碰 SQLite——daemon 拿到 `result` 后自己落库
- claim 抽取（stage 3）是 **LLM agent** 干的，不是 Python worker；worker 只负责 stage 2 parse 及 OCR/transcribe
- 失败：worker 退出码非 0 + stderr 写错误详情；daemon 标 `sources.status = error`

### 1.4 worker 生命周期

- 一个 job 一个 worker 进程，处理完即退出（无状态、无内存泄漏累积）
- daemon 侧并发上限：默认同时 ≤ 4 个 worker 进程（可配）
- 超时：单 worker 默认 120s（OCR/transcribe 可放宽到 600s），超时 daemon kill 之

---

## 2. MCP Server 进程模型 + SDK

### 2.1 问题（澄清一个架构含糊）

`architecture.md §1` 的图把「MCP Server」画在 `wikimindd` daemon 框内——这是**逻辑归属**（MCP 的业务由 daemon 提供）。但物理上有矛盾：

- MCP server（stdio transport）**必须由 MCP host spawn**（Claude Code 启动它作为子进程）
- 但 `wikimindd` 是**常驻** daemon（launchd / Scheduled Task 启动），不可能同时"被 Claude Code spawn"

→ 二者不能是同一个进程。

### 2.2 决策：三类进程

```
┌─ Claude Code (MCP host) ─┐
│                          │ spawn + stdio
│                          ▼
│              ┌──────────────────────────┐
│              │ wikimind mcp serve       │  ← 瘦 bridge 进程
│              │ (stdio MCP ⇄ daemon IPC) │     被 host spawn，短命随 host
│              └───────────┬──────────────┘
└──────────────────────────┼──────────────────────
                           │ IPC (unix socket / named pipe)
                           ▼
              ┌──────────────────────────┐
              │ wikimindd                │  ← 常驻 daemon
              │ (单写者，真正干活)        │     launchd / Scheduled Task
              └───────────┬──────────────┘
                           ▲
                           │ IPC
              ┌────────────┴─────────────┐
              │ wikimind <cmd>           │  ← CLI，短命
              └──────────────────────────┘
```

| 进程 | 角色 | 生命周期 |
|---|---|---|
| `wikimindd` | 常驻 daemon，单写者，持有 SQLite / git / lock | 用户登录起，常驻 |
| `wikimind mcp serve` | stdio MCP server ⇄ daemon 的 bridge | 被 MCP host spawn，随 host 终止 |
| `wikimind <cmd>` | CLI 命令，bridge 到 daemon | 短命，每命令一次 |

**关键**：`wikimind mcp serve` 和 `wikimind <cmd>` 都**不直接动 vault**——它们把请求 IPC 转发给 `wikimindd`，由 daemon 串行执行。单写者承诺不破。

> 这与 architecture.md §1 的逻辑图不冲突——图是"业务逻辑归属"，本节是"物理进程拓扑"。

### 2.3 MCP SDK 选型

- **首选**：官方 Go MCP SDK（`github.com/modelcontextprotocol/go-sdk`）
- **Fallback**：MCP 底层是 JSON-RPC 2.0 over stdio，协议简单；若官方 SDK 不成熟，自实现 stdio JSON-RPC 循环约 1–2 天工作量
- W0 验证项：拉官方 Go SDK 跑通一个 hello-world tool，决定首选还是 fallback

### 2.4 bridge ⇄ daemon 的 IPC

- 复用 §1 的思路但用 **socket**（这里需要长连接 + 多 client）：
  - macOS/Linux：Unix domain socket `~/.wikimind/<vault-hash>.sock`
  - Windows：named pipe `\\.\pipe\wikimind-<vault-hash>`
- 协议：JSON-RPC 2.0（与 MCP 同协议族，bridge 转发时几乎透传）
- 鉴权：socket 文件权限 0600（仅当前用户）；`agent_handshake` 仍走完整校验

---

## 3. SQLite Migration

### 3.1 问题

`index.db` 的 schema 会演进（加表、加列、换 tokenizer）。怎么管 migration？

### 3.2 决策：goose + embed

- 工具：[`goose`](https://github.com/pressly/goose)，Go-native migration 工具
- migration 文件 `migrations/NNNN_xxx.sql`，用 `//go:embed` **编进二进制**——用户机器不需要额外文件
- daemon 启动时自动 `goose up` 到最新版本

```
migrations/
├── 0001_initial_schema.sql
├── 0002_add_topics_table.sql
└── ...
```

### 3.3 为什么 goose 而非 golang-migrate

| | goose | golang-migrate | 自写 |
|---|---|---|---|
| 嵌入式 SQLite 支持 | ✅ 原生 | ✅ 但偏重 | ✅ |
| `//go:embed` migration | ✅ | 需配置 | ✅ |
| 体积 / 依赖 | 轻 | 较重（多 driver） | 零 |
| Go 代码内调用 | ✅ 一等 | ✅ | — |

goose 轻、嵌入友好、Go-native，对单文件 SQLite 刚好。golang-migrate 的多数据库能力用不上。

### 3.4 安全网

`index.db` 是**派生数据**（architecture.md §6.1）——migration 万一失败，最坏 `wikimind rebuild-index` 从 markdown 全量重建。所以 migration 不是高危操作，但仍要：

- 每个 migration 有对应 `-- +goose Down`
- daemon 启动 migration 前自动备份 `index.db` → `index.db.bak`
- migration 失败 → 回滚 + 用备份 + 报错，不让 daemon 带病启动

### 3.5 换 tokenizer 的特殊处理

FTS5 表的 tokenizer（trigram）不能 `ALTER`——换 tokenizer（如 v0.2 上 jieba）必须 DROP + 重建 FTS 表。这类 migration 标 `-- wikimind:rebuild-fts`，daemon 执行后自动触发 FTS 重灌（见 `cjk-tokenizer.md §6`）。

---

## 4. Go 项目模块划分

### 4.1 目录布局

```
wikimind/
├── cmd/
│   ├── wikimind/            # CLI 入口（cobra）
│   └── wikimindd/           # daemon 入口
├── internal/
│   ├── daemon/              # daemon 主循环、生命周期、单实例锁
│   ├── mcp/                 # MCP server（wikimind mcp serve 用）
│   ├── bridge/              # CLI/MCP ⇄ daemon 的 IPC（socket/pipe）
│   ├── service/             # 业务层
│   │   ├── read.go          #   read_page / search / ...
│   │   ├── propose.go       #   propose_* → _review/
│   │   ├── review.go        #   accept / reject / bundle
│   │   ├── lint.go          #   lint 13 规则
│   │   └── dream.go         #   Dream Cycle
│   ├── commit/              # Single-Writer Commit Loop（串行化所有写）
│   ├── git/                 # worktree 操作、commit、revert、revert-cascade
│   ├── lock/                # advisory lock + TTL
│   ├── index/               # SQLite、FTS5、reconcile、rebuild
│   ├── watcher/             # FSEvents / RDCW / inotify 封装
│   ├── vault/               # vault 结构、路径规范化、path-traversal 防御
│   ├── schema/              # schema 加载、schema_version 校验
│   ├── changelog/           # change-log.jsonl + log.md
│   ├── worker/              # Python worker 的 Go 侧调度（§1 的 IPC）
│   └── model/               # 数据结构：Claim/Page/Review/Bundle/Source...
├── migrations/              # SQL migration（//go:embed）
├── worker/                  # Python ingest worker（独立子项目）
│   ├── main.py              #   stdin 读任务，stdout NDJSON
│   ├── parsers/             #   markdown / html / pdf / image / audio
│   └── pyproject.toml
├── testdata/                # 测试 vault、sample raw（含 demo 的 3 个文件）
└── go.mod
```

### 4.2 依赖方向（硬规则）

```
cmd/  →  internal/daemon, internal/bridge
internal/daemon  →  service, commit, watcher, mcp
internal/service  →  index, git, lock, vault, schema, model, worker
internal/commit  →  git, index, changelog        ← 唯一能触发写的路径
internal/*  →  model                              ← model 是叶子，谁都可依赖

禁止：service 绕过 commit 直接调 git 写
禁止：model 依赖任何其它 internal 包
```

`commit` 包是**单写者的物理体现**——所有写操作的 goroutine 最终汇聚到 `commit` 包的一个 channel，由单 goroutine 消费（architecture.md §2.3）。

### 4.3 关键库选型

| 用途 | 库 | 备注 |
|---|---|---|
| CLI 框架 | `spf13/cobra` | roadmap D1 已定 |
| SQLite 驱动 | `modernc.org/sqlite` | **纯 Go**，无 cgo，跨平台编译省心；FTS5 需确认 build tag |
| SQLite migration | `pressly/goose` | §3 |
| 文件监听 | `fsnotify/fsnotify` | 封装 FSEvents/RDCW/inotify |
| YAML frontmatter | `goccy/go-yaml` 或 `gopkg.in/yaml.v3` | |
| Git 操作 | **直接 exec `git`** | 不用 go-git——worktree / LFS 支持更可靠 |
| MCP | `modelcontextprotocol/go-sdk` | §2.3，带 fallback |
| 测试 | 标准 `testing` + `testify/require` | |

> **SQLite 驱动注意**：`modernc.org/sqlite` 是纯 Go（无 cgo），跨平台静态编译最省心，但 FTS5 + trigram 要确认编译选项启用。W0 验证项：纯 Go 驱动跑通 trigram FTS5；若不行，退回 `mattn/go-sqlite3`（cgo，但 FTS5 完整）。

### 4.4 Git：为什么 exec 而非 go-git

worktree per agent、LFS、`git revert` 都是 WikiMind 重度依赖的——`go-git` 对 worktree 和 LFS 支持不完整。直接 `exec.Command("git", ...)` 行为与用户本地 git 完全一致，可调试、可复现。代价是依赖系统装了 git（`wikimind doctor` 检测）。

---

## 5. W0 启动清单（细化）

`roadmap-30d.md` 的 W0 在此落到可执行：

| 项 | 决策 |
|---|---|
| Go 版本 | 1.22+（泛型 + 标准库成熟） |
| Python 版本 | 3.11+（worker） |
| SQLite | ≥ 3.40（trigram 稳定，见 cjk-tokenizer.md §7） |
| 包管理 | Go modules；Python 用 `uv` + `pyproject.toml` |
| Lint/format | `golangci-lint` + `gofmt`；`ruff`（Python） |
| CI | GitHub Actions，矩阵见 `cross-platform.md §8`（macOS 14/15 + Win 11 + Ubuntu 22/24） |
| 仓库骨架 | §4.1 的目录树；W0 即 `go mod init` + 建空包 + 各包一个 `doc.go` |
| 烟雾测试 | W0 结束：`go build ./...` 全绿 + 一个跨进程 IPC hello-world（daemon ⇄ bridge）通 |

### 5.1 W0 三个验证项（降低 W1 风险）

W0 半天内必须验证（否则 W1 会卡）：

1. **纯 Go SQLite + trigram FTS5** 能跑 → 决定 SQLite 驱动（§4.3）
2. **官方 Go MCP SDK** 能跑 hello-world tool → 决定 MCP 用 SDK 还是自实现（§2.3）
3. **daemon ⇄ bridge IPC**（socket/pipe）跨进程跑通一个 echo → 验证 §2.4 的进程模型

三项都绿 → W1 按 roadmap 开干；任一红 → 当天用 fallback 方案，不拖到 W1。

---

## 6. 决策汇总

| # | 技术点 | 决策 |
|---|---|---|
| 1 | Daemon↔Worker IPC | stdin/stdout + NDJSON；worker 短命、只产 normalized 结果 |
| 2 | MCP 进程模型 | 三进程：常驻 `wikimindd` + 瘦 `wikimind mcp serve` bridge + `wikimind` CLI |
| 2 | MCP SDK | 官方 Go SDK 首选，自实现 JSON-RPC 为 fallback |
| 2 | bridge⇄daemon | Unix socket / named pipe + JSON-RPC 2.0 |
| 3 | SQLite migration | `goose` + `//go:embed`；index.db 是派生数据，rebuild 兜底 |
| 4 | Go 模块 | `cmd/` + `internal/`（14 包）+ `worker/`（Python）+ `migrations/` |
| 4 | SQLite 驱动 | `modernc.org/sqlite`（纯 Go）首选，`mattn/go-sqlite3` fallback |
| 4 | Git | 直接 exec `git`（worktree/LFS 可靠），不用 go-git |

---

## 7. 与其它文档的关系

- 本文档是 `roadmap-30d.md` W0 的展开 + 4 个技术点的决策
- 物理进程模型澄清了 `architecture.md §1` 逻辑图的归属问题（§2.1）
- SQLite / FTS5 细节承接 `cjk-tokenizer.md`
- IPC 与 `architecture.md §2`（进程模型）一致

---

## 8. 不在范围

- 具体 API 签名 / 函数级设计 —— 留给 W1 各 PR
- 性能调优策略 —— roadmap D29
- 部署 / 打包细节 —— roadmap D22-23 + cross-platform.md
- v0.2 的 embedding / jieba —— 超出 MVP

---

## 一句话总结

> 4 个技术点定死：worker 用 stdin/stdout NDJSON（短命、只 parse）；MCP 是「常驻 daemon + 瘦 bridge
> 进程」三进程模型；migration 用 goose 嵌入二进制；Go 项目 `cmd/`+`internal/` 14 包，单写者由
> `commit` 包物理保证。W0 三个验证项（纯 Go SQLite FTS5 / Go MCP SDK / 跨进程 IPC）先跑绿再进 W1。
