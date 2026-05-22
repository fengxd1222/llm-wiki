---
schema_version: 1.0
agent: hermes
extends: AGENTS.md
---

# HERMES.md — Hermes 专用 Addendum

> 本文件是 [`AGENTS.md`](AGENTS.md) 的 Hermes 专属补充。
> **只加严，不放宽** AGENTS.md 的任何约束。
>
> **注意**：Hermes 没有官方标准化的 instruction 文件协议。本 addendum 是 WikiMind 的产品约定，
> 通过 Hermes 的 skills / memory 机制接入。MVP 中 Hermes 属"适配指南"级别（非 D14 demo 强制验证）。

---

## 1. 角色：Worker Agent

Hermes 是 **worker agent**。Hermes 的特色是围绕**持续学习的 skills + memory**——
这与 WikiMind 的理念契合（都强调知识沉淀），但要注意边界。

---

## 2. 接入方式

Hermes 没有原生 MCP host 能力 → 通过 **CLI bridge** 接入：

```
Hermes skill 调用 → wikimind CLI bridge (JSON-RPC over named pipe / unix socket)
                  → daemon
```

握手等价于调 `wikimind bridge handshake --agent hermes`。

---

## 3. 关键边界：Hermes memory ≠ WikiMind wiki

Hermes 有自己的 memory 系统。**务必区分**：

| Hermes memory | WikiMind wiki |
|---|---|
| Hermes 私有，跟随 Hermes | 共享，多 agent 协作 |
| 无 source 追溯要求 | 每个 claim 必须挂 raw |
| Hermes 自己管理 | review queue + user 把关 |

**绝不**把 Hermes memory 的内容直接当事实 propose 进 wiki。
Hermes memory 里的东西如果要进 wiki，**必须**：
1. 找到它在 `raw/` 中的真实 source
2. 走标准 claim 抽取流程
3. 没有 raw source → 不能 propose（或标 speculation）

---

## 4. 加严项

| 约束 | AGENTS.md 底线 | HERMES.md 加严 |
|---|---|---|
| 知识来源 | raw/ 是 source of truth | **Hermes memory 不算 source**；只有 raw/ 算 |
| Speculation | < 0.3 标 speculation | Hermes 的推断默认 `confidence ≤ 0.7`（因 memory 易引入未追溯内容） |
| Propose 前 | 4 道自检 | + 额外确认："这条 claim 的依据是 raw 文件，不是我的 memory" |

---

## 5. Hermes 适合的任务

- **跨 session 的长期 ingest 任务**——Hermes 的 memory 帮它记住"还有哪些资料没 ingest"
- **query** —— Hermes 可作为 query 入口，但答案的 sediment 走标准流程

---

## 6. 不要做

- ❌ 不要把 Hermes memory 当 source 引用
- ❌ 不要直接写**正式 wiki/**（直接读 raw/wiki 没问题；写经 CLI bridge → `propose_*`）
- ❌ 不要因为"Hermes 记得"就把未追溯内容 propose 为高 confidence claim

---

## 一句话

> Hermes 经 CLI bridge 接入。最大的纪律：Hermes memory 不是 source——只有 raw/ 是。
> Hermes 记得的东西要进 wiki，必须先找到 raw 出处。
