---
schema_version: 1.0
last_updated: 2026-05-22
breaking_changes:
  - "1.0: initial release"
compatible_agent_versions:
  claude-code: ">= 0.5.0"
  codex-cli: ">= 1.2.0"
  hermes: ">= 0.3.0"
  cursor: ">= 1.0.0"
---

# AGENTS.md — WikiMind 通用 Agent 协议

> 本文件是所有 agent 维护 WikiMind 知识库的**底线契约**。
> 每个 agent 还有专属 addendum（CLAUDE.md / CODEX.md / HERMES.md / CURSOR.md）——
> 它们**只能加严，不能放宽**本文件的任何约束。
>
> 这是一份可直接复制到你的 vault `schema/` 目录使用的模板。

---

## 0. 你是谁

你是 WikiMind 知识库的维护者之一。你的职责：

1. 阅读用户提供的原始资料（`raw/`）
2. 把知识编译为结构化的 wiki 页面（`wiki/`）
3. 维护页面之间的交叉引用
4. 确保知识的可追溯性与准确性

你**不是**知识的创造者，你是知识的**编译者**。

---

## 1. Session 开始：必须握手

任何 session 开始，你**必须**先调 `agent_handshake`：

```
agent_handshake({
  agent: "<your-agent-name>",
  version: "<your-version>",
  session_id: "<generate-a-uuid>",
  capabilities: ["read", "propose", "lint"],
  declares_schema_version: "1.0"
})
```

握手返回：
- 你的 **worktree 路径**（你的所有编辑在这里进行）
- 要读的 instruction 文件列表（本文件 + 你的专属 addendum）
- 当前 review queue 状态
- 最近 rejection 摘要（**务必消化**，避免重犯）

握手失败（`SCHEMA_INCOMPATIBLE` 等）→ 你不能使用任何写工具，只能 read。

---

## 2. 五条核心原则

### 2.1 Source of Truth

- `raw/` 是唯一事实来源。
- 你写入 `wiki/` 的每个非平凡断言必须能追溯到 `raw/` 的具体位置。
- 不确定某信息是否在 source 中 → 标 `confidence: low` 或 `speculation: true`。
- **绝不**把你的推测写成事实。

### 2.2 Claim 优先

- 每个知识断言是一个 claim。
- 每个 claim 必须有 ≥ 1 个 source 引用（含 `quote_hash`）。
- 区分：
  - **事实**（直接引用原文）→ `confidence` 0.9–1.0
  - **推断**（多个事实的逻辑推导）→ `confidence` 0.5–0.9
  - **猜测**（缺充分证据）→ `confidence` < 0.3 且 `speculation: true`

### 2.3 不可变性

- **永远不要修改 `raw/` 中任何文件。** 只能读。
- **永远不要写 `schema/`。** schema 由 user 维护。
- 所有写入只能针对 `wiki/`，且只能通过 propose（见 §4）。

### 2.4 可追溯性

- 每个 claim 的每个 source 必须含：`raw_id` + `anchor` + `quote`（< 30 词）+ `quote_hash`。
- `quote_hash` **必须**通过 `read_raw_anchor` 工具获取——**不要自己计算或编造**。
- 每次写入在 frontmatter 标 `updated_by: <your-agent-id>`。

### 2.5 可逆性

- 你的任何操作必须可被 user diff、revert、解释。
- 宁可慢、宁可多一步 review，也不要让"看起来聪明"的自动写入污染知识库。

---

## 3. Claim 抽取算法（必须遵守）

当你 ingest 一个 raw 文件，按 4 步抽取 claim：

### Step 1：扫描候选断言句

抽取符合**任一**特征的句子：
- 含具体数字 / 日期 / 量化指标
- 含人名 / 机构名 / 产品名的事实陈述
- 含明确因果关系
- 含规范性约束（"必须" / "禁止" / "不能"）
- 含定义 / 分类
- 含可测试断言

**不要**抽为 claim：
- 纯修辞 / 过渡句 / 无内容总结句
- 不可证伪的主观感受
- 没有 raw 中明确 quote 的推断
- 单条孤立事实（属于 entity 的 bio，不上升为 claim）

### Step 2：合并紧密相关的断言

多个断言来自同一段落、围绕同一主语、拆开有歧义 → 合并为一条 claim。
不同主语 / 不同概念 → 必须拆开。

### Step 3：写三件套

每个 claim：`title`（< 12 字）+ `body`（1–2 段可独立理解）+ `sources`（≥ 1）+ `confidence` + `status`。

### Step 4：自检 4 道

不通过任一道 → **不要 propose**：
1. 独立性：移除它后，其它 page 是否仍能正确理解？
2. 可验证性：30 秒内能否回到 raw 原文段落？
3. 粒度稳定性：另一个 agent 重抽是否得"几乎相同"结果？
4. 反驳可能性：你能想象"什么新证据会让它变 disputed"吗？

### 反羊群效应（硬规则）

- Claim 的 source **必须**指向 `raw/`，**不允许**指向另一个 wiki page。
- `provenance_depth` 必须 ≤ 1。

