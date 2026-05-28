# v0.1.0 Code Quality Audit Report

> **审查日期**：2026-05-27
> **审查者**：Claude (Opus 4.7) + fengxd
> **代码版本**：commit `3efa21f`（v0.1.0 已发布）
> **范围**：cmd/{wikimind,wikimindd} + internal/ 17 包 + verify/{fts5,ipc,mcp} + worker/ + go.mod
> **标尺**：`.trellis/spec/backend/` 5 份 spec + 7 个 `<spec-entry>` 场景契约
> **纪律**：全程只读，未修改任何源文件

---

## 🚨 紧急警报（P0 列表）

> P0 = 数据损坏 / 安全漏洞 / 跨平台不可用 / 协议契约破坏。必须 v0.1.1 patch。

**最终全量审查（cmd + 17 internal + verify + worker + go.mod 全覆盖）结果：无严格 P0**。

⚠️ **但有 1 个 W3+ 阶段会立即变 P0 的 P1**：
- **F-041 `internal/bridge` Windows 不可用**：`SocketPath` 返回 Named Pipe namespace 路径 `\\.\pipe\wikimind-<base>` 但 `Listen` 用 `net.Listen("unix", ...)` 监听该路径——必然失败。W2（无 daemon）阶段未爆雷；W3 起 daemon + IPC 上线后 Windows 用户 boot 即失败。**v0.1.1 必须修**。

横切扫描下列高风险维度均未发现 P0：
- ✅ 无业务代码 `panic` 调用（除 `init()` 注册驱动）
- ✅ 无 SQL 字符串拼接注入（`internal/index/page_links.go:146` 用 placeholder 拼接但参数化 + 输入是内部常量）
- ✅ 无 `unsafe` / `reflect` / `CGO` 使用
- ✅ 所有 goroutine 配 `wg.Add/Done` + 监听 `ctx.Done()` 退出（`internal/watcher`, `internal/daemon`）
  - ⚠️ **例外 F-042**：watcher.Close + AfterFunc 回调存在 send-on-closed-channel race，时窗短但 panic 风险存在。
- ✅ 文件 `os.Open` 全部跟 `defer Close()`（资源管理无明显泄漏）
- ✅ MCP 错误码字面量稳定（`SESSION_REQUIRED` / `AGENT_NOT_WHITELISTED` / `CROSS_SESSION_BUNDLE` 等 15+ 个均在 sentinel 中）
- ✅ 跨平台 main/master 分支已通过 `commit 55fc534` 修好（`ensureMainBranch` + `defaultBaseRef`）
- ✅ 并发包 `internal/{lock,daemon,watcher,worktree}` `go test -race` 全过
- ✅ 依赖安全离线核对（F-075）：12 个直接 + 间接关键依赖均在已公开 CVE 的修复线，**无已知 CVE 命中**（限：未联网查 OSV，建议 v0.1.1 加 govulncheck CI step）
- ✅ 4 空壳包（changelog / git / model / worker）确认完全无人 import（F-074），删除零编译风险

**全量审查完成，无新 P0 浮现**。

---

## 📊 严重度统计（最终态 · 全量审查完成）

| 严重度 | 数量 | ID 清单 |
|--------|------|---------|
| **P0** | **0** | 无 |
| **P1** | **24** | F-002 / F-003 / F-005 / F-007 / F-008 / F-009 / F-010 / F-012 / F-013 / F-025 / F-026 / F-027 / F-028 / F-029 / F-030 / F-031 / F-041 / F-042 / F-044 / F-047 / F-048 / F-049 / F-059 / **F-069** |
| **P2** | **48** | F-004 / F-006 / F-011 / F-014 / F-015 / F-016 / F-017 / F-018 / F-019 / F-020 / F-021 / F-022 / F-023 / F-024 / F-032 / F-033 / F-034 / F-035 / F-036 / F-037 / F-038 / F-039 / F-040 / F-043 / F-045 / F-046 / F-050 / F-051 / F-052 / F-053 / F-054 / F-055 / F-056 / F-057 / F-058 / **F-060 / F-061 / F-062 / F-063 / F-064 / F-065 / F-066 / F-067 / F-068 / F-070 / F-071 / F-072 / F-073** |
| **Spec-Drift** | **13** | F-S001 / F-S002（纯）+ F-014 / F-018 / F-025 / F-026 / F-027 / F-031 / F-034 / F-045 / F-048 / F-049 / F-056（双标签）|
| **FYI / 澄清** | **3** | F-001（部分目录无测试按设计）/ F-074（4 空壳包零导入证实）/ F-075（依赖安全离线核对，无 CVE 命中）|
| **总计** | **75 条目**（72 finding + 3 FYI）| F-001 / F-074 / F-075 是澄清/补强记录，不计入 P0/P1/P2 |

> 双标签说明：F-014 / F-018 / F-034 / F-045 / F-056 主标签是 P2 + Spec-Drift；F-025 / F-026 / F-027 / F-031 / F-048 / F-049 主标签是 P1 + Spec-Drift。Spec-Drift 列按双标签累计。本批新增条目 F-060 ~ F-075（14 finding + 2 FYI）。

### 维度分布

| Lens | P0 | P1 | P2 | Spec-Drift |
|------|----|----|----|-----------|
| 正确性 / 逻辑 | 0 | F-003, F-010, F-012, F-028, F-029, F-030, F-042, F-044, F-047, F-052 | F-033, F-035, F-043, F-046, F-053, F-058 | |
| 错误处理 | 0 | F-008, F-009, F-012, F-025, F-044, F-047 | F-004, F-011, F-016, F-017, F-023, F-024, F-032, F-036, F-050, F-053, F-054, F-055, F-058, F-060, F-064, F-066, F-072 | |
| 安全 | 0 | F-010, F-012 | F-018, F-040 | F-018 |
| 并发 | 0 | F-028, F-042 | F-043 | |
| 性能 | 0 | 0 | F-019, F-020 | |
| 契约稳定性 | 0 | F-007, F-010, F-025, F-026, F-027, F-031, F-048, F-049, F-069 | F-014, F-018, F-035, F-045, F-050, F-052, F-056, F-057, F-061, F-062, F-065, F-070, F-071 | F-S001, F-014, F-018, F-025, F-026, F-027, F-031, F-034, F-045, F-048, F-049, F-056 |
| 测试质量 | 0 | F-003, F-069 | F-021, F-022, F-054, F-063, F-064, F-065, F-066, F-073 | |
| 资源管理 | 0 | F-029, F-042, F-044 | F-043, F-046, F-060, F-063, F-068 | |
| 跨平台 | 0 | F-007, F-041 | F-055, F-068, F-072 | |
| 可维护性 | 0 | F-002, F-005, F-008, F-013, F-059 | F-006, F-014, F-015, F-021, F-024, F-034, F-037, F-038, F-039, F-045, F-051, F-055, F-056, F-057, F-061, F-062, F-067, F-070, F-073 | F-S002 |

✅ **全量覆盖完成**：cmd（wikimind + wikimindd）+ 17 个 internal 包（含 4 空壳）+ verify/{fts5,ipc,mcp} + worker/{main.py,pyproject.toml} + go.mod/go.sum 依赖核对。**未覆盖**项见附录 C（性能 benchmark / Windows CI 实机 / 联网 govulncheck）。

---

## 🎯 推荐结论（最终定稿）

**🟠 v0.1.1 patch（重点修复 P0 边缘的 P1 + 跨平台硬阻塞）+ v0.2 集中重构（剩余 P1 / P2 子群）+ 一次 trellis-update-spec 清 Spec-Drift**

### 结论一：v0.1.0 当前状态可冻结发布，但 v0.1.1 必修少量项

理由：
1. **零 P0**——v0.1.0 在数据安全、协议稳定字面量、并发死锁、资源管理、依赖 CVE（离线核对）五个"急性"维度全部通过。
2. **24 条 P1 中真正阻塞下个 milestone（W3 daemon 上线）的是 F-041**——Windows IPC 不可用。W2 用户不痛，W3 启动 daemon 后 Windows 用户 boot 即失败。**必须 v0.1.1 修**。
3. **其他 11-12 条 P1 属"行为偏离 spec 但不致命"**，可 v0.1.1 一并修（成本低、ROI 高），也可延 v0.2。
4. **48 条 P2** 多为命名 / 重复代码 / 错误处理统一化 / context 链路传递，**集中在 v0.2 重构期顺手清**。
5. **13 条 Spec-Drift（2 纯 + 11 双标签）** 由 `trellis-update-spec` 一次 spec-only PR 清，零代码改动。

### 结论二：v0.1.1 必修清单（高 ROI、低风险，建议合 1 PR 完成）

| ID | 一句话 | 严重度依据 | 估算 |
|----|--------|-----------|------|
| **F-041** | **Bridge SocketPath 在 Windows 改用 fs path + Listen 一致用 unix，加 Windows CI smoke** | 跨平台硬阻塞（W3 必需）| **~20 行 + CI** |
| **F-042** | **Watcher Close 加 `closed` 守 + 回调先 check 再 send** | 长跑 panic 风险 | **~15 行** |
| F-027 | tools.go:492 + tools_test.go:605 把 `append_log` 改 `log_append` | 协议契约破坏 | 2 行 |
| F-028 | `vaultBackend.lockManager()` 加 sync.Once 或 eager init | 并发 race | < 10 行 |
| F-029 | session 过期路径加 worktree 清理 | 资源泄漏 | ~20 行 |
| F-030 | daemon 启动注册 `SessionStore.Expire` 周期任务 | dead code 激活 | ~15 行 |
| F-009 | claim_sources.go 把 sql.ErrNoRows 转成包级 sentinel | 错误处理规范 | < 10 行 |
| F-010 | LIKE 模式拼接前对 `idempotency_key` 转义 `%/_/\\` | 幂等契约破坏 | < 15 行 |
| F-012 | `CheckQueueForPropose` 移除 fail-open 兜底 | 安全/正确性 | < 10 行 |
| F-025 | tools.go inline `errors.New` 改包级 sentinel | 协议契约可比对 | ~15 行 |
| F-048 | command.go:108 改成 `server.ToolCount()` 动态查询 | 契约字面量单源 | 2 行 + API 5 行 |
| F-049 | command.go:821 删 "(8 rules)" 字样或动态计算 | CLI 契约 | 2 行 |
| F-003 / F-069 | worker.py 顶部注释 + doctor 对 pypdf 改 warning + pyproject description 改"skeleton" | 用户契约/期望管理 | < 15 行 |

**额外建议（不属 finding 但配套）**：
- **govulncheck**：CI 加 `govulncheck ./...` 卡点（响应 F-075 离线核对的限制）。

合计 ≈ 150-180 行代码改动 + 1 个 CI step，单独 PR 可完成。

### 结论三：v0.2 重构清单（结构性 / 跨包改动）

- **F-002 + F-074 + F-S002**：删 4 空壳包 → 同步 directory-structure.md（spec PR）。
- **F-008 + F-014 + F-034 + F-039**：4 处 git 子进程包装收归到统一包；mcp/tools.go 按工具组拆分。
- **F-005 + F-059**：cmd/wikimind/command.go（892 行）拆为 init.go / status.go / ingest.go / ... 子文件；超长 RunE 提取到 service 层。
- **F-013 + F-038**：service.AcceptReview（120 行）/ handleProposeClaim（87 行）拆。
- **F-031**：RateLimits 真正落地（在 daemon 上线前完成）。
- **F-037**：5 个 propose tool 公共 helper 提取（`proposeWithIdempotency`）。
- **F-044**：watcher 暴露 Errors() 通道 + backpressure 替代 drop（W3 demand）。
- **F-047**：lint Rule.Run 改返回 error（全套规则签名调整 + RunRules 聚合）。

### 结论四：Spec-Drift 一次 spec-only PR 清（trellis-update-spec）

详见"📐 Spec-Drift 汇总"节。13 条全部由 spec 修订处理，零代码改动。

### 结论五：P2 子群集中清

48 条 P2 多数是命名 / 重复代码 / 错误处理统一化 / context.Background 改 cmd.Context / err == ErrXxx 改 errors.Is，v0.2 重构时顺手清；不专门开 patch。特别值得集中清的子群：
- **错误断言风格统一**：F-004 / F-022 / F-054（三处 test 用 `==` 比较 sentinel）。
- **CLI 输出 stderr 规范**：F-050 / F-053 / F-055（warning 应走 stderr）。
- **context 链路**：F-051（review.go 5 处 context.Background）/ F-015 / F-032（_ = ctx）。
- **吞错风险**：F-016 / F-017 / F-023 / F-053 / F-055 / F-058 / F-064 / F-066（批扫 / image meta / accept 后置）。
- **verify/ 改造**：F-063 / F-064 / F-065 / F-066 / F-067 / F-068（统一 `main → run() int` pattern + 错误检查）。
- **worker/ Python 工程化**：F-069 / F-070 / F-071 / F-072 / F-073（pyproject 字段补齐 + KeyboardInterrupt + mypy/pytest 引入）。

### 决策建议（一句话）

> **v0.1.0 状态健康可冻结**；开一个"v0.1.1 patch + CI 加固" PR 处理 F-041/F-042 等 13 条 P1 + govulncheck；同步开一个 spec-only PR 清 13 条 Spec-Drift；剩余 P1/P2 排入 v0.2 重构窗口。

---

## 📋 工具层基线（阶段 1 预扫）

| 工具 | 结果 |
|------|------|
| `go build ./...` | ✅ 全绿 |
| `go vet ./...` | ✅ 全绿 |
| `go test ./...` | ✅ 全绿（17 个 internal 测试包通过；4 个空壳包 + cmd/wikimindd + 3 个 verify 无测试文件，详见 F-001） |
| `go test -race` 并发包（lock/daemon/watcher/worktree）| ✅ 全绿 |
| 全仓 race 测试 | _建议 v0.1.1：`go test -race ./...`_ 全量执行 |
| 依赖漏洞扫描（离线） | ✅ 完成（F-075）—— 12 个直接 + 间接关键依赖均在已公开 CVE 修复线，**无已知 CVE 命中** |
| 依赖漏洞扫描（联网） | _建议 v0.1.1：CI 加 `govulncheck ./...`_ |

---

## 📦 代码量盘点

| 模块 | 实现行数 | 测试行数 | 测试比 | 备注 |
|------|---------|---------|--------|------|
| cmd/wikimind | 1448 | 886 | 0.61 | command.go 892 行偏大 |
| cmd/wikimindd | 38 | 0 | — | daemon 入口；版本号 `0.1.0-dev` |
| internal/bridge | 197 | 112 | 0.57 | |
| internal/changelog | **2 (空壳)** | 0 | — | F-002 |
| internal/commit | 1015 | 363 | 0.36 | |
| internal/daemon | 224 | 74 | 0.33 | |
| internal/git | **2 (空壳)** | 0 | — | F-002 |
| internal/index | 2915 | 1288 | 0.44 | |
| internal/lint | 408 | 163 | 0.40 | |
| internal/lock | 356 | 162 | 0.46 | |
| internal/mcp | 4164 | 1821 | 0.44 | 最大包 |
| internal/model | **2 (空壳)** | 0 | — | F-002 |
| internal/proposal | 561 | 208 | 0.37 | |
| internal/schema | 73 | 36 | 0.49 | |
| internal/service | 3600 | 1637 | 0.45 | |
| internal/vault | 1504 | 844 | 0.56 | |
| internal/watcher | 246 | 119 | 0.48 | |
| internal/worker | **2 (空壳)** | 0 | — | F-002 |
| internal/worktree | 429 | 162 | 0.38 | |
| verify/fts5 | 77 | 0 | — | go run 验证脚本，非单测 |
| verify/ipc | 108 | 0 | — | 同上 |
| verify/mcp | 72 | 0 | — | 同上 |
| worker (Python) | 37 | 0 | — | 待审 |

---

## 🔍 Findings

> 排序：严重度（P0 → P1 → P2 → Spec-Drift）→ 维度 → file:line。

### F-001 · 部分目录无测试文件（按设计）·【FYI / 非问题】

- **维度**：测试质量
- **严重度**：—（非 finding，澄清记录）
- **位置**：`cmd/wikimindd/`、`verify/{fts5,ipc,mcp}/`
- **说明**：
  - `cmd/wikimindd/main.go` 是 daemon 入口，本身只做参数解析 + signal 处理 + 委托给 `internal/daemon`；后者已有 `daemon_test.go`（74 行）。入口本身无测试可接受。
  - `verify/{fts5,ipc,mcp}/main.go` 是独立 `main` 程序（手工跑 `go run`），不是 `_test.go`，按 PRD 设计就是"端到端验证脚本"形态，不计入测试覆盖。
- **结论**：不构成 finding。

---

### F-002 · 4 个空壳 internal 包（仅 doc.go，无实现）·【P1 / Spec-Drift】

- **维度**：可维护性 + Spec-Drift
- **严重度**：P1（可延 v0.2）
- **位置**：
  - `internal/changelog/doc.go:1`
  - `internal/git/doc.go:1`
  - `internal/model/doc.go:1`
  - `internal/worker/doc.go:1`
- **证据**：

  ```
  $ wc -l internal/{changelog,git,model,worker}/*.go
        2 internal/changelog/doc.go     # "实现 change-log.jsonl 与 log.md 的写入"
        2 internal/git/doc.go           # "封装 git worktree、commit、revert"
        2 internal/model/doc.go         # "定义 Claim / Page / Review / Bundle / Source"
        2 internal/worker/doc.go        # "实现 Python ingest worker 的 Go 侧调度"
  ```

- **问题**：每个包的 `doc.go` 声称提供某项能力，但实际目录里**没有任何实现文件**。
  实际代码分布在别处：
  - changelog 实际在 `internal/commit/change_log.go`、`internal/commit/commit.go`
  - git 实际在 `internal/commit/git.go`、`internal/worktree/*.go`
  - model 实际散落在各包（如 `internal/commit/change_log.go` 的 `LogEntry`、`internal/mcp/types.go` 等）
  - worker 实际在 `internal/service/ingest_image.go` + `worker/main.py`
- **违反**：`.trellis/spec/backend/quality-guidelines.md` 要求 "每个 internal/<pkg> 都有 doc.go，**写明职责 + 当前 D 阶段能力清单**"。当前 doc.go 描述了**该包不实现**的能力，对阅读者构成误导。
- **修复方向**（不在本任务执行）：
  - **方案 A**：删除 4 个空壳包，把 doc.go 中的反向链接合并进真正实现的包注释。
  - **方案 B**：把 `internal/commit/change_log.go` 等移回 changelog 包（重构成本高，需评估对 import 链的冲击）。
  - 推荐方案 A：现状重构成本太高且已发布；spec 层面的"包职责"在 directory-structure.md 也需要相应更新。
