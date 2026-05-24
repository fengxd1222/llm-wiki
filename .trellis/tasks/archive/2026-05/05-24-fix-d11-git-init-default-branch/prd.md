# fix: D11 CI fail — git init default branch 跨平台不一致

## Goal

修 D11 push 后 CI 全 5 Go job fail：

```
--- FAIL: TestD11ProposePageAndRequestReview
    tools_test.go:268: handleProposePage: git diff wiki/concepts/review-gate.md: fatal: bad revision 'main'
--- FAIL: TestD11ProposeEditFrontmatterBody
    tools_test.go:432: handleProposeEdit: BASE_HASH_MISMATCH: read base page wiki/concepts/edit-me.md: fatal: invalid object name 'main'.
--- FAIL: TestD11ProposeEditUnifiedDiff (同上)
--- FAIL: TestD11ProposeClaimHappyAndQuoteMismatch (同上)
```

本机 (macOS) pass 因 git config `init.defaultBranch=main`；CI 全新机器无此
配置 → `git init` 创建 `master` 而非 `main`，但 `internal/proposal/patch.go`
hardcode 用 `main` 做 diff target → "bad revision 'main'"。

## What I already know

- `internal/commit/git.go:42` `EnsureRepo` 调 `git init` 不传 `--initial-branch`
- `internal/proposal/patch.go:29-36` `GeneratePatch` hardcode `"main"` 在 diff cmd
- git ≥ 2.28 (2020-07) 支持 `--initial-branch=<name>` flag
  - CI matrix 全部 git 远新于 2.28（ubuntu-22.04 默认 git 2.34, macos-14/15 git 2.44+, windows-2022 git 2.45+）
- 现有 vault 是 master branch 的情况也要 handle（防回归——某些用户从老 repo migrate）

## Requirements

### A. `internal/commit/git.go` EnsureRepo 强制 main

修 `EnsureRepo`：
```go
// 不是仓库 → 自动 `git init --initial-branch=main`
if _, err := runGit(ctx, vaultRoot, "init", "--initial-branch=main"); err != nil {
    return fmt.Errorf("git init: %w", err)
}
```

**额外**：handle 现有 repo 是 master 的情况——`EnsureRepo` 返回 nil 前先 normalize：
```go
// 如果当前 HEAD 是 master 且 main 不存在，rename master → main
// (idempotent: 如果已 main 则 no-op)
if err := ensureMainBranch(ctx, vaultRoot); err != nil {
    return fmt.Errorf("ensure main branch: %w", err)
}
```

`ensureMainBranch` 实施：
```go
func ensureMainBranch(ctx context.Context, vaultRoot string) error {
    // 查当前 branch 名 (`git symbolic-ref --short HEAD`)
    // 若已 'main' → return nil
    // 若 'master' 且 'main' 不存在 → `git branch -M main`
    // 其他情况 → return nil (尊重 user 自定义 branch name)
}
```

注意：fresh `git init` 后 HEAD 指 unborn branch，`symbolic-ref --short HEAD`
会返回 branch 名但若无任何 commit `git branch` 是空——`git branch -M main`
在 unborn HEAD 上仍 work（git ≥ 2.30）。

### B. `internal/proposal/patch.go` 不 hardcode "main"

改为运行时检测 base ref，或参数化：

选项 1（推荐，参数化）：`GeneratePatch` 加 `baseRef` 参数：
```go
func GeneratePatch(ctx context.Context, vaultRoot, branch, path, baseRef string) ([]byte, error)
```
调用方传 `"main"`，但 helper 集中调用方可改。

选项 2（更省）：保留签名，patch.go 内部用 `defaultBaseRef()` 兜底：
```go
func defaultBaseRef(ctx context.Context, vaultRoot string) string {
    // 优先 main，缺则 master，再缺则 HEAD~1
    // 用 `git rev-parse --verify <ref>` 判断存在
}
```

**决策**：**选项 2**——既存调用方无需改签名，向后兼容；通过 helper 在
`patch.go` 内部解决。`baseRef` 内部固定 `defaultBaseRef`。后续 D12+ 若需
显式传 ref（如 review accept 时 main 可能已 advance），再加参数。

### C. `internal/worktree/worktree.go` 也用 main

