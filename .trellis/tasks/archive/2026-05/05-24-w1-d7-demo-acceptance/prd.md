# W1 D7: demo walkthrough + 跨平台验收 + W1 出口

## Goal

跑通 W1 roadmap 出口 demo："手动放 markdown 到 `raw/inbox/` → ingest → 生成
source page → log.md 增行 → git commit → query 命中"，在 macOS + Windows
两平台验证；补 D1-D6 实现中遗留的 source page 生成 gap；写步骤化
walkthrough 文档；总结 W1 闭环。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W1 D7（demo + 跨平台 + 出口标准）

## What I already know

- D1: `wikimind init / status` ✅
- D2: config + path normalize ✅
- D3: `wikimind ingest <path>` 把 raw 文件拷到 `raw/inbox/` + 自动 git commit
  （D6 之后）✅，**但没生成对应的 wiki/sources/<id>.md page**
- D4: page reindex 扫 `wiki/**/*.md` → SQLite pages 表 ✅
- D5: `wikimind query "..."` FTS5 trigram + ripgrep 兜底 ✅
- D6: change-log + git auto-commit + revert ✅
- 当前 95 测试 PASS / CI 5 OS 全绿
- **断点**：raw/inbox/ 里的文件不会被 reindex（reindex 只扫 wiki/），所以
  ingest 后 query 找不到。demo flow "ingest → query 命中"在当前实现下断了

## Requirements

### A. 补 source page 自动生成（ingest 流程闭环）

**`internal/service/ingest.go`** 在拷 raw 完成 + git commit 之前，新增：
- 调 `internal/service/page.go::ParsePage` 解析 raw 文件 frontmatter
- 在 `wiki/sources/<raw-id>.md` 生成 source page，frontmatter：
  ```yaml
  ---
  id: <raw-id>
  type: source
  title: <从 raw frontmatter title 抽，缺则取 first heading，缺则取文件名>
  source_path: raw/inbox/<raw-id>.md   # relative to vault root
  ingested_at: <RFC3339 UTC>
  ---

  # <title>

  Source ingested from `raw/inbox/<raw-id>.md`. See raw file for full content.
  ```
- commit 时把 raw 文件 + source page 一起 git add → 同一个 commit
- commit message format（D6 已定）：`ingest: <raw-id> (seq=<N>)`
- 后续 `wikimind page reindex` 会扫到 source page → query 命中

### B. ingest 自动调用 page reindex（user 体验）

ingest 完成（含 commit）后，自动调 `service.ReindexWiki` 让新 source page
立即可 query，无需 user 手动 reindex。

flag: `--no-reindex` 跳过自动 reindex（CI/script 控制用）。

### C. 跨平台验收

- macOS: 本机跑一次完整 demo（ingest 一个含中文 frontmatter + 中文正文的
  markdown → query 中文 → 命中）
- Windows: CI 已 5 OS 矩阵覆盖；要求至少在 GitHub Actions windows-2022 上 demo
  也跑通（写一个 demo smoke test 跑在 CI 里）

### D. demo walkthrough 文档

`docs/demo/w1-walkthrough.md`：
- 前置：装 git + Go 1.26+
- 步骤 1-N：init vault → 写 raw markdown → ingest → query → revert
- 每步贴出预期 CLI 输出（用 ` ```text ` 块）
- 收尾：W1 已覆盖什么、还差什么（D8+ teaser）

### E. 测试 ≥ 100 总数

当前 95 → 目标 100+。**A 步骤自然带 5-8 个新测试**：
- ingest 后 wiki/sources/<id>.md 存在 + frontmatter 字段正确
- ingest 后 query 能命中新 source page
- `--no-reindex` flag 行为
- ingest 重复同 raw → 不创建重复 source page（基于 raw-id 幂等）
- frontmatter 缺 title 时降级 first heading / filename
- ingest 中文 raw → source page title 正确

### F. CI demo smoke test

新增 `cmd/wikimind/demo_test.go`（或并入 command_test.go）：
- 端到端：`tmpdir vault` → `wikimind init` → 手写 raw file → `wikimind ingest` →
  `wikimind query <substr>` → 断言命中
- 跑在所有 5 OS CI 上确保 demo 不退化

## Acceptance Criteria

- [ ] `wikimind ingest <md>` 后 `wiki/sources/<id>.md` 自动生成（含 frontmatter）
- [ ] ingest 后立即 `wikimind query "<substr>"` 命中（自动 reindex）
- [ ] `--no-reindex` flag 跳过自动 reindex
- [ ] ingest 同一 raw 两次不创建重复 source page（基于 raw-id 幂等）
- [ ] 中文 raw（CJK frontmatter + CJK 正文）demo flow 跑通
- [ ] `docs/demo/w1-walkthrough.md` 完整可复制粘贴跑通
- [ ] demo smoke test 跑在 CI 5 OS 全绿
- [ ] 测试总数 ≥ 100（当前 95 + D7 新增 5+）
- [ ] `go build / vet / test ./...` 全绿
- [ ] W1 出口完整闭环：init → ingest → query → revert 全部 work

## Definition of Done

- A-F 全 done
- CI 5 OS 全绿
- W1 出口标准 4 条达成（demo 两平台 / ≥100 测试 / CI 全绿 / CJK 用例）
- commit + push
- 在 prd Decision 记录 W1 总结（D1-D7 路径地图）

## Out of Scope

- `wiki/index.md` 自动维护（W2 page graph + backlinks 一起做）
- 反向链接 `[[…]]` 解析持久化到 `page_links` 表（W2/D8+）
- file watcher 增量 reindex（W2）
- Lock manager / 多 agent concurrent（W2）
- MCP server（W2）
- review pipeline / propose-accept（W3）

## Decision (ADR-lite)

**Context**: roadmap W1 D7 期望 demo "ingest → 生成 source page →
log.md → git commit → query 命中"，但 D3 ingest 只拷 raw 没生成 source
page，所以 query 找不到。这是 D3-D6 累积的 gap。

**Decision**: D7 范围里**补 source page 生成** + **ingest 自动 reindex**，让
W1 出口 demo 真正 work。source page 是 raw 文件的 wiki 镜像，frontmatter
带 `id / type=source / title / source_path / ingested_at`；内容只是占位
"see raw file for full content"——不复制 raw 正文（FTS5 trigram 索引 raw
内容是 D8+ raw 内容索引层的事，超 W1 范围）。

**Consequences**:
- 优点：W1 出口闭环完成；query 命中 source page；user 通过 source page
  link 跳到 raw 看全文（VS Code / Obsidian 都支持 Markdown link）
- 缺点：query 当前只匹配 source page 的 frontmatter title（FTS5 索引的是
  title + body，body 是占位文字）。**raw 内容本身**在 W1 阶段不可 query
  —— 但这本来就是 architecture §4.2 的设计（raw/ 是审计层，wiki/ 是 user-facing
  层；用 raw_quote 反查表来桥接是 W3 的事）
- 替代方案被拒：（a）source page body 复制 raw 全文——违反"source of truth"
  原则；（b）reindex 直接扫 raw/——绕过 wiki/source page 抽象，污染 page 模型

## Technical Notes

- raw-id 算法：当前 D3 用文件 basename 去扩展名（+ 冲突时加 -<sha8>）；
  source page 用同 id 命名 `wiki/sources/<id>.md`，幂等保证靠"文件已存在
  跳过创建"
- frontmatter title 抽取优先级：raw frontmatter `title` > raw 第一个
  `# heading` > 文件 basename
- source page 创建时的写文件用 `os.WriteFile`（whole file 原子）；不存在才
  写（防覆盖已 user-edit 的 source page，幂等关键）
- ingest 自动 reindex 失败 → 不阻塞 ingest 主流程（log warning，commit 已成功）
- 跨平台路径：source_path 字段统一 POSIX `/`（D2 path normalize 已定）
- demo CJK 测试数据：mock 一个 `karpathy-demo.md` 中文版 / `wiki-cookbook.md`
  中英混
