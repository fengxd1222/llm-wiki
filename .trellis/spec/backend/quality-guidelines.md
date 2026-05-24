# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)

<spec-entry category="quality" keywords="cobra-cli,vault-init,status-contract,go-embed,schema-templates" date="2026-05-22" source="cmd/wikimind/command.go:12;internal/vault/vault.go:46;spec-v2/templates/templates.go:9">

## Scenario: W1 D1 CLI init/status contract

### 1. Scope / Trigger
- Trigger: adding or changing public WikiMind CLI commands, vault bootstrap behavior, or embedded schema template wiring.
- Applies to `cmd/wikimind`, `internal/vault`, `internal/schema`, and the embedded default templates package.

### 2. Signatures
- `wikimind init <vault>` initializes a vault and prints `initialized: <abs-root>` plus `schema_version: <version>`.
- `wikimind status [vault]` accepts zero or one path; with no path, it starts from the current working directory and walks upward to find `.wikimind/config.toml`.
- D1 stubs are `wikimind ingest`, `query`, `review`, `lint`, and `revert`; they must run and print `wikimind <cmd>: D1 未实现`.

### 3. Contracts
- `init` accepts a missing path or an existing empty directory. Existing non-empty directories are rejected.
- `init` creates `raw/{inbox,imported,attachments,manifests}`, `wiki/{claims,entities,concepts,sources,topics,_review,_reports}`, `schema/`, and `.wikimind/{audit,locks}`.
- `init` writes `.wikimind/config.toml` with `vault_root`, `schema_version`, and `created_at`.
- `init` writes `wiki/index.md`, `wiki/log.md`, and all seven default schema templates.
- Default templates must be embedded from the `spec-v2/templates` tree. Because Go embed patterns cannot reach parent directories, the embed package lives next to those template files.
- `status` reports vault path, schema version, raw file count, wiki Markdown page count, claim Markdown count, git branch, git clean/dirty state, and health.

### 4. Validation & Error Matrix
- Empty `init` path -> return `vault path is required`.
- Existing file at target path -> return `vault path exists and is not a directory`.
- Existing non-empty target directory -> return `vault directory already exists and is not empty`.
- Missing vault marker for `status` -> return `no WikiMind vault found`.
- Malformed or missing `schema_version` in config -> return a config parse/read error.
- Git unavailable or non-repository during `status` -> keep status readable and mark git unavailable.

### 5. Good/Base/Bad Cases
- Good: `wikimind init /tmp/vault && wikimind status /tmp/vault` creates a git-backed vault and reports `schema_version: 1.0`.
- Base: `wikimind status` from `vault/wiki/topics` resolves the parent vault root.
- Bad: `wikimind init` into a directory containing any file refuses to modify that directory.

### 6. Tests Required
- Command tests for `init`, `status`, and every D1 stub command.
- Vault tests for directory structure, config file, initial wiki files, git repository presence, and non-empty directory rejection.
- Template tests comparing written schema files with `spec-v2/templates` source content.
- Public helper tests for template filename immutability and unknown template rejection.

### 7. Wrong vs Correct

#### Wrong
```go
//go:embed ../../spec-v2/templates/*.md
```

#### Correct
```go
// In spec-v2/templates/templates.go, next to the source template files.
//go:embed *.md
var files embed.FS
```

</spec-entry>

<spec-entry category="quality" keywords="mcp,agent-handshake,worktree,review-queue,bundles,sessions" date="2026-05-24" source="internal/mcp/tools.go:101;internal/worktree/worktree.go:40;internal/index/reviews.go:30;internal/index/bundles.go:27">

## Scenario: W2 D10 agent handshake + worktree + review base contract

### 1. Scope / Trigger
- Trigger: adding or changing multi-agent write preparation: `agent_handshake`, git worktree allocation, session store, or `reviews` / `bundles` persistence.
- Applies to `internal/mcp`, `internal/worktree`, `internal/index`, `internal/vault`, and `cmd/wikimind worktree`.