worktree.go `CreateWorktree` 用 `git worktree add <path> -b <branch>` 基于
当前 HEAD（不 hardcode main），但创建 branch 时 base 是 HEAD —— 若 HEAD
不是 main 会有问题。**verify 后看是否需改**。

### D. 测试

新增 `internal/commit/git_test.go`：
- `TestEnsureRepoCreatesMainBranch`：fresh dir → EnsureRepo → 验 branch 是 `main`
- `TestEnsureRepoRenamesMasterToMain`：mock init 成 master repo → EnsureRepo → 验 rename 到 main
- `TestEnsureRepoIdempotentOnMain`：已 main 的 repo → EnsureRepo → no-op

`internal/proposal/patch_test.go`（已有）跑全 OS 验证 ——本机已 pass，CI 再跑确认。

### E. CI smoke 加固

`cmd/wikimind/command_test.go` `TestW1DemoWalkthroughCISmokeTest` 已跑 init →
ingest → query → revert。**加一步断言**：
```go
out, _ := exec.Command("git", "-C", vault, "symbolic-ref", "--short", "HEAD").Output()
if strings.TrimSpace(string(out)) != "main" {
    t.Fatalf("expected branch 'main' after init, got %q", out)
}
```
跨平台 verify default branch 是 main——防回归。

## Acceptance Criteria

- [ ] `EnsureRepo` 加 `--initial-branch=main` + `ensureMainBranch` rename helper
- [ ] `patch.go` `defaultBaseRef` 兜底解析 main / master / HEAD~1
- [ ] CI smoke 测试断言 init 后 branch=main
- [ ] D11 4 个 fail 测试在 CI 5 OS 全 pass
- [ ] 既有测试不破坏（183 测试不降）
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-E done
- CI 5 OS 全绿（特别 D11 4 个测试 pass）
- 测试 ≥ 183（baseline + 3 git_test 新）
- commit + push

## Out of Scope

- 允许 user 自定义 branch name（trunk / develop 等）—— 现在仅强制 main
- migration 工具把现有 master vault 自动 rename —— `ensureMainBranch` 已隐式
  handle，无需独立工具
- patch.go 加 `baseRef` 参数（选项 1）—— 留 D12 review accept 真需要时

## Decision (ADR-lite)

**Context**: D11 CI 全平台 fail 因 `git init` default branch 跨平台不一致。
3 修法：(a) `--initial-branch=main` + rename helper（生产代码改） /
(b) 运行时检测 ref（patch.go 兜底） / (c) 测试 setup 强制（不修生产）。

**Decision**: **A + B 组合**——双层防御：
1. **A**（EnsureRepo 强制 main）：让所有新 vault 一致用 main，根除问题
2. **B**（patch.go defaultBaseRef）：兜底 handle 既有 master vault 或 user
   手动改 branch 的情况

不选 C：测试 setup 一致不修生产代码，但生产代码 patch.go 仍 hardcode main—
真用户跑也会遇到 CI 同样问题（如果他们 git config 不一样）。

**Consequences**:
- 优点：root cause 修，新老 vault 都 work；CI 一次绿
- 缺点：ensureMainBranch 引入小复杂度（branch rename 逻辑）；后续 D12+ 若
  支持自定义 base ref 还要重构 defaultBaseRef
- 兼容：git ≥ 2.28（所有 CI 平台都满足）

## Technical Notes

- git `--initial-branch` flag：git 2.28 (2020-07) 起；CI 平台 git 2.34+ 全
  满足
- `git symbolic-ref --short HEAD`：unborn HEAD（fresh init 未 commit）也返
  branch name（"main" 或 "master"），不报错
- `git branch -M <name>`：force rename 当前 branch；unborn HEAD 上也 work
- `git rev-parse --verify <ref>`：ref 存在返 sha + exit 0；不存在 exit 1
  + stderr —— 用于 `defaultBaseRef` 探测
- 跨平台：所有逻辑通过 exec git 走，git binary 行为一致

## 实施建议顺序

1. **patch.go defaultBaseRef helper**（独立可测）
2. **EnsureRepo + ensureMainBranch**（独立可测，3 个 unit test）
3. **CI smoke 加断言**（防回归）
4. local 跑 `go test -count=1 ./...` 全绿
5. push → CI 5 OS verify D11 4 测试转绿
