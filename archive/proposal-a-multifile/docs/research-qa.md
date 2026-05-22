# 重点研究问答（8 个核心问题）

> 用户在需求里提出了 8 个研究方向。这里逐一深度回答，并给出工程取舍。
> 引用外部参考时给出链接。

---

## Q1. "LLM 维护 Wiki" 相比普通 RAG 的优势、限制和风险是什么？

### 优势

| 维度 | RAG | LLM Wiki |
|---|---|---|
| 知识形态 | 每次查询时临时拼接 | 持久化、累积、演化 |
| 跨文档综合 | 每次重做、稳定性差 | 一次综合、长期可用 |
| 引用质量 | chunk 引用、上下文易丢 | claim 级引用、含 anchor + hash |
| 错误修正 | 改 prompt / 改 retrieval | 改 wiki 一次，永久生效 |
| 用户参与 | 几乎只在 query 时 | sourcing / review / direction |
| 离线可用 | 需 retrieval pipeline | 直接打开 markdown 看 |
| 可审计 | chunk 来源 ≠ 可信链 | git history + change_log + quote_hash |

**核心优势一句话**：RAG 是"每次问都重新读书"，LLM Wiki 是"边读边整理笔记，下次直接看笔记"。
笔记不是缓存，是**真正的衍生品**——里面有跨文档的连接、有作者的判断、有时间维度的演化。

