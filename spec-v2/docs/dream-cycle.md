# Dream Cycle：周期性主动维护

> **方案 B 独家贡献**：lint 是被动检查（"有没有错"），Dream Cycle 是主动演化（"能不能更好"）。
> 两者互补——lint 防止 wiki 变坏，Dream Cycle 让 wiki 变好。

---

## 1. 概念

### 1.1 为什么需要

Wiki 是 compounding artifact——但"compounding"不会自动发生。没有主动维护，长期会出现：

- **重复** —— 不同 agent / 不同时间为同一概念建了多个 entity（"Karpathy" / "Andrej Karpathy" / "@karpathy"）
- **术语漂移** —— 同一概念被不同 page 用不同词描述（"compounding artifact" / "复合工件" / "累积制品"）
- **孤岛** —— claim 之间该连的没连，知识图谱碎片化
- **空白** —— 某 entity 被频繁引用却没有自己的 page；某 concept 缺关键 claim
- **陈旧** —— claim 的 source 文件改了，但 claim 没 reverify

Lint 能发现其中一部分（orphan / broken_link / drift），但 lint 是规则匹配——它不会"想"：
"这两个 entity 其实是一个人"、"这个 concept 缺一条关键 claim"。

**Dream Cycle 是让 agent 在 wiki 空闲时段"做梦"——回顾、整理、演化整个知识库。**
名字来自方案 B，灵感是"睡眠时大脑整理记忆"。

### 1.2 与 lint 的分工

| 维度 | Lint | Dream Cycle |
|---|---|---|
| 性质 | 被动、规则匹配 | 主动、agent 推理 |
| 频率 | 每次 commit + 按需 | 周期性（默认每 24h） |
| 问题类型 | "明确的错"（broken_link、无源 claim、drift） | "可以更好"（重复、漂移、空白、连接） |
| 输出 | 警告列表 | propose bundle（进 review queue） |
| 是否改 wiki | 否（只报告） | 是（但所有改动进 review queue，user 拍板） |
| 计算成本 | 低（增量） | 高（要调 LLM agent 全局推理） |

---

## 2. 四阶段

```
┌─────────────────────────────────────────────────────────────┐
│  Dream Cycle  (默认每 24h，wiki 空闲时段触发)                  │
│                                                               │
│   ┌──────────┐   ┌─────────────┐   ┌─────────┐   ┌────────┐ │
│   │ 1. Audit │ → │2.Consolidate│ → │3. Evolve│ → │4.Report│ │
│   └──────────┘   └─────────────┘   └─────────┘   └────────┘ │
│   扫描健康度       合并重复          发现空白       生成周报    │
│   不改 wiki        → review queue   → review queue  → log.md   │
└─────────────────────────────────────────────────────────────┘
```

### 2.1 Stage 1 — Audit（只读扫描）

**目标**：全库体检，产出健康度报告。**不改 wiki**。

扫描项：

| 检查 | 产出 |
|---|---|
| Claim drift | 多少 claim 的 quote_hash 已 mismatch |
| Provenance 健康 | 多少 claim depth > 1（违规） |
| 重复候选 | 标题/别名相似度 > 0.85 的 entity / concept 对 |
| 术语漂移 | 同一 concept 被不同 page 用不同词指代 |
| 孤岛 page | 无 in-link 且无 out-link 的 page |
| 引用空白 | 被 ≥ 3 个 page 提及但无自己 page 的 entity 名 |
| Claim 覆盖 | 哪些 concept 的 claim 数 < 2（可能论据不足） |
| 陈旧 | 超过 N 天未 verify 的 claim |

输出：`audit-result.json`（内部），供 stage 2-3 用。

### 2.2 Stage 2 — Consolidate（合并）

**目标**：把 audit 发现的"重复 / 漂移"收敛。**改动进 review queue**。

| 任务 | 怎么做 | propose 类型 |
|---|---|---|
| 合并重复 entity | 高相似度对 → `propose_merge` | merge |
| 合并重复 concept | 同上 | merge |
| 统一术语 | 选 canonical 词，其它建 alias，更新引用 | edit（多页） |
| 修复 provenance depth > 1 | 把"引用 claim 的 claim"重挂 raw | edit |
| 连接孤岛 | 给孤岛 page 加合理的双链 | edit |

**安全约束**（关键）：

- **100% 相同的重复**（normalize 后完全一致）→ 可进 auto-accept 白名单（user 配置过才行）
- **相似但不完全相同**的合并 → 永远进 review queue，user 拍板
- **绝不**自动删除任何 page——合并是 `merged_into` 标记 + backlink 重定向，原 page 保留
- 术语统一**不改 claim 的语义**，只改用词；语义变化必须 user 决议

