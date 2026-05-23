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
