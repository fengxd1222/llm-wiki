# W4 D30: v0.1.0 发布 + retrospective + v0.2 roadmap

## Goal

发布 v0.1.0：tag → release notes → 上传 binary → Homebrew bump → 公开 GH
Release。同时收集前 10 用户反馈，写 retrospective 和 v0.2 roadmap。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D30

## Requirements

### A. Release 流程

1. `git tag v0.1.0` + `git push origin v0.1.0`
2. release.yml CI 自动出包 4 OS × 2 binary
3. GH Release 创建：
   - title: `WikiMind v0.1.0 — Local-first multi-agent wiki`
   - notes: 复用 `CHANGELOG.md` 0.1.0 section
   - assets: 8 binaries + checksums.txt + SBOM
4. Homebrew tap PR 自动开 (release.yml)
5. 公告：
   - Hacker News show post
   - Reddit r/ObsidianMD r/Anytype
   - Twitter / X

### B. CHANGELOG.md

`CHANGELOG.md`（新）按 Keep a Changelog 格式：
- v0.1.0 (2026-06-23, 实际 D30 日期)：
  - Added: ingest / query / propose / accept / mcp / lint / watcher / demo
  - Known issues: P2 bugs from D28
  - Migration: N/A (first release)

### C. v0.2 roadmap

基于 dogfood + early user 反馈：
- 候选特性：
  - Embedding rerank (search type=fts+vector)
  - Dream Cycle consolidate / evolve
  - propose_delete / propose_merge
  - LLM-driven topic clustering
  - Web UI (review queue + browse)
  - Multi-vault support
- 排优先级（user vote / 自己判断）
- 写入 `roadmap-v02.md`

### D. Retrospective

`docs/retro/v0.1.0.md`：
- 30 天日志摘要（链接 W1-W4 walkthrough）
- 决策回顾（关键 ADR-lite 哪些验证 / 哪些反转）
- 流程 retro：trellis workflow 体验、sub-agent quota、CI fail-fix loop
- v0.2 重点 + 时间预估

### E. 测试

无新代码；release pipeline 跑通。

目标 ≥ 640（D29 数）。

## Acceptance Criteria

- [ ] v0.1.0 tag + GH Release + 8 binary
- [ ] Homebrew formula PR opened
- [ ] CHANGELOG.md
- [ ] v0.2 roadmap 草稿
- [ ] retrospective 写完
- [ ] **W4 全部 9 天完成 (D22-D30) + 整 30 天 MVP 完成**

## Out of Scope

- v0.1.1 patch（推 W5 emergency）
- 多语言公告
- Paid tier discussion

## Decision (ADR-lite)

**Decision**：v0.1.0 是公开 alpha；不承诺向后兼容（schema migration tool
D25/D26 已建底子，V0.2 可破坏性变更带 migration）。Homebrew tap 主推 macOS；
Windows MSI 二级；Linux deb 三级。

## Technical Notes

- semver pre-release: 0.1.0 是 alpha；0.2.0 beta；1.0.0 GA
- SBOM 用 `cyclonedx-gomod`
- release.yml 借鉴 fzf / gh release.yml