### 2.3 Stage 3 — Evolve（演化 / 补空白）

**目标**：发现知识空白，提议补充。**改动进 review queue**。

| 任务 | 怎么做 | propose 类型 |
|---|---|---|
| 补缺失 entity page | "被频繁引用但无 page"的 entity → 提议建 stub page | page（entity，conf 标低） |
| 补缺失 concept | 同上 | page（concept） |
| 提议补 claim | concept 的 claim 数 < 2 → 提示 user "这个 concept 论据不足"（**不自动编 claim**） | 仅 report，不 propose |
| 建议 reverify | drift / 陈旧 claim → 提议 reverify 任务 | report + 标记 |
| 关系补全 | 发现"该连未连"的 page → 提议加双链 | edit |

**关键安全约束**：

- Evolve **不会凭空编造 claim**。"这个 concept 缺 claim" 只会写进 report 提醒 user，
  **不会**让 agent 去"补一条看起来合理的 claim"——那是幻觉之源。
- 补 entity / concept 的 stub page 只含"骨架"（标题 + 已知别名 + 引用它的 page 列表），
  **不含**未经 source 支持的描述性内容。conf 标 `n/a`（stub）。
- 所有 evolve 的 propose 默认 `priority: low`——它们是"锦上添花"，不该挤占 user 对 ingest review
  的注意力。

### 2.4 Stage 4 — Report（生成周报）

**目标**：把本轮 Dream Cycle 的发现 + 历史趋势写成人类可读周报。

输出 `wiki/_reports/dream-cycle-{date}.md`：

```markdown
# Dream Cycle Report · 2026-05-21 22:00

## Health
- Vault health score: 87 (↑3 from last week)
- Claims: 186 (+12) · Entities: 42 (+3) · Concepts: 28 (+2)
- DRIFT claims: 1 (需 reverify)
- Orphan pages: 2

## Consolidate（已提议，等你 review）
- bundle b-0045: merge "Andrej Karpathy" + "@karpathy" → entities/karpathy.md
- bundle b-0045: 统一术语 "复合工件" → "compounding artifact"（影响 4 页）

## Evolve（已提议，priority low）
- bundle b-0046: 新建 entity stub "Andy Matuschak"（被 3 页引用，无 page）
- 提示：concept "source-of-truth" 只有 1 条 claim，论据偏薄——建议你 ingest 更多资料

## Trends（过去 4 周）
- Claim 增长：142 → 158 → 174 → 186（稳定）
- Review accept rate：78%（健康）
- 平均 claim confidence：0.84（稳定）

## Recommendations
1. 清理 1 个 DRIFT claim（karpathy-llm-wiki.md#section-3 已变更）
2. Review bundle b-0045（consolidate，2 分钟）
3. concept "source-of-truth" 需要更多 source
```

同时 `log_append` 一条到 `wiki/log.md`。

---

## 3. 调度

### 3.1 默认调度

```toml
# .wikimind/config.toml
[dream_cycle]
enabled = true
schedule = "daily@idle"     # 每天 wiki 空闲时段跑一次
idle_threshold_min = 30     # "空闲" = 30 分钟无 agent 写操作
max_runtime_min = 20        # 单次最长 20 分钟，超时中止
agent = "claude-code"        # 用哪个 agent 执行（需 master agent 资格）
```

| schedule 取值 | 含义 |
|---|---|
| `daily@idle` | 每天找一个空闲时段（默认） |
| `daily@HH:MM` | 固定时刻（如 `daily@03:00`） |
| `weekly@idle` | 每周一次（适合低频用户） |
| `manual` | 仅 `wikimind dream` 手动触发 |

### 3.2 触发条件

Dream Cycle 启动需**同时**满足：

- 距上次运行 ≥ schedule 间隔
- review queue pending < soft_limit（30）——queue 已经堆积时不该再加 propose
- 当前无 active ingest job
- 距最近一次 agent 写操作 ≥ idle_threshold

不满足 → 跳过本次，下次再判。

### 3.3 手动触发

```bash
wikimind dream                    # 跑完整四阶段
wikimind dream --stage audit      # 只跑 audit（只读，安全）
wikimind dream --dry-run          # 跑但不产生 propose，只出 report
```

---

## 4. 安全模型

Dream Cycle 会改 wiki，所以安全约束比 lint 严：

