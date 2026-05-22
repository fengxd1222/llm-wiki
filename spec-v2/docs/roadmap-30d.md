# 30 天 MVP Roadmap

> D1–D30 day-by-day 任务表 + 周末出口标准。
>
> 整合方案 B 的 day-by-day 颗粒度 + 方案 A 的"周末验收 / 出口标准 / 不通过不开新任务"纪律。

---

## 总览

| 周 | 主题 | 关键里程碑 |
|---|---|---|
| W1 | Skeleton + CLI + 文件 IO | `wikimind init / status / ingest` 可用，跨平台路径规范化 |
| W2 | MCP server + 索引 + worktree | Claude Code 经 MCP 完整 ingest + query；CJK 搜索通 |
| W3 | Review + Lock + Lint + 多 agent | Claude + Codex 同时跑无冲突；review queue 上限保护 |
| W4 | 跨平台 + demo + dogfooding + 发布 | `wikimind demo` 5 分钟通；Mac/Win CI 全绿；v0.1.0 |

**纪律**（来自方案 A）：
- 每周末是 demo / 验收点。**周末不通过则下周不开新任务**。
- 每周计划只填 80%，留 20% 修 bug / 调研 / 写文档。
- 绝不允许：提前做 v0.2 feature / 优化 MVP 用不到的代码 / 引入计划外依赖。

**假设**：1 个全职工程师；兼职按 60–80 小时/周折算。

---

## W0 — 启动日（0.5 天）

| 任务 | 验收 |
|---|---|
| 仓库结构 | `daemon/`（Go）、`worker/`（Python）、`cli/`、`tests/`、`docs/`、`templates/` 就位 |
| 语言/工具链 | Go 1.22+、Python 3.11+、官方 MCP SDK；golangci-lint、ruff、pre-commit |
| CI | GitHub Actions：macOS + Windows + Ubuntu runner |
| 任务跟踪 | GitHub Projects / Issue |
| 引用文档 | 全员读 `SPEC.md` + 4 篇 Wave 1 文档 |

---

## W1 — Skeleton + CLI + 文件 IO（D1–D7）

**目标**：vault 结构走通，CLI 能 init/status/ingest/log，文件读写跨平台规范化通过测试。

### D1（周一）
- [ ] `wikimind init <vault>`：创建三层目录（[architecture §4.1](architecture.md)），写默认 schema/，生成 `.wikimind/config.toml`
- [ ] `wikimind status`：vault 元信息（路径、文件计数、git 状态、health）
- [ ] CLI 框架（cobra），子命令 stub

### D2
- [ ] 配置加载 + 校验（config.toml schema、必填、vault root 可写）
- [ ] 跨平台 path normalize（POSIX 内部 + 系统调用前转换 + path traversal 拒绝）（[cross-platform §7](cross-platform.md)）
- [ ] 单元测试：100 个路径用例（含中文、长路径、符号链接逃逸）

### D3
- [ ] `raw/inbox/` 投递：`wikimind ingest <path>`
- [ ] sha256 + mtime + size 三件套；写 `sources` 表
- [ ] SQLite schema + migrate 工具（[architecture §4.2](architecture.md)）

### D4
- [ ] Markdown 解析：frontmatter（yaml.v3）+ heading + outbound `[[id]]` 链接
- [ ] `pages` 表写入
- [ ] `wikimind page show <id>` / `page list`

### D5
- [ ] FTS5 接入，**trigram tokenizer**（[cjk-tokenizer](cjk-tokenizer.md)）
- [ ] `wikimind query "..."` 走 BM25；短查询 fallback ripgrep
- [ ] SQLite ≥ 3.40 启动检查

### D6
- [ ] `log.md` append-only 写工具；`change-log.jsonl` 同步写（[agent-protocol §7](agent-protocol.md)）
- [ ] git auto-commit（每次 accept 一次 commit）
- [ ] `wikimind revert <seq>`：git revert 包装

### D7（周末验收）
- Demo：手动放 markdown 到 `raw/inbox/` → CLI ingest → 生成 source page → 更新 index.md → log.md 增行 → git commit
- 跨平台跑：macOS + Windows 各一遍
- **W1 出口标准**：上面 demo 在两平台都跑通；≥ 100 单测；CI 全绿；CJK 检索用例通过

---

## W2 — MCP server + 索引 + worktree（D8–D14）

**目标**：Claude Code 经 MCP 完整跑 ingest + query 流程。

