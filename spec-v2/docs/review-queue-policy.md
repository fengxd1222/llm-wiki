# Review Queue 政策：上限保护 + 归并 + Auto-Accept

> **三方共同盲点 P0 #2**：Review queue 是协议的核心闸门，但方案 A 只在 R-18 提到"backlog 高"
> 风险，缓解办法薄弱（"daily summary"+"低风险自动 accept 白名单"）。方案 B/GPT Pro 几乎没提。
>
> 现实：一篇资料 → 20 claim → user review 20 项；10 篇资料 = 200 项 pending。Review queue 失守
> 是这个产品**最容易死的方式**。

本文档定义 review queue 的硬性保护机制。

---

## 1. 为什么这是 P0

Review queue 是 SPEC 的**核心架构选择**——所有 agent 写入必经此闸门，保证可追溯、可审计。
但这个设计有一个隐藏假设：**user 真的会及时 review**。

现实是：

| 场景 | user 行为 | queue 状态 | 结果 |
|---|---|---|---|
| 第 1 天 | 兴致勃勃 review 20 项 | 0 pending | ✓ |
| 第 3 天 | 周末没碰 | 60 pending | 略累但能赶 |
| 第 7 天 | 出差 | 150 pending | 想 review 但量大放弃 |
| 第 14 天 | 干脆不看了 | 400 pending | **queue 退化为摆设** |
| 第 30 天 | user 直接 `accept all` 凑数 | 0 pending（虚假） | **闸门彻底破防** |

闸门破防后，agent 写入 = 直接进 wiki。所有"防幻觉""可追溯"的承诺**同时**失效。

**这不是 user 懒，是协议设计没给 user 留可持续的 review 节奏。**

---

## 2. 政策总览（4 个机制）

| 机制 | 解决什么 | 触发条件 |
|---|---|---|
| **上限保护** | queue 不能无限堆积 | pending > 50 时停接新 propose |
| **Bundle 归并** | review 颗粒度从"条"提到"批" | 同一 ingest / 同一意图的 propose 自动入 bundle |
| **Auto-accept 白名单** | 低风险写入 user 不必看 | 满足 user 定义的规则集 |
| **优先级排序** | user 永远先看最重要的 | 多维评分（详见 §6） |

四个机制**叠加生效**——任一单独都不够。

---

## 3. 上限保护

### 3.1 默认阈值

```yaml
# .wikimind/config.toml
[review_queue]
soft_limit = 30      # 黄色警告：UI 提示 user 该清理
hard_limit = 50      # 红色拒绝：daemon 停接新 propose
critical_limit = 100 # 紧急模式：仅 master agent 可继续 propose
```

### 3.2 触发时的行为

**Pending 数到达 30（soft_limit）**：

- Daemon 在每次 propose 完成后返回 warning：`"queue at 32/50 — please review backlog soon"`
- Sidebar 显示黄色徽章 + tip
- Dream Cycle 触发时优先跑 "summarize review queue"，给 user 一个 4 行摘要

**Pending 数到达 50（hard_limit）**：

- Daemon 拒绝**所有** `propose_*` 调用，返回 `QUEUE_FULL` 错误
- Agent 收到错误后必须：(a) 等待，(b) 切换到只读模式（仍能 query / read），(c) 不能 retry 直到 user
  减少 backlog
- UI 红色 banner：`"Queue 满 (50/50)。Agent 写入被冻结。请 review 当前 backlog。"`
- 唯一例外：`emergency=true` 标记的 propose（如 lint 发现严重 DRIFT，需 user 立即处理）可绕过

**Pending 数到达 100（critical_limit）**：

- 仅 master agent（user 显式授权的"主 agent"，如 Claude Code）能继续 propose
- 其它 worker agent 完全冻结
- Daemon 在 log 中标红：`"CRITICAL: queue overflow, write 99% throttled"`
- 默认推送 OS 通知：`"WikiMind review queue critical — 100 pending"`

### 3.3 减少 backlog 的方式

User 减少 backlog 的合法操作：

1. `wikimind review accept <ids>` — 接受
2. `wikimind review reject <ids> "reason"` — 拒绝（保留 reason 进 rejection memory）
3. `wikimind review accept-bundle <bundle-id>` — 一键接受整个 bundle
4. `wikimind review defer <ids> --until 2026-06-01` — 推迟（不算 pending，但到日期回来）
5. `wikimind review snooze <bundle-id> --for 7d` — 暂时压住整个 bundle
6. `wikimind review purge-stale --older-than 30d` — 清掉超过 30 天没动的（标 auto-rejected）

**永远不允许**的操作：

