---
schema_version: 1.0
agent: cursor
extends: AGENTS.md
---

# CURSOR.md — Cursor 专用 Addendum

> 本文件是 [`AGENTS.md`](AGENTS.md) 的 Cursor 专属补充。
> **只加严，不放宽** AGENTS.md 的任何约束。
>
> MVP 中 Cursor 属"适配指南"级别（非 D14 demo 强制验证）。

---

## 1. 角色：Worker Agent

Cursor 是 **worker agent**。Cursor 是 IDE，user 常在里面同时做别的事——
所以 Cursor 接入 WikiMind 时要特别注意**不干扰 user 的主工作流**。

---

## 2. 接入方式

Cursor 支持 MCP + 支持 AGENTS.md / Project Rules。两种方式：

### 2.1 MCP（推荐）

```jsonc
// .cursor/mcp.json
{
  "mcpServers": {
    "wikimind": {
      "command": "wikimind",
      "args": ["mcp", "serve", "--vault", "/path/to/your-vault"]
    }
  }
}
```

### 2.2 Project Rules

把本文件 + `AGENTS.md` 放进 `.cursor/rules/`，Cursor 会作为 context 加载。

---

## 3. 加严项

| 约束 | AGENTS.md 底线 | CURSOR.md 加严 |
|---|---|---|
| Propose 时机 | 随时 | **不在 user 正在 IDE 里编辑代码时**主动 propose；等 user 显式触发 |
| 主动性 | agent 可主动 ingest | Cursor **不主动** ingest——只在 user 明确要求时做 |
| Query | 自由 | Cursor 的 query 优先用于"辅助 user 当前工作"，sediment 默认 `--no-sediment`（IDE 里的查询多是临时的） |

---

## 4. Cursor 特有约定

### 4.1 Cursor 是"查询入口"多于"维护者"

User 在 Cursor 里写代码 / 文档时，常会问"我们的 wiki 里关于 X 怎么说"。
Cursor 在 WikiMind 中的主要价值是 **query**——把 wiki 作为 user 的"第二大脑"随手查。

维护类工作（ingest、lint fix）更适合 Claude Code / Codex。

### 4.2 显式触发才 propose

只有 user 明确说"把这个 ingest 进 wiki" / "建个 claim" 时，Cursor 才 propose。
不要因为"看到一份资料"就自动 ingest。

### 4.3 Sediment 默认关

Cursor 的 query 默认 `--no-sediment`——IDE 里的查询多是临时的、辅助当前任务的，
不该都沉淀。User 明确说"这个答案有价值，存下来"时才 sediment。

---

## 5. 不要做

- ❌ 不要在 user 编码时弹出 propose 打断
- ❌ 不要自动 ingest user 打开的任意文件
- ❌ 不要默认 sediment 每个 query

---

## 一句话

> Cursor 是 worker，在 WikiMind 里主要当"查询入口"——辅助 user 当前工作。维护类工作交给
> Claude Code / Codex。只在 user 显式触发时 propose，不打断编码流。
