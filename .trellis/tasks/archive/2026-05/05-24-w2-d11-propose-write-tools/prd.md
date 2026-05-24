# W2 D11: 5 个写 tool (propose_page / propose_edit / propose_claim + request_review + log_append)

## Goal

打通 agent → daemon → review queue 的写入路径。Agent 在 worktree 改文件 →
propose_* 工具把改动转成 patch → 写入 reviews 表 + `wiki/_review/r-{seq}.patch`
等待 user accept（D12）。`log_append` 是唯一直写 wiki/log.md 的 tool（仅
append，不进 review）。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D11
- `spec-v2/docs/mcp-tools.md §11 §12 §13 §16 §19`
- `spec-v2/docs/agent-protocol.md §4.3`（worktree → main 合并路径）
- D10 已建 reviews/bundles 表 + worktree subsystem + session store

## What I already know

- D6 已有 `internal/commit/{change_log,git,commit}.go` —— log_append 复用
  commit.Commit pattern
- D8 9 个 read tool + D10 第 10 个 handshake = 当前 10 tool —— D11 加 5 = 15
- D10 已建：
  - `internal/index/{reviews,bundles}.go` CRUD + sentinel error
  - `internal/worktree/worktree.go` CreateWorktree / RemoveWorktree
  - `internal/worktree/permissions.go` IsWorktreeWriteAllowed
  - `internal/mcp/session.go` SessionStore (token / Lookup / Touch / Expire)
- D9 已有 `internal/index/anchor.go` QuoteHash + ResolveAnchor —— propose_claim
  source 校验复用
- D8/D9/D10 propose_* 留 staged note ("require propose_* W2 D11+")—— D11 真做

## Requirements

### A. Session middleware（全部写 tool 共用）

`internal/mcp/session.go` 加 helper：

```go
// AuthRequest checks session_token from request and returns active session.
// Returns ErrSessionRequired if token missing / Lookup fails / session expired.
func (s *SessionStore) Authenticate(token string) (*Session, error)
```

每个写 tool handler 第一步：
```go
sess, err := store.Authenticate(args.SessionToken)
if err != nil { return errToResult(err, "SESSION_REQUIRED"), ..., err }
store.Touch(sess.Token)
```

**注意**：D10 handshake 返 `session_token`，所有写 tool Request 必须含
`session_token` 字段（spec 没显式说但隐含——agent-protocol §3.1 写 "后续每次
MCP 请求附带"）。MCP SDK 没原生 session token 机制 → 通过 Request 字段传。

### B. 新包 `internal/proposal/`

#### B1. `patch.go`

```go
// GeneratePatch runs `git diff <main-ref>..<branch> -- <path>` from vault root
// and returns unified diff bytes. Empty diff → ErrNoChanges.
func GeneratePatch(ctx context.Context, vaultRoot, branch, path string) ([]byte, error)

// WritePatchFile writes patch to wiki/_review/<reviewID>.patch (idempotent: O_EXCL).
// reviewID is 'r-NNNN' format.
func WritePatchFile(ctx context.Context, vaultRoot, reviewID string, patch []byte) (relPath string, err error)

// 错误: ErrNoChanges / ErrPatchExists / ErrPatchWriteFailed
```

实施细节：
- main-ref 默认 `main`（可配置时再说）
- 写 worktree 后必须先 `git add` 才能 diff——daemon 应在 propose_* 调用前
  在 worktree 里 `git add -A`（或 propose handler 内做）
- patch 文件相对路径：`wiki/_review/r-NNNN.patch`
- 首次写 `wiki/_review/` 目录不存在 → MkdirAll

#### B2. `validator.go`

