package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fengxd1222/llm-wiki/internal/commit"
	"github.com/fengxd1222/llm-wiki/internal/index"
	"github.com/fengxd1222/llm-wiki/internal/proposal"
	"github.com/fengxd1222/llm-wiki/internal/service"
	"github.com/fengxd1222/llm-wiki/internal/vault"
	worktreepkg "github.com/fengxd1222/llm-wiki/internal/worktree"
)

// ErrFormatUnsupported 表示客户端请求了 D8 尚未实现的 read_raw format
// （目前仅 "normalized" 触发，等 W2 D9 read_raw_anchor 一起上线 stage-2 parse）。
var ErrFormatUnsupported = errors.New("format unsupported")

// ErrRawIDOutsideRaw 表示 read_raw 的 raw_id 没指向 raw/ 子树。
//
// 严格只允许 raw/ 下访问——避免把 wiki/ 当 raw 读乱权限边界。
var ErrRawIDOutsideRaw = errors.New("raw_id must be under raw/")

// ErrPageNotFound 表示 read_page 在 pages 表（id 查）和 wiki/ 文件树
// （path 查）都没命中。
var ErrPageNotFound = errors.New("page not found")

// ErrRawNotFound 表示 read_raw 解析后路径在 vault 内但文件不存在。
var ErrRawNotFound = errors.New("raw file not found")

// ErrClaimNotFound 表示 read_claim 没找到 type=claim 的 page。
var ErrClaimNotFound = errors.New("claim not found")

// ErrDepthUnsupported 表示 graph_neighbors 收到 D9 尚不支持的 depth。
var ErrDepthUnsupported = errors.New("graph depth unsupported")

// Handshake error codes are intentionally uppercase because MCP clients parse
// them from tool error content.
var (
	ErrAgentNotWhitelisted  = errors.New("AGENT_NOT_WHITELISTED")
	ErrSchemaIncompatible   = errors.New("SCHEMA_INCOMPATIBLE")
	ErrWorktreeCreateFailed = errors.New("WORKTREE_CREATE_FAILED")
)

// daemonVersion 在 wiki_info 响应中返回；与 cmd/wikimind 的 version 解耦
// 一些——MCP 进程语义上是 daemon 角色（D10+ 由真正 daemon 接管前 staged）。
const daemonVersion = "0.1.0-w2"

// schemaVersion 是 MCP 协议宣称的 vault schema 版本——优先从 vault config
// 读真值；读取失败时退回此常量保证 wiki_info 总有响应。
const fallbackSchemaVersion = "1.0"

// historyNote / backlinksNote 给 read_page 的 staged 字段一个稳定的解释，
// 让 LLM agent 看到空数组不会以为是 bug。
const (
	historyNote   = "history requires git log integration (W2 D9+)"
	backlinksNote = "backlinks require page_links table (W2 D10+)"
)

// formatNormalizedNote 留给 read_raw 拒绝 normalized 时返回，提示用户改用
// D9 的 anchor 读取路径。
const formatNormalizedNote = "normalized read_raw is not exposed; use read_raw_anchor for stage-2 anchored reads"

const (
	claimSourcesStagedNote = "claim source validation requires claim_sources table (W2 D11+ propose_claim)"
	inboundLinksStagedNote = "inbound links require page_links table (W2 D11+)"
	minConfidenceNote      = "min_confidence filter requires claims confidence field (W2 D11+)"
	vectorSearchWarning    = "fts+vector requested; embeddings are not available yet, downgraded to fts"
)

// vaultBackend 把 MCP tool 需要的依赖收拢为一个结构，让 server 构造时
// 一次性传入；handler 通过闭包持有，避免每次 tool 调用走 context value 取值。
type vaultBackend struct {
	root     string
	db       *index.DB
	sessions *SessionStore
}

func (b *vaultBackend) sessionStore() *SessionStore {
	if b.sessions == nil {
		b.sessions = NewSessionStore()
	}
	return b.sessions
}