### D8
- [ ] `wikimind mcp serve`（stdio）
- [ ] 实现 `wiki_info` / `read_page` / `read_raw` / `list_index`（[mcp-tools](mcp-tools.md)）
- [ ] 单测 + MCP inspector 联通

### D9
- [ ] `search`（FTS5 BM25 + filter）
- [ ] `read_raw_anchor`（heading/paragraph/char span anchor 解析 + quote_hash）
- [ ] `read_claim` / `graph_neighbors` / `get_history`
- [ ] 单测：anchor 解析 50 个边界用例

### D10
- [ ] `agent_handshake`：schema_version 协商 + worktree 分配（[agent-protocol §3-4](agent-protocol.md)）
- [ ] Git worktree per agent 创建/清理
- [ ] Review queue 持久化（reviews / bundles 表）

### D11
- [ ] `propose_page` / `propose_edit` / `propose_claim`（含 quote_hash 校验、provenance_depth 检查）
- [ ] `request_review` 汇总成 bundle
- [ ] `log_append`（唯一直写工具）

### D12
- [ ] CLI `wikimind review list / show / accept / reject / diff`
- [ ] Review accept 流程：apply patch + git commit + change_log（[architecture §3.3](architecture.md)）
- [ ] Idempotency key 处理

### D13
- [ ] PDF ingest pipeline（pdftotext + heading 识别）— Python worker
- [ ] 图片 ingest（元数据 + 可选 OCR；MVP 不强制 OCR）
- [ ] Watcher：macOS FSEvents 接入 + debounce

### D14（周末验收）
- Demo：Claude Code 里 — `agent_handshake` → 丢论文到 inbox → `read_raw` → `propose_*`（source + entity + concept + claim）→ `request_review` → CLI accept → `search` 验证已索引
- **W2 出口标准**：demo 流畅无 ERROR；git 历史干净；CJK 搜索在 Claude Code 里能查到中文子串

---

## W3 — Review + Lock + Lint + 多 agent（D15–D21）

**目标**：多 agent 同时工作不冲突；lint 跑得动；review queue 上限保护生效。

### D15
- [ ] Lock manager 完整（TTL、stale 清理、disconnected 60s 宽限、`force`）（[agent-protocol §5](agent-protocol.md)）
- [ ] Review queue 状态机完善（superseded、conflict 检测）
- [ ] 单测：模拟剧本 1（两 agent 抢同页）（[conflict-scenarios](conflict-scenarios.md)）

### D16
- [ ] Claim 校验：quote_hash 反验证；DRIFT 错误
- [ ] `propose_claim` 拒绝无源 claim / provenance_depth > 1
- [ ] Rejection memory（`.wikimind/rejections.jsonl`）
- [ ] claim 抽取算法 wired 进 `templates/CLAUDE.md`（[claim-extraction §6](claim-extraction.md)）

### D17
- [ ] Lint 全套 8 规则（orphan / broken_link / contradictions / stale / unverified_claim / duplicate_entity / schema_violation / missing_index_entry）
- [ ] Lint incremental（只扫 changed_since_last_lint）
- [ ] CLI `wikimind lint`

### D18
- [ ] **Review queue 上限保护**（soft 30 / hard 50 / critical 100）（[review-queue-policy §3](review-queue-policy.md)）
- [ ] Bundle 归并 + 一键 accept-bundle（[review-queue-policy §4](review-queue-policy.md)）
- [ ] 优先级排序 + `wikimind review today`

