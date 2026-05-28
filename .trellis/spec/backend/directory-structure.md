# Directory Structure

> 本项目（LLM Wiki / WikiMind）后端代码的组织约定。
> 写新功能前先读这份，避免凭直觉乱放。

---

## Overview

WikiMind 是 local-first 的多 Agent 协作知识库，采用经典 Go 项目布局：
`cmd/` 是入口、`internal/` 是不可被外部模块导入的实现、`verify/` 是跨包的端到端验收测试。
设计文档与 UI 原型分别放在 `spec-v2/`、`prototypes/`。

- **Module**：`github.com/fengxd1222/llm-wiki`（Go 1.26.3）
- **架构**：CLI + 可选 daemon + MCP server，单 vault 单 writer 物理边界。
- **业务边界**：以 vault（用户知识库目录）为单位，所有写操作都过 `internal/commit.Commit` 闸门。

---

## Directory Layout

```
llm-wiki/
├── cmd/
│   ├── wikimind/              # CLI 主入口（init / status / ingest / review / mcp serve ...）
│   │   ├── main.go            # 仅 cobra root + 顶层错误 -> stderr + os.Exit
│   │   ├── command.go         # 所有子命令的 cobra 定义
│   │   ├── demo.go            # 演示场景命令
│   │   ├── review.go          # review 子命令
│   │   ├── watch.go           # watcher 子命令
│   │   └── *_test.go          # CLI E2E 测试（不 mock，跑真实命令）
│   └── wikimindd/             # daemon 入口（W2+）
│
├── internal/                  # 不可被外部模块导入；按"职责"切包，不按"层"切
│   ├── vault/                 # vault 结构、路径规范化、path-traversal 防御
│   ├── index/                 # SQLite 索引、FTS5、goose migrations
│   │   └── migrations/        # *.sql，goose Up/Down 双向迁移
│   ├── service/               # 业务层（ingest / parse / search / propose / review）
│   ├── commit/                # 唯一写入闸门：git commit + 双 log 同步
│   ├── mcp/                   # MCP server 实现（wikimind mcp serve 进程使用）
│   ├── proposal/              # propose patch 生成、base_hash 校验
│   ├── worktree/              # git worktree 分配、session 隔离
│   ├── lock/                  # 分布式锁、队列状态机
│   ├── lint/                  # 增量 lint 规则
│   ├── watcher/               # fsnotify 文件监控
│   ├── bridge/                # Windows watcher IPC bridge
│   ├── daemon/                # daemon 主循环
│   ├── changelog/             # 变更日志写入/解析
│   ├── git/                   # git 命令封装
│   ├── schema/                # YAML frontmatter schema 校验
│   ├── model/                 # 跨包共享的数据模型
│   └── worker/                # 后台 worker（PDF / 图像 OCR 等）
│
├── verify/                    # 跨包端到端验证测试（不在 internal 内）
│   ├── fts5/                  # FTS5 可用性验证
│   ├── ipc/                   # IPC bridge 验证
│   └── mcp/                   # MCP tools 集成测试
│
├── worker/                    # Python 工作器（PDF / 图像处理）
│   ├── main.py
│   └── pyproject.toml
│
├── spec-v2/                   # 产品/工程设计 spec（权威）
├── prototypes/                # UI 原型（HTML mockup）
├── archive/                   # 历史方案归档
├── docs/                      # 项目文档
└── .trellis/                  # Trellis 工作流（任务、spec、journal）
```

---

## Module Organization

### 切包原则

**按职责切，不按层切**。例如：
- 不要建 `internal/handlers/`、`internal/repository/`、`internal/models/`。
- 改建 `internal/service/`、`internal/index/`、`internal/commit/`，每个包内可自带 model + 实现 + 测试。

### 单写者闸门

任何修改 vault 内容（git commit）的代码路径都必须过 `internal/commit.Commit` 或 `commit.CommitWithActor`：
- CLI 直接写：`cmd/wikimind/command.go` 的 ingest / revert 走 `commit.Commit`。
- MCP 写：经 `internal/mcp/tools.go` -> `internal/proposal` 暂存到 worktree -> 写 review queue，accept 时由 `internal/commit` 落地。
- `commit.Commit` 内部用 `sync.Mutex` 串行化（W1 MVP）；W2+ 由 daemon Single-Writer Commit Loop 替换。

