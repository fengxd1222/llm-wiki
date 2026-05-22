# 风险清单

> 26 条风险 + 缓解 + 监控 + 责任方。按"严重度 × 概率"排序：Critical / High / Medium / Low。
>
> 基于方案 A 的 21 条风险清单整合，新增 5 条（三方共同盲点对应的风险）。每条缓解措施链接到
> spec-v2 的对应文档。

---

## 速查

| ID | 风险 | 等级 | 缓解文档 |
|---|---|---|---|
| R-01 | Agent 幻觉写入污染 wiki | Critical | [claim-extraction](claim-extraction.md) / [review-queue-policy](review-queue-policy.md) |
| R-02 | 多 agent 并发写入冲突 | Critical | [agent-protocol](agent-protocol.md) / [conflict-scenarios](conflict-scenarios.md) |
| R-03 | 跨平台路径/编码破坏 wiki | Critical | [cross-platform](cross-platform.md) |
| R-22 | Claim 粒度不一致致 wiki 分裂 | Critical | [claim-extraction](claim-extraction.md) |
| R-04 | Watcher 漏事件致索引漂移 | High | [architecture](architecture.md) / [cross-platform](cross-platform.md) |
| R-05 | SQLite 索引损坏 | High | [failure-playbook](failure-playbook.md) |
| R-06 | Source 漂移致 claim 失追溯 | High | [failure-playbook](failure-playbook.md) |
| R-07 | Embedding API 泄漏数据 | High | SPEC §5.2 |
| R-08 | Daemon 被恶意 MCP client 接管 | High | [agent-protocol §11](agent-protocol.md#11-安全边界) |
| R-09 | Windows Defender / CFA 杀进程 | High | [cross-platform §3.4](cross-platform.md) |
| R-10 | OneDrive/iCloud 占位符致 ingest 失败 | High | [cross-platform §5](cross-platform.md) |
| R-18 | User review 队列堆积 | High | [review-queue-policy](review-queue-policy.md) |
| R-23 | 冷启动流失 | High | [onboarding](onboarding.md) |
| R-24 | CJK 全文搜索失效 | High | [cjk-tokenizer](cjk-tokenizer.md) |
| R-11 | 大规模 lint 过慢 | Medium | [architecture §8](architecture.md) |
| R-12 | Git 仓库膨胀 | Medium | — |
| R-13 | Schema 演化老 wiki 不兼容 | Medium | [agent-protocol §2.3](agent-protocol.md) |
| R-14 | 用户没意识到要 ingest 关键资料 | Medium | — |
| R-15 | Agent 羊群效应 | Medium | [claim-extraction §4 Case 10](claim-extraction.md) |
| R-16 | Backup/同步破坏 vault | Medium | [cross-platform §5.2](cross-platform.md) |
| R-17 | 依赖单一 LLM 厂商锁定 | Medium | [agent-protocol §2](agent-protocol.md) |
| R-25 | 级联污染无法整批回滚 | Medium | [failure-playbook §3](failure-playbook.md) |
| R-26 | Query sedimentation 沉淀垃圾 | Medium | [query-sedimentation §5](query-sedimentation.md) |
| R-19 | 大文件单页 | Low | — |
| R-20 | 多 vault 混用 | Low | [agent-protocol §11](agent-protocol.md) |
| R-21 | 用户卸载残留 | Low | [cross-platform](cross-platform.md) |

---

## Critical

### R-01 — Agent 幻觉写入污染 wiki

| 字段 | 内容 |
|---|---|
| 风险 | Agent 编造"事实"、伪造引用、把推测当结论写入；长期累积成"看似权威实则错误"的库 |
| 概率 | 高（LLM 本质行为） |
| 影响 | 致命——用户对 wiki 失去信任 = 产品死亡 |
| 缓解 | 五层防御（SPEC §8）：① claim quote_hash 强校验；② confidence + status；③ review queue 单一闸门；④ lint 反幻觉规则；⑤ git revert。claim 抽取算法 4 步自检（[claim-extraction](claim-extraction.md)） |
| 监控 | `lint_run` 报告 `unverified_claims` 数；review reject rate；weekly Dream Cycle |
| 责任方 | Agent → Lint → User |

### R-02 — 多 agent 并发写入冲突 / 撕裂

| 字段 | 内容 |
|---|---|
| 风险 | 两 agent 同时改同一 page，git 冲突 / 半完成状态 / 链接断裂 |
| 概率 | 中（取决于使用模式） |
| 影响 | 数据损坏可能、信任流失 |
| 缓解 | Worktree 物理隔离 + advisory lock + daemon 单线程 commit 三层；5 个冲突剧本演练（[conflict-scenarios](conflict-scenarios.md)） |
| 监控 | `.wikimind/audit/conflicts.jsonl`；conflict group 数量 |
| 责任方 | Daemon |

### R-03 — 跨平台路径/编码差异破坏 wiki

| 字段 | 内容 |
|---|---|
| 风险 | Mac 上 `Karpathy.md`，Windows checkout 后变 `karpathy.md`，链接全断 |
| 概率 | 高（首次跨平台必出） |
| 影响 | 整个 vault 不可用 |
| 缓解 | 文件名强制 ASCII lower kebab（lint error 级）；`[[id]]` 而非 `[[slug]]`；`.gitattributes` 锁 LF+UTF-8；`wikimind doctor --fix-names`（[cross-platform §1](cross-platform.md)） |
| 监控 | `wikimind doctor` 启动全扫 |
| 责任方 | Lint → User |

### R-22 — Claim 粒度不一致致 wiki 分裂【新增】

| 字段 | 内容 |
|---|---|
| 风险 | 不同 agent / 不同时间抽 claim 粒度差异巨大（过粗/过细/不一致），wiki 长期分裂为互不兼容的子集 |
| 概率 | 高（无算法约束时必然发生） |
| 影响 | 致命——wiki 失去一致性，query 结果不可预测，无法多 agent 协作 |
| 缓解 | claim 抽取 4 步算法 + 自检 4 道 + 10 案例固化在 schema/CLAUDE.md 等（[claim-extraction](claim-extraction.md)）；weekly 跑粒度稳定性指标 |
| 监控 | `wikimind lint --rule claim_quality`；同一 raw 被不同 agent 抽取的 claim 集合差异 < 0.3 |
| 责任方 | Agent（按算法）→ Lint → User |

---

## High

### R-04 — Watcher 漏事件致索引漂移

| 字段 | 内容 |
|---|---|
| 风险 | FSEvents/RDCW buffer overflow 漏文件改动；rsync 不改 mtime；索引指向旧 hash |
| 概率 | 中 |
| 缓解 | 启动全扫；每小时 reconcile；每日抽样 5% 重算 hash；Windows USN journal 补漏；`wikimind reconcile` 兜底 |
| 监控 | reconcile 报告中 unexpected diff 数 |

### R-05 — SQLite 索引损坏

| 字段 | 内容 |
|---|---|
| 风险 | 断电 / 强杀 / WAL 不一致 |
| 概率 | 低-中 |
| 缓解 | WAL + `synchronous=NORMAL`；启动 `integrity_check`；`wikimind rebuild-index` 一键重建；markdown 是真理源 |
| 监控 | 每次启动校验 |

### R-06 — Source 漂移致 claim 失追溯

| 字段 | 内容 |
|---|---|
| 风险 | 用户/同步工具改 raw/，wiki 仍引用旧版本 |
| 概率 | 中 |
| 缓解 | `read_raw_anchor` 实时校验 quote_hash；mismatch → DRIFT 错误 + 标 `needs_reverify`；`wikimind reverify`；建议设 raw/ 软只读 |
| 监控 | DRIFT 事件日志；Dream Cycle audit |

### R-07 — Embedding API 泄漏用户数据

| 字段 | 内容 |
|---|---|
| 风险 | 启用 embedding 后 wiki 内容发往第三方 |
| 概率 | 中（默认关，开就会发） |
| 缓解 | embedding 默认关；启用时显式弹窗；默认推荐本地 `bge-small`；仅 wiki/ 可 embed，raw/ 永不发；API key 存 keychain |
| 监控 | 每月用量报告 |

### R-08 — Daemon 被恶意 MCP client 接管

| 字段 | 内容 |
|---|---|
| 风险 | 任意 MCP client 连 daemon → 调写工具 → 注入恶意内容 |
| 概率 | 中 |
| 缓解 | stdio MCP 必须父进程 spawn（无网络口）；SSE 默认关；`allowed_clients` 白名单；`agent_handshake` 必填；所有写进 review queue |
| 监控 | audit 日志未知 agent 名 |

### R-09 — Windows Defender / CFA 杀进程

| 字段 | 内容 |
|---|---|
| 风险 | Defender 把未签名 daemon 当威胁；CFA 拒绝写 `Documents/` |
| 概率 | 高（Windows 11 默认 CFA） |
| 缓解 | v1 代码签名；安装器检测 CFA 并引导排除；兜底放 vault 到非保护路径；`wikimind doctor` 检测（[cross-platform §3.4](cross-platform.md)） |

### R-10 — OneDrive/iCloud 占位符致 ingest 失败

| 字段 | 内容 |
|---|---|
| 风险 | 文件本地不存在（云端 only），read 报错 |
| 概率 | 高 |
| 缓解 | 检测占位符属性；标 `needs_hydrate`；询问 user 触发下载；建议 vault 放本地常驻目录（[cross-platform §5](cross-platform.md)） |

### R-18 — User review 队列堆积

| 字段 | 内容 |
|---|---|
| 风险 | 用户没空 review，pending 堆到几百 → 闸门退化为摆设 → 所有防幻觉承诺失效 |
| 概率 | 高 |
| 影响 | 致命级（虽列 High，实际是产品最易死的方式之一） |
| 缓解 | 上限保护（>50 停接 propose）+ bundle 归并 + auto-accept 白名单 + 优先级排序 + "今日 5 分钟"模式（[review-queue-policy](review-queue-policy.md)） |
| 监控 | queue depth；avg review latency；daily accept ratio |

### R-23 — 冷启动流失【新增】

| 字段 | 内容 |
|---|---|
| 风险 | 新用户 `wikimind init` 后面对空目录 + 一堆 CLI，不知道做什么 → 流失 |
| 概率 | 高（无 onboarding 时几乎必然） |
| 影响 | 产品没有用户 = 失败 |
| 缓解 | `wikimind demo` 5 分钟剧本，零配置零网络完整闭环，3 个关键时刻（[onboarding](onboarding.md)） |
| 监控 | demo 完成率 ≥ 70%；demo→真实 vault 转化 ≥ 40% |

### R-24 — CJK 全文搜索失效【新增】

| 字段 | 内容 |
|---|---|
| 风险 | FTS5 默认 unicode61 对中文子串搜索失效——"看起来能用其实查不到" |
| 概率 | 高（中文 vault + 默认配置 = 必然） |
| 影响 | 中文用户的核心功能（搜索）静默损坏 |
| 缓解 | 默认用 trigram tokenizer；短查询 fallback ripgrep；CI 固定跑 CJK 检索回归用例；SQLite ≥ 3.40 强制（[cjk-tokenizer](cjk-tokenizer.md)） |
| 监控 | CI CJK 检索用例；search 召回率 |

---

## Medium

### R-11 — 大规模 lint 过慢

缓解：incremental lint（基于 change_log dirty 列表）；按规则分类可单独跑；scheduler 切片。
监控：lint 全量耗时（目标 < 60s @ 10k 页）。

### R-12 — Git 仓库膨胀

缓解：大文件（PDF/图片/音频）走 git LFS 或外置存储（raw/ 留 hash 引用）；`git maintenance` 定期 GC；
可选 squash 老 commit（保留 change-log.jsonl 作审计）。
监控：`.git/` 体积。

### R-13 — Schema 演化致老 wiki 不兼容

缓解：`schema_version` 强制；minor 升级必带 default；major 升级提供 migration 脚本；
breaking changes 列在 schema 顶部；用"建议"而非"必填"+ warning（[agent-protocol §2.3](agent-protocol.md)）。
监控：schema 升级后 lint 红色数量。

### R-14 — 用户没意识到要 ingest 关键资料

缓解：Obsidian Web Clipper 教程；Slack/Notion/邮件 export 适配器（v0.2）；
lint 检测"被频繁引用但 source 缺失"并提示。
监控：lint `missing_source` 项。

### R-15 — Agent 羊群效应

| 字段 | 内容 |
|---|---|
| 风险 | 一个 agent 写错 claim，其它 agent 把它当事实强化 |
| 缓解 | claim 的 source 必须挂 raw/，禁止挂 wiki page（`provenance_depth ≤ 1`）；lint 检测违规；[claim-extraction](claim-extraction.md) §4 Case 10 |
| 监控 | lint `claim_provenance_depth_gt_1` |

### R-16 — Backup/同步破坏 vault

缓解：`.wikimind/` 默认 gitignore + 同步排除；change-log.jsonl 入 git；rebuild 兜底；
`wikimind init` 检测同步目录并引导。
监控：`wikimind doctor` 检测 vault 是否在同步目录。

### R-17 — 依赖单一 LLM 厂商锁定

缓解：schema 是合同，任何 agent 都能读；`AGENTS.md` 为底线；E2E 在 ≥ 3 agent 验证；
不依赖单一厂商私有特性。
监控：CI 多 agent 矩阵。

### R-25 — 级联污染无法整批回滚【新增】

| 字段 | 内容 |
|---|---|
| 风险 | agent 写了一批互相引用的文件，一个错误 claim 被下游多个 page 引用，`git revert` 留下 dangling link |
| 概率 | 中 |
| 缓解 | `wikimind revert-cascade` + 反向影响分析（依赖图 BFS）+ 三种策略（cascade/stubs/isolate）（[failure-playbook §3](failure-playbook.md)） |
| 监控 | revert 后 lint `broken_link` 数 |

### R-26 — Query sedimentation 沉淀垃圾【新增】

| 字段 | 内容 |
|---|---|
| 风险 | 大量平庸问答被沉淀成 wiki，topic page 泛滥成垃圾 |
| 概率 | 中（无质量闸时） |
| 缓解 | sediment_score ≥ 60 阈值；topic 不引入新断言；进 review queue；去重检查；每日上限；Dream Cycle 合并重复（[query-sedimentation §5](query-sedimentation.md)） |
| 监控 | 沉淀 propose accept rate；沉淀 topic 后续命中率 |

---

## Low

### R-19 — 大文件单页

缓解：schema 强制 page 体 < 50KB；lint 提示拆分；长资料用 source 摘要 + claim 拆分。

### R-20 — 多 vault 混用

缓解：每个 vault 独立 daemon 实例；MCP server 启动指定 vault root；path traversal 拒绝。

### R-21 — 用户卸载后残留

缓解：`wikimind uninstall --purge`；安装器留 `uninstall.sh` / `Uninstall.ps1`。

---

## 安全模型小结

1. **进程边界** —— daemon 仅访问 vault root；path traversal 拒绝（含符号链接逃逸）。
2. **认证** —— MCP client 必须握手；agent 名在白名单。
3. **授权** —— 写工具默认进 review；user 显式开 auto-accept 才直写。
4. **审计** —— 所有操作进 change_log + git；不可篡改。
5. **数据出境** —— 默认零外发；embedding / git push 显式启用。
6. **凭证** —— API key 存 OS keychain；不进配置文件明文。
7. **沙箱** —— daemon 不需 root/admin；不申请 FDA（除非 vault 在受保护目录）。
8. **可观测** —— `wikimind doctor` + `wikimind audit tail` 随时检查。

---

## 风险评审节奏

- 每个 milestone（W1-W4 周末）评审一次：哪些风险已缓解、哪些新出现
- 任何 Critical 风险无缓解 → 阻止 milestone 通过
- weekly Dream Cycle report 含"风险监控指标"摘要
- v0.1 发布前，26 条风险全部"缓解措施已实现并测试"

---

## 与三方案的对比

| | 方案 A | 方案 B | GPT Pro | spec-v2 |
|---|---|---|---|---|
| 风险条数 | 21 | 12 | 散落正文 | **26**（21 + 5 新增） |
| 分级 | ✅ 4 级 | 部分 | ❌ | ✅ 4 级 |
| 缓解链接到文档 | 部分 | ❌ | ❌ | ✅ 全部 |
| 监控指标 | ✅ | 部分 | ❌ | ✅ |

新增 5 条（R-22 ~ R-26）正是三方共同盲点对应的风险——它们能被列出来，本身就是 Wave 1 补丁文档的价值证明。

---

## 一句话总结

> 26 条风险，4 级分级，每条缓解措施都链接到 spec-v2 的具体文档。最危险的不是技术故障，是
> R-01（幻觉污染）和 R-18（review 堆积）——它们让产品"信任崩塌"。新增的 R-22/23/24 是三方
> 原方案都没识别的盲点风险。
