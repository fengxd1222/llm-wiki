# W2 D10: agent_handshake + worktree per agent + reviews/bundles 表

## Goal

打通 multi-agent 协作的"门面三件套"：
1. `agent_handshake` MCP tool 让 agent 注册 session + 拿 worktree + 协商 schema 版本
2. Git worktree per agent 物理隔离（`wiki/_worktrees/agent-<agent>-<sess>/`）
3. SQLite `reviews` + `bundles` 表（D11 propose_* / D12 review accept 的数据基座）

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D10
- `spec-v2/docs/agent-protocol.md §3 §4`（handshake 协议 + worktree 物理结构）
- `spec-v2/docs/mcp-tools.md §1`（agent_handshake schema）
- `spec-v2/docs/architecture.md §2.4`（worktree 隔离设计）

## What I already know

- D6 已有 `internal/commit/git.go`：`exec.Command("git", ...)` + EnsureRepo + GitAdd + GitCommit + GitRevert + GitStatus —— D10 git worktree 子命令复用 exec pattern
- D8 已建 `internal/mcp/` 框架 + 9 个 tool 注册 + `wrapHandler` 泛型 adapter —— D10 第 10 个 tool 直接接入
- D2 `internal/vault/config.go` 有 `Config` struct —— D10 加 `AllowedAgents []string` 字段
- D3/D4 migration 在 `internal/index/migrations/`（0001 sources, 0002 pages_schema）—— D10 新建 0003
- engineering-decisions §4.4 已选 exec git（非 go-git）——`git worktree` 同样走 exec
- agent-protocol §3.2 失败码：SCHEMA_INCOMPATIBLE / AGENT_NOT_WHITELISTED / SESSION_EXISTS / QUEUE_FULL
- agent-protocol §4.2 worktree 内权限矩阵：schema/ 写禁 / raw/ 写禁 / wiki/ 可写 / push 禁

## Requirements

### A. SQLite migration `0003_reviews_bundles.sql`

新建 `internal/index/migrations/0003_reviews_bundles.sql`，含 2 表 + 索引：

```sql
-- +goose Up

CREATE TABLE IF NOT EXISTS reviews (
  id              TEXT PRIMARY KEY,          -- 'r-0001', 'r-0002' ...
  seq             INTEGER NOT NULL UNIQUE,   -- 1, 2, ...
  bundle_id       TEXT,                       -- FK → bundles.id (nullable until bundled)
  agent           TEXT NOT NULL,              -- 'claude-code', 'codex-cli', ...
  session_id      TEXT NOT NULL,              -- handshake session_id
  op              TEXT NOT NULL,              -- 'propose_page' / 'propose_edit' / 'propose_claim' / ...
  target_page_id  TEXT,                       -- nullable for create ops
  patch_path      TEXT NOT NULL,              -- 'wiki/_review/r-0001.patch' relative to vault root
  status          TEXT NOT NULL DEFAULT 'pending',
                                              -- pending / accepted / rejected / superseded / conflict
  created_at      TEXT NOT NULL,              -- RFC3339 UTC
  decided_at      TEXT,                       -- RFC3339 UTC，accept/reject 时填
  decided_by      TEXT,                       -- 'user' / agent name when auto-accept
  meta_json       TEXT NOT NULL DEFAULT '{}'  -- JSON object 装 quote_hash / provenance_depth / etc.
);

CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews(status);
CREATE INDEX IF NOT EXISTS idx_reviews_bundle ON reviews(bundle_id);
CREATE INDEX IF NOT EXISTS idx_reviews_agent_session ON reviews(agent, session_id);

CREATE TABLE IF NOT EXISTS bundles (
  id              TEXT PRIMARY KEY,           -- 'b-0001'
  seq             INTEGER NOT NULL UNIQUE,
  agent           TEXT NOT NULL,
  session_id      TEXT NOT NULL,
  summary         TEXT NOT NULL DEFAULT '',   -- agent 提供的 bundle 描述
  status          TEXT NOT NULL DEFAULT 'open',
                                              -- open / submitted / accepted / rejected
  created_at      TEXT NOT NULL,
  submitted_at    TEXT,
  decided_at      TEXT
);

CREATE INDEX IF NOT EXISTS idx_bundles_status ON bundles(status);
CREATE INDEX IF NOT EXISTS idx_bundles_agent_session ON bundles(agent, session_id);

-- +goose Down

DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS bundles;
```

### B. `internal/index/reviews.go`（新）+ `bundles.go`（新）