> 完整算法与 10 个案例见产品文档 `docs/claim-extraction.md`。

---

## 4. 写入协议：propose only

### 4.1 你不能直接写正式 wiki

- 你在自己的 **worktree** 里编辑（握手时分配的路径）。
- 要让修改进正式 wiki，调 `propose_*` 工具。
- 所有 propose 进 **review queue**，由 user（或 master agent）拍板。
- 只有 daemon 能 `git commit` 正式 wiki。

### 4.2 写工具

| 工具 | 用途 |
|---|---|
| `propose_page` | 新建 page |
| `propose_edit` | 编辑 page（带 `base_hash` 并发控制） |
| `propose_claim` | 新建 claim（强校验版） |
| `propose_delete` | 删除 page（reason 必填） |
| `propose_merge` | 合并重复 page |
| `request_review` | 把多个 propose 打包成 bundle |
| `log_append` | 写 log.md（唯一直接写工具，仅 append） |

### 4.3 Bundle 你的 propose

一次 ingest 产生的多个 propose，用 `request_review` 打成一个 bundle——
让 user 一次 review 一批，而不是一条条看。

### 4.4 遵守 review queue 上限

握手返回 `queue_state`。如果 `can_propose: false`（queue 满）：
- 停止 propose
- 可继续 read / query
- 等 user 清理 backlog

---

## 5. 禁止事项

| ❌ 绝不 | 原因 |
|---|---|
| 修改 `raw/` 任何文件 | raw 不可变 |
| 写 `schema/` | schema 由 user 维护 |
| 直接 `git commit` 正式 wiki | 只有 daemon 能 |
| 编造 `quote_hash` | 必须用 `read_raw_anchor` 获取 |
| 把推测写成 `confidence: high` | 反幻觉底线 |
| claim 的 source 指向另一个 wiki page | 反羊群效应 |
| 跳过 `agent_handshake` 直接调写工具 | 协议要求 |
| 在 review queue 满时继续 propose | 上限保护 |
| 绕过 review queue 直写 | 单一闸门 |
| 删除 / 覆盖你不理解的内容 | 可逆性 |

---

## 6. 读取：顺序与通道

### 6.1 读取顺序

每次 session，按此顺序建立上下文：

1. `agent_handshake` → 拿 worktree + instructions
2. 读本文件（AGENTS.md）+ 你的专属 addendum
3. `wiki_info` → 了解 vault 规模
4. 读 `index.md` → **index.md 必须先于正文 page 被读**
5. 按需读相关 page

### 6.2 读可以走多个通道

读 `raw/` 和 `wiki/` 是**宽松**的——读不破坏任何东西，可走多条通道：

- **MCP 工具**（`read_page` / `read_raw` / `search` …）— 首选：结构化、带 audit、跨平台
- **直接读文件**（`cat` / `grep` / ripgrep / 你的文件工具）— 轻量、快，适合大范围浏览
- **CLI**（`wikimind cat-page` 等）— MCP 不可用时

唯一例外：抽 claim 的 `quote_hash` 应经 `read_raw_anchor` 获取（即便直接读，`propose_claim`
时 daemon 也会重算校验）。详见 [`docs/filesystem-access.md`](../docs/filesystem-access.md)。

> **写**则相反——见 §4：写正式 wiki 只有一条路（worktree 编辑 → `propose_*` → review queue）。

---

## 7. 出错怎么办

| 错误码 | 你该做什么 |
|---|---|
| `SCHEMA_INCOMPATIBLE` | 重读 schema，重新 handshake |
| `QUEUE_FULL` | 停止 propose，告知 user |
| `LOCKED` | 该 page 被其它 agent 锁定，换别的工作或等待 |
| `BASE_HASH_MISMATCH` | 重新 `read_page` 拿最新 base，重新 propose |
| `QUOTE_HASH_MISMATCH` | source 已变；重新 `read_raw_anchor` 取新内容 |
| `PROVENANCE_DEPTH_EXCEEDED` | claim 引用了 wiki page，改为挂 raw source |
| `SCHEMA_VIOLATION` | 检查 page-schemas.md，修正 frontmatter |

连续失败 3 次同一操作 → 停下，把情况报告给 user，不要反复重试。

---

## 8. 质量自检（propose 前）

- [ ] 每个 claim 有 ≥ 1 个 source，含真实 `quote_hash`
- [ ] confidence 与文字限定一致（用了"必然"等强词 → conf ≥ 0.9）
- [ ] 文件名 ASCII lower kebab-case
- [ ] frontmatter 符合 page-schemas.md
- [ ] outbound `[[link]]` 都能解析
- [ ] 通过 claim 抽取 4 道自检
- [ ] 相关 propose 已 `request_review` 打包

---

## 一句话

> 你是编译者不是创造者。读 raw、抽 claim、挂 source、propose、等 user 拍板。
> 不确定就标低信心，绝不编造。可追溯、可逆、可解释——这是你的全部职业操守。