// handleAgentHandshake implements mcp-tools.md §1.
func (b *vaultBackend) handleAgentHandshake(ctx context.Context, args AgentHandshakeArgs) (AgentHandshakeResult, error) {
	args.Agent = strings.TrimSpace(args.Agent)
	args.Version = strings.TrimSpace(args.Version)
	args.SessionID = strings.TrimSpace(args.SessionID)
	args.DeclaresSchemaVersion = strings.TrimSpace(args.DeclaresSchemaVersion)
	if args.Agent == "" {
		return AgentHandshakeResult{}, errors.New("agent_handshake: agent is required")
	}
	if args.Version == "" {
		return AgentHandshakeResult{}, errors.New("agent_handshake: version is required")
	}
	if args.SessionID == "" {
		return AgentHandshakeResult{}, errors.New("agent_handshake: session_id is required")
	}
	if args.DeclaresSchemaVersion == "" {
		return AgentHandshakeResult{}, errors.New("agent_handshake: declares_schema_version is required")
	}

	cfg, err := vault.LoadConfig(b.root)
	if err != nil {
		return AgentHandshakeResult{}, fmt.Errorf("agent_handshake: load config: %w", err)
	}
	daemonSchema := cfg.SchemaVersion
	if daemonSchema == "" {
		daemonSchema = fallbackSchemaVersion
	}
	pending, err := index.CountReviewsByStatus(ctx, b.db, "pending")
	if err != nil {
		return AgentHandshakeResult{}, fmt.Errorf("agent_handshake: queue state: %w", err)
	}
	base := AgentHandshakeResult{
		DaemonSchemaVersion: daemonSchema,
		InstructionsToRead: []string{
			"schema/AGENTS.md",
			"schema/CLAUDE.md",
			"schema/page-schemas.md",
		},
		RateLimits: RateLimitsBlock{
			ProposePerMinute: 30,
			QueryPerMinute:   60,
		},
		QueueState: QueueStateBlock{
			Pending:   pending,
			HardLimit: 50,
		},
	}

	if !agentAllowed(args.Agent, cfg.AllowedAgents) {
		return AgentHandshakeResult{}, fmt.Errorf("%w: %s", ErrAgentNotWhitelisted, args.Agent)
	}
	if !schemaMajorCompatible(args.DeclaresSchemaVersion, daemonSchema) {
		base.Accepted = false
		base.AcceptedCapabilities = []string{"read"}
		base.QueueState.CanPropose = false
		return base, nil
	}

	token, err := newSessionToken()
	if err != nil {
		return AgentHandshakeResult{}, err
	}
	now := time.Now().UTC()
	sess := &Session{
		Token:         token,
		Agent:         args.Agent,
		Version:       args.Version,
		SessionID:     args.SessionID,
		Capabilities:  append([]string(nil), args.Capabilities...),
		SchemaVersion: args.DeclaresSchemaVersion,
		CreatedAt:     now,
		LastSeenAt:    now,
		IdleTimeout:   defaultIdleTimeout,
	}
	store := b.sessionStore()
	if err := store.Register(sess); err != nil {
		return AgentHandshakeResult{}, err
	}

	wt, err := worktreepkg.CreateWorktree(ctx, b.root, args.Agent, args.SessionID)
	if err != nil {
		store.remove(token)
		return AgentHandshakeResult{}, fmt.Errorf("%w: %w", ErrWorktreeCreateFailed, err)
	}
	sess.WorktreePath = wt.Path
	sess.Branch = wt.Branch

	rel, err := filepath.Rel(b.root, wt.Path)
	if err != nil {
		rel = filepath.Join("wiki", "_worktrees", filepath.Base(wt.Path))
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasSuffix(rel, "/") {
		rel += "/"
	}

	base.Accepted = true
	base.Worktree = rel
	base.SessionToken = token
	base.QueueState.CanPropose = pending < base.QueueState.HardLimit
	return base, nil
}

func (b *vaultBackend) handleProposePage(ctx context.Context, args ProposePageArgs) (ProposeResult, error) {
	sess, err := b.authenticateWrite(args.SessionToken)
	if err != nil {
		return ProposeResult{}, err
	}
	if existing, err := index.FindReviewByIdempotencyKey(ctx, b.db, sess.Agent, args.IdempotencyKey); err != nil {
		return ProposeResult{}, err
	} else if existing != nil {
		return ProposeResult{
			ReviewID: existing.ID,
			Status:   existing.Status,
			Validations: ValidationBlock{
				SchemaCheck:    "passed",
				QuoteHashCheck: "skipped",
				PathCheck:      "passed",
			},
		}, nil
	}
	if err := proposal.ValidatePath(args.Path, args.Type); err != nil {
		return ProposeResult{}, err
	}
	if err := proposal.ValidateFrontmatter(args.Frontmatter, args.Type); err != nil {
		return ProposeResult{}, err
	}
	if err := b.writePageInWorktree(ctx, sess, args.Path, args.Frontmatter, args.Body); err != nil {
		return ProposeResult{}, err
	}
	patch, err := proposal.GeneratePatch(ctx, sess.WorktreePath, sess.Branch, args.Path)
	if err != nil {
		return ProposeResult{}, err
	}
	review, err := b.insertPatchReview(ctx, sess, "propose_page", args.Path, patch, map[string]any{
		"idempotency_key": args.IdempotencyKey,
		"path":            args.Path,
		"type":            args.Type,
	})
	if err != nil {
		return ProposeResult{}, err
	}
	return ProposeResult{
		ReviewID: review.ID,
		Status:   review.Status,
		Validations: ValidationBlock{
			SchemaCheck:    "passed",
			QuoteHashCheck: "skipped",
			PathCheck:      "passed",
		},
	}, nil
}

func (b *vaultBackend) handleProposeEdit(ctx context.Context, args ProposeEditArgs) (ProposeResult, error) {
	sess, err := b.authenticateWrite(args.SessionToken)
	if err != nil {
		return ProposeResult{}, err
	}
	if existing, err := index.FindReviewByIdempotencyKey(ctx, b.db, sess.Agent, args.IdempotencyKey); err != nil {
		return ProposeResult{}, err
	} else if existing != nil {
		return ProposeResult{ReviewID: existing.ID, Status: existing.Status,
			Validations: ValidationBlock{SchemaCheck: "passed", QuoteHashCheck: "skipped", PathCheck: "passed", BaseHashCheck: "passed"}}, nil
	}
	pagePath, err := b.resolvePagePath(ctx, args.PageID)
	if err != nil {
		return ProposeResult{}, err
	}
	if err := proposal.ValidateBaseHash(ctx, b.root, pagePath, args.BaseHash); err != nil {
		return ProposeResult{}, err
	}
	if strings.TrimSpace(args.Patch.UnifiedDiff) != "" {
		if err := proposal.ApplyPatch(ctx, sess.WorktreePath, []byte(args.Patch.UnifiedDiff)); err != nil {
			return ProposeResult{}, err
		}
	} else {
		if err := b.applyFrontmatterBodyEdit(ctx, sess, pagePath, args.Patch.FrontmatterChanges, args.Patch.Body); err != nil {
			return ProposeResult{}, err
		}
	}
	patch, err := proposal.GeneratePatch(ctx, sess.WorktreePath, sess.Branch, pagePath)
	if err != nil {
		return ProposeResult{}, err
	}
	review, err := b.insertPatchReview(ctx, sess, "propose_edit", args.PageID, patch, map[string]any{
		"idempotency_key": args.IdempotencyKey,
		"page_id":         args.PageID,
		"path":            pagePath,
		"summary":         args.Summary,
	})
	if err != nil {
		return ProposeResult{}, err
	}
	return ProposeResult{
		ReviewID: review.ID,
		Status:   review.Status,
		Validations: ValidationBlock{
			SchemaCheck:    "passed",
			QuoteHashCheck: "skipped",
			PathCheck:      "passed",
			BaseHashCheck:  "passed",
		},
	}, nil
}

func (b *vaultBackend) handleProposeClaim(ctx context.Context, args ProposeClaimArgs) (ProposeResult, error) {
	sess, err := b.authenticateWrite(args.SessionToken)
	if err != nil {
		return ProposeResult{}, err
	}
	if existing, err := index.FindReviewByIdempotencyKey(ctx, b.db, sess.Agent, args.IdempotencyKey); err != nil {
		return ProposeResult{}, err
	} else if existing != nil {
		return ProposeResult{ReviewID: existing.ID, Status: existing.Status,
			Validations: ValidationBlock{SchemaCheck: "passed", QuoteHashCheck: "passed", PathCheck: "passed"}}, nil
	}
	if err := proposal.ValidateClaimID(args.ClaimID); err != nil {
		return ProposeResult{}, err
	}
	if strings.TrimSpace(args.Title) == "" || len([]rune(args.Title)) > 100 {
		return ProposeResult{}, fmt.Errorf("%w: invalid claim title", proposal.ErrSchemaViolation)
	}
	if args.Confidence < 0 || args.Confidence > 1 {
		return ProposeResult{}, fmt.Errorf("%w: confidence out of range", proposal.ErrSchemaViolation)
	}
	sources := claimSourcesToProposal(args.Sources)
	if len(sources) == 0 && !args.Speculation {
		return ProposeResult{}, fmt.Errorf("%w: sources required", proposal.ErrSchemaViolation)
	}
	if len(sources) > 0 {
		if err := proposal.ValidateClaimSources(ctx, b.root, sources); err != nil {
			return ProposeResult{}, err
		}
	}
	status := strings.TrimSpace(args.Status)
	if status == "" {
		if args.Speculation {
			status = "speculation"
		} else {
			status = "unverified"
		}
	}
	if !validClaimStatus(status) {
		return ProposeResult{}, fmt.Errorf("%w: invalid claim status %q", proposal.ErrSchemaViolation, args.Status)
	}
	path := "wiki/claims/" + args.ClaimID + ".md"
	fm := map[string]any{
		"id":             args.ClaimID,
		"type":           "claim",
		"title":          args.Title,
		"schema_version": fallbackSchemaVersion,
		"confidence":     args.Confidence,
		"status":         status,
		"sources":        args.Sources,
	}
	if args.Speculation {
		fm["speculation"] = true
	}
	if len(args.Related) > 0 {
		fm["related"] = args.Related
	}
	if err := proposal.ValidatePath(path, "claim"); err != nil {
		return ProposeResult{}, err
	}
	if err := proposal.ValidateFrontmatter(fm, "claim"); err != nil {
		return ProposeResult{}, err
	}
	if err := b.writePageInWorktree(ctx, sess, path, fm, args.Body); err != nil {
		return ProposeResult{}, err
	}
	patch, err := proposal.GeneratePatch(ctx, sess.WorktreePath, sess.Branch, path)
	if err != nil {
		return ProposeResult{}, err
	}
	review, err := b.insertPatchReview(ctx, sess, "propose_claim", args.ClaimID, patch, map[string]any{
		"idempotency_key": args.IdempotencyKey,
		"claim_id":        args.ClaimID,
		"path":            path,
		"confidence":      args.Confidence,
	})
	if err != nil {
		return ProposeResult{}, err
	}
	return ProposeResult{
		ReviewID: review.ID,
		Status:   review.Status,
		Validations: ValidationBlock{
			SchemaCheck:    "passed",
			QuoteHashCheck: "passed",
			PathCheck:      "passed",
		},
	}, nil
}

func (b *vaultBackend) handleRequestReview(ctx context.Context, args RequestReviewArgs) (RequestReviewResult, error) {
	sess, err := b.authenticateWrite(args.SessionToken)
	if err != nil {
		return RequestReviewResult{}, err
	}
	if len(args.ReviewIDs) == 0 {
		return RequestReviewResult{}, fmt.Errorf("%w: review_ids required", proposal.ErrSchemaViolation)
	}
	title := strings.TrimSpace(args.Title)
	if title == "" {
		return RequestReviewResult{}, fmt.Errorf("%w: title required", proposal.ErrSchemaViolation)
	}
	kind := strings.TrimSpace(args.Kind)
	if !validReviewKind(kind) {
		return RequestReviewResult{}, fmt.Errorf("%w: invalid review kind %q", proposal.ErrSchemaViolation, args.Kind)
	}
	priorityHint := strings.TrimSpace(args.PriorityHint)
	if priorityHint == "" {
		priorityHint = "normal"
	}
	if !validPriorityHint(priorityHint) {
		return RequestReviewResult{}, fmt.Errorf("%w: invalid priority_hint %q", proposal.ErrSchemaViolation, args.PriorityHint)
	}
	for _, id := range args.ReviewIDs {
		review, err := index.GetReviewByID(ctx, b.db, id)
		if err != nil {
			return RequestReviewResult{}, err
		}
		if review.Agent != sess.Agent || review.SessionID != sess.SessionID {
			return RequestReviewResult{}, errors.New("CROSS_SESSION_BUNDLE")
		}
		if review.Status != "pending" {
			return RequestReviewResult{}, fmt.Errorf("%w: review %s is %s", proposal.ErrSchemaViolation, id, review.Status)
		}
		if review.BundleID != "" {
			return RequestReviewResult{}, errors.New("REVIEW_ALREADY_BUNDLED")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	seq, err := index.NextBundleSeq(ctx, b.db)
	if err != nil {
		return RequestReviewResult{}, err
	}
	bundleID := index.BundleID(seq)
	if err := index.InsertBundle(ctx, b.db, &index.BundleRow{
		ID:          bundleID,
		Seq:         seq,
		Agent:       sess.Agent,
		SessionID:   sess.SessionID,
		Summary:     title,
		Status:      "submitted",
		CreatedAt:   now,
		SubmittedAt: now,
	}); err != nil {
		return RequestReviewResult{}, err
	}
	if err := index.AssignReviewsToBundle(ctx, b.db, bundleID, args.ReviewIDs); err != nil {
		return RequestReviewResult{}, err
	}
	openBundles, err := index.CountBundlesByStatus(ctx, b.db, "submitted")
	if err != nil {
		return RequestReviewResult{}, err
	}
	score := priorityScore(kind, priorityHint, len(args.ReviewIDs))
	return RequestReviewResult{
		BundleID:      bundleID,
		ReviewIDs:     append([]string(nil), args.ReviewIDs...),
		PriorityScore: score,
		QueuePosition: openBundles,
	}, nil
}

func (b *vaultBackend) handleLogAppend(ctx context.Context, args LogAppendArgs) (LogAppendResult, error) {
	sess, err := b.authenticateWrite(args.SessionToken)
	if err != nil {
		return LogAppendResult{}, err
	}
	if !validLogCategory(args.Category) {
		return LogAppendResult{}, fmt.Errorf("%w: invalid category %q", proposal.ErrSchemaViolation, args.Category)
	}
	message := strings.TrimSpace(args.Message)
	if message == "" || len([]rune(message)) > 500 {
		return LogAppendResult{}, fmt.Errorf("%w: message length", proposal.ErrSchemaViolation)
	}
	summary := args.Category + ": " + truncateRunes(message, 80)
	if len(args.Links) > 0 {
		summary += " " + strings.Join(args.Links, " ")
	}
	entry, err := commit.CommitWithActor(ctx, b.root, sess.Agent, "append_log", summary, nil)
	if err != nil {
		return LogAppendResult{}, err
	}
	return LogAppendResult{
		Seq:    entry.Seq,
		SHA:    entry.GitSHA,
		TS:     entry.Timestamp,
		Status: "appended",
	}, nil
}

// handleWikiInfo 实现 mcp-tools.md §2 wiki_info。
//
// 直接 SQL COUNT() 读 sources / pages 各类型计数。SQLite 数据 < 10k 量级，
// 全量 COUNT(*) 一次 < 10 ms，无需缓存。
func (b *vaultBackend) handleWikiInfo(ctx context.Context, _ WikiInfoArgs) (WikiInfoResult, error) {
	result := WikiInfoResult{
		VaultRoot:     b.root,
		SchemaVersion: fallbackSchemaVersion,
		DaemonVersion: daemonVersion,
		Health: HealthBlock{
			Score:        100,
			DriftClaims:  0,
			LintWarnings: 0,
		},
	}
	if cfg, err := vault.LoadConfig(b.root); err == nil && cfg.SchemaVersion != "" {
		result.SchemaVersion = cfg.SchemaVersion
	}

	counts, err := countVault(ctx, b.db)
	if err != nil {
		return WikiInfoResult{}, err
	}
	result.Counts = counts
	return result, nil
}

// countVault 跑 5 条 COUNT() 凑出 wiki_info 的 counts block。
//
// reviews 表在 W3 D10+ 才上线，pending_reviews 暂归 0；不查那张可能不存在
// 的表，避免 SQL 错误污染响应。
func countVault(ctx context.Context, db *index.DB) (CountsBlock, error) {
	out := CountsBlock{}
	sqlDB := db.SQL()
	if sqlDB == nil {
		return out, index.ErrIndexUnavailable
	}
	if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM sources`, &out.RawSources); err != nil {
		return out, fmt.Errorf("count sources: %w", err)
	}
	if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM pages`, &out.WikiPages); err != nil {
		return out, fmt.Errorf("count pages: %w", err)
	}
	for _, t := range []struct {
		typ string
		dst *int
	}{
		{"claim", &out.Claims},
		{"entity", &out.Entities},
		{"concept", &out.Concepts},
	} {
		if err := scanInt(ctx, sqlDB, `SELECT COUNT(*) FROM pages WHERE type = ?`, t.dst, t.typ); err != nil {
			return out, fmt.Errorf("count %s: %w", t.typ, err)
		}
	}
	return out, nil
}

func scanInt(ctx context.Context, db *sql.DB, q string, dst *int, args ...interface{}) error {
	return db.QueryRowContext(ctx, q, args...).Scan(dst)
}

// handleReadPage 实现 mcp-tools.md §3 read_page。
//
// page_id 路由：
//  1. 含 "/" 或 ".md" → 当 vault-relative path 处理，走 ParsePage（fs 读取）
//  2. 其它 → 当 page id，查 pages 表（GetPageByID）
//
// include_history / include_backlinks 返回空数组 + note，不报错——让 agent
// 看到 staged 行为而不是 NOT_IMPLEMENTED crash。
func (b *vaultBackend) handleReadPage(ctx context.Context, args ReadPageArgs) (ReadPageResult, error) {
	id := strings.TrimSpace(args.PageID)
	if id == "" {
		return ReadPageResult{}, fmt.Errorf("read_page: page_id is required")
	}

	var (
		result ReadPageResult
		err    error
	)
	if looksLikePath(id) {
		result, err = b.readPageFromPath(id)
	} else {
		result, err = b.readPageFromIndex(ctx, id)
	}
	if err != nil {
		return ReadPageResult{}, err
	}

	// staged: 始终返回空切片避免 JSON 输出 null（agent 友好）。
	result.History = []any{}
	result.Backlinks = []any{}
	if args.IncludeHistory {
		result.HistoryNote = historyNote
	}
	if args.IncludeBacklinks {
		result.BacklinksNote = backlinksNote
	}
	return result, nil
}

// looksLikePath 用启发式判断 page_id 是否其实是 vault-relative path：
// 含 "/" 或以 ".md" 结尾——这两种形态在 page id 命名规范里都不允许。
func looksLikePath(s string) bool {
	if strings.ContainsAny(s, "/\\") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(s), ".md")
}

// readPageFromIndex 直接走 SQLite pages 表，避免读 fs。
func (b *vaultBackend) readPageFromIndex(ctx context.Context, id string) (ReadPageResult, error) {
	row, err := index.GetPageByID(ctx, b.db, id)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: query index: %w", err)
	}
	if row == nil {
		return ReadPageResult{}, fmt.Errorf("%w: %s", ErrPageNotFound, id)
	}
	return pageRowToResult(row), nil
}

