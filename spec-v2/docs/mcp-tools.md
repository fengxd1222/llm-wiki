# MCP Tools

> 20 个 MCP tool 的完整 JSON schema + `readOnlyHint` 标签 + 错误码 + 调用示例。
>
> 协议总骨架见 [`agent-protocol.md`](agent-protocol.md)；本文档只列 tool 接口。

> **读 vs 写**：下面 9 个 read 类工具是**结构化读取的优化通道**——返回解析好的结构 +
> `quote_hash` + 进 audit。但它们**不是唯一读法**——agent 也可直接 `cat`/`grep` 读 raw/wiki
> （轻量场景，见 [`filesystem-access.md`](filesystem-access.md)）。`propose_*` 写工具则是正式
> wiki 的**唯一**写入路径。

---

## 0. 总览

| # | Tool | 类别 | readOnlyHint | 默认免确认 |
|---|---|---|---|---|
| 1 | `agent_handshake` | meta | ❌ | — |
| 2 | `wiki_info` | read | ✅ | ✅ |
| 3 | `read_page` | read | ✅ | ✅ |
| 4 | `read_raw` | read | ✅ | ✅ |
| 5 | `read_raw_anchor` | read | ✅ | ✅ |
| 6 | `read_claim` | read | ✅ | ✅ |
| 7 | `list_index` | read | ✅ | ✅ |
| 8 | `search` | read | ✅ | ✅ |
| 9 | `graph_neighbors` | read | ✅ | ✅ |
| 10 | `get_history` | read | ✅ | ✅ |
| 11 | `propose_page` | propose | ❌ | ❌ |
| 12 | `propose_edit` | propose | ❌ | ❌ |
| 13 | `propose_claim` | propose | ❌ | ❌ |
| 14 | `propose_delete` | propose | ❌ | ❌ |
| 15 | `propose_merge` | propose | ❌ | ❌ |
| 16 | `request_review` | propose | ❌ | ❌ |
| 17 | `acquire_lock` | mgmt | ❌ | — |
| 18 | `release_lock` | mgmt | ❌ | — |
| 19 | `log_append` | mgmt | ❌ | — |
| 20 | `lint_run` | meta | ✅ | ✅ |

**`readOnlyHint: true`** 让 MCP host（如 Claude Code）跳过 user confirmation，提升体验。
**写工具（11-19）`readOnlyHint: false`**，但**默认不弹 user 确认**——它们写的是 `_review/`，由
review queue 统一把关；user 在 review 阶段才看到。

---

## 1. `agent_handshake`

**用途**：注册 agent session，协商 schema 版本，获取 worktree。

**Schema**：

```json
{
  "name": "agent_handshake",
  "description": "Register agent session, negotiate schema version, get worktree.",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["agent", "version", "session_id", "declares_schema_version"],
    "properties": {
      "agent": {
        "type": "string",
        "enum": ["claude-code", "codex-cli", "hermes", "cursor", "cline", "opencode", "custom"]
      },
      "version": { "type": "string", "description": "agent semver" },
      "session_id": { "type": "string", "description": "client-generated UUID" },
      "capabilities": {
        "type": "array",
        "items": { "type": "string", "enum": ["read", "propose", "lint", "merge"] }
      },
      "declares_schema_version": { "type": "string", "description": "e.g. 1.0" }
    }
  }
}
```

**Response**：

```json
{
  "accepted": true,
  "daemon_schema_version": "1.0",
  "worktree": "wiki/_worktrees/agent-claude-sess-A1/",
  "instructions_to_read": ["schema/AGENTS.md", "schema/CLAUDE.md", "schema/page-schemas.md"],
  "session_token": "sk-...",
  "rate_limits": { "propose_per_minute": 30, "query_per_minute": 60 },
  "queue_state": { "pending": 12, "hard_limit": 50, "can_propose": true },
  "recent_rejections_summary": "Top reasons in last 7 days: quote_hash mismatch (5), claim 粒度过细 (3)..."
}
```

**错误**：`SCHEMA_INCOMPATIBLE` / `AGENT_NOT_WHITELISTED` / `SESSION_EXISTS`

