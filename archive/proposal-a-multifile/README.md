# LLM Wiki：local-first 个人知识库系统 — 研究与产品方案

本仓库是一份**完整的中文产品研究与工程方案**，基于 Andrej Karpathy 的
[*LLM Wiki* gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) 思路，
延展为一个跨 macOS/Windows、可与多 agent（Claude Code、OpenAI Codex、Hermes、OpenCode、Cursor、MCP client 等）
联动的 local-first 个人知识库系统。

> **形态说明**：本仓库**不是代码**，是一份**可直接动手实现的设计与协议文档**。
> 你可以把 `templates/*.md` 直接复制到自己的 wiki vault 的 `schema/` 目录，把 `examples/` 当作起步样例，
> 按 `docs/roadmap-30d.md` 推进 30 天 MVP。

---

## 一句话定位

**LLM Wiki 是一个 local-first 的个人/小团队知识库。原始资料保持只读、不可变；多个 agent 在一个由 Markdown
组成的 wiki 层上持续读写、交叉引用、定期 lint，把"一次性的对话"沉淀为"可累积、可演化、可追溯的知识图谱"。**

---

## 仓库结构

```
.
├── README.md                          ← 你在这里
├── REPORT.md                          ← 主报告：完整产品方案
├── docs/
│   ├── architecture.md                ← MVP 技术架构 + 性能 + 选型对比
│   ├── local-file-access.md           ← macOS / Windows 跨平台本地文件访问方案
│   ├── agent-protocol.md              ← 多 agent 协作协议：锁 / 事务 / change log / review queue
│   ├── mcp-tools.md                   ← MCP tools 设计草案 + JSON schema
│   ├── risks.md                       ← 风险清单 + 缓解措施 + Playbook
│   ├── roadmap-30d.md                 ← 30 天逐周开发计划
│   └── research-qa.md                 ← 8 个重点研究问题的深度回答
├── templates/
│   ├── AGENTS.md                      ← 通用 agent instruction 模板（必读，基底）
│   ├── CLAUDE.md                      ← Claude Code 专用 addendum
│   ├── HERMES.md                      ← Hermes 专用 addendum
│   └── page-schemas.md                ← frontmatter + claim/entity/concept/source/topic/query 模板
└── examples/
    └── directory-tree.md              ← 真实可用的 wiki vault 目录结构样例
```

---

## 怎么读这份方案

### 路径 A — 想 30 分钟看完

1. `REPORT.md`（主报告，含执行摘要 + 路线图）
2. `docs/research-qa.md` 的 Q1、Q2（最有趣的两个理论问题）
3. `examples/directory-tree.md`（目录结构样例）

### 路径 B — 想动手做一个 MVP

1. `REPORT.md` §0–§7（设计哲学、定位、MVP 范围、关键决策）
2. `docs/architecture.md`（技术架构 + SQLite schema + 选型）
3. `docs/local-file-access.md`（跨平台细节）
4. `docs/mcp-tools.md`（MCP 工具集）
5. `docs/agent-protocol.md`（锁、change log、review queue）
6. `templates/AGENTS.md` + `templates/page-schemas.md`（schema 合同）
7. `docs/roadmap-30d.md`（30 天计划）

### 路径 C — 只想看"会出什么问题"

1. `docs/risks.md`（21 条风险 + 缓解）
2. `docs/research-qa.md` Q2（防幻觉策略）

### 路径 D — 直接复用 agent 模板到自己的 wiki

1. 把 `templates/AGENTS.md` 复制到自己的 wiki vault `schema/AGENTS.md`
2. 把 `templates/CLAUDE.md` 复制到 `schema/CLAUDE.md`
3. 把 `templates/HERMES.md` 复制到 `schema/HERMES.md`
4. 把 `templates/page-schemas.md` 复制到 `schema/page-schemas.md`
5. 启动 agent 时让它"先读 schema/AGENTS.md，再开始工作"

---

## 核心设计原则

> 详见 `REPORT.md` §1，这里精简：

1. **Wiki 是 compounding artifact**：不是 cache，每次 ingest / query / lint 都让它增值。
2. **三层结构**：`raw/`（只读）+ `wiki/`（agent 维护）+ `schema/`（合同）。
3. **Claim 是一等公民**：每个非平凡断言独立 markdown 文件，含 raw_id + anchor + quote_hash 三件套追溯。
4. **Local-first，用户授权前提**：所有访问在用户授权目录内；**不绕过任何系统加密、ACL、MDM、企业管控**；
   不读取他人数据；不外发原始资料（除非用户显式同步）。
5. **Boring + small steps**：先 Markdown + SQLite + git，再考虑 vector DB / embedding。
6. **多 agent 协议**：schema 合同 + review queue + advisory lock + change log + git。
7. **可逆 > 智能**：所有写入可 diff、可 revert、可解释。

---

## 9 项交付物对照表

用户在需求里列的 9 项交付物，本仓库对应文件：

| # | 交付物 | 文件 |
|---|---|---|
| 1 | 完整产品方案 | `REPORT.md` |
| 2 | MVP 技术架构 | `docs/architecture.md` |
| 3 | 跨平台本地文件访问方案 | `docs/local-file-access.md` |
| 4 | Agent 协作协议 | `docs/agent-protocol.md` |
| 5 | 目录结构样例 | `examples/directory-tree.md` |
| 6 | `AGENTS.md` / `CLAUDE.md` / `HERMES.md` instruction 草案 | `templates/AGENTS.md`、`templates/CLAUDE.md`、`templates/HERMES.md` |
| 7 | MCP tools 设计草案 | `docs/mcp-tools.md` |
| 8 | 风险清单 | `docs/risks.md` |
| 9 | 30 天开发计划 | `docs/roadmap-30d.md` |

附加：8 个重点研究问题的深度回答 → `docs/research-qa.md`；frontmatter 与页面模板 → `templates/page-schemas.md`。

---

## 终极判据（摘自 `REPORT.md` §12）

> 6 个月后用户回头看自己的 wiki 时——
>
> 如果他能在 30 秒内回到任何一个 claim 的 source；
> 如果他能从任何一个 entity 出发走到 5 跳之外的相关概念；
> 如果他能让一个昨天还没接触过这个 wiki 的新 agent 在 5 分钟内开始有用地工作；
> 如果他敢相信 wiki 里的每一句"事实"——
>
> 那么这个产品就成立了。

---

## 致谢与参考

- Andrej Karpathy，[LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)（核心思路）
- Vannevar Bush，*As We May Think*（1945，Memex 概念）
- [Tolkien Gateway](https://tolkiengateway.net/wiki/Main_Page)（社区 wiki 范例）
- [Obsidian](https://obsidian.md/)、[Obsidian Web Clipper](https://obsidian.md/clipper)、
  [Dataview](https://github.com/blacksmithgu/obsidian-dataview)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [qmd](https://github.com/tobi/qmd)（本地 markdown 搜索 + MCP）
- [sqlite-vec](https://github.com/asg017/sqlite-vec)、[ripgrep](https://github.com/BurntSushi/ripgrep)、
  [whisper.cpp](https://github.com/ggerganov/whisper.cpp)

---

## License

本仓库的方案、文档与模板可自由使用、修改、再分发。商业使用无限制，但**不附带任何担保**。

---

*生成日期：2026-05-20。 中文为主，技术术语保留英文，引用文献给链接。*