参考：Karpathy 原文 [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
强调："The wiki is a persistent, compounding artifact."

### 限制

1. **冷启动成本高**：第一次 ingest 一篇资料要触发多个 wiki 页面更新，单次开销远高于 RAG。
2. **修改放大**：一个 source 改了，可能要更新 10+ wiki 页面；要做好"何时回填、何时让 lint 标记"。
3. **schema 设计成本**：用户/LLM 需要协同设计页面类型、frontmatter、命名约定。RAG 几乎零设计。
4. **离线版本可能落后**：如果你用 LLM Wiki 没勤维护，wiki 比 raw 旧 → 给出过时答案。
5. **不适合"一次性、超大资料"场景**：比如全本《圣经》搜一句话，RAG 更经济。

### 风险

1. **幻觉污染**（Critical）：见 `docs/risks.md` R-01。
2. **过度结构化**：把所有问题都套进 entity/concept 框架，反而失去原文的语境。
3. **羊群效应**：错误 claim 一旦写进 wiki，后续 agent 会引用强化它。
4. **维护成本错觉**：以为 LLM 完全自主，结果 lint 队列堆积没人理。

### 适用 vs 不适用

| 适用 | 不适用 |
|---|---|
| 长期跟踪一个或多个话题 | 一次性查询单个文档 |
| 资料持续增加 | 资料静态、不变 |
| 关心跨文档关系 | 只关心单文档内问答 |
| 需要可审计 / 可追溯 | 只想要"差不多对"的回答 |
| 用户愿意维护 schema | 用户希望"丢进去就能用" |

---

## Q2. 如何保证 wiki 不幻觉、不污染、不把未经证实的信息写成事实？

### 五层防御

**第一层：claim 必须有 source（数据契约）**

- `propose_claim` 强制 `sources[]` 非空；
- 每个 source 含 `raw_id + anchor + quote_hash`；
- 无源 → 必须显式 `speculation=true`；
- daemon 在 accept 前再算一遍 quote_hash 验证。

**第二层：不确定性显式编码**

- `confidence: 0..1`、`status: unverified / verified / disputed / retracted` 必填；
- UI 渲染时不同 status 颜色区分；
- 任何 claim 改动都更新 `last_verified` 时间戳。

**第三层：Review queue（人在闸门）**

- 默认所有写入先到 `wiki/_review/`；
- User 看 diff、accept / reject；
- 拒绝原因写入 rejection memory，agent 下次握手时拿到。

**第四层：Lint 主动反幻觉**

- `unverified_claims` 数量超阈值告警；
- `contradictions` 自动标记并归档到 `wiki/_review/contradictions/`；
- "事实陈述"模式匹配（"是"、"将"、"于…年"等）但无 claim 关联 → 提示。

**第五层：可逆**

- 全 git + change_log；
- 任何 commit 可 30 秒内 revert；
- markdown 是真理，索引可重建。

### Agent instruction 硬约束

`AGENTS.md` 必须包含的禁令：

> 你不可以编造引用。
> 你不可以把推测写成事实。
> 你不可以引用 wiki 自己作为 claim 的 source（必须回到 raw/）。
> 你不可以删除 raw/ 文件。
> 你不可以修改 schema/ 文件。

详见 `templates/AGENTS.md`。

---

## Q3. 如何处理重复概念、同义词、层级混乱、关系类型缺失？

### 重复 / 同义词

- **canonical_id 机制**：每个 entity 必须是 canonical 或指向 canonical（详见 `docs/agent-protocol.md` §9）。
- **alias 字段**：`aliases: ["Andrej Karpathy", "ak"]`，linter 检测新 ingest 时命中 alias 自动指向 canonical。
- **propose_merge 流程**：lint 检测疑似重复 → 进 `wiki/_review/merge-suggestions/` → 用户决议。
- **物理保留**：subsume 的 page 标 `archived` 而非删除，避免链接断裂。

### 层级混乱

- **不强制硬分类**。entity / concept / topic 是分类但不构成树。
- **关系图为主，目录为辅**：`relations` 表才是真实层级，目录只是浏览方便。
- **topic 是"viewpoint"**：可跨 entity / concept 聚合，不是 entity 的祖先。
- **重组成本低**：因为用 ULID 而不是 slug，移动文件不破链接。

### 关系类型缺失

- **预定义 vocabulary**：在 `schema/page-schemas.md` 列出常用 relation type（`works_at`、`uses`、`extends` 等）。
- **agent 创建新 relation 必须解释**：lint 提示用户"这个新 relation 是否要加入 vocabulary"。
- **每个 relation 背后必须有 claim**：`relations.source_claim_id` 非空，否则就是无凭据的连接 → linter 拒绝。
- **强制性 vs 描述性区分**：`works_at`（强约束）vs `mentioned_with`（弱关联），前者要 claim，后者只是 co-occurrence。

---

## Q4. 如何把 claim 作为一等公民，每个 claim 都能追溯 source？

### Data model

```
ClaimPage (wiki/claims/<id>.md)
├── id           (ULID)
├── text         （≤ 800 字符）
├── confidence   (0..1)
├── status       (unverified / verified / disputed / retracted)
├── speculation  (bool)
├── sources[]
│   ├── raw_id     ← 必须存在
│   ├── anchor     ← "#heading/sub-heading" or "p:42" or "c:1024-1500"
│   ├── quote      ← verbatim
│   └── quote_hash ← sha256(quote)
├── supports[]      ← ClaimPage[]
├── contradicts[]   ← ClaimPage[]
├── refines[]       ← ClaimPage[]
└── used_by[]       ← Page[] (entity/concept/topic that reference this claim)
```

### 物理布局

每个 claim 独立 markdown 文件（`wiki/claims/<id>.md`）。

优点：
- git diff 粒度细，可追责谁/何时改了；
- 跨页面引用通过 `[[claim:<id>]]` 直接复用；
- lint 易统计；
- 用户可单独 archive / restore 一个 claim。

缺点：
- 文件多，目录庞大；
- 阅读体验差（需 entity/concept 页把它们 inline）。

**取舍**：MVP 默认每个 claim 独立文件（粒度优先）；
v0.2 提供"inline claim 视图"渲染（entity 页面渲染时自动展开引用的 claim）。

### 引用 source 的细节

anchor 编码：

| 类型 | 例 | 何时用 |
|---|---|---|
| Heading 路径 | `#Architecture/Operations` | 文档有清晰 heading |
| 段落索引 | `p:42` | 长文档、无 heading |
| 字符范围 | `c:1024-1500` | 精细引用 / PDF |
| 时间戳（音视频） | `t:00:42:30-00:43:15` | transcript |
| 行范围（代码） | `L:120-145` | 源码引用 |

`quote_hash` 是 anchor 取到的文本的 sha256。daemon 每次读 anchor 时算 hash → 跟 claim 里存的对比 → 不一致就标 `DRIFT`。

### 与外部世界的关系

如果 source 是网页：

- 必须先用 Obsidian Web Clipper 或 `monolith` 落本地（`raw/articles/<id>.md`）。
- 永远不引用未落地的 URL（防 link rot）。
- claim 的 source 字段始终指 raw_id，不指 URL。

---

## Q5. 如何进行 periodic lint：孤立页面、断链、矛盾、陈旧结论、缺失引用、重复实体？

### Lint 规则表

| 规则 ID | 检查内容 | 算法 | 修复建议 |
|---|---|---|---|
| `L-001` orphan | 入链度 = 0 | 反向索引扫 | 合并 / 删除 / 加链接 |
| `L-002` broken_link | `[[id]]` 指向不存在 | 全扫 + lookup | 修正 id 或新建 page |
| `L-003` contradiction | claim 标记 contradicts / NLI 检查 | 静态字段 + 可选 NLI 模型 | 进 `_review/contradictions/` |
| `L-004` stale_claim | claim 的 source 被 newer source 覆盖 | source.added_at vs claim.last_verified | 标 stale + 重 verify |
| `L-005` unverified_claim | confidence < 阈值 且 sources 弱 | 静态阈值 | reverify 或降级 speculation |
| `L-006` missing_source | 非平凡断言 但无 claim 关联 | 正则 + LLM 抽取建议 | propose_claim |
| `L-007` duplicate_entity | alias 命中 / 名字相似度 > 0.85 | string sim + alias | propose_merge |
| `L-008` schema_violation | frontmatter 必填缺失 | YAML schema validate | 自动补 / 拒绝 commit |
| `L-009` missing_index | page 存在但 index.md 未列 | diff page set vs index | 自动补 entry |
| `L-010` index_phantom | index.md 列了不存在 page | 反向扫 | 删除 entry |
| `L-011` huge_page | page > 50KB markdown | size check | 拆分提议 |
| `L-012` provenance_depth | claim 引 wiki 而非 raw | 关系图遍历 | reverify |
| `L-013` filename_violation | 非 kebab-case / 含非 ASCII / Windows 保留字 | regex | rename 提议 |
| `L-014` git_dirty | uncommitted changes | git status | 提示用户 |

### 调度

- **手动**：`llmwiki lint`
- **每次写入后**：daemon 对 changed pages 触发 incremental lint（仅扫脏页面）
- **每天 / 每周**：cron / launchd / Scheduled Task 跑 full lint
- **schema 变更**：立即 full lint

### 增量 lint 算法

```
changed_pages = git diff HEAD@{last-lint} --name-only -- wiki/
affected_pages = changed_pages ∪ inbound_links(changed_pages)
for p in affected_pages:
    run rules in [L-001, L-002, L-008, L-009, L-013]
for p in changed_pages where p.type == 'claim':
    run rules in [L-003, L-004, L-005, L-006, L-012]
```

### 报告格式

`.llmwiki/lint-report-<date>.jsonl`：

```jsonl
{"rule":"L-001","page_id":"01J...","severity":"warning","msg":"orphan page","fix_hint":"..."}
```

CLI: `llmwiki lint --report .llmwiki/lint-report-2026-05-20.jsonl --format table`

---

## Q6. 如何让 agent 查询时先读 index，再读相关页面，再回溯 source？

### 强制流程（写在 instructions 里）

> 查询流程（agent 必须按顺序执行）：
> 1. `list_index({category: 'all'})` — 强制先读
> 2. `search(query, k=10)` — 找 top-k
> 3. `read_page(id)` × top-k
> 4. 对每个 cited claim：`read_claim(claim_id)`，并对每个 source 跑 `read_raw_anchor`
> 5. 综合答案，写出 citation
> 6. 询问用户："归档为 wiki/queries/<id>.md ？"

agent 不按此顺序 → 答案质量降级，但不报错（schema 不能完全强制 LLM 的行为，只能强烈引导）。

### 为什么 index 必读

- index.md 是"鸟瞰图"，让 agent 不会在 1000+ 页面里盲搜；
- index.md 通常是 LLM 友好的（每个 entry 一行 + 摘要 + id）；
- 在 200 page 量级，单纯 `list_index` 就够回答 80% 的"X 是什么"问题。

### 中等规模再加 search

- 1k+ page 时，index 摘要不够细 → search 加 BM25；
- 10k+ page 时 search 加 embedding rerank；
- 100k+ page 时 search 分层（先 topic shortlist，再 claim level）。

### Citation 强制格式

agent 输出答案时强制：

```
Karpathy 在 2025 年的 gist 中提出 LLM Wiki [^c1]，强调 wiki 是 compounding artifact[^c2]。

[^c1]: wiki/claims/01J5xkA1.md  ← raw/articles/karpathy-llm-wiki.md#core-idea
[^c2]: wiki/claims/01J5xkA2.md  ← raw/articles/karpathy-llm-wiki.md#core-idea
```

每个 citation 必须含 claim_id + raw_id + anchor，三件套。

---

## Q7. 如何把一次高质量 query 的结果沉淀回 wiki？

### "Query as page" 原则

**每次 query 都是潜在的 wiki page。** 这是 Karpathy 原文最重要的洞察之一：
> "good answers can be filed back into the wiki as new pages"

### 自动归档机制

1. agent 答完，必须 prompt 用户：
   > 这个回答涉及 X 个 claim、Y 个 entity。归档为 `wiki/queries/<auto-id>.md` 吗？
2. 默认 yes（除非用户显式 no）。
3. 生成 page 含：
   - 问题原文
   - 答案 markdown
   - 所有引用的 claim_id
   - 所有涉及的 entity_id（自动加到 entity 页的 `mentioned_in`）
   - 时间戳 + 提问者 + 回答者（agent 名）

### Promotion 机制

- 同一 query 在 N 次（默认 3）内被重复提问 → lint 提议 promote 到 `wiki/topics/`。
- 一个 topic page 引用多个 query → 可以再 promote 到 `wiki/topics/<concept>.md` 作为综述。

### 反过来：query 启动 wiki

新建 wiki 时常用模式：

1. user 先抛一组 query；
2. agent 把每个 query 归档为 query page；
3. lint 检测"重复主题"→ 建议建立 topic page；
4. topic 引用回 query；
5. query 引用 raw source。

这是"bottom-up 构建知识库"，比"先规划 ontology 再写"更符合 LLM 时代。

### 失败模式

- query 数量爆炸：30 天内可能有上千 query → 自动归档要做好"质量过滤"（只归档 confidence > 阈值 或 user 显式打分高的）。
- query 质量低：聊天上下文里很多探索性问题，没必要全归档 → MVP 默认 user 确认后归档。

---

## Q8. （隐含）跨平台 + 多 agent + 大规模这些目标，工程上的根本取舍是什么？

总结性回答：

### 三个不可妥协

1. **Markdown 是真理**。任何索引、embedding、关系图都是派生数据，可丢可重建。这让"跨平台、跨工具、跨时间"
   都有不变的基线。
2. **Source-to-claim-to-page 三级追溯**。任何"事实"在 wiki 里都能 3 步走回 raw/，否则就不是事实。
3. **Daemon 是唯一的 commit 入口**。多 agent 不直接 git，避免协议爆炸。

### 三个明确的延后

1. **不内置 LLM 推理**。让用户自己接 agent。我们做协议、做工具、做 schema。
2. **不做云同步**。git remote 是兜底，团队/移动 v1.5 之后再考虑。
3. **不做企业级权限**。单用户产品。多用户用 Notion / Confluence。

### 三个反直觉的决定

1. **小规模别上 embedding**。`index.md` + ripgrep 在 100 page 时反而最准（LLM 看全貌 vs 看 chunk）。
2. **写工具默认走 review**。"AI 自动写知识库"听起来酷，但污染了就毁了。慢一点、人在闸门，长期赢。
3. **不做内置 GUI**。Obsidian 是最好的 GUI，且免费、社区强。我们做协议层、做 CLI，让 Obsidian 当前端。

---

## 附：进一步阅读

- Andrej Karpathy，[LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)
- Vannevar Bush，*As We May Think*（1945）— Memex 概念
- [Tolkien Gateway](https://tolkiengateway.net/wiki/Main_Page) — 优秀社区维护 wiki 范例
- [Obsidian](https://obsidian.md/) + [Dataview](https://github.com/blacksmithgu/obsidian-dataview)
- [Model Context Protocol](https://modelcontextprotocol.io/) — MCP 官方文档
- [qmd](https://github.com/tobi/qmd) — 本地 markdown 搜索 + MCP server
- [sqlite-vec](https://github.com/asg017/sqlite-vec) — SQLite 向量扩展
- [Obsidian Web Clipper](https://obsidian.md/clipper) — 网页落本地 markdown
- [Whisper.cpp](https://github.com/ggerganov/whisper.cpp) — 本地语音转录
- [ripgrep](https://github.com/BurntSushi/ripgrep) — 极快本地 grep