- ❌ `--ignore-queue-limit` — 不提供 bypass，避免破窗效应
- ❌ 直接 `rm wiki/_review/*` — daemon 检测到会自动 reconcile 回来
- ❌ 修改 hard_limit 到 ≥ 200 — config 校验拒绝，要求改 config 后必须 confirm "I understand"

### 3.4 配置 customize

| user 类型 | soft / hard / critical | 理由 |
|---|---|---|
| 个人新手 | 20 / 40 / 80 | 容错更紧，强制规律 review |
| 个人重度 | 30 / 50 / 100（**默认**） | 平衡 |
| 小团队（v0.2） | 60 / 100 / 200 | 多人分担 review |

---

## 4. Bundle 归并

### 4.1 自动归并规则

下列情况下，daemon 自动把多个 propose 合并到一个 bundle：

| 触发 | 规则 | 例子 |
|---|---|---|
| **同 ingest 任务** | 同一 raw 文件触发的所有 propose | "ingest karpathy-llm-wiki.md" 产生的 11 条 → bundle b-0042 |
| **同 query sediment** | 一次 query 沉淀的 page + 引用更新 | "RAG vs LLM Wiki" query 沉淀 → bundle b-0043 |
| **同 lint fix** | 同一 lint rule 命中的 fix proposals | "broken_link in concepts/" → bundle b-0044 |
| **同 dream cycle** | 一次 dream cycle 产生的 consolidate / evolve | "weekly dream cycle 2026-05-21" → bundle b-0045 |
| **agent 显式 `request_review(...)`** | agent 主动把一组 propose 打包 | agent 写 task "重构 concepts/ 的同义词" → 5 个 propose → 一个 bundle |

### 4.2 Bundle 的 review UX

User 看到的是 **bundle 列表**而不是 **propose 列表**：

```
$ wikimind review list

  b-0042  ⚠ critical  Ingest: Karpathy LLM Wiki gist            5 proposes  2 min ago
  b-0043  ◯ normal    Query sediment: "RAG vs LLM Wiki"         2 proposes  8 min ago
  b-0044  ◯ low       Lint fix: broken links in concepts/       4 proposes  14 min ago
  b-0045  ◯ low       Dream cycle: consolidate duplicate ents   3 proposes  19h ago

Total: 4 bundles · 14 proposes pending
```

Bundle 内的 propose 用**依赖图**展示（详见 [`agent-protocol.md`](agent-protocol.md) §4.2）。

### 4.3 一键操作

```bash
wikimind review show b-0042        # 看 bundle 内 5 条 propose 的综合 diff
wikimind review accept b-0042      # 接受整个 bundle（原子，要么全成要么全失败）
wikimind review reject b-0042 "test source not real"
wikimind review split b-0042 r-0245 r-0247 # 拆出 2 条单独 review
```

**关键 UX 决定**：**默认操作粒度是 bundle，不是 propose**。
单条 propose 操作要 user 主动 `wikimind review split` 拆出来。这把 review 工作量从 N 降到 N/5 ~ N/10。

### 4.4 Bundle 内的依赖原子性

Bundle 内的 propose 可能有依赖关系：

```
r-0245 (claim wiki-is-compounding)
  ↓ refers to
r-0246 (entity karpathy)    ← 必须先 accept
  ↓ source-anchor in
r-0249 (source-page)         ← 必须最先 accept
```

`wikimind review accept b-0042` 时，daemon 自动按拓扑序 commit。
如果 user 用 `split` 选择性接受部分 propose，daemon 会**拒绝**让依赖断裂的子集，提示 "r-0245 依赖
r-0246，不能只接受前者"。

---

## 5. Auto-Accept 白名单

### 5.1 默认全关

MVP 默认**不开启**任何 auto-accept。所有 propose 都进 review queue。
这是产品边界——**user 拍板**是核心承诺。

### 5.2 White-list 规则定义

User 显式开启后，可定义 `.wikimind/auto-accept.toml`：

```toml
# 仅允许的 auto-accept 类别（必须明确列举）
[rules.lint_broken_link_fix]
description = "Lint 发现的 broken_link 自动修复"
match.agent = "codex-cli"            # 仅来自 codex 的
match.bundle_type = "lint_fix"        # bundle 是 lint_fix 类型
match.rule = "broken_link"           # 修的是 broken_link
match.confidence = ">= 0.95"         # 高置信度
match.scope = "wiki/concepts/**"     # 仅 concepts/ 目录
max_per_day = 20                     # 单日上限
require_quote_hash_verify = true     # 必须 quote 校验通过

[rules.entity_alias_addition]
description = "为已存在 entity 增加 alias（不删旧、不改 body）"
match.type = "entity"
match.change_type = "frontmatter_only"
match.field = "aliases"
match.operation = "append_only"     # 只增不删
max_per_day = 50

[rules.dream_cycle_consolidate_duplicate]
description = "Dream Cycle 合并完全相同的重复 entity（仅自动合并 100% match）"
match.agent = "wikimindd"
match.bundle_type = "dream_cycle"
match.operation = "merge"
match.similarity = ">= 0.99"        # 100% 相似（含 normalize）
require_user_confirm_first_n = 5     # 头 5 次必须 user 确认，之后才 auto
```

