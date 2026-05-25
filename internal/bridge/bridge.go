package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// SocketPath returns the IPC socket path for a vault.
// Unix: <vault>/.wikimind/daemon.sock
// Windows: \\.\pipe\wikimind-<hash>
func SocketPath(vaultRoot string) string {
	if runtime.GOOS == "windows" {
		// Use a named pipe on Windows.
		return `\\.\pipe\wikimind-` + filepath.Base(vaultRoot)
	}
	return filepath.Join(vaultRoot, ".wikimind", "daemon.sock")
}

// Request is a CLI-to-daemon IPC message.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is a daemon-to-CLI IPC reply.
type Response struct {
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// Listener wraps a net.Listener for the IPC socket.
type Listener struct {
	net.Listener
	path string
}

// Listen creates an IPC listener at the given socket path.
func Listen(socketPath string) (*Listener, error) {
	// Remove stale socket file.
	if runtime.GOOS != "windows" {
		_ = os.Remove(socketPath)
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir for socket: %w", err)
	}

	network := "unix"
	if runtime.GOOS == "windows" {
		network = "unix" // Go supports unix sockets on Windows 10+
	}

	ln, err := net.Listen(network, socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", socketPath, err)
	}
	return &Listener{Listener: ln, path: socketPath}, nil
}

// Close closes the listener and removes the socket file.
func (l *Listener) Close() error {
	err := l.Listener.Close()
	if runtime.GOOS != "windows" {
		_ = os.Remove(l.path)
	}
	return err
}

// Dial connects to the daemon IPC socket.
func Dial(ctx context.Context, socketPath string) (net.Conn, error) {
	network := "unix"
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	return conn, nil
}