---

## 2. `wiki_info`

**用途**：拿 vault 概况——告诉 agent "你在哪个 vault，规模多大"。

```json
{
  "name": "wiki_info",
  "readOnlyHint": true,
  "description": "Get vault overview (root path, page counts, schema_version, daemon version).",
  "inputSchema": { "type": "object", "properties": {} }
}
```

**Response**：

```json
{
  "vault_root": "/Users/feng/karpathy-vault",
  "schema_version": "1.0",
  "daemon_version": "0.1.0",
  "counts": {
    "raw_sources": 91,
    "wiki_pages": 347,
    "claims": 186,
    "entities": 42,
    "concepts": 28,
    "pending_reviews": 12
  },
  "health": {
    "score": 87,
    "drift_claims": 1,
    "lint_warnings": 7
  }
}
```

---

## 3. `read_page`

```json
{
  "name": "read_page",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["page_id"],
    "properties": {
      "page_id": { "type": "string", "description": "e.g. cl-2026-05-21-001 or path 'claims/wiki-is-compounding.md'" },
      "include_history": { "type": "boolean", "default": false },
      "include_backlinks": { "type": "boolean", "default": false }
    }
  }
}
```

**Response**：完整 frontmatter + body + (optional) history + backlinks。

---

## 4. `read_raw`

```json
{
  "name": "read_raw",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["raw_id"],
    "properties": {
      "raw_id": { "type": "string", "description": "e.g. raw/inbox/karpathy-llm-wiki.md" },
      "format": { "type": "string", "enum": ["raw", "normalized"], "default": "normalized" }
    }
  }
}
```

`format: normalized` 返回 stage-2 parser 的输出（headings + paragraphs + char-spans），方便 agent 抽 claim。
`format: raw` 返回原始字节（含 base64 encoded for binary）。

---

## 5. `read_raw_anchor`

**用途**：读 raw 的特定 anchor（heading / paragraph / char span），含 quote_hash 实时计算。

```json
{
  "name": "read_raw_anchor",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["raw_id", "anchor"],
    "properties": {
      "raw_id": { "type": "string" },
      "anchor": {
        "type": "string",
        "description": "format: #heading-slug | #para-N | #char[start:end]"
      }
    }
  }
}
```

**Response**：

```json
{
  "raw_id": "raw/inbox/karpathy-llm-wiki.md",
  "anchor": "#section-1-philosophy",
  "content": "Every ingest, every query, every lint should make the wiki...",
  "quote_hash": "a7f2e3c1",
  "span": [14, 19],
  "source_mtime": "2026-05-20T22:00:00Z",
  "source_sha256": "7f3a91e4..."
}
```

**核心价值**：agent 抽 claim 时**必须**先调此工具拿 quote_hash，不能自己算（防 hash 编造）。

---

## 6. `read_claim`

**用途**：读 claim，附完整 sources 验证状态。

```json
{
  "name": "read_claim",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["claim_id"],
    "properties": {
      "claim_id": { "type": "string" }
    }
  }
}
```

**Response**：除标准 page 内容外，附加每个 source 的当前校验状态：

```json
{
  ...
  "sources": [
    {
      "raw_id": "raw/inbox/karpathy-llm-wiki.md",
      "anchor": "#section-1-philosophy",
      "stored_quote_hash": "a7f2e3c1",
      "current_quote_hash": "a7f2e3c1",
      "drift_status": "verified"
    },
    {
      "raw_id": "raw/inbox/mindstudio-blog.html",
      "anchor": "#para-7",
      "stored_quote_hash": "d4f9...",
      "current_quote_hash": "e1b8...",
      "drift_status": "drift",
      "source_modified_at": "2026-05-20T16:00:00Z"
    }
  ]
}
```

---

## 7. `list_index`

```json
{
  "name": "list_index",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "properties": {
      "type": { "type": "string", "enum": ["all", "claim", "entity", "concept", "source", "topic"] },
      "prefix": { "type": "string", "description": "optional path prefix filter" },
      "limit": { "type": "integer", "default": 100 },
      "offset": { "type": "integer", "default": 0 }
    }
  }
}
```