### 2. Signatures
- MCP: `agent_handshake(agent, version, session_id, capabilities, declares_schema_version) -> AgentHandshakeResult`.
- Worktree: `CreateWorktree(ctx, vaultRoot, agent, sessionID)`, `RemoveWorktree(ctx, vaultRoot, agent, sessionID)`, `ListWorktrees(ctx, vaultRoot)`.
- DB: `reviews(id, seq, bundle_id, agent, session_id, op, target_page_id, patch_path, status, created_at, decided_at, decided_by, meta_json)` and `bundles(id, seq, agent, session_id, summary, status, created_at, submitted_at, decided_at)`.
- CLI: `wikimind worktree list` and `wikimind worktree remove <agent>/<session-id>`.

### 3. Contracts
- `agent_handshake` is registered with `ReadOnlyHint=false`; the 9 read tools remain `ReadOnlyHint=true`.
- Allowed agents come from `.wikimind/config.toml allowed_agents`; empty or old configs fall back to `vault.DefaultAllowedAgents()`.
- Schema compatibility is major-version only: `1.0` and `1.1` are compatible; `2.0` returns `accepted=false`, `accepted_capabilities=["read"]`, and `can_propose=false`.
- A successful handshake creates `wiki/_worktrees/agent-<agent>-<session>/`, branch `wt-<agent>-<session>`, a `sk-` session token, fixed D10 rate limits, and queue state from `COUNT(*) WHERE reviews.status='pending'`.
- Worktree IDs must match `^[A-Za-z0-9_-]{1,64}$`; worktree cleanup must remove both the worktree and branch.
- `wiki/_worktrees/` must be gitignored in newly initialized vaults.

### 4. Validation & Error Matrix
- Agent not in whitelist -> `AGENT_NOT_WHITELISTED`.
- Duplicate `(agent, session_id)` -> `SESSION_EXISTS`.
- Repo has no commit -> worktree creation wraps `ErrEmptyRepo` as `WORKTREE_CREATE_FAILED`.
- Pending reviews `>= 50` -> handshake accepted but `queue_state.can_propose=false`.
- Unsafe agent/session path segment -> `ErrInvalidSessionID`.
- Worktree directory manually deleted but git metadata remains -> `RemoveWorktree` must run `git worktree prune` before branch deletion.

### 5. Good/Base/Bad Cases
- Good: committed vault + `codex-cli/sess-1` handshake returns a token and `wiki/_worktrees/agent-codex-cli-sess-1/`.
- Base: schema major mismatch returns a structured read-only result without creating a worktree.
- Bad: creating a worktree before session duplicate detection leaves orphan worktrees on repeated handshakes.
- Bad: deleting only the worktree directory can leave stale git metadata that blocks branch deletion.

### 6. Tests Required
- MCP server registration test asserts 10 tools, exactly `agent_handshake` is non-read-only.
- Handshake tests cover happy path, whitelist rejection, schema read-only result, duplicate session, worktree failure, and queue full.
- Worktree tests cover create/list/remove, duplicate create, unsafe IDs, empty repo, missing-directory cleanup with `worktree prune`, and write permission matrix.
- Index tests cover review/bundle seq, insert, lookup, status list/count, and status update.
- CLI tests cover `worktree list` empty/non-empty and `worktree remove`.

### 7. Wrong vs Correct

#### Wrong
```go
_ = os.RemoveAll(worktreePath)
_ = runGit(ctx, root, "branch", "-D", branch)
```

#### Correct
```go
if !pathExists(worktreePath) {
    _, _ = runGit(ctx, root, "worktree", "prune")
}
_ = RemoveWorktree(ctx, root, agent, sessionID)
```

</spec-entry>

<spec-entry category="quality" keywords="mcp,read-tools,anchor,quote-hash,search,graph,history" date="2026-05-24" source="internal/index/anchor.go:38;internal/index/anchor.go:76;internal/index/anchor.go:96;internal/mcp/server.go:79;internal/mcp/tools.go:359;internal/mcp/tools.go:488;internal/mcp/tools.go:591;internal/mcp/tools.go:620;internal/mcp/tools.go:676">