| 约束 | 说明 |
|---|---|
| 所有改动进 review queue | consolidate / evolve 的 propose 都走标准 review，user 拍板 |
| 不自动删除 | 合并用 `merged_into` 标记，不真删 |
| 不编造 claim | evolve 只建 stub / 提醒，绝不生成无源 claim |
| 受 review queue 上限约束 | queue 满时 Dream Cycle 跳过（不加剧 backlog） |
| 单次 propose 上限 | 一轮 Dream Cycle 最多产生 1 个 bundle、≤ 15 个 propose；超出截断到下轮 |
| 优先级低 | evolve 的 propose 默认 `priority: low`，不抢 ingest review 的注意力 |
| 可整体 revert | 一轮 Dream Cycle = 1 个 bundle = 可一键 `revert-bundle` |
| 有 dry-run | `--dry-run` 让 user 先看会改什么 |

### 4.1 100% 重复的特例

唯一可以"少一道 review"的情况：consolidate 阶段发现**normalize 后 100% 相同**的重复 entity
（如 `entities/karpathy.md` 和 `entities/Karpathy.md` 内容完全一致，只是文件名大小写不同）。

这种**可以**进 auto-accept 白名单——但：
- 必须 user 在 `.wikimind/auto-accept.toml` 显式配置 `dream_cycle_consolidate_duplicate` 规则
- 头 5 次仍要 user 确认（`require_user_confirm_first_n = 5`）
- 详见 [`review-queue-policy.md §5.2`](review-queue-policy.md#52-white-list-规则定义)

---

## 5. 失败处理

| 失败 | 处理 |
|---|---|
| Dream Cycle 超时（> max_runtime） | 中止；已产生的 propose 保留；report 标 "partial" |
| Agent 在 consolidate 中崩溃 | worktree 保留；下次 Dream Cycle 从 audit 重来 |
| audit 发现 vault 严重损坏 | 中止 consolidate/evolve；report 标红；提示 user 跑 `wikimind doctor` |
| 一轮 Dream Cycle 的 bundle 被 user 整体 reject | 记入 rejection memory；连续 3 轮被 reject → 自动转 `schedule: manual` 并提示 user |
| review queue 满 | 跳过本轮（不产生 propose），仅出 audit report |

---

## 6. 度量

Dream Cycle 自身的健康指标（在 weekly report 中）：

| 指标 | 健康范围 |
|---|---|
| 单轮运行时长 | < 20 min |
| 单轮产生 propose 数 | 3-15 |
| Consolidate propose accept rate | > 70%（低于说明合并判断不准） |
| Evolve propose accept rate | > 40%（evolve 本就是"建议"，accept 率天然低些） |
| 连续被整体 reject 次数 | 0（≥ 3 自动转 manual） |

---

## 7. 与其它机制的关系

- **Lint** —— Dream Cycle 的 audit 阶段复用 lint 的检查规则，但额外做"相似度 / 漂移"这类 lint 做不了的
- **Query Sedimentation** —— 两者都产生 propose；Dream Cycle 是"周期性整理"，Sedimentation 是"查询触发回写"
- **Review Queue** —— Dream Cycle 的所有产出都经 review queue；受其上限保护约束
- **Claim Extraction** —— evolve 阶段补 stub 时遵守 claim-extraction 的"不编造"原则

---

## 8. 配置全集

```toml
[dream_cycle]
enabled = true
schedule = "daily@idle"
idle_threshold_min = 30
max_runtime_min = 20
agent = "claude-code"
max_proposes_per_run = 15
stages = ["audit", "consolidate", "evolve", "report"]   # 可裁剪，如只跑 ["audit", "report"]
report_dir = "wiki/_reports"

[dream_cycle.consolidate]
entity_similarity_threshold = 0.85    # 高于此判为重复候选
term_drift_detection = true

[dream_cycle.evolve]
enabled = true
entity_stub_min_references = 3        # 被引用 ≥ 3 次才提议建 stub
priority = "low"
```

User 可裁剪 stages——例如保守用户只跑 `["audit", "report"]`（纯只读体检，不产生任何 propose）。

---

## 9. 不在范围

- **跨 vault 的 Dream Cycle**（v2+）
- **实时演化**（每次 commit 就整理）——成本太高，违反"周期性"设计
- **自动写 claim 正文**——永不做，违反防幻觉原则
- **ML 驱动的相似度判断**——MVP 用字符串 + embedding（如启用）相似度；ML 模型 v0.2+

---

## 一句话总结

> Lint 防止 wiki 变坏，Dream Cycle 让 wiki 变好。四阶段 audit → consolidate → evolve → report
> 周期性运行，发现重复 / 漂移 / 空白并提议修复——但所有改动进 review queue，绝不编造 claim，
> 绝不自动删除。这是 wiki "compounding" 的发动机。
