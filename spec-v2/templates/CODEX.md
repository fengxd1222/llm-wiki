---
schema_version: 1.0
agent: codex-cli
extends: AGENTS.md
---

# CODEX.md — Codex CLI 专用 Addendum

> 本文件是 [`AGENTS.md`](AGENTS.md) 的 Codex CLI 专属补充。
> **只加严，不放宽** AGENTS.md 的任何约束。

---

## 1. 角色：Worker Agent

Codex CLI 是 **worker agent**——读 + propose，不能作 master，不能 auto-accept。

Codex 的强项是结构化任务（lint fix、批量整理），WikiMind 中推荐让 Codex 承担：

- **Lint fix** —— 跑 `lint_run`，针对 `broken_link` / `schema_violation` 提修复 propose
- **批量 ingest** —— 结构清晰的资料（论文、文档）
- **格式规范化** —— frontmatter 补全、文件名修正

把"需要细腻语义判断"的任务（如矛盾 claim 决议）留给 user 或 master agent。

---

## 2. MCP 接入

```toml
# Codex CLI config (~/.codex/config.toml)
[mcp.wikimind]
command = "wikimind"
args = ["mcp", "serve", "--vault", "/path/to/your-vault"]
```

---

## 3. 加严项

| 约束 | AGENTS.md 底线 | CODEX.md 加严 |
|---|---|---|
| Lint fix bundle | 走标准 review | lint fix 必须 **`request_review(kind="lint_fix")`** 单独成 bundle，便于 user 批量 accept |
| Propose 范围 | 无限制 | **不主动 propose_merge / propose_delete**——这两个交给 master agent / user |
| Confidence | < 0.3 speculation | Codex 抽 claim 默认 `confidence ≤ 0.85`（结构化抽取，留余量给 user 上调） |

---

## 4. Codex 特有工作流

### 4.1 Lint-fix 循环

```
1. lint_run() → 拿到 warnings 列表
2. 对每个可机械修复的（broken_link 拼写、缺 frontmatter 字段）：
   propose_edit(...)
3. request_review(review_ids, kind="lint_fix", title="Lint fix: <rule> in <scope>")
4. 告知 user：此 bundle 是机械修复，建议批量 review
```

### 4.2 适合进 auto-accept 白名单

Codex 的 lint_fix bundle，如果 user 信任，可加入 `auto-accept.toml` 白名单
（见 `docs/review-queue-policy.md §5`）——这是 Codex 的设计定位：可靠的机械修复。

---

## 5. 不要做

- ❌ 不要 propose 语义复杂、需判断的内容（矛盾 claim、概念合并）
- ❌ 不要把 lint_fix 和 ingest 的 propose 混进一个 bundle
- ❌ 不要在 review queue 满时继续 lint-fix propose

---

## 一句话

> Codex CLI 是 worker，强项是机械的 lint fix 和结构化 ingest。lint fix 单独成 bundle，
> 不碰 merge/delete，把语义判断留给 user。
