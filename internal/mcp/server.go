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

// NewServer 构造 wikimind MCP server，注册 4 个 D8 只读 tool。
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

	backend := &vaultBackend{root: vaultRoot, db: db}
	registerTools(server, backend)
	return server, nil
}

// registerTools 把 4 个 handler 装上 server。
//
// go-sdk AddTool 是泛型函数，每个 tool 一行——typed In/Out 让 SDK 自动
// 推断 schema 并校验 request。
//
// 4 个 D8 tool 都是只读，统一打 ReadOnlyHint=true——让 MCP host（如 Claude
// Code / Claude Desktop）跳过 user confirmation（mcp-tools.md §0、§22）。
func registerTools(server *sdk.Server, b *vaultBackend) {
	readOnly := &sdk.ToolAnnotations{ReadOnlyHint: true}

	sdk.AddTool(server, &sdk.Tool{
		Name:        "wiki_info",
		Description: "Get vault overview (root path, page counts, schema_version, daemon version).",
		Annotations: readOnly,
	}, wrapHandler(b.handleWikiInfo))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "read_page",
		Description: "Read a wiki page by id or vault-relative path.",
		Annotations: readOnly,
	}, wrapHandler(b.handleReadPage))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "read_raw",
		Description: "Read a raw file under raw/. format=normalized is staged for W2 D9.",
		Annotations: readOnly,
	}, wrapHandler(b.handleReadRaw))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "list_index",
		Description: "List indexed pages with optional type / prefix filter + limit/offset.",
		Annotations: readOnly,
	}, wrapHandler(b.handleListIndex))
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
