# Agent Protocol

> 多 agent 协作的细节协议：handshake、schema 版本、worktree 隔离、advisory lock、review 状态机、
> change log 一一对应、conflict 边界。
>
> 解决一个工程问题：**Claude Code、Codex、Hermes、OpenCode、Cursor、Cline、自研 agent、CLI 工具
> 等多个 agent，如何共同维护同一个 wiki 而不互相打架、不污染知识、不丢失追溯。**

---

## 1. 角色与边界

| 角色 | 身份 | 能力 | 限制 |
|---|---|---|---|
| **User** | 唯一最终决策者 | 任意；所有 review 由 user 拍板（或显式委托） | — |
| **Master agent**（可选） | 用户委托的"主 agent"，如 Claude Code | 被授权 auto-accept 一部分白名单 review；可代理 user 部分操作 | 不可改 schema |
| **Worker agent** | Codex / Hermes / Cursor / Cline / 自研 | 读 + propose，**不能直接写正式 wiki** | rate limit + schema 校验 |
| **Daemon** | `wikimindd` 系统进程 | 唯一执行 git commit；唯一发锁；唯一维护 `.wikimind/` 状态 | 不主动产生内容 |
| **Bridge** | `wikimind-bridge` 文件访问通道 | 读 raw / wiki；不参与决策 | 路径白名单 |

### 1.1 协议关键不变量

> 1. **只有 daemon 能 `git commit` 正式 wiki 分支** (`refs/heads/main`)。
> 2. **任何 agent 写入都先到 `wiki/_review/`**（通过自己的 worktree 提交，daemon 把 worktree diff
>    转成 patch）。
> 3. **任何 agent session 开始必须 `agent_handshake`**，否则 daemon 拒绝所有写工具。
> 4. **每条 change log 与 git commit 一一对应**——不存在"未记账"的写入。
> 5. **schema 文件由 user 维护**，agent 不能写 schema/。
> 6. **冲突场景 5 个剧本**预定义（详见 [`conflict-scenarios.md`](conflict-scenarios.md) Wave 3）。

---

## 2. Schema 是合同

### 2.1 三层 instruction 文件

```
schema/
├── AGENTS.md            ← baseline rules，所有 agent 共同遵守
├── CLAUDE.md            ← Claude Code 专用 addendum
├── CODEX.md             ← Codex CLI 专用
├── HERMES.md
├── CURSOR.md
├── lint-rules.md        ← lint 规则定义
└── page-schemas.md      ← 各类 page 的 frontmatter schema
```

**强约束**：

- `AGENTS.md` 是**底线**——任何 agent 都必须读、必须遵守。
- 各 agent 的专属文件**只能加严，不能放宽** `AGENTS.md` 里的约束。
  - 例：`AGENTS.md` 说"confidence < 0.7 必须标 unverified"，`CLAUDE.md` 不能说"低 conf 可直接 supported"。
  - 例：`AGENTS.md` 说"raw 只读"，`CLAUDE.md` 不能加例外。
- Schema 文件由 user 维护、进 git；每次 schema 变更触发 lint 全扫。

### 2.2 Schema 版本号

`schema/AGENTS.md` 顶部强制：

```yaml
---
schema_version: 1.0
last_updated: 2026-05-21
breaking_changes:
  - "1.0: initial release"
compatible_agent_versions:
  claude-code: ">= 0.5.0"
  codex-cli: ">= 1.2.0"
  hermes: ">= 0.3.0"
  cursor: ">= 1.0.0"
---
```

`agent_handshake` 协议（详见 §3）会比对 `schema_version`，不兼容 → daemon 拒绝写工具。

### 2.3 Schema 变更政策

| 变更类型 | 处理 |
|---|---|
| **Patch**（1.0 → 1.0.1） | 拼写/澄清；不影响现有 wiki；无需 migration |
| **Minor**（1.0 → 1.1） | 加新字段（必须有 default）；老 page 用 default；不阻断 agent |
| **Major**（1.0 → 2.0） | Breaking change；写 migration 脚本；老 agent 自动 downgrade 或拒绝 |

Migration 脚本格式见 [`failure-playbook.md`](failure-playbook.md) Wave 3。

