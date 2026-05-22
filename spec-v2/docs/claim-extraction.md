# Claim 抽取算法

> **三方共同盲点 P0 #1**：原三方案都说"agent 抽 claim"，但 claim 粒度的可执行判定缺失。
> 没标准 → 不同 agent / 不同时间产出粒度不一致 → wiki 长期分裂。

本文档给出 claim 粒度的**可执行算法** + **10 个 should/should-not 案例** + **agent prompt 模板**。

---

## 1. 为什么这是 P0

如果 claim 粒度未标准化，会同时出现以下三种问题：

1. **过粗** — agent A 把一篇 5000 字论文整合成 5 条 claim，每条都是段落级综合判断，几乎不可校验。
2. **过细** — agent B 把同一篇拆成 80 条 claim，每个数字、每个定义都成 claim，wiki 被 review backlog 淹没。
3. **不一致** — 同一资料、不同 agent / 不同时间抽出来的 claim 集合差异巨大，相互覆盖、矛盾、漏抽。

这不是"agent 太笨"，是**抽取规则没定义**。本文档把规则固化为 agent 必须遵守的算法。

---

## 2. Claim 的定义

> **Claim = 单个可被独立验证、独立反驳、独立引用的事实陈述。**

三个限定：
- **单个**：不是段落综合，也不是关键词。粒度介于"一句话"和"一组紧密相关的句子"之间。
- **可被独立验证**：能在 raw/ 中找到具体段落（≥ 1 个 source + anchor + quote）支持。
- **可被独立引用**：其它 page 用 `[[id]]` 引用时，能不依赖上下文就理解它在说什么。

---

## 3. 抽取算法（四步）

### Step 1：扫描 raw 文档，标记"候选断言句"

对文档每一段，找出符合以下**任一**特征的句子（按优先级排序）：

| 优先级 | 特征 | 例子 |
|---|---|---|
| **P0 必抽** | 含具体数字 / 日期 / 量化指标 | "RAG 召回率约 70%"、"2026 年 4 月提出" |
| **P0 必抽** | 含人名 / 机构名 / 产品名的事实陈述 | "Karpathy 在 X 提出 LLM Wiki" |
| **P0 必抽** | 含明确因果关系 | "因为 wiki 是 stateful，所以知识可积累" |
| **P0 必抽** | 含规范性约束（"必须"、"禁止"、"不能"） | "index.md 必须先于正文被读取" |
| **P1 应抽** | 含定义 / 分类 | "Compounding artifact 是指…" |
| **P1 应抽** | 含可测试断言（能用实验验证） | "FTS5 在 10k 文件下查询 < 100ms" |
| **P2 可抽** | 含明确对比 | "wiki 与 cache 的区别是…" |
| **P3 不抽** | 纯描述性语言、过渡句、修辞 | "这是一种新方式" |
| **P3 不抽** | 不可证伪的主观感受 | "我觉得这个想法很有趣" |
| **P3 不抽** | 无具体内容的总结句 | "总之，wiki 模式有很多优点" |

### Step 2：合并紧密相关的断言

如果多个 P0/P1 断言**满足全部**以下条件，合并为一个 claim：
- 来自**同一段落**（≤ 1 个段落距离）
- 围绕**同一主语**或**同一概念**
- 拆开后任一断言**无法独立成立**（缺另一个就有歧义）

例：原文"index.md 是入口，所有 agent 必须先读 index.md。"——两句围绕同一约束，合并为一条 claim。

**反之**，如果两个断言主语不同或概念不同，**必须拆开**（哪怕在同一段）。

### Step 3：为每个 claim 写出三件套

```yaml
- 标题（< 12 字，可索引）
- 正文（1-2 段，可独立理解）
- sources: [{raw_id, anchor, quote (< 30 词), quote_hash, span: [line_start, line_end]}]
- confidence: 由证据数 + 引用强度决定（见 §5）
- status: 默认 unverified
```

### Step 4：自检 4 道判定

每个 claim **必须**通过下面 4 道是非题，否则拒绝 propose：

1. **独立性**：把它从 wiki 完全移除，其它 page 是否还能正确理解？
2. **可验证性**：30 秒内能否回到 raw 原文段落？
3. **粒度稳定性**：同一 claim 让 agent B 重新抽，是否会抽出"几乎相同"的内容？
4. **反驳可能性**：你能想象出"什么样的新证据会让它从 supported 变 disputed"吗？

任一为否 → 不要 propose，要么放弃要么调整。

---

## 4. 10 个案例

