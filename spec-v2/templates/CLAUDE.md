---
schema_version: 1.0
agent: claude-code
extends: AGENTS.md
---

# CLAUDE.md — Claude Code 专用 Addendum

> 本文件是 [`AGENTS.md`](AGENTS.md) 的 Claude Code 专属补充。
> **只加严，不放宽** AGENTS.md 的任何约束。

---

## 1. 角色：可作 Master Agent

Claude Code 可被 user 授权为 **master agent**——拥有比 worker agent 更多的能力：

- 可代理 user 自动 accept 一部分 **白名单内**的 review（见 `docs/review-queue-policy.md §5`）
- 可触发 Dream Cycle（`docs/dream-cycle.md`）
- 但**仍不能改 schema**，**仍不能直接 git commit**

是否启用 master 角色由 user 在 `.wikimind/config.toml` 配置。默认是 worker。

---

## 2. MCP 接入

Claude Code 通过 MCP stdio 连接 WikiMind daemon：

```jsonc
// Claude Code MCP 配置
{
  "mcpServers": {
    "wikimind": {
      "command": "wikimind",
      "args": ["mcp", "serve", "--vault", "/path/to/your-vault"]
    }
  }
}
```

握手后你会拿到 20 个工具（见 `docs/mcp-tools.md`）。读工具带 `readOnlyHint`，Claude Code 会自动
跳过确认。

---

## 3. 加严项

| 约束 | AGENTS.md 底线 | CLAUDE.md 加严 |
|---|---|---|
| Claim confidence | < 0.3 标 speculation | **< 0.6** 即标 speculation（更保守） |
| 单次 ingest propose 数 | 无硬限 | **一次 ingest ≤ 15 个 propose**；超出分批 |
| Quote 长度 | < 30 词 | **< 25 词**，优先精确短引用 |
| 自检 | 4 道 | 4 道 + **额外**：propose claim 前先 `search` 查重，避免建重复 claim |

---

## 4. Claude Code 特有工作流

### 4.1 Plan 优先

Claude Code 有 plan mode。Ingest 大资料前：
1. 先 `read_raw`（normalized）通读
2. 在 plan 中列出"我打算抽哪些 claim / entity / concept"
3. 与 user 对齐后再 propose

### 4.2 用 TodoWrite 跟踪

Ingest 多文件时，用 TodoWrite 列出每个文件的 ingest 进度，让 user 能实时看到。

### 4.3 消化 rejection memory

握手返回 `recent_rejections_summary`。**务必**在开始 propose 前读它，调整策略。
例如摘要说"claim 粒度过细被 reject 3 次" → 这次抽 claim 时更倾向合并。

---

## 5. 不要做

- ❌ 不要用文件工具直接写**正式 wiki/**（直接读 raw/wiki 没问题；写经 worktree → `propose_*`）
- ❌ 不要在 plan mode 之外大批量 propose 而不告知 user
- ❌ 即使是 master agent，也不要 auto-accept 白名单之外的 review

---

## 一句话

> Claude Code 可作 master agent，经 MCP 接入，但加严：更保守的 confidence、单次 ≤ 15 propose、
> propose 前查重。一切仍走 MCP 工具 + review queue。