- **是否打 Spec-Drift 标签**：是 —— `.trellis/spec/backend/directory-structure.md` "Directory Layout" 章节描述了 changelog/git/model/worker 各自的职责，需配合调整。

---

---

### F-003 · Python worker 实际仍是 W0 skeleton，但 D13 已归档·【P1】

- **维度**：正确性 + 测试质量（Roadmap 偏差）
- **严重度**：P1（可延 v0.2；但用户感知层面 doctor 命令在误导）
- **位置**：
  - `worker/main.py:1-37`（整文件，doc string 自承"W0 skeleton"）
  - `cmd/wikimind/command.go:689`（doctor 检查 pypdf 是否安装）
  - `.trellis/tasks/archive/2026-05/05-24-w2-d13-pdf-image-watcher/prd.md:205`（D13 acceptance 含 `worker/main.py 真实 PDF 解析(pypdf)` 未勾选）
- **证据**：

  ```python
  """WikiMind ingest worker — W0 skeleton.
  ...
  完整 parser（markdown / html / pdf / image / audio）在 roadmap D13 实现。
  """
  ```

  ```
  $ grep -rn "worker/main.py" internal/ cmd/ --include="*.go"
  (no matches — Go 侧无人调用)
  ```

  `internal/service/ingest_image.go:60` 行的实际图像 metadata 提取用 Go 标准库 `image.DecodeConfig`，与 worker 无关。

- **问题链**：
  1. doctor 命令检查 `python3` + `pypdf`，给用户"系统已支持 PDF 解析"的假象。
  2. 但 `worker/main.py` 仍是 skeleton（只回 `progress: 100% skeleton` + 空 `normalized`），没有 pypdf import，没有任何 parser 实现。
  3. Go 侧没有任何代码 exec worker/main.py（grep 全仓无匹配）。
  4. 结果：用户 ingest 一个 PDF 时，`IngestFile` 接受了 `.pdf` 扩展名（`supportedRawFormats[".pdf"]=true`），但不会真正解析 —— 仅记录 source 行。
- **违反**：D13 的 acceptance criteria 未达成却归档；doctor 输出与实际能力不符 = 用户契约违背。
- **修复方向**（不在本任务）：
  - 短期：worker/main.py 顶部 doc string 改为"W0 skeleton（待 D13/v0.2 完善）"，doctor 对 pypdf 改 warning 而非 ✓；或彻底从 doctor 移除 pypdf 检查。
  - 中期：v0.2 真正实现 pypdf + 图像 OCR worker；或决定放弃 PDF 真解析，仅做 metadata 索引。

---

### F-004 · `sql.ErrNoRows` 用 `==` 比较，应改 `errors.Is`·【P2】

- **维度**：错误处理
- **严重度**：P2（FYI；当前 wrap 链短未触发，但防御性写法应统一）
- **位置**：
  - `internal/index/page_links.go:152`
  - `internal/service/wiki_index.go:138`
- **证据**：

  ```go
  // internal/index/page_links.go:152
  if err := db.SQL().QueryRowContext(ctx, q, args...).Scan(&count); err != nil {
      if err == sql.ErrNoRows {   // ❌
          return 0, nil
      }
      return 0, fmt.Errorf("count orphan pages: %w", err)
  }
  ```

- **违反**：`.trellis/spec/backend/error-handling.md` "比较错误：永远 `errors.Is` / `errors.As`" 与 "未找到行返回包级 sentinel，不暴露 `sql.ErrNoRows`"。
- **修复方向**：改 `errors.Is(err, sql.ErrNoRows)`；进阶则把 `sql.ErrNoRows` 转译成包级 sentinel（如 `ErrPageNotFound`），不向 service 漏。

---

### F-005 · `cmd/wikimind/command.go` 单文件 892 行·【P1】

- **维度**：可维护性
- **严重度**：P1（可延 v0.2）
- **位置**：`cmd/wikimind/command.go`（892 行包含所有子命令：init / status / ingest / revert / mcp serve / doctor / log / lint / dream / package install / watch root...）
- **违反**：`.trellis/spec/backend/quality-guidelines.md` "函数过长（> 80 行） / 复杂度 / 重复代码"暗示按职责切分；同包内已示范了 `demo.go` / `review.go` / `watch.go` 按子命令切。
- **修复方向**：按子命令拆为 `init.go` / `status.go` / `ingest.go` / `revert.go` / `mcp.go` / `doctor.go` / `log.go` / `lint.go` / `dream.go` / `package.go`。函数名保持，仅移动位置；测试不动。

---

### F-006 · `internal/mcp` 单包 4164 行·【P2 观察】

- **维度**：可维护性
- **严重度**：P2（FYI；mcp 是协议层天然集中，可接受）
- **位置**：`internal/mcp/{server,session,tools,types}.go`，其中 `tools.go` 占大头（约 1500+ 行）。
- **说明**：mcp 包按"协议聚合层"职责存在，4 个文件分了 server / session / tools / types。tools.go 单文件可考虑按工具组拆分（read tools / write tools / handshake / log_append）但非必须。
- **修复方向**：v0.2 若新增 MCP 工具组，按 `tools_<group>.go` 拆分而不是继续扩 tools.go。

---

### F-S001 · Spec-Drift · daemon + `mcp serve` 已在 v0.1.0 使用 `log` 包

- **维度**：契约稳定性 / Spec-Drift
- **位置**：
  - `internal/daemon/loop.go:6,29,65`（daemon 内置 `*log.Logger`）
  - `cmd/wikimind/command.go:10,87`（mcp serve 入口 `log.New(stderr, ...)`）
- **现状**：daemon 与 mcp serve 已经在用 `log.Logger`（带前缀 + 时间戳）。
- **spec 描述**：`.trellis/spec/backend/logging-guidelines.md` 写道"**不使用** `log` / `slog` ..." 后又补"当某天确实需要结构化 daemon 日志（W2+ daemon 长期运行）时**再引入** `log/slog`"。
- **冲突点**：spec 用"将来时"描述这件事，但 v0.1.0 已经在用 —— 字面上仍可解释（W2 D8 daemon 上线时引入），但读者第一眼读 spec 容易判断成"不该用"。
- **代码 vs spec 谁更对**：**代码更对**。daemon + mcp serve 长期运行场景用 `log.Logger` 走 stderr 是合理的；spec 应该明确"已采用"。
- **跟进**：trellis-update-spec 将 logging-guidelines 中"将来时"改为"现状描述"——daemon / mcp serve 使用 `log` 包写 stderr 是已生效的约定。

---

### F-S002 · Spec-Drift · 4 个空壳包与 directory-structure.md 描述对应

- **维度**：契约稳定性 / Spec-Drift
- **位置**：与 F-002 关联——`.trellis/spec/backend/directory-structure.md` 中 "Directory Layout" 章节列出了 `changelog/`、`git/`、`model/`、`worker/` 包，并标注了职责。
- **现状**：这 4 个目录里都是空壳。
- **跟进**：与 F-002 一并处理；spec 更新方向取决于 F-002 选哪个修复方案（A 删空壳→ spec 同步删；B 重构回 → spec 不动）。

---

---

### F-007 · `proposal.ValidatePath` 未校验文件名 kebab-case 规则·【P1】

- **维度**：契约稳定性 + 跨平台
- **严重度**：P1（可延 v0.2；当前 Agent 都符合规范、未触发）
- **位置**：`internal/proposal/validator.go:48-66`
- **证据**：

  ```go
  func ValidatePath(path, pageType string) error {
      normalized := vault.NormalizePath(path)
      if normalized == "." || strings.HasPrefix(normalized, "../") ||
          strings.HasPrefix(normalized, "/") || !strings.HasSuffix(normalized, ".md") {
          return fmt.Errorf("%w: %s", ErrPathNotAllowed, path)
      }
      // ... 仅做 type 与 prefix 比对，未校验文件名本身
      return nil
  }
  ```

  而 `internal/vault/path.go:207` 已存在 `vault.IsValidFilename(name)`，包括 kebab-case 正则 + Windows 保留名（CON/PRN/AUX/NUL/COM1-9/LPT1-9）拒绝。
- **问题**：Agent 通过 MCP `propose_page` / `propose_claim` 提交 `wiki/claims/Foo Bar.md` 这种含大写/空格/中文/Windows 保留名的文件，`ValidatePath` 会接受；但写到 Windows 时会失败，破坏跨平台保证（cross-platform.md §1.1）。
- **违反**：`.trellis/spec/backend/quality-guidelines.md` 中 W2 D11 `<spec-entry>` 的 "Validation & Error Matrix" 隐含 path 校验完整性；`.trellis/spec/backend/directory-structure.md` "Naming Conventions" + cross-platform.md §1.1。
- **修复方向**：在 `ValidatePath` 内对 `filepath.Base(normalized)` 调用 `vault.IsValidFilename`，错误转译为 `ErrPathNotAllowed`。新增对应单测覆盖（含空格 / 大写 / CJK / Windows 保留名）。

---

### F-008 · `internal/vault` 与 `internal/commit` 重复实现 git 子进程包装·【P1】

- **维度**：可维护性 + 错误处理
- **严重度**：P1（可延 v0.2）
- **位置**：
  - `internal/vault/vault.go:256-270`（无 ctx 版 `runGit`）
  - `internal/commit/git.go:281-310`（有 ctx 版 `runGit`，更完善）
- **差异**：

  | 项 | vault.runGit | commit.runGit |
  |----|--------------|---------------|
  | 接受 ctx | ❌（无法 Ctrl+C 中断） | ✅ `exec.CommandContext(ctx, ...)` |
  | vault root 校验 | ❌ | ✅（line 282-287，空/非目录拒绝） |
  | 工作目录设置 | `-C root` 参数 | `cmd.Dir = root` |
  | 错误格式 | `errors.New(stderr)` | wrap + sentinel-aware |

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "所有 I/O 函数第一参数 `ctx context.Context`"——vault.runGit 没有 ctx，链上的 `vault.Init` / `vault.ReadStatus` 也都无法取消。
  - directory-structure.md "Naming Conventions" 隐含的"不要在两个包里实现同一件事"——这也呼应 F-002 中的 `internal/git/` 空壳：当初规划是有 `internal/git` 包统一收容，落地时分散了。
- **修复方向**：
  - 短期：把 vault 包改为依赖 commit 包的 `runGit`（注意循环依赖：vault 是底层，应反向把 git 工具提到 `internal/git` 实质化）。
  - 中期：F-002 修复方案 A（删空壳）若选 A，则把两份 runGit 统一到一个底层包；若选 B（恢复 `internal/git`），则两个调用方都依赖它。

---

---

### F-009 · `internal/index/claim_sources.go` 把 `sql.ErrNoRows` 直接暴露给上层·【P1】

- **维度**：错误处理
- **严重度**：P1（实现细节泄漏；调用方需要 `errors.Is(err, sql.ErrNoRows)` 才能判断"未命中"，违反错误处理 spec "未找到行返回包级 sentinel，不暴露 `sql.ErrNoRows`"）
- **位置**：`internal/index/claim_sources.go:106-110`
- **证据**：

  ```go
  // UpdateClaimSourceStatus
  res, err := db.SQL().ExecContext(ctx, q, status, verifiedAt, claimID, rawID, anchor)
  if err != nil {
      return fmt.Errorf("update claim source status: %w", err)
  }
  n, _ := res.RowsAffected()    // ❌ 静默吞掉错误
  if n == 0 {
      return sql.ErrNoRows      // ❌ 直接返回 std 错误
  }
  ```

- **违反**：
  - `.trellis/spec/backend/error-handling.md` "未找到行返回包级 sentinel，不暴露 `sql.ErrNoRows`"
  - `.trellis/spec/backend/error-handling.md` "不要静默吞错"
- **修复方向**：
  - 新建 `ErrClaimSourceNotFound = errors.New("claim source not found")`，返回该 sentinel。
  - `res.RowsAffected()` 的错误用 `fmt.Errorf("update claim source rows affected: %w", err)` 包裹。
  - 同包同模式：参考 `internal/index/reviews.go:165-184` 的 `UpdateReviewStatus` 已经做对了。

---

### F-010 · LIKE 模式拼接 user-controlled `idempotency_key`，未转义元字符·【P1】

- **维度**：安全 + 正确性 + 契约稳定性
- **严重度**：P1（不是 SQL 注入——`key` 经 `?` 参数化绑定；但 LIKE 元字符 `%` `_` 未转义，agent 提交 `idempotency_key="%"` 会匹配任意 review 行，返回错误的 `review_id` 给 agent，破坏幂等契约）
- **位置**：`internal/index/reviews.go:97-100`
- **证据**：

  ```go
  // FindReviewByIdempotencyKey
  key = strings.TrimSpace(key)
  if key == "" {
      return nil, nil
  }
  pattern := `%"idempotency_key":"` + key + `"%`   // ❌ key 未 escape LIKE 元字符
  row := db.SQL().QueryRowContext(ctx,
      reviewSelectSQL+` WHERE agent = ? AND meta_json LIKE ? ORDER BY seq LIMIT 1`,
      agent, pattern)
  ```

  对照 `internal/index/search.go:160-167` 已经实现了 `escapeLikePattern` 并配 `ESCAPE '\\'`——同仓内有正确写法可参考。
- **违反**：
  - `.trellis/spec/backend/database-guidelines.md` "FTS5 查询例外允许构造 `MATCH` 表达式，但用户输入仍必须经转义函数处理，不直接拼接"——同精神适用于 LIKE。
  - `.trellis/spec/backend/quality-guidelines.md` W2 D11 spec-entry "Idempotency is scoped to `(agent, idempotency_key)` and returns the existing review before touching the worktree"——错误匹配破坏幂等承诺。
  - CWE-89 LIKE wildcard injection。
- **修复方向**：
  - 用 `escapeLikePattern(key)` 转义后再拼接，SQL 加 `ESCAPE '\\'`。
  - 更优解：把 `idempotency_key` 提升为 `reviews` 表的 first-class 字段（独立 UNIQUE 列），下次 migration 加上；LIKE-on-JSON 是临时方案。

---

### F-011 · `index.FindSourceBySHA256` / `FindSourceByRawID` 用 `(nil, nil)` 表示未命中·【P2】

- **维度**：错误处理
- **严重度**：P2（API 不一致——同包内 `GetReviewByID` 返回 `ErrReviewNotFound`，`GetBundleByID` 返回 `ErrBundleNotFound`，但 sources 的两个查询返回 `(nil, nil)`，调用方必须先判 nil 再用，容易漏判）
- **位置**：
  - `internal/index/sources.go:28-45`（`FindSourceBySHA256`）
  - `internal/index/sources.go:47-65`（`FindSourceByRawID`）
- **证据**：

  ```go
  // FindSourceBySHA256 按 sha256 命中已入仓的 source；未命中返回 (nil, nil)。
  func FindSourceBySHA256(...) (*SourceRow, error) {
      ...
      if errors.Is(err, sql.ErrNoRows) {
          return nil, nil    // ❌ 调用方无 sentinel 可比对
      }
      ...
  }
  ```

  调用方 `internal/service/ingest.go:77-87` 必须 `if existing != nil` 才能分支去重；缺一行就 NPE。
- **违反**：`.trellis/spec/backend/error-handling.md` "未找到行返回包级 sentinel"——同包不一致是更高维度的可维护性问题。
- **修复方向**：
  - 新建 `ErrSourceNotFound = errors.New("source not found")`，二者返回它。
  - 调用方改 `if errors.Is(err, ErrSourceNotFound)` 走去重路径。
  - 注意：这是 API 行为变更，需要审一下所有调用点（grep `FindSourceBy`）。

---

### F-012 · `service.queue.CheckQueueForPropose` "fail open" 静默吞错·【P1】

- **维度**：安全 + 正确性 + 错误处理
- **严重度**：P1（DB 不可用时该函数返回 nil，agent propose 闸门完全失效；W2 D11 spec-entry 中"queue at critical limit"的保护被静默绕过）
- **位置**：`internal/service/queue.go:74-86`
- **证据**：

  ```go
  func CheckQueueForPropose(ctx context.Context, db *index.DB, limits QueueLimits) error {
      state, err := GetQueueState(ctx, db, limits)
      if err != nil {
          return nil // fail open    // ❌ DB 故障 → propose 闸门绕过
      }
      ...
  }
  ```

  同一文件 line 55-58 也吞 backlog 计数错（`backlog = 0 // non-fatal`），但那条仅影响展示数字，不绕过闸门。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "不要静默吞错"。
  - `.trellis/spec/backend/quality-guidelines.md` W2 D11 spec-entry "Pending reviews `>= 50` -> handshake accepted but `queue_state.can_propose=false`"——这是协议保证，不该 fail open。
- **修复方向**：
  - 改 "fail closed"：DB 错误时返回 wrap 错，让 propose 调用方决定是否 abort（多半应 abort）。
  - 或者至少把 backlog 错误也透传，由调用方权衡。

---

### F-013 · `service.AcceptReview` 单函数 120 行做 13 步·【P1】

- **维度**：可维护性
- **严重度**：P1（远超 spec "函数过长（>80 行）"门槛；step 1~13 在同一函数里，注释驱动；rollback 路径分散，错误一个 step 漏挂 rollback 就会留下半成品 git 状态）
- **位置**：`internal/service/review.go:40-164`
- **证据**：
  - `awk` 测算 `AcceptReview` 函数 120 行。
  - Step 6/7（apply check / apply）、Step 8（post-apply validate）、Step 9（commit）三段失败都有自己的 rollback (`gitResetHard`)，但 step 9 的 rollback 在 commit 失败后才执行，commit 已经把 staged 内容尝试入库——靠 commit 内部失败时清空 staged。该耦合脆弱。
- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "函数过长（> 80 行） / 复杂度"。
  - `.trellis/spec/backend/quality-guidelines.md` "Composition over inheritance（同精神：拆步骤）"。
- **修复方向**：
  - 拆为 `validatePending` / `readPatch` / `applyAndValidate` / `commitAccepted` / `incrementalReindex` 5 个 helper，主函数变 30~40 行的协调器。
  - 把 rollback 集中成 `defer rollbackIfErr(&err, root)` 单点，避免分散 `_ = gitResetHard(...)`。