```go
type ValidationResult struct {
    SchemaCheck     string  // "passed" / "failed: <reason>"
    QuoteHashCheck  string  // "passed" / "failed: <reason>" / "skipped"
    PathCheck       string  // "passed" / "failed: <reason>"
    BaseHashCheck   string  // "passed" / "failed: <reason>" / "skipped"
    Errors          []string  // non-empty if any check failed
}

// ValidatePath checks `path` is under wiki/<allowed-type>/ for the given `type`.
//   wiki/claims/   ← claim
//   wiki/entities/ ← entity
//   wiki/concepts/ ← concept
//   wiki/sources/  ← source
//   wiki/topics/   ← topic
//   Anything else → ErrPathNotAllowed
func ValidatePath(path, pageType string) error

// ValidateFrontmatter checks required fields per page type (e.g. claim: confidence,
// title, status). For D11 MVP only check the bare minimum (title required,
// type matches). Full schema enforcement → W3 D17 lint.
func ValidateFrontmatter(fm map[string]any, pageType string) error

// ValidateBaseHash hashes current page content from main branch and compares
// with declared base_hash. ErrBaseHashMismatch if differs.
func ValidateBaseHash(ctx context.Context, vaultRoot, pageID, declaredBaseHash string) error

// ValidateClaimSources iterates sources, re-resolves each anchor in raw,
// recomputes quote_hash, compares with declared. Returns first mismatch.
//   Also asserts raw_id starts with "raw/" (provenance depth = 1).
func ValidateClaimSources(ctx context.Context, vaultRoot string, sources []ClaimSource) error

// PageContentHash computes sha256 of canonical page form (frontmatter YAML +
// "\n---\n" + body, normalized whitespace). Returns first 16 hex chars.
func PageContentHash(frontmatter map[string]any, body string) string
```

错误码: `ErrPathNotAllowed / ErrSchemaViolation / ErrBaseHashMismatch /
ErrQuoteHashMismatch / ErrProvenanceDepthExceeded`

### C. 5 个 handler

注册时全部 `ReadOnlyHint: false`（写 tool）。

#### C1. `handleProposePage` (§11)

```
1. Authenticate session
2. ValidatePath(args.Path, args.Type)
3. ValidateFrontmatter(args.Frontmatter, args.Type)
4. 找 session 的 worktree (sess.WorktreePath)
5. 在 worktree 内写文件 (frontmatter YAML + body) → <worktree>/<path>
6. `git -C <worktree> add <path>`
7. proposal.GeneratePatch(vaultRoot, sess.Branch, path)
   - 没改动 (page 已存在且内容相同) → ErrNoChanges
8. reviewID = "r-NNNN" (proposal.NextReviewSeq, formatted)
9. proposal.WritePatchFile(vaultRoot, reviewID, patch)
10. idempotency_key 校验：查 reviews WHERE meta_json LIKE %"idempotency_key":"<key>"%
    AND agent=<sess.Agent>。如已存在 → 返回原有 reviewID + status (idempotent)
11. index.InsertReview(ReviewRow{ID, Seq, Agent: sess.Agent, SessionID: sess.SessionID,
    Op: "propose_page", TargetPageID: <path>, PatchPath: relPath, Status: "pending",
    CreatedAt: now, MetaJSON: {idempotency_key, path, type}})
12. Response: {review_id, status: "pending", validations: {schema_check, path_check}}
```

#### C2. `handleProposeEdit` (§12)

```
1. Authenticate session
2. patch 字段支持 oneOf：
   - unified_diff 模式：args.Patch.UnifiedDiff 直接用
   - frontmatter_changes + body 模式：worktree 内重写文件 → diff
3. base_hash 校验：proposal.ValidateBaseHash(vaultRoot, pageID, baseHash)
   - 不匹配 → ErrBaseHashMismatch
4. （unified_diff 模式）在 worktree 内 `git apply patch`；失败 → ErrPatchApplyFailed
5. proposal.GeneratePatch → 取 final patch (handle frontmatter_changes 二级)
6. reviewID 分配 + WritePatchFile + InsertReview (Op: "propose_edit", TargetPageID: pageID)
7. Response 同 propose_page
```

#### C3. `handleProposeClaim` (§13)

