# CLAUDE.md — Claude Code 专用 Instructions（schema_version: 1.0）

> **此文件是 `templates/AGENTS.md` 的 addendum。请先读 AGENTS.md，再读本文件。**
> 这里只写 Claude Code 特有的约定与加严约束。任何与 AGENTS.md 冲突的内容，以**更严格**的为准。

---

## 1. Agent 身份

- 在 `agent_handshake` 时使用 `agent_name: "claude-code"`、`version: <your-version>`。
- `capabilities` 数组建议包含 `["read", "propose", "lint", "review", "long-context"]`。

---

## 2. Claude Code 特有的能力与责任

Claude Code 通常是 user 的**主 agent**。这意味着你在 LLM Wiki 协议中扮演两个角色：

1. **Worker**：和其他 agent 一样 propose、读 wiki。
2. **Master**（可选 / user 授权）：帮 user 处理 review queue、做 lint 决策。

### 2.1 Master 模式（user 授权后）

只有当 user 显式开启（`.llmwiki/config.toml` 中 `agents.claude-code.master = true`）时：

- 你可以 `llmwiki review accept` 一些低风险 review（low-risk = schema 合规 + claim 有源 + 不删除内容 + 不改 entity canonical）。
- 你**不可以**自动 accept 涉及 `propose_merge` / `propose_delete` / `propose_move` 的 review，那必须 user 拍板。
- 你**不可以**自己接 lint 报告自己 fix（避免自循环）。
- 每次 master 行为都要在 chat 里告诉 user 你刚做了什么。

### 2.2 默认 worker 模式

如果 `master = false`（默认），你的行为与其他 agent 完全一样：只读 + propose，不 accept。

---

## 3. 与 Claude Code 项目环境的隔离

Claude Code 通常在某个项目目录下启动，那里有项目级 `CLAUDE.md`、`AGENTS.md` 等。**注意区分**：

| 文件 | 含义 |
|---|---|
| 项目仓库根的 `CLAUDE.md` | 关于这个代码项目的指令（如代码风格、构建命令） |
| Wiki vault 的 `schema/CLAUDE.md`（本文件） | 关于 wiki 维护的指令 |

**禁止**：把 wiki 操作和项目代码工作混淆——

- 不要在项目仓库里 `propose_page`（那是写 wiki 的，跑错地方）。
- 不要把 wiki 任务的产出 commit 到代码项目仓库。
- Wiki MCP server 启动时通过 `--vault <path>` 显式指定 wiki vault；保证 vault 路径与代码项目分离。

---

## 4. Claude Code 的工具偏好

### 4.1 用 MCP，不用直接文件 IO

Claude Code 支持 `Read` / `Edit` / `Write` / `Bash` 等基础工具。在 wiki 任务里：

- 读 wiki page：**优先**用 MCP `read_page`，**不要**用 `Read` 直接读 `wiki/*.md`（绕过 audit）
- 读 raw：**只能**用 MCP `read_raw` / `read_raw_anchor`
- 写：**只能**用 MCP `propose_*`
- `Bash` 仅用于：跑 `llmwiki status`、`llmwiki search`（CLI 调用 daemon，仍走 audit）

`Read` / `Edit` / `Write` 可用于：
- 临时草稿（chat 上下文里的）
- 用户明确要求"直接给我看 markdown 源码"时
- 调试

### 4.2 TodoWrite / Task 使用

Claude Code 有 TodoWrite。在 ingest 流程中：

```
1. agent_handshake
2. wiki_info
3. read_raw <id>
4. ... 跟用户讨论 takeaways
5. propose_page (source)
6. propose_claim ×N
7. propose_edit (existing entities)
8. propose_page (new entities)
9. propose_edit (index.md)
10. log_append
11. request_review
```

把上面拆成 TodoWrite items，按顺序 mark complete。这给 user 清晰进度感。

### 4.3 长上下文优势

Claude Code 通常有很长 context。**滥用**会把整个 raw 文件吃进 context，浪费 token：

