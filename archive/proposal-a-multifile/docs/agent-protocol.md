# 多 Agent 协作协议

> 解决一个工程问题：**Claude Code、Codex、Hermes、OpenCode、Cursor、Cline、自研 agent、CLI 工具
> 等多个 agent，如何共同维护同一个 wiki 而不互相打架、不污染知识、不丢失追溯。**
>
> 核心理念：**schema 是合同，review queue 是单一写入点，change log 是审计，git 是最终源。**

---

## 1. 角色与边界

| 角色 | 身份 | 能力 | 限制 |
|---|---|---|---|
| **User** | 唯一最终决策者 | 任意；最终所有 review 由 user 拍板（或显式委托） | — |
| **Master agent**（可选） | 用户委托的"主 agent"，如 Claude Code | 可被 user 授权自动 accept 一部分低风险 review | 不可改 schema |
| **Worker agent** | Codex / Hermes / Cursor / Cline / 自研 | 读 + propose，**不能直接写正式 wiki** | 受 rate limit + schema 校验 |
| **Daemon** | `llmwikid` 系统进程 | 唯一执行 git commit、唯一写 `.llmwiki/` 状态、唯一发锁 | 不主动产生内容 |
| **Bridge** | `llmwiki-bridge` 文件访问通道 | 读 raw / wiki；不参与决策 | 路径白名单 |

> **协议关键不变量**：
> 1. 只有 daemon 能 `git commit` 正式 wiki。
> 2. 任何 agent 写入都先到 `wiki/_review/`，等待 daemon 把 review 应用到正式 wiki。
> 3. 任何 agent **session 开始必须 `agent_handshake`**（见 `docs/mcp-tools.md`），否则写工具拒绝。
> 4. 每条 change log 与 git commit 一一对应；不存在"未记账"的写入。

---

## 2. Schema 是合同

### 2.1 三层 instruction 文件

```
schema/
├── AGENTS.md       # 通用规则（所有 agent 共同遵守）
├── CLAUDE.md       # Claude Code 专用 addendum
├── HERMES.md       # Hermes 专用 addendum
├── lint-rules.md   # lint 规则
└── page-schemas.md # 页面 frontmatter / 模板
```

- `AGENTS.md` 是**底线**；任何 agent 都必须读、必须遵守。
- 各 agent 的专属文件**只能加严，不能放宽** AGENTS.md 里的约束。
- Schema 文件由 user 维护、进 git；每次 schema 变更触发 lint 全扫。

模板见 `templates/AGENTS.md`、`templates/CLAUDE.md`、`templates/HERMES.md`。

### 2.2 Schema 版本号

`schema/AGENTS.md` 顶部强制：

```yaml
---
schema_version: 1.0
last_updated: 2026-05-20
breaking_changes:
  - "1.0: initial release"
---
```

`agent_handshake` 返回当前 schema_version；agent 自报支持版本；版本不兼容 → daemon 拒绝写工具（agent 必
须先重读 schema）。

---

## 3. 写入路径：Review Queue 是单一闸门

### 3.1 状态机

```
            ┌──────────────┐
            │   pending    │
            └──────┬───────┘
                   │
        ┌──────────┼──────────┐
        ▼          ▼          ▼
    accepted   rejected   superseded
        │
        ▼
     merged (in git, change_log written)
```

| 状态 | 含义 | 触发 |
|---|---|---|
| pending | 等 user / master agent 决议 | agent `propose_*` |
| accepted | user 同意，等 daemon merge | `llmwiki review accept <id>` |
| rejected | 拒绝，留档 | `llmwiki review reject <id> "reason"` |
| superseded | 同 page 有新提议覆盖 | 自动 |
| merged | 已写入正式 wiki 和 git | daemon 执行后 |

### 3.2 一条 review 的生命周期（实例）

