# 多 Agent 冲突剧本

> **三方共同盲点 P1 #5**：原三方案都设计了优雅的协议（review queue / lock / worktree），但
> **缺少真实冲突场景的演练**。协议"应该能处理"和"真处理了"之间有差距。
>
> 本文档给 5 个具体冲突剧本，每个含：触发 → 协议处理 → user UX → 回滚动作。

---

## 0. 通用冲突处理原则

所有冲突剧本遵循 4 条原则：

1. **物理隔离优先** —— worktree 让 agent 的修改永不物理覆盖；冲突只在 merge 时暴露。
2. **冲突暴露给 user，不让 agent 自己解决** —— agent 间不协商；冲突一律进 review queue 由 user 决议。
3. **失败保留现场** —— 冲突发生时，所有相关 propose / worktree / patch 都保留，不自动丢弃。
4. **可解释** —— user 看到的不是"git merge conflict"，是"agent A 想这样、agent B 想那样、你选"。

---

## 剧本 1：两 agent 几乎同时改同一个 page

### 触发

```
T0    agent A (claude) read_page(claims/wiki-is-compounding.md)  → base_hash = h1
T0+5s agent B (codex)  read_page(claims/wiki-is-compounding.md)  → base_hash = h1
T0+30s agent A propose_edit(..., base_hash=h1)  → r-0100 pending
T0+45s agent B propose_edit(..., base_hash=h1)  → r-0101 pending
```

两个 propose 都基于 `h1`，但都还没 merge。

### 协议处理

- 物理上无冲突——A 在 worktree-A、B 在 worktree-B，互不可见。
- daemon 接受 **两个** propose（都 pending）。
- daemon 检测到 r-0100 和 r-0101 **target 同一 page**，标记为 `conflict_group: cg-001`。
- review queue 中两条并列显示，附 `⚠ 冲突：r-0100 与 r-0101 都改 wiki-is-compounding.md`。

### User UX

```
$ wikimind review show cg-001

⚠ Conflict group cg-001 — 2 proposes touch the same page

  claims/wiki-is-compounding.md

  ┌─ r-0100 (claude-code, 2 min ago) ──────────────┐
  │  + 补充了 compounding 的第三个例子            │
  │  [view diff]                                   │
  └────────────────────────────────────────────────┘

  ┌─ r-0101 (codex-cli, 1 min ago) ────────────────┐
  │  + 修正了 confidence 0.85 → 0.92               │
  │  + 加了一个 source                              │
  │  [view diff]                                   │
  └────────────────────────────────────────────────┘

  这两个修改互不冲突（改的是不同部分）。选项：
    [1] accept both（daemon 自动顺序合并）
    [2] accept r-0100 only
    [3] accept r-0101 only
    [4] open 3-way merge editor
```

**关键**：daemon 先做一次**语义判定**——如果两个 patch 改的是文件不同区域（无文本重叠），
提示"互不冲突，可都接受"；如果改的是同一行/同一字段，提示"互斥，须选一个或手动 merge"。

### 回滚

- 选 [1] accept both：daemon 按时间序应用 r-0100 → r-0101；若 r-0101 应用失败（base 变了）→
  自动让 codex re-propose（`BASE_HASH_MISMATCH`），r-0100 已 merge 不回滚。
- 任何已 merge 的可 `wikimind review revert <seq>`。

---

## 剧本 2：两 agent propose 互相矛盾的 claim

### 触发

```
agent A ingest 论文 X → propose claim: "RAG 召回率约 70%"  → r-0200
agent B ingest 博客 Y → propose claim: "RAG 召回率可达 95%" → r-0201
```

两个 claim **语义矛盾**（同一指标，不同数值），但来自**不同 source**。

### 协议处理

- 两个 propose 都合法（各有 source + quote_hash 校验通过）。
- daemon 的 lint 规则 `contradictions` 在 propose 阶段做一次轻量检测：
  - 比对新 claim 与已有 claim / 其它 pending claim 的 (主语, 谓语, 量值)
  - 命中疑似矛盾 → 标 `conflict_group: cg-002`，类型 `semantic_contradiction`
- **两个 claim 都不自动 reject**——矛盾本身是有价值的信息。

### User UX