// readPageFromPath 通过 vault-relative path 走 fs：先 ResolveInVault 防 traversal,
// 再调 service.ParsePage 拿 frontmatter+body。
func (b *vaultBackend) readPageFromPath(rel string) (ReadPageResult, error) {
	abs, err := vault.ResolveInVault(rel, b.root)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: %w", err)
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return ReadPageResult{}, fmt.Errorf("%w: %s", ErrPageNotFound, rel)
		}
		return ReadPageResult{}, fmt.Errorf("read_page: stat: %w", statErr)
	}
	parsed, err := service.ParsePage(abs)
	if err != nil {
		return ReadPageResult{}, fmt.Errorf("read_page: parse: %w", err)
	}
	return parsedPageToResult(rel, parsed), nil
}

// pageRowToResult 把 SQLite pages 行映射为 MCP 响应。
func pageRowToResult(row *index.PageRow) ReadPageResult {
	res := ReadPageResult{
		ID:            row.ID,
		Type:          row.Type,
		Path:          row.Path,
		Title:         row.Title,
		Body:          row.Body,
		ContentHash:   proposal.PageContentHashFromJSON(row.Frontmatter, row.Body),
		Status:        row.Status,
		SchemaVersion: row.SchemaVersion,
		Frontmatter:   row.Frontmatter,
	}
	if row.Confidence.Valid {
		v := row.Confidence.Float64
		res.Confidence = &v
	}
	return res
}

