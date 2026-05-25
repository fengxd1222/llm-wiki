# W3 出口 Demo Walkthrough

> WikiMind W3（D15–D21）出口验收 demo。验证 lock manager + drift 检测 +
> lint + queue policy + daemon + multi-agent 并发。
>
> 适用：W3 收尾验收 / 多 agent 并发测试 / daemon 稳定性验证。

---

## 0. 前置

- Go 1.26+, git ≥ 2.30
- 已有 W2 vault（跑过 D14 demo）
- `pip install pypdf`（PDF ingest 可选）

```bash
go build -o bin/wikimind ./cmd/wikimind
go build -o bin/wikimindd ./cmd/wikimindd
```

---

## 1. Daemon 启动

```bash
bin/wikimindd --vault /path/to/vault &
# [wikimindd] starting daemon vault=/path/to/vault
```

Daemon 启动后：
- Watcher 监听 `raw/inbox/`
- Lock reaper 每 30s 清理过期锁
- 日志写入 `.wikimind/daemon.log`

---

## 2. Lock Manager 验证

Agent A 获取锁：
```text
acquire_lock(session_token="sk-A", page_id="cl-001", ttl_seconds=300)
→ { acquired: true, ttl_seconds: 300 }
```

Agent B 尝试获取同页锁：
```text
acquire_lock(session_token="sk-B", page_id="cl-001", ttl_seconds=300)
→ { acquired: false, message: "lock held by another session: held by agent=A since ..." }
```

Agent A 释放：
```text
release_lock(session_token="sk-A", page_id="cl-001")
→ { released: true }
```

---

## 3. Drift 检测

修改 raw 文件内容后：
```bash
echo "modified content" >> /path/to/vault/raw/inbox/paper.md
wikimind lint
# broken_link  cl-001  error  [[missing-page]] points to non-existent page
```

`wiki_info()` 的 `health.drift_claims` 反映真实 drift 数量。

---

## 4. Lint 全套

```bash
wikimind lint
# Rule                 Page                 Severity Detail
# orphan               learning-loop        warn     page "learning-loop" has no inbound or outbound links
# broken_link          learning-loop        error    [[compounding]] points to non-existent page
#
# Summary: 1 errors, 1 warnings, 0 info (2 total)

wikimind lint --json | jq '.summary'
# { "errors": 1, "warnings": 1, "infos": 0, "total": 2 }
```

---

## 5. Queue Policy

```bash
# 查看 queue 状态
wikimind review today
# Top N pending reviews...

# 当 queue 达到 hard limit (50)，新 propose 进入 backlog
# 当 queue 达到 critical limit (100)，新 propose 被拒绝
```

---

## 6. Multi-Agent 并发剧本

两个 agent session 同时操作：
1. Agent A: `propose_page` → `r-0001` (pending)
2. Agent B: `propose_edit` 同 page → lock check → 如果 A 持锁则 ErrLockHeld
3. Agent A: `release_lock` → Agent B 重试成功
4. User: `wikimind review accept r-0001 --no-confirm`
5. User: `wikimind review accept r-0002 --no-confirm`
6. 验证 git log 两个 accept commit + change-log 连续 seq

---

## 7. Rejections Memory

```bash
wikimind review reject r-0003 --reason "This claim contradicts established evidence in cl-002"
```

下次 `agent_handshake` 返回 `recent_rejections` 字段，agent 知道避免重复提交。

---

## 8. W3 闭环已覆盖

- [x] D15: Lock manager + acquire/release MCP tools
- [x] D16: claim_sources + drift detection + rejections memory
- [x] D17: Lint 5 rules + CLI
- [x] D18: Queue limits + review today
- [x] D19: IPC bridge skeleton
- [x] D20: Daemon main loop + wikimindd
- [x] D21: W3 exit demo (this document)

---

## 9. W3 出口标准

| 标准 | 状态 |
|------|------|
| Lock manager 防并发冲突 | ✅ |
| Drift 检测 + health score 真值 | ✅ |
| Lint 规则可发现 vault 问题 | ✅ |
| Queue policy 防膨胀 | ✅ |
| Daemon 可启动 + graceful shutdown | ✅ |
| Multi-agent 串行测试通过 | ✅ |