```
$ wikimind review show cg-002

⚠ Conflict group cg-002 — semantic contradiction

  两个 claim 对同一指标给出不同数值：

  r-0200 (claude)  "RAG 召回率约 70%"
    source: raw/papers/rag-eval-2024.md#table-3
    quote: "recall@10 averaged 0.71 across benchmarks"
    confidence 0.90

  r-0201 (codex)   "RAG 召回率可达 95%"
    source: raw/blog/rag-tuning-tips.md#para-9
    quote: "with careful tuning, recall can reach 95%"
    confidence 0.65

  选项：
    [1] accept both, 标为 disputed —— 两个 claim 共存，互标 contradicts
    [2] accept r-0200, refute r-0201 —— 采信论文，反驳博客
    [3] accept r-0201, refute r-0200
    [4] accept both, 新建 topic 解释分歧（"RAG 召回率：基准 vs 调优"）
    [5] reject both, 留待更多 source
```

**推荐默认 [1] 或 [4]**——保留分歧而不是抹掉它。两个 claim 都进 wiki，互相 `contradicts` link，
status 都标 `disputed`。未来更多 source 进来时再 reverify。

### 回滚

- 选 [2]/[3]：被 refute 的 claim status = `refuted`，不删除（保留审计）。可后续 reverify 改回。
- 整组可 `wikimind review revert-group cg-002`。

---

## 剧本 3：user reject 了被依赖的 propose（级联）

### 触发

Bundle b-0042 含 3 个 propose，有依赖：

```
r-0245 (claim wiki-is-compounding)  ──refers to──→  r-0246 (entity karpathy)
                                    ──source in──→  r-0249 (source page)
```

User 操作：

```
$ wikimind review accept r-0245 r-0249
$ wikimind review reject r-0246 "entity 信息有误，karpathy 的 title 写错了"
```

但 r-0245 **依赖** r-0246（claim 引用了这个 entity）。

### 协议处理

daemon 在执行 accept 前做**依赖检查**：

```
r-0245 accept 请求：
  检查 r-0245 的 outbound link → [[karpathy]]
  [[karpathy]] 对应 r-0246 → r-0246 状态 = rejected
  → 依赖断裂！
```

daemon **拒绝**这个 accept 组合，返回：

```
✗ Cannot accept r-0245: it links to [[karpathy]] which is provided by r-0246 (rejected).

选项：
  [1] 也 reject r-0245（级联 reject）
  [2] 修改 r-0245，移除对 [[karpathy]] 的引用后再 accept
  [3] 撤销 r-0246 的 reject，改为 accept（你之前说"title 写错"——可以让 agent 修正后重新 propose）
  [4] accept r-0245 但标 [[karpathy]] 为 dangling link（lint 会持续警告）
```

### User UX

User 选 [3] 最常见——"title 写错"不该让整个 claim 链崩，应该让 agent 修 entity：

```
$ wikimind review amend r-0246 --request "karpathy 的 title 应为 'AI researcher'，不是 'CEO'"
→ daemon 通知 claude-code，r-0246 转 pending，附 user 的修改要求
→ claude 修正后重新 propose r-0246'
→ user accept r-0246' + r-0245 + r-0249 一起
```

### 回滚

- 若 user 已经 accept 了 r-0245（绕过依赖检查不可能，但假设 force）→ 产生 dangling link →
  lint `broken_link` 持续报警，直到补上。
- 级联 reject 整个 bundle：`wikimind review reject-bundle b-0042`。

---

## 剧本 4：agent 持 lock 后崩溃

### 触发

```
T0     agent A acquire_lock("claims/foo.md", ttl=600s, purpose="big refactor")
T0+2m  agent A 进程被 kill（user 关了 Claude Code / OOM / 崩溃）
T0+3m  agent B 想 propose_edit("claims/foo.md") → daemon 返回 LOCKED (holder=A)
```

Agent A 已死，但 lock 还"挂"着。

### 协议处理

- Lock 有 TTL（最长 30 min）。Agent A 的 lock 在 `T0+600s` 自动过期。
- 但 agent B 不必等 10 分钟——daemon 有**主动探活**：
  - daemon 跟踪每个 agent session 的 MCP 连接状态
  - agent A 的 MCP stdio 连接断开 → daemon 标记 session A `disconnected`
  - `disconnected` session 持有的 lock → **宽限 60s 后自动释放**（防止短暂网络抖动误杀）
- 所以 agent B 实际等待：`min(60s 宽限, 600s TTL)` = 60s 后即可。

### User UX

通常 user 无感知——lock 自动释放，agent B 自动重试成功。

如果 user 查看：

```
$ wikimind lock list

  TARGET                  HOLDER              STATUS        EXPIRES
  claims/foo.md           claude-code:sess-A1 disconnected  releasing in 42s

$ wikimind lock break claims/foo.md     # user 也可手动强制释放
✓ Lock on claims/foo.md released (was held by disconnected session sess-A1)
```

### 回滚

