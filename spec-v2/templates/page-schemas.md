---
schema_version: 1.0
last_updated: 2026-05-22
---

# Page Schemas

> 五类 wiki page 的 frontmatter schema + 正文模板。
> Agent propose 时必须符合对应 schema，否则 daemon 返回 `SCHEMA_VIOLATION`。
>
> 可直接复制到 vault `schema/` 目录使用。

---

## 0. 通用规则

### 0.1 文件命名

- 路径：`wiki/<type>s/<slug>.md`（如 `wiki/claims/wiki-is-compounding.md`）
- 文件名：**ASCII lowercase kebab-case**，正则 `^[a-z0-9][a-z0-9-]*\.md$`
- 中文 / 大写 / 完整标题 → 放 frontmatter 的 `title`

### 0.2 通用 frontmatter 字段

所有 page 类型共有：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `id` | string | ✅ | 全局唯一 ID（格式见各类型） |
| `type` | enum | ✅ | claim / entity / concept / source / topic |
| `title` | string | ✅ | 人类可读标题（可含中文） |
| `schema_version` | string | ✅ | 创建时的 schema 版本 |
| `created_by` | string | ✅ | `<agent> @ <timestamp>` |
| `updated_by` | string | ✅ | 最后修改者 |
| `created_at` | datetime | ✅ | ISO 8601 |
| `updated_at` | datetime | ✅ | ISO 8601 |
| `aliases` | string[] | ⬜ | 别名（用于搜索 / 去重） |
| `tags` | string[] | ⬜ | 自由标签 |

### 0.3 ID 格式

| 类型 | 格式 | 例 |
|---|---|---|
| claim | `cl-YYYY-MM-DD-NNN` | `cl-2026-05-21-001` |
| entity | `en-YYYY-MM-DD-NNN` | `en-2026-05-21-001` |
| concept | `co-YYYY-MM-DD-NNN` | `co-2026-05-21-001` |
| source | `sr-YYYY-MM-DD-NNN` | `sr-2026-05-21-001` |
| topic | `tp-YYYY-MM-DD-NNN` | `tp-2026-05-21-001` |
| review | `r-NNNN` | `r-0245` |
| bundle | `b-NNNN` | `b-0042` |

---

## 1. Claim

知识的最小可验证单元。详见 `docs/claim-extraction.md`。

### Frontmatter schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `confidence` | number 0–1 | ✅ | 见 claim-extraction §5 |
| `status` | enum | ✅ | `unverified` / `supported` / `disputed` / `refuted` |
| `speculation` | boolean | ⬜ | true 时可无 source，默认 false |
| `sources` | object[] | ✅* | ≥ 1（speculation=true 时可空） |
| `provenance_depth` | integer | ✅ | 必须 = 1（直接挂 raw） |
| `last_verified` | datetime | ✅ | 最近一次 quote_hash 校验通过时间 |
| `related` | string[] | ⬜ | `[[id]]` 列表 |
| `contradicts` | string[] | ⬜ | 矛盾的 claim id（status=disputed 时用） |

`sources` 每项：

```yaml
- raw_id: raw/inbox/karpathy-llm-wiki.md
  anchor: "#section-1-philosophy"
  quote: "every ingest, every query, every lint should make the wiki more valuable"
  quote_hash: a7f2e3c1          # 8-hex, 由 read_raw_anchor 获取
  span: [14, 19]                # [line_start, line_end]
```

> `quote` 摘录原文，**≤ 200 字符**（约 30 词）；`quote_hash` 是 `sha256(quote)` 前 8 位 hex，
> 必须由 `read_raw_anchor` 工具返回，agent 不得自行计算。

### 模板