### 5.3 Auto-Accept 的硬约束

无论 user 怎么配 white-list，下列**永远**不能 auto-accept：

- ❌ Schema 文件修改（`schema/**`）
- ❌ 创建新的 claim（除非 confidence ≥ 0.95 且明确在白名单里）
- ❌ 删除任何文件
- ❌ 修改 claim 的 sources / quote_hash / status
- ❌ 修改 entity / concept 的 body（仅 frontmatter 可考虑）
- ❌ Confidence < 0.9 的任何写入

任何 white-list 规则违反这些约束，daemon 启动时拒绝加载并提示。

### 5.4 Auto-Accept 的事后审计

所有 auto-accept 的 commit 必须：

1. 在 git commit message 中标 `[auto-accepted: rule=lint_broken_link_fix]`
2. 写入 `wiki/log.md` 时显式标 "auto-accepted by rule X"
3. 每周 Dream Cycle 时给 user 生成 `auto-accepts-weekly-report.md`：
   - 本周 auto-accept 多少条
   - 按规则分类
   - 抽样 5 条让 user 复核
4. 任何被 user 事后 revert 的 auto-accept 会**自动**触发 rule 评审：连续 3 次 revert 同一 rule →
   暂停该 rule，需 user 重新启用

---

## 6. 优先级排序

User `wikimind review list` 看到的 bundle/propose 排序**默认按多维评分**，让 user 永远先看最重要的。

### 6.1 评分维度

```
priority_score = (
    critical_flag           * 100  +    # 高优先级 bundle（lint 严重 / DRIFT / 冲突 claim）
    blocks_other_bundles    * 50   +    # 阻塞其它 bundle 的（依赖图根节点）
    confidence_inverse      * 30   +    # 低置信度优先（更需要 user 判断）
    age_hours               * 1    +    # 越老分越高
    bundle_size_log         * 5         # 大 bundle 略微优先（清一次清得多）
)
```

`critical_flag` 触发条件：

- Bundle 含 DRIFT claim
- Bundle 含 contradicts 已有 claim 的 propose
- Bundle 来自 master agent 标 `priority: high`
- Bundle 含 schema_version mismatch

### 6.2 排序示例

```
$ wikimind review list --sort priority

  PRIO  ID      LABEL                                      AGE     SIZE
  185   b-0040  ⚠ DRIFT: 3 claims need reverify             2h      3
  142   b-0042  ⚠ critical: ingest karpathy-llm-wiki        2m      5
   78   b-0043  ◯ query sediment: RAG vs Wiki              8m      2
   45   b-0044  ◯ lint fix: broken links in concepts/      14m     4
   12   b-0045  ◯ dream cycle: consolidate dupes          19h     3
```

User 永远先处理高分项。

### 6.3 优先级动态升级

某些事件会**自动升级**优先级：

- Bundle 等待超过 24h 未处理 → +30 分
- Bundle 内有 propose 被新 ingest 引用 → +50 分（说明这是"基础设施"）
- 用户连续 3 次跳过同一 bundle → -30 分（user 似乎不想看，沉到底部）

---

## 7. Review UX 三件套

让 user 每周 ≤ 30 分钟搞定 review，需要 UX 上的三个手势：

### 7.1 "今日 5 分钟" 模式

```bash
wikimind review today
```

输出：

```
今日推荐 review 列表（按优先级 + 你的历史习惯排序）：

  1. b-0040 ⚠ DRIFT (3 claims) ──────── 预计 2 分钟
  2. b-0042 ⚠ Ingest Karpathy (5 items) ─ 预计 2 分钟
  3. b-0043 ◯ Query sediment ──────── 预计 1 分钟

按 Enter 开始第 1 项...
```

完成 1 项自动进入下一项；user 中途 `q` 退出，剩余推迟到明天。

### 7.2 一键 "信任 agent"（高级 user）

```bash
wikimind review trust b-0044 --remember-agent codex-cli --remember-rule lint_fix
```

