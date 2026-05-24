# Journal - fengxd (Part 1)

> AI development session journal
> Started: 2026-05-22

---



## Session 1: W1 D1: CLI 框架与 wikimind init/status

**Date**: 2026-05-23
**Task**: W1 D1: CLI 框架与 wikimind init/status
**Branch**: `main`

### Summary

实现 wikimind CLI 的 cobra 骨架（init/status + 5 个 stub 子命令）；internal/vault 提供 Init/ReadStatus/FindRoot 与 ErrNonEmptyDirectory；internal/schema 通过 go:embed 嵌入 spec-v2/templates 的 7 个模板；init 自动 git init、非空目录拒绝；3 包单测覆盖、go build/vet/test 全绿、CI 5 OS 矩阵 + Python 全绿。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `d1a163e` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 2: W1 D2: 配置加载与跨平台路径规范化

**Date**: 2026-05-23
**Task**: W1 D2: 配置加载与跨平台路径规范化
**Branch**: `main`

### Summary

internal/vault 新增 config.go（BurntSushi/toml v1.4.0 + LoadConfig + cross-validate）与 path.go（NormalizePath / ResolveInVault / IsValidFilename），重构 D1 的 writeConfig / readSchemaVersion 用 toml；100+ 表驱动路径用例（ASCII / 中文 / 长路径 / 符号链接 / traversal / Windows 保留字）；wikimind status 输出 config 校验状态；go build/vet/test 全绿，CI 5 OS 矩阵通过。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `d8a8958` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 3: W1 D3: raw ingest + SQLite + goose migration 链路

**Date**: 2026-05-23
**Task**: W1 D3: raw ingest + SQLite + goose migration 链路
**Branch**: `main`

### Summary

internal/index 提供 Open/Close/BeginTx + goose v3 //go:embed migrations 自动 up + index.db.bak 预备份；migrations/0001 升级为完整 sources 表（按 architecture §4.2 + sha256/status 双索引），migrations/ 移到 internal/index/（go:embed 父目录限制）；internal/service.IngestFile 复制到 raw/inbox + 流式 sha256 (O(1) 内存) + UPSERT sources + 同 sha256 去重 + 同名不同内容自动 -<sha8> 后缀（保持 raw 不可变）；wikimind ingest 真实实现；4 个 sentinel errors；20 测试（index 7 / service 10 / cmd 3）全 PASS，CI 5 OS 矩阵通过。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `f7110ac` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: Fix: Windows path cross-validate case sensitivity（D2 fix-up）

**Date**: 2026-05-23
**Task**: Fix: Windows path cross-validate case sensitivity（D2 fix-up）
**Branch**: `main`

### Summary

修 D2 引入的 LoadConfig vault_root cross-validate 在 Windows NTFS 上失效（D2/D3 CI windows-2022 job 红）。两个 commit：(1) config.go pathsEqual helper Windows EqualFold；(2) config_test.go 把 strings.Replace 改成 toml round-trip 修测试构造在 Windows 反斜杠转义陷阱。新增 TestLoadConfigVaultRootCaseInsensitiveOnWindows（仅 Windows 跑）。CI 5 OS 矩阵 + python 这次真全绿。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `10d7800` | (see git log) |
| `cf76ab4` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: W1 D4: markdown frontmatter 解析 + pages 表 + page 命令组

**Date**: 2026-05-23
**Task**: W1 D4: markdown frontmatter 解析 + pages 表 + page 命令组
**Branch**: `main`

### Summary

migrations/0002 加 pages 表（含 body 列供 trigger 读）+ pages_fts(trigram) + INSERT/UPDATE/DELETE 同步 triggers；internal/index/pages.go 提供 UpsertPage(ON CONFLICT UPDATE 幂等) / ListPages(按 type 过滤) / GetPageByID；internal/service/page.go 用 yaml.v3 解 frontmatter + goldmark 抽 heading + 正则抽 outbound [[id]]（支持 alias + dedup），ReindexWiki 跳 _ 前缀目录、无 frontmatter 文件 type='unknown' 保留；cmd/wikimind page 子命令组 reindex/list/show；依赖 yaml.v3 + goldmark；39 测试全 PASS，CI 5 OS + python 真全绿。Gotcha：goose CREATE TRIGGER 需 +goose StatementBegin/StatementEnd 包，注释避免含 +goose 关键字（keyword-greedy 解析陷阱）。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `a9cd424` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 6: W1 D5: FTS5 trigram query 命令 + ripgrep 兜底

**Date**: 2026-05-24
**Task**: W1 D5: FTS5 trigram query 命令 + ripgrep 兜底
**Branch**: `main`

### Summary

