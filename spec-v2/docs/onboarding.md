# 第一公里：`wikimind demo` 5 分钟剧本

> **三方共同盲点 P0 #3**：原三方案都默认 user 已读过 Karpathy gist，但**第一次跑起来到看到价值的
> 闭环**没人设计。新用户 `wikimind init` 之后看到一个空目录，不知道做什么 → 流失。

本文档定义 `wikimind demo` 命令的完整体验：**5 分钟内让 user 走完 ingest → review → query → 看到
wiki**，每一步都有"啊哈时刻"。

---

## 1. 为什么这是 P0

无 onboarding 的产品形态像这样：

```
$ wikimind init my-vault
✓ Created my-vault/raw, my-vault/wiki, my-vault/schema

$ wikimind status
Vault: my-vault
  Pages: 0
  Claims: 0
  Sources: 0

$ wikimind ???    # ← user 卡在这里
```

新用户面对一个空目录、一堆 CLI 命令、一份 SPEC.md，**没人会真的去读完**。

带 onboarding：

```
$ wikimind demo
✓ Created demo-vault/ with 3 sample raw files
✓ Spawned demo agent (claude-stub)
  → ingesting raw/inbox/karpathy-llm-wiki.md…
  → extracted 6 claims, 3 entities, 2 concepts
  → proposing bundle b-0001 (6 items)
✓ Bundle b-0001 ready for review

Next: try ↓
  wikimind review show b-0001        # 看 agent 想写什么
  wikimind review accept b-0001      # 一键全收
  wikimind query "wiki 与 RAG 区别"   # 看你刚 ingest 的内容能查到什么
  open demo-vault/wiki/index.md      # 看 wiki 长什么样
```

5 分钟后 user 知道**这个产品是什么**，决定是否继续投入。

---

## 2. demo 命令的整体设计

### 2.1 三档模式

```bash
wikimind demo                  # 默认：interactive guided，每步等回车
wikimind demo --auto           # 全自动跑完，最后展示结果
wikimind demo --auto --keep    # 全自动 + 保留 demo-vault（默认 demo 完会问是否删）
wikimind demo --reset          # 删掉旧 demo-vault 重来
```

### 2.2 全过程时间预算

| 阶段 | 用时 | 关键 UX |
|---|---|---|
| `init` 创建 vault + 写 sample raw | 5s | 进度条 + 解释每个目录是什么 |
| 启动 demo agent（mock LLM，离线，无 API key 需求） | 3s | "我们用了一个本地 stub agent，不会发请求到网络" |
| Ingest raw/inbox/ 3 个 sample 文件 | 30s | 5 阶段 pipeline 一一展示 |
| 生成 review bundle | < 1s | "现在 6 条 propose 等你拍板" |
| User `review show` 看 diff | 30s | diff 真实可读，带 source 引用 |
| User `review accept` | 5s | "✓ commit a92d445 written" |
| User `query` 看效果 | 20s | 答案带 citation，30 秒回到 raw 段落 |
| 打开 wiki/index.md 浏览 | 60s | 看到自动生成的目录、entity 页、claim 页 |
| 总结 + 给 next step 路径 | 30s | "你刚做了 wikimind 全部核心流程的一遍" |

**严格 5 分钟内**（含 user 思考与输入时间）。

### 2.3 设计原则

1. **完全离线** —— 不需要 API key、不联网。demo agent 是 deterministic mock，输出固定的 claim。
2. **真实数据** —— sample raw 用 Karpathy gist 本身 + 一篇 MindStudio blog + 一段 Twitter thread。
3. **完整闭环** —— ingest → propose → review → accept → query → 浏览 wiki，缺一环都不算 demo。
4. **可重做** —— `wikimind demo --reset` 一键回到起点，鼓励反复尝试。
5. **不污染** —— `demo-vault/` 独立目录，与 user 的真实 vault 隔离；demo 结束问是否保留。

---

## 3. 完整剧本

### 3.1 启动

```
$ wikimind demo

WikiMind Demo  ·  5 分钟看完产品全貌
────────────────────────────────────

这个 demo 会：
  1. 创建一个独立的 demo-vault/（不会动你已有的 vault）
  2. 自动 ingest 3 个 sample 文件（Karpathy gist、blog、Twitter thread）
  3. 让你看到 agent 想写什么、由你拍板 accept / reject
  4. 演示 query → 答案带 citation
  5. 打开 wiki 让你浏览

完全离线，不发任何请求到网络。Demo agent 是本地 stub。

按 Enter 开始…
```

### 3.2 创建 vault