---

### F-014 · `service/review.go` 绕过 `internal/commit` 直接 `exec.Command("git", ...)`·【P2 / Spec-Drift】

- **维度**：可维护性 + 契约稳定性
- **严重度**：P2（不是写闸门违规——`git apply --check` 不动 commit，`git reset --hard HEAD` 是 rollback；但与 F-008 同精神：git 子进程封装分散在第 3 个包，难以统一加 ctx / 默认 identity / cross-platform fallback）
- **位置**：
  - `internal/service/review.go:215`（`gitApplyCheck`：`exec.CommandContext(ctx, "git", "apply", "--check", ...)`）
  - `internal/service/review.go:226`（`gitResetHard`：`exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")`）
- **证据**：

  ```go
  func gitApplyCheck(ctx context.Context, root string, patch []byte) error {
      cmd := exec.CommandContext(ctx, "git", "apply", "--check", "--whitespace=nowarn", "-")
      cmd.Dir = root
      cmd.Stdin = strings.NewReader(string(patch))
      out, err := cmd.CombinedOutput()
      ...
  }
  ```

  对比 `internal/commit/git.go:281-310` 的 `runGit`：有 ctx、有 vault root 校验、有 sentinel-aware 错误格式、有默认 identity 注入。这两处都没有。
- **违反**：F-008 已记录的同根问题——`internal/git` 包是空壳（F-002），git 子进程封装散落在 vault / commit / service / proposal 四处。
- **修复方向**：
  - 短期：把 `gitApplyCheck` / `gitResetHard` 移进 `internal/commit/git.go` 用同包的 `runGit` 调用（注意 stdin 注入需扩展 runGit 签名或加 `runGitWithStdin`）。
  - 中期：和 F-002 + F-008 一起做——把所有 git 调用收容到 `internal/commit` 或重建 `internal/git`。

---

### F-015 · `service.RecordRejection` / `LoadRecentRejections` 接受 `ctx` 但 `_ = ctx`·【P2】

- **维度**：可维护性 + 错误处理
- **严重度**：P2（ctx 在签名里就该被传递——`os.OpenFile` / `os.ReadFile` 没有 ctx 版本，但 spec 要求 I/O 函数第一参数 `ctx context.Context`，签名兑现内部却忽略，未来上层 ctx cancel 时这条路径无法响应）
- **位置**：`internal/service/rejections.go:26-49` 与 `52-87`
- **证据**：

  ```go
  func RecordRejection(ctx context.Context, vaultRoot string, r Rejection) error {
      _ = ctx                       // ❌ 显式吞 ctx
      if r.TS == "" {
          r.TS = time.Now().UTC().Format(time.RFC3339)
      }
      ...
  }
  ```

- **违反**：`.trellis/spec/backend/quality-guidelines.md` "所有 I/O 函数第一参数 `ctx context.Context`"——签名兑现内部不传是隐性违约。
- **修复方向**：
  - 用 `ctx` 做 select-with-default 的快速取消检查（开头 `if err := ctx.Err(); err != nil { return err }`）。
  - 或文件读写改成支持取消的写法（spawn goroutine + ctx done）——但 W1 简单场景下 ctx 检查就够。
  - 也可考虑：如果确实不需要 ctx，从签名移除（但和 spec 冲突，仍建议保留）。

---

### F-016 · `service.health.ComputeHealth` 静默吞 `CountDriftClaims` 错误·【P2】

- **维度**：错误处理
- **严重度**：P2（DB 故障时 health 显示 `drift_claims: 0`，让用户/Agent 误以为系统干净；不是 P1 因为不阻塞流程）
- **位置**：`internal/service/health.go:34-37`
- **证据**：

  ```go
  driftCount, driftErr := index.CountDriftClaims(ctx, db)
  if driftErr == nil {            // ❌ err != nil 时静默走过
      h.DriftClaims = driftCount
  }
  ```

- **违反**：`.trellis/spec/backend/error-handling.md` "不要静默吞错"。
- **修复方向**：要么 wrap 返回上层（与 OrphanPages 行为一致），要么至少把错误记入返回结构 `HealthScore.Warnings []string`（透明降级）。

---

### F-017 · `service.drift.ScanAllClaims` 批量 silently swallow 错误·【P2】

- **维度**：错误处理
- **严重度**：P2（批扫语义下"单条失败继续扫"可接受，但完全无计数、无日志输出，难以察觉系统性 drift；不是 P1 是因为不破坏数据）
- **位置**：`internal/service/drift.go:66-86`
- **证据**：

  ```go
  for _, page := range pages {
      sources, err := index.ListClaimSources(ctx, db, page.ID)
      if err != nil {
          continue              // ❌ 无计数、无日志
      }
      for _, src := range sources {
          status, verifyErr := VerifyClaimSource(ctx, vaultRoot, src)
          if verifyErr != nil {
              continue          // ❌ 同上
          }
          _ = index.UpdateClaimSourceStatus(...)  // ❌ 又一处
      }
  }
  ```

- **违反**：`.trellis/spec/backend/error-handling.md` "不要静默吞错"——批扫语义允许 best-effort 但应至少计数。
- **修复方向**：
  - 返回结构改 `(driftCount int, scanReport *ScanReport, err error)`，`ScanReport` 含 `Errors []error` / `SkippedPages int`。
  - 或最少把错误数累加进 return 的额外字段，让 CLI/Agent 知道扫过的覆盖率。

---

### F-018 · `index.search.SearchFTS5` 未对 user 输入做 FTS5 元字符处理·【P2 / Spec-Drift】

- **维度**：安全 + 契约稳定性 + Spec-Drift
- **严重度**：P2（不是 SQL 注入——`?` 已参数化；但用户输入直接进 FTS5 MATCH 语法，含 `"` / `*` / `^` / `OR` / `NEAR` 等会被解释为算子；malformed query 触发 FTS5 parse error 返回给用户）
- **位置**：`internal/index/search.go:44-86`（整个 `SearchFTS5`）
- **证据**：

  ```go
  // 用户输入 q 不经任何 sanitize，直接喂给 MATCH
  rows, err := db.SQL().QueryContext(ctx, sqlText, q, limit)
  ```

  对照 spec：
  > FTS5 查询例外允许构造 `MATCH` 表达式，但用户输入仍必须经转义函数处理，不直接拼接。
  > —— `.trellis/spec/backend/database-guidelines.md` "参数化绑定" 段落

- **是否 Spec-Drift**：可两面解读——
  - **代码角度**：用 `?` 参数化已不是 SQL 注入；FTS5 query syntax 是有意暴露给高级用户。
  - **Spec 角度**：spec 用"仍必须经转义函数处理"是绝对句式，与现状有 gap。
  - 当前实现倾向"故意暴露 FTS5 query 语法给高级用户"，但**缺少配套的"sanitize 入口"**——服务层 `service.Search` 也只 trim 不做 quote 转义。
- **违反**：spec letter 与代码现状的字面差异（drift）。
- **修复方向**：
  - 选 A：实现 `sanitizeFTS5Query(q string) string` 把 `"` 包成 phrase（最常见用法），spec letter 满足。
  - 选 B：spec 改成"允许直接传入 FTS5 query 语法，错误返回原始 parser 错"——把现状合法化。
  - 推荐 A 用于普通 query；保留 `--raw-fts` flag 给高级用户传原始语法。

---

### F-019 · `index.anchor.normalizeQuoteText` 用 `strings.Contains+ReplaceAll` 循环压缩空行（O(n²) 风险）·【P2】

- **维度**：性能
- **严重度**：P2（典型 wiki 页面规模下不会触发；但单文件多连续空行会重复扫描全串；不是 P1 因为 QuoteHash 路径不在热路径）
- **位置**：`internal/index/anchor.go:282-290`
- **证据**：

  ```go
  func normalizeQuoteText(text string) string {
      s := strings.ReplaceAll(text, "\r\n", "\n")
      s = strings.ReplaceAll(s, "\r", "\n")
      s = strings.TrimSpace(s)
      for strings.Contains(s, "\n\n") {         // ❌ 每轮重扫
          s = strings.ReplaceAll(s, "\n\n", "\n")
      }
      return s
  }
  ```

- **违反**：行业共识（Effective Go：避免热路径 O(n²)）；Quote-hash 是协议契约的一部分（W2 D11 spec-entry 中 `read_raw_anchor` 由 daemon 计算），未来 daemon long-running 时这条路径变热。
- **修复方向**：
  - 一次性正则 `regexp.MustCompile(\n+).ReplaceAllString(s, "\n")`，或
  - 单 pass strings.Builder，遇到连续 `\n` 跳过。
  - 加 benchmark 测大输入下的 wall time。

---

### F-020 · `service.wiki_index.RebuildIndex` 在 N 页循环里跑 N 次 `countSourceLinks`·【P2】

- **维度**：性能
- **严重度**：P2（小 vault 不可察；上千 page 时每条单独 query 累加显著；纯 SQL 一次 GROUP BY 即可拿到 source-count 字典）
- **位置**：`internal/service/wiki_index.go:99-120`
- **证据**：

  ```go
  for _, p := range pages {
      ...
      srcCount := countSourceLinks(ctx, db, p.ID)  // 每条 1 次 SQL
      ...
  }
  ```

  `countSourceLinks` 内部 SQL 见 `wiki_index.go:133-144`，本身正确，但 N 次往返浪费。
- **违反**：行业共识（避免热路径 N+1 query）。
- **修复方向**：
  - 在 `RebuildIndex` 入口先一次性查：

    ```sql
    SELECT pl.target_id, COUNT(*) FROM page_links pl
      JOIN pages p ON pl.source_id = p.id
      WHERE p.type = 'source'
      GROUP BY pl.target_id
    ```

  - 放进 `map[string]int`，循环里 O(1) 查询。

---

### F-021 · `service.review_test.go` 自造 `contains/containsHelper`，已有 `strings.Contains`·【P2】

- **维度**：可维护性
- **严重度**：P2（FYI；测试代码冗余，看起来像 LLM 自动生成残留）
- **位置**：`internal/service/review_test.go:288-298`
- **证据**：

  ```go
  func contains(s, substr string) bool {
      return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
  }

  func containsHelper(s, substr string) bool {
      for i := 0; i <= len(s)-len(substr); i++ {
          if s[i:i+len(substr)] == substr {
              return true
          }
      }
      return false
  }
  ```

  调用点 line 170, 199 直接换成 `strings.Contains(err.Error(), "...")` 即可。
- **违反**：行业共识（Effective Go：使用标准库优先）。
- **修复方向**：删两个 helper，调用点改 `strings.Contains`。

---

### F-022 · 多处测试用 `err == ErrXxx`（应 `errors.Is`）·【P2】

- **维度**：测试质量 + 错误处理
- **严重度**：P2（当前 wrap 链短未触发；防御性写法应统一）
- **位置**：
  - `internal/service/queue_test.go:49`（`if err != ErrQueueBacklog`）
  - `internal/service/queue_test.go:75`（`if err != ErrQueueCritical`）
- **证据**：

  ```go
  err := CheckQueueForPropose(ctx, db, limits)
  if err != ErrQueueBacklog {            // ❌
      t.Fatalf("err = %v, want ErrQueueBacklog", err)
  }
  ```

  同仓内 `internal/index/reviews_test.go:82,85` 已经在用 `errors.Is`——风格不统一。
- **违反**：`.trellis/spec/backend/error-handling.md` "比较错误：永远 `errors.Is` / `errors.As`"。
- **修复方向**：批量替换为 `!errors.Is(err, ErrQueueBacklog)`。当前未踩坑只是因为 `CheckQueueForPropose` 直接返回 sentinel 未 wrap；一旦上层加 `fmt.Errorf("%w: ...")` 就会断。

---

### F-023 · `service.ingest_image.ExtractImageMeta` 双重静默吞错·【P2】

- **维度**：错误处理
- **严重度**：P2（设计上"image meta 是 best-effort，失败不阻塞"——可接受；但当前实现连权限错误 / corrupt 文件都不区分，无 telemetry 反馈）
- **位置**：`internal/service/ingest_image.go:42-60`
- **证据**：

  ```go
  func ExtractImageMeta(path string) *ImageMeta {
      f, err := os.Open(path)
      if err != nil {
          return nil          // ❌ 不区分 not-exist vs permission vs IO
      }
      defer f.Close()

      cfg, format, err := image.DecodeConfig(f)
      if err != nil {
          return nil          // ❌ 不区分 corrupt vs format-mismatch
      }
      ...
  }
  ```

- **违反**：`.trellis/spec/backend/error-handling.md` "不要静默吞错"。
- **修复方向**：改签名 `(*ImageMeta, error)`，调用方（ingest 流程）拿到 error 后决定 best-effort（继续）还是 abort。

---

### F-024 · `internal/service/ingest_image.go:27` 用 `fmt.Errorf` 声明 sentinel（应 `errors.New`）·【P2】

- **维度**：可维护性 + 错误处理风格
- **严重度**：P2（FYI；无功能差异，但 `fmt.Errorf` 用于 wrap，sentinel 应用 `errors.New`——sentinel 没有 format 参数，用 fmt.Errorf 是误用）
- **位置**：`internal/service/ingest_image.go:27`
- **证据**：

  ```go
  var ErrUnsupportedRawFormat = fmt.Errorf("unsupported raw format")   // ❌
  ```

  对照同包 `internal/service/ingest.go:23-29` 的正确写法：

  ```go
  var ErrSourceMissing = errors.New("ingest source missing")
  var ErrSourceUnreadable = errors.New("ingest source unreadable")
  var ErrInvalidVaultRoot = errors.New("invalid vault root")
  ```

- **违反**：`.trellis/spec/backend/error-handling.md` "sentinel error 用 `errors.New` 在包级 `var` 声明"。
- **修复方向**：改 `errors.New("unsupported raw format")`。

---

---

### F-025 · MCP 错误码 `CROSS_SESSION_BUNDLE` / `REVIEW_ALREADY_BUNDLED` / `SESSION_REQUIRED:` 用 inline `errors.New` 创建，未做包级 sentinel·【P1 / Spec-Drift】

- **维度**：契约稳定性 + 错误处理
- **严重度**：P1（协议错误码字面量必须可被 agent `errors.Is` 匹配/客户端字面量解析；inline 创建意味着每次返回都生成新 error 实例，调用方写 `errors.Is(err, ErrCrossSessionBundle)` 永远 false——只能字符串匹配，破坏 sentinel 语义；spec-v2/mcp-tools.md 中这些都是协议契约错误码）
- **位置**：
  - `internal/mcp/tools.go:433`（`errors.New("CROSS_SESSION_BUNDLE")` inline）
  - `internal/mcp/tools.go:439`（`errors.New("REVIEW_ALREADY_BUNDLED")` inline）
  - `internal/mcp/tools.go:1408,1435`（`errors.New("SESSION_REQUIRED: missing worktree")` 把协议码 + 自由文本混在一起）
- **证据**：

  ```go
  // tools.go:432-440
  if review.Agent != sess.Agent || review.SessionID != sess.SessionID {
      return RequestReviewResult{}, errors.New("CROSS_SESSION_BUNDLE")  // ❌ 应是包级 sentinel
  }
  ...
  if review.BundleID != "" {
      return RequestReviewResult{}, errors.New("REVIEW_ALREADY_BUNDLED")  // ❌ 同上
  }

  // tools.go:1408
  return errors.New("SESSION_REQUIRED: missing worktree")  // ❌ 协议码 + 自由文本
  ```

  对照同文件 `tools.go:56-60` 正确写法：
  ```go
  var (
      ErrAgentNotWhitelisted  = errors.New("AGENT_NOT_WHITELISTED")
      ErrSchemaIncompatible   = errors.New("SCHEMA_INCOMPATIBLE")
      ErrWorktreeCreateFailed = errors.New("WORKTREE_CREATE_FAILED")
  )
  ```

- **违反**：
  - `.trellis/spec/backend/error-handling.md` "sentinel error 用 `errors.New` 在包级 `var` 声明"。
  - `.trellis/spec/backend/error-handling.md` "MCP 错误码用 `SCREAMING_SNAKE_CASE`，定义在 sentinel 的 `errors.New(...)` 消息里"——前提是必须包级 sentinel。
  - `.trellis/spec/backend/quality-guidelines.md` W2 D11 `<spec-entry>` "CROSS_SESSION_BUNDLE / REVIEW_ALREADY_BUNDLED" 是稳定协议错误码契约。
- **修复方向**：
  - 在 `tools.go` 顶部 sentinel 块新增 `ErrCrossSessionBundle = errors.New("CROSS_SESSION_BUNDLE")`、`ErrReviewAlreadyBundled = errors.New("REVIEW_ALREADY_BUNDLED")`。
  - 把 line 1408/1435 改成 `return fmt.Errorf("%w: missing worktree", ErrSessionRequired)`。
  - 单测里把 `errors.Is(err, ErrCrossSessionBundle)` 加进 D11 测试套件。

---

### F-026 · MCP 工具实际注册 17 个但 W2 D11 spec-entry 写"15 total"·【P1 / Spec-Drift】

- **维度**：契约稳定性 + Spec-Drift
- **严重度**：P1（协议契约——MCP host 端的 schema major 版本承诺；新增工具且 ReadOnlyHint=false 影响 user confirmation 路径）
- **位置**：
  - `internal/mcp/server.go:52-157`（实际注册 17 个 tool）
  - `internal/mcp/server_test.go:38-91`（测试断言 17 个）
  - `.trellis/spec/backend/quality-guidelines.md:217` 的 W2 D11 spec-entry "Server registration asserts 15 total tools and all 6 write tools (agent_handshake plus D11 tools) are non-read-only"
- **证据**：

  ```go
  // server.go 17 个 sdk.AddTool 调用，比 D11 spec 多了：
  //   - acquire_lock  (write)
  //   - release_lock  (write)
  // 写工具实际是 8 个（agent_handshake + 5 D11 + 2 lock）而非 6 个
  ```

  ```go
  // server_test.go:89-91
  if len(tools) != len(wantNames) {  // wantNames 17 项
      t.Errorf("tool count = %d, want %d", len(tools), len(wantNames))
  }
  ```

