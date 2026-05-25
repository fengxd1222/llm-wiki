# W4 D27: 文档 README + 安装手册 + Quickstart + FAQ + failure playbook

## Goal

发布前文档全套：README + 双平台安装手册 + Quickstart + FAQ + failure
playbook 9 类命令测试。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D27
- `spec-v2/docs/failure-playbook.md` 全 9 类

## Requirements

### A. README.md

项目 root README 重写：
- 一句话价值（"local-first multi-agent wiki"）
- 5 行 quickstart（install + init + ingest + query）
- screenshot / asciinema gif
- 与 Obsidian / Logseq / Anytype 区别 ("Multi-agent native")
- 链接 docs/

### B. docs/install/

- `macos.md`：Homebrew install + permission 设置
- `windows.md`：MSI install + Scheduled Task 启用
- `linux.md`：deb / 源码编译

### C. docs/quickstart.md

10 分钟从 0 到完成 ingest + query + accept review。

### D. docs/faq.md

20 条 FAQ（基于 dogfooding D28 反馈，但 D27 准备框架）：
- "为什么 daemon 不启动？" → doctor
- "如何添加新 agent？" → templates / allowed_agents
- "vault 加密吗？" → 文件级（git）
- ...

### E. failure-playbook 9 类命令测试

`docs/playbook/`：
- `01-vault-corrupt.md`：fsck + restore
- `02-daemon-stuck.md`：kill + restart
- `03-review-queue-overflow.md`：reject all / bulk accept
- `04-quote-drift.md`：lint --rule=drift_check
- `05-merge-conflict.md`：worktree resolve
- `06-disk-full.md`：vacuum + GC
- `07-rollback-bad-accept.md`：revert-cascade
- `08-agent-token-leak.md`：rotate session
- `09-fresh-install-failed.md`：uninstall --purge + reinstall

每个含 reproducible test case + expected output。

### F. 测试

CI runs all 9 playbook scripts in mock vault each release.

目标 ≥ 590（D26 后 575 + 15）。

## Acceptance Criteria

- [ ] README + 3 install docs + Quickstart + FAQ
- [ ] 9 playbook + auto-test
- [ ] docs/ link 完整不死链
- [ ] CI 5 OS 全绿；测试 ≥ 590

## Out of Scope

- 视频 tutorials
- 多语言文档（英文优先；中文 V0.2+）
- API reference docs（auto-gen from godoc）

## Decision (ADR-lite)

**Decision**：README 英文 + 中文（双语）；其他 docs 仅英文 V1（中文 V0.2）。
Playbook scripts 可 CI 跑（mock vault state injection）。