**Response**：

```json
{
  "total": 186,
  "items": [
    { "id": "cl-2026-05-21-001", "type": "claim", "title": "Wiki 是一个 compounding artifact", "confidence": 0.92, "status": "supported" },
    ...
  ]
}
```

---

## 8. `search`

```json
{
  "name": "search",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "query": { "type": "string" },
      "type": { "type": "string", "enum": ["fts", "fts+vector"], "default": "fts" },
      "filter": {
        "type": "object",
        "properties": {
          "page_type": { "type": "array", "items": { "type": "string" } },
          "min_confidence": { "type": "number" },
          "status": { "type": "array", "items": { "type": "string" } },
          "updated_since": { "type": "string", "format": "date-time" }
        }
      },
      "limit": { "type": "integer", "default": 20 }
    }
  }
}
```

**Response**：

```json
{
  "results": [
    {
      "page_id": "cl-2026-05-21-001",
      "title": "Wiki 是一个 compounding artifact",
      "snippet": "...wiki 是一个 <mark>compounding artifact</mark>，每次 ingest...",
      "score": 0.92,
      "confidence": 0.92
    }
  ],
  "tokenizer_used": "trigram",
  "query_time_ms": 32
}
```

**注意**：默认走 FTS5。`fts+vector` 仅在 user 显式启用 embedding 后才可用，否则降级到 fts。

---

## 9. `graph_neighbors`

```json
{
  "name": "graph_neighbors",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["page_id"],
    "properties": {
      "page_id": { "type": "string" },
      "direction": { "type": "string", "enum": ["out", "in", "both"], "default": "both" },
      "depth": { "type": "integer", "default": 1, "maximum": 3 },
      "link_types": { "type": "array", "items": { "type": "string" } }
    }
  }
}
```

**Response**：邻居 page 列表（含 link_type）。

---

## 10. `get_history`

```json
{
  "name": "get_history",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "required": ["page_id"],
    "properties": {
      "page_id": { "type": "string" },
      "limit": { "type": "integer", "default": 20 }
    }
  }
}
```

**Response**：

```json
{
  "commits": [
    {
      "sha": "a92d445",
      "ts": "2026-05-21T10:18:00Z",
      "actor": "user",
      "op": "accept",
      "bundle_id": "b-0001",
      "summary": "initial ingest from karpathy gist",
      "diff_summary": "+24 -0"
    }
  ]
}
```

---

## 11. `propose_page`

**用途**：新建一个 page。

```json
{
  "name": "propose_page",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["path", "type", "frontmatter", "body"],
    "properties": {
      "path": { "type": "string", "description": "wiki/claims/foo.md 等" },
      "type": { "type": "string", "enum": ["claim", "entity", "concept", "source", "topic"] },
      "frontmatter": { "type": "object" },
      "body": { "type": "string", "description": "markdown body" },
      "idempotency_key": { "type": "string", "description": "防 agent 重试产生重复 review" }
    }
  }
}
```

**Response**：

```json
{
  "review_id": "r-0245",
  "status": "pending",
  "validations": {
    "schema_check": "passed",
    "quote_hash_check": "passed",
    "path_check": "passed"
  }
}
```

**错误**：`SCHEMA_VIOLATION` / `PATH_NOT_ALLOWED` / `BASE_HASH_MISMATCH`

---

## 12. `propose_edit`

```json
{
  "name": "propose_edit",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["page_id", "base_hash", "patch"],
    "properties": {
      "page_id": { "type": "string" },
      "base_hash": { "type": "string", "description": "sha of page content agent based edit on (read_page time)" },
      "patch": {
        "type": "object",
        "oneOf": [
          { "required": ["unified_diff"], "properties": { "unified_diff": { "type": "string" } } },
          { "required": ["frontmatter_changes", "body"], "properties": {
              "frontmatter_changes": { "type": "object" },
              "body": { "type": "string" }
          } }
        ]
      },
      "summary": { "type": "string", "description": "human-readable summary of change" },
      "idempotency_key": { "type": "string" }
    }
  }
}
```

