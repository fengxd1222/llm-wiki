# W2 D14: W2 出口 demo — Claude Code 经 MCP 全链路

## Goal

W2 出口验收：Claude Code 经 MCP 跑完整 demo flow，证明 W2 的 15 MCP tool
+ worktree + reviews + propose/accept 全链路工作。同时补 W2 最后 polish 项
（health score 真实计算 / wiki/index.md 自动维护 / 跨平台跑通）。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D14
- 完整使用 D8-D13 全部能力

## What I already know

- D8-D11 15 个 MCP tool 全注册（9 read + 6 write/meta）
- D10 agent_handshake + worktree + reviews/bundles 表
- D11 propose_page/edit/claim + request_review + log_append
- D12 wikimind review accept/reject/diff CLI
- D13 PDF ingest + watcher
- D7 docs/demo/w1-walkthrough.md 模板复用
- W1 出口 demo flow 已经能跑：init → ingest md → query → revert

## Requirements

### A. Demo flow（W2 出口标准）

`docs/demo/w2-walkthrough.md`：

完整 demo 步骤化文档：
1. 装 Claude Desktop 或 Cursor + MCP support
2. 配置 `wikimind mcp serve` 接入（Claude Desktop config 示例）
3. **Phase 1 — handshake + 浏览**：
   - 在 Claude Code 让 agent 调 `agent_handshake` → 拿 worktree + token
   - `wiki_info` 查看 vault 概况
   - `list_index --type=claim` 浏览现有 claims
4. **Phase 2 — ingest 一篇论文**：
   - User 拖 `karpathy-llm-wiki.md` 到 `raw/inbox/`
   - watcher（D13）检测 + 自动 ingest（含 source page 生成 + commit）
5. **Phase 3 — agent 抽 claim**：
   - Claude `read_raw_anchor` 读特定段 → 拿 quote_hash
   - Claude `propose_claim`：建 `cl-2026-05-24-001` claim，附 source + quote_hash
   - daemon 验证 quote_hash → 接受 → 写 wiki/_review/r-NNNN.patch
6. **Phase 4 — request_review + user accept**：
   - Claude `request_review([r-NNN])` → bundle b-NNN
   - User 终端：`wikimind review list` → 看到 pending r-NNN
   - `wikimind review diff r-NNN` 看 patch
   - `wikimind review accept r-NNN --no-confirm` → apply 到 main + commit + change-log
7. **Phase 5 — verify**：
   - Claude `search "compounding"` → 命中新 claim
   - `get_history cl-2026-05-24-001` → 看 git log + change-log
   - 终端 `wikimind log --limit 5` → 看 ingest + accept 两条
   - `git log --oneline` → 看完整 git 历史
8. **跨平台**：
   - macOS / Windows / Linux 各跑一遍
   - CI demo smoke test 跑端到端

每步贴 expected output（```text 块 + agent 对话片段）

### B. wiki/index.md 自动维护

W1 D7 OOS 留的项：每次 ingest 后 wiki/index.md 加一行。D14 实现：

`internal/service/wiki_index.go`（新）：
- `EnsureIndex(ctx, vault) error` 首次写 `wiki/index.md` 含表头
- `AppendIndexEntry(ctx, vault, page PageInfo) error` 追加一行
- `RebuildIndex(ctx, vault, db) error` 从 pages 表全重建（recovery 用）

格式：
```markdown
# WikiMind Index