### Should（抽，且这样抽是对的）

#### Case 1：定量陈述

> 原文：「Karpathy 在 2026 年 4 月发布 LLM Wiki gist，提出三层架构：raw / wiki / schema。」

✅ 抽 2 个 claim：

```yaml
claim 1:
  title: Karpathy 于 2026 年 4 月发布 LLM Wiki gist
  body: |
    Karpathy 在 2026 年 4 月公开发布 LLM Wiki gist，
    首次系统地提出 agent 维护持续知识库的模式。
  sources: [{anchor: "#para-1", quote: "in April 2026, Karpathy published…", ...}]
  confidence: 0.95
```

```yaml
claim 2:
  title: LLM Wiki 三层架构
  body: |
    LLM Wiki 的核心架构由三层组成：
    raw（只读原始资料）/ wiki（agent 维护）/ schema（agent 指令）。
  sources: [{anchor: "#para-1", quote: "raw, wiki, schema layers", ...}]
  confidence: 0.95
```

**为什么拆**：发布时间 vs 架构是不同主语，独立成立。

#### Case 2：规范性约束

> 原文：「index.md 必须先于正文被读取，agent 不得跳过 index 直接读 page。」

✅ 抽 1 个 claim（合并）：

```yaml
claim:
  title: agent 必须先读 index.md
  body: |
    在 Karpathy 的 LLM Wiki 模式中，index.md 是 wiki 的目录。
    任何 agent 在读取正文 page 之前，必须先读 index.md，
    不得跳过此步骤直接 grep / cat 任意 page。
  sources: [{anchor: "#para-3", quote: "index.md must be read first…", ...}]
  confidence: 0.90
  status: supported
```

**为什么合并**："必须先读"和"不得跳过"是同一约束的正反两面，独立成立会语义重复。

#### Case 3：因果陈述

> 原文：「因为 RAG 是 stateless 的，所以知识无法在多次查询之间积累。」

✅ 抽 1 个 claim：

```yaml
claim:
  title: RAG stateless 导致知识无法积累
  body: |
    传统 RAG 架构在每次查询时独立检索 chunk，
    查询完成后不更新任何持久化的知识结构。
    这种 stateless 特性使得多次查询之间无法形成知识沉淀。
  sources: [{anchor: "#sec-2", quote: "RAG is stateless …", ...}]
  confidence: 0.85
```

**为什么不拆**：因和果合在一起才是完整的可验证断言，拆开任一半都无法独立校验。

#### Case 4：对比陈述

> 原文：「Wiki 是 compounding artifact，cache 是 ephemeral；前者越用越值钱，后者每次重置。」

✅ 抽 1 个 claim（对比作为一个完整论点）：

```yaml
claim:
  title: Wiki 与 cache 的本质区别
  body: |
    Wiki 是 compounding artifact——内容随使用累积、演化、增值；
    cache 是 ephemeral——每次访问后内容重置或失效。
    本质区别在于"是否产生持续的状态"。
  sources: [{anchor: "#sec-3", quote: "wiki compounds, cache resets", ...}]
  confidence: 0.90
```

#### Case 5：引用他方观点

> 原文：「Karpathy 引用 Andy Matuschak 的 evergreen notes 概念，认为 wiki 应模仿 evergreen notes
> 的演化方式。」

✅ 抽 1 个 claim（明确归属）：

```yaml
claim:
  title: Karpathy 主张 wiki 应模仿 evergreen notes
  body: |
    Karpathy 在 LLM Wiki gist 中引用 Andy Matuschak 的 evergreen notes 概念，
    主张 wiki page 的演化方式应类似 evergreen notes——
    随理解加深逐步重写而非一次性归档。
  sources:
    - anchor: "#para-7"
      quote: "Karpathy cites Matuschak's evergreen notes …"
  confidence: 0.85
  status: supported
```

**为什么抽**：明确标注观点归属（Karpathy 引用 Matuschak），是可验证的事实而非主观推断。

### Should NOT（不该抽，或这样抽是错的）

#### Case 6：纯修辞 / 过渡句

> 原文：「这就是 LLM Wiki 模式的精彩之处。」

❌ **不抽**。理由：无具体内容、不可证伪、纯修辞。

#### Case 7：综合推断（无直接出处）

> 原文（agent 想抽的）：「LLM Wiki 模式将彻底取代 RAG。」

❌ **不抽**（除非原文有明确陈述）。理由：

- 原文可能只说"LLM Wiki 是 RAG 的补充"——agent 加戏脑补成"取代"
- "彻底"、"将"都是 agent 自己加的强限定词
- 没有 raw 中的具体 quote 可挂