`base_hash` 是并发控制——若当前 main branch 的 page 已变（base_hash mismatch），返回 `BASE_HASH_MISMATCH`，agent 必须重新 read_page 再 propose。

---

## 13. `propose_claim`

**用途**：propose_page 的 claim 特化版，加强校验。

```json
{
  "name": "propose_claim",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["claim_id", "title", "body", "sources", "confidence"],
    "properties": {
      "claim_id": { "type": "string", "pattern": "^cl-\\d{4}-\\d{2}-\\d{2}-\\d{3}$" },
      "title": { "type": "string", "maxLength": 100 },
      "body": { "type": "string" },
      "sources": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["raw_id", "anchor", "quote", "quote_hash"],
          "properties": {
            "raw_id": { "type": "string" },
            "anchor": { "type": "string" },
            "quote": { "type": "string", "maxLength": 200 },
            "quote_hash": { "type": "string", "pattern": "^[a-f0-9]{8}$" },
            "span": { "type": "array", "items": { "type": "integer" }, "minItems": 2, "maxItems": 2 }
          }
        }
      },
      "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
      "status": { "type": "string", "enum": ["unverified", "supported", "speculation"] },
      "speculation": { "type": "boolean", "default": false },
      "related": { "type": "array", "items": { "type": "string" } },
      "idempotency_key": { "type": "string" }
    }
  }
}
```

**额外验证**：
- 调用时 daemon 重新计算每个 source 的 quote_hash，必须匹配（否则 `QUOTE_HASH_MISMATCH`）
- 每个 source 的 `raw_id` 必须直接指向 raw/，不允许 wiki/（否则 `PROVENANCE_DEPTH_EXCEEDED`）
- 至少 1 个 source 否则 `SCHEMA_VIOLATION`（除非 `speculation: true`）

---

## 14. `propose_delete`

```json
{
  "name": "propose_delete",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["page_id", "reason"],
    "properties": {
      "page_id": { "type": "string" },
      "reason": { "type": "string", "minLength": 10 }
    }
  }
}
```

**特殊行为**：
- 如该 page 有 backlinks → daemon 检查；若有，propose 仍然成立但 review 时会显式提示影响范围
- Reason 必填（最少 10 字）——deletion 必须有清楚理由

---

## 15. `propose_merge`

**用途**：合并两个 page（如发现 entity duplicate）。

```json
{
  "name": "propose_merge",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["keep_id", "merge_ids", "reason"],
    "properties": {
      "keep_id": { "type": "string", "description": "the page to keep" },
      "merge_ids": { "type": "array", "items": { "type": "string" }, "minItems": 1 },
      "reason": { "type": "string" },
      "merged_aliases": { "type": "array", "items": { "type": "string" }, "description": "merged page titles added as aliases" }
    }
  }
}
```

合并后：merged page 的所有 backlinks 改指向 keep_id；merged page 标 `merged_into: keep_id` 但保留（不真删）。

---

## 16. `request_review`

**用途**：把多个 propose 打包成 bundle。

```json
{
  "name": "request_review",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["review_ids", "title", "kind"],
    "properties": {
      "review_ids": { "type": "array", "items": { "type": "string" }, "minItems": 1 },
      "title": { "type": "string" },
      "kind": { "type": "string", "enum": ["ingest", "lint_fix", "query_sediment", "dream_cycle", "custom"] },
      "priority_hint": { "type": "string", "enum": ["critical", "normal", "low"], "default": "normal" }
    }
  }
}
```

**Response**：

```json
{
  "bundle_id": "b-0042",
  "review_ids": ["r-0245", "r-0246", "r-0247"],
  "priority_score": 142,
  "queue_position": 2
}
```

---

## 17. `acquire_lock`

```json
{
  "name": "acquire_lock",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["target"],
    "properties": {
      "target": { "type": "string", "description": "page_id or path glob" },
      "ttl_seconds": { "type": "integer", "default": 600, "maximum": 1800 },
      "purpose": { "type": "string" }
    }
  }
}
```

**Response**：

