---
schema_version: 1.0
last_updated: 2026-05-22
---

# lint-rules.md — Lint 规则定义

> WikiMind **全部 lint 规则的单一权威定义**。SPEC / claim-extraction / dream-cycle / roadmap
> 等文档引用本文件，不另行定义规则。
>
> 可直接复制到 vault `schema/` 目录使用。

---

## 0. Lint 是什么

Lint 是对 wiki 的**被动质量检查**——只报告问题，不修改 wiki（修复要走 `propose_*` → review queue）。

- **何时跑**：每次 commit 后增量跑；`wikimind lint` 手动全量；Dream Cycle 的 audit 阶段复用
- **怎么跑**：`lint_run` MCP 工具 / `wikimind lint` CLI
- **输出**：warnings 列表（severity + page + detail + suggested_action）

---

## 1. 命名规范

| 类别 | 前缀 | 作用范围 | 例 |
|---|---|---|---|
| 通用规则 | 无前缀 | 所有 page 类型 | `orphan` / `broken_link` |
| claim 专项规则 | `claim_` | 仅 claim | `claim_drift` |
| 结构规则 | 无前缀 | 文件系统层 | `filename_convention` |

---

## 2. Severity 级别

| 级别 | 含义 | 是否阻断 |
|---|---|---|
| `error` | 违反硬约束，必须修 | 阻断 propose / commit |
| `warn` | 质量问题，应修 | 不阻断 |
| `info` | 提示，可选处理 | 不阻断 |

---

## 3. 通用规则（8 条）

| 规则 | severity | 检测 | 触发动作 |
|---|---|---|---|
| `orphan` | warn | page 无 in-link 且无 out-link（默认 30 天内无引用、无 backref） | 标 candidate_for_removal |
| `broken_link` | warn | `[[id]]` 无法解析到存在的 page | 提示创建或修拼写 |
| `contradictions` | warn | 两个 claim 语义矛盾（同主语、同指标、不同值） | 标 disputed，进 review |
| `stale` | info | page 超过 N 天（默认 180）未更新 / 未验证 | 提示 reverify |
| `unverified_claim` | warn | claim 长期处于 unverified 状态（默认 > 30 天） | 提示 user 决议 |
| `duplicate_entity` | warn | 疑似重复 entity（标题 / 别名相似度 > 0.85） | 提示 merge → Dream Cycle consolidate |
| `schema_violation` | error | frontmatter 不符 [`page-schemas.md`](page-schemas.md) | 阻断 propose |
| `missing_index_entry` | info | page 未登记在 `index.md` | 提示补 index |

---

## 4. Claim 专项规则（4 条，`claim_` 前缀）

| 规则 | severity | 检测 | 触发动作 |
|---|---|---|---|
| `claim_no_source` | error | claim 无 source 且 status != speculation | 拒绝 propose / 标 needs_source |
| `claim_drift` | warn | source 的 quote_hash 与存储值 mismatch | 标 needs_reverify，review 中显示 DRIFT |
| `claim_provenance_depth_gt_1` | error | claim 的 source 指向 wiki page 而非 `raw/` | 拒绝 propose |
| `claim_confidence_qualifier_mismatch` | warn | 正文用"必然 / 一定 / 绝对"等强限定词但 confidence < 0.9 | 警告，建议下调措辞或上调 conf |

> 孤立 claim 的检测用通用规则 `orphan`，**不另设 `claim_orphan`**——孤立检查对所有 page 类型
> 一视同仁，无需 claim 特化。

---

## 5. 结构规则（1 条）

| 规则 | severity | 检测 | 触发动作 |
|---|---|---|---|
| `filename_convention` | error | 文件名不符 `^[a-z0-9][a-z0-9-]*\.md$`（ASCII 小写 kebab-case） | 阻断；`wikimind doctor --fix-names` 修复 |

详见 [`../docs/cross-platform.md §1`](../docs/cross-platform.md)。

---

## 6. 规则组别名

为方便批量执行，定义规则组别名：

| 别名 | 等价于 |
|---|---|
| `claim_quality` | 全部 4 条 claim 专项规则 |
| `structural` | `orphan` + `broken_link` + `missing_index_entry` + `filename_convention` |
| `all` | 全部 13 条规则（默认） |

用法：`wikimind lint --rule claim_quality` 或 `lint_run({rules: ["claim_quality"]})`。

---

## 7. 增量与性能

- 默认**增量**：只扫 `change_log` 标记的 dirty 文件
- `wikimind lint --full` 全量
- 全量性能目标：< 60s @ 10k 页（见 [`../docs/architecture.md §8`](../docs/architecture.md)）
- 单规则可独立跑：`wikimind lint --rule broken_link`

---

## 8. 配置

```toml
# .wikimind/config.toml
[lint]
stale_days = 180
unverified_claim_days = 30
orphan_days = 30
duplicate_entity_similarity = 0.85
```

---

## 9. Lint 与其它机制

| 机制 | 关系 |
|---|---|
| Dream Cycle | audit 阶段复用全部 lint 规则；consolidate 处理 `duplicate_entity` / `contradictions` |
| Review queue | lint 只报告；修复一律走 `propose_*` → review queue |
| Claim Extraction | claim 专项 4 条规则保障 claim 抽取质量（见 [`../docs/claim-extraction.md §7`](../docs/claim-extraction.md)） |
| Cross-platform | `filename_convention` 是跨平台一致性的强制规则 |

---

## 一句话

> 13 条 lint 规则 = 通用 8 条（无前缀）+ claim 专项 4 条（`claim_` 前缀）+ 结构 1 条。
> Lint 只报告不修改；修复一律走 review queue。本文件是规则的单一权威定义。