### D19
- [ ] Windows watcher：ReadDirectoryChangesW + USN journal 补漏（[cross-platform §3.2](cross-platform.md)）
- [ ] Windows 长路径 `\\?\`、CFA 检测
- [ ] CLI bridge（JSON-RPC over named pipe / unix socket）

### D20
- [ ] Multi-agent E2E：Claude + Codex 同时 ingest + lint，验证最终 wiki 一致 + change_log 完整
- [ ] 失败注入：杀 daemon → 重启恢复 review queue；杀 agent → lock 宽限释放
- [ ] `wikimind doctor` 自检（[failure-playbook §5](failure-playbook.md)）

### D21（周末验收）
- Demo：Claude Code（ingest 文章）+ Codex CLI（跑 lint 命中 broken_link）同时工作 → user accept 部分 → git log + change log 一一对应
- **W3 出口标准**：两 agent 同时跑无冲突；lint 全套规则在测试 vault 无 false positive；review queue 上限保护触发正常

---

## W4 — 跨平台 + demo + dogfooding + 发布（D22–D30）

**目标**：`wikimind demo` 5 分钟通；本人开始每天用；可发布 v0.1.0。

### D22
- [ ] `wikimind demo` 命令（[onboarding](onboarding.md)）：内嵌 3 个 sample raw + 确定性 claude-stub agent
- [ ] demo 5 阶段完整闭环（init → ingest → review → query → 浏览）
- [ ] demo 用时 CI 自动测（≤ 5 分钟）

### D23
- [ ] Windows 安装包（MSI / wix）+ Scheduled Task 自动注册
- [ ] macOS 安装包（homebrew tap）+ launchd plist
- [ ] `wikimind uninstall --purge`

### D24
- [ ] Obsidian vault 兼容验证（`wiki/` 在 Obsidian 渲染、双链、Dataview）
- [ ] frontmatter 与 Dataview 兼容测试

### D25
- [ ] Dream Cycle 基础版（audit + report 两阶段；consolidate/evolve 标记为 v0.1 beta）（[dream-cycle](dream-cycle.md)）
- [ ] Query Sedimentation 基础版（评分 + topic 沉淀；进 review queue）（[query-sedimentation](query-sedimentation.md)）

### D26
- [ ] 完善 templates：AGENTS / CLAUDE / CODEX / HERMES / CURSOR / page-schemas
- [ ] `wikimind revert-cascade` + 依赖图反向影响分析（[failure-playbook §3](failure-playbook.md)）

### D27
- [ ] 文档：README、安装手册（macOS / Windows 各一）、Quickstart、FAQ
- [ ] 失败 playbook 9 类命令全部测试通过

### D28
- [ ] 本人 dogfooding：把最近一周的论文/文章全 ingest，跑 query
- [ ] 找 3 个朋友试 `wikimind demo`，收 bug

### D29
- [ ] Bug 修复（dogfooding 反馈，优先 P0/P1）
- [ ] 性能微调（query < 100ms p95、ingest < 5s 等，[architecture §8](architecture.md)）

### D30（发布日）
- [ ] v0.1.0 打 tag、release notes、上传二进制
- [ ] 收集前 10 个用户反馈作为 v0.2 输入
- [ ] retrospective + v0.2 roadmap

---

## 关键里程碑

| 日期 | 里程碑 | 验收命令 |
|---|---|---|
| D7 | CLI + 文件 IO 跨平台 | `wikimind init && wikimind ingest sample.md` |
| D14 | MCP server 可被 Claude Code 端到端用 | Claude Code 跑完 ingest 流程 |
| D21 | 多 agent 协作不冲突 | demo 脚本：Claude + Codex 同时跑 |
| D30 | v0.1.0 发布 | `wikimind demo` 5 分钟通 + release tarball |

---

## MVP 验收（D30 出口）

SPEC §10 的 8 条验收判据，全部为真才发布：

1. 本人 dogfooding 30 天，ingest ≥ 30 篇
2. 任一 claim 30 秒回到 source 原文
3. Claude + Codex 同时跑无冲突，change_log 1:1 git commit
4. macOS + Windows 各装一遍全套通过
5. `wikimind demo` 5 分钟完整跑通
6. review queue 上限保护启用，每周 review ≤ 30 分钟
7. failure-playbook 9 类命令测试通过
8. macOS + Windows CI 全绿

任一不满足 → 不发布，延期或缩范围。

---

## D31+ Outlook（不在 30 天内）

v0.2 优先级队列：
1. Web dashboard（localhost 只读）
2. Dream Cycle 全四阶段默认开启
3. jieba tokenizer 可选
4. 本地 embedding（bge-small）
5. Cursor / Cline / OpenCode 深度适配
6. Slack / Notion / 邮件 ingest 适配器
7. NLI-based 矛盾检测
8. 多 vault 联合查询

---

## 工程纪律

- 每天结束跑：`gofmt`、`golangci-lint`、`go test ./...`、`ruff`、`pytest`，全绿才下班
- 每周一开周计划会，周五开 retro
- 每个 PR 必须含：测试、文档、相应 `docs/` 或 `templates/` 更新
- 不允许 `git push --force`；不允许 squash 老 commit
- 任何 schema 变更 → schema_version bump + migration 脚本

---

## 一句话总结

> 30 天、4 周、day-by-day。W1 骨架、W2 MCP、W3 多 agent、W4 demo + 发布。每周末硬验收，不过不
> 开新任务。D30 的判据只有一个本质问题：**我自己敢不敢每天用它。**
