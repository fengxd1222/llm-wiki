# W4 D28: Dogfooding + 朋友试用 + bug 收集

## Goal

本人 dogfood + 3 个朋友试 `wikimind demo`，收 bug + UX 痛点 → 出 bug 列表
给 D29 修。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D28

## Requirements

### A. Personal dogfooding

7 天内每天用 wikimind 处理一篇论文 / 文章：
- 上传 PDF / Markdown 到 raw/inbox/
- Claude 经 MCP ingest + propose claims
- user review + accept
- query 验证可检索

记录到 `docs/dogfood/2026-05-25-week.md`：
- 每天 1 段：今天的 ingest 内容 + 遇到的痛点 + bug

### B. 3 朋友试用

- 给 3 friends 装 wikimind demo
- 30 分钟试用 + 录屏
- 收集表（Google Form 或 markdown）：
  - 安装是否顺利？
  - demo 是否 5 分钟内完成？
  - 最 confusing 步骤？
  - 最 delightful 步骤？
  - 是否会再用？
- 结果到 `docs/dogfood/friends-feedback.md`

### C. Bug 列表

`docs/dogfood/bugs-w4-d28.md`：
- 按 severity 排序（P0 / P1 / P2）
- 每条含：reproduce steps / expected / actual / proposed fix
- D29 修 P0 + P1，P2 推 V0.2

### D. 测试增量

dogfood 中发现的 edge case 都写 unit test 防 regression。

目标 ≥ 605（D27 后 590 + 15）。

## Acceptance Criteria

- [ ] 7 天 dogfood 记录
- [ ] 3 friends 反馈收齐
- [ ] bug 列表分级
- [ ] CI 5 OS 全绿；测试 ≥ 605

## Out of Scope

- 公开 beta（V0.1.0 发布后 W5+）
- A/B test
- 性能 benchmark（D29）