// parsedPageToResult 把 fs 解析结果映射为 MCP 响应。
//
// 与 pageRowToResult 不同，path 形态没经 reindex，所以 confidence / status
// 等字段从 frontmatter 直接抓——尽量贴近 pageRow 的语义。
// Frontmatter 字段也 JSON-marshal 一份保持与 by-id 路径输出 shape 一致。
func parsedPageToResult(rel string, p *service.ParsedPage) ReadPageResult {
	res := ReadPageResult{
		Path: filepath.ToSlash(rel),
		Body: p.Body,
	}
	if p.Frontmatter != nil {
		if v, ok := p.Frontmatter["id"].(string); ok {
			res.ID = v
		}
		if v, ok := p.Frontmatter["type"].(string); ok {
			res.Type = v
		}
		if v, ok := p.Frontmatter["title"].(string); ok {
			res.Title = v
		}
		if v, ok := p.Frontmatter["status"].(string); ok {
			res.Status = v
		}
		if v, ok := p.Frontmatter["schema_version"].(string); ok {
			res.SchemaVersion = v
		}
		switch c := p.Frontmatter["confidence"].(type) {
		case float64:
			res.Confidence = &c
		case int:
			f := float64(c)
			res.Confidence = &f
		}
		if raw, err := json.Marshal(p.Frontmatter); err == nil {
			res.Frontmatter = string(raw)
		}
	}
	res.ContentHash = proposal.PageContentHash(p.Frontmatter, p.Body)
	if res.Title == "" {
		for _, h := range p.Headings {
			if h.Level == 1 && h.Text != "" {
				res.Title = h.Text
				break
			}
		}
	}
	return res
}