---

## 3. Agent Handshake

### 3.1 协议

```
agent → daemon (via MCP):
  agent_handshake({
    agent: "claude-code",
    version: "0.7.2",
    session_id: "sess-A1",         # 客户端生成 UUID
    capabilities: ["read", "propose", "lint"],
    declares_schema_version: "1.0"
  })

daemon → agent:
  {
    accepted: true,
    daemon_schema_version: "1.0",
    worktree: "wiki/_worktrees/agent-claude-sess-A1/",
    instructions: [
      "schema/AGENTS.md",
      "schema/CLAUDE.md"
    ],
    session_token: "sk-abc...",     # 后续每次 MCP 请求附带
    rate_limits: {
      propose_per_minute: 30,
      query_per_minute: 60
    },
    queue_state: {
      pending: 12,
      hard_limit: 50,
      can_propose: true
    }
  }
```

### 3.2 失败处理

| 失败原因 | 错误码 | 处理 |
|---|---|---|
| Schema 版本不兼容（agent < daemon major） | `SCHEMA_INCOMPATIBLE` | agent 必须升级；daemon 拒绝任何写工具，可继续 read |
| Agent 未在白名单 | `AGENT_NOT_WHITELISTED` | daemon 完全拒绝（user 需在 `.wikimind/config.toml` `allowed_agents` 加入） |
| 已有 session_id 在用 | `SESSION_EXISTS` | agent 重新生成 session_id 重试 |
| Queue 满 | `QUEUE_FULL` | daemon 接受 handshake（read 可用），但 `can_propose: false` |

### 3.3 Session 生命周期

```
handshake → active → idle (60min) → expired (auto-clean worktree)
              ↓        ↓
           explicit close / agent crash / user kill
```

- 每个 session 关联一个 worktree
- session 结束（explicit close / TTL expired / agent process killed）→ daemon 清理 worktree
- worktree 中**未 propose 的修改丢失**（agent 责任：要么 propose 要么丢）

---

## 4. Worktree 隔离

### 4.1 Worktree 物理结构

```
vault/
├── .git/
└── wiki/
    ├── (main branch checkout 给 daemon 用)
    └── _worktrees/                ← 每个 agent 一个子目录
        ├── agent-claude-sess-A1/   ← branch: wt-claude-sess-A1
        │   ├── claims/
        │   ├── entities/
        │   └── ...                 ← 完整 wiki/ 的副本（共享 .git/）
        ├── agent-codex-sess-B2/
        └── ...
```

`git worktree add` 让多个分支同时 checkout 到不同目录，共享 `.git/`，磁盘开销极小。

### 4.2 Agent 在 worktree 中能做什么

| 操作 | 允许？ |
|---|---|
| 读 worktree 中任何文件 | ✅ |
| 写 / 改 worktree 中 wiki/ 文件 | ✅（但仍需 propose 才能进 main） |
| 写 worktree 中 schema/ 文件 | ❌（schema 由 user 维护，agent 不能改） |
| Commit 到自己的 worktree branch | ✅（daemon 不阻止，但只在 propose 时取 diff） |
| Push 到 remote | ❌（worktree branch 不允许 push） |
| Pull 别人的 worktree | ❌（agent 间不直接通信） |
| 读 raw/ | ✅ |
| 写 raw/ | ❌（任何尝试都被 daemon 拒绝） |

### 4.3 Worktree → Main 的合并路径

```
agent in worktree:
  edit wiki/claims/foo.md
  call propose_edit(...)  ← 通过 MCP

daemon:
  1. git diff wt-claude-sess-A1 main -- wiki/claims/foo.md
  2. patch = 生成的 diff
  3. validate (schema, quote_hash, provenance_depth)
  4. write patch to wiki/_review/r-{seq}.patch
  5. add to review queue

user accept r-{seq}:
  daemon:
    1. apply patch to main branch
    2. git add + commit
    3. (optional) update agent worktree: git pull or auto-rebase
    4. clear patch from _review/
    5. write change_log
```

### 4.4 多 agent 并发的物理图