| ID | Type | Title | Sources | Confidence | Updated |
|---|---|---|---|---|---|
| cl-2026-05-24-001 | claim | Wiki 是 compounding artifact | 2 | 0.92 | 2026-05-24 |
| karpathy-llm-wiki | source | Karpathy LLM Wiki | — | — | 2026-05-24 |
```

集成点：
- D7 `ingest` 之后 → `AppendIndexEntry`
- D12 `review accept` 后 → `AppendIndexEntry`（page 类型 propose）
- 失败不阻塞主流程（warning to stderr）

### C. Health score 真实计算

D8 `wiki_info.health.score` D8 占位返 100。D14 真算：

```go
// 评分公式 (架构暗示):
//   score = 100
//     - 5 * drift_claims_count (max -50)
//     - 1 * lint_warnings_count (max -30)
//     - 2 * orphan_pages_count (max -20)
//   floor 0
```

- drift_claims 来源：claim_sources 表（W3 D11+ 才建）—— D14 仍占位 0
- lint_warnings：来源 lint_run（W3 D17）—— D14 占位 0
- orphan_pages：pages 表 `[[...]]` 无 inbound 数（D9 graph_neighbors 实时算
  能拿，但全 vault 扫太慢）—— D14 计算简化：COUNT pages WHERE
  type IN ('claim', 'entity', 'concept') AND id NOT IN (SELECT target FROM
  page_links table) —— page_links 表 D14 也建一下！

**额外加 page_links 表 (migration 0004)**：
```sql
CREATE TABLE IF NOT EXISTS page_links (
    source_id   TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    link_type   TEXT NOT NULL DEFAULT 'ref',
    PRIMARY KEY (source_id, target_id, link_type)
);
CREATE INDEX idx_page_links_target ON page_links(target_id);
```

`internal/service/page.go::ReindexWiki` 每次 reindex 时：
- parse body `[[...]]` outbound
- INSERT INTO page_links (source_id=current page, target_id=parsed link)
  ON CONFLICT DO NOTHING

D9 `graph_neighbors` direction=in staged 占位 → D14 改成真查 page_links table。

D14 加 page_links 的 incremental impact：
- D9 inbound staged note 转 real implementation
- D8 wiki_info health.score 真值
- 后续 W3 lint orphan 规则有数据源

### D. CI demo smoke test

`cmd/wikimind/d14_demo_test.go`（新）：
端到端 8 阶段一气跑：
1. tmpdir vault → wikimind init
2. mock raw markdown ingest
3. mock propose_claim (跳过 MCP 直接调 service.AcceptReview wrap)
4. wikimind review accept
5. wikimind search "..." 命中
6. wiki/index.md 验有新行
7. wiki_info health.score 验有真值（不是 100 占位）
8. graph_neighbors in 验真返 inbound（用 backlink 测试数据）

跑在 5 OS CI 矩阵全绿。

### E. CLI 完善

`cmd/wikimind/command.go`：
- `wikimind doctor`（新）：检查 git binary / python3 / pypdf / vault 完整性
- `wikimind reindex`（重命名 `page reindex`，保留 alias）：full rebuild 含 page_links

### F. 测试

- `internal/service/wiki_index_test.go`：EnsureIndex / AppendIndexEntry / RebuildIndex
- `internal/index/page_links_test.go`：INSERT / SELECT inbound / 清理 stale links
- `internal/mcp/tools_test.go` 加 health score 真值测试 + graph inbound 真值测试
- `cmd/wikimind/d14_demo_test.go`：end-to-end demo（B 段提到）
- `cmd/wikimind/command_test.go`：doctor 命令存在

目标测试总数：235（D13 后）→ ≥260（+25）

## Acceptance Criteria

- [ ] `docs/demo/w2-walkthrough.md` 完整可复制跑通
- [ ] CI demo smoke test 在 5 OS 全绿
- [ ] wiki/index.md 自动维护 + RebuildIndex 命令可用
- [ ] migration 0004 page_links 表 + INSERT on reindex + INDEX inbound
- [ ] wiki_info.health.score 真实计算（基于 orphan_pages_count 至少）
- [ ] graph_neighbors direction=in 真返 inbound（不再 staged 占位）
- [ ] `wikimind doctor` 命令可用
- [ ] **W2 出口标准 4 条达成**：
  - demo 流畅无 ERROR
  - git 历史干净（每个 commit 有意义 + change-log 1:1）
  - CJK 搜索在 Claude Code 里能查到中文子串
  - 单 vault 多 agent session 并发（D14 简化：两 session 串行测试，并发留 W3）
- [ ] 单测：≥ 25 个新测试
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-F done
- CI 5 OS 全绿
- 测试 ≥ 260
- commit + push
- **W2 全部 14 天完成（D1-D14）**

## Out of Scope

- Multi-agent 真并发剧本（W3 D15 lock manager + conflict）
- propose_delete / propose_merge（W3 D15+）
- Lint 真规则（W3 D17）
- Daemon 长生命周期（W3 daemon 主循环）
- Embedding rerank `search type=fts+vector`（v0.2+）
- Release packaging（W4 D22-30）

## Decision (ADR-lite)

**Context**: W2 出口 demo 需要把 D8-D13 串成端到端故事。同时发现两个
W2 早期 staged 字段（health.score / graph inbound）现在该兑现，需 page_links
表。

**Decision**:
1. **D14 范围包括 page_links 表（migration 0004）**：让 graph_neighbors
   inbound 兑现 + health score 有 orphan 数据源。本来 page_links 是 W3+
   但 D14 demo 闭环必须
2. **demo smoke test 端到端跑在 CI**：W1 D7 `TestW1DemoWalkthroughCISmokeTest`
   pattern 延展，让 W2 出口 forever 不破
3. **`wikimind doctor` 加入 D14**：release 准备但 CLI dev 用 doctor 检查
   依赖，提前到 D14 让 demo flow 真实可跑
4. **MCP demo skip Claude Desktop 自动化**：D14 manual walkthrough 步骤；
   CI 用 mock service 调用代替 MCP stdio handshake（MCP inspector 自动化
   太复杂留 W4）

**Consequences**:
- 优点：W2 出口完整闭环，graph inbound 真值，user 在 Claude Desktop 真能
  完整使用 wikimind
- 缺点：D14 范围比 roadmap 标称略宽（加了 page_links 表）—— 但这是 W2 完整
  必需，无 page_links graph 是半残
- W3 D15 起 lock manager 直接基于 D14 完整数据层

## Technical Notes

- migration 0004 page_links 跟 D10 0003 reviews/bundles 同 pattern（goose
  StatementBegin/StatementEnd 包裹）
- `[[target|alias]]` parse 复用 D4 service/page.go regex
- wiki/index.md 是 git tracked—— 跟 source 文件同 commit
- health.score 计算放 internal/service/health.go（新文件，独立可测）
- doctor 命令检查：
  - `exec.LookPath("git")` / `exec.LookPath("python3")`
  - python3 `import pypdf` 是否 ok
  - .wikimind/index.db schema_version 检查
  - vault 三层目录存在
- d14_demo_test.go 用 tmpdir 隔离，每次跑全新 vault

## 实施建议顺序

1. **migration 0004 page_links 表**（独立可测）
2. **internal/index/page_links.go CRUD**（独立）
3. **改 service/page.go ReindexWiki INSERT page_links**
4. **graph_neighbors direction=in 真查 page_links**
5. **internal/service/health.go score 真算**
6. **wiki_info health.score wired**
7. **internal/service/wiki_index.go**
8. **D7 ingest + D12 review accept 后 AppendIndexEntry**
9. **wikimind doctor CLI**
10. **d14_demo_test.go 端到端**
11. **docs/demo/w2-walkthrough.md**
12. **测试 + ≥ 260**
