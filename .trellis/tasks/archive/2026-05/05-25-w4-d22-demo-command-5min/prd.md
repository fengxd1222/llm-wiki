# W4 D22: `wikimind demo` 5 分钟闭环 + CI 自动测

## Goal

新用户跑 `wikimind demo` 5 分钟内体验完整 ingest → review → query →
浏览闭环。内嵌 3 个 sample raw + 确定性 claude-stub agent，无外部依赖。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D22
- `spec-v2/docs/onboarding.md`

## Requirements

### A. `wikimind demo` CLI 命令

`cmd/wikimind/command.go` 加 `newDemoCommand`：
- `wikimind demo [--target <dir>]` 默认 `~/wikimind-demo/`
- 不存在 → 初始化全新 vault
- 5 阶段：
  1. **init** vault + 写 3 个 sample raw（embed 在 binary 通过 `//go:embed`）
  2. **ingest** 3 sample → 自动 reindex + commit
  3. **mock review**：claude-stub agent propose 2 claim + 1 entity → user
     `wikimind review accept --all`
  4. **query** "compounding" → 命中 1 claim
  5. **浏览**：`wikimind page list` + `wikimind log --limit 10`

每阶段进度条 + ≤ 60s 完成。

### B. 内嵌 sample raw

`internal/demo/samples/` (go:embed)：
- `sample-1-karpathy-llm-wiki.md` (中文 + 英文 mix)
- `sample-2-systems-paper.pdf` (mini PDF 100KB)
- `sample-3-quote-image.jpg` (含 EXIF)

### C. claude-stub agent

`internal/demo/stub.go`：
- 确定性 mock agent，跑固定 propose_* 序列（不调真 Claude API）
- 用于 D14 / D21 / D22 CI 自动化

### D. CI 5 分钟测

`cmd/wikimind/demo_test.go`：
- 整 demo 流程跑 + 断言 ≤ 5 min（CI 测试 timeout 300s）

### E. 测试

≥ 495（D21 后 475 + 20）。

## Acceptance Criteria

- [ ] `wikimind demo` 5 阶段 ≤ 5 min
- [ ] CI 自动测时长断言
- [ ] sample raw embed
- [ ] claude-stub 确定性
- [ ] CI 5 OS 全绿

## Out of Scope

- Real Claude API integration（W4+）
- Interactive demo（GUI / TUI）
- Custom sample selection
