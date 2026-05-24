# W2 D9: MCP read tools 第二批

## Goal

实现 5 个 read tool：`search` / `read_raw_anchor` / `read_claim` /
`graph_neighbors` / `get_history`，让 Claude Code 经 MCP 能：
- 全文检索 wiki（FTS5 BM25 + filter）
- 读 raw 文件特定 anchor + 实时 quote_hash（防 agent 编造 hash）
- 读 claim + sources 校验状态（drift detection）
- 看 page graph 邻居（含 `[[…]]` 实时 parse）
- 看 page 编辑历史（git log + change-log）

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D9
- `spec-v2/docs/mcp-tools.md §5 §6 §8 §9 §10`
- D8 已建 `internal/mcp/` 框架；新 tool 接入既有 wrapHandler 泛型 adapter

## What I already know

- D5 已有 `internal/index/search.go`：`SearchFTS5` + `SearchLike` + `extractSnippet` —— `search` tool 直接复用
- D6 已有 `wiki/log.md` + `.wikimind/change-log.jsonl` —— `get_history` 复用
- D4 `pages` 表 + `ParsePage`；page type='claim' 已能存（D9 复用，不另建 claim 表）
- D8 已建 `internal/mcp/`（server/tools/types）+ `wrapHandler` 泛型 adapter
- **没有的**：
  - `claim_sources` 表（quote_hash + drift）→ D11 propose_claim 时再建；D9 staged
  - `page_links` 表 → D10/D11；D9 用实时 parse `[[…]]` from page body 兜底
  - stage-2 raw parser（heading slug / paragraph / char span）→ D9 必须实现（read_raw_anchor 核心）

## Requirements

### A. `internal/index/anchor.go`（新文件，stage-2 raw parser）

最小可用 stage-2 parser，解析 markdown 文件：
- **3 种 anchor 格式**（按 mcp-tools.md §5）：
  - `#heading-slug`：扫所有 `# / ## / ### / ...` heading，slugify（小写 + 中文保留 + 空格→`-` + 特殊字符去除），返 first match
  - `#para-N`：1-indexed paragraph（按 blank line 分割，跳过 frontmatter 区）
  - `#char[start:end]`：UTF-8 rune index 范围（不是 byte，避免 CJK 半切）
- `ParseAnchor(s string) (kind, value, error)` 解析 anchor 字符串
- `ResolveAnchor(content []byte, anchor string) (text string, span [2]int, error)` 在 raw 内容中定位
  - span 返回 [startRune, endRune]（rune index，跨平台一致）
- `QuoteHash(text string) string`：sha256(normalized text)[:8]
  - normalize：strip leading/trailing whitespace + 多连续 `\n` 折叠为单个
- 错误：`ErrAnchorMalformed / ErrHeadingNotFound / ErrParaOutOfRange / ErrCharSpanInvalid`

### B. tools.go 新增 5 个 handler

#### B1. `handleSearch`（mcp-tools.md §8）

- 复用 `internal/service/search.Search`（D5 router）
- filter:
  - `page_type` 数组 → 过滤 `index.SearchHit.Type`
  - `min_confidence` → 过滤（pages 表当前无 confidence 字段，D9 暂忽略此 filter
    + 附 note "min_confidence filter requires claims confidence field (W2 D11+)"）
  - `status` 数组 → pages 表有 status 字段（D4），直接过滤
  - `updated_since` RFC3339 → pages 表有 updated_at，>= 过滤
- Response: `{results: [{page_id, title, snippet, score, confidence?}], tokenizer_used: "trigram"/"like"/"ripgrep", query_time_ms}`
- `type=fts+vector` → 降级 fts + warning（embedding 是 v0.2+）

#### B2. `handleReadRawAnchor`（§5）

- 复用 D8 `read_raw` 的 path resolution + traversal 防护
- 读 raw 文件 → 调 `index.ResolveAnchor` + `index.QuoteHash`
- Response: `{raw_id, anchor, content, quote_hash, span: [start, end], source_mtime, source_sha256}`
- source_sha256 复用 D3 `sources` 表已存（`SourceRow.SHA256`）；若 raw 不在表里
  现场 sha256 算一次

#### B3. `handleReadClaim`（§6）

- 查 `pages` WHERE id=claim_id AND type='claim'
- 复用 read_page response 结构 + 附加 `sources` 数组
- **sources 数组 D9 阶段 staged**：返回空数组 + note "claim source validation
  requires claim_sources table (W2 D11+ propose_claim)"
- not found → ErrClaimNotFound

#### B4. `handleGraphNeighbors`（§9）

- 输入 page_id（D8 read_page 同 id/path 两态）
- **D9 staged 实施**：
  - direction=out（or both）：实时 parse page body 的 `[[…]]`（复用 D4
    `service.ParsePage` regex），返每个 outbound target 的 page id
  - direction=in：返空 + note "inbound links require page_links table (W2 D11+)"
  - depth >1：D9 不实现（只支持 depth=1），depth>1 → ErrDepthUnsupported
  - link_types filter：parse 出的 link 当前无类型，全归为 "ref"；filter 应用前过滤
- Response: `{neighbors: [{page_id, title, link_type}]}`

#### B5. `handleGetHistory`（§10）

- 输入 page_id（同上两态）
- 查 page 实际文件 path（vault-relative）
- `git log --format='%H|%aI|%s' -- <path>` 拿提交历史（exec git，limit）
- 对每个 commit：从 commit message 抽 `seq=N` → 查 change-log.jsonl 拿 actor/op/bundle_id
  - 如果 commit message 不含 seq=（非 D6 之后的）→ 用 git author + 标 `op: "git-direct"`
