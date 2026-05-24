# W2 D8: MCP server stdio + 4 个只读 tool

## Goal

实现 `wikimind mcp serve`：stdio 上跑 MCP server（modelcontextprotocol Go SDK），
暴露 4 个只读 tool 让 Claude Code 能查 vault：`wiki_info` / `read_page` /
`read_raw` / `list_index`。打通"Claude Code 经 MCP 读 vault"的第一公里
（写 tool 留 D10+ propose_*）。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D8
- `spec-v2/docs/mcp-tools.md` §2-4, §7（4 个 tool 的 JSON schema + response shape）
- `spec-v2/docs/agent-protocol.md`（MCP 协议总骨架）
- W0 验证项已确认 modelcontextprotocol/go-sdk v1.6.1 可用（go.mod:7）

## What I already know

- `cmd/wikimindd/main.go` 是 W0 留下的 daemon stub（仅 14 行 fmt.Println），
  D10+ 才会用；D8 在 `cmd/wikimind/` 下加 `mcp serve` 子命令而非用 wikimindd
- `internal/index/pages.go`（D4）：`PageRow` + `ListPages(typeFilter)` +
  `GetPageByID` —— `read_page` 和 `list_index` 直接复用
- `internal/index/sources.go`（D3）：`SourceRow` —— `read_raw` 复用
- `internal/service/page.go`（D4）：`ParsePage` —— `read_page` body 解析复用
- `internal/vault/config.go`（D2）：`LoadConfig` + `Config` —— `wiki_info` 复用
- 文件读取：用 `internal/vault.ResolveInVault` 防 path traversal
- modelcontextprotocol/go-sdk v1.6.1 API：`mcp.NewServer` + `AddTool` +
  `RunStdio`（与 W0 hello-world 同模式）

## Requirements

### A. 新包 `internal/mcp/`

- `server.go`：
  - `NewServer(ctx, vaultRoot) (*mcp.Server, error)`：构建 MCP server 实例
    + 注册 4 个 tool
  - `RunStdio(ctx, server) error`：跑 server 在 stdio
  - SDK 风格：`mcp.NewServer(impl Info, opts ...)`
- `tools.go`：4 个 tool handler，每个签名：
  ```go
  func handleXxx(ctx context.Context, req *XxxRequest) (*mcp.CallToolResult, *XxxResponse, error)
  ```
- `types.go`：Request / Response struct（按 mcp-tools.md schema）

### B. 4 个 tool 实现

#### B1. `wiki_info`（mcp-tools.md §2）

Request: 空对象

Response 字段：
- `vault_root`: 绝对路径（vault config 里读）
- `schema_version`: 从 config / hardcode "1.0"
- `daemon_version`: hardcode "0.1.0-w2"
- `counts`:
  - `raw_sources`: COUNT(*) FROM sources
  - `wiki_pages`: COUNT(*) FROM pages
  - `claims` / `entities` / `concepts`: COUNT WHERE type='claim'/'entity'/'concept'
  - `pending_reviews`: 0（W3 才有 reviews 表）
- `health`:
  - `score`: 100（占位，W3 lint 跑后真算）
  - `drift_claims`: 0（W3 claim drift 检测）
  - `lint_warnings`: 0（W3 lint）

#### B2. `read_page`（§3）

Request: `page_id` (required) + optional `include_history` / `include_backlinks`

实现：
- `page_id` 接受两种形态：纯 id（`cl-2026-...`）或 path（`claims/foo.md`）
  - id 形态：先 `GetPageByID`
  - path 形态：拼 vaultRoot + path → `ParsePage`
- Response：完整 frontmatter + body
- `include_history` 和 `include_backlinks`：**D8 阶段返回空数组**，附 note
  "history requires git log integration (W2 D9+)" / "backlinks require
  page_links table (W2 D10+)" —— 让 user 知道不是 bug，是 staged

#### B3. `read_raw`（§4）

Request: `raw_id` (required, e.g. `raw/inbox/karpathy.md`) + optional
`format` enum `raw|normalized` default `normalized`

实现：
- `raw_id` 必须是相对 vaultRoot 的 raw/ 下路径（不允许 ../ traversal）
- 用 `vault.ResolveInVault` 解析
- `format=raw`：返回文件原始字节
  - text 文件：utf-8 string
  - binary 文件（用 http.DetectContentType 嗅探）：base64 encode + 加
    `encoding: "base64"` 字段
- **`format=normalized`：D8 阶段不实现**，返回 `ErrFormatUnsupported`
  "normalized format requires stage-2 parser (W2 D9 with read_raw_anchor)"
  —— 真要 normalized parsing 是 D9 范围

#### B4. `list_index`（§7）

Request: optional `type` (enum all/claim/entity/concept/source/topic) +
optional `prefix` + `limit` (default 100) + `offset` (default 0)

实现：
- 直接 `ListPages(typeFilter)` from D4
- `prefix` 过滤：内存 filter pages.Path startswith prefix（pages 总数小，OK）
- `limit`/`offset` slice 切片
- Response：`{total, items: [{id, type, title, ...}]}`
  - total 是过滤后总数（不是切片后）
  - `confidence` / `status` 字段 D8 暂留空（claim 才有，W2 D11 加）

### C. CLI 集成

