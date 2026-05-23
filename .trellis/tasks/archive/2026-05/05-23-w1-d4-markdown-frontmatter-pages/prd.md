# W1 D4: markdown frontmatter 解析与 pages 表

## Goal

解析 `wiki/*.md`，把每个 page 的 frontmatter / heading / outbound `[[id]]` 写入
`pages` 表 + `pages_fts` 物化，提供 `wikimind page show/list` 查询。为 D5 FTS5
query 命令提供数据基础。

需求来源：`spec-v2/docs/roadmap-30d.md` W1 D4 + `architecture.md §4.2`（pages
schema）+ `cjk-tokenizer.md §3`（pages_fts 必须 trigram）+ `engineering-decisions.md
§4.3`（YAML 库选项）。

## What I already know

- D3 已落地（commit f7110ac）：`internal/index.Open/Close/BeginTx` + goose embed +
  `migrations/0001` (`sources` + `meta`)，CI 5 OS 全绿
- D4 加 `migrations/0002`：pages 表 + pages_fts（trigram）+ 同步 triggers
- `engineering-decisions §4.3` 列 YAML 库：`gopkg.in/yaml.v3`（boring）或
  `goccy/go-yaml`；倾向 yaml.v3
- markdown parser 未在 spec 中列；Go 主流是 `github.com/yuin/goldmark`（commonmark）
- D4 阶段无 watcher（D7+）、无 propose/review（W2+），pages 填充需显式触发——见 Q1

## Requirements

- **依赖**：`gopkg.in/yaml.v3` + `github.com/yuin/goldmark`
- **`migrations/0002`**：按 architecture §4.2 完整 pages schema
  （id PK / type / path / title / confidence / status / schema_ver / created_by /
  updated_by / created_at / updated_at / frontmatter JSON）+ `pages_fts USING
  fts5(id UNINDEXED, title, body, tokenize='trigram')` + INSERT/UPDATE/DELETE
  triggers 保持同步
- **`internal/service/page.go`**（新文件）：
  - `ParsePage(path string) (*ParsedPage, error)` —— strip frontmatter →
    yaml.Unmarshal；body → goldmark → 抽 heading + outbound `[[id]]`
  - `ReindexWiki(ctx, db, vaultRoot) error` —— 遍历 `wiki/**/*.md` →
    `ParsePage` → UPSERT pages + pages_fts（trigger 自动同步）
  - sentinel: `ErrInvalidFrontmatter` / `ErrInvalidPage`
- **`cmd/wikimind/command.go`** —— `page` 子命令组：
  - `wikimind page reindex` —— 显式全扫 `wiki/` 写 pages + pages_fts
  - `wikimind page list` —— 从 pages 表读，按 type 分组输出；pages 表空时输出
    友好提示 "请先跑 wikimind page reindex"
  - `wikimind page show <id>` —— 输出 frontmatter + body 摘要
- **测试**：
  - frontmatter（标准 + 缺字段 + 类型错 + 无 frontmatter 文件）
  - heading 解析（多级 H1-H6）
  - outbound `[[id]]` 抓取（多个、重复、嵌套上下文）
  - `ReindexWiki` 写入 + 幂等（二次 reindex 不重复）
  - `page list` / `show` 命令

## Acceptance Criteria

- [ ] `migrations/0002` 加 pages + pages_fts + triggers
- [ ] `ParsePage` 解析 frontmatter / heading / outbound `[[id]]`
- [ ] `ReindexWiki` 全扫 + UPSERT + 幂等
- [ ] `wikimind page reindex` 全扫 `wiki/` 写 pages + pages_fts，幂等
- [ ] `wikimind page list` 输出已索引页（空时友好提示先 reindex）
- [ ] `wikimind page show <id>` 输出 page 详情
- [ ] 单测覆盖：frontmatter 边界、heading 多级、[[id]] 抓取、reindex 幂等、命令路径
- [ ] `go build` + `go vet` + `go test ./...` 全绿；CI 5 OS 矩阵通过

## Definition of Done

- 测试覆盖正常 + 边界 + 错误路径
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/` 规范（database / error-handling / quality）
- commit + push

## Out of Scope

- FTS5 query 命令（D5）
- watcher / 增量 reindex（D7+）
- claim / entity 等 page type 的语义校验（W2+ propose 阶段强校验）
- HTML / PDF / 音频 parser（D13）
- 完整 schema 其它表（claim_sources / page_links / reviews 等）—— D5+ 增量

## Decision (ADR-lite)

**Context**: D4 阶段无 watcher（D7+）/ propose（W2+），pages 表需显式触发机制把
`wiki/*.md` 索引进去。
**Decision**: 提供 **`wikimind page reindex`** 显式命令——user 主动触发全扫；
list / show 从 pages 表读；pages 空时 list 输出友好提示。
**Consequences**: 最明确、可调试、最小 MVP；D7+ watcher 接入后此命令仍作为
"打不起 watcher 的兜底" 保留；不引入隐式行为。

## Technical Notes

- 不新建 `internal/page`：解析归 `internal/service/page.go`（业务，与 ingest 同包）
- frontmatter 协议：文件开头 `---` 包围 yaml block
- outbound `[[id]]` 抓取用正则（page-schemas.md 的 id 模式：`[a-z]{2}-\d{4}-\d{2}-\d{2}-\d{3}`
  + slug 形式），D4 用宽松正则 `\[\[([^\]]+)\]\]` 兼容 alias / pipe（`[[id|alias]]`），
  splitting on `|` 取前半
- pages_fts 同步：trigger 自动 UPSERT 而非 service 层双写（更不易漏）
- 错误类型：沿用 sentinel pattern