- `diff_summary` = `git show --stat <sha> -- <path>` 简化为 `+N -M` 字符串
- Response: `{commits: [{sha, ts, actor, op, bundle_id?, summary, diff_summary}]}`

### C. 注册 tool + types

- `internal/mcp/types.go`：5 套 Request/Response struct（严格按 mcp-tools.md schema）
- `internal/mcp/server.go`：注册 5 个 tool（全部 `ReadOnlyHint: true`）
- 9 个 tool 总数（D8 4 + D9 5）

### D. 测试

- `internal/index/anchor_test.go`：**50+ 边界用例**（roadmap 硬要求）：
  - heading slug：英文 / 中文 / 含数字 / 含特殊字符 / 重名取第一个 / 大小写
  - para-N：1-index / 0 invalid / 超界 / 跳过 frontmatter / blank line 分割
  - char[start:end]：合法 rune 范围 / start > end / 超界 / CJK 半切防御
  - anchor 字符串解析：缺 `#` / 缺 prefix / 多 `#` / 空字符串
  - quote_hash：normalize 一致性（前后空白 / 连续 \n 折叠）
- `internal/mcp/tools_test.go`：5 个新 handler 各 2-3 测试
  - search: page_type filter / status filter / updated_since / 空 query / 短 query 走 LIKE
  - read_raw_anchor: heading hit / para hit / char hit / anchor malformed / heading miss
  - read_claim: by id (type=claim) / not claim type / not found / sources 字段 staged 空 + note
  - graph_neighbors: out parse [[]] / in staged 空 + note / depth>1 拒
  - get_history: 单 commit / 多 commit / 含 seq= 反查 change-log / 无 seq= git-direct

### E. CI 全绿 + 测试总数 ≥ 180

D8 后 129 测试；D9 自然带 50+ (anchor) + 15+ (5 tool × 3) = 65+ 新增 → 194+

## Acceptance Criteria

- [ ] `search` 走 FTS5 + 4 个 filter（page_type / min_confidence staged / status / updated_since）
- [ ] `read_raw_anchor` 3 种 anchor format 全支持 + 实时 quote_hash + span/mtime/sha256
- [ ] `read_claim` 查 type='claim' page，sources 字段返空 + note（staged）
- [ ] `graph_neighbors` direction=out parse [[]] 实时，direction=in staged 空 + note
- [ ] `get_history` 复合 git log + change-log seq= 反查
- [ ] anchor 解析 50+ 边界单测（roadmap 硬要求）
- [ ] 9 个 tool 全 ReadOnlyHint=true + schema 严格按 mcp-tools.md
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-E 全 done
- CI 5 OS 全绿
- 测试 ≥ 180（baseline 129 + 50+ anchor + 15+ handler）
- commit + push

## Out of Scope

- `claim_sources` 表 + drift status 真实计算（W2 D11+ propose_claim 时建）
- `page_links` 表 + inbound graph（W2 D11+）
- `depth>1` graph traversal（W2 D10+ depth 1-3）
- embedding rerank `type=fts+vector`（v0.2+）
- stage-2 parser 高级特性（normalize → markdown AST 完整 walk，D9 用够用的 regex）
- 写 tool（propose_*，D11）

## Decision (ADR-lite)

**Context**: D9 5 个 tool 中 3 个（read_claim sources / graph_neighbors inbound /
search min_confidence）依赖 D11+ 才建的表（claim_sources / page_links 等）。
要么阻塞 D9 等 D10/D11，要么 staged。

**Decision**: **D9 工具接口完整注册 + 缺数据的字段走 staged 占位**（同 D8
read_page include_history pattern）：
- 返结构化 empty + note 说明 staged 原因 + 何时实现
- 不抛 NOT_IMPLEMENTED 让 agent 看到 staged 是有意为之
- 5 tool 全部 ReadOnlyHint=true

stage-2 parser 范围最小：3 种 anchor + quote_hash，足够 read_raw_anchor 用；
markdown AST 完整 walk（解析嵌套列表 / table cell / code block 内 char span
特殊处理）留 D13+ PDF/HTML ingest 时再扩展。

**Consequences**:
- 优点：D9 deliverable 不 block 等表，agent 立即可用 5 tool
- 缺点：3 个高级字段 staged—mcp-inspector.md 文档需补 staged 说明
- 与 architecture 暗示一致：claim sources / page_links 是 propose_* 写时
  populate 的，read 端 W2 早期没数据正常

## Technical Notes

- 复用 D5 `internal/service/search.Search` 路由器
- 复用 D8 `vault.ResolveInVault` 防 path traversal
- exec git 在 vault root：`cmd.Dir = vaultRoot`（D6 既有 pattern）
- heading slug：中文保留（trigram tokenizer 路线一致）；空格 → `-`；
  非字母数字非 CJK 字符去除
- para-N 跳过 frontmatter：用 `---` 边界 detect
- char[start:end] **rune index** 不是 byte（避 CJK 半切）：`utf8.RuneCountInString` 算长度
- quote_hash 长度 8 hex char：`hex.EncodeToString(sha256(normalized))[:8]`
- get_history 反查 change-log.jsonl：每行 JSON unmarshal，按 seq 索引
  （可能 N 行，D9 阶段 in-memory map 足够；W3 lint 后再加 index 表）