```
agent A worktree (wt-A)        agent B worktree (wt-B)
        edit foo.md                    edit foo.md
            ↓                              ↓
       propose r-0100                  propose r-0101
            ↓                              ↓
       _review/r-0100.patch             _review/r-0101.patch
            \                          /
             \                        /
              ▼                      ▼
                Review queue
              "r-0100 and r-0101 both touch foo.md"
                       ↓
                user decides (sees both diffs, picks one or merge manually)
```

物理上两个 agent 的修改**永不直接冲突**（在各自 worktree 里）。
逻辑冲突由 review queue 暴露，由 user 决议。详见 [`conflict-scenarios.md`](conflict-scenarios.md)。

---

## 5. Advisory Lock

### 5.1 何时需要 lock

Worktree 物理隔离已经避免了"互相覆盖"。Lock 解决的是**协调问题**：

- agent A 正在大改 `claims/foo.md`（多次 propose 累积），不希望 agent B 同时也 propose 它
- user 正在手动改 `concepts/bar.md`，不希望 agent 写

Lock = **agent 自愿遵守的协调机制**，daemon 不阻止物理操作，只在 propose 时检查。

### 5.2 Lock API

```
acquire_lock({
  target: "wiki/claims/foo.md",
  holder: "claude-code:sess-A1",
  ttl_seconds: 600,                 # max 30 min
  purpose: "refactoring source citations"
})
→ { lock_id: "lk-001", expires_at: ... }
→ or { error: "ALREADY_LOCKED", holder: "codex-cli:sess-B2", expires_at: ... }

release_lock({ lock_id: "lk-001" })
→ { released: true }

# Daemon 行为：
# - propose_* 检查 target 是否被 lock；如被他人 lock → 拒绝 propose
# - lock 过期自动清理
# - User 可强制 break：wikimind lock break <target>
```

### 5.3 Lock 与 worktree 的关系

| 场景 | 行为 |
|---|---|
| agent A acquire lock on foo.md | agent B 仍可在自己 worktree 中编辑 foo.md（物理无阻），但 `propose_edit(foo.md)` 会被 daemon 拒 |
| agent A 持锁但崩溃 | TTL 过期后自动释放；其它 agent 收到 `LOCK_EXPIRED` 错误码 |
| agent A 持锁，user 手动改 foo.md | watcher 报告变化；user 操作不受 lock 约束（user > agent）；agent A 后续 propose 会得到 `BASE_HASH_MISMATCH` 错误 |

### 5.4 Lock 失败模式

| 失败 | 缓解 |
|---|---|
| Agent 死锁（互等对方） | 单 agent 同时只能持 ≤ 5 个 lock；超出强制释放 |
| Lock holder 不释放 | TTL 强制（max 30 min） |
| 多 user 时锁混乱 | MVP 单 user；v0.2 加 user 维度 |

---

## 6. Review State Machine

### 6.1 状态

```
            ┌──────────────┐
            │   pending    │
            └──────┬───────┘
                   ↓
        ┌──────────┼──────────┐
        ▼          ▼          ▼
    accepted   rejected   superseded
        │
        ▼
     merged
```

| 状态 | 含义 | 触发 |
|---|---|---|
| pending | 等 user / master agent 决议 | agent `propose_*` |
| accepted | user 同意，等 daemon merge | `wikimind review accept r-...` |
| rejected | 拒绝，留档 | `wikimind review reject r-... "reason"` |
| superseded | 同 page 有新提议覆盖；旧的自动作废 | 自动 |
| merged | 已写入正式 wiki + git + change_log | daemon 执行后 |

### 6.2 一条 review 的生命周期

```
T0   Claude Code 读了一篇论文：
       propose_page(claims/wiki-is-compounding.md, ...)  → r-0001 pending
       propose_edit(entities/karpathy.md, ...)            → r-0002 pending
       propose_claim(...)                                 → r-0003 pending
       request_review([r-0001, r-0002, r-0003],
                      title="ingest karpathy 2026")        → bundle b-0001

T1   user 看 bundle b-0001:
       wikimind review show b-0001      # 综合 diff
       wikimind review accept b-0001    # 一键接受整个 bundle

T2   daemon:
       - 把 r-0001, r-0002, r-0003 的 patch 按拓扑序应用到 main
       - 写 change_log（seq 推进）
       - git commit + 一次 commit
       - update index.db
       - delete drafts from wiki/_review/
       - 释放 agent 持有的 lock（如有）
```

