# 风险清单与缓解措施

> 把"会出什么问题"和"出了问题怎么办"摊开。
> 风险按"严重度 × 概率"排序：**Critical / High / Medium / Low**。

---

## R-01（Critical）Agent 幻觉写入污染 wiki

| 字段 | 内容 |
|---|---|
| 风险 | Agent 编造"事实"、伪造引用、把推测当结论写入 wiki，长期累积成"看起来权威实则错误"的知识库 |
| 概率 | 高（LLM 本质行为） |
| 影响 | 致命；用户对 wiki 失去信任 → 产品死亡 |
| 触发场景 | 老资料缺失上下文、agent 想"补全"故事、多次 query 同一话题 |
| 缓解 | ① Claim 一等公民 + quote_hash 强校验；② Review queue 默认开启；③ Lint 检测无源 claim 并降级；④ Agent instructions 明确禁止编造；⑤ rejection memory 让 agent 不重复同样幻觉；⑥ User 可一键 revert 到任意 git commit |
| 监控 | `lint_run` 报告中 `unverified_claims` 数量超阈值告警 |
| 责任方 | Agent → Lint → User |

---

## R-02（Critical）多 Agent 并发写入冲突 / 撕裂

| 字段 | 内容 |
|---|---|
| 风险 | 两个 agent 同时改同一 page，git 冲突 / 半完成状态 / 链接断裂 |
| 概率 | 中（取决于使用模式） |
| 影响 | 数据损坏可能、信任流失 |
| 缓解 | ① 单一写入点（daemon）+ commit 串行化；② Advisory lock；③ Review queue 是天然 staging；④ git 是回滚基线；⑤ daemon 检测 stale lock 并清理 |
| 监控 | `.llmwiki/locks/` 长存活；review queue conflict 比例 |

---

## R-03（Critical）跨平台路径/编码差异破坏 wiki

| 字段 | 内容 |
|---|---|
| 风险 | Mac 上叫 `Karpathy.md`，Windows checkout 后变成 `karpathy.md`，链接全断 |
| 概率 | 高（首次跨平台必出） |
| 影响 | 整个 vault 不可用 |
| 缓解 | ① 文件名强制 ASCII kebab-case + lowercase（linter）；② `[[id]]` 而非 `[[slug]]`；③ git `.gitattributes` 锁 LF + UTF-8；④ pre-commit hook 拒绝违规文件 |
| 监控 | `llmwiki doctor` 启动时全扫 |

---

## R-04（High）Watcher 漏事件导致索引漂移

| 字段 | 内容 |
|---|---|
| 风险 | macOS FSEvents/Windows RDCW buffer overflow，watcher 错过文件改动；mtime 没变（rsync）；索引指向旧 hash |
| 概率 | 中 |
| 影响 | 搜索结果错误、claim 引用错误源 |
| 缓解 | ① 启动时全扫一次；② 每小时 reconcile（对 sources 表全扫 mtime+size）；③ daily background hash 校验（取样 5%）；④ `llmwiki rebuild-index` 兜底 |
| 监控 | reconcile 报告中 unexpected diff 数量 |

---

## R-05（High）SQLite 索引损坏

| 字段 | 内容 |
|---|---|
| 风险 | 断电 / 强制 kill / WAL 不一致 |
| 概率 | 低-中 |
| 影响 | 搜索无法工作 |
| 缓解 | ① WAL 模式 + `synchronous=NORMAL`；② 启动 `PRAGMA integrity_check`；③ 一键 rebuild from markdown；④ markdown 是真理，索引只是派生 |
| 监控 | 每次启动校验 |

---

## R-06（High）Source 文件被外部修改导致 claim 漂移

| 字段 | 内容 |
|---|---|
| 风险 | 用户/同步工具改动 raw/，但 wiki 已经引用了旧版本 |
| 概率 | 中 |
| 影响 | Claim 不再可追溯 |
| 缓解 | ① 每次 read_raw_anchor 校验 quote_hash；② mismatch → `DRIFT` 错误 + 标记 claim 为 `needs_reverify`；③ 用户也可设 raw/ 为软只读（chflags / ACL） |
| 监控 | DRIFT 错误日志 |

---

## R-07（High）Embedding API 泄漏用户数据

