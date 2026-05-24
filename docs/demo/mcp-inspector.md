# W2 D8 — MCP Inspector 手动验收

> 验证 `wikimind mcp serve` 在 stdio 上跑通 4 个只读 tool（wiki_info /
> read_page / read_raw / list_index），让 Claude Code / Claude Desktop /
> MCP Inspector 都能查到 vault 内容。
>
> 本文档是**手动验收脚本**，不进 CI——CI 跑的是 `internal/mcp` 与
> `cmd/wikimind` 的单测。

---

## 0. 前置

- WikiMind 已 build：`go build ./cmd/wikimind`
- 一个 init 过的 vault，里面至少有一个 reindexed page，例如：

```bash
./wikimind init /tmp/demo-vault
cd /tmp/demo-vault

# 写一个 claim
mkdir -p wiki/claims
cat > wiki/claims/wiki-is-compounding.md <<'EOF'
---
id: cl-2026-05-21-001
type: claim
title: "Wiki 是 compounding artifact"
schema_version: "1.0"
confidence: 0.92
status: supported
---

# Wiki is compounding

每一次 ingest 都让 wiki 更值钱。
EOF

# 写一个 raw 文件
echo "# Hello WikiMind" > raw/inbox/hello.md

# reindex 让 SQLite 看见
/path/to/wikimind page reindex
```

---

## 1. 装 MCP Inspector

官方工具 `@modelcontextprotocol/inspector` 用来交互式调用任意 MCP server。

```bash
npm install -g @modelcontextprotocol/inspector
# 或免装直接 npx：
npx @modelcontextprotocol/inspector /path/to/wikimind mcp serve --vault /tmp/demo-vault
```

Inspector 会在浏览器开 UI，左侧列出 server 暴露的 4 个 tool。

---

## 2. 4 个 tool 调用 + 预期输出

### 2.1 `wiki_info`

**输入**：`{}`（空对象）

**预期 response**（具体计数取决于 vault 内容）：

```json
{
  "vault_root": "/tmp/demo-vault",
  "schema_version": "1.0",
  "daemon_version": "0.1.0-w2",
  "counts": {
    "raw_sources": 0,
    "wiki_pages": 3,
    "claims": 1,
    "entities": 0,
    "concepts": 0,
    "pending_reviews": 0
  },
  "health": {
    "score": 100,
    "drift_claims": 0,
    "lint_warnings": 0
  }
}
```

> `pending_reviews=0` 是 D8 占位（reviews 表 W3 D10+ 才上）。
> `health.score=100` / `drift_claims=0` / `lint_warnings=0` 是 W3 lint
> 上线前的稳定占位值。

### 2.2 `read_page`

**输入**（by id）：

```json
{ "page_id": "cl-2026-05-21-001" }
```

**预期 response**：

```json
{
  "id": "cl-2026-05-21-001",
  "type": "claim",
  "path": "wiki/claims/wiki-is-compounding.md",
  "title": "Wiki 是 compounding artifact",
  "body": "\n# Wiki is compounding\n\n每一次 ingest 都让 wiki 更值钱。\n",
  "confidence": 0.92,
  "status": "supported",
  "schema_version": "1.0",
  "frontmatter": "{...}",
  "history": [],
  "backlinks": []
}
```

**输入**（by path）：

```json
{ "page_id": "wiki/claims/wiki-is-compounding.md" }
```

返回同上（路径形态走 ParsePage 而非 SQLite）。

**输入**（include 标志）：

```json
{
  "page_id": "cl-2026-05-21-001",
  "include_history": true,
  "include_backlinks": true
}
```

`history` / `backlinks` 仍是空数组，但响应里多两个 note 字段说明 W2 D9+ /
D10+ 才会填真值。

**输入**（不存在）：

```json
{ "page_id": "no-such" }
```

返回错误：`page not found: no-such`。

### 2.3 `read_raw`

**输入**：