```
T0  Claude Code 读了一篇论文，调 propose_page(...) → review_id=r-001 状态 pending
T0+   propose_edit(entities/karpathy.md, …)        → r-002 pending
T0+   propose_claim(...)                            → r-003 pending
T0+   request_review(r-001..r-003, title="ingest karpathy 2025")  → bundle b-001

T1  user 看 bundle b-001:
        llmwiki review show b-001          # 综合 diff
        llmwiki review accept r-001 r-003
        llmwiki review reject r-002 "title typo"

T2  daemon:
        - 把 r-001、r-003 的 patch 应用到 wiki/
        - 写 change_log（seq 推进）
        - git add + commit
        - update index.db
        - delete drafts from wiki/_review/
```

### 3.3 Conflict 处理

两个 agent 同时提议改同一个 page：

- 后到 propose 的 review 会 `LOCKED` 错误（如果前一个有 lock），或者 `CONFLICT`（如果只是 base hash 不同）。
- Agent 收到 `CONFLICT` 必须 re-read page → re-propose（重新 patch）。
- 用户视角：review queue 里看到两个相互冲突的 r-id，UI 提示"互斥，选一个 / 合并"。

### 3.4 单一线程化的 commit

daemon 内部用 channel 串行化所有 commit 操作。这是**故意的**：

- 单机产品，commit 不是瓶颈（每次几十 ms）；
- 串行化避免 git 内部锁的问题；
- 让 change_log 的 seq 严格单调。

---

## 4. 锁机制

### 4.1 为什么需要锁

多 agent 工作时，**跨多个 tool call 的写入序列**容易撕裂。例如 Claude Code 正在做"merge alice 和 alice-2"，
中间一段时间 Codex 改了 alice，merge 就完蛋。

### 4.2 Advisory lock

通过 `acquire_lock` MCP 工具拿，TTL 默认 5 分钟，最长 30 分钟。

```
acquire_lock(page_id="entities/alice.md", agent="claude-code", ttl_sec=300)
→ lock_id="lk-1234", expires_at=...
... do many edits ...
release_lock(lock_id="lk-1234")
```

锁持有期间，daemon 对该 page 的其他 propose 返回 `LOCKED { holder: "claude-code", expires_at: ... }`。

### 4.3 锁的物理形态

- `.llmwiki/locks/<page-id>.lock` 文件，内容：
  ```json
  { "lock_id":"lk-...","agent":"claude-code","pid":12345,"acquired_at":"...","expires_at":"...","ttl_sec":300 }
  ```
- daemon 启动时清理：PID 不存在 / 已过期 → 删。
- 锁文件不进 git（`.gitignore`）。

### 4.4 强占用（不推荐）

`release_lock` 接受 `force: true` 参数，由 user 显式触发（CLI: `llmwiki lock break <page-id>`）。
用于 agent 崩了、PID 还在的边缘情况。

### 4.5 推荐使用模式

- **小改动**：不用锁，propose_edit 一次写完就好。
- **多步骤改造**：拿锁 → 多次 propose → 提交 bundle → 释放锁。
- **lint pass / 重命名**：daemon 内部串行化，user 无需理。

---

## 5. Change Log：审计与可回滚

### 5.1 双轨制

| 文件 | 受众 | 格式 |
|---|---|---|
| `wiki/log.md` | 人 | Markdown，可读，append-only，章节化 |
| `.llmwiki/change-log.jsonl` | 机器 | JSONL，1 行 = 1 op |

两者**总是同步生成**（daemon 写入时一并 flush）。

### 5.2 log.md 约定

每个 entry 标题统一前缀：

```markdown
## [2026-05-20 14:23] ingest | Karpathy LLM Wiki gist

source: raw/articles/karpathy-llm-wiki.md (sha256: abc…)
agent: claude-code
review: bundle b-001
pages_touched: 5 created, 3 edited
```

`grep "^## \[" wiki/log.md | tail -10` 即可看最近 10 条。

### 5.3 change-log.jsonl 字段

```json
{
  "seq": 1234,
  "ts": "2026-05-20T14:23:01.234Z",
  "agent": "claude-code",
  "op": "propose_page",          // or accept / reject / merge / rollback
  "review_id": "r-001",
  "page_id": "01J5...",
  "page_path": "wiki/sources/01J5.../index.md",
  "git_commit": null,
  "hash_before": null,
  "hash_after": "abc..."
}
```