效果：
- 立即接受 b-0044
- 询问 "记住此规则吗？以后 codex-cli 的 lint_fix bundle 自动 accept？"
- user 答 yes → 提示 "已加入 white-list；可在 .wikimind/auto-accept.toml 查看"

### 7.3 拒绝时强制留 reason

```bash
$ wikimind review reject r-0247
Error: reject 必须带 reason。

$ wikimind review reject r-0247 "quote 不在原文，agent 编的"
✓ rejected r-0247
✓ reason 已加入 .wikimind/rejections.jsonl
```

Rejection memory 的用途：

- agent session 开始时读取近期 reject 摘要
- 让 agent 不重复同样的错误
- Dream Cycle 周报告分析 "top 5 rejection reasons"

---

## 8. 度量与监控

### 8.1 健康指标（user 可见）

| 指标 | 健康范围 | 异常处理 |
|---|---|---|
| Queue depth | < 30 | > 30 触发 soft warning |
| Avg review latency | < 24h | > 72h 触发 banner "review backlog 沉旧" |
| Daily accept ratio | 60-95% | < 50% 提示 "agent 抽取质量可能问题，检查 schema" |
| Auto-accept revert rate | < 5% | > 10% 自动暂停对应 rule |
| Bundle 内平均 propose 数 | 3-8 | > 15 提示 "bundle 太大，agent 应拆分" |

### 8.2 周度报告

每周 Dream Cycle 输出 `wiki/_reports/review-week-{date}.md`：

```markdown
# Review Week 2026-05-21

## Numbers
- Pending start of week: 12
- Pending end of week: 8
- Accepted: 47 (78%)
- Rejected: 13 (22%)
- Auto-accepted: 5 (white-list rules)
- Reverted (post-accept): 0

## Top reasons for rejection
1. quote_hash mismatch (5)
2. claim 粒度过细 (3)
3. duplicate of existing claim (2)
4. confidence 过低 (2)
5. provenance_depth > 1 (1)

## Recommendations
- Codex-cli 的 lint_fix 准确率 98%，建议升级到 auto-accept
- Hermes 的 claim 粒度偏细，schema/HERMES.md 可加约束
- 5 个 bundle 超过 7 天未处理，建议 purge 或 process
```

### 8.3 异常告警

下列情况推 OS 通知（默认开）：

- Queue 触达 hard_limit (50)
- 任一 auto-accept rule 连续 3 次被 revert
- 单次 ingest 产生 > 20 个 propose（agent 可能粒度爆炸）
- DRIFT 检测命中（任何 source 文件变化）

---

## 9. 失败模式与回滚

### 9.1 User 完全不 review

| 场景 | 系统行为 |
|---|---|
| pending = 50 | Agent 写入冻结，user 再不看也无法继续工作 |
| 7 天无任何 review | 推送 OS 通知 + email（如配置） |
| 30 天无任何 review | 自动 `wikimind review purge-stale --older-than 30d`（标 auto-rejected，不丢失） |

**核心保证**：queue 不会无限增长 → 系统不会被沉默淹死。

### 9.2 User 误 accept 一整批垃圾

```bash
wikimind review revert-bundle b-0042
# 等价于 git revert <commit>，change_log seq 推进，但语义反向
```

如果错过窗口（已被后续 commit 引用）→ 进入级联回滚流程，
详见 [`failure-playbook.md`](failure-playbook.md)（Wave 3）。

### 9.3 Auto-accept rule 误伤

```bash
wikimind review revert --since 2026-05-20T10:00 --by-rule lint_broken_link_fix
```

按规则 + 时间范围批量回滚 + 自动暂停 rule。

---

## 10. 与 SPEC 的关系

本文档定义的政策是 SPEC §4.4 "写入流程" 的**配套约束**。
SPEC 描述"写入怎么走"，本文档描述"如果 user 不及时拍板，怎么不让产品死"。

任何对 SPEC §4 的修改，必须同步检查本文档是否仍成立。

---

## 11. 不在范围

- **多 user 协作的 review**（v0.2+）—— MVP 默认单 user
- **分布式 review queue**（v1+）—— MVP 单机
- **ML-based 优先级评分**（v0.2+）—— MVP 用规则
- **Web review UI**（v0.2+）—— MVP CLI + MCP

---

## 一句话总结

> Review queue 不是"功能"，是协议的核心闸门。它的成败由"user 能不能稳定每周 30 分钟搞定"决定。
> 上限保护 + bundle 归并 + auto-accept 白名单 + 优先级排序——四个机制叠加，把 review 工作量降到
> 可持续区间。任一缺失，闸门破防。