// handleReadRaw 实现 mcp-tools.md §4 read_raw。
//
// 严格限制：
//   - format=normalized → ErrFormatUnsupported（W2 D9 才上）
//   - raw_id 必须以 "raw/" 开头（normalize 后判断）—— 把 read_raw 锁在
//     raw/ 子树，wiki/ 用 read_page
//   - ResolveInVault 防 path traversal / symlink 逃逸
//   - 非 utf-8 文本（http.DetectContentType 嗅探 text/*）走 base64
func (b *vaultBackend) handleReadRaw(ctx context.Context, args ReadRawArgs) (ReadRawResult, error) {
	_ = ctx
	rawID := strings.TrimSpace(args.RawID)
	if rawID == "" {
		return ReadRawResult{}, fmt.Errorf("read_raw: raw_id is required")
	}
	format := strings.ToLower(strings.TrimSpace(args.Format))
	if format == "" {
		// spec 默认 normalized，但 D8 未实现；为不让空 input 直接报错，
		// 默认走 raw（content 字节）—— Decision §3 已记录。
		format = "raw"
	}
	if format == "normalized" {
		return ReadRawResult{}, fmt.Errorf("%w: %s", ErrFormatUnsupported, formatNormalizedNote)
	}
	if format != "raw" {
		return ReadRawResult{}, fmt.Errorf("read_raw: unknown format %q", args.Format)
	}

	posix := vault.NormalizePath(rawID)
	if !strings.HasPrefix(posix, "raw/") && posix != "raw" {
		return ReadRawResult{}, fmt.Errorf("%w: %s", ErrRawIDOutsideRaw, rawID)
	}

	abs, err := vault.ResolveInVault(posix, b.root)
	if err != nil {
		return ReadRawResult{}, fmt.Errorf("read_raw: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadRawResult{}, fmt.Errorf("%w: %s", ErrRawNotFound, rawID)
		}
		return ReadRawResult{}, fmt.Errorf("read_raw: stat: %w", err)
	}
	if info.IsDir() {
		return ReadRawResult{}, fmt.Errorf("read_raw: %s is a directory", rawID)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return ReadRawResult{}, fmt.Errorf("read_raw: read: %w", err)
	}

	res := ReadRawResult{
		RawID:  posix,
		Format: format,
		Bytes:  len(data),
	}
	if isTextual(data) {
		res.Content = string(data)
	} else {
		res.Content = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
	}
	return res, nil
}

// handleReadRawAnchor 实现 mcp-tools.md §5 read_raw_anchor。
func (b *vaultBackend) handleReadRawAnchor(ctx context.Context, args ReadRawAnchorArgs) (ReadRawAnchorResult, error) {
	rawID := strings.TrimSpace(args.RawID)
	if rawID == "" {
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: raw_id is required")
	}
	anchor := strings.TrimSpace(args.Anchor)
	if anchor == "" {
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: anchor is required")
	}

	posix, abs, err := b.resolveRawPath(rawID)
	if err != nil {
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: %w", err)
	}
	info, data, err := readExistingFile(abs, posix)
	if err != nil {
		if errors.Is(err, ErrRawNotFound) {
			return ReadRawAnchorResult{}, err
		}
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: %w", err)
	}
	content, span, err := index.ResolveAnchor(data, anchor)
	if err != nil {
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: %w", err)
	}

	sourceMTime := info.ModTime().UTC()
	sourceSHA := sha256Hex(data)
	if src, err := index.FindSourceByRawID(ctx, b.db, posix); err != nil {
		return ReadRawAnchorResult{}, fmt.Errorf("read_raw_anchor: query source: %w", err)
	} else if src != nil {
		if src.MTime > 0 {
			sourceMTime = time.Unix(src.MTime, 0).UTC()
		}
		if src.SHA256 != "" {
			sourceSHA = src.SHA256
		}
	}

	return ReadRawAnchorResult{
		RawID:        posix,
		Anchor:       anchor,
		Content:      content,
		QuoteHash:    index.QuoteHash(content),
		Span:         span,
		SourceMTime:  sourceMTime.Format(time.RFC3339),
		SourceSHA256: sourceSHA,
	}, nil
}