```
1. Authenticate session
2. claim_id pattern 校验: ^cl-\d{4}-\d{2}-\d{2}-\d{3}$
3. ValidatePath("wiki/claims/<claim_id>.md", "claim")
4. proposal.ValidateClaimSources(vaultRoot, args.Sources):
   - 遍历每个 source: ResolveAnchor + QuoteHash 重算
   - 不匹配 quote_hash → ErrQuoteHashMismatch (附 stored vs current)
   - raw_id 不以 "raw/" 开头 → ErrProvenanceDepthExceeded
5. min sources >= 1 (除非 speculation=true)
6. confidence in [0, 1]
7. 组装 frontmatter (id/type=claim/title/confidence/status/sources/related)
8. 在 worktree 写 wiki/claims/<claim_id>.md
9. patch + WritePatchFile + InsertReview (Op: "propose_claim")
10. Response: {review_id, status, validations: {schema_check, quote_hash_check, path_check}}
```

#### C4. `handleRequestReview` (§16)

```
1. Authenticate session
2. 遍历 args.ReviewIDs：
   - index.GetReviewByID 找到
   - 检查 review.Agent + SessionID 匹配 sess (防跨 session 打包别人的 review)
   - 检查 review.Status == "pending" 且 review.BundleID 为空 (未已被打包)
3. NextBundleSeq → bundleID "b-NNNN"
4. InsertBundle(BundleRow{ID, Seq, Agent: sess.Agent, SessionID: sess.SessionID,
   Summary: args.Title, Status: "submitted", CreatedAt: now, SubmittedAt: now})
5. UPDATE reviews SET bundle_id=<bundleID> WHERE id IN (...)
6. priority_score 计算 (简化 MVP):
   - kind=lint_fix → 50 / ingest → 100 / 其他 100
   - priority_hint=critical → +50 / low → -30
   - 加上 reviews 数量 (越多 bundle 越优先打包)
7. queue_position: 当前已 submitted 但未 decided 的 bundle 数 + 1
8. Response: {bundle_id, review_ids, priority_score, queue_position}
```

#### C5. `handleLogAppend` (§19)

```
1. Authenticate session
2. category enum 校验
3. message maxLength 500 校验
4. 拼 log 行：
   - timestamp RFC3339 UTC
   - actor = sess.Agent
   - line format: "| <seq> | <ts> | <actor> (<category>) | append_log | <message> [<links>] |"
5. 调 commit.Commit(ctx, vaultRoot, "append_log",
     fmt.Sprintf("<category>: <message-first-40-chars>"),
     []string{}) // no source files; only log.md changes
   - 注意 commit.Commit 当前会写 wiki/log.md + .wikimind/change-log.jsonl，
     这正是我们要的。category 信息嵌入 commit summary
6. log_append 不进 reviews 表 (直接 commit)
7. Response: {seq, sha (空"" — D6 决定), ts, status: "appended"}
```

### D. Server 注册 + types

`internal/mcp/server.go` 注册 5 tool，**全 ReadOnlyHint=false**（写 tool）。
总 tool 数：10 (D10) + 5 (D11) = **15**。

`internal/mcp/types.go` 加 5 套 Request/Response struct。每个 Request 含
`SessionToken string \`json:"session_token"\`` 字段（强制）。

### E. wiki/_review/ 目录

- `wikimind init` 时（D1）当前没有建 _review 子目录 —— D11 改 init 模板加这个
- 或 propose 首次时 MkdirAll —— **推荐**（不动 init 减小 D11 范围）

### F. CLI

D11 不加新 CLI 命令（review accept/reject/diff 留 D12）。

### G. 测试

- `internal/proposal/patch_test.go`：GeneratePatch happy / no changes / WritePatchFile 幂等
- `internal/proposal/validator_test.go`：
  - ValidatePath 5 type × 2 (allowed / not allowed) + 边界（path traversal）
  - ValidateFrontmatter (claim/entity/concept/source/topic) 必填字段
  - ValidateBaseHash 匹配 / 不匹配 / page not found
  - ValidateClaimSources 全 verified / 单 source mismatch / provenance violation
  - PageContentHash 一致性 (相同 input 同 hash / 序列化稳定)
