# W2 D12: review accept/reject/diff CLI + apply patch 到 main

## Goal

实现 user 审稿台：`wikimind review list / show / accept / reject / diff`
让 user 看到 D11 propose 进来的 patch，决定是否合并到 main branch。Accept
流程严格按 `architecture.md §3.3` 13 步：read patch → re-validate quote_hash
→ git apply → commit + change-log + index update → 清 patch + reviews.status
`pending → accepted`。失败任何一步全回滚。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D12
- `spec-v2/docs/architecture.md §3.3`（Review 流程 13 步）
- `spec-v2/docs/agent-protocol.md §4.3`（worktree → main 合并路径）
- D11 已建：reviews/bundles 表 + `wiki/_review/r-NNNN.patch` 文件 + proposal 校验器

## What I already know

- D6 `internal/commit/` 含 `GitAdd / GitCommit / GitRevert` + `commit.Commit`（写 source + log.md + change-log.jsonl 一起）
- D9 `internal/index/anchor.go` `QuoteHash` + `ResolveAnchor`—— accept 时复用
- D10 `internal/index/reviews.go` 已有 `GetReviewByID / UpdateReviewStatus / ListReviewsByStatus`
- D10 `internal/index/bundles.go` 已有 bundles CRUD
- D11 `internal/proposal/validator.go` 已有 `ValidateClaimSources / ValidateBaseHash` 校验
- D11 patches 在 `wiki/_review/r-NNNN.patch`（unified diff，git apply 可消化）

## Requirements

### A. `internal/service/review.go`（新）

review accept/reject 业务层，承担 architecture §3.3 13 步原子性：

```go
// AcceptOptions 接受参数
type AcceptOptions struct {
    ReviewID    string
    AcceptedBy  string  // "user" 默认 / agent name 自动
    SkipReindex bool    // CI / debug 用
}

// AcceptResult 返回值
type AcceptResult struct {
    ReviewID  string
    GitSHA    string  // commit hash (short)
    Seq       int     // change-log seq
    Files     []string  // 实际改动的文件
}

// AcceptReview 13 步原子接受流程
func AcceptReview(ctx context.Context, vaultRoot string, db *index.DB, opts AcceptOptions) (*AcceptResult, error) {
    // 1. Validate review_id exists, status = pending
    // 2. Read patch from wiki/_review/r-NNNN.patch
    // 3. (Optional) Re-validate quote_hash / base_hash (D11 validator)
    // 4. Validate schema (D11 ValidateFrontmatter)
    // 5. Build commit message: "<op>: <summary> (seq=<N>) [r-<rid>]"
    // 6. SQLite tx begin
    // 7. git apply <patch>
    // 8. Verify: re-parse modified file, ensure valid markdown + frontmatter
    // 9. commit.Commit(ctx, vaultRoot, op, summary, files) — D6 already does git add + commit + log + change-log
    // 10. index.UpsertPage if page changed (reindex single file)
    // 11. Delete patch file from wiki/_review/
    // 12. UpdateReviewStatus(reviewID, "accepted", acceptedBy)
    // 13. tx commit
    // ON ANY FAILURE: git reset --hard HEAD + tx rollback + restore patch file
}

// RejectReview 状态机：pending → rejected (无 git 影响)
func RejectReview(ctx context.Context, db *index.DB, reviewID, rejectedBy, reason string) error
```

错误：`ErrReviewNotPending / ErrPatchMissing / ErrPatchApplyFailed / ErrPostApplyValidationFailed / ErrAcceptRollbackPartial`

### B. CLI `cmd/wikimind/command.go` 加 `wikimind review` 子命令组

5 个子命令：

#### B1. `wikimind review list`
- flags: `--status pending|accepted|rejected|all` (default pending) / `--bundle <id>` / `--agent <name>` / `--limit N`
- 调 `index.ListReviewsByStatus`
- 输出：表格 (review_id / op / target / agent / created_at / status / bundle_id)

#### B2. `wikimind review show <review_id>`
- 详细：review row + patch 内容预览（前 N 行）+ meta_json 解析
- `--full` 显示完整 patch

#### B3. `wikimind review diff <review_id>`
- 直接 cat patch 文件内容（unified diff，pretty print with color in TTY）
- `--no-color` 强制纯文本

#### B4. `wikimind review accept <review_id>`
- 调 `service.AcceptReview`
- 默认询问 user 确认（"Apply r-NNNN to main? [y/N]"）；`--no-confirm` 跳过
- `--bundle <bid>` accept 整个 bundle（遍历 reviews，逐个 accept，全成功才整 bundle status=accepted；任一失败整 bundle rollback）
- 输出：accepted commit sha + change-log seq

#### B5. `wikimind review reject <review_id>`
- `--reason <text>` 必填（最少 10 字符，同 propose_delete reason 约束）
- 调 `service.RejectReview` → reviews.status='rejected' + decided_by/at 填
- patch 文件**不删**（保留作 audit；W3 加 GC）

从 stub 列表移除 `review`（参考 D5 query / D6 revert 升级 pattern）。

### C. Bundle accept 流程

`wikimind review accept --bundle <bid>`：
- 查 bundles 表 → 遍历 review_ids
- 逐个 `service.AcceptReview`，**遇错全停**
- 全成功 → `index.UpdateBundleStatus(bid, "accepted")`
- 部分失败 → 已 accept 的 reviews 保留 accepted 状态 (single-writer 已 commit)，未 accept 的保持 pending，bundle 状态 'partial'（新 status，schema 兼容）

**简化**：D12 阶段 bundle accept 是 best-effort（不试图整 bundle 回滚已 accept 的），user 看到 partial 提示自己决定。完美原子 bundle accept 留 W3。