## Scenario: W2 D9 MCP read tools contract

### 1. Scope / Trigger
- Trigger: adding or changing MCP read tools, anchor parsing, quote hash generation, page graph reads, or git-backed page history.
- Applies to `internal/mcp`, `internal/index/anchor.go`, page/source index reads, and `cmd/wikimind mcp serve` tool advertising.

### 2. Signatures
- `index.ParseAnchor(s) (AnchorKind, string, error)` supports `#heading-slug`, `#para-N`, and `#char[start:end]`.
- `index.ResolveAnchor(content, anchor) (text, [2]int, error)` returns raw text plus `[startRune,endRune]` in the original file.
- `index.QuoteHash(text) string` returns `sha256(normalizedText)[:8]`.
- D9 registers `read_raw_anchor`, `read_claim`, `search`, `graph_neighbors`, and `get_history` in addition to the D8 tools. All registered tools must have `ReadOnlyHint=true`.

### 3. Contracts
- Heading anchors return the matched heading section up to the next same-or-higher-level heading. Slugs lowercase ASCII, keep CJK, convert whitespace/underscore/dash runs to `-`, and drop other punctuation.
- Paragraph anchors are 1-indexed and skip YAML frontmatter before blank-line paragraph splitting.
- Char spans use UTF-8 rune indexes, not byte indexes, to avoid splitting CJK characters.
- `read_raw_anchor` must enforce the same `raw/` path boundary as `read_raw`, compute quote hashes server-side, and prefer the `sources` table SHA/mtime when present.
- `search` uses the existing service search router; `fts+vector` downgrades to FTS with a warning; `min_confidence` is staged as a note.
- `read_claim.sources` and inbound `graph_neighbors` are staged empty arrays with explanatory notes until `claim_sources` / `page_links` exist.
- `get_history` resolves the actual page path, reads `git log -- <path>`, extracts `(seq=N)` from commit subjects, and joins change-log metadata when available. Non-seq commits use `op=git-direct`.

### 4. Validation & Error Matrix
- Malformed anchor -> `ErrAnchorMalformed`.
- Heading miss -> `ErrHeadingNotFound`.
- Paragraph out of range -> `ErrParaOutOfRange`.
- Invalid char span -> `ErrCharSpanInvalid`.
- `read_raw_anchor` outside `raw/` -> `ErrRawIDOutsideRaw`; missing raw file -> `ErrRawNotFound`.
- `read_claim` missing or non-claim page -> `ErrClaimNotFound`.
- `graph_neighbors depth > 1` -> `ErrDepthUnsupported`.
- Invalid `search.filter.updated_since` -> wrapped RFC3339 parse error.

### 5. Good/Base/Bad Cases
- Good: `read_raw_anchor(raw_id, "#char[2:4]")` over `ab中文cd` returns `中文`, span `[2,4]`, and an 8-char quote hash.
- Good: `graph_neighbors(direction="both")` returns outbound `[[...]]` refs plus a staged inbound note.
- Base: `read_claim` returns the normal page fields with `sources: []` and `sources_note`.
- Bad: calculating quote_hash in an agent instead of via `read_raw_anchor`; the daemon must be the authority.
- Bad: returning byte spans for CJK content; consumers need rune spans for cross-platform consistency.

### 6. Tests Required
- `internal/index/anchor_test.go` must cover 50+ anchor/slug/span/hash boundary cases.
- MCP server registration tests must assert all 9 read tools are registered and read-only.
- MCP handler tests must cover each D9 tool happy path plus staged/error behavior: filters, anchor misses, non-claim pages, inbound notes, depth rejection, seq history, and git-direct history.
- Project checks: `go test ./...`, `go build ./...`, `go vet ./...`; D9 requires at least 180 passing test/subtest events.

### 7. Wrong vs Correct

#### Wrong
```go
hash := localAgentHash(quote)
span := []int{byteStart, byteEnd}
```

#### Correct
```go
content, span, _ := index.ResolveAnchor(raw, "#char[2:4]")
hash := index.QuoteHash(content)
```

</spec-entry>