- `internal/mcp/tools_test.go` 加 5 handler 测试：
  - propose_page: session valid / session invalid / path not allowed / schema violation / happy
  - propose_edit: base_hash mismatch / unified_diff happy / frontmatter+body happy
  - propose_claim: source quote_hash mismatch / provenance > 1 / happy with verified
  - request_review: cross-session 打包 / 已 bundled review 拒 / happy
  - log_append: invalid category / maxLength / happy + 验 log.md 真的多了行
- `internal/mcp/session_test.go` 加 `Authenticate` 测试

目标测试总数：166 → ≥210（+40：proposal 18 + handlers 15 + session 3 + 其他 5）

### H. 跨平台

- patch unified diff 行尾：unified diff 标准 `\n`（不要 CRLF），exec git
  会自动处理
- `wiki/_review/r-NNNN.patch` 路径用 `filepath.Join`
- worktree 路径已用 D10 `worktree.Worktree.Path`（绝对路径）

## Acceptance Criteria

- [ ] 5 个写 tool 注册 + ReadOnlyHint=false (15 tool 总数)
- [ ] 所有写 tool 强制 session_token 校验（SESSION_REQUIRED）
- [ ] propose_page schema/path 双校验，patch 写入 wiki/_review/，reviews 表 insert
- [ ] propose_edit base_hash mismatch 返 BASE_HASH_MISMATCH
- [ ] propose_claim quote_hash 重算 + provenance_depth=1 强制
- [ ] propose_* idempotency_key 防重复 (同 key 返原 reviewID)
- [ ] request_review 跨 session 打包拒绝，已 bundled review 拒绝重复打包
- [ ] log_append 直接 commit (走 commit.Commit) 不进 review queue
- [ ] wiki/_review/ 目录首次 propose 自动 MkdirAll
- [ ] 单测：≥ 40 个新测试
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-H 全 done
- CI 5 OS 全绿
- 测试 ≥ 210
- commit + push

## Out of Scope

- propose_delete / propose_merge (W3+)
- acquire_lock / release_lock (W3 D15)
- lint_run (W3 D17)
- review accept/reject CLI (D12)
- patch apply 到 main branch (D12)
- agent worktree 自动同步 main 更新 (W3 daemon 主循环)
- rate_limits 真实 enforce (W3 D18)
- queue_state 计算精度 (D11 实时 COUNT, 性能优化留 D18)
- frontmatter 完整 schema 校验 (D11 仅最小检查; full schema W3 D17 lint)
- bundle 状态机完善 (D11 简单 open → submitted; W3 D15 加 conflict / superseded)

## Decision (ADR-lite)

**Context**: 写 tool 涉及 worktree 物理写 + 多重校验 + idempotency + 跨
tool 共用 session middleware。复杂度比 D10 高。

**Decision**:
1. **session_token 通过 Request 字段传**：MCP SDK 没原生 session 机制，
   每个写 tool Request struct 加 `session_token` 字段（required）。比
   middleware 注入 ctx 简洁
2. **idempotency 查 meta_json LIKE**：D11 在 reviews 表 meta_json 里存
   idempotency_key；查询用 LIKE %"idempotency_key":"<key>"%。性能可接受
   （reviews 表小）；W3 D17 lint 后加专用 idempotency_key 列 + index
3. **patch 生成走 git diff（不直接构造 unified diff）**：worktree 模型让
   git 自然算 diff，不重新发明轮子。propose_edit 的 unified_diff 模式则
   是 `git apply patch` 到 worktree 后再 diff
4. **PageContentHash 16 hex**：canonical = YAML frontmatter + "\n---\n" +
   body，去末尾换行规范化。sha256 截前 16 hex (8 bytes 防碰撞够 reviews 规模)