```json
{ "lock_id": "lk-001", "expires_at": "2026-05-21T10:28:00Z" }
```

**错误**：`LOCKED`（含 holder 信息）

---

## 18. `release_lock`

```json
{
  "name": "release_lock",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["lock_id"],
    "properties": { "lock_id": { "type": "string" } }
  }
}
```

---

## 19. `log_append`

**用途**：写 wiki/log.md（唯一直接写正式 wiki 的 MCP 工具——但仅限 append 模式）。

```json
{
  "name": "log_append",
  "readOnlyHint": false,
  "inputSchema": {
    "type": "object",
    "required": ["category", "message"],
    "properties": {
      "category": { "type": "string", "enum": ["agent_note", "dream_cycle_report", "lint_summary", "milestone"] },
      "message": { "type": "string", "maxLength": 500 },
      "links": { "type": "array", "items": { "type": "string" } }
    }
  }
}
```

**特殊**：
- 不进 review queue（append-only，无修改风险）
- 立即 commit
- 写入 change_log（seq 推进），但 op 标 `append_log`

---

## 20. `lint_run`

```json
{
  "name": "lint_run",
  "readOnlyHint": true,
  "inputSchema": {
    "type": "object",
    "properties": {
      "rules": { "type": "array", "items": { "type": "string" }, "description": "subset of rules to run; default all" },
      "scope": { "type": "string", "description": "path glob; default whole vault" },
      "since": { "type": "string", "format": "date-time", "description": "incremental: only files changed since" }
    }
  }
}
```

**Response**：

```json
{
  "results": [
    {
      "rule": "broken_link",
      "page_id": "concepts/source-of-truth.md",
      "severity": "warn",
      "detail": "Link [[non-existent-page]] does not resolve",
      "suggested_action": "Create page or fix typo"
    }
  ],
  "summary": { "errors": 0, "warnings": 7, "infos": 12 },
  "scanned_pages": 347,
  "elapsed_ms": 1840
}
```

**注意**：lint 是 read-only（不修改 wiki）。若 lint 后想自动修复，用 `propose_*` 工具，进 review queue。

---

## 21. 调用约束

### 21.1 必须先 `agent_handshake`

任何 tool 调用前，agent **必须**在 session 中先调一次 `agent_handshake`，否则返回 `SESSION_REQUIRED`。

### 21.2 Idempotency Key

写工具（11-15）建议带 `idempotency_key`（agent 自生成 UUID）。
重复 propose 同一 key → daemon 返回已有 review_id（不重复创建）。

### 21.3 Rate Limit

详见 [`agent-protocol.md §8`](agent-protocol.md#8-rate-limits)。
每次 tool 调用响应 header 含 `X-RateLimit-Remaining`，agent 应主动节流。

### 21.4 错误处理

所有错误返回统一结构：

```json
{
  "code": "QUOTE_HASH_MISMATCH",
  "message": "...",
  "details": { ... },
  "suggested_action": "..."
}
```

详细错误码见 [`agent-protocol.md §10`](agent-protocol.md#10-协议错误码)。

---

## 22. 与 MCP 规范的对齐

- 所有工具遵循 MCP Tool Definition 规范
- `readOnlyHint` 让 MCP host 选择是否要 user confirmation
- stdio transport 是 MVP 唯一支持的；SSE / HTTP 在 v0.2 评估
- Tool annotations 用于 host UI 渲染：

```json
{
  "name": "propose_claim",
  "annotations": {
    "title": "Propose new claim",
    "readOnlyHint": false,
    "destructiveHint": false,
    "idempotentHint": true,
    "openWorldHint": false
  }
}
```

---

## 23. 不在范围

- Streaming 工具（v0.2，如 streaming search results）
- Multi-modal 工具（v0.2，如 image upload）
- Tool composition / chaining（永不做——agent 决策）

---

## 一句话总结

> 20 个 MCP 工具 = 9 读（免确认）+ 6 写提案（进 review queue）+ 3 管理 + 2 元。
> 所有写入只经 propose_* → review queue → daemon commit 三步走，没有任何 bypass。
