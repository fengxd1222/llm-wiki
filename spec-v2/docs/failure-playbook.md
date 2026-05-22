# 失败 Playbook

> "会出什么问题"和"出了问题怎么办"摊开。9 类失败 + 对应回滚命令 + 依赖图级联回滚。
>
> **三方共同盲点 P1 #6 补丁**：原三方案都讲"git revert"，但"agent 写了 50 个互相引用的文件如何
> 整批回滚"没人答。本文档新增**依赖图级联回滚**。

---

## 0. 核心原则

1. **Markdown + git 是真理源** —— 任何派生数据（index.db / 向量）损坏都可重建，不慌。
2. **每个失败都有一条命令** —— user 不需要懂 git 内部，记住 `wikimind <verb>` 即可。
3. **回滚优先于修复** —— 不确定怎么修时，先回到上一个已知良好状态，再排查。
4. **回滚也是 commit** —— revert 不抹掉历史，是追加一条反向 commit + change_log。

---

## 1. 速查表

| 失败 | 命令 | 严重度 |
|---|---|---|
| 索引损坏 | `wikimind rebuild-index` | 低（可重建） |
| Agent 写错了单个 propose | `wikimind review revert <seq>` | 低 |
| Agent 写错了一整批 | `wikimind review revert-bundle <bundle-id>` | 中 |
| Watcher 漏事件 | `wikimind reconcile` | 低 |
| Lock 死锁 | `wikimind lock list` / `lock break <target>` | 低 |
| Git 冲突 | `wikimind conflict list` / `resolve <id>` | 中 |
| 跨平台文件名违规 | `wikimind doctor --fix-names` | 中 |
| Source 漂移 (DRIFT) | `wikimind reverify --since <date>` | 中 |
| Schema 不兼容 | `wikimind migrate <to-version>` | 中 |
| 级联污染（依赖链） | `wikimind revert-cascade <seq>` | 高 |
| 全盘出问题 | `git reset --hard <good-commit>` + `rebuild-index` | 高 |

---

## 2. 九类失败详解

### 2.1 索引损坏

**症状**：`wikimind query` 报错 / 结果明显不对 / 启动时 `PRAGMA integrity_check` 失败。

**原因**：断电、强杀进程、WAL 不一致、SQLite 版本问题。

**修复**：

```bash
wikimind rebuild-index
# 删除 .wikimind/index.db
# 从 wiki/*.md + raw/ 全量重建（含 FTS5 trigram）
# 5000 页约 5-10s
```

**为什么不慌**：index.db 是 100% 派生数据。markdown 是真理源。重建无任何信息损失。

---

### 2.2 Agent 写错了单个 propose

**症状**：user 发现某个已 accept 的 propose 内容有错。

**修复**：

```bash
wikimind log --tail 20                  # 找到对应的 change_log seq
wikimind review revert 42               # revert seq=42 的那次写入
```

`revert` 行为：
- `git revert` 对应 commit（生成反向 commit）
- change_log 追加一条 `op: revert, reverts: 42`
- index 更新
- 原 commit 42 **保留在 git 历史**（审计需要）

---

### 2.3 Agent 写错了一整批

**症状**：一次 ingest / dream cycle 产生的整个 bundle 有问题。

**修复**：

```bash
wikimind review revert-bundle b-0042
# revert 该 bundle 的所有 commit（按逆拓扑序）
# 一条 change_log: op=revert-bundle
```

**前提**：bundle 的 commit 之后**没有**其它 commit 引用它的产物。
如果有 → 进入级联回滚（§3）。

---

### 2.4 Watcher 漏事件

**症状**：改了 raw/ 文件但 wiki 没反应；`wikimind status` 显示的计数与实际不符。

**原因**：FSEvents buffer overflow / Windows RDCW miss / rsync 不改 mtime。

**修复**：

```bash
wikimind reconcile               # 全扫 raw/ + wiki/，对比 SQLite，修差异
wikimind reconcile --full        # 含重算所有 sha256（慢但彻底）
```

`reconcile` 输出 diff 报告，user 确认后应用。日常由 daemon 每小时自动跑一次。

---

### 2.5 Lock 死锁

**症状**：agent 反复收到 `LOCKED` 错误，长时间不解。

**修复**：

```bash
wikimind lock list                       # 看谁持锁
# TARGET            HOLDER               STATUS        EXPIRES
# claims/foo.md     codex:sess-B2        active        in 8m

wikimind lock break claims/foo.md        # 强制释放
```

