# W1 出口 Demo Walkthrough

> WikiMind W1（D1–D7）出口验收 demo。完整跑一遍 `init → ingest → query
> → revert`，验证 vault 骨架、source page 自动生成、FTS5 CJK 检索、git-backed
> change log 全链路。
>
> 适用：W1 收尾验收 / 新机器 smoke test / 跨平台（macOS / Windows / Linux）
> 验证。

---

## 0. 前置

- Go 1.26+
- git（任意 ≥ 2.30 版本）
- 终端支持 UTF-8（macOS / Linux 默认；Windows 用 PowerShell 7+ 或 Windows
  Terminal）

```bash
go version
git --version
```

构建 CLI：

```bash
go build -o bin/wikimind ./cmd/wikimind
```

把 `bin/wikimind` 放进 `PATH`，或直接用绝对路径调用。下面命令统一用
`wikimind` 简称。

---

## 1. 初始化 vault

新建一个空目录给 vault，跑 `init`：

```bash
wikimind init /tmp/wm-demo
```

预期输出：

```text
initialized: /tmp/wm-demo
schema_version: 1.0
```

`init` 做了：
- 建 `raw/{inbox,imported,attachments,manifests}`、
  `wiki/{claims,entities,concepts,sources,topics,_review,_reports}`、
  `schema/`、`.wikimind/{audit,locks}` 三层目录
- 写默认 7 个 schema 模板到 `schema/`
- 写 `wiki/index.md` + `wiki/log.md`
- 写 `.wikimind/config.toml`
- `git init` + 首个 commit

进入 vault 并看状态：

```bash
cd /tmp/wm-demo
wikimind status
```

预期输出（路径自洽）：

```text
vault: /tmp/wm-demo
schema_version: 1.0
raw_files: 0
wiki_pages: 2
claims: 0
git_branch: main
git_status: dirty
config: ok
health: ok
```

> 注：`git_status: dirty` 是因为 `init` 创建的文件还没被提交到 git；这是
> 已知行为，不影响 demo。

---

## 2. 写一份 raw markdown

```bash
cat > /tmp/karpathy-demo.md <<'EOF'
---
title: "Karpathy 的 LLM 笔记"
---

# Karpathy 的 LLM 笔记

每一次 ingest 都让 wiki 更值钱。
EOF
```

故意用中英混排 + CJK frontmatter title 来覆盖 trigram tokenizer。

---

## 3. ingest

```bash
wikimind ingest /tmp/karpathy-demo.md
```

预期输出（sha256 / size 因平台行尾差异略有不同）：

```text
ingested: raw/inbox/karpathy-demo.md
sha256: <64-hex-string>
size: <bytes>
status: pending
source_page: wiki/sources/karpathy-demo.md
reindexed: <N> pages
```

ingest 做了什么：
1. 复制 raw 文件到 `raw/inbox/karpathy-demo.md`
2. 算 sha256 + 写 SQLite `sources` 表
3. **生成 `wiki/sources/karpathy-demo.md`（D7 新增）**——frontmatter 含
   `id / type=source / title=Karpathy 的 LLM 笔记 / source_path /
   ingested_at`，body 是占位的 "See raw file for full content."
4. 把 raw + source page + `wiki/log.md` + `.wikimind/change-log.jsonl`
   一起放进一个 git commit，message：`ingest: raw/inbox/karpathy-demo.md (seq=1)`
5. **自动 reindex（D7 新增）**——`pages_fts` 立刻可查
   （可用 `--no-reindex` 跳过）

看 source page：

```bash
wikimind page show karpathy-demo
```

预期输出（节选）：

```text
id: karpathy-demo
type: source
path: wiki/sources/karpathy-demo.md
title: Karpathy 的 LLM 笔记
schema_version: 0
---
# Karpathy 的 LLM 笔记

Source ingested from `raw/inbox/karpathy-demo.md`. See raw file for full content.
```

看 git 历史：

```bash
git log --oneline
```

应能看到 `ingest: raw/inbox/karpathy-demo.md (seq=1)` 这个 commit。

