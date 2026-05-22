// Command wikimind 是 WikiMind 的命令行入口。
//
// W0 骨架：仅打印占位信息。CLI 子命令（init / status / ingest / query / review /
// lint / revert）在 roadmap D1+ 用 cobra 实现。
package main

import "fmt"

// version 由构建时注入；W0 暂为占位。
const version = "0.0.0-w0"

func main() {
	fmt.Printf("wikimind %s — WikiMind CLI (W0 skeleton)\n", version)
}