// isTextual 用 http.DetectContentType 嗅探前 512 字节；text/* 视为文本。
// 同时强校验 utf-8 合法性，防止把损坏的二进制当文本编返。
func isTextual(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	mime := http.DetectContentType(head)
	if !strings.HasPrefix(mime, "text/") && !strings.HasPrefix(mime, "application/json") &&
		!strings.HasPrefix(mime, "application/xml") {
		return false
	}
	return utf8.Valid(data)
}

// handleListIndex 实现 mcp-tools.md §7 list_index。
//
// 链路：ListPages（type filter，SQL 层）→ prefix filter（内存）→ slice
// limit/offset。pages 总数预计 < 10k，内存 filter + slice 完全够。
func (b *vaultBackend) handleListIndex(ctx context.Context, args ListIndexArgs) (ListIndexResult, error) {
	typeFilter := strings.TrimSpace(args.Type)
	if strings.EqualFold(typeFilter, "all") {
		typeFilter = ""
	}
	rows, err := index.ListPages(ctx, b.db, typeFilter)
	if err != nil {
		return ListIndexResult{}, fmt.Errorf("list_index: %w", err)
	}

	prefix := strings.TrimSpace(args.Prefix)
	if prefix != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if strings.HasPrefix(r.Path, prefix) {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	total := len(rows)

	limit := 100
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}
	offset := 0
	if args.Offset != nil && *args.Offset > 0 {
		offset = *args.Offset
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	items := make([]*IndexItem, 0, end-offset)
	for _, r := range rows[offset:end] {
		item := &IndexItem{
			ID:     r.ID,
			Type:   r.Type,
			Path:   r.Path,
			Title:  r.Title,
			Status: r.Status,
		}
		if r.Confidence.Valid {
			v := r.Confidence.Float64
			item.Confidence = &v
		}
		items = append(items, item)
	}
	return ListIndexResult{Total: total, Items: items}, nil
}

// handleSearch 实现 mcp-tools.md §8 search。
func (b *vaultBackend) handleSearch(ctx context.Context, args SearchArgs) (SearchResult, error) {
	start := time.Now()
	q := strings.TrimSpace(args.Query)
	if q == "" {
		return SearchResult{Results: []SearchResultItem{}, TokenizerUsed: "like"}, nil
	}
	searchType := strings.TrimSpace(args.Type)
	if searchType == "" {
		searchType = "fts"
	}
	var warnings []string
	if searchType == "fts+vector" {
		warnings = append(warnings, vectorSearchWarning)
		searchType = "fts"
	}
	if searchType != "fts" {
		return SearchResult{}, fmt.Errorf("search: unknown type %q", args.Type)
	}

	limit := 20
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}
	searchLimit := limit
	if args.Filter != nil {
		searchLimit = limit * 5
		if searchLimit < 50 {
			searchLimit = 50
		}
	}
	hits, err := service.Search(ctx, b.db, b.root, q, service.SearchOptions{Limit: searchLimit})
	if err != nil {
		return SearchResult{}, fmt.Errorf("search: %w", err)
	}

	updatedSince, err := parseUpdatedSince(args.Filter)
	if err != nil {
		return SearchResult{}, err
	}
	pageTypes := stringSet(nil)
	statuses := stringSet(nil)
	var notes []string
	if args.Filter != nil {
		pageTypes = stringSet(args.Filter.PageType)
		statuses = stringSet(args.Filter.Status)
		if args.Filter.MinConfidence != nil {
			notes = append(notes, minConfidenceNote)
		}
	}

	items := make([]SearchResultItem, 0, minInt(len(hits), limit))
	tokenizer := tokenizerForQuery(q)
	for _, hit := range hits {
		if hit.Source != "" {
			tokenizer = tokenizerFromSource(hit.Source)
		}
		if len(pageTypes) > 0 && !pageTypes[hit.Type] {
			continue
		}
		var row *index.PageRow
		if len(statuses) > 0 || !updatedSince.IsZero() {
			row, err = index.GetPageByID(ctx, b.db, hit.PageID)
			if err != nil {
				return SearchResult{}, fmt.Errorf("search: load page %s: %w", hit.PageID, err)
			}
			if row == nil {
				continue
			}
			if len(statuses) > 0 && !statuses[row.Status] {
				continue
			}
			if !updatedSince.IsZero() && time.Unix(row.UpdatedAt, 0).Before(updatedSince) {
				continue
			}
		}
		if row == nil {
			row, _ = index.GetPageByID(ctx, b.db, hit.PageID)
		}
		item := SearchResultItem{
			PageID:  hit.PageID,
			Title:   hit.Title,
			Snippet: hit.Snippet,
			Score:   hit.Score,
		}
		if row != nil && row.Confidence.Valid {
			v := row.Confidence.Float64
			item.Confidence = &v
		}
		items = append(items, item)
		if len(items) >= limit {
			break
		}
	}
	return SearchResult{
		Results:       items,
		TokenizerUsed: tokenizer,
		QueryTimeMS:   time.Since(start).Milliseconds(),
		Warnings:      warnings,
		Notes:         notes,
	}, nil
}

// handleReadClaim 实现 mcp-tools.md §6 read_claim。
func (b *vaultBackend) handleReadClaim(ctx context.Context, args ReadClaimArgs) (ReadClaimResult, error) {
	id := strings.TrimSpace(args.ClaimID)
	if id == "" {
		return ReadClaimResult{}, fmt.Errorf("read_claim: claim_id is required")
	}
	row, err := index.GetPageByID(ctx, b.db, id)
	if err != nil {
		return ReadClaimResult{}, fmt.Errorf("read_claim: query page: %w", err)
	}
	if row == nil || row.Type != "claim" {
		return ReadClaimResult{}, fmt.Errorf("%w: %s", ErrClaimNotFound, id)
	}
	page := pageRowToResult(row)
	return ReadClaimResult{
		ID:            page.ID,
		Type:          page.Type,
		Path:          page.Path,
		Title:         page.Title,
		Body:          page.Body,
		Confidence:    page.Confidence,
		Status:        page.Status,
		SchemaVersion: page.SchemaVersion,
		Frontmatter:   page.Frontmatter,
		Sources:       []ClaimSourceStatus{},
		SourcesNote:   claimSourcesStagedNote,
	}, nil
}

