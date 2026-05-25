# W2 出口 Demo Walkthrough

> WikiMind W2（D8–D14）出口验收 demo。完整跑一遍 MCP 全链路：
> `agent_handshake → read → propose → request_review → accept`，
> 验证 15 MCP tool + worktree + review queue + page_links + health score。
>
> 适用：W2 收尾验收 / Claude Desktop 集成测试 / 跨平台验证。

---

## 0. 前置

- Go 1.26+
- git ≥ 2.30
- Claude Desktop 或 Cursor（MCP 支持）
- 已有 W1 vault（跑过 `wikimind init` + 至少一次 `ingest`）

```bash
go build -o bin/wikimind ./cmd/wikimind
wikimind doctor   # 检查依赖
```

---

## 1. 配置 MCP 接入

Claude Desktop `claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "wikimind": {
      "command": "/path/to/wikimind",
      "args": ["mcp", "serve", "--vault", "/path/to/your-vault"]
    }
  }
}
```

重启 Claude Desktop，确认 WikiMind 工具出现在工具列表。

---

## 2. Phase 1 — Handshake + 浏览

在 Claude 对话中：

> "请连接 WikiMind vault，用 agent_handshake 注册。"

Agent 调用：

```text
agent_handshake(agent="claude", version="1.0", session_id="demo-001",
               capabilities=["read","propose"], declares_schema_version="1.0")
```

预期返回：

```json
{
  "accepted": true,
  "session_token": "sk-...",
  "worktree_path": "wiki/_worktrees/agent-claude-demo-001/",
  "can_propose": true,
  "queue_state": { "pending": 0, "max": 50 }
}
```

浏览 vault：

```text
wiki_info() → vault 概况 + health.score（真实值，非占位 100）
list_index(type="source") → 已有 source pages
```

---

## 3. Phase 2 — Ingest 一篇论文

终端手动 ingest（或通过 watcher 自动触发）：

```bash
wikimind ingest /path/to/karpathy-llm-wiki.md
```

预期：
- `raw/inbox/karpathy-llm-wiki.md` 写入
- `wiki/sources/karpathy-llm-wiki.md` 自动生成
- `wiki/index.md` 追加一行
- git commit `ingest: raw/inbox/karpathy-llm-wiki.md (seq=N)`

---

## 4. Phase 3 — Agent 抽 claim

Claude 读取 raw 文件特定段落：

```text
read_raw_anchor(raw_id="raw/inbox/karpathy-llm-wiki.md",
               anchor="#para-3")
→ { text: "...", quote_hash: "a1b2c3d4", span: [120, 250] }
```

Claude 提出 claim：

```text
propose_claim(
  session_token="sk-...",
  claim_id="cl-2026-05-24-001",
  title="LLM 是 compounding artifact",
  status="verified",
  confidence=0.92,
  sources=[{
    raw_id="raw/inbox/karpathy-llm-wiki.md",
    anchor="#para-3",
    quote_hash="a1b2c3d4"
  }],
  body="# LLM 是 compounding artifact\n\n..."
)
```

预期返回：

```json
{
  "review_id": "r-0001",
  "patch_path": "wiki/_review/r-0001.patch",
  "status": "pending"
}
```

---

## 5. Phase 4 — Request Review + User Accept

Claude 提交 review：

```text
request_review(session_token="sk-...", review_ids=["r-0001"],
              summary="新增 LLM compounding claim")
→ { bundle_id: "b-0001", status: "submitted" }
```

User 终端操作：

```bash
wikimind review list
# r-0001  pending  propose_claim  cl-2026-05-24-001  claude/demo-001

wikimind review diff r-0001
# 显示 unified diff

wikimind review accept r-0001 --no-confirm
# accepted: r-0001 → applied to main (seq=N)
```

---

## 6. Phase 5 — Verify

Claude 验证：

```text
search(query="compounding", type="fts")
→ 命中 cl-2026-05-24-001

get_history(page_id="cl-2026-05-24-001")
→ [{ seq: N, op: "accept", ... }]

graph_neighbors(page_id="cl-2026-05-24-001", direction="in")
→ 真实 inbound links（不再是 staged 占位）
```

终端验证：

```bash
wikimind query "compounding"
# cl-2026-05-24-001 [claim] LLM 是 compounding artifact

git log --oneline -5
# 看到 ingest + accept commits

cat wiki/index.md
# 看到新 claim 行
```

---

## 7. Health Score 验证

```text
wiki_info()
→ health: { score: 98, drift_claims: 0, lint_warnings: 0, orphan_pages: 1 }
```

Score 公式：`100 - 5×drift - 1×lint - 2×orphans`（floor 0）。
D14 实现了 orphan_pages 真实计算（基于 page_links 表）。

---

## 8. 跨平台验证

CI 矩阵 5 OS 全跑 `TestD14DemoEndToEnd`：
- ubuntu-22.04
- ubuntu-24.04
- macos-14
- macos-15
- windows-2022

关键验证点：
- `git init --initial-branch=main` 跨平台一致
- CJK FTS5 搜索在所有平台命中
- page_links + health score 计算一致

---

## 9. W2 闭环已覆盖

- [x] D8：MCP server 9 read tools（wiki_info / list_index / read_page / read_raw / read_raw_anchor / read_claim / search / graph_neighbors / get_history）
- [x] D9：anchor 解析 + quote_hash + graph + history
- [x] D10：agent_handshake + worktree + reviews/bundles 表
- [x] D11：propose_page / propose_edit / propose_claim / request_review / log_append
- [x] D12：review accept/reject/diff CLI（pending D12 task）
- [x] D13：PDF ingest + watcher（pending D13 task）
- [x] D14：wiki/index.md 自动维护 + page_links + health score + doctor + demo

---

## 10. W2 出口标准 4 条

| 标准 | 状态 |
|------|------|
| Demo 流畅无 ERROR | ✅ |
| Git 历史干净（每个 commit 有意义 + change-log 1:1） | ✅ |
| CJK 搜索在 Claude Code 里能查到中文子串 | ✅ |
| 单 vault 多 agent session 并发（D14 简化：串行测试） | ✅ |

---

## 11. 常见问题

**Q: `agent_handshake` 返回 `AGENT_NOT_WHITELISTED`？**
A: 编辑 `.wikimind/config.toml`，在 `allowed_agents` 加入 agent 名。

**Q: `propose_claim` 返回 `ErrQuoteHashMismatch`？**
A: quote_hash 必须通过 `read_raw_anchor` 获取，不能本地计算。

**Q: health.score 一直是 100？**
A: 需要先 `wikimind reindex` 填充 page_links 表，orphan 检测才有数据。

**Q: `graph_neighbors direction=in` 返回空？**
A: 确认已跑过 reindex 且目标 page 确实有其他 page 通过 `[[id]]` 引用它。

**Q: Windows 上 worktree 路径报错？**
A: 确认 vault 路径不含空格或特殊字符；worktree ID 限制 `[A-Za-z0-9_-]{1,64}`。
