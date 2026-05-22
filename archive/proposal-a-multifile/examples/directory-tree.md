# 目录结构样例

> 一个真实可用的 wiki vault 目录结构。每个文件名都有具体含义；可以照搬作为 `llmwiki init` 后的默认结构。

```
my-wiki/                                        ← vault root；同时也是 git repo 根
│
├── README.md                                   ← user-facing 简短说明（git tracked）
├── .gitignore
├── .gitattributes                              ← LF + UTF-8 强制
│
├── raw/                                        ── 第一层：source of truth（只读）
│   ├── README.md                               ← 一句话说明这是只读区
│   │
│   ├── inbox/                                  ← 新进、未处理（用户直接放入）
│   │   ├── 2026-05-19-paper-attention.pdf
│   │   ├── 2026-05-20-blog-karpathy-llm-wiki.md   (Obsidian Web Clipper 输出)
│   │   └── 2026-05-20-meeting-q2-okrs.md
│   │
│   ├── articles/                               ← 网页文章（clipper 落地）
│   │   ├── 2025-karpathy-llm-wiki.md
│   │   ├── 2024-anthropic-mcp-spec.md
│   │   └── 2024-stratechery-ai-business.md
│   │
│   ├── papers/                                 ← PDF 论文
│   │   ├── 2017-vaswani-attention.pdf
│   │   └── 2023-openai-gpt4-technical-report.pdf
│   │
│   ├── transcripts/                            ← 会议 / 播客 / 视频转录
│   │   ├── 2026-05-12-podcast-lex-fridman-karpathy.md
│   │   └── 2026-05-15-meeting-design-review.md
│   │
│   ├── notes/                                  ← user 手写笔记（user 拥有，agent 只读）
│   │   ├── 2026-05-10-思考-llm-wiki.md
│   │   └── 2026-05-18-reading-list.md
│   │
│   ├── images/                                 ← 截图、图表
│   │   ├── architecture-karpathy.png
│   │   └── obsidian-graph-2026-05-20.png
│   │
│   └── assets/                                 ← 其他二进制
│       └── slides-q2-okrs.pdf
│
├── wiki/                                       ── 第二层：agent 维护的 markdown
│   │
│   ├── index.md                                ← 全局索引（agent 每次 ingest 更新）
│   ├── log.md                                  ← 时间线 append-only
│   │
│   ├── sources/                                ← 每个 raw/ 文件 → 一个 source page
│   │   ├── 01J5XK8M5G9P1ZWX0M.md               (← raw/articles/2025-karpathy-llm-wiki.md)
│   │   └── 01J5XK9N0H7K3RVY1P.md
│   │
│   ├── entities/                               ← 人、组织、地点、产品
│   │   ├── 01J5XM1A2B3C4D5E6F.md               (Andrej Karpathy)
│   │   ├── 01J5XM2A2B3C4D5E6F.md               (OpenAI)
│   │   └── 01J5XM3A2B3C4D5E6F.md               (Tolkien Gateway)
│   │
│   ├── concepts/                               ← 抽象概念
│   │   ├── 01J5XP1A2B3C4D5E6F.md               (RAG)
│   │   ├── 01J5XP2A2B3C4D5E6F.md               (LLM Wiki pattern)
│   │   └── 01J5XP3A2B3C4D5E6F.md               (Memex)
│   │
│   ├── claims/                                 ← 一等公民
│   │   ├── 01J5XR1A2B3C4D5E6F.md
│   │   ├── 01J5XR2A2B3C4D5E6F.md
│   │   └── 01J5XR3A2B3C4D5E6F.md
│   │
│   ├── topics/                                 ← 主题综述（跨 entity/concept）
│   │   ├── 01J5XT1A2B3C4D5E6F.md               (Personal Knowledge Management)
│   │   └── 01J5XT2A2B3C4D5E6F.md               (Multi-agent collaboration)
│   │
│   ├── queries/                                ← 沉淀的高质量 query
│   │   ├── 01J5XV1A2B3C4D5E6F.md               (RAG 和 LLM Wiki 的区别？)
│   │   └── 01J5XV2A2B3C4D5E6F.md
│   │
│   ├── _review/                                ← 待人工 review 的草稿（git ignored 大部分）
│   │   ├── r-001-source-karpathy-gist.md
│   │   ├── r-002-claim-wiki-is-compounding.md
│   │   └── bundles/
│   │       └── b-001-ingest-karpathy-gist.md   ← review bundle 元信息
│   │
│   ├── _inbox/                                 ← agent 自己列的"该补的 query / source"
│   │   ├── 2026-05-20-suggested-queries.md
│   │   └── 2026-05-20-missing-sources.md
│   │
│   └── _archive/                               ← 软删除区（保留 30 天）
│       └── 01J5...md
│
├── schema/                                     ── 第三层：合同
│   ├── AGENTS.md                               ← 通用 agent 指令（必读）
│   ├── CLAUDE.md                               ← Claude Code 专用 addendum
│   ├── HERMES.md                               ← Hermes 专用 addendum
│   ├── CODEX.md                                ← (可选) OpenAI Codex 专用
│   ├── CURSOR.md                               ← (可选) Cursor 专用
│   ├── page-schemas.md                         ← frontmatter & 页面模板
│   ├── lint-rules.md                           ← lint 规则细则
│   └── glossary.md                             ← 术语表（vocabulary）
│
├── .llmwiki/                                   ── 系统文件（大部分 git ignored）
│   ├── config.toml                             ← 全局配置（进 git）
│   ├── schema-version                          ← 当前 schema 版本（进 git）
│   ├── change-log.jsonl                        ← 机器可读 audit（进 git）
│   ├── index.db                                ← SQLite FTS5 + sqlite-vec（git ignored）
│   ├── index.db-wal                            ← WAL（git ignored）
│   ├── index.db-shm                            ← shared memory（git ignored）
│   ├── locks/                                  ← advisory lock sentinel（git ignored）
│   │   └── <page-id>.lock
│   ├── review-queue.jsonl                      ← review 状态（进 git，方便恢复）
│   ├── rejections.jsonl                        ← agent reject 记忆（进 git）
│   ├── lint-reports/                           ← 历史 lint 报告（进 git）
│   │   └── 2026-05-20.jsonl
│   ├── cache/                                  ← OCR / PDF 转换缓存（git ignored）
│   ├── embeddings.db                           ← 可选 embedding 索引（git ignored）
│   ├── run/                                    ← 运行时（socket / pid，git ignored）
│   │   ├── bridge.sock
│   │   └── daemon.pid
│   └── metrics.jsonl                           ← 性能采样（git ignored）
│
└── .git/                                       ← git 本体
```

