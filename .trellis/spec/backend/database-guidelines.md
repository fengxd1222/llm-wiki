# Database Guidelines

> WikiMind 的持久化层约定。
> 数据库 = 一个 vault 一份本地 SQLite（`.wikimind/index.db`），不是共享服务。

---

## Overview

- **引擎**：SQLite 3，通过 `modernc.org/sqlite`（纯 Go 驱动，免 CGO）连接。
- **数据库文件**：`<vault>/.wikimind/index.db`，单 vault 单 daemon 单 writer。
- **Migration 工具**：`github.com/pressly/goose/v3`，迁移文件 `internal/index/migrations/*.sql` 通过 `//go:embed migrations/*.sql` 内嵌进二进制。
- **驱动注册**：`_ "modernc.org/sqlite"`，驱动名 `"sqlite"`（不是 `sqlite3`）。
- **全文检索**：FTS5（trigram tokenizer），存 markdown 正文，bm25 排序。
- **不使用 ORM**：直接 `database/sql` + 手写 SQL；查询函数集中在 `internal/index/<table>.go`。

数据库并非主存储——它是 git-backed Markdown vault 的**衍生索引**，删除后可以 reconcile 重建。
Git commit 是单一事实源；SQLite 只为查询/搜索加速。

---

## Migrations

### Migration 文件位置

```
internal/index/migrations/
├── 0001_initial_schema.sql        # meta + sources
├── 0002_pages_schema.sql          # pages + pages_fts + triggers
├── 0003_reviews_bundles.sql       # 多 Agent 评审队列
├── 0004_page_links.sql            # wiki link 图
└── 0005_claim_sources.sql         # claim 溯源
```

### 命名规则

- 文件名格式：`NNNN_<short_snake_name>.sql`，4 位数字补零，单调递增，**不可复用编号**。
- 数字与 goose 序号一一对应；落库后不可改名或改内容（用新 migration 修）。
- 短描述用 snake_case，反映该 migration 引入的核心对象。

### 文件骨架

```sql
-- +goose Up
-- W1 D3: 简述本次迁移目的与对应 spec 章节。
-- 完整 schema 见 spec-v2/docs/architecture.md §4.2

CREATE TABLE IF NOT EXISTS <table_name> (
    <col>   <TYPE> NOT NULL,
    ...
);

CREATE INDEX IF NOT EXISTS idx_<table>_<col> ON <table>(<col>);

-- +goose Down
DROP INDEX IF EXISTS idx_<table>_<col>;
DROP TABLE IF EXISTS <table_name>;
```

### Migration 规则

- **永远写 `+goose Down`**，且对称回滚（不能只 Up 不 Down）。
- **永远用 `IF NOT EXISTS` / `IF EXISTS`**：migration 必须可重入。
- 一个 migration 一个语义单元：不要在 0003 里既加表又改 0001 的列；要改用新 migration `0006_alter_xxx.sql`。
- 不在 migration 里写数据迁移逻辑（除最小的 `INSERT OR IGNORE INTO meta`）；复杂 backfill 用 Go 代码 + 单独的 `reconcile` 命令。
- 顶部注释指明本次迁移引入的对象 + 反向引用 spec-v2 章节。

### 启动流程

`internal/index.Open(vaultRoot)`：

1. 解析 `vaultRoot` 绝对路径，确保 `.wikimind/` 目录存在。
2. **备份**：现有 `index.db` 先做时间戳备份（migration 失败可回滚，见 spec-v2 engineering-decisions §3.4）。
3. `sql.Open("sqlite", dbPath)` + `Ping`。
4. `goose.Up(sqlDB, "migrations")` 通过 embed FS 跑到最新版本。
5. 任一步失败：清理已建立的连接，包成 `ErrIndexUnavailable` 返回。

调用方负责 `defer db.Close()`。

---

## Query Patterns

### 不使用 ORM

直接用 `database/sql` 或事务包装；查询语句写成 const 或在函数内字面量，不动态拼接列。

```go
const insertSourceSQL = `
    INSERT INTO sources (raw_id, sha256, size, mtime, status, ingested_at)
    VALUES (?, ?, ?, ?, ?, ?)
    ON CONFLICT(raw_id) DO UPDATE SET
        sha256 = excluded.sha256,
        size   = excluded.size,
        mtime  = excluded.mtime
`
```

### 参数化绑定

**永远用 `?` 占位符**，绝不字符串拼接 SQL：

