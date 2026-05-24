# W1 D6: change-log + git auto-commit + revert

## Goal

实现"每次 user 主动操作（ingest / page reindex）→ 自动 git commit + 写
`wiki/log.md` 一行 + `.wikimind/change-log.jsonl` 一行"的闭环。让 W1 出口
demo "ingest → 生成 source page → 更新 index.md → log.md 增行 → git commit"
能在 macOS + Windows 跑通。同时实现 `wikimind revert <seq>` 走 git revert
+ 新 op=revert log 行。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W1 D6（log.md + change-log.jsonl + auto-commit + revert）
- `spec-v2/docs/agent-protocol.md §7`（change-log 格式 + append-only 强制）
- `spec-v2/docs/architecture.md §2.3`（Single-Writer Commit Loop——MVP 简化版）
- `spec-v2/docs/engineering-decisions.md §4.4`（exec git，不用 go-git）

## What I already know

- D3 `wikimind ingest` 已写 raw/inbox/，未 git commit
- D4 `wikimind page reindex` 已写 SQLite pages 表，未 git commit
- `wiki/log.md` 当前不存在；`.wikimind/change-log.jsonl` 也不存在
- `cmd/wikimind/command.go` `review / lint / revert` 三个 stub 未实现；D6 把 revert 升为真实命令
- engineering-decisions §4.4 已定 `exec.Command("git", ...)`（非 go-git）
- architecture §2.3 Single-Writer Commit Loop 是 W2 daemon 的事；W1 MVP
  阶段直接用串行函数调用 + service-layer mutex 就够
- agent-protocol §7.3 强制 append-only：任何"修改"只能新写 op=revert log 行

## Requirements

### 依赖

无新 Go 依赖（exec git + os 标准库 + encoding/json）。

### 新包：`internal/commit/`

- `change_log.go`：
  - `LogEntry` struct：`Seq / GitSHA / Timestamp / Actor / Op / Bundle? / Reviews? / Summary`
  - `AppendChangeLog(ctx, vaultRoot, entry) error`：原子追加到 `.wikimind/change-log.jsonl`（O_APPEND）
  - `AppendLogMd(ctx, vaultRoot, entry) error`：追加到 `wiki/log.md`（首次写自动建表头）
  - `NextSeq(vaultRoot) (int, error)`：读 `change-log.jsonl` 最后一行 + 1（无文件返回 1）
  - `ReadEntryBySeq(vaultRoot, seq) (LogEntry, error)`：revert 找 git_sha 用
- `git.go`：
  - `GitAdd(ctx, vaultRoot, paths...) error`：`exec.Command("git", "add", ...)`
  - `GitCommit(ctx, vaultRoot, message) (sha string, error)`：`git commit -m <msg>`，返回 short SHA
  - `GitRevert(ctx, vaultRoot, sha) (newSHA string, error)`：`git revert <sha> --no-edit`
  - `GitStatus(ctx, vaultRoot) ([]string, error)`：检查 dirty（pre-commit assert）
  - `EnsureRepo(ctx, vaultRoot) error`：首次 commit 前 `git init`（如果不是 repo）
- `commit.go`（service 入口，原子 boundary）：
  - `Commit(ctx, vaultRoot, op, summary, files) (LogEntry, error)`：
    1. `EnsureRepo` 确保 git 仓库
    2. `NextSeq` 取下一个 seq
    3. `GitAdd(files + wiki/log.md + .wikimind/change-log.jsonl)`
    4. **先写两份 log 到磁盘**（append）
    5. 重新 `GitAdd` 把 log 文件也加进去
    6. `GitCommit` → 拿 sha
    7. 用 sha 回填 entry 的 GitSHA → **rewrite 最后一行**（jsonl + log.md）
       - 这里有个 atomicity 问题：commit 时 log 文件已经在 staged 里，commit 后再改文件让 working tree dirty
       - **简化方案**：commit message 里**不带 sha**，commit 后用 `git log -1 --format=%h` 拿 sha
         写入 log；commit + log 写入失败 → 用 `git reset --soft HEAD~1` 回滚
- 错误：sentinel pattern `ErrNotGitRepo / ErrDirtyWorktree / ErrSeqNotFound`