- **现状**：代码引入了 lock 工具（D14+ 改动），但 spec-entry 数字未更新。
- **代码 vs spec**：**代码更新更新，spec 滞后**。
- **修复方向**（属 Spec-Drift，交给 trellis-update-spec 处理）：
  - 更新 `.trellis/spec/backend/quality-guidelines.md` 内 W2 D11 spec-entry：工具总数 → 17，write 工具 → 8（含 acquire_lock / release_lock）。
  - 同步在 spec-v2/docs/mcp-tools.md 加入这两个 tool 的协议定义。
  - 决定这两个 tool 是否属于 D11 schema 范围（major 不变），或要 minor +1。

---

### F-027 · 协议契约 op 字面量从 `log_append` 漂到 `append_log`·【P1 / Spec-Drift】

- **维度**：契约稳定性 + Spec-Drift
- **严重度**：P1（change-log JSON 字段 `op` 是审计契约，agent / revert / dream cycle 都要按这个字面量反查；spec 多处用 `log_append`，代码写成 `append_log`——双向不一致）
- **位置**：
  - `internal/mcp/tools.go:492`：`commit.CommitWithActor(ctx, b.root, sess.Agent, "append_log", ...)`
  - `internal/mcp/tools_test.go:605`：测试断言 `entry.Op != "append_log"`（绑定了错误字面量）
  - spec 中始终是 `log_append`：
    - `.trellis/spec/backend/logging-guidelines.md:60,88,186`
    - `.trellis/spec/backend/quality-guidelines.md:198,213,219` （W2 D11 spec-entry）
    - `spec-v2/docs/mcp-tools.md:36,649,655` （tool 名）
    - `spec-v2/templates/AGENTS.md:162`
- **证据**：

  ```go
  // tools.go:492
  entry, err := commit.CommitWithActor(ctx, b.root, sess.Agent, "append_log", summary, nil)

  // tools_test.go:605 锁死了字面量
  if entry.Actor != "codex-cli" || entry.Op != "append_log" {
  ```

- **冲突点**：
  - 工具名 `log_append`（MCP tool name 一致）。
  - 但产生的 change-log `op` 字段是 `append_log`（动词后置）。
  - 未来 dream-cycle / revert 按 spec 写 `WHERE op = 'log_append'` 时永远空集。
- **违反**：
  - `.trellis/spec/backend/logging-guidelines.md` op 枚举段："`review_accept` / `review_reject` / `log_append` / `lint_fix` / `dream_consolidate`"。
  - `.trellis/spec/backend/quality-guidelines.md` W2 D11 spec-entry 描述 log_append 直 commit 行为时用的字面量。
- **修复方向**：
  - 修代码：`tools.go:492` 改为 `"log_append"`；测试 `tools_test.go:605` 同步改。
  - 改完会产生 1 行 schema breaking change（旧 vault 里如有 `op=append_log` 的历史 commit，要么保留兼容、要么 reconcile）；但 v0.1.0 dogfood 阶段量很小，建议直接改并写一条 migration 注释。

---

### F-028 · `vaultBackend.lockManager()` 懒初始化非线程安全·【P1】

- **维度**：并发 + 正确性
- **严重度**：P1（race window 短但存在——两个并发 `acquire_lock` 在首次调用时都看到 `b.locks == nil`，各自 `NewManager()`，后写覆盖前者，先 acquire 的锁丢失。`go test -race` 当前不覆盖这条路径——D14 lock 工具集成测试在单 goroutine 跑）
- **位置**：`internal/mcp/tools.go:103-108`
- **证据**：

  ```go
  func (b *vaultBackend) lockManager() *lock.Manager {
      if b.locks == nil {           // ❌ read-modify-write 非原子
          b.locks = lock.NewManager()
      }
      return b.locks
  }
  ```

  对照 `server.go:40` 构造期：
  ```go
  backend := &vaultBackend{root: vaultRoot, db: db, sessions: NewSessionStore()}
  //                                                ^^^^^^^^^^^^^^^^^^^^^^^
  //                                                sessions 在构造时初始化
  //                                                locks 没初始化
  ```

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "Forbidden Patterns" 之"并发安全"隐含约定（不在显式列表但属常识）。
  - 行业共识：`go-race` / Effective Go "Maps are not safe for concurrent use" 同精神适用于 lazy-init pattern。
- **修复方向**：
  - 短期：在 `NewServer` 里 eager init：`backend := &vaultBackend{..., sessions: NewSessionStore(), locks: lock.NewManager()}`。
  - 中期：用 `sync.Once` 包裹 lazy-init（如果保留懒加载需求）。
  - `sessionStore()` 同样存在该模式，但构造期已 eager init 所以无 race 实际触发；建议两者一并处理。

---

### F-029 · Session 过期时未清理 worktree——资源泄漏·【P1】

- **维度**：资源管理 + 正确性
- **严重度**：P1（agent handshake 创建 git worktree + branch；session idle 超时 60min 后 token 从 store 删除，但 `wiki/_worktrees/agent-<agent>-<session>/` 目录与 `wt-<agent>-<session>` branch 都不清理；长 running daemon 上累计成几 GB 垃圾 + git plumbing 慢化）
- **位置**：
  - `internal/mcp/session.go:102-106`（`Authenticate` 内的 timeout-expire）
  - `internal/mcp/session.go:120-138`（`Expire` 函数本身不做 worktree 清理）
  - 全仓 grep 显示 `RemoveWorktree` 在 mcp 包从未被调用
