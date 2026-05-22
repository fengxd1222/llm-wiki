# 示例：`wikimind demo` 完整走查

> 搭配 [`docs/onboarding.md`](../docs/onboarding.md)。本文件给出 demo 的**确定性细节**——
> 3 个 sample raw 文件、demo agent 的预制输出、产出的 11 条 propose。
>
> demo 是**确定性**的：同样的输入永远产出同样的结果，用于教学与回归测试。

---

## 1. 三个 Sample Raw 文件

### 1.1 `karpathy-llm-wiki.md`（28 KB）

WikiMind 自我引用——产品本身的来源文档。结构：

```
# LLM Wiki
## Philosophy            ← claim 来源：compounding artifact, index.md read first
## Three Layers          ← claim 来源：raw/wiki/schema 三层
## Claims as Citizens    ← claim 来源：claim 一等公民
## Workflows             ← claim 来源：ingest/query/lint
```

### 1.2 `mindstudio-blog.html`（142 KB）

二手转述——一篇介绍 LLM Wiki 模式的博客。**故意**比 gist 多一些转述细节，
用于演示"二手 source 的 confidence 应该更低"+"DRIFT 场景"。

### 1.3 `rag-vs-wiki-thread.md`（8 KB）

Twitter thread 整理——提供"RAG 主流观点 vs LLM Wiki 新模式"的对比素材。

---

## 2. Demo Agent（`claude-stub`）

### 2.1 为什么是 stub

`wikimind demo` 不调任何真实 LLM API——它用一个**确定性 stub agent**：

- 完全离线，无需 API key
- 同一 sample 文件 → 同一组 claim（hash 固定）
- 严格按 `docs/claim-extraction.md` 的 4 步算法执行
- 用预制 prompt + 预制响应（下表）

### 2.2 这不是"假功能"

stub 是产品功能的**确定性快照**——它走的是真实的 ingest pipeline、真实的 propose 流程、
真实的 review queue。只有"LLM 推理"那一步被替换为预制响应。

这让 demo：可复现、可教学、可作回归测试基线。

---

## 3. Demo 产出的 11 条 Propose

ingest 三个文件，stub agent 产出 bundle `b-0001`，含 11 条 propose：

| review_id | type | path | confidence | 来源 |
|---|---|---|---|---|
| r-0001 | claim | claims/wiki-is-compounding.md | 0.92 | karpathy-gist #philosophy |
| r-0002 | claim | claims/index-md-read-first.md | 0.85 | karpathy-gist #philosophy |
| r-0003 | claim | claims/claims-are-citizens.md | 0.90 | karpathy-gist #claims |
| r-0004 | claim | claims/raw-is-immutable.md | 0.95 | karpathy-gist #three-layers |
| r-0005 | claim | claims/wiki-vs-rag-stateful.md | 0.88 | rag-vs-wiki-thread |
| r-0006 | claim | claims/query-sediments-back.md | 0.80 | mindstudio-blog #para-7 |
| r-0007 | entity | entities/karpathy.md | — | 三个文件 |
| r-0008 | entity | entities/llm-wiki.md | — | 三个文件 |
| r-0009 | entity | entities/rag.md | — | rag-vs-wiki-thread |
| r-0010 | concept | concepts/compounding-artifact.md | 0.90 | karpathy-gist + blog |
| r-0011 | concept | concepts/source-of-truth.md | 0.92 | karpathy-gist #three-layers |

### 3.1 故意的"教学点"

| review | 教学意图 |
|---|---|
| r-0002（conf 0.85） | 演示"推断 vs 直接引用"——比 r-0001 的 0.92 略低 |
| r-0006（conf 0.80，来自 blog） | 演示"二手 source confidence 更低" |
| r-0006 的 source | mindstudio-blog 的 quote 故意与 gist 略有差异 → demo 第二次跑会显示 DRIFT |
| r-0007 karpathy entity | 三个文件都提到 → 演示 entity 的多 source 聚合 |

---

## 4. 预制 Prompt / 响应样例

stub agent 的 claim 抽取，以 r-0001 为例：

### 输入（pipeline stage 3 给 stub 的）

```
PURPOSE: 从 raw/inbox/karpathy-llm-wiki.md 抽取 claim
TASK: 按 claim-extraction.md 4 步算法
CONTEXT: @raw/inbox/karpathy-llm-wiki.md (normalized)
```