### 简化决定（W1 MVP）

- **commit message 不嵌 sha**：format `<op>: <summary> (seq=<N>)`
  - example: `ingest: raw/inbox/karpathy.md (seq=1)`
- **写日志顺序**：先 commit（source files only）→ 后 append log（log 文件
  作为下一次 commit 的 staged 内容）
  - **trade-off**：log 和 commit 不在同一个 commit 里，跨 commit 滞后
    一拍——但简化原子性 + commit message 已含 seq 可以 cross-reference
  - alternative：log 和 source 一起 commit（atomic）但需要"commit 前
    sha 未知"的 placeholder → revert 时再 rewrite，比简化方案复杂得多
  - **决定**：MVP 选简化方案；agent-protocol §7.3 "1:1 对应 git commit"
    可在 W2 通过 daemon-level commit loop 改造为严格 atomic
  - **wait**: 重新审视 → 更简单的方案：**log 文件本身作为下一次 commit 的一部分**
    - commit_1: 文件 X + 自动建空 `wiki/log.md` 和 `change-log.jsonl`
    - 等等，这个还是要 2 个 commit
  - **最终决定（更干净）**：
    1. apply changes（如 ingest 写 raw 文件）
    2. `nextSeq = readLastSeq() + 1`
    3. **预先**写 log entries（git_sha 字段留空 ""）到 log.md / change-log.jsonl
    4. `git add <changed files> wiki/log.md .wikimind/change-log.jsonl`
    5. `git commit -m "<op>: <summary> (seq=<N>)"` → 拿 sha
    6. **追加性修正**：用 `git log -1` 拿到 sha 后，**不**回 patch 已 commit
       的 log 文件——而是在 `.wikimind/change-log.jsonl` 写一个**索引附录**
       `<seq> -> <sha>` 到 `.wikimind/seq-index.jsonl`（次表，可丢失重建）
    7. 实际查询时 join：`change-log.jsonl[seq]` + `seq-index.jsonl[seq->sha]`
  - **再简化**：seq-index.jsonl 也是 W2 的事；W1 MVP 接受 "change-log
    entry 里 git_sha 为空字符串，需要时用 git log + commit message 的 seq
    捞回"。`wikimind revert <seq>` 用 `git log --grep "seq=<N>"` 找 commit。

### 修改：`internal/service/page.go` + `internal/service/raw.go`（如果有）

- ingest 完成后调 `commit.Commit(ctx, vault, "ingest", summary, []string{ingestedPath})`
- page reindex 完成后调 `commit.Commit(ctx, vault, "reindex", summary, []string{".wikimind/index.db" 不 commit})`
  - **wait**: `.wikimind/index.db` 在 `.gitignore` 里（SQLite 是 derived state）
  - reindex 不改任何 git-tracked 文件——所以 reindex 不 trigger commit
  - **修正**：D6 的 auto-commit 触发**只有 ingest**（实际改了 git-tracked 的
    `raw/inbox/` 文件）；reindex 不 commit
  - 后续 D7 demo flow："手动放 markdown 到 raw/inbox/ → ingest → log.md
    增行 → git commit"，确实只有 ingest 这一步触发 commit

### 修改：`cmd/wikimind/command.go`

- `newRevertCommand`（替换 stub）：
  - `wikimind revert <seq>` → 找 seq 对应 commit → `commit.GitRevert` → 写新
    `op=revert` log 行（指向原 seq）
  - flag `--no-confirm` skip 二次确认（CI 用）
- 移除 `revert` 从 stub 列表（同 D5 query 升级 pattern）

### 测试

- `internal/commit/change_log_test.go`：
  - `NextSeq` 空文件返回 1、有 N 行返回 N+1
  - `AppendChangeLog` 并发安全（O_APPEND 单写，goroutine race 测试可省）
  - `AppendLogMd` 首次写自动 header
  - `ReadEntryBySeq` 命中 + miss
- `internal/commit/git_test.go`：
  - `EnsureRepo` 非 repo 自动 init
  - `GitCommit` 干净 worktree 报错（nothing to commit）
  - `GitRevert` 创建反向 commit