```markdown
---
id: cl-2026-05-21-001
type: claim
title: "Wiki 是一个 compounding artifact"
schema_version: "1.0"
confidence: 0.92
status: supported
speculation: false
provenance_depth: 1
sources:
  - raw_id: raw/inbox/karpathy-llm-wiki.md
    anchor: "#section-1-philosophy"
    quote: "every ingest, every query, every lint should make the wiki more valuable"
    quote_hash: a7f2e3c1
    span: [14, 19]
related: ["[[karpathy]]", "[[compounding-artifact]]"]
created_by: "claude-code @ 2026-05-21T10:14:00Z"
updated_by: "you @ 2026-05-21T10:18:00Z"
created_at: 2026-05-21T10:14:00Z
updated_at: 2026-05-21T10:18:00Z
last_verified: 2026-05-21T10:18:00Z
aliases: ["compounding wiki"]
---

# Wiki 是一个 compounding artifact

Karpathy 在 LLM Wiki gist 中明确主张：wiki 是一个 [[compounding-artifact]]，
而不是临时缓存。每一次 ingest、query、lint 都应让 wiki 更值钱。
```

---

## 2. Entity

人、组织、产品、地点等具体实体。

### Frontmatter schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `entity_kind` | enum | ✅ | `person` / `org` / `product` / `place` / `other` |
| `bio` | string | ⬜ | 简短描述（单条事实集合，非 claim） |
| `related` | string[] | ⬜ | `[[id]]` 列表 |
| `external_refs` | string[] | ⬜ | 外部 URL（如官网、维基百科） |

### 模板

```markdown
---
id: en-2026-05-21-001
type: entity
entity_kind: person
title: "Andrej Karpathy"
schema_version: "1.0"
bio: "AI researcher; OpenAI 联合创始人; 前 Tesla 自动驾驶总监; LLM Wiki 模式提出者。"
aliases: ["Andrej Karpathy", "@karpathy", "karpathy"]
related: ["[[llm-wiki]]", "[[cl-2026-05-21-001]]"]
external_refs: ["https://karpathy.ai"]
created_by: "claude-code @ 2026-05-21T10:15:00Z"
updated_by: "claude-code @ 2026-05-21T10:15:00Z"
created_at: 2026-05-21T10:15:00Z
updated_at: 2026-05-21T10:15:00Z
---

# Andrej Karpathy

AI 研究者，LLM Wiki 模式的提出者。

## 相关 claim
- [[cl-2026-05-21-001]] — Wiki 是一个 compounding artifact
```

> **注意**：entity 的 `bio` 是"单条事实的集合"——这些事实**不上升为 claim**（claim-extraction §4
> Case 8）。如果某个关于 entity 的断言有"论点价值"、会被独立引用，那它应该是独立 claim。

---

## 3. Concept

抽象概念、理论、方法。

### Frontmatter schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `definition` | string | ✅ | 一句话定义 |
| `related` | string[] | ⬜ | `[[id]]` 列表 |
| `key_claims` | string[] | ⬜ | 支撑此概念的核心 claim id |

### 模板

```markdown
---
id: co-2026-05-21-001
type: concept
title: "Compounding Artifact"
schema_version: "1.0"
definition: "随使用持续累积、演化、增值的知识工件，与每次重置的 ephemeral cache 相对。"
aliases: ["compounding artifact", "复合工件"]
related: ["[[llm-wiki]]", "[[source-of-truth]]"]
key_claims: ["[[cl-2026-05-21-001]]"]
created_by: "claude-code @ 2026-05-21T10:16:00Z"
updated_by: "claude-code @ 2026-05-21T10:16:00Z"
created_at: 2026-05-21T10:16:00Z
updated_at: 2026-05-21T10:16:00Z
---

# Compounding Artifact

随使用持续累积、演化、增值的知识工件。

## 核心 claim
- [[cl-2026-05-21-001]] — Wiki 是一个 compounding artifact
```

---

## 4. Source

每个 raw 资料对应一个 source page（资料的摘要 + 元信息 + 它产出的 claim 索引）。

### Frontmatter schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `raw_id` | string | ✅ | 对应的 raw 文件路径 |
| `raw_sha256` | string | ✅ | raw 文件的 sha256 |
| `source_kind` | enum | ✅ | `paper` / `article` / `blog` / `transcript` / `book` / `thread` / `other` |
| `ingested_at` | datetime | ✅ | ingest 时间 |
| `claims_extracted` | string[] | ⬜ | 此 source 产出的 claim id 列表 |
| `original_url` | string | ⬜ | 原始来源 URL |

