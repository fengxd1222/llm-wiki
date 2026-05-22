# LLM Wiki / WikiMind

> 一个 local-first、multi-agent 协作的个人知识库系统的**研究与设计仓库**。
> 基于 Andrej Karpathy 的
> [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)。

本仓库不是代码，是**产品研究 + 工程设计文档** + **UI 原型**。

---

## 目录结构

```
llm-wiki/
├── README.md          ← 你在这里
├── spec-v2/           ★ 当前权威：整合后的统一 spec（24 文件 / 7600+ 行）
├── prototypes/          UI 原型（单页 HTML mockup，v1 + v2）
└── archive/             历史研究方案（三套早期独立方案，已归档，不再维护）
```

| 目录 | 是什么 | 状态 |
|---|---|---|
| **[`spec-v2/`](spec-v2/)** | 整合三套方案 + 补盲点的统一 spec | ✅ 当前权威 |
| **[`prototypes/`](prototypes/)** | 四场景 UI mockup（Dashboard / Review / Wiki / Ingest），Vercel/Geist 风 | ✅ v2 |
| **[`archive/`](archive/)** | 三套早期独立方案（多文件 A / 单文件 B / GPT Pro 深度研究） | 🗄 已归档 |

---

## 怎么读

### 想了解产品方案 → 看 `spec-v2/`

1. [`spec-v2/README.md`](spec-v2/README.md) — 整合说明 + 文档导航
2. [`spec-v2/SPEC.md`](spec-v2/SPEC.md) — 主 spec（定位 / 三层架构 / Claim / 协议 / MVP）
3. `spec-v2/docs/` — 14 篇设计文档
4. `spec-v2/templates/` — 6 个可直接复制到 vault 用的 agent 指令 / schema 模板

### 想看产品长什么样 → 看 `prototypes/`

双击 [`prototypes/wikimind-ui-v2.html`](prototypes/wikimind-ui-v2.html)（浏览器打开）。

### 想追溯设计血统 → 看 `archive/`

三套早期方案的原文。`spec-v2/SPEC.md §11` 标注了每个设计点来自哪套方案。

---

## 一句话定位

**WikiMind 是一个 local-first 的个人 / 小团队知识库。原始资料只读、不可变；多个 agent
（Claude Code / Codex / Hermes / Cursor 等）在一个由 Markdown 组成的 wiki 层上持续读写、
交叉引用、定期 lint，把"一次性对话"沉淀为"可累积、可演化、可追溯的知识图谱"。**

它不是 RAG（wiki 是长期演化的工件，不是查询时拼接的碎片），不是 Obsidian 的替代品，
是一个 **agent + 知识库的协作协议 + 工具链**。

---

## 演进历史

| 日期 | 阶段 |
|---|---|
| 2026-05-20 ~ 21 | 三套独立方案产生（多文件 A / 单文件 B / GPT Pro），现归档于 `archive/` |
| 2026-05-21 | 三方案横向审查；UI 原型 v1 / v2 验证产品形态 |
| 2026-05-21 ~ 22 | 整合三方案 + 补共同盲点 → `spec-v2/`（Wave 1/2/3，24 文件） |

---

## 当前状态

- ✅ 产品研究与设计：`spec-v2/` 完整（含 30 天 MVP roadmap）
- ✅ UI 原型：`prototypes/` v2
- ⬜ 代码实现：未开始（见 `spec-v2/docs/roadmap-30d.md` 的 D1–D30 计划）
