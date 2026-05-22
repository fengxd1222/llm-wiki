# Query Sedimentation：问答回写

> **方案 B 独家贡献**：把"一次性的高质量问答"沉淀回 wiki，让 query 本身成为 wiki 增值的一种方式。
> 这是 SPEC §1 哲学第 1 条"不能让高质量对话答案消失在 chat history 里"的具体机制。

---

## 1. 概念

### 1.1 问题

传统用法里，query 是"消费"——你问 wiki 一个问题，得到答案，答案用完即弃。

但很多时候，一次高质量 query 实际上**产生了新知识**：

- 你问"RAG 和 LLM Wiki 的区别"，agent 综合了 wiki 里 5 个 claim，给出一个结构化对比——
  **这个对比本身**是有价值的、值得保留的、未来会被反复需要的。
- 下次别人（或你自己、或另一个 agent）问同样问题，又要重新综合一遍——浪费。

**Query Sedimentation = 把"高质量 query 的答案"回写成 wiki page，让 wiki 越查越值钱。**

"sediment"（沉淀）：query 像水流过，把有价值的东西沉淀下来，留在 wiki 里。

### 1.2 为什么这是 "compounding" 的关键

SPEC §1 第 1 条："每一次 ingest、每一次 query、每一次 lint 都让 wiki 更值钱。"

- Ingest 让 wiki 增值——显然（新资料进来）
- Lint / Dream Cycle 让 wiki 增值——通过整理
- **Query 让 wiki 增值——靠的就是 Sedimentation**

没有 Sedimentation，query 就是纯消费，wiki 不会因为"被查"而变好。
有了 Sedimentation，"查得越多" → "沉淀越多" → "wiki 越值钱" → 形成正循环。

---

## 2. 沉淀什么

不是所有 query 都该沉淀。沉淀的是**新产生的、有保留价值的结构**。

### 2.1 三类沉淀产物

| 产物类型 | 何时产生 | 例子 |
|---|---|---|
| **新 topic page** | query 的答案是对已有 claim 的**结构化综合** | "RAG vs LLM Wiki" → 新建 `topics/rag-vs-llm-wiki.md`，组织已有 5 个 claim |
| **新 claim** | query 过程中 agent 从 raw 里发现了**之前没抽出的断言** | 查询时 agent 翻 raw 发现一个未被抽过的事实 → propose 新 claim |
| **关系补全** | query 发现两个 page **该连未连** | 答案用到了 claim A 和 entity B，但它们之间没有 link → propose 加 link |

### 2.2 不沉淀什么

- ❌ 纯检索类 query（"karpathy 的 entity page 在哪"）——没有新结构
- ❌ 答案完全等于某个已有 claim 的 query——已经在 wiki 里了
- ❌ 低质量 / 失败的 query（agent 没找到好答案）
- ❌ 答案含大量 agent 推测（confidence 低）的 query
- ❌ 用户明确标 `--no-sediment` 的 query

---

## 3. 质量判定（什么 query 值得沉淀）

Query 结束后，daemon 用一个**沉淀评分**决定是否触发：

```
sediment_score = (
    answer_uses_multiple_claims  * 30  +   # 答案综合了 ≥ 2 个已有 claim
    answer_has_new_structure     * 30  +   # 答案产生了新的组织（对比/分类/时间线）
    all_sources_verified         * 20  +   # 答案引用的所有 source 都 verified（非 drift）
    answer_confidence_avg        * 20      # 答案各部分平均 confidence
)
# 阈值：score >= 60 才触发沉淀
```

| 信号 | 怎么判定 |
|---|---|
| `answer_uses_multiple_claims` | 答案的 citation 指向 ≥ 2 个不同 claim |
| `answer_has_new_structure` | agent 在答案里产生了表格 / 分类 / 对比 / 时间线（结构化输出） |
| `all_sources_verified` | 答案引用的所有 claim 的 source quote_hash 都未 drift |
| `answer_confidence_avg` | 答案各论据的 confidence 均值 |

**只有高分 query 才沉淀**——这是防止"问答垃圾"淹没 wiki 的第一道闸。

---

## 4. 沉淀机制