### 文档与代码分离

- 设计 spec 在 `spec-v2/`，代码不要重复 spec 的内容。
- 包级 doc.go 写一句话职责 + 当前 D 阶段提供的能力清单（例：`internal/index/doc.go`）。
- 详细的"为什么"在 spec-v2，代码里用 `// 见 spec-v2/docs/architecture.md §X.Y` 反向引用。

### 测试位置

- 单元测试：`xxx_test.go` 与被测代码同包同目录（黑盒测试用 `package xxx_test`）。
- CLI E2E：`cmd/wikimind/*_test.go`，跑真实命令、真实 vault、真实 git。
- 跨包验收：`verify/<feature>/`，在 internal 之外，可被任意包导入但不会被打包进二进制。

---

## Naming Conventions

### 文件命名

- 全小写 + 下划线分隔：`change_log.go`、`source_page.go`。
- 测试：`<file>_test.go`，特殊场景测试可加日期/Daily 标签：`d14_demo_test.go`。
- 包级文档：每个包必须有 `doc.go`（仅 `package xxx` 注释 + 一段 Package 文档）。
- Migration：`NNNN_<short_name>.sql`，例 `0003_reviews_bundles.sql`，4 位数字单调递增不复用。

### 包名

- 单数、全小写、无下划线：`vault`、`index`、`commit`、`proposal`。
- 避免与标准库重名（`index` 例外，因为业务语义强烈；可在 import alias 中消歧）。
- 包名即目录名，import path 末段。

### 符号命名

- 导出类型/函数：`PascalCase`；包内：`camelCase`。
- Sentinel error：`Err` 前缀，例 `ErrIndexUnavailable`、`ErrNonEmptyDirectory`、`ErrSessionRequired`。
- 错误码（MCP 协议返回字符串）：`SCREAMING_SNAKE_CASE`，例 `CROSS_SESSION_BUNDLE`、`AGENT_NOT_WHITELISTED`。
- 常量：包内 `camelCase`，导出 `PascalCase`；分组写在 `const ( ... )` 块。

### Vault 内路径约定

- `raw/{inbox,imported,attachments,manifests}/` — 原始资料（只读、不可变）。
- `wiki/{claims,entities,concepts,sources,topics,_review,_reports,_worktrees}/` — 协作 wiki 层。
- `wiki/_review/r-NNNN.patch` — 待审 patch，4 位数字补零。
- `wiki/_worktrees/agent-<agent>-<session>/` — 每个 MCP session 的隔离 worktree。
- `.wikimind/{config.toml,index.db,audit,locks,change-log.jsonl}` — 引擎元数据，gitignore 大部分内容。
- 所有 vault-relative 路径在代码里以 **POSIX 风格** 存储（`raw/inbox/foo.md`），落盘时通过 `filepath` 转本地分隔符。

---

## Examples

### 标准包结构（参考 `internal/vault`、`internal/index`）

```
internal/vault/
├── doc.go                 # package 注释
├── vault.go               # 核心类型 + 构造函数（Init / Open）
├── config.go              # 子职责：config.toml 读写
├── path.go                # 子职责：路径规范化与 traversal 防御
├── config_test.go
├── path_test.go
└── vault_test.go
```

### 添加新 internal 包的检查清单

- [ ] 包名是单数小写，目录名一致。
- [ ] 有 `doc.go` 写明职责 + 当前阶段能力。
- [ ] 导出 API 都有 godoc 注释（中英文都可，与同包风格一致）。
- [ ] sentinel error 用 `Err` 前缀、`errors.New` 在包级 `var` 声明。
- [ ] 写操作不绕过 `internal/commit`；读操作不在 internal 之外重复实现。
- [ ] 跨包接口稳定后才放进 `internal/model`；包内类型留在包内。
- [ ] 至少一个 `*_test.go`；CLI/集成场景在 `verify/` 或 `cmd/wikimind/`。