实现 wikimind query：FTS5 trigram MATCH + BM25 + snippet，短查询(< 3 chars) LIKE fallback，--no-index/--regex 走 ripgrep，rg 缺失 silently 降级 LIKE。CLI flags --no-index/--regex/--limit/--json/--verbose。+14 测试覆盖 FTS5 / LIKE / ripgrep 路由 / Windows 盘符路径 / 空索引友好错误。trellis-check 顺便修正 escapeLikePattern 注释 + ripgrep --max-count 注释。CI 5 OS 全绿。W1 完成 5/7 天。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `bbba12c` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 7: W1 D6: change-log + git auto-commit + revert

**Date**: 2026-05-24
**Task**: W1 D6: change-log + git auto-commit + revert
**Branch**: `main`

### Summary

实现 internal/commit/ 包（change_log.go + git.go + commit.go）+ wikimind ingest 自动 git commit + wiki/log.md + .wikimind/change-log.jsonl + wikimind revert <seq>。+10 测试覆盖 NextSeq / Append / EnsureRepo / Commit / Revert / E2E (ingest + revert 链路)。sub-agent 越权 commit + 顺手加 quality-guidelines.md spec-entry（揭示 revert 非显见 trap：不能简单 git revert --no-edit 会双 commit，需 GitRevertNoCommit + commit.Commit 一起原子）。CI 5 OS 全绿，95 测试 PASS (+10)。W1 完成 6/7 天，仅剩 D7 demo + 跨平台验收。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `a31fa5b` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: W1 D7: demo + 跨平台验收 + W1 出口

**Date**: 2026-05-24
**Task**: W1 D7: demo + 跨平台验收 + W1 出口
**Branch**: `main`

### Summary

补 D3 遗留 gap: ingest 自动生成 wiki/sources/<id>.md (frontmatter title 三级 fallback, POSIX source_path, body 占位不复制 raw 全文)。ingest 后 auto reindex (--no-reindex 跳过, 失败 warning 不阻塞)。docs/demo/w1-walkthrough.md 完整步骤化文档 + D8+ teaser。3 demo smoke tests (含 CJK 端到端) + 8 source_page tests。trellis-check 实际跑 manual demo (init → CJK ingest → query → revert) 确认 walkthrough work。Sub-agent 严守 prompt 未越权。CI 5 OS 全绿，106 测试 (95 → 106, +11)。W1 出口 4 条达成: demo 跨平台 / ≥100 测试 / CI 全绿 / CJK 通过。W1 完成 7/7 天。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `ad69b40` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: W2 D8: MCP server stdio + 4 个只读 tool

**Date**: 2026-05-24
**Task**: W2 D8: MCP server stdio + 4 个只读 tool
**Branch**: `main`

### Summary

新建 internal/mcp/ 包 (server/tools/types/2 test)：modelcontextprotocol/go-sdk v1.6.1 NewServer + ToolHandlerFor 泛型 wrapHandler adapter，注册 4 个 read tool (wiki_info/read_page/read_raw/list_index)，全部 ReadOnlyHint=true。严格按 spec-v2/docs/mcp-tools.md §2-4 §7 schema。read_page by id/by path 两路 Frontmatter 统一。read_raw 双层 path traversal 防护 (prefix raw/ + ResolveInVault) + text/binary 嗅探。list_index total = 过滤后总数。CLI wikimind mcp serve 子命令 (stderr-only logging 不污染 protocol stream) + SIGINT/SIGTERM 优雅退出 + --vault flag。docs/demo/mcp-inspector.md 手动验收脚本。trellis-check 修 4 真问题 (ReadOnlyHint spec 漏 / Frontmatter 不一致 / 2 gofmt)。CI 5 OS 全绿, 129 测试 PASS (+23), race-clean。W2 第一公里打通: Claude Code 经 MCP 读 vault。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `13511a0` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 10: W2 D9: MCP read tools 第二批 (5 tool + stage-2 anchor parser)

**Date**: 2026-05-24
**Task**: W2 D9: MCP read tools 第二批 (5 tool + stage-2 anchor parser)
**Branch**: `main`

### Summary

用户直接实现: 5 个新 read tool (search / read_raw_anchor / read_claim / graph_neighbors / get_history) + internal/index/anchor.go stage-2 raw parser (heading slug + para-N + char[start:end] + QuoteHash sha256[:8]) + 9 tool 全 ReadOnlyHint=true。staged 占位 (claim sources / graph inbound / search min_confidence) 留 W2 D11+ propose_claim 表后真做。+62 行 quality-guidelines.md spec-entry。11 文件 1661 行 / 测试 129 → 145 (+16)。CI 5 OS 全绿。新工作流: D9 之后 main agent 只 create task + brainstorm + prd + curate jsonl, user 自实现。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `b74f63a` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete
