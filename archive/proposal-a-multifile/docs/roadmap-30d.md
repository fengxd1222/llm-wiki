# 30 天开发计划（MVP）

> 目标：30 个日历日 / 4 个迭代，做出一个可在自己机器上每天用的 MVP。
> 哲学：先做对、再做大；先 Mac、再 Win；先 boring、再 fancy。
> 假设：1 个全职工程师；如果是兼职 / 业余，按 60–80 小时/周折算。

---

## 总览

| 周 | 主题 | 关键里程碑 |
|---|---|---|
| W1 | Skeleton + CLI + 文件 IO | `llmwiki init` 可用，read/write 流转通了 |
| W2 | MCP server + 文件 bridge + 索引 | Claude Code 能完整 ingest + query |
| W3 | Review queue + Lock + Lint + Git | 多 agent demo（Claude + Codex 同时跑）跑通 |
| W4 | 跨平台 + 打磨 + 文档 + 自用 | Mac/Windows CI 全绿，本人开始每天用 |

每周末是一个 demo / 验收点。**周末不通过则下周不开新任务**。

---

## W0（启动日，0.5 天）

| 任务 | 验收 |
|---|---|
| 仓库结构定 | repo 创建，`docs/`、`templates/`、`examples/`、`src/`、`tests/` 就位 |
| 语言决议 | Go（daemon + CLI）+ Python（ingest worker）+ 官方 MCP SDK |
| 基线工具链 | golangci-lint、gofmt、pre-commit、CI（macOS + Windows runner）|
| 任务管理 | GitHub Projects / Linear / Issue tracking |

---

## W1：Skeleton + CLI + 文件 IO（D1–D7）

**目标**：vault 结构走通，CLI 能 init / status / ingest / log，文件读写跨平台规范化通过测试。

### D1（周一）

- [ ] `llmwiki init <vault>`：创建三层目录、写默认 `schema/AGENTS.md` 等、生成 `.llmwiki/config.toml`
- [ ] `llmwiki status`：输出 vault 元信息（路径、文件计数、git 状态）
- [ ] CLI 框架（`cobra` / `urfave/cli`）、子命令 stub

### D2

- [ ] 配置加载 + 校验（`config.toml` schema、必填字段、cross-validate vault root 可写）
- [ ] 跨平台 path normalize（POSIX 内部 + 系统调用前转换 + path traversal 拒绝）
- [ ] 单元测试：100 个路径用例（含中文、长路径、符号链接）

### D3

- [ ] `raw/inbox/` 投递：单文件 ingest 命令 `llmwiki ingest <path>`
- [ ] sha256 + mtime + size 三件套；写入 `sources` 表
- [ ] SQLite schema + migrate 工具（`golang-migrate` 或自写）

### D4

- [ ] Markdown 解析：frontmatter（`gopkg.in/yaml.v3`）+ heading + outbound `[[id]]` 链接
- [ ] `pages` 表写入；`pages_fts` 物化
- [ ] `llmwiki page show <id>` / `llmwiki page list`

### D5

- [ ] FTS5 接入；`llmwiki query "..."` 走 BM25
- [ ] ripgrep 兜底（当 SQLite 不存在时）
- [ ] `llmwiki log --tail N`：从 `log.md` 解析

### D6

- [ ] `log.md` append-only 写工具；`change-log.jsonl` 同步写
- [ ] git auto-commit（每次 accept 一次 commit）
- [ ] `llmwiki revert <seq>`：`git revert` 包装

### D7（周末验收）

- 跑通 demo：手动放一个 markdown 到 `raw/inbox/`，CLI 触发 ingest，生成 source page，update index.md，log.md 增加一行，git 自动 commit。
- 跨平台跑：macOS + Windows 笔记本各一遍。
- 写本周 retrospective。

**W1 出口标准**：上面 demo 在两个平台都能跑；至少 100 个单测；CI 全绿。

---

## W2：MCP server + 文件 bridge + 索引（D8–D14）

**目标**：Claude Code 能通过 MCP 完整跑 ingest + query 流程。

### D8

- [ ] `llmwiki mcp serve`（stdio）
- [ ] 实现 `wiki_info`、`read_page`、`read_raw`、`list_index`
- [ ] 单测 + `mcp inspector` 联通

