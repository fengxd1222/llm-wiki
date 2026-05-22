# 示例：一个真实的 WikiMind Vault 目录结构

> 一个已运行一段时间、ingest 了若干资料的 vault 长什么样。
> 用于 onboarding 时让 user 对"产物形态"有具体预期。

---

## 完整目录树

```
karpathy-vault/
│
├── raw/                                      # 只读原始资料（daemon 永不写）
│   ├── inbox/                                # user 投递文件的入口
│   │   └── (空 — 已 ingest 的移到 imported/)
│   ├── imported/
│   │   ├── karpathy-llm-wiki.md              # 28 KB
│   │   ├── mindstudio-blog.html              # 142 KB
│   │   ├── rag-vs-wiki-thread.md             # 8 KB
│   │   ├── attention-is-all-you-need.pdf     # 1.4 MB (git LFS)
│   │   └── podcast-karpathy-interview.mp3    # 48 MB (git LFS)
│   ├── attachments/
│   │   └── architecture-diagram.png          # 512 KB (git LFS)
│   └── manifests/
│       └── sources.jsonl                     # raw_id ↔ 原始 URL 映射
│
├── wiki/                                     # agent 维护的知识层
│   ├── index.md                              # ★ agent 必须先读
│   ├── log.md                                # append-only 演化账本
│   │
│   ├── claims/                               # 186 个 claim
│   │   ├── wiki-is-compounding.md
│   │   ├── index-md-read-first.md
│   │   ├── claims-are-citizens.md
│   │   ├── raw-is-immutable.md
│   │   ├── wiki-vs-rag-stateful.md
│   │   ├── query-sediments-back.md
│   │   └── ... (180 more)
│   │
│   ├── entities/                             # 42 个 entity
│   │   ├── karpathy.md
│   │   ├── llm-wiki.md
│   │   ├── rag.md
│   │   ├── andy-matuschak.md
│   │   └── ... (38 more)
│   │
│   ├── concepts/                             # 28 个 concept
│   │   ├── compounding-artifact.md
│   │   ├── source-of-truth.md
│   │   ├── evergreen-notes.md
│   │   └── ... (25 more)
│   │
│   ├── sources/                              # 91 个 source page（每个 raw 一个）
│   │   ├── karpathy-llm-wiki.md
│   │   ├── mindstudio-blog.md
│   │   └── ... (89 more)
│   │
│   ├── topics/                               # 14 个 topic（query sediment + 手建）
│   │   ├── rag-vs-llm-wiki.md
│   │   ├── multi-agent-coordination.md
│   │   └── ... (12 more)
│   │
│   ├── _review/                              # 待审 propose 暂存区
│   │   ├── r-0245.patch
│   │   ├── r-0246.patch
│   │   ├── r-0247.patch
│   │   └── b-0042.meta.json                  # bundle 元数据
│   │
│   ├── _worktrees/                           # git worktree per agent (gitignored)
│   │   ├── agent-claude-sess-A1/
│   │   └── agent-codex-sess-B2/
│   │
│   └── _reports/                             # Dream Cycle 周报
│       ├── dream-cycle-2026-05-21.md
│       └── review-week-2026-05-21.md
│
├── schema/                                   # user 维护的合同（agent 只读）
│   ├── AGENTS.md                             # 底线契约
│   ├── CLAUDE.md                             # Claude Code addendum
│   ├── CODEX.md
│   ├── HERMES.md
│   ├── CURSOR.md
│   ├── page-schemas.md                       # frontmatter schema
│   └── lint-rules.md                         # lint 规则定义
│
├── .wikimind/                                # daemon 内部（勿手动改）
│   ├── config.toml
│   ├── daemon.pid
│   ├── index.db                              # SQLite (gitignored)
│   ├── change-log.jsonl                      # 机器可读 change log（进 git）
│   ├── rejections.jsonl                      # rejection memory
│   ├── auto-accept.toml                      # auto-accept 白名单（user 配）
│   ├── audit/
│   │   ├── ingest-errors.jsonl
│   │   ├── conflicts.jsonl
│   │   └── auth-events.jsonl
│   └── locks/
│
├── .gitattributes                            # 锁 LF + UTF-8 + LFS 规则
├── .gitignore                                # .wikimind/index.db, _worktrees/ 等
└── .git/
```

---

## 关键文件示例

### index.md

```markdown
---
type: index
schema_version: "1.0"
updated_at: 2026-05-21T22:00:00Z
---

# karpathy-vault 索引

> Agent：阅读任何正文 page 前，必须先读本文件。

## 统计
- 186 claims · 42 entities · 28 concepts · 91 sources · 14 topics
- Vault health: 87/100

## 核心 concept
- [[compounding-artifact]] — 随使用累积增值的知识工件
- [[source-of-truth]] — raw/ 不可变原则

## 高频 entity
- [[karpathy]] · [[llm-wiki]] · [[rag]]

## 最近 topic
- [[rag-vs-llm-wiki]] — RAG 与 LLM Wiki 的区别
- [[multi-agent-coordination]] — 多 agent 协作机制

## 待办（lint / dream cycle 提示）
- 1 个 DRIFT claim 待 reverify
- 12 个 pending review
```

### log.md（节选）

```markdown
# Wiki Change Log

| seq | ts | actor | op | summary |
|-----|-----|-------|-----|---------|
| 48 | 2026-05-21 22:00 | daemon | dream-cycle | consolidate 3 dup concepts |
| 47 | 2026-05-21 14:35 | you | accept | b-0043: query sediment "RAG vs Wiki" |
| 46 | 2026-05-21 10:32 | codex-cli (auto) | auto-accept | b-0002: lint fix broken links |
| 45 | 2026-05-21 10:18 | you | accept | b-0001: initial ingest from karpathy gist |
```

---

## 规模演化预期

| 阶段 | raw | claims | wiki 总页 | index.db | .git/ |
|---|---|---|---|---|---|
| Day 1（demo） | 3 | 11 | ~20 | < 1 MB | < 1 MB |
| Week 4（dogfood） | 30 | ~180 | ~350 | ~3 MB | ~15 MB |
| 半年（重度用户） | ~300 | ~2000 | ~3500 | ~30 MB | ~150 MB |
| 上限（MVP 设计目标） | ~1000 | ~10000 | ~15000 | ~100 MB | ~500 MB |

超过上限 → 考虑 v0.2 的 jieba tokenizer + embedding 层（见 `docs/cjk-tokenizer.md`）。

---

## 与 Obsidian 的兼容

整个 `wiki/` 目录可直接作为 Obsidian vault 打开：

- `[[id]]` 双链 → Obsidian 原生渲染
- frontmatter → Obsidian Properties / Dataview 可查
- `_review/` `_worktrees/` `_reports/` 以 `_` 前缀，Obsidian 可配置忽略
- `raw/` 可在 Obsidian 中只读浏览

→ user 可以把 Obsidian 当"IDE"，把 agent 当"程序员"，把 wiki 当"代码库"。

---

## 一句话

> 三层结构（raw 只读 / wiki agent 维护 / schema 合同）+ 五类 page + `.wikimind/` 内部状态。
> 整个 wiki/ 是合法的 Obsidian vault——文本、可 diff、可迁移、永不锁定。