// handleGraphNeighbors 实现 mcp-tools.md §9 graph_neighbors。
func (b *vaultBackend) handleGraphNeighbors(ctx context.Context, args GraphNeighborsArgs) (GraphNeighborsResult, error) {
	pageID := strings.TrimSpace(args.PageID)
	if pageID == "" {
		return GraphNeighborsResult{}, fmt.Errorf("graph_neighbors: page_id is required")
	}
	depth := 1
	if args.Depth != nil && *args.Depth > 0 {
		depth = *args.Depth
	}
	if depth != 1 {
		return GraphNeighborsResult{}, fmt.Errorf("%w: depth=%d", ErrDepthUnsupported, depth)
	}
	direction := strings.TrimSpace(args.Direction)
	if direction == "" {
		direction = "both"
	}
	if direction != "out" && direction != "in" && direction != "both" {
		return GraphNeighborsResult{}, fmt.Errorf("graph_neighbors: unknown direction %q", args.Direction)
	}

	var notes []string
	var neighbors []GraphNeighbor
	if direction == "in" || direction == "both" {
		notes = append(notes, inboundLinksStagedNote)
	}
	if direction == "out" || direction == "both" {
		rel, err := b.resolvePagePath(ctx, pageID)
		if err != nil {
			return GraphNeighborsResult{}, fmt.Errorf("graph_neighbors: %w", err)
		}
		if linkFilter := stringSet(args.LinkTypes); len(linkFilter) > 0 && !linkFilter["ref"] {
			return GraphNeighborsResult{Neighbors: []GraphNeighbor{}, Notes: notes}, nil
		}
		abs, err := vault.ResolveInVault(rel, b.root)
		if err != nil {
			return GraphNeighborsResult{}, fmt.Errorf("graph_neighbors: %w", err)
		}
		parsed, err := service.ParsePage(abs)
		if err != nil {
			return GraphNeighborsResult{}, fmt.Errorf("graph_neighbors: parse page: %w", err)
		}
		for _, target := range parsed.Outbounds {
			neighbor := GraphNeighbor{PageID: target, LinkType: "ref"}
			if row, err := index.GetPageByID(ctx, b.db, target); err == nil && row != nil {
				neighbor.Title = row.Title
			}
			neighbors = append(neighbors, neighbor)
		}
	}
	if neighbors == nil {
		neighbors = []GraphNeighbor{}
	}
	return GraphNeighborsResult{Neighbors: neighbors, Notes: notes}, nil
}

// handleGetHistory 实现 mcp-tools.md §10 get_history。
func (b *vaultBackend) handleGetHistory(ctx context.Context, args GetHistoryArgs) (GetHistoryResult, error) {
	pageID := strings.TrimSpace(args.PageID)
	if pageID == "" {
		return GetHistoryResult{}, fmt.Errorf("get_history: page_id is required")
	}
	rel, err := b.resolvePagePath(ctx, pageID)
	if err != nil {
		return GetHistoryResult{}, fmt.Errorf("get_history: %w", err)
	}
	limit := 20
	if args.Limit != nil && *args.Limit > 0 {
		limit = *args.Limit
	}
	out, err := runVaultGit(ctx, b.root,
		"log", "--format=%H|%aI|%s", "-n", strconv.Itoa(limit), "--", filepath.FromSlash(rel))
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") {
			return GetHistoryResult{Commits: []HistoryCommit{}}, nil
		}
		return GetHistoryResult{}, fmt.Errorf("get_history: git log: %w", err)
	}
	var commits []HistoryCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		sha, ts, subject := parts[0], parts[1], parts[2]
		hc := HistoryCommit{
			SHA:         sha,
			TS:          ts,
			Actor:       "git",
			Op:          "git-direct",
			Summary:     subject,
			DiffSummary: b.diffSummary(ctx, sha, rel),
		}
		if seq, ok := seqFromSubject(subject); ok {
			if entry, err := commit.ReadEntryBySeq(b.root, seq); err == nil {
				hc.Actor = entry.Actor
				hc.Op = entry.Op
				hc.BundleID = entry.Bundle
				hc.Summary = entry.Summary
			}
		}
		commits = append(commits, hc)
	}
	if commits == nil {
		commits = []HistoryCommit{}
	}
	return GetHistoryResult{Commits: commits}, nil
}

func (b *vaultBackend) resolveRawPath(rawID string) (string, string, error) {
	posix := vault.NormalizePath(rawID)
	if !strings.HasPrefix(posix, "raw/") && posix != "raw" {
		return "", "", fmt.Errorf("%w: %s", ErrRawIDOutsideRaw, rawID)
	}
	abs, err := vault.ResolveInVault(posix, b.root)
	if err != nil {
		return "", "", err
	}
	return posix, abs, nil
}

func readExistingFile(abs, display string) (os.FileInfo, []byte, error) {
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("%w: %s", ErrRawNotFound, display)
		}
		return nil, nil, fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return nil, nil, fmt.Errorf("%s is a directory", display)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, fmt.Errorf("read: %w", err)
	}
	return info, data, nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func parseUpdatedSince(filter *SearchFilter) (time.Time, error) {
	if filter == nil || strings.TrimSpace(filter.UpdatedSince) == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(filter.UpdatedSince))
	if err != nil {
		return time.Time{}, fmt.Errorf("search: invalid updated_since: %w", err)
	}
	return t, nil
}

func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out[v] = true
		}
	}
	return out
}

func tokenizerForQuery(q string) string {
	if utf8.RuneCountInString(strings.TrimSpace(q)) < 3 {
		return "like"
	}
	return "trigram"
}

func tokenizerFromSource(source string) string {
	switch source {
	case index.SearchSourceFTS5:
		return "trigram"
	case index.SearchSourceLike:
		return "like"
	case index.SearchSourceRipgrep:
		return "ripgrep"
	default:
		return tokenizerForQuery(source)
	}
}

func (b *vaultBackend) resolvePagePath(ctx context.Context, pageID string) (string, error) {
	id := strings.TrimSpace(pageID)
	if id == "" {
		return "", ErrPageNotFound
	}
	if looksLikePath(id) {
		return vault.NormalizePath(id), nil
	}
	row, err := index.GetPageByID(ctx, b.db, id)
	if err != nil {
		return "", fmt.Errorf("query page: %w", err)
	}
	if row == nil {
		return "", fmt.Errorf("%w: %s", ErrPageNotFound, id)
	}
	return row.Path, nil
}