正常情况 lock 有 TTL（≤ 30min）自动过期 + disconnected session 60s 宽限释放（见
[`conflict-scenarios.md` 剧本 4](conflict-scenarios.md#剧本-4agent-持-lock-后崩溃)）。
`lock break` 是 user 不想等的兜底。

---

### 2.6 Git 冲突

**症状**：daemon 报告 `CONFLICT`；review accept 失败。

**原因**：user 在 daemon 外手动改了 wiki/ 文件，与 pending propose 的 base 冲突。

**修复**：

```bash
wikimind conflict list                   # 列出所有冲突
wikimind conflict show cf-001            # 看冲突详情（3-way diff）
wikimind conflict resolve cf-001 --ours  # 用 user 手动改的版本
wikimind conflict resolve cf-001 --theirs # 用 agent propose 的版本
wikimind conflict resolve cf-001 --edit  # 打开编辑器手动 merge
```

**预防**：尽量不在 daemon 外手动改 wiki/；要改就先 `wikimind lock acquire` 或停 daemon。

---

### 2.7 跨平台文件名违规

**症状**：跨 mac/Windows/Linux 同步后链接断裂；`wikimind doctor` 报 filename violation。

**修复**：

```bash
wikimind doctor --fix-names --dry-run    # 预览
wikimind doctor --fix-names              # 重命名为 ASCII lower kebab + 更新所有 [[link]]
```

详见 [`cross-platform.md §1.4`](cross-platform.md#14-已有违规的迁移)。

---

### 2.8 Source 漂移（DRIFT）

**症状**：lint / Dream Cycle 报 N 个 claim 的 `quote_hash` mismatch。

**原因**：raw/ 文件被外部修改（user 编辑、同步工具覆盖）。

**修复**：

```bash
wikimind reverify --since 2026-05-01     # 重新校验该日期后所有 claim 的 source
# 对每个 DRIFT claim：
#   - quote 仍能在新版 source 中找到 → 更新 quote_hash + span，claim status 不变
#   - quote 找不到了 → claim status = needs_reverify，进 review queue 等 user 决定
```

单个处理：

```bash
wikimind reverify claim cl-2026-05-21-001
```

---

### 2.9 Schema 不兼容

**症状**：升级 schema 后 lint 全红 / agent 报 `SCHEMA_INCOMPATIBLE`。

**修复**：

```bash
wikimind migrate 1.1                     # 把所有 page 迁移到 schema 1.1
# - minor 升级：补 default 值
# - major 升级：跑 migration 脚本（schema/migrations/1.0-to-2.0.lua 等）
wikimind migrate --dry-run 2.0           # 预览迁移影响
```

回退 schema：

```bash
git revert <schema-commit>               # schema/ 进 git，可回退
wikimind reload-schema
```

---

## 3. 依赖图级联回滚（新增补丁）

### 3.1 问题

原三方案的 `revert` 假设"回滚一个 commit 是孤立的"。现实不是：

```
seq 40: accept r-0300  → 新建 claim "RAG 召回率 70%"  (claims/rag-recall.md)
seq 42: accept r-0310  → 新建 topic 引用了 [[rag-recall]]
seq 45: accept r-0320  → 新建 claim 在 body 里 [[rag-recall]]
seq 48: accept r-0330  → entity "rag" 的 related 加了 [[rag-recall]]
```

如果 user 现在想 revert seq 40（那个 claim 错了）——直接 `git revert` 会留下 **3 个 dangling
link**（seq 42/45/48 都引用了被删的 claim）。

### 3.2 反向影响分析

```bash
wikimind revert-cascade 40 --analyze

Reverting seq 40 (claims/rag-recall.md) will affect:

  Direct:
    ✗ claims/rag-recall.md          will be removed

  Cascade (3 pages link to it):
    ⚠ topics/rag-comparison.md       seq 42 — [[rag-recall]] becomes dangling
    ⚠ claims/wiki-vs-rag.md          seq 45 — [[rag-recall]] in body becomes dangling
    ⚠ entities/rag.md                seq 48 — related: [[rag-recall]] becomes dangling

  Options:
    [1] revert-cascade        revert seq 40 + 42 + 45 + 48 (全部，干净)
    [2] revert-with-stubs     revert seq 40，把 3 个引用替换为 tombstone 标记
    [3] revert-isolate        仅 revert seq 40，留 3 个 dangling link（lint 持续警告）
    [4] cancel
```

### 3.3 三种级联策略

| 策略 | 行为 | 何时用 |
|---|---|---|
| **revert-cascade** | 连同所有下游引用一起 revert | 下游产物也都依赖这个错误，整条链都该删 |
| **revert-with-stubs** | revert 目标，下游的 `[[link]]` 替换成 `[[~~rag-recall~~ (reverted seq 40)]]` tombstone | 下游产物本身有价值，只是引用了错误 claim |
| **revert-isolate** | 只 revert 目标，留 dangling link | 临时操作；lint 会持续提醒补 |

### 3.4 依赖图怎么来

daemon 维护 `page_links` 表（见 [`architecture.md §4.2`](architecture.md#42-sqlite-schema-关键表)）。
`revert-cascade --analyze` 就是对 `page_links` 做反向 BFS：

```sql
-- 找所有直接 / 间接引用 target 的 page
WITH RECURSIVE affected(id) AS (
  SELECT 'rag-recall'
  UNION
  SELECT pl.from_id FROM page_links pl JOIN affected a ON pl.to_id = a.id
)
SELECT * FROM affected;
```

深度可能 > 1——级联分析递归到底。

### 3.5 执行

```bash
wikimind revert-cascade 40 --strategy cascade
# 生成一个反向 bundle：revert seq 40,42,45,48
# 一条 change_log: op=revert-cascade, reverts=[40,42,45,48]
# 一次原子操作（要么全成要么全不动）
```

---

## 4. 全盘灾难恢复

最坏情况：vault 状态完全混乱，不知道哪里错了。

```bash
# Step 1: 找最后一个已知良好状态
wikimind log --tail 50                    # 看 change_log
git log --oneline -50                     # 看 git 历史

# Step 2: 硬回退到那个 commit
git reset --hard <last-good-commit>        # ⚠ 丢弃之后所有 wiki 改动

# Step 3: 重建一切派生数据
wikimind rebuild-index
wikimind reconcile --full

# Step 4: 自检
wikimind doctor                            # 全面体检
```

**前提**：raw/ 没坏（raw 只读，正常不会坏）。
**保证**：只要 git 历史在 + raw/ 在，vault 总能恢复到任意历史 commit 的状态。

### 4.1 如果连 git 都坏了

```bash
# .wikimind/change-log.jsonl 也进 git，是审计基线
# 如果 git objects 损坏：
git fsck --full
git reflog                                 # 找悬空 commit
# 最坏：从备份恢复（用户应对 vault 做常规备份）
```

WikiMind **不**替代备份。SPEC §1 第 4 条：local-first，用户对自己的数据负责。
`wikimind doctor` 会检测"vault 是否在任何备份方案覆盖下"并提醒。

---

## 5. 预防性检查

`wikimind doctor` —— 全面体检，建议每周跑（Dream Cycle 也会自动跑）：

```
$ wikimind doctor

Checking vault health...
  ✓ Directory structure (raw/ wiki/ schema/ .wikimind/)
  ✓ Git repo healthy (git fsck passed)
  ✓ SQLite integrity (PRAGMA integrity_check)
  ✓ Filename conventions (347 files, 0 violations)
  ✓ Schema version (1.0, consistent)
  ✓ Change log ↔ git commits (1:1, seq 1-48 all matched)
  ⚠ 1 DRIFT claim (run: wikimind reverify)
  ⚠ Time Machine includes index.db (recommend exclude)
  ✓ No stale locks
  ✓ Watcher running
  ✓ Disk space (vault 142 MB, 89 GB free)

1 warning. Run `wikimind doctor --explain` for details.
```

---

## 6. 失败恢复的 SLA

| 失败类型 | 恢复目标 | 数据损失 |
|---|---|---|
| 索引损坏 | < 1 min（rebuild） | 零 |
| 单 propose 错误 | < 10s（revert） | 零（git 保留） |
| Bundle 错误 | < 30s（revert-bundle） | 零 |
| 级联污染 | < 2 min（revert-cascade） | 零 |
| 全盘灾难 | < 5 min（reset + rebuild） | 仅丢 reset 点之后的改动 |
| Agent worktree 丢失 | 即时 | 仅丢未 propose 的草稿 |

---

## 7. 与其它文档的关系

- 冲突场景的回滚 → [`conflict-scenarios.md`](conflict-scenarios.md)
- 风险与失败的对应 → [`risks.md`](risks.md)
- revert 的协议语义 → [`agent-protocol.md §7`](agent-protocol.md#7-change-log审计真理源)
- 跨平台失败 → [`cross-platform.md §9`](cross-platform.md#9-失败-playbook平台特定)

---

## 8. 不在范围

- 自动灾难恢复（永不做——破坏性操作必须 user 确认）
- 云备份（用户责任；WikiMind 只检测"是否有备份"并提醒）
- 多 user 的并发回滚协调（v0.2+）

---

## 一句话总结

> 9 类失败、每类一条 `wikimind` 命令、依赖图级联回滚补上"50 个互相引用文件怎么整批回退"的盲点。
> 底气来自一条铁律：markdown + git 是真理源，一切派生数据可重建，vault 总能恢复到任意历史状态。