- Agent A 在 worktree 里的未 propose 修改——**丢失**（session 清理时 worktree 删除）。
  这是设计选择：未 propose = 未声明意图 = 可丢。
- 如果 agent A 崩溃前已 propose 了一部分 → 那些 propose 仍在 review queue，不受影响。

---

## 剧本 5：schema 在 agent session 中途升级

### 触发

```
T0     agent A handshake，daemon schema_version = 1.0，A 声明支持 1.0 → OK
T0+10m user 编辑 schema/AGENTS.md，把 schema_version 升到 1.1
       （1.1 新增必填字段 claim.evidence_type）
T0+12m agent A propose_claim(...)  ← A 还以为是 1.0，没填 evidence_type
```

### 协议处理

- daemon 监测 `schema/` 目录变化（watcher）。
- schema_version 变化 → daemon 标记所有 active session 为 `schema_stale`。
- agent A 的下一次写工具调用（propose_claim）：
  - daemon 检查 session A 的 `declared_schema_version` (1.0) ≠ 当前 (1.1)
  - 返回 `SCHEMA_INCOMPATIBLE`：

```json
{
  "code": "SCHEMA_INCOMPATIBLE",
  "message": "Schema upgraded to 1.1 during your session. Re-read schema and re-handshake.",
  "details": {
    "your_version": "1.0",
    "current_version": "1.1",
    "breaking_changes": ["1.1: claim.evidence_type now required"]
  },
  "suggested_action": "Re-read schema/AGENTS.md, then call agent_handshake again"
}
```

- agent A 必须：重读 schema → 重新 `agent_handshake` → 用 1.1 规则重新 propose。
- agent A 在 session 中途**已 propose 的**（1.0 规则下的）propose 仍 pending——
  daemon 在它们被 accept 时按"宽容模式"处理：1.0 的 claim 缺 `evidence_type` → 用 schema 1.1 的
  default 值补（minor 升级保证有 default，见 [`agent-protocol.md §2.3`](agent-protocol.md#23-schema-变更政策)）。

### User UX

```
$ wikimind status

⚠ Schema upgraded to 1.1 while 1 agent session was active.
  - claude-code:sess-A1 → marked schema_stale, must re-handshake
  - 3 pending proposes created under 1.0 → will be migrated on accept

无需你操作；agent 会自动重新握手。
```

如果是 **major 升级**（2.0，无 default 的 breaking change）：

- 1.0 下的 pending propose **无法**自动迁移 → daemon 标它们 `needs_resubmit`
- review queue 显示 "⚠ 3 proposes created under schema 1.0, incompatible with 2.0 — reject and ask agent to resubmit"

### 回滚

- Schema 升级本身是 git commit（schema/ 进 git）→ `git revert` 可回退 schema。
- 回退 schema 后，daemon 再次标记 session stale，agent 重新握手回 1.0。

---

## 通用：冲突的可观测性

所有冲突进 `.wikimind/audit/conflicts.jsonl`：

```jsonl
{"ts": "...", "type": "concurrent_edit", "group": "cg-001", "page": "claims/wiki-is-compounding.md", "proposes": ["r-0100", "r-0101"], "resolution": "accept_both", "decided_by": "user"}
{"ts": "...", "type": "semantic_contradiction", "group": "cg-002", "proposes": ["r-0200", "r-0201"], "resolution": "both_disputed", "decided_by": "user"}
```

Weekly Dream Cycle report 含"本周冲突统计"——如果某类冲突频发，说明协议或 agent prompt 需调整。

---

## 冲突类型速查

| 类型 | 剧本 | 默认处理 | user 必须介入 |
|---|---|---|---|
| 并发改同页 | 1 | 暴露两 diff，提示是否互斥 | 是 |
| 语义矛盾 claim | 2 | 两者共存标 disputed | 是 |
| 依赖断裂 | 3 | 拒绝断裂的 accept 组合 | 是 |
| Lock holder 崩溃 | 4 | 60s 宽限后自动释放 | 否（自动） |
| Schema 中途升级 | 5 | session stale + 重握手 | 否（自动；major 升级才需介入） |

---

## 不在范围

- 多 user 同时 review 的冲突（v0.2+，MVP 单 user）
- 跨 vault 的冲突（v2+）
- 自动冲突解决（永不做——冲突一律 user 决议）

---

## 一句话总结

> 协议设计得优雅不等于真能处理冲突。5 个剧本——并发改同页、语义矛盾、依赖断裂、lock 崩溃、
> schema 中途升级——把"应该能处理"变成"演练过怎么处理"。核心原则：物理隔离优先、冲突暴露给
> user、失败保留现场、对 user 可解释。
