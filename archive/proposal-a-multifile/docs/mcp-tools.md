# MCP Tools 设计草案

> `llmwiki mcp serve` 通过 stdio transport 暴露的工具集。
> 目标：**让任何符合 MCP 协议的 client（Claude Code、Cursor、Continue、Cline、Codex、自研 agent 等）
> 都能通过同一套契约读写知识库**。

---

## 1. 设计原则

1. **读写分离**。读工具全自由调用、写工具一律进入 review queue。
2. **小而精**。控制在 ~18 个工具内，避免拖累 agent 的 tool selection 性能。
3. **幂等**。同一参数重复调用得到相同结果；写工具必须接受可选的 `idempotency_key`。
4. **结构化输出**。所有返回值是带 `version` 字段的 JSON，便于 agent 解析。
5. **明确不确定性**。所有读取出来的 claim 都附带 `confidence`、`sources`、`last_verified`。
6. **可关停**。写工具默认在 `dry_run` 模式可被切换为 readonly（用于 review 中的 agent）。

---

## 2. 工具列表速览

| # | Tool | 类别 | 副作用 |
|---|---|---|---|
| 01 | `wiki_info` | meta | 无 |
| 02 | `read_page` | read | 无 |
| 03 | `read_raw` | read | 无 |
| 04 | `read_raw_anchor` | read | 无 |
| 05 | `read_claim` | read | 无 |
| 06 | `list_index` | read | 无 |
| 07 | `search` | read | 无 |
| 08 | `graph_neighbors` | read | 无 |
| 09 | `log_tail` | read | 无 |
| 10 | `propose_page` | write→review | 进队列 |
| 11 | `propose_edit` | write→review | 进队列 |
| 12 | `propose_move` | write→review | 进队列 |
| 13 | `propose_merge` | write→review | 进队列 |
| 14 | `propose_delete` | write→review | 进队列 |
| 15 | `propose_claim` | write→review | 进队列 |
| 16 | `log_append` | append-only | 直接写 log.md |
| 17 | `lint_run` | management | 写 lint 报告 |
| 18 | `acquire_lock` / `release_lock` | management | 临时排他 |
| 19 | `agent_handshake` | meta | 注册 agent，写 audit |
| 20 | `request_review` | management | review 摘要 |

下面给出每个工具的 schema 草案。

---

## 3. 工具详细 schema

> 所有 schema 用 JSON Schema 表达。`output` 是建议格式。

### 3.1 `wiki_info`

获取当前 vault 的元信息和健康度。

```json
{
  "name": "wiki_info",
  "description": "Return metadata about the current wiki vault: root path, sizes, last lint, schema version, git head.",
  "inputSchema": { "type": "object", "properties": {}, "required": [] },
  "output": {
    "schema_version": "1.0",
    "vault_root": "/Users/me/wiki",
    "counts": { "sources": 421, "pages": 1283, "claims": 4012, "pending_review": 7 },
    "git": { "head": "abc1234", "dirty": false },
    "last_lint": "2026-05-18T03:00:00Z",
    "agents_registered": ["claude-code", "codex", "hermes"]
  }
}
```

### 3.2 `read_page`

按 ID 或路径读取 wiki 页面。

```json
{
  "name": "read_page",
  "description": "Read a wiki page by id or POSIX path. Returns frontmatter + body + outbound/inbound links.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id":   { "type": "string", "description": "ULID page id" },
      "path": { "type": "string", "description": "Vault-relative POSIX path, e.g. wiki/entities/alice.md" },
      "include_body": { "type": "boolean", "default": true }
    },
    "oneOf": [ {"required": ["id"]}, {"required": ["path"]} ]
  },
  "output": {
    "id": "01J5XK...",
    "path": "wiki/entities/alice.md",
    "frontmatter": { "type": "entity", "title": "Alice", "...": "..." },
    "body": "...markdown...",
    "outbound_links": ["01J5XQ...", "01J5XR..."],
    "inbound_links": ["01J5XS..."],
    "claims": ["01J5YA...", "01J5YB..."]
  }
}
```

### 3.3 `read_raw`

读取 `raw/` 中的原始资料。**只读**，不可改。

```json
{
  "name": "read_raw",
  "description": "Read a raw source file. Output is text (PDF/DOCX auto-converted). Always include content hash for verification.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id":   { "type": "string" },
      "path": { "type": "string" },
      "format": { "type": "string", "enum": ["text", "markdown", "json"], "default": "markdown" },
      "max_chars": { "type": "integer", "default": 200000 }
    },
    "oneOf": [ {"required": ["id"]}, {"required": ["path"]} ]
  },
  "output": {
    "id": "01J5...",
    "path": "raw/papers/karpathy-2025.pdf",
    "format": "markdown",
    "content": "...",
    "hash_sha256": "...",
    "truncated": false,
    "mime": "application/pdf"
  }
}
```

