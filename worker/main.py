"""WikiMind ingest worker — W0 skeleton (PDF/image parsing deferred to v0.2).

从 stdin 读一行任务 JSON，向 stdout 输出 NDJSON 事件流。
协议见 spec-v2/docs/engineering-decisions.md §1。

当前状态：仅回 skeleton 事件，不做真实解析。Go 侧目前也没有调用此 worker。
完整 parser（markdown / html / pdf / image / audio）推迟到 v0.2 milestone。
"""

from __future__ import annotations

import json
import sys


def main() -> int:
    line = sys.stdin.readline()
    if not line.strip():
        print(json.dumps({"type": "error", "message": "empty task"}))
        return 1

    try:
        task = json.loads(line)
    except json.JSONDecodeError as exc:
        print(json.dumps({"type": "error", "message": f"invalid task json: {exc}"}))
        return 1

    task_id = task.get("task_id", "")
    print(json.dumps({"type": "progress", "task_id": task_id, "stage": "skeleton", "pct": 100}))
    print(json.dumps({
        "type": "result",
        "task_id": task_id,
        "normalized": {"headings": [], "paragraphs": [], "anchors": []},
    }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