合并到 git 后再追加一行：

```json
{ "seq": 1235, "ts":"...", "agent":"system", "op":"merge", "review_id":"r-001", "git_commit":"def456" }
```

### 5.4 可回滚

```
llmwiki revert <change-log-seq>
```

- daemon 找到对应 git commit；
- 用 `git revert` 创建反向 commit；
- 在 change_log 追加 `op=rollback`；
- 索引重建相关页面。

绝不用 `git reset --hard`，否则 audit 链断。

---

## 6. Git 集成

### 6.1 Commit 策略

- **每一次 review accept = 一次 commit**。
- Commit message 模板：
  ```
  <op>: <page-title>

  agent: <agent>
  review_id: <r-id>
  bundle_id: <b-id|->
  change_log_seq: <seq>
  ```
- 不允许 squash；保留细粒度历史方便 bisect 谁污染了 wiki。

### 6.2 Branch 策略

MVP：单 `main` 分支即可。

v1+：可选 `agent-sandbox/<agent>` 分支策略：

```
main                    ← 已 accept 的内容
agent-sandbox/codex     ← codex 的所有 proposal，未 review 之前先 commit 到这里
agent-sandbox/claude    ← claude 的同上
```

review accept 时 cherry-pick 到 main，原 sandbox branch reset。优点：每个 agent 自带审计轨迹；缺点：复杂。

### 6.3 Remote / 同步

- MVP 不强制 remote；用户自配 GitHub / GitLab / Gitea / 自建。
- 自动 push 默认关；`llmwiki sync` 显式触发。
- pull 时如果有冲突 → 进 `wiki/_review/conflicts/`（不自动解）。

---

## 7. PR-Style Review（可选 / v0.2）

为团队场景准备的"重型"模式：

| 流程 | 说明 |
|---|---|
| Agent 在 `agent-sandbox/<agent>` 分支多次 propose | 不进 main |
| 周期性 daemon 触发 `git request-pull` 或本地 PR | 生成 diff 摘要 |
| User / master agent 在 CLI 或 web UI review | accept / request_changes |
| accept 后 daemon 在 main 上 cherry-pick + change_log | 与 review queue 路径合流 |

MVP 不做。v0.2 后做。

---

## 8. 防幻觉 / 防污染机制

### 8.1 Claim-Source 强绑定

`propose_claim` 强制 `sources[]` 非空（除非 `speculation=true`），且每个 source 必须含：

- `raw_id`（必须在 `sources` 表存在）
- `anchor`（heading / paragraph / char span）
- `quote_hash`（被引文本的 sha256）

daemon 在 accept 前再校验一次：用 `read_raw_anchor` 取实际文本，验证 quote_hash。**不匹配 → 自动 reject + 标记 `DRIFT`**。

### 8.2 不确定性显式

| 字段 | 含义 |
|---|---|
| `confidence: 0..1` | 必填 |
| `status: unverified / verified / disputed / retracted` | 必填 |
| `speculation: true` | 没 source 时强制 |
| `last_verified: <ISO>` | lint 时更新 |

UI 渲染：

- `status=unverified` → 灰色标签 `[未核实]`
- `status=disputed` → 红色 `[存在矛盾]`
- `status=retracted` → 删除线 + tombstone

### 8.3 Lint 防污染规则

| 规则 | 触发 | 处理 |
|---|---|---|
| 任何非 frontmatter 的"事实陈述"必须对应到 claim | NLI / 模式匹配 | 提示用户"这一句是 claim 吗？" |
| `[[id]]` 链接到不存在的 page | broken_link | 自动 reject 写入 |
| 同一 entity 多个 slug | duplicate_entity | 进 merge review |
| 同一 claim 被多 page 重复 | dup_claim | 合并到唯一 page，其他 page 引用 |
| 引用的 source hash 漂移 | DRIFT | 标 `needs_reverify`，agent 重新读 source |

### 8.4 Agent 行为护栏（schema 文件强制）

`AGENTS.md` 必须包含的规则（详见模板）：

