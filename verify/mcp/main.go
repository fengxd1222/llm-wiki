// W0 验证项 2：官方 Go MCP SDK 能否跑通 server+client 往返。
// 决定 engineering-decisions §2.3 — 用官方 SDK 还是自实现 JSON-RPC。
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type pingArgs struct {
	Name string `json:"name" jsonschema:"the name to echo"`
}

func main() {
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "wikimind-verify", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ping",
		Description: "verification echo tool",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args pingArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "pong: " + args.Name}},
		}, nil, nil
	})

	clientT, serverT := mcp.NewInMemoryTransports()

	go func() {
		if err := server.Run(ctx, serverT); err != nil {
			fmt.Println("server error:", err)
		}
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "verify-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		fmt.Println("✗ Connect 失败:", err)
		os.Exit(1)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		fmt.Println("✗ ListTools 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("  ListTools → %d 个工具\n", len(tools.Tools))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "ping",
		Arguments: map[string]any{"name": "WikiMind"},
	})
	if err != nil {
		fmt.Println("✗ CallTool 失败:", err)
		os.Exit(1)
	}
	if len(res.Content) == 0 {
		fmt.Println("✗ CallTool 返回空 content")
		os.Exit(1)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok || tc.Text != "pong: WikiMind" {
		fmt.Printf("✗ 返回不符预期: %+v\n", res.Content[0])
		os.Exit(1)
	}
	fmt.Printf("  CallTool ping → %q\n", tc.Text)
	fmt.Println("✓ 验证项 2 通过：官方 Go MCP SDK v1.6.1 可用（server+client 往返通）")
}