```
[1/6] Creating demo-vault/ ─────────────────────────────────
✓ demo-vault/raw/        ← 原始资料只读区
✓ demo-vault/wiki/        ← agent 维护的知识层
✓ demo-vault/schema/      ← agent 行为合同（AGENTS.md 等）
✓ demo-vault/.wikimind/   ← 索引、锁、change log
✓ demo-vault/raw/inbox/karpathy-llm-wiki.md  (28 KB)
✓ demo-vault/raw/inbox/mindstudio-blog.html  (142 KB)
✓ demo-vault/raw/inbox/rag-vs-wiki-thread.md (8 KB)

按 Enter 启动 demo agent…
```

### 3.3 启动 agent

```
[2/6] Starting demo agent ──────────────────────────────────
✓ Daemon: wikimindd PID 47821
✓ Agent: claude-stub (local, deterministic, offline)
✓ Handshake: schema_version 1.0 compatible
✓ MCP transport: stdio

Next: agent will ingest raw/inbox/. Press Enter…
```

### 3.4 Ingest pipeline（关键时刻 #1）

```
[3/6] Ingesting raw/inbox/ ─────────────────────────────────

karpathy-llm-wiki.md
  [✓] stage 1 deliver     2.1 MB · sha256 ok · 0.4s
  [✓] stage 2 parse       3,847 tokens · 14 sections · 1.2s
  [✓] stage 3 extract     claude-stub · 4 claims + 2 entities + 2 concepts · 4.8s
  [✓] stage 4 propose     8 proposes → bundle b-0001
  [✓] stage 5 review      queued

  → Extracted:
    ◈ claim   wiki-is-compounding         conf 0.92
    ◈ claim   index-md-read-first          conf 0.85
    ◈ claim   claims-are-citizens          conf 0.90
    ◈ claim   raw-is-immutable             conf 0.95
    ◉ entity  karpathy                      (new)
    ◉ entity  llm-wiki                      (new)
    ◎ concept compounding-artifact          (new)
    ◎ concept source-of-truth               (new)

mindstudio-blog.html
  …  (extracts 1 claim, joins bundle b-0001)

rag-vs-wiki-thread.md
  …  (extracts 1 claim + 1 entity, joins bundle b-0001)

✓ Total: 6 claims · 3 entities · 2 concepts = 11 proposes · 1 bundle
✓ Time: 11.3s

  ┌─────────────────────────────────────────────┐
  │ 关键时刻 #1                                  │
  │ Agent 产出 11 条 propose（6 claim + 3        │
  │ entity + 2 concept），没有直接写进 wiki/，   │
  │ 而是放进 _review/ 等你审查——这是单一闸门。  │
  └─────────────────────────────────────────────┘

按 Enter 进入 review…
```

### 3.5 Review（关键时刻 #2）

```
[4/6] Review bundle b-0001 ─────────────────────────────────

$ wikimind review show b-0001

Bundle b-0001 · 11 proposes · agent claude-stub · 2 min ago
Source: raw/inbox/karpathy-llm-wiki.md + 2 more

╭─ r-0001 ─ claim ─ claims/wiki-is-compounding.md ──────────╮
│                                                            │
│   + ## Wiki 是一个 compounding artifact                   │
│   +                                                        │
│   + Karpathy 在 LLM Wiki gist 中明确主张：wiki 是一个     │
│   + compounding artifact，而不是临时缓存。其核心区别       │
│   + 在于——每一次 ingest、每一次 query、每一次 lint，      │
│   + 都应该让 wiki 变得"更值钱"……                          │
│                                                            │
│   sources:                                                 │
│     - raw/inbox/karpathy-llm-wiki.md#section-1             │
│       quote: "every ingest, every query, every lint        │
│               should make the wiki more valuable…"         │
│       quote_hash: a7f2e3c1  ✓ verified                    │
│                                                            │
│   conf 0.92 · sources 1 · provenance_depth 1 ✓            │
╰────────────────────────────────────────────────────────────╯

… (省略其它 10 条 propose，user 可滚动查看)

操作选项：
  [a] accept all                  ← 一键全收
  [r] reject all
  [s] selective (打开交互界面，逐条决议)
  [v] view specific (输入 r-0001 看完整 diff)
  [q] quit demo

你选什么? [a/r/s/v/q]: a
```

### 3.6 Accept

