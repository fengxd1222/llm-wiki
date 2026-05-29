package mcp

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fengxd1222/llm-wiki/internal/index"
)

// ErrServerBuild 是构造 *sdk.Server 阶段的统一错误前缀，用于 CLI 区分
// "构造失败" vs "运行失败" 两类错误。
var ErrServerBuild = errors.New("mcp server build failed")

// NewServer 构造 wikimind MCP server，注册 WikiMind 只读 tool。
//
// vaultRoot 必须是已 init 的 vault 绝对路径；db 是已打开的 SQLite 索引
// （调用方负责 Close）。同一进程内 server 全程持有 db handle—— go-sdk 的
// handler 通过闭包绑定，避免 ctx-value 路由。
//
// 返回 *sdk.Server 之后调用方可：
//   - RunStdio(ctx, server)：在 stdin/stdout 上跑 stdio transport
//   - 自定义 transport（test 用 in-memory）：直接 server.Run(ctx, transport)
func NewServer(ctx context.Context, vaultRoot string, db *index.DB) (*sdk.Server, error) {
	_ = ctx
	if vaultRoot == "" {
		return nil, fmt.Errorf("%w: vault root is empty", ErrServerBuild)
	}
	if db == nil || db.SQL() == nil {
		return nil, fmt.Errorf("%w: index db is nil", ErrServerBuild)
	}

	server := sdk.NewServer(&sdk.Implementation{
		Name:    "wikimind",
		Version: daemonVersion,
	}, nil)

	backend := &vaultBackend{root: vaultRoot, db: db, sessions: NewSessionStore()}
	registerTools(server, backend)
	return server, nil
}

// toolSpec 把一个 tool 的元数据 + handler 注册逻辑打包，让工具清单成为
// 单一数据源——registerTools、ToolCount、RegisteredTools 都从这里派生，
// banner 里的工具数不再硬编码。
type toolSpec struct {
	name        string
	description string
	readOnly    bool
	register    func(server *sdk.Server, b *vaultBackend, t *sdk.Tool)
}

// toolSpecs 是 WikiMind MCP server 注册的全部工具的权威清单。
//
// 只读 tool 统一打 ReadOnlyHint=true——让 MCP host（如 Claude Code /
// Claude Desktop）跳过 user confirmation（mcp-tools.md §0、§22）；写工具
// （propose_*/request_review/log_append/acquire_lock/release_lock/
// agent_handshake）打 ReadOnlyHint=false。
var toolSpecs = []toolSpec{
	{"agent_handshake", "Register agent session, negotiate schema version, get worktree.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleAgentHandshake)) }},
	{"wiki_info", "Get vault overview (root path, page counts, schema_version, daemon version).", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleWikiInfo)) }},
	{"read_page", "Read a wiki page by id or vault-relative path.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleReadPage)) }},
	{"read_raw", "Read a raw file under raw/. Use read_raw_anchor for anchored quote_hash reads.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleReadRaw)) }},
	{"list_index", "List indexed pages with optional type / prefix filter + limit/offset.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleListIndex)) }},
	{"read_raw_anchor", "Read a heading, paragraph, or char-span anchor from raw/ and return quote_hash.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleReadRawAnchor)) }},
	{"read_claim", "Read a claim page and staged source validation metadata.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleReadClaim)) }},
	{"search", "Search wiki pages with FTS5/LIKE routing and staged filters.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleSearch)) }},
	{"graph_neighbors", "Read page graph neighbors from live wikilink parsing.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleGraphNeighbors)) }},
	{"get_history", "Read git/change-log history for a wiki page.", true,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleGetHistory)) }},
	{"propose_page", "Propose a new wiki page by writing a patch into the review queue.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleProposePage)) }},
	{"propose_edit", "Propose an edit to an existing wiki page using base_hash concurrency control.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleProposeEdit)) }},
	{"propose_claim", "Propose a verified claim with quote_hash and provenance checks.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleProposeClaim)) }},
	{"request_review", "Bundle pending review proposals for user review.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleRequestReview)) }},
	{"log_append", "Append an audit note to wiki/log.md and commit it directly.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleLogAppend)) }},
	{"acquire_lock", "Acquire an advisory lock on a page to prevent concurrent edits.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleAcquireLock)) }},
	{"release_lock", "Release an advisory lock on a page.", false,
		func(s *sdk.Server, b *vaultBackend, t *sdk.Tool) { sdk.AddTool(s, t, wrapHandler(b.handleReleaseLock)) }},
}

// registerTools 把 handler 装上 server。
//
// go-sdk AddTool 是泛型函数，typed In/Out 让 SDK 自动推断 schema 并校验
// request——每个 tool 的注册闭包写在 toolSpecs 里，这里只做遍历。
func registerTools(server *sdk.Server, b *vaultBackend) {
	for i := range toolSpecs {
		spec := toolSpecs[i]
		spec.register(server, b, &sdk.Tool{
			Name:        spec.name,
			Description: spec.description,
			Annotations: &sdk.ToolAnnotations{ReadOnlyHint: spec.readOnly},
		})
	}
}

// ToolCount 返回 WikiMind MCP server 注册的工具总数。
//
// CLI banner（mcp serve）用它替代硬编码数字，保证 banner 与实际注册数永远
// 一致（toolSpecs 是单一数据源）。
func ToolCount() int {
	return len(toolSpecs)
}

// RegisteredTools 返回注册工具名的副本，按注册顺序排列。
func RegisteredTools() []string {
	names := make([]string, len(toolSpecs))
	for i := range toolSpecs {
		names[i] = toolSpecs[i].name
	}
	return names
}

// wrapHandler 把 "args → result, error" 风格的 handler 适配成 go-sdk 期望的
// ToolHandlerFor 签名。SDK 用 Out 类型推断 output schema 并自动填 Content，
// 调用方不必手动构造 CallToolResult。
//
// 错误返回让 SDK 自动包装成 CallToolResult with IsError=true，并写到
// Content 里；agent 仍能看到错误信息（spec ToolHandlerFor doc）。
func wrapHandler[In any, Out any](
	h func(context.Context, In) (Out, error),
) sdk.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, Out, error) {
		out, err := h(ctx, in)
		if err != nil {
			var zero Out
			return nil, zero, err
		}
		return nil, out, nil
	}
}

// RunStdio 在 stdin/stdout 上跑 MCP server 直到 ctx cancel 或 transport 关闭。
//
// 关键纪律：stdout 是 protocol stream，调用方必须保证所有 logging 走 stderr。
// 这里不做 stdout 拦截——是 CLI 入口（cmd/wikimind/command.go mcp serve）
// 的责任。
func RunStdio(ctx context.Context, server *sdk.Server) error {
	if server == nil {
		return fmt.Errorf("%w: server is nil", ErrServerBuild)
	}
	transport := &sdk.StdioTransport{}
	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("mcp server run: %w", err)
	}
	return nil
}