### 3.4 `read_raw_anchor`

按 anchor（heading / 段落 / 字符 offset）精确取一段原文。**claim 引用回溯专用**。

```json
{
  "name": "read_raw_anchor",
  "description": "Return a specific anchored slice of a raw source. Anchors are: heading path, paragraph index, or char span. Used to verify claims.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id":     { "type": "string", "description": "raw source id" },
      "anchor": { "type": "string", "description": "e.g. '#Architecture/Operations' or 'p:42' or 'c:1024-1500'" },
      "expected_quote_hash": { "type": "string", "description": "sha256 of expected quoted text; mismatch raises drift error" }
    },
    "required": ["id", "anchor"]
  },
  "output": {
    "id": "01J5...",
    "anchor": "#Architecture",
    "text": "...",
    "quote_hash": "...",
    "drift": false
  }
}
```

### 3.5 `read_claim`

读取 claim 详细（含 sources + 关系）。

```json
{
  "name": "read_claim",
  "description": "Read a claim and its full provenance.",
  "inputSchema": {
    "type": "object",
    "properties": { "id": { "type": "string" } },
    "required": ["id"]
  },
  "output": {
    "id": "01J5...",
    "text": "GPT-4 was released in March 2023.",
    "confidence": 0.95,
    "status": "verified",
    "sources": [
      { "raw_id": "01J5...", "anchor": "p:3", "quote_hash": "abc..." }
    ],
    "supports": ["01J5..."],
    "contradicts": [],
    "page_id": "01J5..."
  }
}
```

### 3.6 `list_index`

返回 `wiki/index.md` 的结构化解析。

```json
{
  "name": "list_index",
  "description": "Parsed view of wiki/index.md. Use this BEFORE any deep query — it's the cheapest way to orient.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "category": { "type": "string", "enum": ["entities","concepts","claims","sources","topics","queries","all"], "default": "all" }
    }
  },
  "output": {
    "generated_at": "2026-05-20T...",
    "categories": {
      "entities": [ { "id": "...", "title": "...", "summary": "...", "updated": "..." } ],
      "concepts": [ ... ]
    }
  }
}
```

### 3.7 `search`

```json
{
  "name": "search",
  "description": "Hybrid search over wiki pages. BM25 by default; rerank with embeddings if vault has them.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query":   { "type": "string" },
      "k":       { "type": "integer", "default": 10, "minimum": 1, "maximum": 50 },
      "type":    { "type": "string", "enum": ["page","claim","source","any"], "default": "any" },
      "filter":  {
        "type": "object",
        "properties": {
          "tags":    { "type": "array", "items": {"type":"string"} },
          "since":   { "type": "string", "description": "ISO date" },
          "status":  { "type": "string" }
        }
      },
      "include_snippets": { "type": "boolean", "default": true }
    },
    "required": ["query"]
  },
  "output": {
    "query": "...",
    "results": [
      { "id": "...", "type": "page", "title": "...", "score": 12.4, "snippet": "..." }
    ]
  }
}
```

### 3.8 `graph_neighbors`

图遍历：从一个节点出发 N 跳邻居（entity / concept / claim 都可）。

```json
{
  "name": "graph_neighbors",
  "description": "Return N-hop neighbors of a node with relation labels. Use for 'what's connected to X' queries.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "id": {"type":"string"},
      "hops": {"type":"integer","default":1,"maximum":3},
      "rel_filter": {"type":"array","items":{"type":"string"}},
      "max_nodes": {"type":"integer","default":50}
    },
    "required": ["id"]
  },
  "output": {
    "root": "...",
    "nodes": [ { "id":"...","type":"entity","title":"..." } ],
    "edges": [ { "src":"...","dst":"...","rel":"works_at","claim_id":"..." } ]
  }
}
```

### 3.9 `log_tail`

```json
{
  "name": "log_tail",
  "description": "Tail of log.md (chronological events) and change-log (machine-readable ops).",
  "inputSchema": {
    "type": "object",
    "properties": {
      "n": {"type":"integer","default":20,"maximum":200},
      "kind": {"type":"string","enum":["log","change_log","both"],"default":"both"}
    }
  },
  "output": {
    "log_entries": [ { "ts":"...","kind":"ingest","title":"..." } ],
    "change_log":  [ { "seq":42,"op":"create","agent":"claude-code","page_id":"..." } ]
  }
}
```

### 3.10 `propose_page`

**写工具**：提议新建一个 wiki 页面。**不直接写**，进 review queue。

