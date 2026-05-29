package bridge

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestListenAndDial(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	ln, err := Listen(socketPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	// Accept in background.
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	// Dial.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := Dial(ctx, socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Wait for server side.
	select {
	case serverConn := <-accepted:
		defer serverConn.Close()
	case <-time.After(2 * time.Second):
		t.Fatalf("accept timeout")
	}
}

func TestRequestResponseRoundtrip(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "rpc.sock")

	ln, err := Listen(socketPath)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	// Server: read request, write response.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var req Request
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			return
		}
		resp := Response{OK: true, Result: json.RawMessage(`"pong"`)}
		_ = json.NewEncoder(conn).Encode(resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := Dial(ctx, socketPath)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Send request.
	req := Request{Method: "ping"}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	// Read response.
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}
	if string(resp.Result) != `"pong"` {
		t.Fatalf("result = %s, want \"pong\"", resp.Result)
	}
}

func TestSocketPath(t *testing.T) {
	path := SocketPath("/tmp/my-vault")
	if path == "" {
		t.Fatalf("SocketPath returned empty")
	}
	if runtime.GOOS == "windows" {
		// Windows uses a file-based AF_UNIX socket under TempDir, never the
		// named-pipe namespace (F-041).
		if strings.HasPrefix(path, `\\.\pipe\`) {
			t.Fatalf("SocketPath = %s, must not use named-pipe namespace", path)
		}
		if !strings.HasSuffix(path, ".sock") {
			t.Fatalf("SocketPath = %s, want .sock suffix", path)
		}
	} else {
		if filepath.Dir(path) != "/tmp/my-vault/.wikimind" {
			t.Fatalf("SocketPath = %s, want under /tmp/my-vault/.wikimind", path)
		}
	}
}
