# 页面 Schema 与 Frontmatter 模板

> 所有 wiki page 的 frontmatter 字段定义 + 各类页面的 markdown 模板。
> 把此文件复制到 `schema/page-schemas.md`；linter 据此校验。

---

## 1. Frontmatter 公共字段（所有 page 必填）

```yaml
---
id: 01J5XK8M5G9P1ZWX0M           # ULID，文件名 == id
type: entity | concept | claim | source | topic | query | misc
title: "Karpathy, Andrej"        # 显示名（可含中文）
created: 2026-05-20T14:23:00Z    # ISO 8601 UTC
updated: 2026-05-20T14:23:00Z
status: draft | published | archived | needs_reverify | stale
schema_version: 1.0
agent_authored: claude-code      # 创建该 page 的 agent；user 手写则填 'user'
---
```

### 可选公共字段

```yaml
tags: ["llm", "memex", "pkm"]
aliases: ["AK"]
sources: [01J5..., 01J5...]      # 这个 page 整体引用的 raw source 列表
relations:                       # 与其他 page 的关系
  - rel: "uses"
    target: 01J5XP1...
    claim: 01J5XR1...            # 强制：这条关系背后的 claim
inbound_links: [...]             # daemon 自动维护，不要手写
outbound_links: [...]            # daemon 自动维护
```

linter 强制：
- `id` 必须是合法 ULID 且与文件名一致；
- `type` 必须在 enum 内；
- `created` ≤ `updated`；
- `agent_authored` 必填（user / agent 名）；
- `relations[*].claim` 必填（关系必须有凭据）。

---

## 2. SourcePage 模板（`wiki/sources/<id>.md`）

每个 `raw/` 文件 1:1 对应一个 source page。

```markdown
---
id: 01J5XK8M5G9P1ZWX0M
type: source
title: "Karpathy: LLM Wiki gist (2025)"
created: 2026-05-20T14:23:00Z
updated: 2026-05-20T14:23:00Z
status: published
schema_version: 1.0
agent_authored: claude-code

raw_path: raw/articles/2025-karpathy-llm-wiki.md
raw_hash_sha256: abc123...
raw_size_bytes: 11985
raw_mime: text/markdown
raw_added_at: 2026-05-20T14:20:00Z

publish_info:
  authors: ["Andrej Karpathy"]
  published_at: 2025-XX-XX
  url: https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f
  venue: "GitHub Gist"

claims_extracted: [01J5XR1..., 01J5XR2..., 01J5XR3...]
entities_mentioned: [01J5XM1..., 01J5XM2...]
concepts_mentioned: [01J5XP1..., 01J5XP2..., 01J5XP3...]

tags: ["llm-wiki", "rag", "memex", "personal-knowledge"]
---

## 一句话摘要

Karpathy 提出用 LLM 持续维护一个 Markdown wiki，作为 RAG 的替代方案；
强调 wiki 是"持续累积的 artifact"而非"查询时拼接的缓存"。

## 关键 takeaways

- LLM Wiki 与 RAG 的本质差异：[[claim:01J5XR1...]]
- 三层结构（raw / wiki / schema）：[[claim:01J5XR2...]]
- 工具链建议（Obsidian + git + qmd）：[[claim:01J5XR3...]]
- ...

## Outline

1. The core idea
2. Architecture (raw / wiki / schema)
3. Operations (ingest / query / lint)
4. Indexing and logging
5. CLI tools (optional, qmd)
6. Tips and tricks

## 我的笔记（user 可手动追加）

> （此段不会被 lint 强制；agent 不主动写）
```

---

## 3. EntityPage 模板（`wiki/entities/<id>.md`）