```json
{
  "name": "propose_page",
  "description": "Propose a NEW wiki page. Goes to wiki/_review/ and review queue. NOT a direct write.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "type":        { "type": "string", "enum": ["source","claim","entity","concept","topic","query","misc"] },
      "title":       { "type": "string" },
      "frontmatter": { "type": "object" },
      "body":        { "type": "string", "description": "Markdown body" },
      "rationale":   { "type": "string", "description": "Why this page should exist; reviewed by human" },
      "idempotency_key": { "type": "string" }
    },
    "required": ["type","title","body","rationale"]
  },
  "output": {
    "review_id": "r-...",
    "proposed_path": "wiki/_review/01J5.../page.md",
    "status": "pending_review",
    "diff": "...unified diff..."
  }
}
```

### 3.11 `propose_edit`

```json
{
  "name": "propose_edit",
  "description": "Propose an edit to an existing wiki page. Provide a unified diff or a structured patch.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "page_id":      { "type": "string" },
      "patch_format": { "type": "string", "enum": ["unified_diff","structured_patch","full_replace"] },
      "patch":        { "type": "string" },
      "rationale":    { "type": "string" },
      "idempotency_key": { "type": "string" }
    },
    "required": ["page_id","patch_format","patch","rationale"]
  },
  "output": { "review_id": "r-...", "status": "pending_review" }
}
```

### 3.12 `propose_move` / `propose_merge` / `propose_delete`

行为同上。`propose_merge` 必须给出 `keep_id` 和 `subsume_ids`，并解释别名映射。

```json
{
  "name": "propose_merge",
  "inputSchema": {
    "type": "object",
    "properties": {
      "keep_id":      { "type": "string" },
      "subsume_ids":  { "type": "array", "items": {"type":"string"} },
      "alias_map":    { "type": "object" },
      "rationale":    { "type": "string" }
    },
    "required": ["keep_id","subsume_ids","rationale"]
  }
}
```

### 3.13 `propose_claim`

claim 是一等公民，单独 tool。

```json
{
  "name": "propose_claim",
  "description": "Propose a new claim with required source citations. A claim WITHOUT sources will be rejected unless marked 'speculation'.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "text":       { "type": "string", "maxLength": 800 },
      "page_id":    { "type": "string", "description": "Wiki page this claim belongs to (entity/concept/topic)" },
      "sources":    {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "properties": {
            "raw_id":     { "type": "string" },
            "anchor":     { "type": "string" },
            "quote":      { "type": "string", "description": "verbatim quoted text" },
            "quote_hash": { "type": "string", "description": "sha256 of quote" }
          },
          "required": ["raw_id","anchor","quote_hash"]
        }
      },
      "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
      "speculation": { "type": "boolean", "default": false },
      "rationale":  { "type": "string" }
    },
    "required": ["text","page_id","confidence","rationale"]
  }
}
```

### 3.14 `log_append`

**唯一直接写**的工具（log.md 是 append-only）。

```json
{
  "name": "log_append",
  "description": "Append a line to log.md. Format enforced: '## [YYYY-MM-DD HH:MM] <kind> | <title>'. Other fields go into the body.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "kind":  { "type": "string", "enum": ["ingest","query","lint","note","decision"] },
      "title": { "type": "string" },
      "body":  { "type": "string", "default": "" },
      "ts":    { "type": "string", "description": "Optional ISO timestamp, default = now" }
    },
    "required": ["kind","title"]
  },
  "output": { "line": "## [2026-05-20 14:23] ingest | Karpathy LLM Wiki gist", "byte_offset": 102489 }
}
```

### 3.15 `lint_run`

```json
{
  "name": "lint_run",
  "description": "Run wiki lint and return a structured report. Does NOT auto-fix.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "checks": { "type": "array", "items": { "type": "string", "enum":
        ["orphan_pages","broken_links","contradictions","stale_claims","unverified_claims","duplicate_entities","schema_violations","missing_index_entries"]
      }},
      "scope": { "type": "string", "enum": ["all","changed_since_last_lint","subset"], "default": "all" },
      "subset_ids": { "type": "array", "items": {"type":"string"} }
    }
  },
  "output": {
    "ran_at": "2026-05-20T...",
    "report_path": ".llmwiki/lint-report-2026-05-20.jsonl",
    "summary": {
      "orphan_pages": 3,
      "broken_links": 7,
      "contradictions": 1,
      "stale_claims": 12,
      "unverified_claims": 5
    },
    "items": [
      { "kind": "broken_link", "page_id": "...", "target_id": "...", "fix_hint": "..." }
    ]
  }
}
```

### 3.16 `acquire_lock` / `release_lock`

```json
{
  "name": "acquire_lock",
  "description": "Acquire an advisory lock on a page id. Use during multi-step edits across multiple tool calls.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "page_id":   { "type": "string" },
      "ttl_sec":   { "type": "integer", "default": 300, "maximum": 1800 },
      "agent":     { "type": "string" }
    },
    "required": ["page_id","agent"]
  },
  "output": { "lock_id": "...", "expires_at": "..." }
}
```