<spec-entry category="quality" keywords="git,change-log,append-only,revert,auto-commit" date="2026-05-24" source="internal/commit/commit.go:14;internal/commit/git.go:114;cmd/wikimind/command.go:423;internal/service/ingest.go:101">

## Scenario: W1 D6 Git-backed change log contract

### 1. Scope / Trigger
- Trigger: adding or changing user-initiated write operations that must create a WikiMind git commit and append audit logs.
- Applies to `internal/commit`, service-layer write entry points, and CLI commands that mutate vault content.

### 2. Signatures
- `commit.Commit(ctx, vaultRoot, op, summary, files) (*LogEntry, error)` is the service-layer write boundary.
- `wikimind ingest <path>` calls `commit.Commit(..., "ingest", rawID, []string{rawID})` after source copy and source-row insert.
- `wikimind revert <seq> [--no-confirm]` finds the seq commit, applies `GitRevertNoCommit`, then commits the reverse patch with `op=revert`.
- Commit messages must be `<op>: <summary> (seq=<N>)`; seq lookup uses the literal `(seq=<N>)` suffix.

### 3. Contracts
- `wiki/log.md` and `.wikimind/change-log.jsonl` are append-only audit files.
- `change-log.jsonl` stores `git_sha` as `""` in W1; runtime lookup uses `git log --grep "(seq=<N>)"`.
- Source/content changes and the new log rows must be staged into the same seq-tagged commit.
- `GitRevertNoCommit` must restore `wiki/log.md` and `.wikimind/change-log.jsonl` from `HEAD` after applying the reverse patch; otherwise reverting a seq deletes historical log rows.
- Use `git add -A -- <paths>` for normal paths, but skip paths already staged for deletion and absent from the worktree; git rejects those pathspecs even though the deletion is already staged.
- `GitCommit` and auto-commit revert paths must supply a default git identity so a fresh machine without global `user.name` / `user.email` can still run the demo.

### 4. Validation & Error Matrix
- Missing git binary -> return `ErrGitMissing` and surface a concise CLI error.
- Empty or non-positive revert seq -> `revert: seq must be a positive integer`.
- Missing change-log seq -> `revert: no change-log entry for seq=<N>`.
- Missing seq commit -> wrap `ErrSeqNotFound` from `FindCommitBySeq`.
- Revert conflict -> return the underlying `git revert --no-commit` error; W1 does not auto-resolve conflicts.
- Clean worktree commit with no staged changes -> return the git `nothing to commit` error.

### 5. Good/Base/Bad Cases
- Good: ingest creates `raw/inbox/file`, appends both logs, and commits all three with subject `ingest: raw/inbox/file (seq=1)`.
- Base: duplicate ingest returns the existing source row and does not allocate a new seq.
- Bad: reverting a seq with plain `git revert --no-edit` creates a reverse commit plus a separate log commit; `revert <new-seq>` then targets the log-only commit instead of content.
- Bad: reverting a commit that contains log files without restoring logs removes old audit rows and violates append-only.

### 6. Tests Required
- `internal/commit`: `NextSeq`, `AppendChangeLog`, `AppendLogMd`, `ReadEntryBySeq`, `EnsureRepo`, missing git, clean commit error, `GitRevert`, `GitRevertNoCommit`, `Commit`, and `FindCommitBySeq`.
- CLI E2E: `wikimind ingest <file>` creates logs and seq commit; duplicate ingest does not increment seq.
- CLI E2E: `wikimind revert 1 --no-confirm` removes the ingested content, appends `op=revert`, preserves seq 1, and `wikimind revert 2 --no-confirm` restores the content.

### 7. Wrong vs Correct

#### Wrong
```go
revertSHA, _ := commit.GitRevert(ctx, vaultRoot, origSHA)
logEntry, _ := commit.Commit(ctx, vaultRoot, "revert", summary, nil)
```

#### Correct
```go
paths, _ := commit.GitRevertNoCommit(ctx, vaultRoot, origSHA)
logEntry, _ := commit.Commit(ctx, vaultRoot, "revert", summary, paths)
```

</spec-entry>