> - 你**不可以**写下任何未在 raw/ 中存在的"事实"；
> - 你**不可以**编造引用；
> - 你**不可以**修改 `raw/`、`schema/`、`.git/`；
> - 你**必须**在每次写入前调 `read_page` 取最新版本；
> - 你**必须**在 claim 不确定时设 `speculation=true`；
> - 你**必须**保留 user 已 reject 过的 review 的记忆，不重复提交；
> - 你**应当**在每次 ingest 后建议 user 归档 query；
> - 你**应当**主动报告检测到的矛盾。

### 8.5 Rejection memory

`.llmwiki/rejections.jsonl` 记录每次 reject 的原因；agent_handshake 返回最近 N 条，避免 agent 反复提同样错误。

---

## 9. 重复实体与同义词

### 9.1 别名 + canonical id

每个 entity 页面 frontmatter：

```yaml
id: 01J5XK...
type: entity
title: "Karpathy, Andrej"
aliases: ["Andrej Karpathy","karpathy","ak"]
canonical: true
```

非 canonical 页面：

```yaml
id: 01J5XL...
type: entity
title: "Andrej Karpathy"
canonical_id: 01J5XK...   # 指向 canonical
status: archived
```

linter 强制：`type=entity` 必须有 canonical 或 canonical_id。

### 9.2 Merge proposal

`propose_merge(keep_id, subsume_ids[], alias_map)`：

- subsume 的 page 不立即删除，先标 `status=archived` + `canonical_id`；
- 所有 `[[<subsume_id>]]` 链接保持有效（reader 自动重定向到 canonical）；
- 30 天后 lint 提议物理删除（仍可 git history 找回）。

---

## 10. Query → Wiki 回写流程

每次高质量 query 都是潜在的新 wiki page。强制流程：

1. Agent 答完问题后，必须调用 `propose_page(type="query", …)` 或显式问 user "归档吗？"。
2. 归档的 query 页面 frontmatter：
   ```yaml
   type: query
   question: "X 和 Y 的关系？"
   answer_summary: "..."
   asked_at: 2026-05-20T...
   asked_by: user
   answered_by: claude-code
   sources: [...]
   claims_used: [...]
   ```
3. linter 把高频被引 query 提议升级为 topic 页面。

这是把 chat history 沉淀为 wiki 的关键。

---

## 11. 失败与降级

| 失败 | 处理 |
|---|---|
| Agent 不握手就调写工具 | `PERMISSION_DENIED` + 写 audit |
| Agent 提议明显幻觉（claim 无源） | review 自动拒绝 + 通知 user |
| Agent 频繁提议同样的错误 | rate limit 升级 → 暂时 mute |
| Agent 写超大 patch（>10MB） | 拒绝 + 要求拆分 |
| daemon 崩溃 | review queue 状态在文件里，重启恢复 |
| review queue 积压 | `llmwiki review list --pending` + 提醒 |

---

## 12. 多 agent 同时工作示例

场景：你在 Claude Code 里 ingest 一篇论文；同时 Codex 在做 lint。

```
T0  Claude Code:  agent_handshake → session s-1
                  propose_page(...)   → r-001 (locks entities/karpathy)
T0  Codex:        agent_handshake → session s-2
                  lint_run() → 100 件 issue
                  propose_edit(entities/karpathy.md, ...) → LOCKED { holder=s-1 }
                  → Codex 跳过这一项，处理其他
T1  Claude Code:  request_review(...) → bundle b-001
                  release_lock
T1+ user: accept bundle b-001
T2  Codex:        重试之前 LOCKED 的 propose → 现在拿到最新 hash → propose_edit OK
                  → r-101 pending
T3  user: accept r-101
```

两者**不需要直接通信**；通过 wiki + daemon 间接协作。这就是协议的核心价值。

---

## 13. 一句话总结

> **Schema 让 agent 知道怎么写；review queue 让人保留否决权；锁让多 agent 不踩脚；
> change log 让一切可追溯；git 让一切可回滚。**
>
> 任何一环没做，多 agent 协作就会退化成"看着像在协作的混乱"。
