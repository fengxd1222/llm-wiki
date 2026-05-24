package mcp

// AgentHandshakeArgs 镜像 mcp-tools.md §1 agent_handshake input。
type AgentHandshakeArgs struct {
	Agent                 string   `json:"agent" jsonschema:"agent name"`
	Version               string   `json:"version" jsonschema:"agent semver"`
	SessionID             string   `json:"session_id" jsonschema:"client-generated session id"`
	Capabilities          []string `json:"capabilities,omitempty" jsonschema:"read|propose|lint|merge"`
	DeclaresSchemaVersion string   `json:"declares_schema_version" jsonschema:"declared schema version, e.g. 1.0"`
}

// RateLimitsBlock mirrors the staged D10 handshake rate limits response.
type RateLimitsBlock struct {
	ProposePerMinute int `json:"propose_per_minute"`
	QueryPerMinute   int `json:"query_per_minute"`
}

// QueueStateBlock mirrors the staged D10 review queue state response.
type QueueStateBlock struct {
	Pending    int  `json:"pending"`
	HardLimit  int  `json:"hard_limit"`
	CanPropose bool `json:"can_propose"`
}

// AgentHandshakeResult 镜像 mcp-tools.md §1 agent_handshake response。
type AgentHandshakeResult struct {
	Accepted             bool            `json:"accepted"`
	DaemonSchemaVersion  string          `json:"daemon_schema_version"`
	Worktree             string          `json:"worktree,omitempty"`
	InstructionsToRead   []string        `json:"instructions_to_read"`
	SessionToken         string          `json:"session_token,omitempty"`
	RateLimits           RateLimitsBlock `json:"rate_limits"`
	QueueState           QueueStateBlock `json:"queue_state"`
	AcceptedCapabilities []string        `json:"accepted_capabilities,omitempty"`
}

// 本文件定义只读 tool 的 Request / Response 类型。
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

// ReadRawAnchorArgs 镜像 mcp-tools.md §5 read_raw_anchor input。
type ReadRawAnchorArgs struct {
	RawID  string `json:"raw_id" jsonschema:"vault-relative path under raw/, e.g. raw/inbox/x.md"`
	Anchor string `json:"anchor" jsonschema:"format: #heading-slug | #para-N | #char[start:end]"`
}

// ReadRawAnchorResult 是 read_raw_anchor 的输出。
type ReadRawAnchorResult struct {
	RawID        string `json:"raw_id"`
	Anchor       string `json:"anchor"`
	Content      string `json:"content"`
	QuoteHash    string `json:"quote_hash"`
	Span         [2]int `json:"span"`
	SourceMTime  string `json:"source_mtime"`
	SourceSHA256 string `json:"source_sha256"`
}

// ReadClaimArgs 镜像 mcp-tools.md §6 read_claim input。
type ReadClaimArgs struct {
	ClaimID string `json:"claim_id" jsonschema:"claim page id"`
}

// ClaimSourceStatus 是 read_claim.sources 的元素。
type ClaimSourceStatus struct {
	RawID            string `json:"raw_id"`
	Anchor           string `json:"anchor"`
	StoredQuoteHash  string `json:"stored_quote_hash"`
	CurrentQuoteHash string `json:"current_quote_hash"`
	DriftStatus      string `json:"drift_status"`
	SourceModifiedAt string `json:"source_modified_at,omitempty"`
}

// ReadClaimResult 是 read_claim 的输出；D9 sources staged 为空数组。
type ReadClaimResult struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	Path          string              `json:"path"`
	Title         string              `json:"title"`
	Body          string              `json:"body"`
	Confidence    *float64            `json:"confidence,omitempty"`
	Status        string              `json:"status,omitempty"`
	SchemaVersion string              `json:"schema_version"`
	Frontmatter   string              `json:"frontmatter,omitempty"`
	Sources       []ClaimSourceStatus `json:"sources"`
	SourcesNote   string              `json:"sources_note,omitempty"`
}

// SearchArgs 镜像 mcp-tools.md §8 search input。
type SearchArgs struct {
	Query  string        `json:"query"`
	Type   string        `json:"type,omitempty" jsonschema:"fts|fts+vector"`
	Filter *SearchFilter `json:"filter,omitempty"`
	Limit  *int          `json:"limit,omitempty"`
}

// SearchFilter 是 search.filter 的输入。
type SearchFilter struct {
	PageType      []string `json:"page_type,omitempty"`
	MinConfidence *float64 `json:"min_confidence,omitempty"`
	Status        []string `json:"status,omitempty"`
	UpdatedSince  string   `json:"updated_since,omitempty" jsonschema:"date-time"`
}

// SearchResultItem 是 search.results 的元素。
type SearchResultItem struct {
	PageID     string   `json:"page_id"`
	Title      string   `json:"title"`
	Snippet    string   `json:"snippet"`
	Score      float64  `json:"score"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// SearchResult 是 search 的输出。
type SearchResult struct {
	Results       []SearchResultItem `json:"results"`
	TokenizerUsed string             `json:"tokenizer_used"`
	QueryTimeMS   int64              `json:"query_time_ms"`
	Warnings      []string           `json:"warnings,omitempty"`
	Notes         []string           `json:"notes,omitempty"`
}

// GraphNeighborsArgs 镜像 mcp-tools.md §9 graph_neighbors input。
type GraphNeighborsArgs struct {
	PageID    string   `json:"page_id"`
	Direction string   `json:"direction,omitempty" jsonschema:"out|in|both"`
	Depth     *int     `json:"depth,omitempty" jsonschema:"default 1; D9 supports only 1"`
	LinkTypes []string `json:"link_types,omitempty"`
}

// GraphNeighbor 是 graph_neighbors.neighbors 的元素。
type GraphNeighbor struct {
	PageID   string `json:"page_id"`
	Title    string `json:"title,omitempty"`
	LinkType string `json:"link_type"`
}

// GraphNeighborsResult 是 graph_neighbors 的输出。
type GraphNeighborsResult struct {
	Neighbors []GraphNeighbor `json:"neighbors"`
	Notes     []string        `json:"notes,omitempty"`
}

// GetHistoryArgs 镜像 mcp-tools.md §10 get_history input。
type GetHistoryArgs struct {
	PageID string `json:"page_id"`
	Limit  *int   `json:"limit,omitempty"`
}

// HistoryCommit 是 get_history.commits 的元素。
type HistoryCommit struct {
	SHA         string `json:"sha"`
	TS          string `json:"ts"`
	Actor       string `json:"actor"`
	Op          string `json:"op"`
	BundleID    string `json:"bundle_id,omitempty"`
	Summary     string `json:"summary"`
	DiffSummary string `json:"diff_summary"`
}

// GetHistoryResult 是 get_history 的输出。
type GetHistoryResult struct {
	Commits []HistoryCommit `json:"commits"`
}