```
[5/6] Accept & commit ──────────────────────────────────────

Applying 11 proposes to wiki/…

  ✓ wiki/claims/wiki-is-compounding.md     (new)
  ✓ wiki/claims/index-md-read-first.md     (new)
  ✓ wiki/claims/claims-are-citizens.md     (new)
  ✓ wiki/claims/raw-is-immutable.md         (new)
  ✓ wiki/claims/wiki-vs-rag-stateful.md    (new)
  ✓ wiki/claims/query-sediments-back.md    (new)
  ✓ wiki/entities/karpathy.md               (new)
  ✓ wiki/entities/llm-wiki.md               (new)
  ✓ wiki/entities/rag.md                    (new)
  ✓ wiki/concepts/compounding-artifact.md  (new)
  ✓ wiki/concepts/source-of-truth.md       (new)
  ✓ wiki/index.md                           (updated, +11 entries)
  ✓ wiki/log.md                             (updated, 1 entry)

  → git commit a92d445 "accept b-0001: initial ingest from demo"
  → change_log seq 1 written
  → index.db updated (FTS5 reindexed in 0.3s)

  ┌─────────────────────────────────────────────┐
  │ 关键时刻 #2                                  │
  │ 这就是 wiki 增值的一次循环：raw → claim →    │
  │ 你的确认 → git commit。下次类似的查询，      │
  │ wiki 已经有现成答案，不需要再 retrieve。     │
  └─────────────────────────────────────────────┘

按 Enter 查询试试…
```

### 3.7 Query（关键时刻 #3）

```
[6/6] Query ────────────────────────────────────────────────

$ wikimind query "wiki 与 RAG 的本质区别是什么?"

Searching… (FTS5 BM25 + relations)
Found 3 highly relevant pages.

──────────────────────────────────────────────────────────
ANSWER

Wiki 与 RAG 的本质区别在于状态性：

1. **stateful vs stateless** — Wiki 是持续累积的工件，
   RAG 是每次查询独立的临时拼接。[1]

2. **ingest-time vs retrieval-time** — Wiki 在 ingest 时
   把知识结构化，RAG 在 retrieval 时才组装 chunk。[2]

3. **compounding 价值** — Wiki 越用越值钱（每次 ingest /
   query / lint 都增值），RAG 的索引不会因使用而进化。[3]

CITATIONS
  [1] wiki/claims/wiki-vs-rag-stateful.md
      ← raw/inbox/karpathy-llm-wiki.md#sec-2  quote_hash 8f3c…
  [2] wiki/claims/wiki-is-compounding.md
      ← raw/inbox/karpathy-llm-wiki.md#sec-1  quote_hash a7f2…
  [3] wiki/concepts/compounding-artifact.md
      ← raw/inbox/mindstudio-blog.html#para-7  quote_hash e1b8…
──────────────────────────────────────────────────────────

  ┌─────────────────────────────────────────────┐
  │ 关键时刻 #3                                  │
  │ 答案的每一条都能 30 秒内回到 raw 原文段落。  │
  │ 没有"模型自己编的"——所有内容都可验证。     │
  └─────────────────────────────────────────────┘

按 Enter 看 wiki 长什么样…
```

### 3.8 浏览 wiki

```
Opening demo-vault/wiki/index.md in your default editor…

  (Obsidian / VS Code / vim / Cursor — 取决于系统默认)

提示：
  - 试试点 [[karpathy]] 双链跳转到 entity 页
  - 试试 [[wiki-is-compounding]] 跳到 claim 页
  - 每个 page 顶部 frontmatter 有完整 metadata
  - 每个 claim 页底部有 sources 区，含 quote_hash + ✓ verified
  - wiki/log.md 是按时间追加的演化账本
```

### 3.9 总结 + Next Step

```
────────────────────────────────────────────────────────────
DEMO COMPLETE  ·  5 分钟你做了什么
────────────────────────────────────────────────────────────

✓ Created vault with 3 raw files (Karpathy gist + blog + thread)
✓ Agent extracted 6 claims, 3 entities, 2 concepts (11 proposes)
✓ Reviewed and accepted bundle b-0001 (single gate; reversible)
✓ Queried wiki, got cited answer (every claim → raw segment)
✓ Browsed wiki structure (frontmatter, double-links, sources)

这就是 WikiMind 的核心循环。所有后续功能（多 agent 协作、
Dream Cycle、跨平台同步）都是在这个循环之上的增量。

Next:
  □ 想用真 vault → wikimind init ~/my-vault
  □ 想接入 Claude Code → wikimind mcp serve（然后配置 Claude Code）
  □ 想看产品全貌 → cat docs/SPEC.md
  □ 想看具体路线图 → cat docs/roadmap-30d.md

Demo vault: ./demo-vault/
  □ Keep it for exploration → already saved
  □ Delete it → wikimind demo --reset

Thanks for trying WikiMind 🌱
```

---

## 4. Sample raw 文件设计

### 4.1 三个文件清单

| 文件 | 大小 | 类型 | 为什么选 |
|---|---|---|---|
| `karpathy-llm-wiki.md` | 28 KB | Markdown | 自我引用：产品本身的来源文档 |
| `mindstudio-blog.html` | 142 KB | HTML | 演示 HTML parsing；引用 Karpathy 的二手转述 |
| `rag-vs-wiki-thread.md` | 8 KB | Markdown（Twitter thread 整理） | 短文本；提供"主流观点 vs 新模式"对比 |