如果坚持抽，**必须**标 `status: speculation=true, confidence: < 0.3`，且 user review 时大概率 reject。

#### Case 8：粒度过细（拆得稀碎）

> 原文：「Karpathy 是 OpenAI 联合创始人，目前为独立研究者，曾任 Tesla 自动驾驶总监。」

❌ **不应抽 3 个独立 claim**（每个职位一条）。

✅ 应抽 1 个 entity page（`entities/karpathy.md`），把这些事实作为 entity 的 `bio` 字段，不上升到 claim 级别。

**为什么**：单条事实没有"论点价值"，不会被其它 page 单独引用。entity 页天然就是这些事实的容器。

#### Case 9：粒度过粗（一个 claim 包了多个论点）

> 原文：「RAG 与 LLM Wiki 的区别在于：stateless vs stateful，retrieval-time vs ingest-time，
> chunk-based vs page-based。」

❌ **不应抽成一个大 claim**："RAG 与 LLM Wiki 的全部区别"。

✅ 应抽 3 个独立 claim：

- `stateless vs stateful`
- `retrieval-time vs ingest-time`
- `chunk-based vs page-based`

**为什么**：三个对比维度独立成立，未来可能各自被反驳 / 修正 / 引用。打成一个 claim 后无法精准 reverify。

#### Case 10：跨 page 引用强化（违反 provenance_depth ≤ 1）

> 场景：agent 想抽一个 claim："LLM Wiki 比 RAG 更有效率"——它的 source 不指向 raw/，而指向另一个
> wiki claim `claims/wiki-vs-rag-efficiency.md`。

❌ **拒绝**。任何 claim 的 source 必须指向 `raw/`，**不允许**指向另一个 wiki page。

**为什么**：这是反"羊群效应"的硬规则。如果允许 claim 引用 claim，错误会通过引用链固化为"共识"。

---

## 5. Confidence 的赋值规则

| confidence 区间 | 含义 | 触发条件 |
|---|---|---|
| **0.9 – 1.0** | 直接引用 + 原文明确陈述 | quote 直接覆盖 claim 全文 90%+ |
| **0.7 – 0.9** | 强证据 + 一处轻度推断 | 多 source 互相印证；或单 source 但语境无歧义 |
| **0.5 – 0.7** | 中度推断 | 原文未直接陈述，agent 综合 ≥ 2 处文字推出 |
| **0.3 – 0.5** | 弱推断 | 单 source 间接支持；需 user 复核 |
| **< 0.3** | 必须标 `speculation=true` | 几乎纯推测；建议拒绝 propose |

**Lint 规则**：claim 正文若使用"必然"、"一定"、"绝对"、"绝不"等强限定词，confidence 必须 ≥ 0.9，否则 lint 警告"语气与 confidence 不一致"。

---

## 6. Agent Prompt 模板（给 agent 用）

下面是 wired 到 `schema/CLAUDE.md` / `CODEX.md` 等 agent 指令文件的 claim 抽取部分。可直接复制粘贴。

```markdown
## Claim 抽取规则（必须遵守）

当你 ingest 一个 raw 文件时，按以下算法抽取 claim：

### Step 1：扫描候选断言句
找出符合以下任一特征的句子：
- 含具体数字 / 日期 / 量化指标
- 含人名 / 机构名 / 产品名的事实陈述
- 含明确因果关系（"因为…所以…"）
- 含规范性约束（"必须"、"禁止"、"不能"）
- 含定义 / 分类
- 含可测试断言

**不要**抽以下内容为 claim：
- 纯修辞 / 过渡句 / 总结句
- 不可证伪的主观感受
- 没有 raw 中明确 quote 的推断
- 单条事实（属于 entity 的 bio，不是 claim）

### Step 2：合并紧密相关的断言
若多个断言来自同一段落、围绕同一主语、拆开就有歧义 → 合并为一条 claim。
反之，不同主语 / 不同概念 → 必须拆开。

### Step 3：写三件套
每个 claim 包含：title（< 12 字）+ body（1-2 段可独立理解）+
sources（≥ 1 个 raw_id + anchor + quote ≤ 30 词 + quote_hash + span）+
confidence（按规则赋值）+ status（默认 unverified）。

### Step 4：自检 4 道
不通过 4 道是非题的，**不要 propose**：
1. 独立性：移除它后，其它 page 是否还能正确理解？
2. 可验证性：30 秒内能否回到 raw 原文段落？
3. 粒度稳定性：另一个 agent 重抽是否会得出"几乎相同"的结果？
4. 反驳可能性：你能想象出"什么新证据会让它变 disputed"吗？

### 反"羊群效应"约束（硬规则）
- Claim 的 source 必须指向 `raw/`，**不允许**指向另一个 wiki page。
- `provenance_depth` 必须 ≤ 1。

### Confidence 规则
- 直接 quote 覆盖 90%+ → 0.9-1.0
- 多 source 互相印证 → 0.7-0.9
- 单 source + 轻度推断 → 0.5-0.7
- 弱推断 → 0.3-0.5（user 必须复核）
- < 0.3 → 标 speculation=true，建议放弃

正文若用"必然"、"一定"、"绝对"、"绝不"等强限定词，confidence 必须 ≥ 0.9。
```

