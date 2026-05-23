# W1 D3: raw ingest 与 SQLite schema 接入

## Goal

打通"文件 → 入仓 → 索引"的最小链路：`wikimind ingest <path>` 把文件落 `raw/inbox/`、
算 `sha256/mtime/size` 三件套、写 `sources` 表；同时首次引入 SQLite + goose migration
链路。为 D4+（page 抽取 / FTS5）准备索引基础设施。

需求来源：`spec-v2/docs/roadmap-30d.md` W1 D3 + `spec-v2/docs/architecture.md` §4.2
（sources 表 schema）+ `spec-v2/docs/engineering-decisions.md` §3（goose + go:embed）
+ §4.3（modernc.org/sqlite，W0 已验证）。

## What I already know

- D1/D2 已落地（commit d8a8958）：CLI 框架 + init/status + config (BurntSushi/toml) +
  path normalize/traversal + 100+ 路径测试
- W0 验证项 1 已确认：`modernc.org/sqlite` v1.50.1 + trigram FTS5 + CJK 子串搜索可用
- `migrations/0001_initial_schema.sql` W0 只占位建了 `meta` 表 —— D3 升级加 `sources`
- `internal/index/doc.go` 空包 W0 建好 —— SQLite 接入归这里
- `architecture.md §4.2` 给了完整 schema —— D3 只落 `sources`（+ 保留 `meta`），其它
  表（pages/claim_sources/reviews 等）后续 D 加新 migration
- `engineering-decisions §3.4` 安全网：index.db 是派生数据，migration 失败可
  rebuild；启动前备份 `index.db.bak`

## Requirements

- **依赖引入**：`github.com/pressly/goose/v3`
- **`internal/index` 包**（首次实质实现）：
  - `Open(vaultRoot) (*DB, error)` — 打开 `.wikimind/index.db`、跑 goose up 到最新
  - `Close()`、`BeginTx` 基础
  - migrations 用 `//go:embed migrations/*.sql`（engineering-decisions §3.2）
- **`migrations/0001`** 升级：实现 `sources` 表（按 architecture §4.2 完整 schema）+ 保留 meta
- **`internal/service` 包**（首次实质实现）：
  - `IngestFile(db, vaultRoot, srcPath) (*Source, error)` —
    - **复制** srcPath → `raw/inbox/<basename>`（原文件保留不动）
    - 流式计算 sha256（crypto/sha256 + io.Copy，O(1) 内存）+ mtime + size
    - UPSERT `sources`（status=pending）
    - 同 sha256 已存在 → 去重不重复写（返回已有 source）
- **`cmd/wikimind/command.go`**：`ingest` stub 升级为真实命令（调 service.IngestFile）
- **测试**：
  - goose up/down 幂等
  - ingest 单文件成功写入 sources 表
  - 同 sha256 文件重复 ingest 去重
  - 大文件（≥ 10MB 模拟）流式 hash 不溢内存
  - missing file / unreadable / 非 vault 目录 → 清晰错误

## Acceptance Criteria

- [ ] `wikimind ingest <path>` 成功把外部文件入 raw/inbox + 写 sources 行
- [ ] migrations 通过 `//go:embed` 嵌入；daemon-less CLI 路径 `index.Open` 自动跑
      goose up
- [ ] 同 sha256 文件重复 ingest 去重（不报错、不重复 INSERT）
- [ ] missing / unreadable / 非 vault 目录 → 清晰错误（sentinel error pattern）
- [ ] `internal/index.Open/Close` API 可用，含 BeginTx
- [ ] sources 表 schema 与 architecture §4.2 一致（raw_id PK / sha256 / size /
      mtime / status / ingested_at / parser / metadata JSON）
- [ ] 单测覆盖 ingest 正常路径 + 去重 + 大文件 + 错误路径
- [ ] go build / vet / test 全绿；CI 5 OS 矩阵通过

## Definition of Done

- 测试覆盖正常路径 + 边界 + 错误
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/` 的 error-handling / database / quality 规范
- commit 并 push（D3 一个 commit）

## Out of Scope

- Markdown 解析 + pages 表（D4）
- FTS5 + query（D5）
- Watcher 自动 ingest（D7+）
- claim 抽取 / agent / MCP（W2+）
- PDF / OCR / 音频 worker（D13）
- 全 schema（pages / claim_sources / reviews / bundles / locks / change_log / ...）
  —— D4+ 按需新 migration（0002, 0003...）增量加，roadmap 每天一个 D 自然分批

## Decision (ADR-lite)

### Q1 — ingest 输入语义

- **Context**: `wikimind ingest <path>` 处理外部文件的方式直接决定 raw 不可变保证的强度。
- **Decision**: **复制到 `raw/inbox/<basename>`**（不动原文件）。
- **Consequences**: 多占一份磁盘空间，但 raw 副本可信；外部修改原文件不污染 vault；
  watcher 只在 `raw/` 自身变化时报 DRIFT，符合 architecture §1 "raw 只读、不可变" 哲学。

### Q2 — D3 migration 范围

- **Context**: architecture §4.2 定义了 10 个表，D3 只需 `sources`。
- **Decision**: **0001 仅升级到 `sources` + 保留 `meta`**；其它表 D4+ 用新 migration
  增量加（roadmap 每天一个 D 自然分批）。
- **Consequences**: 每天最小、可独立 verify；migration 链条与 roadmap 一一对应；
  不为"未来"预留空表。

## Technical Notes

- 不新建 `internal/ingest` 包：ingest 是业务，按 engineering-decisions §4.1 放
  `internal/service/ingest.go`
- `internal/index` 是 daemon-less 时直接调；后续 D 引入 daemon 时它仍是单点 SQLite
  入口（commit loop 也调它）
- sha256：`io.Copy(h, file)` 流式，避免 PDF / 音频大文件爆内存
- 去重策略：UPSERT `ON CONFLICT(raw_id) DO UPDATE`（保留 sha256 不变，刷新 mtime/size）
  或 SELECT 命中跳过（D3 选其中一种，倾向 SELECT 命中跳过——更明确）
- 错误类型：沿用 D1/D2 sentinel：`ErrSourceMissing` / `ErrSourceUnreadable` /
  `ErrIngestDuplicate`（如需明确返回）
- index.db 路径：`{vault}/.wikimind/index.db`