三个文件**共同支持** 11 条 claim：覆盖核心概念 + 引用关系 + DRIFT 场景（mindstudio-blog.html 比
karpathy-gist 多了一些转述细节，故意在 claim 中标 0.85 conf 演示"二手 quote 强度弱"）。

### 4.2 Demo agent 的输出确定性

为了 demo 可复现，`claude-stub` 是**确定性 mock**：

- 同一 sample 文件 → 同一组 claim（hash 固定）
- 不调任何 LLM API → 完全离线
- 抽取规则严格按 [`claim-extraction.md`](claim-extraction.md) 的 4 步算法
- 用预制 prompt + 预制响应模板（在 `examples/demo-walkthrough.md` 列出）

**这不是产品功能的简化版**——是产品功能的**确定性快照**，让 demo 可教学、可调试。

---

## 5. 出错处理

### 5.1 常见出错点

| 出错 | 原因 | 处理 |
|---|---|---|
| demo-vault 已存在 | 上次 demo 没清理 | 提示 "use --reset to recreate" |
| port 被占（daemon） | 上次实例没退 | 自动 kill 旧 PID 并 restart |
| sample 文件下载失败 | 首次 demo 需下载 | 内嵌 fallback（编译进二进制），无需联网 |
| FTS5 索引建立失败 | SQLite 版本 < 3.31 | 提示升级 SQLite + 给链接 |
| user 在 review 阶段 Ctrl-C | user 中途退出 | 保留 bundle 在 _review/，下次 demo 提示 "resume?" |

### 5.2 全程不需要的东西

- ❌ API key（任何）
- ❌ 网络（首次 demo 二进制已带 sample）
- ❌ git config 用户名/邮箱（auto-fallback 到 "wikimind-demo@local"）
- ❌ admin / sudo 权限
- ❌ 任何 install 步骤之外的配置

---

## 6. 成功度量

### 6.1 user 实际行为

通过本地 telemetry（默认关，user 显式打开才发）跟踪：

- `wikimind demo` 启动后 5 分钟内**完整跑完** 的比例 → 目标 ≥ 70%
- demo 完成后 30 分钟内**至少跑一次** `wikimind init`（创建真实 vault）→ 目标 ≥ 40%
- demo 完成后 7 天内**至少跑一次** `wikimind ingest` 真实文件 → 目标 ≥ 20%

### 6.2 内部目标

- 任何修改 demo 命令的 PR 必须 **本人 + 至少 1 个新用户**跑通一遍
- demo 总用时（含等待 + 输入）控制在 5 分钟以内（CI 自动测）
- demo 输出每行不超过 80 列（终端宽度兼容）

---

## 7. 边界情况

### 7.1 完全无 git 的用户

`wikimind init` 检测到无 git → 自动 `git init`（无需 user 配置），不强制 user 懂 git。
demo 不展示 git 命令，但底层确实在 commit。

### 7.2 在 Obsidian vault 中跑 demo

如果当前目录已经是 Obsidian vault：
- 不阻止
- 警告 "demo-vault/ 将在当前目录下创建，Obsidian 会扫到这些 markdown"
- 建议 `--cd ~/Desktop` 隔离

### 7.3 demo 之后用户写错操作

例如 user 自己手动改了 `demo-vault/raw/xx.md`：

- daemon 启动时 reconcile 会检测 mtime 变化
- 触发 `claim_drift` lint
- 这本身**也是教学**：demo 之后第二次跑 `wikimind status` 会看到 1 个 DRIFT，可以演示 reverify

---

## 8. 与 SPEC 的关系

`wikimind demo` 是 SPEC 的**可执行教学**——SPEC 描述"产品是什么"，demo 让 user 一次跑完证明它"真
能跑"。两者必须保持同步：

- SPEC 改了任何核心流程 → demo 剧本要更新
- demo 改了任何输出格式 → SPEC 引用的截图要更新
- 任何不能在 demo 里展示的"卖点"——重新审视是否真是产品价值

---

## 9. 不在范围

- demo 不展示**多 agent 同时跑**（Day 21 才到此里程碑，demo 是单 agent stub）
- demo 不展示**Dream Cycle**（运行需 ≥ 1 周数据，demo 给 5 分钟）
- demo 不展示**Query Sedimentation**（需要 user 实际多次 query 才有意义）
- demo 不展示**MCP 接入**（需要 user 配置 Claude Code 等，超出"零配置"承诺）

以上都在 [`SPEC.md`](../SPEC.md) 里描述，让 user 自己看，不是 demo 的职责。

---

## 一句话总结

> 5 分钟、零配置、零网络、完整闭环、3 个关键时刻、可重做。这是 wikimind 留给新用户的第一印象——
> 也是决定 user 是否继续投入的唯一机会。