### D9

- [ ] 实现 `search`（FTS5 BM25 + 过滤）
- [ ] 实现 `read_raw_anchor`（heading / paragraph / char span anchor 解析）
- [ ] 实现 `read_claim`、`graph_neighbors`
- [ ] 单测：anchor 解析 50 个边界用例

### D10

- [ ] `agent_handshake`：写 audit + 返回 instructions path
- [ ] Review queue 持久化（`.llmwiki/review-queue.jsonl` + table）
- [ ] 实现 `propose_page`、`propose_edit`、`propose_claim`
- [ ] 实现 `request_review` 汇总

### D11

- [ ] `log_append`（唯一直接写 MCP 工具）
- [ ] `lint_run`（先实现 4 个 check：orphan / broken_link / schema_violation / missing_index_entry）
- [ ] `acquire_lock` / `release_lock`

### D12

- [ ] CLI `llmwiki review list / show / accept / reject / diff`
- [ ] Review accept 流程：apply patch + git commit + change_log
- [ ] Idempotency key 处理

### D13

- [ ] PDF ingest pipeline（`pdftotext` + heading 识别）
- [ ] 图片 ingest（仅元数据 + 可选 OCR；MVP 不强制 OCR）
- [ ] Watcher：macOS FSEvents 接入；事件 debounce

### D14（周末验收）

- Demo：在 Claude Code 里
  - `agent_handshake` 注册
  - 把一篇论文丢到 `raw/inbox/`
  - 调 `read_raw` 阅读
  - 调 `propose_page`（source + entity + concept + claim）
  - 调 `request_review` 汇总
  - 用 `llmwiki review accept` 合并
  - 调 `search` 验证已索引
- 写 retrospective。

**W2 出口标准**：上面 demo 流畅、无 ERROR、git 历史干净。

---

## W3：Review + Lock + Lint + 多 agent（D15–D21）

**目标**：多个 agent 同时工作不冲突；lint 跑得动；review UI 可用。

### D15

- [ ] Lock manager 完整实现（TTL、stale 清理、`force` 标志）
- [ ] Review queue 状态机完善（superseded、conflict 检测）
- [ ] 单测：模拟两个 agent 抢同一个 page

### D16

- [ ] Claim 校验：quote_hash 反验证；DRIFT 错误
- [ ] `propose_claim` 拒绝无源 claim（除非 speculation=true）
- [ ] Rejection memory（`.llmwiki/rejections.jsonl`）

### D17

- [ ] Lint 全套规则：contradictions / stale / unverified / duplicate_entity（基于 alias）
- [ ] Lint incremental（只扫 changed_since_last_lint）
- [ ] CLI: `llmwiki lint [--fix]`

### D18