var seqSubjectRe = regexp.MustCompile(`\(seq=([0-9]+)\)`)

func seqFromSubject(subject string) (int, bool) {
	m := seqSubjectRe.FindStringSubmatch(subject)
	if m == nil {
		return 0, false
	}
	seq, err := strconv.Atoi(m[1])
	return seq, err == nil
}

func (b *vaultBackend) diffSummary(ctx context.Context, sha, rel string) string {
	out, err := runVaultGit(ctx, b.root, "show", "--numstat", "--format=", sha, "--", filepath.FromSlash(rel))
	if err != nil {
		return ""
	}
	added, deleted := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "-" || fields[1] == "-" {
			continue
		}
		a, errA := strconv.Atoi(fields[0])
		d, errD := strconv.Atoi(fields[1])
		if errA == nil {
			added += a
		}
		if errD == nil {
			deleted += d
		}
	}
	return fmt.Sprintf("+%d -%d", added, deleted)
}

func runVaultGit(ctx context.Context, vaultRoot string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = vaultRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), errors.New(msg)
	}
	return stdout.String(), nil
}

func agentAllowed(agent string, allowed []string) bool {
	if len(allowed) == 0 {
		allowed = vault.DefaultAllowedAgents()
	}
	for _, name := range allowed {
		if strings.TrimSpace(name) == agent {
			return true
		}
	}
	return false
}

func schemaMajorCompatible(agentVersion, daemonVersion string) bool {
	agentMajor := schemaMajor(agentVersion)
	daemonMajor := schemaMajor(daemonVersion)
	return agentMajor != "" && agentMajor == daemonMajor
}

func schemaMajor(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if major, _, ok := strings.Cut(version, "."); ok {
		return major
	}
	return version
}

func (b *vaultBackend) authenticateWrite(token string) (*Session, error) {
	sess, err := b.sessionStore().Authenticate(token)
	if err != nil {
		return nil, err
	}
	b.sessionStore().Touch(sess.Token)
	return sess, nil
}

func (b *vaultBackend) writePageInWorktree(ctx context.Context, sess *Session, rel string, fm map[string]any, body string) error {
	if strings.TrimSpace(sess.WorktreePath) == "" {
		return errors.New("SESSION_REQUIRED: missing worktree")
	}
	if err := worktreepkg.IsWorktreeWriteAllowed(rel); err != nil {
		return err
	}
	abs, err := vault.ResolveInVault(rel, sess.WorktreePath)
	if err != nil {
		return fmt.Errorf("worktree path: %w", err)
	}
	content, err := proposal.EncodePage(fm, body)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create worktree page dir: %w", err)
	}
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		return fmt.Errorf("write worktree page %s: %w", rel, err)
	}
	if err := proposal.StagePath(ctx, sess.WorktreePath, rel); err != nil {
		return err
	}
	return nil
}

func (b *vaultBackend) applyFrontmatterBodyEdit(ctx context.Context, sess *Session, rel string, changes map[string]any, body string) error {
	if strings.TrimSpace(sess.WorktreePath) == "" {
		return errors.New("SESSION_REQUIRED: missing worktree")
	}
	if err := worktreepkg.IsWorktreeWriteAllowed(rel); err != nil {
		return err
	}
	abs, err := vault.ResolveInVault(rel, sess.WorktreePath)
	if err != nil {
		return fmt.Errorf("worktree path: %w", err)
	}
	parsed, err := service.ParsePage(abs)
	if err != nil {
		return fmt.Errorf("parse worktree page: %w", err)
	}
	fm := map[string]any{}
	for k, v := range parsed.Frontmatter {
		fm[k] = v
	}
	for k, v := range changes {
		if v == nil {
			delete(fm, k)
			continue
		}
		fm[k] = v
	}
	if strings.TrimSpace(body) == "" {
		body = parsed.Body
	}
	content, err := proposal.EncodePage(fm, body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		return fmt.Errorf("write worktree page %s: %w", rel, err)
	}
	return proposal.StagePath(ctx, sess.WorktreePath, rel)
}

func (b *vaultBackend) insertPatchReview(
	ctx context.Context,
	sess *Session,
	op string,
	target string,
	patch []byte,
	meta map[string]any,
) (*index.ReviewRow, error) {
	seq, err := index.NextReviewSeq(ctx, b.db)
	if err != nil {
		return nil, err
	}
	reviewID := index.ReviewID(seq)
	patchPath, err := proposal.WritePatchFile(ctx, b.root, reviewID, patch)
	if err != nil {
		return nil, err
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal review meta: %w", err)
	}
	row := &index.ReviewRow{
		ID:           reviewID,
		Seq:          seq,
		Agent:        sess.Agent,
		SessionID:    sess.SessionID,
		Op:           op,
		TargetPageID: target,
		PatchPath:    patchPath,
		Status:       "pending",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		MetaJSON:     string(metaJSON),
	}
	if err := index.InsertReview(ctx, b.db, row); err != nil {
		return nil, err
	}
	return row, nil
}

func claimSourcesToProposal(in []ClaimSourceArg) []proposal.ClaimSource {
	out := make([]proposal.ClaimSource, 0, len(in))
	for _, src := range in {
		out = append(out, proposal.ClaimSource{
			RawID:     src.RawID,
			Anchor:    src.Anchor,
			Quote:     src.Quote,
			QuoteHash: src.QuoteHash,
			Span:      append([]int(nil), src.Span...),
		})
	}
	return out
}

func priorityScore(kind, hint string, reviewCount int) int {
	score := 100
	if kind == "lint_fix" {
		score = 50
	}
	switch hint {
	case "critical":
		score += 50
	case "low":
		score -= 30
	}
	return score + reviewCount
}

func validClaimStatus(status string) bool {
	switch status {
	case "unverified", "supported", "speculation":
		return true
	default:
		return false
	}
}

func validReviewKind(kind string) bool {
	switch kind {
	case "ingest", "lint_fix", "query_sediment", "dream_cycle", "custom":
		return true
	default:
		return false
	}
}

func validPriorityHint(hint string) bool {
	switch hint {
	case "critical", "normal", "low":
		return true
	default:
		return false
	}
}

func validLogCategory(category string) bool {
	switch category {
	case "agent_note", "dream_cycle_report", "lint_summary", "milestone":
		return true
	default:
		return false
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
