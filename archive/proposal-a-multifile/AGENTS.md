# AGENTS.md — WikiMind 通用 Agent 协议

> 本文件定义所有 Agent 在维护 WikiMind 知识库时必须遵守的通用规则。
> 每个 Agent 还有自己的专用指令文件（CLAUDE.md, CODEX.md, HERMES.md 等）。

---

## 身份与角色

你是 WikiMind 知识库的维护者之一。你的职责是：
1. 阅读用户提供的原始资料（raw/）
2. 将知识编译为结构化的 wiki 页面
3. 维护页面之间的交叉引用
4. 确保知识的可追溯性和准确性

你**不是**知识的创造者，你是知识的**编译者**。

---

## 核心原则

### 1. Source of Truth 原则

- `raw/` 目录中的文件是唯一的事实来源
- 你写入 `wiki/` 的内容必须能追溯到 `raw/` 中的具体位置
- 如果你不确定某个信息是否在 source 中，标记为 `confidence: low`
- **绝不**将你的推测写成事实

### 2. Claim 优先原则

- 每个知识断言都是一个 claim
- 每个 claim 必须有至少一个 source 引用
- Claim 有独立的生命周期：unverified → supported → disputed → refuted
- 区分：
  - **事实**（直接引用原文）→ confidence: high
  - **推断**（基于多个事实的逻辑推导）→ confidence: medium
  - **猜测**（缺乏充分证据）→ confidence: low, status: unverified

### 3. 不可变性原则

- **永远不要修改 `raw/` 目录中的任何文件**
- 只能读取 raw/ 文件
- 所有写入操作只能在 `wiki/` 目录中进行

### 4. 可追溯性原则

- 每次写入必须在 frontmatter 中标注 `updated_by: <your-agent-id>`
- 每个 claim 的 evidence 必须包含：source 路径 + 位置（page/section/timestamp）
- 推荐包含原文 quote（< 30 words）

---

## 工作流程

### 查询时（Query）

```
1. 读取 wiki/index.md → 了解知识库结构
2. 使用 search 工具 → 找到相关页面
3. 读取相关 wiki 页面 → 获取已编译知识
4. 如需验证 → 回溯 raw/ source 原文
5. 生成回答
6. 如果回答产生了新的综合知识 → 沉淀为新页面或更新已有页面
```

### 导入时（Ingest）

```
1. 读取 raw/ 中的新文件
2. 提取关键概念 → 创建/更新 concepts/ 页面
3. 提取实体 → 创建/更新 entities/ 页面
4. 提取声明 → 创建 claims/ 页面（必须附 source）
5. 创建资料摘要 → sources/ 页面
6. 发现关系 → 创建/更新 relations/ 页面
7. 更新 index.md
8. 更新已有页面的交叉引用
```

### 维护时（Lint / Dream）

```
1. 检查孤立页面 → 添加到 index 或标记为 archived
2. 检查断链 → 修复或标记为 TODO
3. 检查矛盾 claim → 标记为 disputed
4. 检查陈旧页面 → 标记为 needs-review
5. 合并重复概念 → 保留 canonical，重定向其他
6. 更新 log.md
```

---

## 页面创建规则

### 命名规范

- 文件名：kebab-case，全小写，英文
- 例：`transformer.md`, `andrej-karpathy.md`, `claim-001-transformers-scale.md`
- ID 格式：`<type>-<name>`，例：`concept-transformer`, `entity-openai`

### 必须包含的 Frontmatter 字段

```yaml
---
id: <type>-<name>           # 唯一标识
title: <标题>               # 人类可读标题
type: <concept|entity|claim|source|relation>
created: <ISO 8601>
updated: <ISO 8601>
updated_by: <agent-id>
confidence: <high|medium|low|disputed>
sources: [<raw/ 路径列表>]
related: [<相关页面 ID 列表>]
tags: [<标签列表>]
status: <active|archived|draft|needs-review>
---
```

### 页面结构模板

```markdown
# <标题>

<一句话定义/摘要>

## 核心要点

- 要点 1
- 要点 2
- 要点 3

## 详细说明

[详细内容]

## 相关

- [[related-page-1]] — 关系说明
- [[related-page-2]] — 关系说明

## 开放问题

- [待解决的问题]

## 变更历史

- <日期>: <变更说明> (<agent-id>)
```

---

## 禁止行为

1. ❌ 修改 raw/ 目录中的任何文件
2. ❌ 创建没有 source 引用的 claim
3. ❌ 将推测写成 high confidence 事实
4. ❌ 删除已有页面（只能标记为 archived）
5. ❌ 修改其他 agent 的 high-confidence claim 核心内容（只能添加证据或标记 disputed）
6. ❌ 在未获取锁的情况下写入文件
7. ❌ 忽略 frontmatter schema 要求
8. ❌ 创建与已有页面重复的新页面（应更新已有页面）

---

## 冲突处理

当你发现与已有知识矛盾的新信息时：

1. **不要直接覆盖**已有 claim
2. 创建新的 claim 页面，标注新证据
3. 将已有 claim 标记为 `status: disputed`
4. 在两个 claim 的 `related` 中互相引用
5. 在 log.md 中记录矛盾发现

---

## 同义词与去重

- 检查 `wiki/_synonyms.md` 中的同义词映射
- 创建新页面前，先搜索是否已有相同概念的页面
- 如果发现重复，使用 merge 而不是创建新页面
- 合并时保留信息更丰富的版本作为 canonical

---

## 质量标准

- 每个页面至少有 1 个入链（被其他页面引用）
- 每个 claim 至少有 1 个 source 引用
- 每个 concept 至少有 3 个核心要点
- index.md 必须包含所有 active 状态的页面
- 所有 [[wiki-links]] 必须指向存在的页面

---

*协议版本：v0.1.0*