- **证据**：

  ```go
  // session.go:102-106 expire 路径只删 map entry
  if time.Since(sess.LastSeenAt) > timeout {
      delete(s.byToken, token)
      delete(s.byKey, sessionKey(sess.Agent, sess.SessionID))
      return nil, ErrSessionRequired
      // ❌ 未调 worktreepkg.RemoveWorktree(ctx, vaultRoot, sess.Agent, sess.SessionID)
  }

  // session.go:120-138 Expire 同样的问题
  // mcp 包内全仓 grep:
  // $ grep -rn "RemoveWorktree" internal/mcp/  →  (no matches)
  ```

  对照 W2 D10 spec-entry：
  > Worktree cleanup must remove both the worktree and branch.

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` W2 D10 `<spec-entry>` 明确要求"worktree cleanup must remove both the worktree and branch"。
  - `.trellis/spec/backend/quality-guidelines.md` "Forbidden Patterns" 之"git worktree、临时目录清理"（资源管理 checklist）。
- **修复方向**：
  - `Expire` / `Authenticate` 拿到的 expired session 把（agent, session_id）传给 mcp 包注册的"清理回调"（避免 session 包反向依赖 worktree 包）。
  - 或者把 expire 路径整体上提到 `vaultBackend`，里面 own 清理 worktree 的责任。
  - 加单测：模拟 LastSeenAt 越界 → Authenticate 触发 expire → 验证 wiki/_worktrees/ 下对应目录消失 + `git worktree list` 不再含该 branch。

---

### F-030 · `SessionStore.Expire` 在生产代码中从未被调用·【P1】

- **维度**：资源管理 + 可维护性
- **严重度**：P1（与 F-029 互补：F-029 是"清理逻辑漏 worktree"，本条是"清理调度从未触发"。`Expire` 函数完整实现 + 测试覆盖，但 daemon 主循环 / handshake 路径都没有定期调用——sessions map 只增不减，10k 次 handshake 后 token 全部留在 map 里）
- **位置**：
  - `internal/mcp/session.go:120-138`（`Expire` 方法实现）
  - 全仓 grep `\.Expire\(` 仅命中 session_test.go:36
- **证据**：

  ```bash
  $ grep -rn "\.Expire(" internal/mcp/
  internal/mcp/session_test.go:36:	expired := store.Expire(now.Add(2 * time.Hour))
  # 生产代码无任何调用
  ```

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "Forbidden Patterns" 之"资源管理"隐含约定。
  - 行业共识：实现 + 测试 + 没用调用者 = dead code（Effective Go "Unused code"）。
- **修复方向**：
  - 在 `RunStdio` 或 daemon 主循环里起一个 ticker goroutine 周期（如每 5min）调用 `Expire(time.Now())`。
  - 配合 F-029：每个 expired session 触发 worktree 清理。
  - 加 metrics（W3+）：暴露当前活跃 session 数。

---

### F-031 · RateLimits 在 handshake 响应中宣称（30/min, 60/min）但代码完全不强制·【P1 / Spec-Drift】

- **维度**：契约稳定性 + 安全 + Spec-Drift
- **严重度**：P1（W2 D10 spec-entry "fixed D10 rate limits" 是协议承诺；但代码只在 handshake response 里返回这两个数字，从无任何调用 site 真正限流——agent 一秒打 1000 个 propose 也能跑过）
- **位置**：
  - `internal/mcp/tools.go:148-151`（设置 ProposePerMinute=30, QueryPerMinute=60）
  - `internal/mcp/types.go:13-16`（RateLimitsBlock 结构）
  - 全仓无 `propose_per_minute` / `query_per_minute` / rate limiting 计数器实现
- **证据**：

  ```go
  // tools.go:148-151
  RateLimits: RateLimitsBlock{
      ProposePerMinute: 30,
      QueryPerMinute:   60,
  },

  // 但 handleProposePage / handleProposeEdit / handleProposeClaim / handleSearch
  // 都没有任何 rate counter check
  ```

  ```bash
  $ grep -rn "rate\|Rate.*Limit\|propose_per_minute" internal/mcp/
  # 仅 types/tools.go 的 struct + literal，没有 enforce 路径
  ```

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` W2 D10 `<spec-entry>` "session token，fixed D10 rate limits"——承诺存在却未实施。
  - CWE-770（Allocation of Resources Without Limits or Throttling）。
- **是否 Spec-Drift**：是——选 A：实现 rate limiter 让代码兑现 spec；选 B：spec 改为"D10 stage 仅返回数字，强制留 W3+"。
- **修复方向**：
  - 短期（Spec-Drift 路径）：spec 段加注释 "rate_limits 字段当前 staged，agents 应自律；强制施加将随 D14+ 添加"。
  - 中期（实现路径）：在 `SessionStore` 里给每个 session 加 `proposeCounter`, `queryCounter` 滑动窗口；在 `handleProposeXxx` 入口先 `IncrementAndCheck`。
  - 推荐先走 Spec-Drift 让 v0.1.0 状态描述自洽；v0.2 再加 enforce。

---

### F-032 · `read_raw` 显式 `_ = ctx` 丢弃 ctx，未做 cancellation 检查·【P2】

- **维度**：错误处理 + 可维护性
- **严重度**：P2（同 F-015 同精神——signature 接收 ctx 但内部不传；I/O 函数应在 read/stat 之前先 check ctx.Err()，长读阻塞期间外层 cancel 无法 propagate）
- **位置**：`internal/mcp/tools.go:779-783`
- **证据**：

  ```go
  func (b *vaultBackend) handleReadRaw(ctx context.Context, args ReadRawArgs) (ReadRawResult, error) {
      _ = ctx          // ❌ ctx 丢弃
      rawID := strings.TrimSpace(args.RawID)
      ...
      data, err := os.ReadFile(abs)   // 没 ctx 版本 → 大文件读 + 外层 cancel 无效
  ```

  对照同包 `handleReadRawAnchor`（line 838-886）正确传 ctx 给 `index.FindSourceByRawID`。
- **违反**：`.trellis/spec/backend/quality-guidelines.md` "所有 I/O 函数第一参数 `ctx context.Context`"——签名兑现内部不传是隐性违约（同 F-015 模式）。
- **修复方向**：开头加 `if err := ctx.Err(); err != nil { return ReadRawResult{}, err }`；或 wrap `os.ReadFile` 为 ctx-aware（开 goroutine + select）。

---

### F-033 · `handleGetHistory` 用 `strings.Contains(err.Error(), "does not have any commits yet")` 判 git 空仓·【P2】

- **维度**：跨平台 + 错误处理
- **严重度**：P2（git 错误消息字符串非协议契约；不同 git 版本 / locale 翻译会变（如 Chinese git "尚未提交"）；当前未踩坑只因 CI 用英文 locale）
- **位置**：`internal/mcp/tools.go:1180-1185`
- **证据**：

  ```go
  out, err := runVaultGit(ctx, b.root, "log", ...)
  if err != nil {
      if strings.Contains(err.Error(), "does not have any commits yet") {  // ❌
          return GetHistoryResult{Commits: []HistoryCommit{}}, nil
      }
      return GetHistoryResult{}, fmt.Errorf("get_history: git log: %w", err)
  }
  ```

  问题加剧：`runVaultGit` 内部用 `errors.New(stderr)` 把原始 `*exec.ExitError` 丢了——上层连 exit code 都拿不到，只能字符串匹配。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "比较错误：永远 `errors.Is` / `errors.As`"。
  - `.trellis/spec/backend/quality-guidelines.md` "Required Patterns" 之"跨平台 git init"同精神：不要硬编码 git 输出文案。
- **修复方向**：
  - 改用 `git rev-parse --verify HEAD` 先探测仓库是否有 commit：有 → 跑 git log；无 → 返回空。
  - 或先 `runGit("rev-list", "-n", "1", "HEAD")` 失败 → 空仓兜底。
  - 与 F-008/F-014/F-034 一并：把 runVaultGit 收归 `internal/commit` 包，那边的 `runGit` 会保留 `*exec.ExitError` 让上层用 `errors.As` 拿 exit code。

---

### F-034 · `runVaultGit` 是第 4 个 git 子进程包装·【P2 / Spec-Drift 关联】

- **维度**：可维护性 + 错误处理
- **严重度**：P2（与 F-008 / F-014 / F-002 同根问题——`internal/git` 包是空壳，git 包装散落四处。runVaultGit 又比 vault/commit/proposal 三处多了一个"丢失原始 exec.Error"的缺陷）
- **位置**：`internal/mcp/tools.go:1352-1366`
- **证据**：

  ```go
  func runVaultGit(ctx context.Context, vaultRoot string, args ...string) (string, error) {
      cmd := exec.CommandContext(ctx, "git", args...)
      cmd.Dir = vaultRoot
      ...
      if err := cmd.Run(); err != nil {
          msg := strings.TrimSpace(stderr.String())
          ...
          return stdout.String(), errors.New(msg)  // ❌ 原始 *exec.ExitError 丢了
      }
  ```

  对照 `internal/commit/git.go:281-310` 的 `runGit`：保留原始 error + sentinel-aware（git missing → `ErrGitMissing`）；mcp 的版本两者都不具备。
- **违反**：F-008 同根原因；`.trellis/spec/backend/quality-guidelines.md` "Forbidden Patterns" 之"重复实现"。
- **修复方向**：和 F-008 / F-014 一并——把所有 git 子进程包装收归 `internal/commit` 或重建 `internal/git`；mcp 包改用统一入口。

---

### F-035 · 幂等 early-return 硬编码 `"passed"` validations·【P2】

- **维度**：契约稳定性 + 正确性
- **严重度**：P2（agent 重复用同一 idempotency_key 发 propose，第一次失败被人工 reject 后留下 `status=rejected` 的 review；第二次同 key 命中 early-return，但响应里依旧返回 `Validations.SchemaCheck="passed"` 等字样——与实际 review 状态矛盾，agent 可能误以为新提议被接受）
- **位置**：
  - `internal/mcp/tools.go:221-229`（propose_page）
  - `internal/mcp/tools.go:270-273`（propose_edit）
  - `internal/mcp/tools.go:322-325`（propose_claim）
- **证据**：

  ```go
  // tools.go:220-229
  } else if existing != nil {
      return ProposeResult{
          ReviewID: existing.ID,
          Status:   existing.Status,    // 可能是 "rejected" / "merged" / ...
          Validations: ValidationBlock{
              SchemaCheck:    "passed",  // ❌ 硬编码，与 existing.Status 不匹配
              QuoteHashCheck: "skipped",
              PathCheck:      "passed",
          },
      }, nil
  }
  ```

- **违反**：W2 D11 spec-entry "Idempotency is scoped to (agent, idempotency_key) and returns the existing review before touching the worktree"——返回 existing review 是对的，但 validation block 不应捏造。
- **修复方向**：
  - 把 validation 状态持久化到 reviews.meta_json（首次 propose 时存）；early-return 时反序列化复原。
  - 或返回 `Validations: ValidationBlock{SchemaCheck: "cached", ...}` 显式表明"复用历史结果"。

---

### F-036 · `acquire_lock` / `release_lock` 把 lock 包内部 err 转字符串塞 Message 字段·【P2】

- **维度**：错误处理 + 信息泄漏
- **严重度**：P2（不算硬泄漏——lock 包内部 err 是"already held by agent X"这类业务 message；但暴露了"agent X 持有锁"这一跨 session 信息，且方式是 `err.Error()` 字符串透传，未来 lock 包加内部细节会一起泄漏出去）
- **位置**：
  - `internal/mcp/tools.go:516-521`（acquire_lock 失败路径）
  - `internal/mcp/tools.go:536-541`（release_lock 失败路径）
- **证据**：

  ```go
  if err := b.lockManager().Acquire(args.PageID, sess.Token, sess.Agent, ttl); err != nil {
      return AcquireLockResult{
          Acquired: false,
          Message:  err.Error(),    // ❌ 内部 err 直透客户端
      }, nil
  }
  ```

- **违反**：
  - `.trellis/spec/backend/error-handling.md` "CLI 错误堆栈泄漏" 同精神（虽是 MCP 不是 CLI，但客户端可见）。
  - `.trellis/spec/backend/logging-guidelines.md` "不要日志敏感信息"宽义涵盖跨 session 信息暴露。
- **修复方向**：
  - 把 lock 包错误翻译成稳定协议码（如 `LOCK_HELD_BY_OTHER` / `LOCK_NOT_FOUND` / `LOCK_EXPIRED`），返回字符串错误码而非 raw `err.Error()`。
  - 与 F-025 一并：所有协议错误都走包级 sentinel + 协议码字面量。

---

### F-037 · 5 个 propose tool 共用样板未抽 helper·【P2】

- **维度**：可维护性
- **严重度**：P2（每个 propose_xxx + log_append + request_review 都重复写 `b.authenticateWrite(token)` + `index.FindReviewByIdempotencyKey` + `proposal.Validate*` + `b.writePageInWorktree` + `proposal.GeneratePatch` + `b.insertPatchReview` 这条链；改一处要改 3-4 处，回归风险高）
- **位置**：
  - `internal/mcp/tools.go:213-261`（handleProposePage 49 行）
  - `internal/mcp/tools.go:263-313`（handleProposeEdit 51 行）
  - `internal/mcp/tools.go:315-402`（handleProposeClaim 87 行）
- **证据**：

  ```go
  // 三处都是：
  sess, err := b.authenticateWrite(args.SessionToken)
  if err != nil { return ..., err }
  if existing, err := index.FindReviewByIdempotencyKey(...); err != nil {
      return ..., err
  } else if existing != nil {
      return ProposeResult{ReviewID: existing.ID, Status: existing.Status, Validations: ...}, nil
  }
  // ...
  if err := b.writePageInWorktree(ctx, sess, path, fm, body); err != nil {
      return ..., err
  }
  patch, err := proposal.GeneratePatch(ctx, sess.WorktreePath, sess.Branch, path)
  if err != nil { return ..., err }
  review, err := b.insertPatchReview(ctx, sess, op, target, patch, meta)
  ```

- **违反**：`.trellis/spec/backend/quality-guidelines.md` "Forbidden Patterns" 之"重复代码" + "Composition over inheritance（提取 helper）"。
- **修复方向**：抽 `proposeWithIdempotency(ctx, sess, op, target, idempotencyKey, validateAndStage func(...), meta) (ProposeResult, error)`——把 auth / dedup / patch generate / review insert 收归 helper；3 个 handler 各自只负责 validate + stage 自己的内容。

---

### F-038 · `handleProposeClaim` 单函数 87 行 13 步·【P2】

- **维度**：可维护性
- **严重度**：P2（超过 spec "函数过长（> 80 行）"门槛，但相比 F-013 review.AcceptReview 的 120 行轻得多；F-037 抽完 helper 后这个会自然 < 50 行）
- **位置**：`internal/mcp/tools.go:315-402`
- **证据**：87 行连续 if-check + manual frontmatter 构造 + validate*4 + write + patch + insert review；步骤间无注释 section 分隔。
- **违反**：`.trellis/spec/backend/quality-guidelines.md` "函数过长（> 80 行）/ 复杂度"。
- **修复方向**：先抽 F-037 共用 helper，剩余 claim 专属逻辑（confidence 校验 / 默认 status 决策 / sources frontmatter 构造）单独拆 `validateClaimArgs(args) (fm map[string]any, err error)`。

---

### F-039 · `internal/mcp/tools.go` 单文件 1588 行·【P2】

- **维度**：可维护性
- **严重度**：P2（更新 F-006 的具体度量——tools.go 占 mcp 包近 4 成；按工具组拆 read / write / handshake / log_append / lock 5 个文件即可缓解）
- **位置**：`internal/mcp/tools.go` 1588 行总长
- **证据**：

  ```
  $ wc -l internal/mcp/tools.go
  1588 internal/mcp/tools.go
  $ grep -c "^func" internal/mcp/tools.go
  35
  ```

  35 个函数全塞一个文件；含 read（10 handler）/ write（5 handler）/ lock（2 handler）/ helpers（18 个）。
- **违反**：行业共识（Effective Go：单文件 < 1000 行）+ F-006 已记录的同精神。
- **修复方向**：
  - `tools_read.go`：read_page / read_raw / read_raw_anchor / read_claim / list_index / search / graph_neighbors / get_history / wiki_info
  - `tools_write.go`：propose_page / propose_edit / propose_claim / request_review
  - `tools_handshake.go`：agent_handshake
  - `tools_log.go`：log_append
  - `tools_lock.go`：acquire_lock / release_lock
  - `tools_helpers.go`：runVaultGit / sha256Hex / parseUpdatedSince / stringSet / tokenizer* / resolvePagePath / etc

---

### F-040 · 写工具 body / summary / unified_diff 缺输入上限·【P2】

- **维度**：安全（DoS） + 正确性
- **严重度**：P2（MCP 端口暴露给本地 agent，且 stdio transport 单 daemon 单进程；恶意 / bug agent 一次发 1 GB body 会让 daemon OOM。当前只 LogAppend.Message 有 500 rune 限制，propose_page/edit/claim 的 Body 和 ProposeEdit.Patch.UnifiedDiff 完全无限制）
- **位置**：
  - `internal/mcp/types.go:50-95`（ProposePageArgs.Body / ProposeEditArgs.Patch.UnifiedDiff / ProposeClaimArgs.Body 都无上限）
  - `internal/mcp/tools.go:281-282`（ApplyPatch 直接 cast `[]byte(args.Patch.UnifiedDiff)`）
- **证据**：

  ```go
  // types.go：所有这些字段未声明 jsonschema maxLength
  Body           string `json:"body"`
  UnifiedDiff    string `json:"unified_diff,omitempty"`

  // tools.go:485-486 LogAppend 有上限：
  if message == "" || len([]rune(message)) > 500 {
      return ..., fmt.Errorf("%w: message length", proposal.ErrSchemaViolation)
  }
  // 但 ProposePage/Edit/Claim 路径无任何 len 检查
  ```

- **违反**：
  - CWE-770（Resource exhaustion）。
  - `.trellis/spec/backend/error-handling.md` "Fail fast with descriptive messages" 隐含的"输入验证应在边界完成"。
- **修复方向**：
  - 在 types.go 加 jsonschema tag `maxLength`；server 解析层就 reject 超长输入。
  - 或在 handler 入口加 `if len(args.Body) > 1<<20 { return ..., fmt.Errorf("%w: body too large", proposal.ErrSchemaViolation) }`（1 MB 阈值）。
  - 同步加 unified_diff 上限（如 1 MB）。

---

### F-041 · `internal/bridge` 在 Windows 上完全不可用——Named Pipe 路径配 unix socket 监听·【P1】

- **维度**：跨平台 + 正确性
- **严重度**：P1（W3+ daemon 上 Windows 时直接 boot 失败；W2 阶段未启用 daemon 所以暂未爆雷，但是部署 blocker）
- **位置**：
  - `internal/bridge/bridge.go:14-22`（SocketPath 在 Windows 返回 `\\.\pipe\wikimind-<base>`）
  - `internal/bridge/bridge.go:53-58`（Listen 永远用 `network := "unix"`，包括 Windows 分支）
- **证据**：

  ```go
  func SocketPath(vaultRoot string) string {
      if runtime.GOOS == "windows" {
          // Use a named pipe on Windows.
          return `\\.\pipe\wikimind-` + filepath.Base(vaultRoot)  // ← Named Pipe path
      }
      return filepath.Join(vaultRoot, ".wikimind", "daemon.sock")
  }

  func Listen(socketPath string) (*Listener, error) {
      ...
      network := "unix"
      if runtime.GOOS == "windows" {
          network = "unix" // Go supports unix sockets on Windows 10+  ← 没改 network!
      }
      ln, err := net.Listen(network, socketPath)  // 用 unix 监听 Named Pipe 路径 → 失败
      ...
  }
  ```

- **问题**：
  1. Windows 的 `\\.\pipe\...` 是 Named Pipe 命名空间，不是文件系统路径。
  2. `net.Listen("unix", "\\.\pipe\wikimind-foo")` 会试图在文件系统的根上创建 Unix socket 文件——路径解析失败或拒绝。
  3. Windows 10+ 支持 Unix domain socket，但路径必须是文件系统路径（如 `C:\Users\foo\.wikimind\daemon.sock`），不是 Named Pipe namespace。
  4. line 54 注释 `Go supports unix sockets on Windows 10+` 和 line 17 注释 `Use a named pipe on Windows` 自相矛盾。
- **违反**：
  - 跨平台测试缺失（test 在 macOS/Linux 跑过，没 Windows CI smoke）。
  - 与 `.trellis/spec/backend/quality-guidelines.md` "跨平台兜底" 类比 `git init --initial-branch=main` 的 spec-entry——bridge 需要类似的统一抽象。
  - CWE-393（Return of Wrong Status Code）：Listen 返回 nil err 但 socketPath 不通。
- **修复方向**：
  - 选 A：SocketPath 在 Windows 返回 `filepath.Join(localAppData, "WikiMind", base+".sock")`（文件系统路径），Listen 在所有平台都用 unix；删除 Named Pipe 注释。
  - 选 B：用真正的 Windows Named Pipe 实现，引入 `gopkg.in/natefinch/npipe.v2` 或 `microsoft/go-winio`，给 Windows 起一个并行 listener。
  - 当前测试 `TestSocketPath` 只断言 macOS/Linux 路径，未验证 Windows。必须加 Windows CI smoke。

---

### F-042 · `Watcher.Close` 与 `time.AfterFunc` 回调存在 send-on-closed-channel race·【P1】

- **维度**：并发 + 资源管理 + 正确性
- **严重度**：P1（debounce 时间窗内 Close 可触发 panic；W3 daemon 长跑场景，watcher 长期持有，CTRL-C 时窗口期短但概率 > 0）
- **位置**：
  - `internal/watcher/watcher.go:93-105`（Close 序列）
  - `internal/watcher/watcher.go:107-125`（debounce + AfterFunc 回调）
- **证据**：

  ```go
  func (w *Watcher) Close() error {
      close(w.done)
      err := w.fsw.Close()
      w.wg.Wait()
      // Drain any pending timers.
      w.mu.Lock()
      for _, t := range w.timers {
          t.Stop()         // ← 已 fired 但未 acquire mu 的回调，Stop 拦不住
      }
      w.mu.Unlock()
      close(w.events)      // ← 此后任何 send 都 panic
      return err
  }

  func (w *Watcher) debounce(path string, op fsnotify.Op) {
      ...
      w.timers[path] = time.AfterFunc(w.debounceMs, func() {
          w.mu.Lock()           // ← 已 fired, 等 Close 释放锁
          delete(w.timers, path)
          w.mu.Unlock()

          select {
          case w.events <- FileEvent{...}:   // ← Close 已关 channel → panic
          default:
          }
      })
  }
  ```

- **场景**：
  1. T0：fsnotify 事件触发 debounce，创建 timer，回调将在 +200ms 触发。
  2. T+200ms：timer 触发，回调进入 goroutine，开始等 `w.mu.Lock()`。
  3. T+201ms：用户 Ctrl-C，daemon 调用 `w.Close()` —— 拿到 mu，stop 所有 timer（但此 timer 已 fired，stop 返回 false），unlock，关闭 events 通道。
  4. T+202ms：回调拿到 mu，delete map 项，unlock，进入 select，尝试 `w.events <- ...` → **send on closed channel panic**。
- **违反**：
  - 行业共识：Go 并发指南"don't close channels you're still trying to send on"。
  - `.trellis/spec/backend/error-handling.md` "不要 panic 在业务路径"。
- **修复方向**：
  - 引入 `closed bool` + mutex 保护：回调 select 前查 `if w.closed { return }`。
  - 或者用 `done` 信号 + 非阻塞 select，把 `case w.events <-` 改成 `case <-w.done: return; case w.events <- ...`。
  - 更彻底：用 `for range timer.C` + 单独的循环 goroutine 替代 AfterFunc，统一通过 ctx.Done 退出。

---

### F-043 · `Daemon.Shutdown` 重入会 panic（close on closed channel via watcher）·【P2】

- **维度**：并发 + 资源管理
- **严重度**：P2（当前调用方都不重入；但是 `Daemon.Run` 末尾会调一次 `d.Shutdown()`，而 `daemon_test.go:64` 的 cleanup 也调一次 → 若 Run 已正常退出，cleanup 触发第二次 panic）
- **位置**：
  - `internal/daemon/loop.go:125-138`（Shutdown 实现）
  - `internal/watcher/watcher.go:94`（close(w.done) 不可重入）
  - `internal/daemon/loop_test.go:64`（cleanup 兜底）
- **证据**：

  ```go
  // daemon/loop.go
  func (d *Daemon) Shutdown() error {
      if d.cancel != nil {
          d.cancel()  // ← 可重入，CancelFunc 自带 once 语义
      }
      _ = d.watcher.Close()  // ← watcher.Close 不可重入！
      d.wg.Wait()
      d.db.Close()           // ← 可重入（已确认 index.DB.Close idempotent）
      d.logger.Printf("stopped")
      if d.logFile != nil {
          _ = d.logFile.Close()  // ← *os.File.Close 二次调返回 ErrClosed，不 panic
      }
      return nil
  }

  // watcher/watcher.go:94
  close(w.done)  // ← 第二次 close 触发 panic: close of closed channel
  ```

- **当前测试 cleanup 之所以没爆**：`TestDaemonLockManager` 不调用 Run，只 cleanup → 单次 Shutdown，OK。
- **违反**：
  - 行业共识：Go 并发指南"close once"；公共方法应 idempotent 或文档化"only call once"。
  - `.trellis/spec/backend/quality-guidelines.md` "测试不 mock fs/git/SQLite" + "新代码有测试覆盖正常+错误路径"——但 Shutdown 重入未测。
- **修复方向**：
  - Daemon.Shutdown 加 `sync.Once`：`d.shutdownOnce.Do(func() { ... })`。
  - 或在 Watcher.Close 加同样保护：用 `closed bool` + atomic CAS 守 close(w.done)。
  - 加测试：`TestDaemonShutdownIsIdempotent`，连调两次 Shutdown 不 panic。

---

### F-044 · `internal/watcher` 静默吞 fsnotify error 与丢事件·【P1】

- **维度**：错误处理 + 正确性 + 可观察性
- **严重度**：P1（fsnotify 错误是基础设施信号——文件系统 unmount / inotify quota 用满 / Windows ReadDirectoryChangesW 失败，全部被 swallow；daemon 在错误后继续转但 watcher 已死）
- **位置**：
  - `internal/watcher/watcher.go:82-87`（吞 Errors 通道）
  - `internal/watcher/watcher.go:119-123`（channel 满时丢事件无任何上报）
- **证据**：

  ```go
  case _, ok := <-w.fsw.Errors:
      if !ok {
          return
      }
      // Swallow errors for now; W3 will add error reporting.

  select {
  case w.events <- FileEvent{Path: path, Op: op, Ts: time.Now()}:
  default:
      // Channel full; drop event (W3 will add backpressure).
  }
  ```

- **影响**：
  - daemon `loop.go:104-117` 的消费者只对 `w.Events()` 起反应；fsnotify 死后，daemon 仍主循环转，但不会再收到任何文件事件——`wikimind watch --auto-ingest` 用户以为它在工作，实际盲了。
  - 丢事件无任何 metric / log，bug 极难诊断。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "不要静默吞错"。
  - `.trellis/spec/backend/logging-guidelines.md` "用户输出"段——错误应至少走 stderr 一次。
- **修复方向**：
  - 给 `Watcher` 增加 `Errors() <-chan error` 通道暴露 fsnotify 错误；daemon 消费方决定怎么处理（log + 终止 / log + 重启 watcher）。
  - 丢事件路径：增加 `OverflowCount uint64` atomic 计数器，daemon 周期性 log；或者把 `events` channel 扩到 256 + 阻塞 send（背压而不是丢）。
  - W3 注释承诺的 backpressure 是 future tense，但 v0.1.0 用户已能 `wikimind watch --auto-ingest`——风险已暴露。

---

### F-045 · `internal/bridge.SocketPath` 实现与文档自相矛盾·【P2 / Spec-Drift 关联】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P2（独立于 F-041 的 Windows 不可用问题；这里只看注释 vs 代码一致性）
- **位置**：
  - `internal/bridge/bridge.go:14-16`（注释承诺 Windows 用 Named Pipe `\\.\pipe\wikimind-<hash>`）
  - `internal/bridge/bridge.go:19`（实际用 `filepath.Base(vaultRoot)`，不是 hash）
  - `internal/bridge/bridge.go:54-56`（注释说 "Go supports unix sockets on Windows 10+"，与注释 17 冲突）
- **证据**：

  ```go
  // SocketPath returns the IPC socket path for a vault.
  // Unix: <vault>/.wikimind/daemon.sock
  // Windows: \\.\pipe\wikimind-<hash>          ← 说是 hash
  func SocketPath(vaultRoot string) string {
      if runtime.GOOS == "windows" {
          // Use a named pipe on Windows.
          return `\\.\pipe\wikimind-` + filepath.Base(vaultRoot)  // ← 实际是 base name
      }
      ...
  }
  ```

- **风险**：
  1. `filepath.Base("/Users/foo/My Vault")` = `"My Vault"` 含空格——Named Pipe 名称合法但脆弱。
  2. 两个 vault 同名（如不同盘符下都叫 `vault`）→ pipe path 冲突。
  3. 用户读注释期待 hash，看代码以为是 hash，实际是 base name——日后调试时混淆来源。
- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "包级 doc.go 与新功能一致"。
  - 行业共识：注释与实现矛盾是 maintainability 红线。
- **修复方向**：
  - 修注释为 `\\.\pipe\wikimind-<sanitized-base>` 或真用 sha256[:8] hash（仍需先解决 F-041 跨平台问题）。
  - 在 spec-v2 增补 "IPC socket naming convention" 章节。

---

### F-046 · 锁 reaper 在 daemon shutdown 期间最后一轮 reap 缺失·【P2】

- **维度**：资源管理 + 正确性
- **严重度**：P2（影响：daemon 优雅退出时已 expired 的 lock 不会被 reap 清理，重启后会继承 stale 状态；W3 daemon 持久化后影响放大）
- **位置**：
  - `internal/daemon/loop.go:84-100`（reaper goroutine）
  - `internal/daemon/loop.go:120-123`（Run 末尾直接 Shutdown，无终态 sweep）
- **证据**：

  ```go
  d.wg.Add(1)
  go func() {
      defer d.wg.Done()
      ticker := time.NewTicker(30 * time.Second)
      defer ticker.Stop()
      for {
          select {
          case <-ctx.Done():
              return                          // ← 退出前不做 final Reap
          case now := <-ticker.C:
              reaped := d.lockMgr.Reap(now)
              ...
          }
      }
  }()
  ```

- **问题**：
  - reaper 每 30s 跑一次；如果 lock 在第 29s 过期，daemon 第 31s shutdown，那个 lock 永远未被 reap。
  - 当前 `lock.Manager` 是内存的，daemon shutdown 即丢失，所以"未 reap"无实质影响。
  - 但是若 W3 把 lock 持久化（roadmap 提到 `internal/lock` 会上 SQLite），未 reap 的 stale lock 会跨重启遗留。
- **违反**：
  - 一般工程原则：长跑服务退出前应做 best-effort cleanup。
  - `.trellis/spec/backend/quality-guidelines.md` "资源管理"维度——daemon 退出时是清理时机。
- **修复方向**：
  - 在 `ctx.Done()` 分支前加一次 `d.lockMgr.Reap(time.Now())` 作为 final sweep。
  - 或者在 `Shutdown` 函数里显式调用一次 Reap。

---

### F-047 · `internal/lint` 所有规则把 DB 错误转成"无 finding"，掩盖真实问题·【P1】

- **维度**：错误处理 + 正确性 + 可观察性
- **严重度**：P1（lint 是"vault 健康检查"——DB 不可用时返回零 finding 用户以为"全绿"；与"无问题"完全混淆）
- **位置**：
  - `internal/lint/rules.go:17-22`（OrphanRule.Run）
  - `internal/lint/rules.go:46-51`（BrokenLinkRule.Run）
  - `internal/lint/rules.go:78-83`（SchemaViolationRule.Run）
  - `internal/lint/rules.go:113-117`（UnverifiedClaimRule.Run）
  - `internal/lint/rules.go:137-149`（MissingIndexEntryRule.Run，2 处）
  - `internal/lint/rules.go:27-28`（inline 吞错）
  - `internal/lint/rules.go:58`（inline 吞错）
- **证据**：

  ```go
  func (r *OrphanRule) Run(ctx context.Context, vaultRoot string, db *index.DB) []Finding {
      var findings []Finding
      pages, err := index.ListPages(ctx, db, "")
      if err != nil {
          return nil    // ← DB 错误 → 静默返回 0 finding
      }
      for _, p := range pages {
          ...
          inbound, _ := index.InboundLinks(ctx, db, p.ID)   // ← 错误吞
          outbound, _ := index.OutboundLinks(ctx, db, p.ID) // ← 错误吞
          ...
      }
      return findings
  }
  ```

- **场景**：
  - SQLite 被 unmount / 文件损坏 / migration 未跑 → `index.ListPages` 返回错误 → lint 显示 "✓ No issues found"。
  - `cmd/wikimind/command.go:843` 的 `"✓ No issues found (%d rules checked)"` 把这种情况渲染成"完全健康"。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "Required: 未找到结果就返回 sentinel，不返回 nil error"——这里返回 `nil findings` 但 nil 意为"无问题"，不是"未检查"。
  - `.trellis/spec/backend/quality-guidelines.md` "不要静默吞错"。
- **修复方向**：
  - `Rule.Run` 签名改成 `Run(ctx, vaultRoot, db) ([]Finding, error)`；上层 RunRules 聚合多 rule 的错误并显示给用户。
  - 或保持签名但在错误时返回一个 `Severity=error Rule=lint_internal_error` 的 Finding，让 CLI 看见而不是隐瞒。
  - inline `_, _ := index.InboundLinks(...)` 至少应 log 到 stderr 或聚合 error counter。

---

### F-048 · `cmd/wikimind/command.go:108` 硬编码 `"ready: 15 tools registered"`——契约漂移 F-026 的另一证据·【P1 / Spec-Drift】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P1（多处硬编码 "15 tools" 字面量 → 真实是 17——是 F-026 的延伸；用户启动 MCP server 看到 daemon 自报"15 tools"但实际 client 拿到 17 个）
- **位置**：
  - `cmd/wikimind/command.go:108`（logger.Printf "ready: 15 tools registered"）
- **证据**：

  ```go
  server, err := mcppkg.NewServer(ctx, vaultRoot, db)
  if err != nil {
      return fmt.Errorf("build mcp server: %w", err)
  }
  logger.Printf("ready: 15 tools registered")  // ← 硬编码，不查 server.ToolCount()
  ```

- **关联**：
  - F-026 已记录 spec-entry "W2 D11" 说 "15 total tools"，代码实际 17。
  - 这里 daemon 启动日志也硬编码 15，证明开发者复制粘贴而非动态查询。
  - `cmd/wikimind/command_test.go:680` 的 TestMcpServeCommandRegistered 只验证 help 文本里有 9 个 read tool 名字，没断言数字。
- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` spec-entry "W2 D11" §1-3。
  - 行业共识：count 字面量应从单一源（server.ToolCount() 之类的 API）派生。
- **修复方向**：
  - 给 `mcppkg.Server` 暴露 `ToolCount() int`；daemon log 改成 `logger.Printf("ready: %d tools registered", server.ToolCount())`。
  - 与 F-026 一同修；F-026 修 spec，F-048 修代码字面量，全链路一致。

---

### F-049 · `cmd/wikimind/command.go:821` CLI Short 描述 "Run vault health checks (8 rules)" 实际 5 rules·【P1 / Spec-Drift】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P1（CLI 文档与实际规则数不一致——用户敲 `wikimind lint --help` 看到"8 rules"但只运行 5 规则；spec-v2 又写 "13 规则"——三方漂移）
- **位置**：
  - `cmd/wikimind/command.go:821`（Short: "Run vault health checks (8 rules)"）
  - `internal/lint/lint.go:41-49`（AllRules 实际返回 5 条）
  - `spec-v2/docs/engineering-decisions.md:200`（spec 注释 "lint 13 规则"）
- **证据**：

  ```go
  // cmd/wikimind/command.go:817-822
  cmd := &cobra.Command{
      Use:   "lint",
      Short: "Run vault health checks (8 rules)",   // ← 说 8

  // internal/lint/lint.go:41-49
  func AllRules() []Rule {
      return []Rule{
          &OrphanRule{},
          &BrokenLinkRule{},
          &SchemaViolationRule{},
          &UnverifiedClaimRule{},
          &MissingIndexEntryRule{},                  // ← 实际 5
      }
  }

  // spec-v2/docs/engineering-decisions.md:200
  // │   │   ├── lint.go          #   lint 13 规则       ← 说 13
  ```

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "CLI 输出格式没破坏已有断言（CLI 契约也是协议）"。
  - 三方漂移使 spec 失效作为权威源。
- **修复方向**：
  - 短期：把 Short 改成动态：`fmt.Sprintf("Run vault health checks (%d rules)", len(lint.AllRules()))`。
  - 或最简：删数字 → `"Run vault health checks"`，让 `--help` 不携带 count。
  - 同步修 spec-v2 写 "5 rules (W3 expands to 13)" 之类的过渡说明，避免误导。

---

### F-050 · `cmd/wikimind/command.go:763` warning 输出到 stdout 而不是 stderr·【P2】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P2（违反 logging-guidelines 的 "warning → stderr" 约定；automated 脚本管道 stdout 时会把 warning 当作数据混入）
- **位置**：
  - `cmd/wikimind/command.go:761-766`（reindex 命令的 rebuildIndex warning）
- **证据**：

  ```go
  // Also rebuild wiki/index.md
  if err := service.RebuildIndex(cmd.Context(), db, vaultRoot); err != nil {
      fmt.Fprintf(stdout, "warning: rebuild wiki/index.md: %v\n", err)  // ← warning 写 stdout!
  } else {
      fmt.Fprintf(stdout, "rebuilt wiki/index.md\n")
  }
  ```

- **对比**：`cmd/wikimind/command.go:302-304` 的 ingest 自动 reindex 失败正确用 `cmd.ErrOrStderr()`：

  ```go
  fmt.Fprintf(cmd.ErrOrStderr(),
      "warning: auto reindex failed (run 'wikimind page reindex' manually): %v\n", rerr)
  ```

  同一文件存在两种风格，证明是疏忽而非有意。
- **违反**：
  - `.trellis/spec/backend/logging-guidelines.md` "错误到 stderr" + "进度/warning 走 stderr"。
- **修复方向**：把 line 763 的 `stdout` 换成 `cmd.ErrOrStderr()`。

---

### F-051 · `cmd/wikimind/review.go` 5 个子命令用 `context.Background()` 替代 `cmd.Context()`·【P2】

- **维度**：正确性 + 可维护性
- **严重度**：P2（review 子命令 Ctrl-C 不会取消 DB 查询；CLI cancellation 链路断裂）
- **位置**：
  - `cmd/wikimind/review.go:47` ReviewList
  - `cmd/wikimind/review.go:105` ReviewShow
  - `cmd/wikimind/review.go:204` ReviewAccept
  - `cmd/wikimind/review.go:277` ReviewReject
  - `cmd/wikimind/review.go:308` ReviewToday
- **证据**：

  ```go
  // review.go:204 - AcceptReview 是慢操作（git apply + commit + index update）
  RunE: func(cmd *cobra.Command, args []string) error {
      ...
      ctx := context.Background()  // ← 应是 cmd.Context()
      result, err := service.AcceptReview(ctx, vaultRoot, db, ...)
      ...
  }
  ```

  对照 `cmd/wikimind/command.go` 一致用 `cmd.Context()`（10+ 处）。
- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "context.Context 首参" + Required Patterns。
  - `.trellis/spec/backend/error-handling.md` "context 传递" 段。
  - Effective Go: "Never use context.Background() inside a function that has a context.Context parameter or is a CLI handler with one available".
- **修复方向**：5 处全改 `ctx := cmd.Context()`；run race tests 验证。

---

### F-052 · `cmd/wikimind/review.go:291` newReviewTodayCommand 自称"sorted by priority" 实际未排序·【P2】

- **维度**：正确性 + 契约稳定性
- **严重度**：P2（文档承诺 vs 行为不一致；用户期待按 priority 排但拿到 default insertion order）
- **位置**：
  - `cmd/wikimind/review.go:294-295`（Short 描述 "sorted by priority"）
  - `cmd/wikimind/review.go:309`（直接调 `ListReviewsByStatus`，没二次排序）
- **证据**：

  ```go
  cmd := &cobra.Command{
      Use:   "today",
      Short: "Show high-priority pending reviews (sorted by priority)",
      Args:  cobra.NoArgs,
      RunE: func(cmd *cobra.Command, args []string) error {
          ...
          reviews, err := index.ListReviewsByStatus(ctx, db, "pending")
          // ↑ ListReviewsByStatus 按 created_at 排，不按 priority
          if err != nil {
              return err
          }
          ...
          if limit > 0 && limit < len(reviews) {
              reviews = reviews[:limit]    // ← 直接截断
          }
          // 输出
      },
  }
  ```

- **背景**：
  - `internal/index/reviews.go` schema 里 `reviews.meta_json` 可存 priority 但当前列表 SQL `ORDER BY seq ASC` 不解析 meta。
  - "today" 命令完全等价于 `review list --status pending --limit 20`——但用户被 Short 误导。
- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "新代码有测试覆盖" + CLI 输出契约。
- **修复方向**：
  - 选 A：删除 "(sorted by priority)" 字样，明确说"by creation order"。
  - 选 B：在 service 层加 `ListPendingReviewsByPriority`，解析 meta_json 内的 priority，按"high → normal → low"再 created_at 二级排。

---

### F-053 · `cmd/wikimind/review.go:223,244` 静默吞 parse/append 错误，accept 后置任务失败无任何反馈·【P2】

- **维度**：错误处理 + 正确性
- **严重度**：P2（accept review 后追加 wiki/index.md 失败无任何 warning；用户看到 "accepted" 以为完事，实际 index.md 未更新）
- **位置**：
  - `cmd/wikimind/review.go:220-249`（accept 后置任务）
- **证据**：

  ```go
  // D14: append to wiki/index.md after accept (best-effort).
  for _, f := range result.Files {
      if strings.HasPrefix(f, "wiki/") && strings.HasSuffix(f, ".md") {
          ...
          page, parseErr := service.ParsePage(absPath)
          if parseErr != nil {
              continue       // ← 完全无 log，用户不知道
          }
          ...
          _ = service.AppendIndexEntry(ctx, vaultRoot, service.PageInfo{...})
          //  ↑ append 失败也不报
      }
  }
  ```

- **影响**：
  - parseErr 可能因 YAML 格式怪、文件权限、frontmatter 缺失而触发；用户不知道 → 后续 query 缺失。
  - AppendIndexEntry 错误更隐蔽：commit 已成，git 已记录，但 wiki/index.md 跑偏。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "不要静默吞错"。
  - logging-guidelines "warning 走 stderr"。
- **修复方向**：

  ```go
  if parseErr != nil {
      fmt.Fprintf(cmd.ErrOrStderr(), "warning: skip index entry for %s (parse: %v)\n", f, parseErr)
      continue
  }
  if err := service.AppendIndexEntry(ctx, vaultRoot, ...); err != nil {
      fmt.Fprintf(cmd.ErrOrStderr(), "warning: append index entry for %s failed: %v\n", f, err)
  }
  ```

---

### F-054 · `internal/lock/lock_test.go` 5 处用 `err != ErrLockXxx` 比较，应 `errors.Is`·【P2】

- **维度**：错误处理 + 测试质量
- **严重度**：P2（与 F-004/F-009/F-022 同类；当前 Manager.Release 等直接返回 sentinel 没 wrap，所以测试碰巧通过；但是一旦 Release 内部加 wrap 这些测试全炸）
- **位置**：
  - `internal/lock/lock_test.go:41,49,61,64,102`
- **证据**：

  ```go
  if err := m.Release("page-x", "sess-a"); err != ErrLockNotHeld {
      t.Fatalf("err = %v, want ErrLockNotHeld", err)
  }
  if err := m.Touch("page-1", "sess-b"); err != ErrLockNotMine { ... }
  if err := m.ForceRelease("page-1"); err != ErrLockNotHeld { ... }
  ```

- **违反**：
  - `.trellis/spec/backend/error-handling.md` "比较错误：永远 `errors.Is` / `errors.As`"。
  - 同 spec 第 5 段 "❌ 已踩过的坑：err == ErrX 直接比较"。
- **修复方向**：5 处全改 `if !errors.Is(err, ErrLockXxx)`；这一改可与 F-022 / F-004 合并成一个"测试错误断言风格统一" PR。

---

### F-055 · `cmd/wikimind/demo.go` 多处 `_ = os.Chdir(...)` 静默吞错 + ✅ emoji 渗 CLI 输出·【P2】

- **维度**：错误处理 + 跨平台 + 契约稳定性
- **严重度**：P2（demo 是用户首次接触命令，体验差；Chdir 失败后续 cd 全错，错误归因困难；emoji 在 Windows cmd.exe 默认 codepage 下乱码）
- **位置**：
  - `cmd/wikimind/demo.go:67,68,72,82,99,103`（6 处 `_ = os.Chdir(...)` 静默吞错）
  - `cmd/wikimind/demo.go:23,104`（🚀 / ✅ emoji 直接 print）
  - `cmd/wikimind/demo.go:91`（`_ = lintCmd.Execute()` 注释说"lint may return error, that's OK"——但 lint err 包括 DB 损坏，全 swallow）
- **证据**：

  ```go
  origDir, _ := os.Getwd()
  _ = os.Chdir(vaultPath)
  ...
  if err := ingestCmd2.Execute(); err != nil {
      _ = os.Chdir(origDir)
      return fmt.Errorf("demo ingest: %w", err)
  }
  ...
  fmt.Fprintf(stdout, "🚀 WikiMind 5-Minute Demo\n")
  ...
  fmt.Fprintf(stdout, "\n✅ Demo complete! Vault at: %s\n", vaultPath)
  ```

- **影响**：
  - Windows cmd.exe（GBK / 437 codepage）下，🚀 / ✅ 显示为乱码 `?` / `?`。
  - `os.Chdir` 失败（权限 / 路径异常）→ 后续命令在错误 CWD 执行，错误信息含混。
- **违反**：
  - `.trellis/spec/backend/logging-guidelines.md` "用户输出 ... 不要 ASCII art / 颜色码（除非用户开了 --color）；CI 解析友好优先"。
  - `.trellis/spec/backend/error-handling.md` "禁止 `_ =` 静默吞错（清理操作除外）"。
- **修复方向**：
  - Chdir 错误至少 `fmt.Fprintf(stderr, "warning: chdir failed: %v\n", err)`。
  - emoji 改用纯 ASCII：`"WikiMind 5-Minute Demo"`、`"Demo complete!"`。或加 `--ascii-only` flag，默认 emoji（Mac/Linux 多数 OK），出问题用户可关。
  - 同 F-049 把 ✓/✗ 在 doctor 命令里也搞 ascii fallback（line 673-723）。

---

### F-056 · `internal/worktree.inferAgentSession` 与 `vault.DefaultAllowedAgents` agent 列表不一致·【P2 / Spec-Drift】

- **维度**：契约稳定性 + 可维护性 + Spec-Drift
- **严重度**：P2（worktree 解析接受的 agent 与 handshake 接受的 agent 不对齐；解析路径"包容"hermes/custom 但 handshake 又拒绝它们）
- **位置**：
  - `internal/worktree/worktree.go:188`（硬编码 7 个 agent）
  - `internal/vault/config.go:33-35`（DefaultAllowedAgents 返回 5 个）
- **证据**：

  ```go
  // worktree/worktree.go:188
  for _, agent := range []string{"claude-code", "codex-cli", "opencode", "cursor", "cline", "hermes", "custom"} {
      ...
  }

  // vault/config.go:33-35
  func DefaultAllowedAgents() []string {
      return []string{"claude-code", "codex-cli", "cursor", "cline", "opencode"}
  }
  ```

- **影响**：
  - 7 vs 5 不对称；handshake 拒绝 "hermes" agent，但若以前手动建过 wt-hermes-* 分支，listWorktrees 仍能解析出来；造成"有 ghost worktree 显示"的混乱。
  - 双方都是硬编码 → 任何新 agent 须同时改两处。
- **违反**：
  - `.trellis/spec/backend/directory-structure.md` "model 暂存共享类型"——agent 列表应是共享常量。
  - DRY 原则。
- **修复方向**：
  - worktree.inferAgentSession 改用 `vault.DefaultAllowedAgents()`，删除自己的硬编码 list。
  - 或抽出 `internal/agent/registry.go` 统一注册中心，vault + worktree + mcp 都从这里读。

---

### F-057 · 两个 main 入口版本号不一致：`wikimind` 报 0.1.0-d1，`wikimindd` 报 0.1.0-dev·【P2】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P2（同 v0.1.0 发布的 2 个 binary，自报版本不同；用户 `wikimind --version` 见 "0.1.0-d1"，`wikimindd --version` 见 "0.1.0-dev"，bug report 时来源混乱）
- **位置**：
  - `cmd/wikimind/main.go:8`（var version = "0.1.0-d1"）
  - `cmd/wikimindd/main.go:16`（const version = "0.1.0-dev"）
- **证据**：

  ```go
  // cmd/wikimind/main.go:8
  var version = "0.1.0-d1"

  // cmd/wikimindd/main.go:16
  const version = "0.1.0-dev"
  ```

- **背景**：
  - "0.1.0-d1" 显然是 W1 D1 阶段的 placeholder，2026-05-25 v0.1.0 release 时未更新。
  - "0.1.0-dev" 是 daemon 入口（D14 引入），用 "-dev" 较 conventional。
  - 同 release 的两个 binary 应该自报同一 release version（如 `0.1.0`）或同一 dev tag。
- **违反**：
  - 行业共识：semver `<major>.<minor>.<patch>-<prerelease>` 同 release 两 binary 应一致；CI/CD 通常注入 build tag。
  - `.trellis/spec/backend/quality-guidelines.md` "CLI 输出格式没破坏已有断言"——version string 是 CLI 契约。
- **修复方向**：
  - 短期：两处都改成 `"0.1.0"`（已发布）或 `"0.1.0-dev"`（未发布）；保持一致。
  - 长期：用 `-ldflags "-X main.version=..."` 在 build 时注入，避免源码 const 漂移。

---

### F-058 · `cmd/wikimind/command.go` 多个 `out, _ := exec.Command("python3", "--version").Output()` 静默吞错·【P2】

- **维度**：错误处理 + 正确性
- **严重度**：P2（doctor 显示版本时如果 Output 返回 err 但 out 非空（比如 stderr 输出），仍会以"成功"渲染：✓ git: <空字符串>）
- **位置**：
  - `cmd/wikimind/command.go:676`（git --version）
  - `cmd/wikimind/command.go:685`（python3 --version）
- **证据**：

  ```go
  if _, err := exec.LookPath("git"); err != nil {
      fmt.Fprintf(stdout, "✗ git: not found\n")
      allOK = false
  } else {
      out, _ := exec.Command("git", "--version").Output()        // ← err 丢
      fmt.Fprintf(stdout, "✓ git: %s\n", strings.TrimSpace(string(out)))
      // ↑ 若 Output 返回 err（git exits non-zero），仍打 "✓ git: <empty>"
  }
  ```

- **影响**：
  - 极小概率（git --version 不会 fail），但写法掩盖意图。
  - 同样 python3 --version 在某些 wrapper 会 exit 1（如 conda env 未激活时打 message 到 stderr 后 exit 1）。
- **违反**：
  - `.trellis/spec/backend/error-handling.md` "禁止：`_, _ = doSomething()` 把错误丢掉"。
- **修复方向**：

  ```go
  out, err := exec.Command("git", "--version").Output()
  if err != nil {
      fmt.Fprintf(stdout, "✗ git: version probe failed: %v\n", err)
      allOK = false
  } else {
      fmt.Fprintf(stdout, "✓ git: %s\n", strings.TrimSpace(string(out)))
  }
  ```

---

### F-059 · `cmd/wikimind/command.go` 单文件按 RunE 行数粒度分析——18 个子命令至少 3 个超 60 行·【P1 关联 F-005】

- **维度**：可维护性
- **严重度**：P1（与 F-005 关联，本条为细粒度数据补充）
- **位置**：
  - `cmd/wikimind/command.go:663-735` newDoctorCommand（72 行函数包括 RunE 大体 60+）
  - `cmd/wikimind/command.go:583-645` newRevertCommand（62 行包括 RunE 50+）
  - `cmd/wikimind/command.go:254-314` newIngestCommand（60 行包括 RunE 50+）
- **细粒度数据**：

  | 函数 | 总行 | RunE 大致行 | 复杂度估计 |
  |------|------|-----------|----------|
  | newDoctorCommand | 72 | 60 | 4 个 if-else 层 |
  | newRevertCommand | 62 | 50 | 4 步串行（reverse + commit + log）|
  | newIngestCommand | 60 | 45 | ingest + 自动 reindex |
  | newMcpServeCommand | 47 | 38 | logger/server/runStdio 3 段 |
  | newLintCommand | 45 | 35 | findings + summary print |
  | newQueryCommand | 44 | 32 | 3 输出格式分支 |
  | newReviewAcceptCommand | 78 | 65 | accept + 后置 index 追加 |

- **建议拆分**：
  - newDoctorCommand 拆成 `checkGit() / checkPython() / checkVault() / checkVaultDirs()`，每函数 < 20 行。
  - newReviewAcceptCommand 把 D14 后置 index 追加（review.go:219-250）抽到 service 层 `service.PostAcceptIndexUpdate`。
- **违反**：`.trellis/spec/backend/quality-guidelines.md` "Code Review Checklist · 结构"。
- **修复方向**：v0.2 重构期统一处理，配合 F-005 拆分 command.go 进 commands/ 子目录。

---

### F-060 · `cmd/wikimindd/main.go` 入口 daemon init 失败时未关闭已分配资源·【P2】

- **维度**：资源管理 + 错误处理
- **严重度**：P2（入口程序，失败后 `os.Exit(1)` 会让 OS 兜底回收；不致命；但 `daemon.New` 内部可能已经打开过 DB / 日志文件——退出前显式 Close 会更干净，便于未来注入诊断 hook）
- **位置**：`cmd/wikimindd/main.go:29-33`
- **证据**：

  ```go
  d, err := daemon.New(cfg)
  if err != nil {
      fmt.Fprintf(os.Stderr, "daemon init: %v\n", err)
      os.Exit(1)
  }
  ```

  `daemon.New` 内部在 `internal/daemon/loop.go:33-72` 会按序打开 log file、index DB、watcher；任一步失败前面已分配资源未必由 `New` 的内部 rollback 完全回收。
- **违反**：行业共识（Effective Go：构造函数失败应清理已分配资源，避免半成品 leaked）；与 `.trellis/spec/backend/quality-guidelines.md` "Required: 资源管理" 同精神。
- **修复方向**：
  - `daemon.New` 内部保证 partial cleanup（如果尚未做），或入口处 `defer d.Shutdown()` 兜底（但 d 此时为 nil）。
  - 在 `daemon.New` 失败路径加单测覆盖：注入 `index.Open` 失败 → 验证 logFile 已关闭。

---

### F-061 · `cmd/wikimindd/main.go` 启动 banner 用 `fmt.Fprintf` 直接 stderr 而非走 daemon logger·【P2】

- **维度**：可维护性 + 契约稳定性
- **严重度**：P2（daemon 内部已用 `log.Logger` 写 stderr 带前缀 + 时间戳；入口 banner 走 raw fmt.Fprintf 风格不一致，日志聚合时这一行没有时间戳难以与 daemon 内消息对齐）
- **位置**：`cmd/wikimindd/main.go:38`
- **证据**：

  ```go
  fmt.Fprintf(os.Stderr, "wikimindd %s starting (vault=%s)\n", version, vaultRoot)
  ```

  对比 `internal/daemon/loop.go:62-65` 内 logger：`d.logger.Printf("started vault=%s", cfg.VaultRoot)` 输出 `2026/05/28 10:15:00 [daemon] started vault=...`（带 prefix + 时间戳）。
  入口 banner 输出 `wikimindd 0.1.0-dev starting (vault=/tmp/v)`（无时间戳），与 daemon 内日志风格不一致。
- **违反**：`.trellis/spec/backend/logging-guidelines.md` "Daemon / MCP Server Logging" 段——"运维部分（启动、关停、健康检查、内部状态）→ `log/slog` 写 stderr，结构化"。
- **修复方向**：
  - 把 banner 改为传给 `daemon.New` 的 logger 输出（让 daemon 内部首条日志做 banner），统一格式。
  - 或入口 banner 也用 `log.New(os.Stderr, "[wikimindd] ", log.LstdFlags).Printf(...)`。

---

### F-062 · `cmd/wikimindd` 无 `--version` flag，与 wikimind CLI 不一致·【P2】

- **维度**：契约稳定性 + 可维护性
- **严重度**：P2（用户排查问题时 `wikimindd --version` 会 panic with "flag provided but not defined"，不像 `wikimind --version` 那样 cobra 自动支持）
- **位置**：`cmd/wikimindd/main.go:19-21`
- **证据**：

  ```go
  var vaultRoot string
  flag.StringVar(&vaultRoot, "vault", "", "Path to WikiMind vault")
  flag.Parse()
  // 只声明了 --vault，没有 --version / -v / --help
  ```

  banner（line 38）会在每次启动打 version，但用户无法直接查询版本。
- **违反**：CLI 契约一致性（同发布的 2 个 binary 应有同等 introspection 能力）；行业共识（任何 CLI 工具都应支持 `--version`）。
- **修复方向**：
  - 加 `var showVersion bool; flag.BoolVar(&showVersion, "version", false, "Print version and exit")`；`if showVersion { fmt.Println(version); os.Exit(0) }`。
  - 或直接换成 cobra 与 wikimind 风格一致（version 字段 + auto-gen `--version`），但当前 38 行简单度可保留。

---

### F-063 · `verify/ipc/main.go` 拉起子进程后未做超时保护——server 端 Accept 失败时 client 可能成为孤儿·【P2】

- **维度**：资源管理 + 测试质量
- **严重度**：P2（verify 是手工跑的"打通验证"脚本，出错时偶发悬挂；不阻塞 v0.1.0 发布但污染 dev 体验）
- **位置**：`verify/ipc/main.go:44-71`
- **证据**：

  ```go
  cmd := exec.Command(os.Args[0], "client")
  cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
  if err := cmd.Start(); err != nil {
      fmt.Println("✗ spawn client 失败:", err)
      os.Exit(1)
  }

  _ = ln.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
  conn, err := ln.Accept()
  if err != nil {
      fmt.Println("✗ accept 失败:", err)
      os.Exit(1)            // ← 直接 Exit，client 子进程没 Kill
  }
  ```

- **问题**：Accept 失败时 `os.Exit(1)` 不等于父进程 cleanup；spawned client 子进程仍在 retry Dial（100 次 × 20ms = 2s），父退出后变孤儿（macOS/Linux 通常被 init 回收，但 dev 工作流中会看到 stray process）。
- **违反**：行业共识（Effective Go：spawn child 时配 defer Kill）；`.trellis/spec/backend/quality-guidelines.md` "资源管理"。
- **修复方向**：
  - 在 spawn 后立刻 `defer func() { _ = cmd.Process.Kill() }()`；正常 cmd.Wait 成功后该 Kill 会变 no-op。
  - 或封一个 `runVerify(ctx, ...) error` 用 ctx cancel 控制 client 退出。

---

### F-064 · `verify/ipc/main.go` JSON unmarshal 错误全部 `_ = json.Unmarshal(...)` 静默吞·【P2】

- **维度**：错误处理 + 测试质量
- **严重度**：P2（验证脚本本身用静默 unmarshal 会让"server 端发出 garbage 也判 echo 通过"，验证语义被弱化）
- **位置**：
  - `verify/ipc/main.go:63`（server 端解析 client 请求）
  - `verify/ipc/main.go:64,65`（resp marshal + write 都吞错）
  - `verify/ipc/main.go:91`（client 端构造 req）
  - `verify/ipc/main.go:102`（client 端解析 echo 响应）
- **证据**：

  ```go
  var m msg
  _ = json.Unmarshal(line, &m)        // ❌ 解析失败时 m.Text="" 仍走下去
  resp, _ := json.Marshal(msg{...})    // ❌ 几乎不会失败但风格不一致
  _, _ = conn.Write(append(resp, '\n')) // ❌ write 失败也吞
  ```

- **违反**：
  - `.trellis/spec/backend/error-handling.md` "禁止：`_, _ = doSomething()` 把错误丢掉"。
  - 验证脚本的本职就是"测试通信"——unmarshal 错误本就是验证目标之一。
- **修复方向**：
  - 至少把 server 端 `json.Unmarshal` 错检入 `if err := json.Unmarshal(...); err != nil { fmt.Println("✗ unmarshal:", err); os.Exit(1) }`。
  - 同 verify/mcp/main.go 已经把 ListTools / CallTool 错都检查了，verify/ipc 应风格一致。

---

### F-065 · `verify/ipc/main.go` socket 路径 `wm-ipc-verify.sock` 在 `/tmp` 与未来 daemon 实际路径不一致·【P2】

- **维度**：契约稳定性 + 测试质量
- **严重度**：P2（与 F-041 关联——verify/ipc 用 `/tmp/wm-ipc-verify.sock` 验证通信，但 daemon 实际用 `<vault>/.wikimind/daemon.sock`（Linux/macOS）或 Named Pipe（Windows，已坏）；验证场景未覆盖真实路径）
- **位置**：`verify/ipc/main.go:23`
- **证据**：

  ```go
  func sockPath() string { return filepath.Join(os.TempDir(), "wm-ipc-verify.sock") }
  ```

  对照 `internal/bridge/bridge.go:14-22` 真实 SocketPath。
- **问题**：
  - 验证"Unix socket IPC 跑通"是对的，但跨平台陷阱（Windows Named Pipe vs Unix socket）未在 verify 中暴露。
  - 当 F-041 修好后，verify/ipc 应跟进调用 `bridge.SocketPath` 而不是自造路径，保证 verify 真正预演生产路径。
- **违反**：`.trellis/spec/backend/quality-guidelines.md` "测试用真实依赖：不 mock `os` / `git` / `database/sql`；用 `t.TempDir()` 起隔离环境"——verify 是端到端验证，应跑真实 SocketPath。
- **修复方向**：
  - 改用 `bridge.SocketPath(t.TempDir())`（需重构成可调用 bridge 包的 main，加 vault root flag）。
  - 或保留独立路径但在 verify/ipc README 注明"仅验证 Unix socket primitives，不覆盖跨平台 socket 路径策略"。

---

### F-066 · `verify/fts5/main.go` 用 `err == nil && n > 0` 判命中而不分流真实 error·【P2】

- **维度**：错误处理 + 测试质量
- **严重度**：P2（FTS5 query 失败（语法错误 / DB 异常）会被当作"未命中"。当前用例硬编码，FTS5 syntax 都是合法的，未爆雷；但任何未来加入"边界 query"的 case 会被静默隐藏）
- **位置**：`verify/fts5/main.go:48-49`
- **证据**：

  ```go
  got := err == nil && n > 0     // ❌ err != nil 时 got=false → 与 want=false 一致就 OK
  st := "OK"
  if got != want {
      st = "FAIL"
      pass = false
  }
  ```

- **违反**：测试本质——"未命中" vs "查询失败" 是两种不同的失败模式，不应折叠。
- **修复方向**：
  - 把 err 单独输出：`if err != nil { fmt.Printf("  [FAIL] %q ERROR: %v\n", q, err); pass = false; continue }`。
  - 然后再判 `got=n>0`。

---

### F-067 · `verify/fts5/main.go` 用 `must2(_ any, err error)` helper 折叠两个返回值非标准用法·【P2】

- **维度**：可维护性
- **严重度**：P2（`must2` 用变长参数模拟 (any, error) → must 失败时丢失 `any` 部分；GoMacro/Effective Go 风格指南不建议，但脚本场景可接受）
- **位置**：`verify/fts5/main.go:77`
- **证据**：

  ```go
  func must2(_ any, err error) { must(err) }
  ```

- **影响**：
  - 调用 `must2(db.Exec(...))`：Exec 返回 (sql.Result, error)，第一参数被丢；测试脚本不需要 sql.Result，但模式让 helper 数量随返回值数增加。
- **违反**：Effective Go: "Don't write code just because Go lacks a feature — write idiomatic Go"。
- **修复方向**：
  - 移除 must2，调用处改成 `if _, err := db.Exec(...); err != nil { must(err) }`。
  - 或直接 `_, err := db.Exec(...); must(err)`。
  - 当前 78 行的脚本接受现状即可，本条为 FYI。

---

### F-068 · `verify/` 3 个 main 都用 `os.Exit(1)` 在 deferred cleanup 之前——defer 不执行·【P2】

- **维度**：资源管理 + 跨平台
- **严重度**：P2（影响：socket 文件、子进程、DB 连接等未必清理；脚本短跑 + tmp 目录的话 OS 兜底，但行为不规范，模式不应蔓延到生产代码）
- **位置**：
  - `verify/fts5/main.go:64`（`defer db.Close()` 之后 `os.Exit(1)` 不执行 Close）
  - `verify/ipc/main.go:38-42`（`defer ln.Close(); defer os.Remove(sock)` 之后 Exit）
  - `verify/mcp/main.go:42-43`（`defer session.Close()` 之后 Exit）
- **证据**：

  ```go
  // verify/fts5/main.go
  defer db.Close()
  ...
  if !pass {
      fmt.Println("✗ 验证项 1 失败 — fallback 到 mattn/go-sqlite3（cgo）")
      os.Exit(1)        // ← defer db.Close() 不执行
  }
  ```

- **违反**：行业共识（Effective Go：`os.Exit` 不跑 deferred function）。
- **修复方向**：
  - 把 verify main 改成 wrapper：`func main() { os.Exit(run()) }`；`run` 用 `return 1` 让 deferred 跑。
  - 这是 Go CLI 推荐 pattern；3 个 verify 应统一改造。

---

### F-069 · `worker/main.py` doc string 自承"W0 skeleton" 与归档 D13 "PDF skeleton OK"不一致·【P1 关联 F-003】

- **维度**：契约稳定性 + 测试质量
- **严重度**：P1（与 F-003 同根；这里补充更细证据：pyproject.toml dependencies 为空——没声明 pypdf / Pillow / pytesseract 等 D13 应当依赖的库）
- **位置**：
  - `worker/main.py:1-7`（顶部 doc string 自承"W0 skeleton"）
  - `worker/pyproject.toml:6`（`dependencies = []`）
  - 全仓 grep 显示 worker/main.py 无 Go 侧调用方
- **证据**：

  ```toml
  # worker/pyproject.toml
  [project]
  name = "wikimind-worker"
  version = "0.0.1"
  description = "WikiMind ingest worker — parse / OCR / transcribe"
  requires-python = ">=3.11"
  dependencies = []          # ← description 说"OCR / transcribe"但无依赖
  ```

  ```python
  # worker/main.py:1-5
  """WikiMind ingest worker — W0 skeleton.

  从 stdin 读一行任务 JSON，向 stdout 输出 NDJSON 事件流。
  协议见 spec-v2/docs/engineering-decisions.md §1。
  完整 parser（markdown / html / pdf / image / audio）在 roadmap D13 实现。
  """
  ```

- **关联 F-003**：F-003 已识别 doctor 命令检查 pypdf 误导用户；本条补充 pyproject 层面证据：连"声明依赖"都没做，更不用说装。
- **违反**：
  - PEP 621（pyproject.toml）— description 与 dependencies 应一致：声明"OCR / transcribe"就应有 `pillow` / `pytesseract` 等依赖（或在 dependency-groups 里 staged）。
  - F-003 同 spec 违反链。
- **修复方向**（不在本任务）：
  - 短期：pyproject.toml description 改"WikiMind ingest worker — W0 skeleton (D13 PDF / image / audio parsers staged)"；dependencies 仍 `[]` 直到真正实现。
  - 中期：实现 PDF 时一并落 `dependencies = ["pypdf>=4.0,<5"]` + version pinning。

---

### F-070 · `worker/pyproject.toml` 无 author / license / readme / repository / classifiers·【P2】

- **维度**：可维护性 + 契约稳定性
- **严重度**：P2（PEP 621 推荐字段——长期看影响 `pip install`、PyPI 发布、SBOM 工具识别；当前 worker 仅本机 spawn，不是 PyPI 包，可宽容但建议补齐）
- **位置**：`worker/pyproject.toml:1-10`
- **证据**：

  ```toml
  [project]
  name = "wikimind-worker"
  version = "0.0.1"
  description = "WikiMind ingest worker — parse / OCR / transcribe"
  requires-python = ">=3.11"
  dependencies = []
  # ❌ 无 authors / license / readme / homepage / repository / keywords / classifiers
  ```

- **违反**：PEP 621 推荐实践；行业共识（标准 pyproject.toml 至少有 license + authors）。
- **修复方向**：
  - 补 `authors = [{name = "fengxd", email = "..."}]`、`license = {text = "Apache-2.0"}`（与项目其他位置一致）、`readme = "README.md"` 占位。

---

### F-071 · `worker/main.py` empty task 错误未带 task_id 字段·【P2】

- **维度**：契约稳定性
- **严重度**：P2（其他 print 都带 `task_id`；empty task 路径输出 `{"type": "error", "message": "empty task"}` 缺 task_id；agent 端追踪日志按 task_id 索引时会丢这条）
- **位置**：`worker/main.py:14-18`
- **证据**：

  ```python
  def main() -> int:
      line = sys.stdin.readline()
      if not line.strip():
          print(json.dumps({"type": "error", "message": "empty task"}))   # ← 无 task_id
          return 1
      try:
          task = json.loads(line)
      except json.JSONDecodeError as exc:
          print(json.dumps({"type": "error", "message": f"invalid task json: {exc}"}))  # ← 同样
          return 1

      task_id = task.get("task_id", "")
      print(json.dumps({"type": "progress", "task_id": task_id, ...}))
  ```

- **违反**：协议契约一致性——所有 NDJSON 事件应有同样 envelope 字段；agent 端按 task_id 解码会因缺字段降级。
- **修复方向**：
  - empty task / parse error 路径输出 `{"type": "error", "task_id": "", "message": "..."}`，保持 envelope 一致。

---

### F-072 · `worker/main.py` 缺 `KeyboardInterrupt` / `BrokenPipeError` 处理——Ctrl-C / 上游关 stdin 时栈跟踪喷 stderr·【P2】

- **维度**：错误处理 + 跨平台
- **严重度**：P2（W3+ daemon spawn worker 后用 Ctrl-C 取消任务时，worker stderr 会有 Python traceback；不影响功能但污染日志）
- **位置**：`worker/main.py:36-37`
- **证据**：

  ```python
  if __name__ == "__main__":
      sys.exit(main())
  # ❌ 无 try/except KeyboardInterrupt
  # ❌ 无 try/except BrokenPipeError（daemon 关 stdin pipe 时）
  ```

- **违反**：Python 工程实践（CPython 文档推荐：CLI 入口包 try/except KeyboardInterrupt）。
- **修复方向**：

  ```python
  if __name__ == "__main__":
      try:
          sys.exit(main())
      except KeyboardInterrupt:
          sys.exit(130)   # standard exit code for Ctrl-C
      except BrokenPipeError:
          sys.exit(0)     # upstream closed pipe, treat as clean exit
  ```

---

### F-073 · `worker/pyproject.toml` 仅声明 ruff，未声明 mypy / pytest / black·【P2】

- **维度**：测试质量 + 可维护性
- **严重度**：P2（无 mypy 配置 → type hints 未被强制验证（F-072 之类的遗漏靠人眼）；无 pytest 依赖 → 没有任何 Python 单测；与 worker/main.py 标 D13 PDF 实现的承诺脱节）
- **位置**：`worker/pyproject.toml:8-10`
- **证据**：

  ```toml
  [tool.ruff]
  line-length = 100
  target-version = "py311"
  # ❌ 无 [tool.mypy]、[tool.pytest.ini_options]、[dependency-groups]
  ```

- **违反**：
  - `.trellis/spec/backend/quality-guidelines.md` "测试覆盖真实路径"——Python 侧零测试。
  - PEP 8 / Python 工程实践——type hints 应配 mypy。
- **修复方向**：
  - 加 `[tool.mypy]` strict 模式 + `[tool.pytest.ini_options]`；
  - 加 `[dependency-groups.dev] = ["pytest>=7", "mypy>=1.5"]`；
  - 加 `worker/tests/test_main.py` 至少 3 case（empty stdin / invalid json / valid task）。

---

### F-074 · 4 个空壳 internal 包确认完全无人 import·【FYI / 补强 F-002】

- **维度**：可维护性
- **严重度**：—（本条是 F-002 的补充证据，最终归并到 F-002 处置；不算独立 finding）
- **位置**：
  - `internal/changelog/doc.go`
  - `internal/git/doc.go`
  - `internal/model/doc.go`
  - `internal/worker/doc.go`
- **证据**：

  ```bash
  $ grep -rn 'fengxd1222/llm-wiki/internal/changelog\|fengxd1222/llm-wiki/internal/git\|fengxd1222/llm-wiki/internal/model\|fengxd1222/llm-wiki/internal/worker' --include="*.go"
  (no matches)
  ```

  4 个包零导入：既无生产代码引用，也无测试代码引用。`go build ./internal/...` 仍通过，因为每个包都有合法的 `package <name>` 声明 + 任何代码都能定义空包，编译器不报"unused package"（Go 没有这种警告）。
- **结论**：
  - 比 F-002 更强的事实证据——这 4 个包**纯粹**是占位符，零 import、零调用、零代码。
  - 任何"删除"操作都不会触发任何编译错误。
  - 与 F-S002 联动：trellis-update-spec 应同时清 `.trellis/spec/backend/directory-structure.md` 中"Directory Layout"对这 4 个目录的描述。
- **修复方向**（合并到 F-002 / F-S002 一起处理）：
  - 删除 4 个空壳目录；spec 同步删；相关说明合并到真实实现包的 doc.go（如 `internal/commit/doc.go` 已自述包含 changelog 实现）。

---

### F-075 · 依赖安全离线核对（10 个有/无已知风险条目）·【按行细分】

- **维度**：安全 + 依赖管理
- **严重度**：见下表逐行
- **位置**：`go.mod:1-37`、`worker/pyproject.toml:6`
- **方法**：
  - 不联网；仅基于"版本号 + 已知公开 CVE 模式"做离线核对。
  - 真实结论需用户后续跑 `govulncheck ./...`（建议在 CI smoke 加这一步）。
- **核对表**：

| # | 依赖 | 版本 | 已知模式风险 | 严重度 | 离线判断 / 建议 |
|---|------|------|-------------|--------|-----------------|
| 1 | `github.com/BurntSushi/toml` | v1.4.0 | 无已知 CVE；v1.4.x 是 2024 稳定线 | — | OK |
| 2 | `github.com/modelcontextprotocol/go-sdk` | v1.6.1 | 新 SDK（< 2 年），CVE 数据库覆盖少；MCP 协议本身的安全性依赖 host 实现 | — | OK；建议跟进 upstream release notes |
| 3 | `github.com/pressly/goose/v3` | v3.27.1 | 历史 CVE：goose CLI 早期版本（v2.x）有 SQL 文件路径遍历，v3.x 重写后无已知 CVE | — | OK |
| 4 | `github.com/spf13/cobra` | v1.10.2 | 无已知 CVE；广泛使用 | — | OK |
| 5 | `github.com/yuin/goldmark` | v1.8.2 | 历史 CVE-2024-31451（XSS via raw HTML，已修于 v1.7.4）；当前 v1.8.2 已修复 | — | OK，已修复线 |
| 6 | `gopkg.in/yaml.v3` | v3.0.1 | 历史 CVE-2022-28948（DoS via aliases），v3.0.1 是 fix 后的稳定线；后续无新 CVE 公开 | — | OK |
| 7 | `modernc.org/sqlite` | v1.50.1 | 历史早期版本有 memory bug；v1.30+ 起稳定；v1.50.1 在 2024-2025 区间 | — | OK；建议跟踪 v1.5x → v1.6x release notes |
| 8 | `github.com/fsnotify/fsnotify` | v1.10.1 | 无已知 CVE；v1.10.x 是稳定线 | — | OK |
| 9 | `github.com/google/uuid` | v1.6.0 | 无已知 CVE；UUID v6/v7 实现稳定 | — | OK |
| 10 | `golang.org/x/oauth2` | v0.35.0 | 历史 CVE-2025-22868（OAuth2 PKCE bypass，已修于 v0.27+）；v0.35.0 已修复 | — | OK，已修复线 |
| 11 | `golang.org/x/sync` | v0.20.0 | 无 CVE | — | OK |
| 12 | `golang.org/x/sys` | v0.43.0 | 无 CVE；纯 syscall 包 | — | OK |
| 13 | `wikimind-worker (Python)` | 0.0.1 | dependencies=[]，无外部依赖即无 CVE 暴露面 | — | OK（但与 F-069 关联：声明"OCR / transcribe"但零依赖，能力声明虚假）|

- **总体结论**：**无已知 CVE 命中**。所有列出的依赖版本都在或晚于已公开 CVE 的修复线。
- **限制**：
  - 离线核对受训练时间窗约束，**不能覆盖 2025-2026 新发布的 CVE**。
  - 强烈建议用户后续跑 `govulncheck ./...`（Go 官方工具，连 OSV 数据库）做最终验证。
  - CI smoke 应加 `govulncheck ./...` step，落到 v0.1.1 patch 内做。
- **违反**：本条非违反，是"未覆盖的检查"——`.trellis/spec/backend/quality-guidelines.md` 的 "测试覆盖真实路径" 章节未明确要求 SCA（Software Composition Analysis），属可补强项。
- **修复方向**：
  - **v0.1.1**：CI 加 `govulncheck` 卡点；每次 PR 跑一次。
  - **v0.2**：定期 `go mod tidy` + `go list -m -u all` 检查可升版本；订阅依赖仓库 release。

---

## 📐 Spec-Drift 汇总

> 代码实现 vs `.trellis/spec/backend/` 描述不一致，但代码更优或更准确——供后续 trellis-update-spec 参考。

### 纯 Spec-Drift（无代码改动需求）

| ID | 标题 | trellis-update-spec 目标文件 |
|----|------|------------------------------|
| F-S001 | daemon + mcp serve 已在 v0.1.0 使用 `log` 包 | `.trellis/spec/backend/logging-guidelines.md`（"将来时"改"现状描述"）|
| F-S002 | 4 空壳包应从 directory-structure.md 删 | `.trellis/spec/backend/directory-structure.md`（与 F-002 / F-074 联动）|

### 双标签 Spec-Drift（代码 + spec 都要动）

| ID | 标题 | 代码侧动作 | Spec 侧动作 |
|----|------|------------|-------------|
| F-014 | service/review.go 直接 git 子进程绕过 commit 包 | F-008 / F-014 / F-034 一并收归 `internal/git` 或 `internal/commit` | 同步更新 directory-structure.md |
| F-018 | search FTS5 query 不 sanitize | 实现 `sanitizeFTS5Query` 或 `--raw-fts` flag | database-guidelines.md FTS5 段补"高级 query"说明 |
| F-025 | MCP 协议错误码 inline 创建 | tools.go 加包级 sentinel | quality-guidelines.md W2 D11 spec-entry 强调 sentinel 化 |
| F-026 | MCP 工具 17 个 vs spec 写 15 | （代码已对，不动）| quality-guidelines.md W2 D11 spec-entry：15→17，write 6→8 |
| F-027 | change-log op 字面量 `append_log` vs spec `log_append` | tools.go:492 + tools_test.go:605 改 `log_append` | （spec 已对，不动）|
| F-031 | RateLimits 宣称但未强制 | （短期 spec 改 staged，长期实施 enforcement）| quality-guidelines.md W2 D10 spec-entry 加 staged note |
| F-034 | runVaultGit 是第 4 个 git wrapper | 同 F-008 收归 | directory-structure.md 同步 |
| F-045 | bridge SocketPath 注释与代码矛盾 | 同 F-041 修 Windows + 改注释 | （spec-v2 增补 IPC socket naming）|
| F-048 | command.go 硬编码 "15 tools" | 改成 `server.ToolCount()` 动态 | 与 F-026 同步 |
| F-049 | CLI lint "8 rules" / 代码 5 / spec 13 | CLI 动态 / 删数字 | spec-v2 同步真实数 |
| F-056 | worktree agent list 7 vs vault.DefaultAllowedAgents 5 | worktree 改用 vault.DefaultAllowedAgents | 抽 `internal/agent/registry.go` 收容 |

### Spec-Drift 一句话总结

13 条 Spec-Drift（2 纯 + 11 双标签）由 `trellis-update-spec` 一次性收，零代码改动可做 1 个 spec-only PR；代码侧的双标签条目随 v0.1.1 / v0.2 修复 PR 同步落地，spec 跟进。

---

## 附录 A · 审查方法

- **阶段 1 预扫**：`go build` / `go vet` / `go test` / `go test -race` 工具层基线。
- **阶段 2 包级深扫**：按依赖底层→上层逐包（vault → commit → index → service → proposal → worktree → mcp → 其他）。
- **阶段 3 横切扫描**：跨包维度 grep（path traversal / SQL 拼接 / goroutine 启动 / 字面量错误码 / 跨平台路径 / panic）。
- **阶段 4 Python + 依赖**：worker/ PEP 8 / type hints / 异常处理；go.mod/go.sum 版本核对。
- **阶段 5 综合**：分级、统计、推荐结论。

## 附录 B · 比对锚点

每条 finding 在"违反"字段引用以下之一：
- `.trellis/spec/backend/quality-guidelines.md` 某条款（含 7 个 `<spec-entry>`）
- `.trellis/spec/backend/{directory,database,error,logging}-guidelines.md` 某条款
- 行业共识（PEP 8 / Effective Go / CWE）+ 来源标注

## 附录 C · 审查覆盖完成度

### 全量覆盖（逐文件 Read）

| 范围 | 实现行数 | 测试行数 | 覆盖深度 |
|------|---------|---------|---------|
| `cmd/wikimind/` | 1448 | 886 | 全量 |
| `cmd/wikimindd/` | 38 | 0 | 全量 |
| `internal/vault/` | 1504 | 844 | 全量 |
| `internal/commit/` | 1015 | 363 | 全量 |
| `internal/index/` | 2915 | 1288 | 全量 |
| `internal/service/` | 3600 | 1637 | 全量 |
| `internal/proposal/` | 561 | 208 | 全量 |
| `internal/worktree/` | 429 | 162 | 全量（含测试） |
| `internal/mcp/` | 4164 | 1821 | 全量（4 文件） |
| `internal/lock/` | 356 | 162 | 全量 |
| `internal/lint/` | 408 | 163 | 全量 |
| `internal/watcher/` | 246 | 119 | 全量 |
| `internal/bridge/` | 197 | 112 | 全量 |
| `internal/daemon/` | 224 | 74 | 全量 |
| `internal/schema/` | 73 | 36 | 全量 |
| `internal/changelog/` | 2 (空壳) | 0 | 全量（确认空） |
| `internal/git/` | 2 (空壳) | 0 | 全量（确认空） |
| `internal/model/` | 2 (空壳) | 0 | 全量（确认空） |
| `internal/worker/` | 2 (空壳) | 0 | 全量（确认空） |
| `verify/fts5/` | 77 | 0 | 全量 |
| `verify/ipc/` | 108 | 0 | 全量 |
| `verify/mcp/` | 72 | 0 | 全量 |
| `worker/main.py` | 37 | 0 | 全量（Python 视角）|
| `worker/pyproject.toml` | 10 | — | 全量（PEP 621 核对）|

### 横切扫描（grep / find 全仓）

- ✅ `panic(` —— 业务代码无（仅 init 注册驱动）
- ✅ SQL 拼接 —— 无注入风险（参数化）
- ✅ `unsafe` / `reflect` / CGO —— 无
- ✅ `_ = ctx` —— 找到 F-015 / F-032 / F-051
- ✅ goroutine 启动点 —— 找到 F-042 / F-043 / F-044
- ✅ MCP 错误码字面量 —— 找到 F-025
- ✅ 跨平台 git init —— 已修（F-S001 关联）
- ✅ 文件权限位 0600/0644 —— 已扫
- ✅ 静默吞错（`_, _` / `_ =`）—— 找到 F-016 / F-017 / F-023 / F-053 / F-055 / F-058 / F-064 等

### 依赖安全（离线核对）

- ✅ go.mod 12 个直接 + 间接关键依赖逐一核对（F-075）
- ⚠️ 未联网查 NVD/OSV —— 建议用户后续跑 `govulncheck ./...`
- ✅ worker/pyproject.toml dependencies 状态核对

### 抽样验证

- ✅ F-027 `append_log` 字面量：`grep -rn "\"append_log\"" internal/` 命中代码 + 测试 + 4 处 spec
- ✅ F-041 Windows IPC：手工读 `internal/bridge/bridge.go` 14-58 行
- ✅ F-074 空壳包：`grep -rn 'fengxd1222/llm-wiki/internal/changelog\|...'` 零匹配
- ✅ F-026 工具数：`grep -c "sdk.AddTool\|mcp.AddTool" internal/mcp/server.go` 命中 17 个

### 未覆盖（明示）

- ❌ 性能 benchmark（仅反模式扫描，无 wall time 测试）
- ❌ Windows CI smoke（仅静态推理 F-041 风险）
- ❌ 真实 govulncheck 网络扫描（离线推理）
- ❌ `spec-v2/`、`prototypes/`、`archive/`、`docs/`、`README.md`（PRD 明示 Out of Scope）