```json
{ "raw_id": "raw/inbox/hello.md", "format": "raw" }
```

**预期 response**：

```json
{
  "raw_id": "raw/inbox/hello.md",
  "format": "raw",
  "content": "# Hello WikiMind\n",
  "bytes": 17
}
```

二进制文件（如 PDF / PNG）会返回 `"encoding": "base64"` 加 base64 编码的
content。

**输入**（normalized）：

```json
{ "raw_id": "raw/inbox/hello.md", "format": "normalized" }
```

返回错误：`format unsupported: normalized format requires stage-2 parser
(W2 D9 with read_raw_anchor)`。

**输入**（path traversal）：

```json
{ "raw_id": "raw/../../etc/passwd" }
```

返回错误：`raw_id must be under raw/` 或 `path escapes vault root`。

### 2.4 `list_index`

**输入**（全部）：

```json
{}
```

**预期 response**：

```json
{
  "total": 3,
  "items": [
    {
      "id": "cl-2026-05-21-001",
      "type": "claim",
      "path": "wiki/claims/wiki-is-compounding.md",
      "title": "Wiki 是 compounding artifact",
      "confidence": 0.92,
      "status": "supported"
    },
    {
      "id": "index",
      "type": "unknown",
      "path": "wiki/index.md",
      "title": "WikiMind Index"
    },
    {
      "id": "log",
      "type": "unknown",
      "path": "wiki/log.md",
      "title": "WikiMind Log"
    }
  ]
}
```

**输入**（type filter）：

```json
{ "type": "claim" }
```

只返 claim 类型条目。

**输入**（prefix + 分页）：

```json
{ "prefix": "wiki/claims/", "limit": 10, "offset": 0 }
```

只返 wiki/claims/ 下的 page，按 type/id 排序。

---

## 3. 接入 Claude Desktop

编辑 `~/Library/Application Support/Claude/claude_desktop_config.json`（macOS）
或 `%APPDATA%\Claude\claude_desktop_config.json`（Windows）：

```json
{
  "mcpServers": {
    "wikimind": {
      "command": "/abs/path/to/wikimind",
      "args": ["mcp", "serve", "--vault", "/tmp/demo-vault"]
    }
  }
}
```

重启 Claude Desktop。然后在 Claude 里说"读一下 vault 里的 claim
cl-2026-05-21-001"——Claude 会调用 `read_page` tool 拿到结果。

---

## 4. 验收清单

- [ ] Inspector 列出 4 个 tool（wiki_info / read_page / read_raw / list_index）
- [ ] `wiki_info` 返回正确的 counts
- [ ] `read_page` by id 命中、by path 命中、未命中报错
- [ ] `read_raw` text 文件返回 utf-8、format=normalized 报 ErrFormatUnsupported
- [ ] `list_index` type 过滤 / prefix 过滤 / limit-offset 分页都工作
- [ ] Claude Desktop 接入后能调用任意 tool 并展示结果
- [ ] stderr 看得到 `wikimind-mcp: vault=...` 与 `ready: 4 tools registered`，
  stdout 全部是 JSON-RPC protocol 消息（不应该有人类可读文本）

---

## 5. 故障排查

| 现象 | 排查 |
|---|---|
| Inspector 报 "Failed to start MCP server" | 跑 `wikimind mcp serve --vault /tmp/demo-vault` 看 stderr |
| `vault config invalid` | 检查 `.wikimind/config.toml` 是否完整 |
| Claude Desktop 看不到 tool | 重启 Claude，看 `~/Library/Logs/Claude/` 日志 |
| read_page 返回错的 frontmatter | 跑 `wikimind page reindex` 再试 |

---

## 6. 不在 D8 范围

- `read_page` 的真 history / backlinks 字段（W2 D9 / D10+）
- `read_raw` 的 normalized format（W2 D9 read_raw_anchor）
- 其它 16 个 tool（`search` / `agent_handshake` / `propose_*` / lock 等，W2-W3）