```
[User / agent: query "RAG 与 LLM Wiki 的区别"]
        ↓
[Read Service 检索 + agent 综合答案]
        ↓
[答案返回给 user]
        ↓
[Sediment 评分]  ── score < 60 ──→  不沉淀，结束
        │
     score >= 60
        ↓
[Sediment Service]
        ↓
1. 判定产物类型（topic / claim / 关系补全）
2. agent 在自己 worktree 生成草稿：
   - 若 topic：新建 topics/rag-vs-llm-wiki.md
     · 内容是答案的结构化版本
     · 每个论点都 [[link]] 到它综合的源 claim
     · 不引入新的无源断言
   - 若 new claim：走标准 claim-extraction 算法
   - 若关系补全：propose_edit 加 link
3. propose_* → review queue
4. request_review(kind="query_sediment", priority="low")
        ↓
[Review queue]  ── user accept ──→  沉淀成功，wiki 增值
                └─ user reject ──→  记入 rejection memory
```

### 4.1 Topic page 的特殊性

最常见的沉淀产物是 **topic page**。它和 claim / entity / concept 不同：

| 维度 | claim / entity / concept | topic |
|---|---|---|
| 来源 | 直接挂 raw | **综合已有 wiki page** |
| provenance_depth | 必须 = 1 | 可 > 1（它本就是综合层） |
| 内容 | 原子知识 | 组织 / 视角 / 对比 |
| 可被 claim 引用 | ✅ | ❌（topic 不是事实来源） |

**关键约束**：topic page **不引入新的事实断言**。它只能：
- 重新组织已有 claim（对比、分类、时间线）
- `[[link]]` 到被它综合的 claim / entity / concept
- 写"视角性"的导读文字（"本主题对比 X 和 Y 两个方案……"）

如果 query 答案里有"新事实"——那个事实必须走**独立的 new claim** 流程（挂 raw），不能塞进 topic。
这保证 topic 是"导航层"，事实永远在 claim 层、永远可追溯。

### 4.2 沉淀的 topic page 样例

```markdown
---
id: tp-2026-05-21-001
type: topic
title: "RAG 与 LLM Wiki 的区别"
created_by: query-sedimentation @ 2026-05-21 14:30
origin_query: "RAG 和 LLM Wiki 的本质区别是什么?"
synthesizes: [[cl-...-001]] [[cl-...-008]] [[cl-...-012]] [[co-...-003]]
schema_version: 1.0
---

# RAG 与 LLM Wiki 的区别

> 本主题综合 wiki 中已有的 claim，对比两种知识架构。
> 所有事实性断言来自被引用的 claim 页，本页不引入新断言。

## 三个核心区别

| 维度 | RAG | LLM Wiki |
|---|---|---|
| 状态性 | stateless（见 [[cl-...-001]]） | stateful（见 [[cl-...-008]]） |
| 知识时机 | retrieval-time | ingest-time（见 [[cl-...-012]]） |
| 价值演化 | 不增值 | compounding（见 [[co-...-003]]） |

## 延伸

- 概念：[[compounding-artifact]]
- 相关 claim：[[wiki-is-compounding]]
```

注意每个断言都 `[[link]]` 到源 claim——topic 是"可点击的导航"，不是"新的事实容器"。

---

## 5. 防止问答垃圾

Query Sedimentation 最大的风险：**把一堆平庸问答沉淀成 wiki 垃圾**。多重防御：

| 防御层 | 机制 |
|---|---|
| 1. 评分阈值 | `sediment_score >= 60` 才触发，多数 query 不沉淀 |
| 2. 不引入新断言 | topic 只能综合已有 claim，新事实必须走独立 claim 流程 |
| 3. Review queue | 所有沉淀产物进 review queue，user 拍板 |
| 4. 低优先级 | `priority: low`，不抢 ingest review 的注意力 |
| 5. 去重检查 | 沉淀前检查是否已有相似 topic（避免每次同类 query 都建新 topic） |
| 6. 限流 | 每天最多沉淀 N 个（默认 5），超出当天不再沉淀 |
| 7. Dream Cycle 兜底 | Dream Cycle 的 consolidate 阶段会合并重复 topic |

### 5.1 去重检查细节