### stub 的预制响应（r-0001）

```yaml
# Step 1 扫描：#philosophy 段落含规范性 + 因果陈述
# Step 2 合并：两句围绕"compounding"，合并为一条
# Step 3 三件套：
title: "Wiki 是一个 compounding artifact"
body: |
  Karpathy 在 LLM Wiki gist 中明确主张：wiki 是一个 compounding artifact，
  而不是临时缓存。每一次 ingest、query、lint 都应让 wiki 更值钱。
sources:
  - raw_id: raw/inbox/karpathy-llm-wiki.md
    anchor: "#section-1-philosophy"
    quote: "every ingest, every query, every lint should make the wiki more valuable"
    quote_hash: a7f2e3c1
    span: [14, 19]
confidence: 0.92
status: unverified
# Step 4 自检：
#   独立性 ✓  可验证性 ✓  粒度稳定性 ✓  反驳可能性 ✓
#   provenance_depth = 1 ✓
```

完整 11 条的预制响应存于 demo 二进制内嵌资源 `demo/stub-responses.yaml`。

---

## 5. Demo 时间线（确定性）

| 时刻 | 事件 | 累计用时 |
|---|---|---|
| 0:00 | `wikimind demo` 启动，显示介绍 | — |
| 0:05 | 创建 demo-vault + 3 sample raw | 5s |
| 0:08 | 启动 stub agent + handshake | 8s |
| 0:20 | ingest karpathy-gist（5 阶段） | 20s |
| 0:35 | ingest mindstudio-blog | 35s |
| 0:42 | ingest rag-thread，bundle b-0001 完成 | 42s |
| —— | 关键时刻 #1：11 条 propose 进 _review/，不直接进 wiki | —— |
| 1:30 | user 看 `review show b-0001` | 思考时间 |
| 2:00 | user `review accept b-0001` | —— |
| 2:05 | daemon apply 11 patches + commit a92d445 | —— |
| —— | 关键时刻 #2：raw → claim → 你的确认 → git commit 一次循环 | —— |
| 3:00 | user `query "wiki vs RAG?"` | —— |
| 3:20 | 答案返回，带 3 条 citation | —— |
| —— | 关键时刻 #3：每条 answer 30 秒回到 raw 原文 | —— |
| 4:00 | 打开 wiki/index.md 浏览 | —— |
| 5:00 | 总结 + next step | 5min |

---

## 6. Demo 的回归测试

CI 把 demo 当 e2e 测试跑：

```bash
wikimind demo --auto --keep --vault /tmp/ci-demo
# 断言：
#   - demo-vault/wiki/claims/ 有 6 个文件
#   - demo-vault/wiki/entities/ 有 3 个文件
#   - demo-vault/wiki/concepts/ 有 2 个文件
#   - change-log.jsonl 有 1 条 accept 记录
#   - query "wiki vs RAG" 返回 ≥ 3 citations
#   - 全程 < 5 分钟（--auto 模式无 user 思考时间，应 < 60s）
#   - 无 console error
```

任何对 ingest pipeline / claim 抽取 / review 流程的改动，都会让这个测试反映出来。

---

## 7. Demo 第二次跑：DRIFT 教学

如果 user 在 demo 后**手动修改** `demo-vault/raw/imported/mindstudio-blog.html`，
再跑 `wikimind status`：

```
$ wikimind status
...
⚠ 1 DRIFT claim detected:
  claims/query-sediments-back.md
  → source raw/inbox/mindstudio-blog.html#para-7 modified
  → quote_hash mismatch: stored d4f9... vs current e1b8...
  → run: wikimind reverify claims/query-sediments-back.md
```

这是 demo 的"隐藏第二课"——演示 DRIFT 检测机制真的在工作。

---

## 8. 与 onboarding.md 的关系

| 文档 | 角色 |
|---|---|
| `docs/onboarding.md` | demo 的**设计规格**——为什么这么设计、UX、成功度量 |
| 本文件 | demo 的**确定性细节**——sample 数据、stub 响应、产出清单、回归测试 |

实现 `wikimind demo` 时两份都要遵守。

---

## 一句话

> demo 是确定性的：3 个 sample raw → stub agent → 11 条 propose → 1 个 bundle → 1 次 commit。
> 同样输入永远同样输出，可教学、可复现、可作 e2e 回归基线。