```markdown
---
id: 01J5XM1A2B3C4D5E6F
type: entity
title: "Karpathy, Andrej"
created: ...
updated: ...
status: published
schema_version: 1.0
agent_authored: claude-code

canonical: true                    # 或 canonical_id: 01J5XM... 指向 canonical
entity_kind: person                # person | organization | place | product | event | other
aliases: ["Andrej Karpathy", "ak"]

notable_claims:
  - 01J5XR1...
  - 01J5XR4...

relations:
  - rel: "founder_of"
    target: 01J5XM2...           # entity: Eureka Labs
    claim: 01J5XR5...
  - rel: "former_employee_of"
    target: 01J5XM3...           # entity: Tesla
    claim: 01J5XR6...

tags: ["ai-researcher", "education"]
---

## 简介

Andrej Karpathy 是一位 AI 研究者和教育者，知名于其在深度学习教学方面的贡献。
具体事实参见 `notable_claims`，请勿在此段落中重复事实陈述。

## 别名 / 拼写

- Andrej Karpathy
- AK

## 相关 entity / concept

参见 frontmatter 的 `relations` 与下方的 [[wiki link]]：

- 工作经历：[[01J5XM3]]（Tesla）
- 创业项目：[[01J5XM2]]（Eureka Labs）
- 知名思想：[[01J5XP2]]（LLM Wiki pattern）
```

linter 规则：
- `type=entity` 必须有 `entity_kind`；
- 非 canonical 必须有 `canonical_id`；
- `relations[*]` 必须有 `claim`；
- 正文不应包含事实陈述（用 claim）。

---

## 4. ConceptPage 模板（`wiki/concepts/<id>.md`）

```markdown
---
id: 01J5XP1A2B3C4D5E6F
type: concept
title: "LLM Wiki pattern"
created: ...
updated: ...
status: published
schema_version: 1.0
agent_authored: claude-code

aliases: ["LLM-maintained wiki"]
parent_concepts: [01J5XP5...]      # 上位概念（如 PKM）
related_concepts: [01J5XP3..., 01J5XP6...]  # Memex, RAG
defining_claims:
  - 01J5XR1...                     # 定义性 claim
key_distinctions:
  - vs: 01J5XP6...                 # RAG
    claim: 01J5XR7...

tags: ["pkm", "llm", "knowledge-management"]
---

## 定义

LLM Wiki 是一种知识库模式，由 LLM 持续维护一个由 Markdown 文件组成的、结构化的、可累积的 wiki，
替代传统 RAG 的"每次查询时重组"模式。具体定义见 [[claim:01J5XR1]]。

## 与相关概念的关系

- 与 [[01J5XP6]] RAG 的差异：见 [[claim:01J5XR7]]
- 与 [[01J5XP3]] Memex 的关系：LLM Wiki 可视为 Memex 思想的 LLM 时代具体实现
```

---

## 5. ClaimPage 模板（`wiki/claims/<id>.md`）

```markdown
---
id: 01J5XR1A2B3C4D5E6F
type: claim
title: "LLM Wiki is a persistent, compounding artifact (Karpathy)"
created: ...
updated: ...
status: verified
schema_version: 1.0
agent_authored: claude-code

text: |
  Karpathy 定义 LLM Wiki 时强调其核心特征是
  "a persistent, compounding artifact"——
  与 RAG 在查询时临时拼接知识形成对比。

confidence: 0.95
speculation: false
last_verified: 2026-05-20T14:23:00Z

sources:
  - raw_id: 01J5XK8M5G9P1ZWX0M
    anchor: "#The core idea"
    quote: "the wiki is a persistent, compounding artifact"
    quote_hash: 7c8d9e0f1a2b3c4d...

supports: []
contradicts: []
refines: []
used_by:
  - 01J5XP1...        # concept: LLM Wiki pattern
  - 01J5XP2...        # topic: PKM patterns
  - 01J5XV1...        # query: RAG vs LLM Wiki

tags: ["definition", "llm-wiki"]
---

## 文本

> Karpathy 定义 LLM Wiki 时强调其核心特征是 "a persistent, compounding artifact"——
> 与 RAG 在查询时临时拼接知识形成对比。

## 来源

引用 [[source:01J5XK8M5G9P1ZWX0M]] 的 `#The core idea` 段落：

