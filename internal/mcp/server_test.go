package mcp

import (
	"context"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/vault"
)

// TestNewServerRejectsBadInputs 校验构造期参数防线：空 vaultRoot / 空 db
// 都必须返回 ErrServerBuild 而不是 panic。
func TestNewServerRejectsBadInputs(t *testing.T) {
	if _, err := NewServer(context.Background(), "", nil); err == nil {
		t.Fatal("NewServer(\"\", nil) returned no error")
	}
	tempVault := setupVault(t)
	if _, err := NewServer(context.Background(), tempVault, nil); err == nil {
		t.Fatal("NewServer with nil db returned no error")
	}
}

// TestNewServerListsFourTools 跑 in-memory transport 上 server↔client
// 全链路，验证 4 个只读 tool 都被注册。
func TestNewServerListsFourTools(t *testing.T) {
	ctx := context.Background()
	root, db := setupVaultWithIndex(t)
	t.Cleanup(func() { _ = db.Close() })

	server, err := NewServer(ctx, root, db)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	tools := listToolsViaTransport(t, ctx, server)
	wantNames := map[string]bool{
		"wiki_info":  false,
		"read_page":  false,
		"read_raw":   false,
		"list_index": false,
	}
	for _, tool := range tools {
		if _, ok := wantNames[tool.Name]; ok {
			wantNames[tool.Name] = true
		}
		// mcp-tools.md §0 要求 9 个 read 类 tool 全部带 readOnlyHint: true，
		// 让 MCP host 跳过 user confirmation。D8 只上 4 个，但每个都要打上。
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q missing ReadOnlyHint=true annotation", tool.Name)
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
}

// TestRunStdioRejectsNilServer 防御 RunStdio 被 misuse；ctx 还没 cancel，
// 这条快速失败让上层早断。
func TestRunStdioRejectsNilServer(t *testing.T) {
	if err := RunStdio(context.Background(), nil); err == nil {
		t.Fatal("RunStdio(nil) returned no error")
	}
}

// listToolsViaTransport 启动 in-memory transport 上的 server，建一个 client
// session 跑一次 ListTools——封装让测试避免重复样板。
func listToolsViaTransport(t *testing.T, ctx context.Context, server *sdk.Server) []*sdk.Tool {
	t.Helper()
	clientT, serverT := sdk.NewInMemoryTransports()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, serverT)
	}()
	t.Cleanup(func() {
		// drain run goroutine; we ignore err — client.Close ends Run.
		<-errCh
	})

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	return result.Tools
}

// setupVault 建一个空 vault 并返回绝对路径。
func setupVault(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "vault")
	if _, err := vault.Init(root); err != nil {
		t.Fatalf("vault.Init: %v", err)
	}
	return root
}

// setupVaultWithIndex 在 setupVault 之上 Open index db；caller 负责 close。
func setupVaultWithIndex(t *testing.T) (string, *index.DB) {
	t.Helper()
	root := setupVault(t)
	db, err := index.Open(root)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	return root, db
}