---

## 4. query

试一次 CJK substring 查询：

```bash
wikimind query "Karpathy"
```

预期输出（FTS5 trigram 命中）：

```text
karpathy-demo [source] Karpathy 的 LLM 笔记
  ...
```

也可用中文短查询（< 3 字时走 LIKE 兜底）：

```bash
wikimind query "笔记"
```

或 NDJSON 给脚本消费：

```bash
wikimind query "Karpathy" --json
```

---

## 5. revert

回退第一个 commit（seq=1，即 ingest）：

```bash
wikimind revert 1 --no-confirm
```

预期输出（short sha 各异）：

```text
revert seq=1 (commit=<sha> op=ingest summary=raw/inbox/karpathy-demo.md)
reverted: <orig-sha> -> <new-sha> (new seq=2)
```

验证回滚效果：

```bash
ls raw/inbox/ 2>&1                           # 文件不存在（或目录已被 git 清掉）
ls wiki/sources/ 2>&1                        # 同上
cat .wikimind/change-log.jsonl | head -3     # seq=1 ingest + seq=2 revert
```

raw 文件 + source page 同时消失，log 保持 append-only：seq=1 行不删，
seq=2 新追加一行 `op=revert`。

> 注：git 不跟踪空目录，所以 `git revert` 把目录里唯一的文件删掉后，目录
> 本身在 worktree 里也会被清理；`ls` 报 "No such file or directory" 是
> 预期。`wikimind init` 重建目录用 `os.MkdirAll`。

revert of revert（恢复内容）：

```bash
wikimind revert 2 --no-confirm
```

预期：raw + source page 回来。

---

## 6. W1 闭环已覆盖

跑完上面 6 步，验证了：

- [x] D1：vault 三层骨架 + `init` / `status`
- [x] D2：跨平台 path normalize（demo 中 `source_path` 字段统一 POSIX `/`）
- [x] D3：raw/inbox 投递 + sha256/mtime/size + sources 表
- [x] D4：markdown 解析 + page UPSERT + `page show / list`
- [x] D5：FTS5 trigram + CJK 检索 + 短查询 LIKE 兜底
- [x] D6：append-only `log.md` + `change-log.jsonl` + git auto-commit
  + `revert <seq>`
- [x] D7：source page 自动生成（ingest 闭环）+ auto reindex + demo 全平台
  smoke

---

## 7. D8+ Teaser（不在 W1 出口）

W2 即将到来的能力：

- `wiki/index.md` 自动维护（page graph + backlinks）
- 反向链接 `[[…]]` 解析持久化到 `page_links` 表
- file watcher 增量 reindex（FSEvents / RDCW / inotify）
- Lock manager + git worktree per agent
- MCP server（`wikimind mcp serve`）：让 Claude Code 经 MCP 完整跑
  ingest + query
- propose / review queue / bundle（W3）

---

## 8. 常见问题

**Q: `wikimind status` 报 `no WikiMind vault found`？**
A: cwd 不在 vault 内，且祖先目录也不是。`cd` 进 vault 或带路径参数
`wikimind status /tmp/wm-demo`。

**Q: `ingest` 报 `git executable not found in PATH`？**
A: 装 git；W1 的所有 ingest / revert 都强依赖 git。

**Q: 自动 reindex 失败但 ingest 成功了，怎么办？**
A: ingest 主流程不被 reindex 失败阻塞——commit 已成功。手动重跑
`wikimind page reindex` 即可。

**Q: Windows 上中文 vault 路径有问题？**
A: D2 的 path normalize 处理了 `\\?\` 长路径前缀和盘符；如有 issue
请贴出 `wikimind status` 输出。

**Q: query 中文短查询（如 "笔记" 2 字）为什么走 LIKE 而不是 FTS5？**
A: SQLite FTS5 trigram tokenizer 最小匹配长度 3 字符（见
[`spec-v2/docs/cjk-tokenizer.md`](../../spec-v2/docs/cjk-tokenizer.md) §3.3）。
短查询自动 fallback 到 LIKE，召回略低但正确。
