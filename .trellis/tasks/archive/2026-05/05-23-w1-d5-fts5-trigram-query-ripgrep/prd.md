# W1 D5: FTS5 trigram query 与 ripgrep 兜底

## Goal

实现 `wikimind query "..."` 命令——pages_fts trigram BM25 全文搜索；短查询
（< 3 chars）/ regex / `--no-index` fallback ripgrep；ripgrep 不存在再降到 SQL
LIKE。打通"用户能从命令行查 wiki"的最后一公里。

需求来源：`spec-v2/docs/roadmap-30d.md` W1 D5 + `cjk-tokenizer.md §3.3` +
`architecture.md §3.2` query 流程。

## What I already know

- D4 已建 `pages_fts USING fts5(id UNINDEXED, title, body, tokenize='trigram')`
  + INSERT/UPDATE/DELETE triggers 同步 + `body` 列（D4 sub-agent 加进 pages 表
  让 trigger 可读）
- `cjk-tokenizer.md §3.3` 路由：≥ 3 chars trigram MATCH / < 3 chars LIKE /
  正则 ripgrep / `--no-index` ripgrep
- `architecture.md §3.2` Query 流程：FTS5 BM25 → (optional) embedding rerank →
  graph traversal → context bundle；D5 只做 FTS5（rerank / graph 留 v0.2+）
- ripgrep 是外部二进制，user 可能没装（`cross-platform.md` 没强制要求）
- SQLite FTS5 内置 `snippet(fts_table, col, before, after, ellipsis, max_tokens)`
  函数可生成高亮 snippet

## Requirements

- **依赖**：无新依赖（FTS5 已在 modernc.org/sqlite；ripgrep 用 `exec.LookPath`）
- **`internal/index/search.go`**（新文件）：
  - `SearchHit` struct：`PageID / Type / Title / Snippet / Score`
  - `Search(ctx, db, query string, opts SearchOptions) ([]SearchHit, error)` 走 FTS5
    `MATCH`，用 `snippet()` 高亮，`ORDER BY rank` (BM25)
  - `SearchLike(ctx, db, needle string, limit int) ([]SearchHit, error)` 短查询
    fallback（LIKE）
- **`internal/service/search.go`**（新文件）：
  - `SearchOptions`: `NoIndex bool / Regex bool / Limit int`
  - `Search(ctx, db, vaultRoot, query, opts) ([]SearchHit, error)` 路由器：
    - `opts.NoIndex || opts.Regex` → ripgrep（不存在 → `index.SearchLike` 兜底）
    - `RuneCount(query) < 3` → `index.SearchLike`
    - 否则 → `index.Search`（FTS5）
  - ripgrep 不存在不报错，silently 降级到 LIKE + 在 result 加 `Source: "like-fallback"`
    元信息（debug 用）
- **`cmd/wikimind/command.go`**：
  - `wikimind query "<text>"`：flags `--no-index` / `--regex` / `--limit N`（default 20）
    / `--json`（NDJSON 输出）/ `--verbose`（显式 score）
  - 默认输出：每命中 1-2 行 `<id> [type] <title> — <snippet>`，BM25 score 排序
  - vault 空 / 无 pages 索引 → 提示 "no pages indexed yet — run 'wikimind page reindex' first"
- **测试**：
  - FTS5 trigram MATCH（中英文，复用 D4 测试数据 pattern）
  - 短查询自动 LIKE fallback（断言路径走 LIKE）
  - regex flag 走 ripgrep（mock or skip if no rg）
  - ripgrep 不存在 → LIKE 降级（不报错）
  - 命令路径完整（reindex + query）

## Acceptance Criteria

- [ ] `wikimind query "..."` 走 FTS5 trigram，按 BM25 排序，含 snippet 高亮
- [ ] 短查询（< 3 chars）自动 LIKE fallback
- [ ] `--no-index` 或 `--regex` 走 ripgrep；不存在则 LIKE 兜底
- [ ] 中文子串匹配命中（依赖 D4 trigram + CJK 已验证）
- [ ] 输出格式按 Q1
- [ ] 单测：FTS5 / LIKE / ripgrep 路由 / ripgrep absence / 命令路径
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 矩阵通过

## Definition of Done

- 测试覆盖正常 + 短查询 + 正则 + ripgrep 缺失
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/`
- commit + push

## Out of Scope

- Embedding rerank（v0.2）
- Graph traversal / backlinks（D6+ 加 page_links 表）
- Query sedimentation（W2+，spec-v2 已设计但 MVP 后期）
- claim 反查 raw quote（W2+ claim 表后）
- Watcher / 增量 reindex（D7+）

## Decision (ADR-lite)

**Context**: query 是 W1 D5 最 user-facing 的命令；输出既要 CLI 友好（人读），
又要 machine 可解析（agent / script 读）。
**Decision**: **CLI 简洁列表（默认人类可读）+ `--json` flag**。默认每命中
1-2 行 `<id> [type] <title> — <snippet>`，BM25 score 排序；`--json` 切到 NDJSON
（每行一个 hit）；`--verbose` 显式 score。
**Consequences**: 两个模式互不打扰；NDJSON streamable + agent 友好；未来加新字段
不破坏 CLI 输出。

## Technical Notes

- 不新建 `internal/search` 包：service 路由 + index 实现，与 D4 page 同骨架
- snippet 函数签名：`snippet(pages_fts, 2, '«', '»', '...', 16)`（col 2 = body；
  开闭标记可调）
- 短查询定义：`utf8.RuneCountInString(query) < 3`
- ripgrep 调用：`exec.Command("rg", "--type=md", "--vimgrep", needle, vaultRoot+"/wiki")`
  - regex 模式：直接传 needle；非 regex：用 `--fixed-strings`
- ripgrep 不存在：`exec.LookPath("rg")` 返回 ErrNotFound → 降到 LIKE
- 错误类型：沿用 sentinel pattern；`ErrIndexEmpty`（pages 表空）友好提示
