package mcp

// 本文件定义 4 个只读 tool 的 Request / Response 类型。
//
// 字段命名与 spec-v2/docs/mcp-tools.md §2-4 §7 的 JSON schema 一一对应。
// 使用 jsonschema struct tag 让 go-sdk 自动推断 input schema。

// WikiInfoArgs 是 wiki_info 的输入；按 schema 是空对象。
//
// 必须是 struct（不能是 any），AddTool 才能推断出 "type: object" 的空对象 schema。
type WikiInfoArgs struct{}

// CountsBlock 镜像 mcp-tools.md §2 counts.* 字段。
type CountsBlock struct {
	RawSources     int `json:"raw_sources"`
	WikiPages      int `json:"wiki_pages"`
	Claims         int `json:"claims"`
	Entities       int `json:"entities"`
	Concepts       int `json:"concepts"`
	PendingReviews int `json:"pending_reviews"`
}

// HealthBlock 镜像 mcp-tools.md §2 health.* 字段。
//
// D8 阶段 score / drift_claims / lint_warnings 全为占位（W3 lint / claim drift
// 上线后再填真值）。
type HealthBlock struct {
	Score        int `json:"score"`
	DriftClaims  int `json:"drift_claims"`
	LintWarnings int `json:"lint_warnings"`
}

// WikiInfoResult 镜像 mcp-tools.md §2 wiki_info response。
type WikiInfoResult struct {
	VaultRoot     string      `json:"vault_root"`
	SchemaVersion string      `json:"schema_version"`
	DaemonVersion string      `json:"daemon_version"`
	Counts        CountsBlock `json:"counts"`
	Health        HealthBlock `json:"health"`
}

// ReadPageArgs 镜像 mcp-tools.md §3 read_page input。
//
// PageID 可以是纯 id（如 "cl-2026-05-21-001"）或 vault-relative path
// （如 "wiki/claims/foo.md"）。include_history / include_backlinks 在 D8
// 阶段返回空数组 + note —— 不实际查询，是 staged 行为。
type ReadPageArgs struct {
	PageID           string `json:"page_id" jsonschema:"page id or vault-relative path"`
	IncludeHistory   bool   `json:"include_history,omitempty" jsonschema:"include git history (W2 D9+ staged)"`
	IncludeBacklinks bool   `json:"include_backlinks,omitempty" jsonschema:"include backlinks (W2 D10+ staged)"`
}

// ReadPageResult 是 read_page 的输出。
//
// Frontmatter 为 JSON 字符串（保持与 pages.frontmatter 列一致）；调用方需要
// 二次 json.Unmarshal —— D8 不强制 typed map，避免 schema 漂移。
type ReadPageResult struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	Confidence    *float64 `json:"confidence,omitempty"`
	Status        string   `json:"status,omitempty"`
	SchemaVersion string   `json:"schema_version"`
	Frontmatter   string   `json:"frontmatter,omitempty"`
	History       []any    `json:"history"`
	HistoryNote   string   `json:"history_note,omitempty"`
	Backlinks     []any    `json:"backlinks"`
	BacklinksNote string   `json:"backlinks_note,omitempty"`
}

// ReadRawArgs 镜像 mcp-tools.md §4 read_raw input。
//
// Format 仅接受 "raw" / "normalized"；D8 阶段 normalized 友好拒绝。空字符串
// 默认 "raw"（spec 写的是 normalized 默认，但 D8 normalized 未实现，所以
// CLI 用 raw 做兜底——避免空 input 直接 ErrFormatUnsupported）。
type ReadRawArgs struct {
	RawID  string `json:"raw_id" jsonschema:"vault-relative path under raw/, e.g. raw/inbox/x.md"`
	Format string `json:"format,omitempty" jsonschema:"raw|normalized; D8 only supports raw"`
}

// ReadRawResult 是 read_raw 的输出。
//
// Encoding="base64" 表示 Content 是 base64 编码的 binary；空表示 utf-8 文本。
// Bytes 是原始字节数；Format 回显请求的 format。
type ReadRawResult struct {
	RawID    string `json:"raw_id"`
	Format   string `json:"format"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"`
	Bytes    int    `json:"bytes"`
}

// ListIndexArgs 镜像 mcp-tools.md §7 list_index input。
//
// Type 取空表示 all；服务端把 "all" / "" 都视作 nil filter。
// Limit / Offset 用指针让客户端能区分"未传"与"传 0"；nil 时取默认 100 / 0。
type ListIndexArgs struct {
	Type   string `json:"type,omitempty" jsonschema:"all|claim|entity|concept|source|topic"`
	Prefix string `json:"prefix,omitempty" jsonschema:"vault-relative path prefix filter"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"max items to return (default 100)"`
	Offset *int   `json:"offset,omitempty" jsonschema:"items to skip (default 0)"`
}

// IndexItem 是 list_index 数组元素。
//
// Confidence / Status 在 W2 D11 propose_claim 上线后才有真值；D8 留空。
type IndexItem struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Path       string   `json:"path"`
	Title      string   `json:"title"`
	Confidence *float64 `json:"confidence,omitempty"`
	Status     string   `json:"status,omitempty"`
}

// ListIndexResult 是 list_index 的输出。
//
// Total 是 prefix / type 过滤后的总数（不是切片后的数量），让客户端能做分页。
type ListIndexResult struct {
	Total int          `json:"total"`
	Items []*IndexItem `json:"items"`
}