### 模板

```markdown
---
id: sr-2026-05-21-001
type: source
title: "Karpathy — LLM Wiki gist"
schema_version: "1.0"
raw_id: raw/inbox/karpathy-llm-wiki.md
raw_sha256: 7f3a91e4d8c2...
source_kind: article
original_url: "https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f"
ingested_at: 2026-05-21T10:08:00Z
claims_extracted: ["[[cl-2026-05-21-001]]", "[[cl-2026-05-21-002]]"]
created_by: "claude-code @ 2026-05-21T10:14:00Z"
updated_by: "claude-code @ 2026-05-21T10:14:00Z"
created_at: 2026-05-21T10:14:00Z
updated_at: 2026-05-21T10:14:00Z
---

# Karpathy — LLM Wiki gist

## 摘要
Karpathy 提出 LLM Wiki 模式：用 agent 持续维护结构化 Markdown 知识库，
替代传统 RAG 的"查询-遗忘"循环。

## 产出的 claim
- [[cl-2026-05-21-001]] — Wiki 是一个 compounding artifact
- [[cl-2026-05-21-002]] — index.md 必须先于正文被读取
```

---

## 5. Topic

主题页——组织 / 视角 / 对比，由 Query Sedimentation 或 user 创建。详见 `docs/query-sedimentation.md`。

### Frontmatter schema

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `synthesizes` | string[] | ✅ | 此 topic 综合的 page id 列表 |
| `origin_query` | string | ⬜ | 若由 query sediment 产生，记录原 query |
| `provenance_depth` | integer | ⬜ | 可 > 1（topic 是综合层，允许引用 claim） |

### 模板

```markdown
---
id: tp-2026-05-21-001
type: topic
title: "RAG 与 LLM Wiki 的区别"
schema_version: "1.0"
synthesizes: ["[[cl-2026-05-21-001]]", "[[cl-2026-05-21-008]]", "[[co-2026-05-21-003]]"]
origin_query: "RAG 和 LLM Wiki 的本质区别是什么?"
created_by: "query-sedimentation @ 2026-05-21T14:30:00Z"
updated_by: "you @ 2026-05-21T14:35:00Z"
created_at: 2026-05-21T14:30:00Z
updated_at: 2026-05-21T14:35:00Z
---

# RAG 与 LLM Wiki 的区别

> 本主题综合 wiki 中已有 claim，不引入新断言。

| 维度 | RAG | LLM Wiki |
|---|---|---|
| 状态性 | stateless（[[cl-2026-05-21-008]]） | stateful |
| 价值演化 | 不增值 | compounding（[[co-2026-05-21-003]]） |
```

> **Topic 的约束**：topic **不引入新事实断言**。它只重组已有 claim + 写视角性导读。
> 任何新事实必须走独立 claim 流程。

---

## 6. Lint 对 schema 的校验

Daemon 在 propose 时校验：

| 检查 | 违反 |
|---|---|
| 必填字段齐全 | `SCHEMA_VIOLATION` |
| `id` 格式匹配类型 | `SCHEMA_VIOLATION` |
| 文件名 ASCII kebab | `SCHEMA_VIOLATION` |
| claim 有 ≥ 1 source（非 speculation） | 拒绝 propose |
| claim `provenance_depth` = 1 | `PROVENANCE_DEPTH_EXCEEDED` |
| `quote_hash` 可校验 | `QUOTE_HASH_MISMATCH` |
| outbound `[[link]]` 可解析 | lint `broken_link`（warn，不阻断） |

---

## 7. Schema 演化

- 加字段（必须有 default）→ minor 升级（1.0 → 1.1）
- 改必填 / 删字段 → major 升级（1.0 → 2.0），需 migration 脚本
- 每个 page 的 `schema_version` 记录创建时版本
- 升级见 `docs/failure-playbook.md §2.9`

---

## 一句话

> 五类 page：claim（可验证事实）、entity（具体实体）、concept（抽象概念）、source（资料摘要）、
> topic（综合视角）。claim 必须挂 raw、provenance_depth=1；topic 可综合 claim 但不引入新断言。