### D. CLI `wikimind log` 新增

D6 实现了 log.md 写入，但没读端。D12 加：
- `wikimind log [--limit N] [--seq <range>] [--actor <name>] [--op <op>]`
- 读 change-log.jsonl 反向（最新在上）
- 输出：表格 + 高亮 op (ingest/accept/reject/revert/append_log)

这是 D12 范围的小补 —— 让 user 看到 review accept 后 log 真的增了行。

### E. 测试

- `internal/service/review_test.go`：
  - AcceptReview happy（D11 propose → D12 accept → 验文件真的在 main + commit + log + reviews.status=accepted）
  - AcceptReview review not found → ErrReviewNotFound
  - AcceptReview status != pending → ErrReviewNotPending
  - AcceptReview patch 丢失 → ErrPatchMissing
  - AcceptReview git apply fail → 全回滚（git status clean / reviews.status 仍 pending / patch 文件还在）
  - AcceptReview post-apply validation fail → 全回滚
  - RejectReview happy / reason 缺失
  - Bundle accept happy（多 review 全 accepted） / partial（中间一个 fail）
- `cmd/wikimind/command_test.go`：5 个 review 子命令存在 + 基础 flow
- E2E in `command_test.go`：D11 propose → D12 accept → 验 main branch 有新文件 + page index 中可 query

目标测试总数：当前（fix-d11 完后 186）→ ≥210（+24：service 12 + CLI 5 + E2E 3 + log 4）

## Acceptance Criteria

- [ ] `wikimind review list/show/diff/accept/reject` 5 子命令工作
- [ ] AcceptReview 严格 13 步，任何 step 失败全回滚（git + tx + patch 文件状态都恢复）
- [ ] reviews.status 从 'pending' 转 'accepted' / 'rejected' 正确
- [ ] accept 后 wiki/_review/r-NNNN.patch 被清；reject 后保留（audit）
- [ ] commit message 含 `(seq=N) [r-NNNN]` 反查锚点
- [ ] Bundle accept 多 review 串行 + partial 状态正确
- [ ] `wikimind log` 读 change-log 显示历史
- [ ] 单测：≥ 24 个新测试
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-E done
- CI 5 OS 全绿
- 测试 ≥ 210
- commit + push

## Out of Scope

- patch GC（rejected 的 patch 文件长期清理 W3）
- bundle 完美原子 accept（W3 single-writer commit loop daemon）
- review 优先级排序（W3 D18 review-queue-policy）
- 自动 lint 在 accept 前跑（W3 D17 lint）
- conflict detection（W3 D15 lock manager 一起）
- accept 后 worktree auto sync（W3 daemon 主循环）

## Decision (ADR-lite)

**Context**: review accept 13 步涉及 git apply + SQLite tx + 文件系统三方
原子性。完美原子要 daemon single-writer commit loop（W3）；D12 是 MVP CLI
版本，user 单进程串行，原子性靠"逐步前进 + 失败时 git reset --hard +
patch 文件恢复"近似实现。

**Decision**:
1. **D12 single-process accept**：所有 accept 调用在 user CLI 进程内串行，无
   daemon。git apply 失败 → git reset --hard HEAD + 把 patch 文件留在
   _review/ + reviews.status 仍 pending → 等价"未发生"
2. **post-apply validation**：apply 完后 re-parse 文件确保 frontmatter +
   markdown 合法；不合法回滚——防 patch 把 page 写坏
3. **commit message 嵌 review_id**：format `<op>: <summary> (seq=N) [r-NNNN]`
   方便 `git log --grep "r-0245"` 反查
4. **bundle partial 状态**：bundles.status 加新值 'partial' (schema 兼容，
   D10 已有 TEXT 字段没 ENUM 约束)，user 看到提示决定 retry 还是 manual
5. **reject 不删 patch**：reject 的 patch 留 _review/ 作 audit；W3 D17 lint
   有 stale review 规则可批量清理

**Consequences**:
- 优点：D12 单进程 MVP 简洁，user 体验：propose → accept → 看 main 改了；
  失败可重试（patch 还在）
- 缺点：极端情况（user 中途 Ctrl-C）可能 patch 已 apply 但 reviews.status
  没更新 → 下次 accept 同 review 会再 apply 失败 → user 手工清。W3 daemon
  解决（journal 模式）
- 与 D11 wired：D11 propose_* 生成 patch 文件 + reviews row；D12 消费这两者

## Technical Notes

- `git apply --check <patch>` 先 dry-run，验证 patch 适用，再 `git apply`
- `git reset --hard HEAD` 回滚 working tree（保留 reviews 表 SQLite tx 回滚另路）
- D6 `commit.Commit` 内已含 git add + commit + log + change-log，accept
  调用它即可——`op="accept"`，summary 用 review meta_json 的 summary 或自动生成
- patch 文件路径：`filepath.Join(vaultRoot, "wiki", "_review", reviewID+".patch")`
- post-apply 文件 list 从 patch 头解析：`diff --git a/<path> b/<path>` 抽 path
- `service.AcceptReview` 自带 `ReindexPage(ctx, db, pageRow)` 调用（incremental），
  避免全 vault reindex
- bundle accept 的 partial 状态：CLI 输出明确告诉 user "X/Y reviews accepted,
  Z failed: retry with `wikimind review accept r-NNN`"

## 实施建议顺序

1. **internal/service/review.go AcceptReview + RejectReview 函数 + 错误**
2. **CLI: review list / show / diff**（最简单 read-only）
3. **CLI: review reject**（写状态，简单）
4. **CLI: review accept**（核心，13 步原子）
5. **CLI: review accept --bundle**
6. **CLI: wikimind log**（D6 read 端补足）
7. **测试 + ≥ 210 验证**