- `ReviewRow / BundleRow` struct
- `NextReviewSeq(ctx, db) (int, error)` / `NextBundleSeq(ctx, db) (int, error)`
- `InsertReview(ctx, db, ReviewRow) error`
- `InsertBundle(ctx, db, BundleRow) error`
- `ListReviewsByStatus(ctx, db, status string) ([]ReviewRow, error)`
- `GetReviewByID(ctx, db, id string) (ReviewRow, error)`
- `UpdateReviewStatus(ctx, db, id, status, decidedBy string) error`
- 错误：`ErrReviewNotFound / ErrBundleNotFound`
- D10 只搭表 + 基础 CRUD；propose 流程 / accept 流程留 D11/D12

### C. `internal/worktree/` 新包

3 文件：

#### C1. `worktree.go`

```go
type Worktree struct {
    Path       string // absolute path to worktree dir
    Branch     string // 'wt-<agent>-<sess_id>'
    Agent      string
    SessionID  string
    CreatedAt  time.Time
}

// CreateWorktree adds a git worktree at wiki/_worktrees/agent-<agent>-<sess>/
// and creates a branch wt-<agent>-<sess> based on current HEAD.
func CreateWorktree(ctx context.Context, vaultRoot, agent, sessionID string) (*Worktree, error)

// RemoveWorktree force-removes worktree dir + deletes branch.
// Safe to call on missing worktree (idempotent).
func RemoveWorktree(ctx context.Context, vaultRoot, agent, sessionID string) error

// ListWorktrees parses `git worktree list --porcelain` for wt-* branches.
func ListWorktrees(ctx context.Context, vaultRoot string) ([]Worktree, error)

// 错误：ErrWorktreeExists / ErrWorktreeNotFound / ErrInvalidSessionID
```

实施细节：
- worktree 路径强制规范化：`agent` / `sessionID` 必须 `[a-zA-Z0-9_-]+`（防 path injection）
- exec：`cd vaultRoot && git worktree add wiki/_worktrees/agent-<agent>-<sess>/ -b wt-<agent>-<sess>`
- 删除：`git worktree remove --force wiki/_worktrees/agent-<agent>-<sess>/ && git branch -D wt-<agent>-<sess>`
- 首次建 worktree 需 vault 至少有 1 commit（D6 之后已保证：wikimind init 应该有 initial commit；如果 vault 还没任何 commit → 报 `ErrEmptyRepo`）

#### C2. `permissions.go`

```go
// IsWorktreeWriteAllowed checks whether `relPath` (relative to worktree root)
// is allowed to be written. Implements agent-protocol §4.2 matrix:
//   wiki/* → allowed
//   raw/*  → denied (ErrRawWriteForbidden)
//   schema/* → denied (ErrSchemaWriteForbidden)
//   _worktrees/* → denied
//   anything else → denied (out of scope)
func IsWorktreeWriteAllowed(relPath string) error
```

D10 提供 helper，D11 propose_* 在写 patch 前调用强制。D10 阶段无 wired caller，但要测覆盖矩阵。

#### C3. `worktree_test.go`

- CreateWorktree happy path（建 + 验文件存在 + 验 branch 存在）
- CreateWorktree 同 agent+session 二次调 → ErrWorktreeExists
- RemoveWorktree 幂等（已删 + 不存在都 OK）
- RemoveWorktree 后路径不存在 + branch 不存在
- 非法 agent/sessionID（含 `/` 或 `..`）→ ErrInvalidSessionID
- IsWorktreeWriteAllowed 5 路径矩阵全覆盖
- 空 vault repo → CreateWorktree 报 ErrEmptyRepo

### D. Session 管理 `internal/mcp/session.go`（新）

D10 阶段 in-memory map（持久化留 W3）：

```go
type Session struct {
    Token            string         // sk-<32 hex chars>
    Agent            string
    Version          string
    SessionID        string         // client-provided
    Capabilities     []string
    SchemaVersion    string
    WorktreePath     string         // absolute path
    CreatedAt        time.Time
    LastSeenAt       time.Time
    IdleTimeout      time.Duration  // default 60min
}

type SessionStore struct {
    mu       sync.RWMutex
    byToken  map[string]*Session
    byKey    map[string]*Session  // key = agent + "/" + sessionID
}

func NewSessionStore() *SessionStore
func (s *SessionStore) Register(sess *Session) error  // ErrSessionExists if (agent, sessionID) 存在
func (s *SessionStore) Lookup(token string) (*Session, bool)
func (s *SessionStore) Touch(token string)            // 刷 LastSeenAt
func (s *SessionStore) Expire(now time.Time) []*Session  // 返回 expired sessions（caller 负责清 worktree）
```

D10 阶段 token 校验 only `agent_handshake` 用；后续 D11+ propose_* 才强制查 token。

### E. `agent_handshake` MCP tool

`internal/mcp/tools.go` 追加 handler；`server.go` 注册（10 个 tool 总数；
**ReadOnlyHint=false**，spec §0 标记 meta 类）。