> "the wiki is a persistent, compounding artifact"

## 状态

- confidence: 0.95
- 最后核实：2026-05-20
- 状态：verified（由 user 在 review 2026-05-20 确认）
```

---

## 6. TopicPage 模板（`wiki/topics/<id>.md`）

```markdown
---
id: 01J5XT1A2B3C4D5E6F
type: topic
title: "Personal Knowledge Management (PKM)"
created: ...
updated: ...
status: published
schema_version: 1.0
agent_authored: claude-code

scope: "如何用工具 + 流程长期管理个人知识"
related_concepts: [01J5XP1..., 01J5XP3..., 01J5XP6...]
key_entities: [01J5XM1..., 01J5XM4...]
key_claims:
  - 01J5XR1...
  - 01J5XR10...

evolving_thesis: |
  PKM 的核心痛点不是"读不完"，而是"维护不动"。
  LLM Wiki 的关键贡献是让 maintenance cost ≈ 0。

tags: ["pkm", "synthesis"]
---

## 这个主题是什么

PKM = Personal Knowledge Management。本 topic page 综合了 vault 内关于 PKM 的多个 concept / claim / source。

## 当前 thesis

PKM 的核心痛点不是"读不完"，而是"维护不动"。
LLM Wiki 的关键贡献是让 maintenance cost ≈ 0（见 [[claim:01J5XR10]]）。

## 关键概念

- [[01J5XP1]] LLM Wiki pattern
- [[01J5XP3]] Memex
- [[01J5XP6]] RAG

## 关键人物

- [[01J5XM1]] Karpathy
- [[01J5XM4]] Vannevar Bush

## 已沉淀的 query

- [[01J5XV1]] "RAG 和 LLM Wiki 的区别？"
- [[01J5XV2]] "如何防止 LLM Wiki 幻觉？"

## 待研究

- 团队 PKM vs 个人 PKM 的差异
- 长期记忆模块设计（涉及画像）
```

---

## 7. QueryPage 模板（`wiki/queries/<id>.md`）

```markdown
---
id: 01J5XV1A2B3C4D5E6F
type: query
title: "RAG 和 LLM Wiki 的区别？"
created: 2026-05-20T15:00:00Z
updated: 2026-05-20T15:00:00Z
status: published
schema_version: 1.0
agent_authored: claude-code

question: "RAG 和 LLM Wiki 的本质区别是什么？"
asked_by: user
answered_by: claude-code
asked_at: 2026-05-20T14:55:00Z

claims_used:
  - 01J5XR1...     # LLM Wiki is compounding
  - 01J5XR7...     # vs RAG distinction
  - 01J5XR8...
entities_mentioned: []
concepts_mentioned: [01J5XP1..., 01J5XP6...]

quality_grade: high   # high | medium | low；user 打分
promote_to_topic: false   # 是否升级为 topic page
---

## 问题

RAG 和 LLM Wiki 的本质区别是什么？

## 答案

核心区别：**RAG 在查询时临时拼接，LLM Wiki 是长期累积的产物。**

具体看四个维度：

1. **知识形态**：RAG 是"每次重读资料"，LLM Wiki 是"边读边整理笔记，下次直接看笔记"[^c1]。
2. **跨文档综合**：RAG 每次重做综合，稳定性差；LLM Wiki 综合一次，长期复用 [^c2]。
3. **引用质量**：RAG 是 chunk 引用，上下文易丢；LLM Wiki 是 claim 级引用，含 anchor + hash [^c3]。
4. **可审计**：RAG 难追溯；LLM Wiki 有 git history + change_log + quote_hash [^c1]。

