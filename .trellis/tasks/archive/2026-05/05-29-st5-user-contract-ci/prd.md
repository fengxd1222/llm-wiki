# ST5 · User Contract + CI — Worker Honesty + govulncheck

> 父任务：`05-28-v0-1-1-patch-13-fixes-from-v0-1-0-audit`
> 维度：用户契约期望管理 + CI 增强。含 F-003+F-069 + govulncheck（F-075 follow-up）。

## Goal

修复 worker / doctor 对用户的虚假"PDF ready"暗示，并把依赖漏洞扫描纳入 CI。**只改这块。**

## Fixes

### F-003 + F-069 · Python worker 仍是 skeleton 但 doctor 假装 ready【P1 / 用户契约】

- **位置**：
  - `worker/main.py:1-37`（docstring 自承"W0 skeleton"）
  - `worker/pyproject.toml`（description / dependencies）
  - `cmd/wikimind/command.go:689`（doctor 检查 pypdf 标 ✓）
- **现状**：doctor 输出"✓ pypdf"暗示系统能解析 PDF，实际 worker 只回 skeleton 事件、Go 侧无人调 worker.py。用户被误导。
- **修复方向**：
  1. `worker/main.py` docstring 改成 `"WikiMind ingest worker — W0 skeleton (PDF/image parsing deferred to v0.2)"`。
  2. `worker/pyproject.toml`：description 写明 skeleton；`dependencies = []` 显式（不挂 pypdf 假承诺）。
  3. `cmd/wikimind/command.go:689`：把 pypdf 检查从 `✓/✗` 改成 `⚠ pypdf: optional, not used by v0.1.x worker`，或直接删除该检查项（实现时选其一，倾向改 warning 保留信息）。
- **测试**：doctor 命令输出断言更新（command_test.go 中 doctor 相关用例，若有）；worker.py 无需单测（skeleton）。

### govulncheck CI step（F-075 follow-up）【依赖安全】

- **背景**：F-075 离线核对未发现已知 CVE，但离线核对查不到 2025-2026 新 CVE，建议 CI 跑 `govulncheck`。
- **修复方向**：
  1. GitHub Actions 加 step：`go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...`。
  2. v0.1.1 设为 **warning gate**（`continue-on-error: true` 或仅打印），不 hard fail；v0.2 升 hard。
  3. 与 ST1 的 windows CI matrix 协调——若 ST1 已建 `.github/workflows/ci.yml`，本任务在其上加 step（注意两子任务都碰 CI yaml，实现时后做的要 rebase 前者改动）。
- **测试**：CI 配置本身无单测；本地可跑一次 `govulncheck ./...` 确认无 high-severity 命中。

## 与 ST1 的 CI 文件协调

ST1 和 ST5 都改 `.github/workflows/ci.yml`。建议：
- 若 ST1 先做 → 本任务在其 matrix 基础上加 govulncheck job/step。
- 若本任务先做 → 创建基础 ci.yml + govulncheck，ST1 再加 windows matrix。
- 实现时先 `ls .github/workflows/` + `git log` 看对方是否已动过。

## Acceptance Criteria

- [ ] F-003/069：worker.py docstring + pyproject 明示 skeleton；doctor 不再用 ✓ 误导 pypdf。
- [ ] doctor 输出对 pypdf 改为 optional/warning 或移除。
- [ ] govulncheck 纳入 CI（warning gate）。
- [ ] 本地 `govulncheck ./...` 跑通（记录结果；high-severity 命中需上报）。
- [ ] `go build ./...` / `go vet ./...` / `go test ./...` 全绿。
- [ ] 未触碰范围外 finding。

## Out of Scope

- ❌ 其他 finding（ST1~ST4）。
- ❌ Python worker 真实 PDF 解析（推 v0.2 或 deprecate）。
- ❌ govulncheck 升 hard fail（v0.2）。
- ❌ 修改 `.trellis/spec/`。

## Technical Notes

- 审查报告：F-003 / F-069 / F-075 详条。
- 标尺：`quality-guidelines.md`（测试要求 / 用户契约）/ Python 用 PEP 8。
- worker/main.py 是 stdin→stdout NDJSON skeleton；Go 侧目前无 caller（grep 证实）。
- CI 文件与 ST1 共享，注意协调避免互相覆盖。