沉淀前，daemon 检查：

```
是否已有 topic 的 synthesizes 集合与本次高度重叠（Jaccard > 0.7）？
  是 → 不新建。改为 propose_edit 更新已有 topic（如有新内容）
  否 → 新建 topic
```

这避免"问 10 次 RAG vs Wiki 就有 10 个雷同 topic"。

---

## 6. 用户 UX

### 6.1 查询时

```
$ wikimind query "RAG 和 LLM Wiki 的本质区别?"

[答案输出...]

  ╭───────────────────────────────────────────────╮
  │ 💧 这次 query 产生了可沉淀的内容               │
  │   sediment_score: 78 / 100                     │
  │   建议沉淀为 topic: "RAG 与 LLM Wiki 的区别"   │
  │                                                 │
  │   已提议为 bundle b-0043（review queue，low）  │
  │   wikimind review show b-0043  查看            │
  ╰───────────────────────────────────────────────╯
```

### 6.2 控制

```bash
wikimind query "..." --no-sediment       # 本次不沉淀
wikimind query "..." --sediment-force    # 强制沉淀（即使评分 < 60）

# 全局配置
[query_sedimentation]
enabled = true
auto_propose = true        # true: 自动进 review queue; false: 仅提示 user 可手动沉淀
score_threshold = 60
max_per_day = 5
```

### 6.3 `auto_propose = false` 模式

保守用户可设 `auto_propose = false`——这样 query 高分时只**提示**，不自动产生 propose：

```
  💧 这次 query 可沉淀（score 78）。运行 wikimind sediment last 来沉淀它。
```

User 主动 `wikimind sediment last` 才真正生成 propose。

---

## 7. 与其它机制的关系

| 机制 | 关系 |
|---|---|
| **Claim Extraction** | 沉淀产物若是 new claim，走完整 claim-extraction 算法（4 步 + 自检） |
| **Review Queue** | 所有沉淀进 review queue，kind=`query_sediment`，priority=low |
| **Dream Cycle** | Dream Cycle 的 consolidate 会合并重复 topic；evolve 不与沉淀冲突 |
| **Topic page** | Sedimentation 是 topic page 的主要来源（topic 也可 user 手建） |

---

## 8. 度量

Weekly report 中的 sedimentation 指标：

| 指标 | 健康范围 | 异常含义 |
|---|---|---|
| 本周沉淀触发数 | 视查询量 | — |
| 沉淀 propose accept rate | > 50% | < 30%：评分阈值偏低，调高 |
| 重复 topic 被 Dream Cycle 合并数 | < 2/周 | 偏高：去重检查不灵 |
| 沉淀产物被后续 query 命中率 | > 30% | 低：沉淀的 topic 没价值，反思阈值 |

最后一个指标最重要：**沉淀的 topic 后来真的被查到了吗？** 如果沉淀的东西没人再用，说明沉淀
判定有问题。

---

## 9. 失败处理

| 失败 | 处理 |
|---|---|
| 沉淀 agent 崩溃 | query 答案已返回 user，沉淀失败不影响主流程；worktree 清理 |
| 沉淀产物 review 被 reject | 记 rejection memory；同类 query 短期内不再自动沉淀 |
| review queue 满 | 跳过沉淀（不加剧 backlog），仅在查询结果里提示"可手动沉淀" |
| 去重检查误判（把不同 topic 当重复） | user 可 `wikimind sediment last --force-new` 强制建新 |

---

## 10. 不在范围

- **自动回答历史 query**（缓存问答对）——那是 cache，不是 wiki，违反 SPEC §1
- **跨 vault 沉淀** —— v2+
- **沉淀对话上下文**（多轮对话整体沉淀）—— v0.2 评估
- **ML 评分模型** —— MVP 用规则评分；ML v0.2+

---

## 一句话总结

> Query Sedimentation 让"查询"也成为 wiki 增值的方式：高质量 query（score ≥ 60）的结构化答案
> 沉淀为 topic page，进 review queue 由 user 拍板。topic 只综合已有 claim、不引入新断言——
> 保证事实永远在可追溯的 claim 层。这是 wiki "越查越值钱" 的机制。
