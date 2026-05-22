// W0 验证项 3：跨进程 IPC（Unix domain socket + JSON line）— daemon ⇄ bridge echo。
// 验证 engineering-decisions §2.4 的 bridge⇄daemon 通信模型。
//
// 无参数运行 = server：监听 socket，spawn 自身作为 client 子进程，echo 一次往返。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type msg struct {
	Method string `json:"method"`
	Text   string `json:"text"`
}

func sockPath() string { return filepath.Join(os.TempDir(), "wm-ipc-verify.sock") }

func main() {
	if len(os.Args) > 1 && os.Args[1] == "client" {
		runClient()
		return
	}
	runServer()
}

func runServer() {
	sock := sockPath()
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		fmt.Println("✗ listen 失败:", err)
		os.Exit(1)
	}
	defer ln.Close()
	defer os.Remove(sock)

	cmd := exec.Command(os.Args[0], "client")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Println("✗ spawn client 失败:", err)
		os.Exit(1)
	}

	_ = ln.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	conn, err := ln.Accept()
	if err != nil {
		fmt.Println("✗ accept 失败:", err)
		os.Exit(1)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		fmt.Println("✗ server 读取失败:", err)
		os.Exit(1)
	}
	var m msg
	_ = json.Unmarshal(line, &m)
	resp, _ := json.Marshal(msg{Method: "echo", Text: m.Text})
	_, _ = conn.Write(append(resp, '\n'))
	_ = conn.Close()

	if err := cmd.Wait(); err != nil {
		fmt.Println("✗ client 子进程退出异常:", err)
		os.Exit(1)
	}
	fmt.Println("✓ 验证项 3 通过：跨进程 Unix socket IPC 往返通")
}

func runClient() {
	sock := sockPath()
	var conn net.Conn
	var err error
	for i := 0; i < 100; i++ {
		if conn, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		fmt.Println("✗ client dial 失败:", err)
		os.Exit(1)
	}
	defer conn.Close()

	req, _ := json.Marshal(msg{Method: "ping", Text: "hello-from-bridge"})
	if _, err := conn.Write(append(req, '\n')); err != nil {
		fmt.Println("✗ client 写入失败:", err)
		os.Exit(1)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		fmt.Println("✗ client 读取失败:", err)
		os.Exit(1)
	}
	var m msg
	_ = json.Unmarshal(line, &m)
	if m.Text != "hello-from-bridge" {
		fmt.Printf("✗ echo 不符: %q\n", m.Text)
		os.Exit(1)
	}
	fmt.Printf("  bridge → daemon → echo: %q ✓\n", m.Text)
}