5. **log_append 复用 commit.Commit**：不另写 path，符合 D6 "ingest 自动
   commit" 同一逻辑 — log_append 是另一种 commit op
6. **provenance_depth = 1 强制**：D11 严格要求 source.raw_id 必须 raw/ 开头。
   wiki/ 间接引用留 W4 propose_synthesis 类（spec 未来 tool）
7. **D11 不做的写 tool**：propose_delete + propose_merge 留 W3—需要 backlinks
   分析（page_links 表 W2 D11+ 才在 schema，D11 不一起做避免范围爆炸）

**Consequences**:
- 优点：D11 5 tool 覆盖 80% 写场景（page/edit/claim 三个最常用 + bundle +
  log），agent 经 MCP 可以完整 "在 worktree 改 → propose → 等 user accept"
- 缺点：propose_edit 的 patch oneOf 两种模式实现复杂；base_hash 计算与
  agent read_page 时的 hash 必须一致——D8 read_page 当前**没**返 base_hash
  字段，D11 要补回 read_page response 加 content_hash 字段
- D12 review accept 流程基于 D11 reviews/bundles 表 + patch 文件直接 apply

## Technical Notes

### D8 read_page 需小改

D8 read_page Response 现没 `content_hash` 字段；D11 propose_edit 的
base_hash 依赖 read_page 给出。改动：
- `internal/mcp/types.go` ReadPageResult 加 `ContentHash string`
- `internal/mcp/tools.go` handleReadPage 末尾：
  `res.ContentHash = proposal.PageContentHash(p.Frontmatter, p.Body)`

### D6 commit.Commit 复用

log_append 调用现有 `commit.Commit(ctx, vaultRoot, "append_log", summary, nil)`：
- D6 commit.Commit 应 handle empty files slice — 仅 add log.md + change-log.jsonl
- 看 D6 实现是否支持 nil/empty files；不支持则小改

### 错误码映射

```go
var (
    ErrSessionRequired          = errors.New("session required")        // SESSION_REQUIRED
    ErrSchemaViolation          = errors.New("schema violation")        // SCHEMA_VIOLATION
    ErrPathNotAllowed           = errors.New("path not allowed")        // PATH_NOT_ALLOWED
    ErrBaseHashMismatch         = errors.New("base hash mismatch")      // BASE_HASH_MISMATCH
    ErrQuoteHashMismatch        = errors.New("quote hash mismatch")     // QUOTE_HASH_MISMATCH
    ErrProvenanceDepthExceeded  = errors.New("provenance depth > 1")    // PROVENANCE_DEPTH_EXCEEDED
    ErrPatchApplyFailed         = errors.New("patch apply failed")      // PATCH_APPLY_FAILED
    ErrNoChanges                = errors.New("no changes detected")     // NO_CHANGES
    ErrReviewAlreadyBundled     = errors.New("review already bundled")  // REVIEW_ALREADY_BUNDLED
    ErrCrossSessionBundle       = errors.New("cross-session bundle")    // CROSS_SESSION_BUNDLE
)
```

CallToolResult.IsError=true 时 content text 含 `code: <ERROR_CODE>` JSON 让 agent parse。

## 实施建议顺序

1. **D8 read_page 加 ContentHash 字段**（小改）
2. **internal/proposal/ 新包**（patch + validator，独立可测）
3. **session.Authenticate helper**（小改）
4. **handleLogAppend**（最简单——复用 commit.Commit）
5. **handleProposePage**（中等——worktree 写 + validate + reviews insert）
6. **handleProposeEdit**（最复杂——base_hash + oneOf patch 模式）
7. **handleProposeClaim**（中等——复用 ValidatePath + ValidateClaimSources）
8. **handleRequestReview**（中等——bundles 表 + 状态机简单）
9. **server 注册 5 tool**
10. **types.go 5 套 struct**
11. **测试 + ≥ 210 验证**