---

## 7. 与 Lint 的配合

全部 lint 规则在 [`templates/lint-rules.md`](../templates/lint-rules.md) 单一定义。
其中与 claim 抽取直接相关的 **claim 专项规则**（`claim_` 前缀）有 4 条：

| Lint Rule | 检测什么 | 触发动作 |
|---|---|---|
| `claim_no_source` | 无 source 但 status != speculation | 拒绝 / 标 needs_source |
| `claim_drift` | quote_hash mismatch | 标 needs_reverify + 在 review 中显示 |
| `claim_provenance_depth_gt_1` | source 指向 wiki/ 而非 raw/ | 拒绝 |
| `claim_confidence_qualifier_mismatch` | 用了"必然"等强词但 conf < 0.9 | 警告 |

此外，通用规则 `orphan`（30 天内无被引用且无 backref → 标 candidate_for_removal）同样适用于
claim——它不带 `claim_` 前缀，因为对所有 page 类型一视同仁。规则组别名 `claim_quality` 一次性
跑全部 4 条 claim 专项规则。

---

## 8. 边界情况

### 8.1 多语言资料

中英文混合原文：claim 正文优先用**原文语言**（避免翻译失真）；title 双语（中文为主，括号注英文术语）。
quote 保持原文。

### 8.2 引用 / 转述

如果 raw 是"二手转述"（A 转述 B 的观点），claim 必须标明转述链：

```yaml
title: Karpathy 转述 Matuschak 的 evergreen notes 观点
provenance_chain:
  - {who: "Matuschak (原作者)", source: "evergreen-notes.com"}
  - {who: "Karpathy (转述者)", source: "raw/karpathy-llm-wiki.md#para-7"}
```

直接 source 仍然只挂 `raw/karpathy-llm-wiki.md`（depth=1），但 chain 暴露给 user 决策。

### 8.3 矛盾 claim

抽到的新 claim 与已有 claim 矛盾时（用 entity / concept 关系图判定）：
- **不要**自动覆盖旧 claim
- 新 claim 标 `status: disputed`，body 中写明"与 [[old-claim-id]] 矛盾"
- 写入 review queue，user 决议（accept new + refute old / accept both 标对立观点 / reject new）

### 8.4 超长资料

单文件 > 20k tokens 的资料：

- 不强求一次抽完——分批抽，每批 ≤ 10 个 claim
- 多批可以归在同一 bundle，user 一起 review
- 大文件优先抽 entity / concept，claim 按章节分批

---

## 9. 验收（如何确认 agent 真的遵守了算法）

每周（Dream Cycle 一部分）跑：

```bash
wikimind lint --rule claim_quality --since 7d --report
```

该报告含：
- 新增 claim 数 / 平均 confidence / quote 平均长度
- 违反 4 道自检的 claim 数（理论应为 0）
- 同一 raw 文件被不同 agent 抽出的 claim 集合差异（粒度稳定性指标）
- top 5 "可能过细" / "可能过粗" 的 claim 列表（人工抽查样本）

如果连续 2 周稳定性 < 0.7，触发 agent prompt 微调流程。

---

## 10. 不在范围

- **多步推理 claim**（如"通过 A、B、C 三步推出 D"）—— Wave 2 之后讨论，需要 graph 表达
- **跨语言对齐**（中英同义判定）—— Dream Cycle 的 consolidate 阶段处理，非抽取阶段
- **自动学习 prompt**（让 agent 从 user reject 中学习）—— rejection memory 是 v0.2 功能

---

## 一句话总结

> Claim 粒度不是哲学，是工程。把"什么算 claim、什么不算"固化为可执行的 4 步算法 + 自检 4 道，
> 才能在多 agent 协作时保持 wiki 长期一致。
