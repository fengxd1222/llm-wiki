# Archive — 历史研究方案

> 本目录归档**三套独立的早期研究方案**。它们已被 [`../spec-v2/`](../spec-v2/) 的整合 spec 取代，
> 保留于此仅供**历史追溯、血统对比、设计回溯**。
>
> ⚠️ 这些文件**不再维护**。当前权威文档是 `spec-v2/`。

---

## 三套方案

| 方案 | 文件 | 思维模式 | 一句话定位 |
|---|---|---|---|
| **A — 多文件** | [`proposal-a-multifile/`](proposal-a-multifile/) | 协议优先 | 模块化文档集，协议最严谨，有可复用 templates，21 条风险清单 |
| **B — 单文件** | [`llm-wiki-product-spec.md`](llm-wiki-product-spec.md) | 工程优先 | 66KB 一体化产品文档，Day-by-day 任务表 + Dream Cycle + Query Sedimentation |
| **GPT Pro** | [`gpt-pro-深度研究.md`](gpt-pro-深度研究.md) | 研究综述优先 | 深度研究风格，强引用，CJK tokenizer / git worktree / 平台细节独家 |

三套方案均基于 Karpathy
[LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)。

---

## 为什么保留

1. **血统追溯** —— `spec-v2/SPEC.md §11` 标注了每个设计点来自哪套方案；要核对就回这里看原文。
2. **设计回溯** —— 整合时做的取舍（拿谁的、弃谁的）可对照原方案复盘。
3. **对比基线** —— 未来若 spec 走偏，可回看原始三方案的不同侧重。

---

## 整合去向

`spec-v2/` 的整合策略：

- **以方案 A 为基底** —— 协议骨架、review queue、风险清单、templates
- **吸收方案 B 独家** —— Dream Cycle、Query Sedimentation、Day-by-day roadmap
- **吸收 GPT Pro 独家** —— git worktree per agent、CJK tokenizer、平台细节、MCP `readOnlyHint`
- **新增补三方共同盲点** —— claim 抽取算法、onboarding 剧本、review queue 上限保护、多 agent 冲突剧本、依赖图级联回滚

详见 [`../spec-v2/README.md`](../spec-v2/README.md)。

---

## proposal-a-multifile/ 内部结构

方案 A 是多文件方案，原样保留其目录结构：

```
proposal-a-multifile/
├── README.md            ← 方案 A 自己的 README
├── REPORT.md            ← 方案 A 主报告
├── AGENTS.md            ← 方案 A 的 agent 协议
├── docs/                ← 7 篇设计文档
├── templates/           ← 4 个 agent 指令模板
└── examples/            ← 目录结构样例
```

> 注意：`proposal-a-multifile/` 内部的相对链接在归档后依然有效（整个目录一起移动）。