#### E1. Request / Response types

按 `spec-v2/docs/mcp-tools.md §1`：
- Request: `agent / version / session_id / capabilities[] / declares_schema_version`
- Response: `accepted / daemon_schema_version / worktree / instructions / session_token / rate_limits / queue_state`

#### E2. handler 逻辑

```go
func handleAgentHandshake(ctx, req, args) (*sdk.CallToolResult, AgentHandshakeResult, error) {
    // 1. 校验 agent 在 config.AllowedAgents 白名单
    //    → ErrAgentNotWhitelisted (code: AGENT_NOT_WHITELISTED)
    // 2. 校验 schema_version 主版本匹配 (declares "1.0" vs daemon "1.0")
    //    → 不兼容 ErrSchemaIncompatible (code: SCHEMA_INCOMPATIBLE)，但仍返回 accepted=false + accepted_capabilities=["read"]
    //      (允许 read 工具继续工作，写工具拒绝——D11 wired)
    // 3. SessionStore.Register
    //    → ErrSessionExists (code: SESSION_EXISTS)，建议 agent 重生 UUID 重试
    // 4. worktree.CreateWorktree(vaultRoot, agent, sessionID)
    //    → 失败传播 (raw exec error wrapped)
    // 5. 生成 session_token (`sk-` + crypto/rand 32 hex)
    // 6. 查 reviews 表 pending count → queue_state.pending
    //    queue_state.hard_limit = 50 (hardcode；W3 D18 配置化)
    //    queue_state.can_propose = (pending < hard_limit && schema 兼容)
    // 7. instructions 数组：固定返 ["schema/AGENTS.md", "schema/CLAUDE.md"]
    //    (D10 不验证文件存在；W3 wikimind init 时建)
    // 8. rate_limits 固定：propose_per_minute=30 / query_per_minute=60
    //    (D10 hardcode 占位；W3 D18 配置化 + enforce)
    // 9. Response 组装返回
}
```

#### E3. config 加 `allowed_agents` 字段

`internal/vault/config.go` `Config` struct 加：
```go
AllowedAgents []string `toml:"allowed_agents"`
```

`.wikimind/config.toml` 默认 init 时写：
```toml
allowed_agents = ["claude-code", "codex-cli", "cursor", "cline", "opencode"]
```

`wikimind init` 默认 config 模板（D1）要更新——加 allowed_agents 字段。

### F. CLI 集成

新命令 `wikimind worktree`（user-facing 调试用）：
- `wikimind worktree list`：调 `worktree.ListWorktrees`，table 输出
- `wikimind worktree remove <session-key>`：force clean (debug 用)
  - `session-key` = `<agent>/<session-id>`

不动 `wikimind mcp serve`——agent_handshake 通过 MCP 走，CLI 不直接调用。

### G. 测试

- `internal/index/reviews_test.go`：NextReviewSeq / Insert / List / Get / UpdateStatus
- `internal/index/bundles_test.go`：NextBundleSeq / Insert / Get
- `internal/worktree/worktree_test.go`：见 C3
- `internal/mcp/session_test.go`：Register / Lookup / Touch / Expire / 并发 race-safe
- `internal/mcp/tools_test.go`：agent_handshake 6 路径：
  - happy path：return accepted=true + worktree 存在 + token 非空
  - AllowedAgents 不在白名单 → AGENT_NOT_WHITELISTED
  - schema 不兼容 → accepted=false + capabilities=["read"]
  - 同 (agent, sessionID) 二次 → SESSION_EXISTS
  - worktree create 失败 (e.g. 空 repo) → 友好错误
  - queue 满 (mock reviews 表 50+ pending) → accepted=true + can_propose=false
- `cmd/wikimind/command_test.go`：worktree list / remove 命令存在 + 基础 flow

目标测试总数：145 → 175+（migration 5 + worktree 8 + session 6 + handshake 6 + CLI 2）

### H. 跨平台

- worktree 路径：`filepath.Join(vaultRoot, "wiki", "_worktrees", ...)`（Windows OK）
- git worktree add 子命令在 Windows 也支持（git 2.5+）
- session token：`crypto/rand` + `hex.EncodeToString` 跨平台一致
- migration SQL：纯 SQLite 标准 SQL，无平台依赖

## Acceptance Criteria