### 6.3 Bundle vs 单条

详见 [`review-queue-policy.md §4`](review-queue-policy.md#4-bundle-归并)。

### 6.4 Conflict 处理

两个 agent 同时提议改同一个 page：

| 场景 | daemon 行为 |
|---|---|
| 后到 propose，前一个有 lock | 后者拒绝 → `LOCKED` 错误 |
| 后到 propose，前一个 pending（无 lock） | 后者接受 → review queue 中暴露为"两份冲突 review" |
| 后到 propose，base_hash 与当前 main 不一致 | 拒绝 → `CONFLICT` 错误；agent 必须 re-read page → re-propose |
| 同一 agent 同 page 多次 propose | 旧的自动 superseded，仅保留最新 |

User 视角：review queue 里看到两个相互冲突的 r-id，UI 提示"互斥，选一个 / 合并"。

---

## 7. Change Log（审计真理源）

### 7.1 两份 change log

| 文件 | 格式 | 受众 |
|---|---|---|
| `wiki/log.md` | Markdown，append-only | **human-readable**，user 浏览 |
| `.wikimind/change-log.jsonl` | JSONL，append-only | **machine-readable**，daemon / lint / 备份 |

两者**1:1 对应 git commit**：每个 commit 必有 1 行 log.md + 1 行 change-log.jsonl。

### 7.2 change-log.jsonl 格式

```jsonl
{"seq": 1, "git_sha": "a92d445", "ts": "2026-05-21T10:18:00Z", "actor": "user", "op": "accept", "bundle": "b-0001", "reviews": ["r-0001", "r-0002", "r-0003"], "summary": "initial ingest from karpathy gist"}
{"seq": 2, "git_sha": "b1f3008", "ts": "2026-05-21T10:32:00Z", "actor": "claude-code", "op": "auto-accept", "rule": "lint_broken_link_fix", "bundle": "b-0002", "reviews": ["r-0004"], "summary": "fix broken link in concepts/source-of-truth.md"}
```

### 7.3 Append-only 强制

- `change-log.jsonl` 任何 line 一旦写入**不可修改**
- 唯一可执行的"修改"是 `revert` 操作 → 写新一条 `op: revert` 的 log（reverse semantics）
- 如发现历史 line 被外部改动 → daemon 启动时 integrity check 报错，要求 user 决定是否 trust git

### 7.4 log.md 格式

```markdown
# Wiki Change Log

| seq | ts | actor | op | summary |
|-----|----|-------|-----|---------|
| 1 | 2026-05-21 10:18 | you | accept | b-0001: initial ingest from karpathy gist (3 reviews) |
| 2 | 2026-05-21 10:32 | claude-code (auto, rule=lint_broken_link_fix) | auto-accept | b-0002: fix broken link in concepts/source-of-truth.md |
| ... |
```

User 可在 Obsidian / VS Code 直接看，配 dataview 等插件做 query。

---

## 8. Rate Limits

防止 agent 失控：

| 维度 | 限制 |
|---|---|
| propose_* per agent per minute | 30（可配） |
| query per agent per minute | 60 |
| lock count per agent | 5 simultaneously |
| worktree disk size | < 100 MB per agent |
| concurrent agents | 5 |

超限 → 返回 `RATE_LIMITED` 错误，附带 retry-after。

---

## 9. Rejection Memory

User reject 的 propose 都被记录，agent session 启动时读最近 N 条作为"避坑指南"：

```
.wikimind/rejections.jsonl
{"review_id": "r-0247", "agent": "claude-code", "page": "claims/index-md-read-first.md", "reason": "quote 不在原文，agent 编的", "ts": "2026-05-21T10:35:00Z"}
```

`agent_handshake` 响应中包含：

```
{
  ...
  recent_rejections_summary: "Top reasons in last 7 days:
    - quote_hash mismatch (5)
    - claim 粒度过细 (3)
    - duplicate of existing claim (2)
   Suggest: re-read schema/CLAUDE.md sections 3.2, 4.1 before propose"
}
```

agent prompt 应消化这个摘要，调整后续行为。详见 [`templates/CLAUDE.md`](../templates/CLAUDE.md) Wave 3。

---

## 10. 协议错误码

| Code | 含义 | 通常处理 |
|---|---|---|
| `OK` | 成功 | — |
| `SCHEMA_INCOMPATIBLE` | schema 版本不兼容 | agent 升级 |
| `AGENT_NOT_WHITELISTED` | agent 未在 allowed_agents | user 加白名单 |
| `SESSION_REQUIRED` | 没 handshake | 先 handshake |
| `RATE_LIMITED` | 速率超限 | retry-after |
| `QUEUE_FULL` | review queue 满 | user 清理 backlog |
| `LOCKED` | target 被其它 agent lock | 等待 / 协调 |
| `LOCK_EXPIRED` | 持有的 lock 已过期 | 重新 acquire |
| `BASE_HASH_MISMATCH` | propose 基于陈旧 base | re-read + re-propose |
| `QUOTE_HASH_MISMATCH` | source 已变更 | re-verify source |
| `SCHEMA_VIOLATION` | propose 不符合 page schema | 修 propose 内容 |
| `PROVENANCE_DEPTH_EXCEEDED` | claim 引用了另一个 wiki claim 而非 raw | 重新挂 raw source |
| `CONFLICT` | 与已有内容冲突 | review queue 暴露 |
| `PATH_NOT_ALLOWED` | 试图写禁止路径 | 协议错误 |
| `INTERNAL_ERROR` | daemon 内部错误 | 报 bug |

每个错误返回结构：

```json
{
  "code": "QUOTE_HASH_MISMATCH",
  "message": "Source raw/inbox/karpathy-llm-wiki.md has been modified since claim was created",
  "details": {
    "stored_hash": "a7f2e3c1",
    "current_hash": "8f3c1d44",
    "source_mtime_changed_at": "2026-05-21T14:22:00Z"
  },
  "suggested_action": "Call read_raw_anchor() to fetch fresh content, then re-propose"
}
```

---

## 11. 安全边界

### 11.1 Daemon 进程边界

- 仅访问 `vault/`（vault root 在 config.toml 中绝对路径写死）
- 任何 path traversal 拒绝（包括符号链接逃逸）
- 不需要 root / admin 权限
- 不申请 macOS Full Disk Access（除非 vault 在受保护路径，详见 [`cross-platform.md`](cross-platform.md)）

### 11.2 MCP 安全

- stdio MCP 必须由父进程 spawn（无网络口）
- SSE / HTTP MCP 默认关闭
- `[mcp].allowed_clients` 白名单（agent 名）
- 所有写工具默认进 review queue（user 把关）

### 11.3 凭证管理

- API key 存 OS keychain（macOS Keychain / Windows Credential Manager / libsecret）
- 不进 config.toml 明文
- daemon 启动时仅读到内存，进程结束清空

---

## 12. 与其它文档的关系

- [`SPEC.md §4`](../SPEC.md#4-多-agent-协议) 给协议骨架；本文档给细节
- [`review-queue-policy.md`](review-queue-policy.md) 详化 review queue 的 user UX 与上限保护
- [`mcp-tools.md`](mcp-tools.md) 列出 20 个 MCP 工具的具体 JSON schema
- [`filesystem-access.md`](filesystem-access.md) 给文件访问的 5 路径模型与读写分离
- [`conflict-scenarios.md`](conflict-scenarios.md) (Wave 3) 给 5 个具体冲突剧本
- [`failure-playbook.md`](failure-playbook.md) (Wave 3) 给 9 类失败的回滚步骤

---

## 13. 不在范围

- 多 user 协作的角色/权限（v0.2+）
- 跨设备 / 跨 vault 的 agent 协同（v2+）
- 联邦学习 / agent 间直接通信（永不做——违反单一闸门原则）

---

## 一句话总结

> Schema 是合同（版本化），handshake 是入场券，worktree 是物理隔离，advisory lock 是协调，
> review queue 是单一闸门，change log 是审计——六个机制叠加，多 agent 才能在 wiki 上协作而不打架。