`cmd/wikimind/command.go`:
- `newMcpCommand`：父命令 `wikimind mcp`，含子命令 `serve`
- `newMcpServeCommand`：
  - `wikimind mcp serve`：load config → open DB → `mcp.NewServer` → `RunStdio`
  - flag `--vault <path>` 覆盖默认 vault（也可从 cwd 自动发现 .wikimind/）
- 关键：MCP server 跑在 stdio，所有 logging 必须到 **stderr**（不能污染 stdout
  protocol stream）

### D. 测试

- `internal/mcp/server_test.go`：MCP server 启动/关闭
- `internal/mcp/tools_test.go`：4 个 tool 各 2-3 测试
  - `wiki_info`：空 vault 返回 counts 全 0 / 有 pages 返回真实 counts
  - `read_page`：by id 命中 / by path 命中 / not found 错误
  - `read_raw`：text raw 命中（utf-8）/ path traversal 拒绝 / not found
    - `format=normalized` 返回 ErrFormatUnsupported
  - `list_index`：type filter / prefix filter / limit/offset 边界
- `cmd/wikimind/command_test.go`：`mcp serve` 命令存在 + flag 解析 + 不实跑
  stdio（test 无法 mock stdin/stdout）

### E. MCP inspector 联通

写 `docs/demo/mcp-inspector.md`：
- 装 `npm install -g @modelcontextprotocol/inspector`
- 配置 Claude Desktop `claude_desktop_config.json` 接入 wikimind mcp
- 跑 4 个 tool 各 1 次 + 预期 response
- **手动验收步骤**，不进 CI

## Acceptance Criteria

- [ ] `wikimind mcp serve` 启动不 crash，stdio JSON-RPC 握手成功
- [ ] 4 个 tool 注册 + handler 返回符合 mcp-tools.md schema 的 response
- [ ] `wiki_info` counts 准确（COUNT(*) 与 SQLite 一致）
- [ ] `read_page` by id 和 by path 都 work；not found 返回结构化错误
- [ ] `read_raw` text 文件返回 utf-8；path traversal 拒绝；normalized 友好拒绝
- [ ] `list_index` type + prefix + limit/offset 全工作
- [ ] MCP server stderr-only logging（不污染 stdout protocol stream）
- [ ] 单测：4 个 tool 各覆盖 happy + error
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过
- [ ] `docs/demo/mcp-inspector.md` 手动验收步骤完整

## Definition of Done

- A-E 全 done
- CI 5 OS 全绿
- 测试覆盖 happy + error + path traversal + format unsupported
- commit + push
- prd Decision 记录 MCP SDK 选型 W0 决策 follow-up

## Out of Scope

- `read_page` 的 `include_history` 实际实现（W2 D9 +）
- `read_page` 的 `include_backlinks` 实际实现（W2 D10+ page_links 表）
- `read_raw` 的 `format=normalized`（W2 D9 read_raw_anchor 一起做）
- 其余 16 个 tool（D9-D11）
- `agent_handshake`（D10）
- worktree 分配（D10）
- propose_* 写工具（D11）
- lock manager（D15）
- `wikimindd` daemon main loop（W2 D10+，本 D8 仍是 stub）

## Decision (ADR-lite)

**Context**: D8 需选 MCP transport 实现 + tool handler 模式。

**Decision**:
1. **transport**：stdio（roadmap 明确，符合 Claude Desktop / Claude Code 接入
   习惯）；socket / HTTP 留 future
2. **CLI 入口**：放 `wikimind mcp serve`（非 `wikimindd`），让 user 用熟悉
   的 wikimind binary，wikimindd 留 D10+ 长生命周期 daemon
3. **未实现字段策略**：返回结构化 placeholder + 附 note（不抛 NOT_IMPLEMENTED
   error）—— 让 agent 不至于 crash，user 看到 0 计数会理解是 staged 而非 bug
4. **logging 隔离**：stdio MCP server stdout 是 protocol 通道；所有 log
   必须到 stderr（用 `log.New(os.Stderr, ...)`）—— 否则 protocol stream 破坏

**Consequences**:
- 优点：D8 最小可用 MCP；user 可以在 Claude Code 里 `read_page claims/x.md`
  立即看到 vault 内容
- 缺点：3 个高级字段（history / backlinks / normalized format）staged 但
  agent 知道
- 与 architecture.md §2.1 进程模型暂时不严格一致（D8 MCP server 在 `wikimind`
  CLI 而非独立 daemon），D10 daemon 接管后再调整

## Technical Notes

- modelcontextprotocol/go-sdk v1.6.1 API：
  - `mcp.NewServer(&mcp.Implementation{Name: "wikimind", Version: "0.1.0"}, nil)`
  - `mcp.AddTool(server, &mcp.Tool{Name: "wiki_info", Description: "...",}, handler)`
  - `server.Run(ctx, mcp.NewStdioTransport())`
- W0 hello-world 模板应该在 git log 找得到（早期 spike，参考实现模式）
- tool handler 返回 `*mcp.CallToolResult` 含 content array + `IsError`
- 用 `json.Marshal` response struct → `mcp.TextContent` 文本载体
- Path traversal 防护：`vault.ResolveInVault` 已实现（D2）
- DB 连接共享：`mcp serve` 进程内单 `*sql.DB`（不要每 tool 调用都开关）
- vault config 检查：`mcp serve` 启动失败（找不到 .wikimind/config.toml）
  → exit 非 0 + stderr 友好错误
- binary 文件检测：`http.DetectContentType(first 512 bytes)` —— 非
  `text/*` 走 base64