| 字段 | 内容 |
|---|---|
| 风险 | 启用 embedding 后，wiki 内容被发送到第三方（OpenAI / Voyage / Cohere） |
| 概率 | 中（默认关，开就会发） |
| 影响 | 隐私违规 |
| 缓解 | ① Embedding 默认关；② 启用时显式弹窗"将向 \<API> 发送数据"；③ 默认推荐本地 embedding（`bge-small`，gguf）；④ 仅 wiki/ 内容可被 embed，raw/ 永远不发；⑤ API key 存 keychain |
| 监控 | 每月用量报告 |

---

## R-08（High）Daemon 在用户高权限下被恶意 MCP client 接管

| 字段 | 内容 |
|---|---|
| 风险 | 任意 MCP client 连接 daemon → 调写工具 → 修改 vault |
| 概率 | 中（取决于配置） |
| 影响 | wiki 被注入恶意内容 |
| 缓解 | ① stdio MCP 必须由父进程 spawn，没有网络口；② SSE 模式默认关；③ `[mcp].allowed_clients` 白名单；④ `agent_handshake` 必填；⑤ 所有写入进 review queue，user 把关 |
| 监控 | audit 日志中未知 agent 名 |

---

## R-09（High）Windows Defender / Controlled Folder Access 杀进程

| 字段 | 内容 |
|---|---|
| 风险 | Defender 把未签名的 daemon 当威胁 / Controlled Folder 拒绝写 `Documents/` |
| 概率 | 高（Windows 11 默认 CFA） |
| 影响 | 装完即用不了 |
| 缓解 | ① v1 代码签名；② 安装器检测 CFA 并引导添加排除；③ 兜底放 vault 到非保护路径；④ 安装手册详细说明 |

---

## R-10（High）OneDrive / iCloud 占位符导致 ingest 失败

| 字段 | 内容 |
|---|---|
| 风险 | 文件本地不存在（云端 only），read 报错 |
| 概率 | 高（用户常用） |
| 影响 | 部分资料无法 ingest |
| 缓解 | ① 检测占位符属性；② 标 `needs_hydrate`；③ 询问用户是否触发下载；④ vault 强烈建议放本地常驻目录 |

---

## R-11（Medium）大规模时 lint 过慢

| 字段 | 内容 |
|---|---|
| 风险 | 10 万 page 时 full lint > 30 分钟 |
| 概率 | 中（远期） |
| 影响 | 用户跳过 lint → 风险积累 |
| 缓解 | ① Incremental lint（基于 change_log 的 dirty 列表）；② 按规则分类，可单独跑；③ scheduler 切片执行；④ v1 期专门优化 |

---

## R-12（Medium）Git 仓库膨胀

| 字段 | 内容 |
|---|---|
| 风险 | 频繁 commit + 大文件 → git 仓库 GB 级 |
| 概率 | 中 |
| 影响 | clone / push / 同步慢 |
| 缓解 | ① 大文件（PDF / 图片 / 音频）走 Git LFS 或外置存储（仅在 raw/ 留 hash 引用）；② `git maintenance start` 定期 GC；③ 选项 squash 老 commit（保留 change_log.jsonl 作为审计） |

---

## R-13（Medium）Schema 演化导致老 wiki 不兼容

| 字段 | 内容 |
|---|---|
| 风险 | 改 frontmatter 必填字段，老 page 全部违规 |
| 概率 | 中 |
| 影响 | lint 全红、user 沮丧 |
| 缓解 | ① `schema_version` 强制；② 升级提供迁移脚本（`llmwiki migrate <ver>`）；③ breaking changes 列在 schema 顶部；④ 不写"必填"，用"建议" + warning |

---

## R-14（Medium）"重要资料"用户没意识到要 ingest

| 字段 | 内容 |
|---|---|
| 风险 | 用户的关键资料留在邮件 / Slack / 私人 Notion，没进 raw/ |
| 概率 | 高 |
| 影响 | wiki 知识不完整 → 推理质量下降 |
| 缓解 | ① Obsidian Web Clipper 教程；② 提供 Slack / Notion / 邮件 export 适配器（v0.2）；③ lint 检测"被频繁引用但 source 缺失"并提示用户补 |

---

## R-15（Medium）Agent 之间"羊群效应"

| 字段 | 内容 |
|---|---|
| 风险 | 一个 agent 写了错误 claim，其他 agent 把它当事实强化 |
| 概率 | 中 |
| 影响 | 错误固化为"共识" |
| 缓解 | ① 任何 claim 必须回溯到 raw/，不是从 wiki 另一处引用；② lint 检测"引用链上没有 raw/ 末端"的 claim；③ `provenance_depth` 字段最大 1（即必须直接挂 raw） |