---

## 文件命名约定（强制）

| 类型 | 规则 | 例 |
|---|---|---|
| 所有 wiki/ markdown | ULID（`01J5XK...`）+ `.md`；不含中文/空格 | `01J5XK8M5G9P1ZWX0M.md` |
| raw/articles, raw/papers, raw/transcripts | `YYYY-MM-DD-<slug>.<ext>`，slug 小写 kebab-case | `2025-karpathy-llm-wiki.md` |
| raw/notes（user 手写） | 用户自由命名，linter 容忍 | `2026-05-10-思考.md`（注意：会触发 linter warning） |
| raw/images | 描述性 kebab-case | `architecture-karpathy.png` |
| review draft | `r-<id>-<slug>.md` | `r-001-source-karpathy-gist.md` |
| review bundle | `b-<id>-<slug>.md` | `b-001-ingest-karpathy-gist.md` |
| schema 文件 | `UPPERCASE.md` 或描述性 kebab-case | `AGENTS.md`, `page-schemas.md` |

### 为什么 wiki/ 用 ULID 文件名

- **跨平台稳定**：ASCII 短字符，全平台 OK；
- **不破链接**：rename 标题不会破坏 `[[id]]` 链接；
- **可排序**：ULID 时间戳前缀，列表自动按创建顺序排；
- **去重容易**：碰撞概率几乎为零。

中文标题怎么办？放 frontmatter `title:` 字段；Obsidian / Dataview / 搜索都用这个 title 展示。

---

## `.gitignore` 推荐

```gitignore
# Runtime / cache
.llmwiki/index.db
.llmwiki/index.db-wal
.llmwiki/index.db-shm
.llmwiki/cache/
.llmwiki/embeddings.db
.llmwiki/run/
.llmwiki/metrics.jsonl
.llmwiki/locks/

# OS
.DS_Store
Thumbs.db
desktop.ini

# Tooling
.obsidian/workspace*.json
.obsidian/cache
```

进 git 的关键文件：

- `wiki/**` 所有 markdown（包括 `_review/`、`_inbox/`、`_archive/`）
- `schema/**`
- `.llmwiki/config.toml`
- `.llmwiki/schema-version`
- `.llmwiki/change-log.jsonl`
- `.llmwiki/review-queue.jsonl`
- `.llmwiki/rejections.jsonl`
- `.llmwiki/lint-reports/*.jsonl`
- `.gitignore`、`.gitattributes`

---

## `.gitattributes` 推荐

```gitattributes
* text=auto eol=lf
*.md     diff=markdown
*.pdf    diff=astextplain
*.png    binary
*.jpg    binary
*.mp3    binary
*.mp4    binary
```

---

## Obsidian 兼容性

打开 vault 时 Obsidian 把 `wiki/` 当成笔记区：

- 设置中 ignore 这些路径：`.git/`、`.llmwiki/`、`raw/inbox/`（用户偏好）
- "Default location for new attachments" → `raw/images/`
- 启用 Dataview 插件查询 frontmatter
- 启用 Templater 提供页面模板
- Graph view 关注 `wiki/` 子树

---

## 一句话总结

> **三层目录、ULID 命名、UTF-8/LF/ASCII 文件名、ID 链接、git 跟踪三层 + 关键状态文件 —— 把这些定下来，
> 跨平台、跨工具、跨时间的所有协作问题都有了基线。**