`release_lock` 反之。daemon 在 lock TTL 过期后自动清理。

### 3.17 `agent_handshake`

每个 agent **session 开始时必须调用**，注册自己、声明能力、拿到 schema 引用。

```json
{
  "name": "agent_handshake",
  "description": "Register the calling agent. Returns the instruction file path the agent must follow, plus the current wiki schema version.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "agent_name":    { "type": "string", "examples": ["claude-code","codex","hermes","cursor","cline","custom-xyz"] },
      "version":       { "type": "string" },
      "capabilities":  { "type": "array", "items": {"type": "string"}, "examples": [["read","propose","lint"]] }
    },
    "required": ["agent_name","version"]
  },
  "output": {
    "session_id": "s-...",
    "instructions_path": "schema/CLAUDE.md",
    "schema_version": "1.0",
    "permissions": {
      "can_propose": true,
      "can_direct_write": false,
      "rate_limit_per_min": 60
    },
    "advisory": "Read this file before any write proposal: schema/CLAUDE.md"
  }
}
```

握手会自动写 audit 到 `.llmwiki/change-log.jsonl`。

### 3.18 `request_review`

agent 完成一组提议后，可以主动请求 review summary（用于在对话里向用户展示）。

```json
{
  "name": "request_review",
  "description": "Bundle pending proposals into a single review summary for the user.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "review_ids": { "type": "array", "items": {"type":"string"} },
      "title":       { "type": "string" }
    },
    "required": ["review_ids","title"]
  },
  "output": {
    "bundle_id": "b-...",
    "summary_markdown": "...",
    "diff_summary": { "files_changed": 5, "lines_added": 120, "lines_removed": 13 }
  }
}
```

---

## 4. 错误模型

所有工具统一错误结构：

```json
{
  "error": {
    "code": "DRIFT" ,
    "message": "Quote hash mismatch — raw source has changed since claim was created.",
    "details": { "expected": "abc...", "actual": "def..." },
    "retryable": false
  }
}
```

错误码：

| Code | 含义 |
|---|---|
| `NOT_FOUND` | id/path 不存在 |
| `PERMISSION_DENIED` | client 未在 allowed_clients |
| `LOCKED` | 资源被其他 agent 锁 |
| `DRIFT` | 引用的 quote hash 不匹配 |
| `SCHEMA_VIOLATION` | frontmatter / 模板违规 |
| `RATE_LIMITED` | 超过 rate limit |
| `REVIEW_REQUIRED` | 必须走 review 路径 |
| `IO_ERROR` | 文件系统错误 |
| `CONFLICT` | git 冲突 |
| `INTERNAL` | 其他 |

---

## 5. 实现建议

### 5.1 包结构（Go 版）

```
cmd/llmwiki/main.go
internal/mcp/
  server.go
  tools.go
  tools_read.go
  tools_propose.go
  tools_mgmt.go
internal/core/
  vault.go
  index.go
  review.go
  lock.go
  changelog.go
  git.go
```

### 5.2 推荐 MCP SDK

- 官方 [modelcontextprotocol](https://github.com/modelcontextprotocol) SDK（Go / Python / TypeScript）。
- 若 Go SDK 不全，可直接实现 stdio JSON-RPC（协议很简单）。

### 5.3 测试

- 单元：每个 tool 一组 fixture（small vault snapshot）。
- 集成：用 `mcp inspector` 跑端到端 schema 校验。
- 模糊：让 Claude Code 在沙箱 vault 里跑 100 次 ingest，检查 review queue 是否一致。

---

## 6. 兼容性矩阵

| Client | 支持 stdio MCP | 验证状态 |
|---|---|---|
| Claude Code | ✓ | MVP 必须 |
| Cursor | ✓ | MVP 必须 |
| Continue | ✓ | v0.2 |
| Cline | ✓ | v0.2 |
| Codex CLI | 部分（CLI bridge） | MVP 用 `llmwiki` CLI 兜底 |
| Hermes | TBD | MVP 用 CLI 兜底；MCP 看上游进展 |
| 自研 MCP client | ✓ | 文档自带 example |

不支持 MCP 的 agent，统一通过 `llmwiki` CLI 走（输入参数 → daemon → 输出 JSON）。

---

## 7. 安全注记

- MCP 工具的 `read_raw` 默认有路径白名单（vault.root 下）；任何越界访问返回 `PERMISSION_DENIED`。
- 写工具默认 `dry_run=false`，但 review queue 是天然 dry run（不立即 merge）。
- Rate limit：每个 agent 默认 60 ops/min，配置可调。
- `agent_handshake` 是 audit 锚点；任何不握手就调写工具的 client → `PERMISSION_DENIED`。
