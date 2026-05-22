# WikiMind Spec v2

> 整合三方案 + 补三方共同盲点的**统一新 spec**。

## 这是什么

仓库根目录的 `gpt-pro-深度研究.md`、`llm-wiki-product-spec.md`、`REPORT.md + docs/* + templates/*`
三套独立方案经过横向审查、对比、整合后的统一 spec。

- **不替代** 原三方案——它们仍保留在根目录作为历史与对比基线。
- **整合策略**：以多文件方案 A 为基底（协议最严谨、模块化最好、有可复用 templates、风险清单 21 条），
  吸收方案 B（单文件 WikiMind）的 Dream Cycle / Query Sedimentation / Day-by-day roadmap，
  吸收 GPT Pro 方案的 CJK tokenizer / git worktree per agent / 平台细节 / MCP `readOnlyHint`，
  并**新增** 4 篇补丁文档解决三方共同没解决好的工程问题。

详细血统标注见 [`SPEC.md` §11](SPEC.md#11-三方案血统标注)。

---

## 怎么读

### 30 分钟扫读路径

1. **[`SPEC.md`](SPEC.md)** —— 主 spec，含定位 / 三层架构 / Claim / 协议 / MVP / 五层防御 / 验收
2. **[`docs/claim-extraction.md`](docs/claim-extraction.md)** —— Claim 粒度算法（产品的核心可执行约束）
3. **[`docs/onboarding.md`](docs/onboarding.md)** —— `wikimind demo` 5 分钟剧本（产品体验的第一公里）
4. **[`docs/review-queue-policy.md`](docs/review-queue-policy.md)** —— Review queue 上限保护（最容易翻车的环节）

读完这 4 篇即可判断"整合方向是否对、我是否要继续推进 Wave 2/3"。

### 完整阅读路径

按 `SPEC.md` §9 的文档导航顺序读。

---

## 这次新 spec 跟原方案的核心差异

| 维度 | 原三方案最强者 | spec v2 |
|---|---|---|
| 协议严谨度 | 方案 A（review queue + handshake + change log） | A + GPT Pro 的 worktree per agent **物理隔离** |
| Claim 校验 | 方案 A（quote_hash） | A + lint 检测"confidence 与文字限定不一致" |
| 跨平台 | GPT Pro（APFS / NTFS / CJK 细节） | A 的约定 + GPT Pro 的细节 + **CJK tokenizer 独立成篇** |
| 周期维护 | 方案 B（Dream Cycle） | A 的 lint（被动）+ B 的 Dream Cycle（主动），**互补不替代** |
| 问答回写 | 方案 B（Query Sedimentation） | 吸收 + 明确入 review queue 而非直接写入 |
| Roadmap | 方案 B（Day-by-day） | B 的 D1–D30 + A 的周末出口标准 |
| **Claim 粒度** | 三方均缺 | **新增可执行算法 + 10+ 案例** |
| **Onboarding** | 三方均缺 | **新增 `wikimind demo` 5 分钟剧本** |
| **Review queue 上限** | 仅方案 A R-18 浅提 | **新增硬阈值 + bundle 归并 + auto-accept 白名单** |
| **多 agent 冲突剧本** | 协议有，剧本缺 | **新增 5 个真实场景剧本** Wave 3 |
| **依赖图级联回滚** | 三方均缺 | **新增** 扩展 git revert Wave 3 |

---

## 文件结构

```
spec-v2/
├── README.md                       ← 你在这里
├── SPEC.md                         ← 主 spec
├── docs/
│   ├── claim-extraction.md         ✅ Wave 1
│   ├── onboarding.md               ✅ Wave 1
│   ├── review-queue-policy.md      ✅ Wave 1
│   ├── architecture.md             ✅ Wave 2
│   ├── agent-protocol.md           ✅ Wave 2
│   ├── mcp-tools.md                ✅ Wave 2
│   ├── cross-platform.md           ✅ Wave 2
│   ├── filesystem-access.md        ✅ 整合补遗
│   ├── cjk-tokenizer.md            ✅ Wave 2
│   ├── dream-cycle.md              ✅ Wave 2
│   ├── query-sedimentation.md      ✅ Wave 2
│   ├── conflict-scenarios.md       ✅ Wave 3
│   ├── failure-playbook.md         ✅ Wave 3
│   ├── risks.md                    ✅ Wave 3
│   ├── roadmap-30d.md              ✅ Wave 3
│   └── engineering-decisions.md    ✅ 工程准备
├── templates/
│   ├── AGENTS.md                   ✅ Wave 3（基于 A 的微调）
│   ├── CLAUDE.md                   ✅ Wave 3
│   ├── CODEX.md                    ✅ Wave 3
│   ├── HERMES.md                   ✅ Wave 3
│   ├── CURSOR.md                   ✅ Wave 3（新增）
│   ├── page-schemas.md             ✅ Wave 3
│   └── lint-rules.md               ✅ 一致性审查补
└── examples/
    ├── directory-tree.md           ✅ Wave 3
    └── demo-walkthrough.md         ✅ Wave 3（搭配 onboarding.md）
```

---

## 整合的 5 个默认决策

以下决策已在 spec 中固化，**如要改请在 Wave 2 之前提出**：

| 决策 | 取值 | 理由 |
|---|---|---|
| 产品名 | **WikiMind** | UI 原型已固化，方案 B 也用此名 |
| Daemon 语言 | **Go** | 跨平台单二进制、并发好、watcher 库成熟 |
| Ingest worker | **Python** | PDF/OCR/whisper 生态成熟 |
| MCP 优先适配 | **Claude Code + Codex CLI** | MVP D14 demo 必须验证 |
| Embedding 默认 | **关** | MVP 不引入；W4 可选本地 `bge-small` |
| 审稿风格 | **严肃工程文档** | 多表格、显式取舍、出口标准 |

---

## Wave 1 验收

Wave 1 写完意味着你能从**5 篇文档**判断整合方向是否成立：

- ✅ `SPEC.md` 能让一个没读过三原方案的人 15 分钟理解新 spec
- ✅ `claim-extraction.md` 含 10+ 可执行案例（5 个 should + 5 个 should-not）
- ✅ `onboarding.md` 含完整 5 分钟 demo 剧本（命令、输出、关键时刻）
- ✅ `review-queue-policy.md` 明确上限、归并算法、user UX 三件套
- ✅ `README.md`（本文件）说清三方案血统、整合策略、怎么读

如果你看完这 5 篇说"方向对 → 继续 Wave 2"，就推进；说"方向不对 → 改"，就在这里调整，避免浪费 Wave 2/3。

---

## 给原三方案的"补丁清单"（一句话回应）

| 原方案 | 这次 v2 新增了什么 |
|---|---|
| 方案 A | + Dream Cycle / Query Sedimentation / Day-by-day roadmap / CJK tokenizer / git worktree per agent / 4 篇盲点补丁 |
| 方案 B | + 严格的 review queue 协议 / quote_hash 反验证 / schema_version handshake / 21 条风险清单 / templates 可复用 / 4 篇盲点补丁 |
| GPT Pro | + Day-by-day roadmap / Dream Cycle / Query Sedimentation / templates 可复用 / review queue 上限保护 / 多 agent 冲突剧本 / 依赖图级联回滚 |

> 每个原方案的"独家优秀点"都被吸收；每个原方案的"未提之处"都被另外两方或新增补丁补上。

---

## 历史

- **2026-05-20 ~ 21**：三套独立方案产生（A、B、GPT Pro）
- **2026-05-21**：三方案横向审查 + UI 原型 v1/v2 验证产品形态
- **2026-05-21**：spec v2 Wave 1 完成（SPEC + README + 3 篇 P0 盲点补丁）
- **2026-05-22**：spec v2 Wave 2 完成（7 篇：架构 / 协议 / MCP / 跨平台 / CJK / Dream Cycle / Query Sedimentation）
- **2026-05-22**：spec v2 Wave 3 完成（4 篇文档 + 6 个 templates + 2 个 examples）— **整合 spec 全部完成**