```go
// 正确
row := db.QueryRowContext(ctx, `SELECT sha256 FROM sources WHERE raw_id = ?`, rawID)

// 错误：SQL 注入风险
row := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT ... WHERE raw_id = '%s'`, rawID))
```

FTS5 查询例外允许构造 `MATCH` 表达式，但用户输入仍必须经转义函数处理，不直接拼接。

### Context 传递

所有 DB 函数第一参数 `ctx context.Context`：

```go
func InsertSource(ctx context.Context, db *sql.DB, src Source) error { ... }
func FindReviewByIdempotencyKey(ctx context.Context, db *sql.DB, agent, key string) (*Review, error) { ... }
```

### 错误返回与 sentinel

- 未找到行：返回 `(nil, ErrXxxNotFound)`，不向上暴露 `sql.ErrNoRows`（在包内消化）。
- 不可恢复错误：用 `fmt.Errorf("query xxx: %w", err)` 包裹原始错误。
- 包级 sentinel 列在文件顶部，命名 `Err<Subject><Verb>`，例 `ErrReviewNotFound`、`ErrPatchExists`。

### 事务边界

- 由 service 层决定事务边界：`db.BeginTx(ctx, nil)` -> 多次查询 -> `tx.Commit()` 或 `tx.Rollback()`。
- 写操作必须在事务内，且与 git commit 同一闸门（`internal/commit.Commit`）保证 SQLite 与 git 不漂移。
- 不要在 `internal/index` 包内开事务：它只暴露 `BeginTx` 与原子查询函数，组合逻辑交给 service。

---

## Naming Conventions

### 表名

- 复数、snake_case：`sources`、`pages`、`reviews`、`bundles`、`change_log`、`page_links`、`claim_sources`。
- FTS5 影子表：`<table>_fts`，例 `pages_fts`。

### 列名

- snake_case：`raw_id`、`content_hash`、`ingested_at`、`base_hash`、`session_id`。
- 主键多数为 `id TEXT PRIMARY KEY`（业务 ID）或 `raw_id TEXT PRIMARY KEY`（vault-relative 路径）。
- 时间字段：`<verb>_at INTEGER`（unix epoch，整数秒），不用 `DATETIME`。
- 状态字段：`status TEXT NOT NULL`，值是固定枚举字符串（`pending` / `parsed` / `done` / `error`），状态机变更必须同步更新 spec-v2/architecture §5.1。

### 索引名

- 格式：`idx_<table>_<col>` 或 `idx_<table>_<col1>_<col2>`。
- FTS triggers：`<table>_ai`、`<table>_ad`、`<table>_au`（after-insert/delete/update）。

### Vault-relative 路径作主键

`raw_id` / `page_id` 等存的是 vault-relative POSIX 路径字符串（`raw/inbox/foo.md`），不是 UUID。
原因：路径是 git 的天然身份证，无需额外 ID 表；renamed/moved 时通过 `change_log` 追踪历史。

---

## SQLite-Specific Rules

### `database/sql` 配置

- 驱动名是 `"sqlite"`（modernc.org），不是 `"sqlite3"`。
- 单 writer 序列化由 `internal/commit` 的 `sync.Mutex`（W1）/ daemon commit loop（W2+）保证；不要在外部依赖 SQLite 的 BUSY retry。
- 不开启默认之外的特殊 pragma（如需调优，先查 spec-v2 engineering-decisions）。
- 保持 `database/sql` 默认连接池配置；并发读 SQLite 支持得很好，串行写由应用层保证。

### FTS5 触发器

`pages_fts` 是 contentless FTS5，content 通过触发器从 `pages` 同步：

```sql
CREATE TRIGGER pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(rowid, body) VALUES (new.rowid, new.body);
END;
```

修改 `pages` schema 时必须同步检查三个触发器（`_ai` / `_ad` / `_au`）。

### 短查询兜底

FTS5 对 < 3 字符查询效果差；`internal/service/search.go` 实现了三路兜底链：
**FTS5 trigram BM25 → ripgrep（外部命令）→ SQL `LIKE`**。
跨越兜底链时返回结果中带 `degraded` 标记 + `warning` 提示，调用方不可静默降级。

---

## Common Mistakes / Forbidden

### Forbidden

- ❌ 字符串拼接 SQL（除 FTS5 `MATCH` 经转义函数构造外）。
- ❌ 在 migration 里写大段数据迁移逻辑；用单独的 reconcile 命令。
- ❌ 删除/修改已落库的 migration 文件；只能用新 migration 修。
- ❌ 在 `internal/index` 之外直接 `sql.Open`；统一从 `index.Open(vaultRoot)` 拿 `*DB`。
- ❌ 绕过 `internal/commit.Commit` 直接 INSERT/UPDATE `change_log` 或写 git；这破坏 vault 单一事实源。
- ❌ 引入 ORM（gorm / ent 等）；现有手写 SQL 风格是项目偏好。

### Required

- ✅ 每个 migration 都有对称 Up/Down + `IF [NOT] EXISTS`。
- ✅ 所有 DB 函数第一参数 `ctx context.Context`。
- ✅ 参数化查询；FTS5 输入经转义。
- ✅ 写操作通过 `internal/commit.Commit`（或 `CommitWithActor`）保持 git 与 SQLite 一致。
- ✅ 新表新列必须同步更新 `spec-v2/docs/architecture.md §4.2`。
- ✅ Migration 数字单调递增；不复用、不跳号、不修改历史 migration。

---

## Examples

参考实现：

- `internal/index/index.go:44` — `Open` 函数完整启动流程（路径解析 → 备份 → goose migration）。
- `internal/index/migrations/0001_initial_schema.sql` — migration 模板。
- `internal/index/sources.go` — 单表 CRUD 函数风格（`InsertSource` / `FindSourceByID` / `ListSources`）。
- `internal/index/search.go` — FTS5 + LIKE 兜底实现。
- `internal/commit/commit.go:31` — 写入闸门，串行化 + git commit 与 jsonl/log.md 同事务。
