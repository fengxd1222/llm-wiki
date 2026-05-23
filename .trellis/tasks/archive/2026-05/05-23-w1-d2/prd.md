# W1 D2: 配置加载与跨平台路径规范化

## Goal

在 D1 基础上加 (1) `.wikimind/config.toml` 读取与校验；(2) 跨平台路径规范化模块 +
path-traversal 防御；(3) 100 个路径用例的单测。为 D3+（ingest / SQLite）提供可信的
路径与配置基础。

需求来源：`spec-v2/docs/roadmap-30d.md` W1 D2 + `spec-v2/docs/cross-platform.md` §1
（文件名约定）/ §7（路径规范化）+ `spec-v2/docs/architecture.md` §4.1。

## What I already know

- D1 已落地（commit d1a163e）：`internal/vault.Init/ReadStatus/FindRoot/writeConfig/
  readSchemaVersion`、`internal/schema` embed 模板、cobra CLI + 5 stub
- `writeConfig` 已用 `strconv.Quote` 写 TOML 三字段（vault_root / schema_version /
  created_at），`readSchemaVersion` 已用 `strings.Cut` 手写读
- `cross-platform.md §7.3` 给了 normalize/traversal 的伪代码（Python 版，需翻 Go）
- `cross-platform.md §1.1-1.3` 文件名规则：`^[a-z0-9][a-z0-9-]*\.md$`
- engineering-decisions §4.1 的 14 包结构无 `internal/config` —— config 读写继续放
  `internal/vault`

## Requirements

- **Config 加载**（扩展 `internal/vault`）：
  - 引入 `github.com/BurntSushi/toml` v1
  - `LoadConfig(vaultRoot) (*Config, error)` 用 `toml.DecodeFile` 读
    `.wikimind/config.toml`
  - 重构 D1 `writeConfig` 用 `toml.NewEncoder().Encode`，保持 read/write 对称
  - 重构 D1 `readSchemaVersion` 改用 `LoadConfig` 后取字段（避免重复 parse）
  - 必填字段校验：`vault_root`（绝对路径、与实际位置一致）/ `schema_version`（"1.0"）
    / `created_at`
  - 字段缺失 / 类型错 / 文件不可读 → 清晰错误（按 `.trellis/spec/backend/error-handling.md`）
- **路径规范化**（`internal/vault` 新增）：
  - `NormalizePath(p string) string` —— 内部 POSIX `/`、`filepath.ToSlash`
  - `ResolveInVault(rel, vaultRoot string) (abs string, err error)` ——
    `filepath.Join` + `EvalSymlinks` + `startswith` vault root → 拒绝 `..` 越界 /
    符号链接逃逸
  - `IsValidFilename(name string) error` —— `^[a-z0-9][a-z0-9-]*\.md$` 校验 +
    Windows 保留字（CON/PRN/AUX/NUL/COM1-9/LPT1-9）拒绝
- **100+ 路径用例单测**（表驱动）：
  - ASCII 基本（10）/ 中文路径（10）/ 长路径（10）/ 符号链接（10）/
    Path traversal（15）/ 大小写漂移（10）/ Windows 保留字 + 非法字符（10）/
    空 / 根 / 相对（10）/ 双斜杠 / 尾斜杠（5）/ 跨平台分隔符（10）
- **`wikimind status` 升级**：输出 config 校验状态（OK / 错误细节）

## Acceptance Criteria

- [ ] `LoadConfig` 实现 + 必填校验 + cross-validate（vault_root ↔ 实际目录）
- [ ] `NormalizePath` / `ResolveInVault` / `IsValidFilename` 实现
- [ ] 拒绝：`..` 越界、符号链接逃逸 vault root、Windows 保留字、非法字符
- [ ] 100+ 路径单测（表驱动）全 pass（macOS + Linux 真测，Windows 路径用模拟）
- [ ] `wikimind status` 输出 config 校验状态
- [ ] config 文件缺失 / 字段缺失 / 类型错误 → 报清晰错误（错误信息含字段名）
- [ ] `go build` / `go vet` / `go test ./...` 全绿
- [ ] CI 5 OS 矩阵通过

## Definition of Done

- 单测覆盖 ≥ 100 用例（表驱动）
- lint / vet / CI 绿
- 遵循 `.trellis/spec/backend/` 的 error-handling / quality 规范
- commit 并 push（W1 D2 一个 commit）

## Out of Scope

- ingest / SQLite / FTS5（D3）
- Watcher 接入（D7 macOS / D18 Windows）
- daemon `wikimindd`（W2+）
- log.md / change-log / git auto-commit（D6）
- Unicode NFC 规范化 —— 文件名已限 ASCII，NFC 对文件名无意义；如未来 frontmatter 处理需要再加

## Decision (ADR-lite)

**Context**: D1 的 `readSchemaVersion` 手写 TOML 读够用，但 W1+ 配置字段会快速膨胀
（auto-accept rules / dream_cycle / lint / sediment 等）。
**Decision**: D2 引入 `github.com/BurntSushi/toml` v1（Go 社区标准、纯 Go、轻量）。
**Consequences**: 一次引入新依赖；后续加字段零成本；D1 手写的 `writeConfig` 与
`readSchemaVersion` 重构为 `toml.Marshal/Decode`，保持 read/write 对称。

## Technical Notes

- `filepath.EvalSymlinks` 是符号链接解析关键；用 `filepath.Clean` + `strings.HasPrefix`
  做 startswith 检查
- 100 用例用 Go idiomatic 表驱动测试
- Windows 路径在 macOS/Linux 上**测不到真 OS 行为**——用纯字符串测 `NormalizePath`
  逻辑（输入 `a\b\c` → 输出 `a/b/c`），符号链接等 OS 行为只在 macOS/Linux 测
- 错误类型：沿用 D1 的 sentinel error pattern（如 `ErrPathEscapeVault` /
  `ErrInvalidFilename` / `ErrConfigMissing`）
- `github.com/BurntSushi/toml` v1：Go 社区 TOML 库的事实标准，纯 Go 无 cgo