[^c1]: [[claim:01J5XR1]] (← raw/articles/2025-karpathy-llm-wiki.md#The core idea)
[^c2]: [[claim:01J5XR7]] (← raw/articles/2025-karpathy-llm-wiki.md#Architecture)
[^c3]: [[claim:01J5XR8]] (← raw/articles/2025-karpathy-llm-wiki.md#Operations)
```

---

## 8. log.md 格式（append-only）

```markdown
# Wiki Log

This file is append-only. Each entry starts with `## [YYYY-MM-DD HH:MM] <kind> | <title>`.
Use `grep "^## \[" log.md | tail -10` to see recent activity.

---

## [2026-05-20 14:23] ingest | Karpathy LLM Wiki gist

source: raw/articles/2025-karpathy-llm-wiki.md (sha256: abc...)
agent: claude-code
review_bundle: b-001
pages_created: 4 (sources/01J5XK..., concepts/01J5XP1..., entities/01J5XM1..., claims/01J5XR1..×3)
pages_edited: 2 (index.md, concepts/01J5XP6...)
log: detailed change in .llmwiki/change-log.jsonl seq 1240-1247

## [2026-05-20 15:00] query | RAG 和 LLM Wiki 的区别？

asked_by: user
answered_by: claude-code
filed_as: wiki/queries/01J5XV1...md
quality_grade: high

## [2026-05-20 18:00] lint | weekly full lint

report: .llmwiki/lint-reports/2026-05-20.jsonl
issues:
  - orphan_pages: 2
  - broken_links: 0
  - unverified_claims: 5
  - contradictions: 0
  - duplicate_entities: 1
```

---

## 9. index.md 格式

```markdown
# Wiki Index

> Auto-maintained by llmwiki. Last update: 2026-05-20T18:00:00Z. Schema: 1.0.

## Topics

- [[01J5XT1]] **Personal Knowledge Management (PKM)** — 综合视角；关键 thesis 见 page
- [[01J5XT2]] **Multi-agent collaboration** — 多 agent 协作模式

## Concepts

- [[01J5XP1]] **LLM Wiki pattern** — Karpathy 提出的 LLM 维护知识库模式
- [[01J5XP2]] **Memex** — Vannevar Bush 1945 个人知识系统设想
- [[01J5XP3]] **RAG** — Retrieval-Augmented Generation

## Entities

- [[01J5XM1]] **Karpathy, Andrej** — AI 研究者
- [[01J5XM2]] **Eureka Labs** — Karpathy 创办的 AI 教育公司
- [[01J5XM3]] **Tesla** — 汽车与 AI 公司

## Sources (recent 10)

- [[01J5XK8...]] **Karpathy: LLM Wiki gist (2025)** — gist.github.com
- ...

## Recent Queries

- [[01J5XV1]] **RAG 和 LLM Wiki 的区别？** — 2026-05-20

## Stats

- Sources: 47
- Entities: 23
- Concepts: 18
- Claims: 132 (verified 89, unverified 38, disputed 5)
- Topics: 4
- Queries archived: 12
```

---

## 10. lint-rules.md 简表

详细 lint 规则参考 `docs/research-qa.md` Q5 部分。schema 中存档一份精简版即可。

```yaml
rules:
  - id: L-001
    name: orphan_page
    severity: warning
    check: inbound_links == 0
    fix_hint: "merge / delete / add inbound"
  - id: L-002
    name: broken_link
    severity: error
    check: "[[id]] references missing page"
    fix_hint: "fix id or create page"
  - id: L-003
    name: contradiction
    severity: warning
    check: claim.contradicts != [] or NLI(claim_a, claim_b) > 0.8
    fix_hint: "review and resolve in _review/contradictions/"
  # ... 见 docs/research-qa.md Q5 完整表
```

---

## 11. 校验工具

```bash
llmwiki lint --check-schema       # 仅 schema_violation
llmwiki page validate <id>        # 单 page 校验
llmwiki page validate --all
```

---

## 12. 演化

schema 是 living document：

- 修改时 bump `schema_version`；
- breaking changes 在 `schema/AGENTS.md` 顶部记录；
- 同时提供 `llmwiki migrate <to-ver>` 脚本（如果需要数据变换）；
- 所有 agent 在 handshake 时拿到新版本号，重读 schema。
