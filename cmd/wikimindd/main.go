// Command wikimindd 是 WikiMind 的常驻 daemon。
//
// W0 骨架：仅打印占位信息。daemon 主循环、单写者 commit loop、watcher、MCP
// bridge 在 roadmap D1+ 实现。
package main

import "fmt"

// version 由构建时注入；W0 暂为占位。
const version = "0.0.0-w0"

func main() {
	fmt.Printf("wikimindd %s — WikiMind daemon (W0 skeleton)\n", version)
}
