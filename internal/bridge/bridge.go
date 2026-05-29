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
//
// Both platforms use an AF_UNIX socket file (Go supports AF_UNIX on
// Windows 10 1803+). We deliberately avoid the Named Pipe namespace
// (\\.\pipe\...) because net.Listen("unix", ...) cannot bind it — that
// mismatch crashed the daemon on Windows (F-041).
//
//   - Unix:    <vault>/.wikimind/daemon.sock
//   - Windows: <TempDir>/wikimind-<base>.sock  (kept short to stay well
//     under the AF_UNIX sun_path ~108-byte limit even for deep vaults)
func SocketPath(vaultRoot string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "wikimind-"+filepath.Base(vaultRoot)+".sock")
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
//
// Uses an AF_UNIX socket on all platforms (see SocketPath). The stale
// socket file is removed first so a crashed daemon doesn't block rebind.
func Listen(socketPath string) (*Listener, error) {
	// Remove stale socket file (all platforms — Windows now uses a real file).
	_ = os.Remove(socketPath)
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir for socket: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", socketPath, err)
	}
	return &Listener{Listener: ln, path: socketPath}, nil
}

// Close closes the listener and removes the socket file.
func (l *Listener) Close() error {
	err := l.Listener.Close()
	_ = os.Remove(l.path)
	return err
}

// Dial connects to the daemon IPC socket.
func Dial(ctx context.Context, socketPath string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial daemon: %w", err)
	}
	return conn, nil
}