- `internal/commit/commit_test.go`：
  - `Commit` happy path：apply → log → git commit → 返回 entry 含 seq
  - 失败回滚（git commit fail → log 文件状态如何？MVP 接受 "log 已写但
    commit 失败"——下次 commit 一起带走；不 rollback log，因 append-only 强制）
- `cmd/wikimind/command_test.go`：
  - `TestRevertCommand`：ingest seq=1 → revert 1 → 新 commit + 新 log 行 op=revert
  - `TestStubCommands` 移除 revert
- E2E flow（在 `cmd/wikimind/command_test.go` 或单独）：
  - init vault → ingest 一个 md → 断言 wiki/log.md 有 1 行 + change-log.jsonl
    有 1 行 + git log 有 1 个 commit

## Acceptance Criteria

- [ ] `wikimind ingest <file>` 后自动 git commit + log.md + change-log.jsonl
- [ ] commit message 格式 `<op>: <summary> (seq=<N>)`
- [ ] `wikimind revert <seq>` 反向 commit + 写 op=revert 新 log 行
- [ ] log.md 首次写自动建 Markdown 表头
- [ ] change-log.jsonl append-only（每行有效 JSON）
- [ ] git 仓库不存在时自动 `git init`
- [ ] `wikimind` 在非 git 环境（无 git binary）友好报错
- [ ] 单测：NextSeq / AppendChangeLog / AppendLogMd / Commit / Revert /
  E2E (init + ingest + log 验证)
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 矩阵通过

## Definition of Done

- 测试覆盖 happy path + git missing + dirty worktree + revert 已 reverted
  (revert of revert)
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/`
- commit + push

## Out of Scope

- Single-Writer Commit Loop（channel + goroutine 串行化）—— W2 daemon 实施
- `bundle` / `reviews` 字段 —— W3 review pipeline
- `actor` 多值 —— W2 加 agent 区分；MVP 写 "user"
- seq-index.jsonl 反查 git_sha —— W2 性能优化
- Integrity check（daemon 启动检查 jsonl 完整性）—— W3
- Rejections memory（`.wikimind/rejections.jsonl`）—— W3 review 后做
- LFS / 大文件 worktree —— W4+

## Decision (ADR-lite)

**Context**: log.md + change-log.jsonl 需要 1:1 对应 git commit，但
commit 创建前 sha 未知，导致 atomic 写 log + commit 困难。

**Decision**: W1 MVP 采用 **commit message 嵌 seq + log 文件留空 git_sha
字段**。流程：
1. apply source change
2. 计算 nextSeq
3. write log entries (git_sha="")
4. git add source files + log files
5. git commit -m "<op>: <summary> (seq=<N>)"
6. 不回填 sha——`wikimind revert <seq>` 用 `git log --grep "seq=<N>" --format=%H`
   反查

**Consequences**:
- 优点：所有内容（source + log）在**同一个** git commit 里，原子且
  append-only 自然满足
- 缺点：change-log.jsonl 的 git_sha 字段为空——需要 grep 反查；W2
  daemon 可加 seq-index.jsonl 加速
- 与 agent-protocol §7.2 字段定义兼容（git_sha 是 optional——实际等于 ""
  时表示"用 commit message 的 seq 反查"）

## Technical Notes

- 包路径：`internal/commit/`（D5 同结构：service-layer thin wrapper 调
  internal lib）
- exec git 调用 working dir = vault root（用 `cmd.Dir = vaultRoot`，不靠 chdir）
- log.md 表头模板（首次写）：
  ```
  # Wiki Change Log

  | seq | ts | actor | op | summary |
  |-----|----|-------|-----|---------|
  ```
- change-log.jsonl 行格式严格 JSON（无 trailing newline 问题——`json.Encoder`
  自动加 `\n`）
- timestamp 格式：RFC3339 UTC（`time.Now().UTC().Format(time.RFC3339)`）
- revert 反查 commit：`git log --all --grep "seq=<N>" --format=%H` —— 注意
  `--grep` 是 regex，seq 字面要 escape；用 `\(seq=<N>\)` 加圆括号锚定避免
  误命中（如 seq=1 命中 seq=10）
- Windows：`exec.Command("git", ...)` 自动找 PATH 里的 `git.exe`；macOS/Linux
  找 `git`——cross-platform.md 无特殊处理
