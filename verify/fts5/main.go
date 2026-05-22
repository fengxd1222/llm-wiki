// W0 验证项 1：纯 Go SQLite 驱动 + trigram FTS5 对 CJK 子串搜索是否可用。
// 决定 engineering-decisions §4.3 的 SQLite 驱动选型。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	must(err)
	defer db.Close()

	var ver string
	must(db.QueryRow("SELECT sqlite_version()").Scan(&ver))
	fmt.Println("SQLite:", ver, "(modernc.org/sqlite, 纯 Go, 无 cgo)")

	if _, err := db.Exec(
		`CREATE VIRTUAL TABLE pages_fts USING fts5(title, body, tokenize='trigram')`,
	); err != nil {
		fmt.Println("✗ trigram FTS5 建表失败:", err)
		os.Exit(1)
	}
	must2(db.Exec(`INSERT INTO pages_fts(title, body) VALUES (?,?)`,
		"Wiki 是一个 compounding artifact",
		"每一次 ingest、每一次 query 都让 wiki 更值钱"))

	pass := true
	check := func(q string, want bool) {
		var n int
		var err error
		if utf8.RuneCountInString(q) >= 3 {
			err = db.QueryRow(
				`SELECT count(*) FROM pages_fts WHERE pages_fts MATCH ?`, q,
			).Scan(&n)
		} else {
			// 短查询（< 3 字符）fallback LIKE，见 cjk-tokenizer.md §3.3
			err = db.QueryRow(
				`SELECT count(*) FROM pages_fts WHERE body LIKE ? OR title LIKE ?`,
				"%"+q+"%", "%"+q+"%",
			).Scan(&n)
		}
		got := err == nil && n > 0
		st := "OK"
		if got != want {
			st = "FAIL"
			pass = false
		}
		fmt.Printf("  [%s] %-14q hit=%-5v want=%v\n", st, q, got, want)
	}

	check("更值钱", true)       // 3 字符中文 → trigram MATCH
	check("每一次", true)       // 3 字符中文 → trigram MATCH
	check("compounding", true) // 英文
	check("值钱", true)         // 2 字符 → LIKE fallback
	check("不存在xyz", false)   // 负例

	if !pass {
		fmt.Println("✗ 验证项 1 失败 — fallback 到 mattn/go-sqlite3（cgo）")
		os.Exit(1)
	}
	fmt.Println("✓ 验证项 1 通过：纯 Go SQLite + trigram FTS5 对 CJK 可用")
}

func must(err error) {
	if err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

func must2(_ any, err error) { must(err) }