- [ ] Windows watcher：`ReadDirectoryChangesW` 接入 + debounce + buffer overflow 处理
- [ ] Windows 长路径 `\\?\` 自动处理
- [ ] Windows 编码（manifest UTF-8、PowerShell 兼容性）

### D19

- [ ] 文件 bridge：JSON-RPC over Unix socket / named pipe
- [ ] CLI 兜底通道：`llmwiki cat-source / cat-page`
- [ ] Codex / Hermes 走 CLI bridge demo

### D20

- [ ] Multi-agent E2E：写一个测试脚本，两个 agent 同时 ingest + lint，验证最终 wiki 一致 + change_log 完整
- [ ] 失败注入：杀掉 daemon → 重启恢复 review queue
- [ ] `llmwiki doctor`：自检（路径、权限、git、watcher、SQLite）

### D21（周末验收）

- Demo：Claude Code + Codex CLI 同时工作
  - Claude 在 ingest 一篇文章
  - Codex 在跑 lint（命中 broken_link，proposal）
  - 用户 accept Claude + 部分 Codex 的 proposals
  - 一切 git log + change log 一一对应
- 写 retrospective。

**W3 出口标准**：两个 agent 同时跑无冲突；lint 全套规则在测试 vault 上无 false positive。

---

## W4：跨平台打磨 + 自用 + 文档（D22–D30）

**目标**：本人开始每天用；文档完整；可发布 v0.1.0。

### D22

- [ ] Windows 安装包：MSI（用 `wix`）or scoop bucket；Scheduled Task 自动注册
- [ ] macOS 安装包：homebrew tap；launchd plist 自动注册
- [ ] `llmwiki uninstall --purge`

### D23

- [ ] Obsidian vault 兼容验证（在 vault 里打开 Obsidian，`wiki/` 渲染正确、图谱视图正常）
- [ ] frontmatter 字段与 Dataview 兼容（测试一些查询）

### D24

- [ ] `embedding`（可选）模块：本地 `bge-small` via `llama.cpp` HTTP；rerank API
- [ ] `search` 增加 hybrid 模式（FTS5 + 向量 rerank）
- [ ] 性能 benchmark（10k 文件）

### D25

- [ ] OCR ingest pipeline（macOS Vision via shortcuts、Windows PowerToys OCR、Tesseract 兜底）
- [ ] Audio transcribe（whisper.cpp 集成）
- [ ] 这些都进 `wiki/_review/`

### D26

- [ ] 完善 instruction 模板：AGENTS.md / CLAUDE.md / HERMES.md（包括 Cursor、Cline、Codex CLI 的注释段）
- [ ] schema page-schemas.md：claim / entity / concept / source / topic / query
- [ ] lint-rules.md

### D27

- [ ] 文档：README、安装手册（macOS、Windows 各一份）、Quickstart、FAQ
- [ ] Onboarding 视频脚本（先不录）
- [ ] 示例 vault（10 篇资料、覆盖 5 种类型）

### D28

- [ ] 本人自用 dogfooding：把自己最近一周的论文 / 文章全 ingest，跑 query
- [ ] 找 3 个朋友试用，收 bug

### D29

- [ ] Bug 修复（基于 dogfooding 反馈，优先 P0/P1）
- [ ] 性能微调（看 prof 报告）

### D30（发布日）

- [ ] v0.1.0 打 tag、生成 release notes、上传二进制
- [ ] 写 launch 博客（可选）
- [ ] 收集前 10 个用户反馈，作为 v0.2 优先级输入
- [ ] 总结 retrospective + roadmap v0.2

---

## 关键里程碑总览

| 日期 | 里程碑 | 验收命令 |
|---|---|---|
| D7  | CLI + 文件 IO 跨平台 | `llmwiki init && llmwiki ingest sample.md` |
| D14 | MCP server 可被 Claude Code 端到端使用 | Claude Code 跑完 ingest 流程 |
| D21 | 多 agent 协作不冲突 | demo 脚本：Claude + Codex 同时跑 |
| D30 | v0.1.0 发布 | release tarball + 安装命令 |

---

## 风险与缓冲

每周计划只填 80%，留 20% 做以下任意一件事：

- 修上周遗留 bug；
- 调研下周技术难点（如 Windows watcher 抖动）；
- 写文档；
- 重构上周的临时代码。

**绝对不允许做的**：

- 提前做 v0.2 feature；
- 优化"看起来很 clever"但 MVP 用不到的代码；
- 引入新依赖；
- 调研未列在 roadmap 的工具。

---

## 工程纪律

- 每天结束跑：`gofmt`、`golangci-lint`、`go test ./...`、`pytest`，全绿才能下班。
- 每周一开周计划会，周五开 retro。
- 每个 PR 必须包含：测试、文档、`docs/` 或 `templates/` 更新（如果涉及）。
- 不允许 `git push --force`；不允许 squash 老 commit。
- 任何 schema 变更 → schema_version bump + migration 脚本。

---

## D31+ Outlook（不在 MVP 范围）

排在 v0.2 优先级队列里，不在 30 天内做：

1. Web dashboard（localhost，只读）
2. PR-style review 模式
3. qmd 适配器
4. Linux 一等公民
5. NLI-based 矛盾检测
6. Slack / Notion / 邮件 ingest 适配器
7. Dream cycle 默认开启
8. 多 vault 联合查询

---

## 一句话总结

> **MVP 不在于功能多，而在于"我能不能从今天起每天用它，并相信它"。**
>
> 30 天结束时，如果我自己已经把 30 篇资料 ingest 进去、能 query 出有 citation 的答案、敢相信里面的 claim，
> 那 v0.1 就成立了。如果我自己都不敢用，先别发布。