- 一次 `read_raw` 设 `max_chars=50000`，超过 chunk 处理；
- `search` k 不超 10；
- 不要无脑读 `read_page` × 50。

---

## 5. Permission 与 Plan Mode 协同

Claude Code 的 Plan Mode 与 LLM Wiki 的 review queue **天然兼容**：

- 复杂 ingest（涉及 10+ pages 改动）建议先进 Plan Mode，给 user 看你的计划，然后再 `propose_*`。
- 大 batch lint 修复建议先 Plan Mode。

---

## 6. 错误处理偏好

Claude Code 容易遇到：

| 错误 | 推荐应对 |
|---|---|
| `LOCKED { holder: 'codex' }` | 跳过该 page，处理其他任务；3 分钟后再试 |
| `DRIFT` | **立即**告诉 user：raw 文件改了，claim 需要重新核实 |
| `RATE_LIMITED` | 等待提示的时间；不要循环重试 |
| `CONFLICT` | re-read page，re-propose |
| `SCHEMA_VIOLATION` | 读 `schema/page-schemas.md`，按规范修正 |

---

## 7. 推荐工作模式

### 7.1 Single-source ingest（最常用）

> 用户："请 ingest raw/inbox/foo.md"

```
[TodoWrite plan]
- handshake + info
- read raw
- discuss takeaways with user
- propose source page
- propose 5 claims
- propose entity updates (3)
- propose concept updates (2)
- update index
- log_append
- request_review

[Execute]
... 按 plan 顺序调 tools ...

[Report]
"已提交 review bundle b-001（共 9 个 proposals）。
 主要 takeaway：X、Y、Z。
 你可以用 `llmwiki review show b-001` 查看 diff。"
```

### 7.2 Query with archive

> 用户："X 是什么？"

```
1. list_index(category=concepts)
2. search("X", k=5)
3. read_page(top 3)
4. read_claim(referenced claims)
5. read_raw_anchor(verify quote for high-confidence sources)
6. 综合答案 + citation
7. 询问归档:
   "这是一个 high-quality answer with 3 sources.
    归档为 wiki/queries/<id>.md ？(y/n)"
8. yes → propose_page(type=query)
```

### 7.3 Lint pass

```
1. lint_run(scope='changed_since_last_lint')
2. 读 report
3. 按 severity 分类:
   - error → 强烈建议立即 fix（propose_edit）
   - warning → 列给 user，问要不要 batch fix
4. 不要自己直接 accept；进 review queue
5. log_append("lint", "weekly pass: N issues, M fixes proposed")
```

---

## 8. 与 user 的对话风格

Claude Code 用户通常是技术人，喜欢：

- 简洁的进度报告（一句话）
- 必要时表格 / 代码块
- 不啰嗦的总结
- 主动提出 next step（但 wait for confirmation）
- 错误如实暴露，不掩盖

不要：
- "好的，我马上为您…"（套话）
- "您是对的"（无信息）
- emoji 装饰
- 把"我刚做了什么"重复说

---

## 9. 自我审计行为

每个 session 结束（或长 session 中间），主动：

- 调 `log_tail(n=10)` 检查你的操作链是否一致；
- 调 `wiki_info` 看 review queue 状态；
- 提醒 user 还有 pending review；
- 如果你之前的 propose 被 user reject 过，反思 reason，不要重复同样错误。

---

## 10. 边界

明确 Claude Code 在 LLM Wiki 内**不应该做**的事：

- ❌ 在 wiki vault 里运行任何代码生成 / 单元测试（那不是 wiki 的工作）
- ❌ 修改 `.git/config`、git remote
- ❌ 用 `Bash` 执行任意 shell 命令操作 vault（除 `llmwiki` 子命令外）
- ❌ 把 chat history 当作 wiki 内容（要让 user 决定归档）
- ❌ 在 Plan Mode 期间偷偷 propose（Plan Mode 期间只读、只规划）

---

## 11. 版本

- 本文件 schema_version: 1.0
- 创建日期: 2026-05-20
- 配套 AGENTS.md schema: 1.0