- [ ] `agent_handshake` MCP tool 注册（第 10 个 tool，ReadOnlyHint=false）
- [ ] migration 0003 reviews + bundles 表 + 索引创建
- [ ] worktree 子系统：create / remove / list 全工作
- [ ] worktree 权限矩阵 helper：5 路径分类正确
- [ ] session store: register + lookup + touch + expire + 并发 safe
- [ ] handshake 6 失败模式全 cover（白名单/schema/session 重复/queue full/worktree 失败/happy）
- [ ] CLI `wikimind worktree list/remove` 工作
- [ ] config 新增 `allowed_agents` 字段 + init 时默认 5 agent 白名单
- [ ] 单测：≥ 30 个新测试 (migration 5 + worktree 8 + session 6 + handshake 6 + CLI 2 + reviews/bundles CRUD 5+)
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-H 全 done
- CI 5 OS 全绿
- 测试 ≥ 175（baseline 145 + 30+）
- commit + push

## Out of Scope

- propose_page / propose_edit / propose_claim（D11）
- request_review / log_append（D11）
- CLI `wikimind review accept/reject/diff`（D12）
- review accept 流程：apply patch + git commit + change_log（D12）
- session 持久化（D10 内存即可；持久化留 W3 daemon 重启场景）
- rate_limits 真实 enforce（D10 占位返回；W3 D18 review-queue-policy）
- queue hard_limit 50 配置化（D10 hardcode；W3 D18）
- worktree TTL idle 自动清理 goroutine（D10 留 Expire helper；不启动定时任务）
- watcher / FSEvents（D13）
- multi-vault session（D10 单 vault 假设）

## Decision (ADR-lite)

**Context**: D10 同时引入 3 个新概念（handshake / worktree / reviews/bundles 表）
体量大；session 状态在 daemon 重启会丢失；rate_limits 真实 enforce 需要全局
计数。

**Decision**:
1. **session 内存存储**：D10 SessionStore 用 sync.RWMutex map。daemon 重启
   = 所有 session 失效（agent 重 handshake 即可），acceptable for MVP；
   持久化到 SQLite 留 W3
2. **rate_limits 占位返回**：handshake response 写死 30/60，不真 enforce。
   wired enforce 在 W3 D18 一起做（与 review queue 上限保护配套）
3. **queue_state.pending COUNT(\*)**：每次 handshake 现场算 COUNT(\*) FROM
   reviews WHERE status='pending'。reviews 表小（< 1k 行），全表扫 OK；
   有 `idx_reviews_status` index 加速
4. **worktree 自动清理**：D10 只提供 `SessionStore.Expire()` helper 返回
   过期 session 列表；不启动定时清理 goroutine（避免 D10 引入异步 worker）。
   清理由 W3 daemon 主循环驱动
5. **schema_version 比较**：major 版本比对（"1.0" vs "1.0" OK / "1.0" vs
   "2.0" 不兼容）。minor 差异允许（forward compat）
6. **agent 白名单默认值**：5 个主流 agent（claude-code / codex-cli /
   cursor / cline / opencode）。User 可在 .wikimind/config.toml 增删

**Consequences**:
- 优点：D10 deliverable 不依赖 W3 复杂特性，最小可用 multi-agent 准备
- 缺点：daemon 重启丢 session（agent 需重 handshake）；rate_limits 字段
  返回但不 enforce（agent 看到 spec 但可绕过）—— W3 D18 收尾
- D11 propose_* 可直接基于 D10 reviews 表 + worktree 进行

## Technical Notes

- `git worktree add` syntax：`git worktree add <path> -b <branch>` 自动建 branch
- `git worktree remove --force <path>`：力删（worktree 有未 commit 改动也删）
- `git worktree list --porcelain`：machine-readable，每个 worktree 块以空行分隔
- session_token 生成：`crypto/rand.Read(16 bytes)` → `"sk-" + hex.EncodeToString(b)`
  （32 hex chars + 3 prefix = 35 chars）
- worktree 路径中 sessionID 必须合法：regex `^[a-zA-Z0-9_-]{1,64}$`，否则
  ErrInvalidSessionID。防御 `..` traversal + shell 注入
- worktree branch name 同 sessionID 规则（git branch name 严格限制）
- reviews.id format：`r-{seq:04d}`（4 位数字，> 9999 时自动扩位）
- bundles.id format：`b-{seq:04d}`
- 错误码映射 (MCP CallToolResult.IsError + content)：
  - `AGENT_NOT_WHITELISTED` / `SCHEMA_INCOMPATIBLE` / `SESSION_EXISTS` /
    `QUEUE_FULL` / `WORKTREE_CREATE_FAILED`
  - 包含 `code` 字段在 content text 让 agent parse

## 实施建议顺序

User 实现时建议顺序：
1. **migration 0003 + reviews/bundles CRUD**（基础数据层，独立可测）
2. **worktree 包**（独立可测，不依赖 mcp）
3. **session store**（独立可测）
4. **config allowed_agents**（小改动）
5. **agent_handshake handler**（最后整合 1-4）
6. **CLI worktree list/remove**（debug 用，可选）
7. 测试 + ≥175 验证