---

## R-16（Medium）Backup / 同步把 vault 破坏

| 字段 | 内容 |
|---|---|
| 风险 | Time Machine / iCloud / Dropbox 在 `.llmwiki/` 写半截、文件锁冲突 |
| 概率 | 中 |
| 影响 | 索引损坏 / 数据丢失 |
| 缓解 | ① `.llmwiki/` 默认 git ignore；② 提示用户 exclude 同步工具；③ 重要文件（change-log.jsonl）入 git；④ rebuild 兜底 |

---

## R-17（Medium）用户依赖单一 LLM 厂商被锁定

| 字段 | 内容 |
|---|---|
| 风险 | Wiki 隐式适配某个 agent 的写作风格 / tool 偏好 |
| 概率 | 中 |
| 影响 | 切换 agent 时质量骤降 |
| 缓解 | ① schema 是合同，任何 agent 都能读；② 模板 AGENTS.md 为底线；③ E2E 测试在 3 个以上 agent 上验证；④ 不依赖单一厂商私有特性 |

---

## R-18（Medium）User 在 review 队列堆积

| 字段 | 内容 |
|---|---|
| 风险 | 用户没空 review，pending 堆积到几百 → 失去意义 |
| 概率 | 高 |
| 影响 | Review queue 退化为摆设 |
| 缓解 | ① `llmwiki status` 显示 backlog；② daily summary 邮件 / 通知（可选）；③ 支持"低风险自动 accept"白名单规则；④ master agent 模式：授权 Claude Code 为你 review 一部分 |

---

## R-19（Low）大文件单页（>1MB markdown）

| 字段 | 内容 |
|---|---|
| 风险 | Agent 把一篇 30 万字论文整页写进一个 wiki page |
| 概率 | 低 |
| 影响 | 编辑器卡 / git diff 难看 |
| 缓解 | ① schema 强制 page 体 < 50KB；② lint 提示拆分；③ 长篇资料用 source 摘要 + claim 拆分 |

---

## R-20（Low）多 vault 混用

| 字段 | 内容 |
|---|---|
| 风险 | 用户同时维护多个 vault，agent 误读其他 vault |
| 概率 | 低 |
| 影响 | 信息泄漏 |
| 缓解 | ① 每个 vault 独立 daemon 实例；② MCP server 启动指定 vault root；③ path traversal 拒绝 |

---

## R-21（Low）用户卸载后残留

| 字段 | 内容 |
|---|---|
| 风险 | launchd plist / Scheduled Task / 索引 db 留在磁盘 |
| 概率 | 中 |
| 影响 | 卡顿 / 隐私担忧 |
| 缓解 | ① `llmwiki uninstall --purge` 命令；② 安装器留 `uninstall.sh` / `Uninstall.ps1` |

---

## 安全模型小结

我们的安全 posture：

1. **进程边界**：daemon 仅访问 `[vault].root`；任何 path traversal 拒绝（包括符号链接逃逸）。
2. **认证**：MCP client 必须握手；agent 名在白名单。
3. **授权**：写工具默认进 review；user 显式开启 auto-accept 才直写。
4. **审计**：所有操作进 change_log + git；不可篡改（git history）。
5. **数据出境**：默认零外发；embedding / git push 显式启用。
6. **凭证**：API key 存 OS keychain；不进配置文件明文。
7. **沙箱**：daemon 不需要 root / admin；不申请 FDA（除非用户 vault 在受保护目录）。
8. **可观测**：`llmwiki doctor` + `llmwiki audit tail` 让用户随时检查。

---

## 失败模式恢复 Playbook

| 失败 | 命令 |
|---|---|
| 索引损坏 | `llmwiki rebuild-index` |
| Agent 写错了 | `llmwiki revert <change-log-seq>` |
| Watcher 漏事件 | `llmwiki reconcile` |
| Lock 死锁 | `llmwiki lock list / break <page-id>` |
| Git 冲突 | `llmwiki conflict list / resolve <id>` |
| 跨平台文件名违规 | `llmwiki doctor --fix-names` |
| Source 漂移 | `llmwiki reverify --since <date>` |
| Schema 不兼容 | `llmwiki migrate <to-version>` |
| 全盘出问题 | `git reset --hard <last-good-commit>` + `llmwiki rebuild-index` |
