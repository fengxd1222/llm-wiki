# W2 D13: PDF ingest + 图片元数据 + macOS FSEvents watcher

## Goal

扩展 ingest 支持 PDF + 图片，并接入 macOS FSEvents watcher 让 raw/inbox/
拖入新文件自动 ingest。三件套：
1. **PDF ingest pipeline**：用 Python worker 调 pdftotext + heading 识别
2. **图片 ingest**：元数据（dimensions, EXIF）+ 占位 page（MVP 不强制 OCR）
3. **Watcher**：macOS FSEvents 接 raw/inbox/ 新文件 → 自动 ingest + debounce

需求来源：
- `spec-v2/docs/roadmap-30d.md` W2 D13
- `spec-v2/docs/cross-platform.md §2.2` FSEvents 已知问题 + debounce 200ms
- `spec-v2/docs/architecture.md §3.4` Watcher 与 Sync 流程
- D3 ingest 当前只 markdown；D7 demo flow 仅文本路径

## What I already know

- D3 `internal/service/ingest.go` 当前 `IngestFile(path)` 只处理 markdown
  （读 content + frontmatter parse）
- D7 ingest 自动生成 wiki/sources/<id>.md，body 是占位 "see raw file"
  —— PDF / 图片走同 pattern：source page 是元数据描述 page，raw 文件本身保留
- `worker/main.py` (W0 skeleton)：从 stdin 读 task JSON → stdout NDJSON 事件，
  接 D13 PDF 解析
- `fsnotify` Go 库支持 macOS FSEvents + Linux inotify + Windows RDCW
- W3 D17 lint 才做"watcher 触发增量 reindex"——D13 watcher 只 watch raw/inbox/
  触发 ingest

## Requirements

### A. PDF ingest pipeline

#### A1. Python worker 扩展（`worker/main.py`）

当前 W0 skeleton 仅 echo。D13 改成真 ingest worker：

```python
# task JSON 输入：
{
    "task_id": "ing-001",
    "kind": "pdf",
    "raw_path": "/abs/path/to/paper.pdf"
}

# stdout NDJSON 事件输出：
{"type": "progress", "task_id": "ing-001", "stage": "extract_text", "pct": 20}
{"type": "progress", "task_id": "ing-001", "stage": "detect_headings", "pct": 60}
{"type": "result", "task_id": "ing-001",
 "text": "<full extracted text>",
 "headings": [
    {"level": 1, "text": "Introduction", "char_start": 0, "char_end": 120},
    {"level": 2, "text": "Background", "char_start": 121, "char_end": 480}
 ],
 "page_count": 12,
 "metadata": {"title": "...", "author": "...", "creation_date": "..."}
}
```

实施：
- `pdftotext -layout` (poppler-utils) 抽全文，捕 stdout
- heading 识别：行级启发式（字号变化无法靠 pdftotext —— 改用 markdown
  风格 `^#+\s` 反向匹配，对 academic PDF 通常 well-structured）
- metadata 用 `pdfinfo`（同 poppler）
- 错误：`pdftotext` 未装 → 事件 type=error + message "pdftotext required"
- 跨平台：mac/linux poppler-utils 装一致；Windows 留 W3+（D13 macOS 优先）

依赖：`pyproject.toml` 加 `pypdf>=4.0`（fallback：纯 Python 解析无需 poppler）
或 `pdfplumber>=0.10`（更强 heading 识别但更慢）

**推荐**：MVP 走 `pypdf` 纯 Python，无外部 binary 依赖；性能限：>50MB PDF
慢但 acceptable。

#### A2. Go 调 Python worker（`internal/service/ingest.go`）

`IngestFile` 加 PDF 分支：

```go
func IngestFile(ctx, vault, rawPath) (*IngestResult, error) {
    ext := strings.ToLower(filepath.Ext(rawPath))
    switch ext {
    case ".md", ".markdown":
        return ingestMarkdown(...)  // 现有 D3/D7 路径
    case ".pdf":
        return ingestPDF(ctx, vault, rawPath)  // 新
    case ".png", ".jpg", ".jpeg", ".gif", ".webp":
        return ingestImage(ctx, vault, rawPath)  // 新
    default:
        return nil, ErrUnsupportedRawFormat
    }
}

func ingestPDF(ctx, vault, rawPath) (*IngestResult, error) {
    // 1. 拷 raw 到 raw/inbox/<basename> (D3 既有)
    // 2. exec worker/main.py，stdin 写 task JSON，stdout 读 NDJSON
    // 3. 取 result 事件的 text + headings + metadata
    // 4. 生成 source page：
    //    frontmatter: id/type=source/title (from metadata.title or basename)
    //                 / source_path (raw/inbox/<basename>) / ingested_at
    //                 / pdf_page_count / pdf_author
    //    body: markdown 风格 outline（headings 转 # ## ###）+ 占位
    //          "see raw PDF for full content"
    // 5. D6 commit.Commit + D10 reviews 表无关 (ingest 直接走 main)
}
```

Python worker 调用：`exec.Command("python3", "worker/main.py")` + stdin pipe
+ stdout scanner (line-by-line NDJSON)。

#### A3. 图片 ingest（`internal/service/ingest.go`）

```go
func ingestImage(ctx, vault, rawPath) (*IngestResult, error) {
    // 1. 拷 raw 到 raw/inbox/<basename>
    // 2. 提取元数据：用 image package decode 拿 width/height
    //    用 github.com/rwcarlsen/goexif 拿 EXIF (optional)
    // 3. source page:
    //    frontmatter: id/type=source/title (filename) / source_path
    //                 / image_width / image_height / image_format
    //                 / exif_make / exif_camera / exif_taken_at (optional)
    //    body: "![alt](raw/inbox/<basename>)\n\n*see raw image for full*"
    //    (markdown 渲染时 user 看到缩略图)
    // 4. D6 commit
}
```

依赖：`image/png` + `image/jpeg` (std lib) decode header 拿 dimensions；
EXIF 用 `github.com/rwcarlsen/goexif/exif`（小、纯 Go）—— 加 go.mod。

### B. Watcher（`internal/watcher/`）

新包 3 文件：

#### B1. `watcher.go`

```go
type Watcher struct {
    vaultRoot   string
    fsnotify    *fsnotify.Watcher
    eventQueue  chan FileEvent
    debounceMs  time.Duration  // default 200ms
}

type FileEvent struct {
    Path string  // absolute
    Op   fsnotify.Op  // Create / Write / Remove
    Ts   time.Time
}

func NewWatcher(vaultRoot string, debounceMs time.Duration) (*Watcher, error)
func (w *Watcher) Watch(ctx context.Context, dirs ...string) error  // Add dirs, start goroutine
func (w *Watcher) Events() <-chan FileEvent  // debounced events
func (w *Watcher) Close() error
```

#### B2. `debounce.go`

按 path debounce 200ms (cross-platform.md §2.2)：
- 同 path 200ms 内多次事件 → 取最后一个
- 用 map[path]*time.Timer + sync.Mutex

#### B3. `watcher_test.go`

- Create file in watched dir → 收到 1 个 Create 事件（≥200ms 后）
- 同 path 高频写 → debounce 后只 1 个 Write 事件
- Watch 多 dir 同时
- Close 时 goroutine 干净退出
- fsnotify 失败 → 友好错误

### C. CLI 集成

`cmd/wikimind/command.go` 加 `wikimind watch` 子命令：
- `wikimind watch [--vault <path>] [--auto-ingest]`
- 跑 watcher in foreground (Ctrl-C 退出)
- `--auto-ingest`：raw/inbox/ 新文件自动调 `IngestFile`（debounced）
- 不带 flag：只打印事件到 stderr（debug 用）
- 后续 D13/W2 daemon 跑长生命周期 watcher；D13 是 CLI 启动验证版本

不动 `wikimind mcp serve`——watcher 不进 MCP。

### D. 测试

- `worker/test_main.py`（新）：
  - PDF task → result 事件含 text + headings
  - 非 PDF / 损坏 PDF → error 事件
  - empty task → error
- `internal/service/ingest_test.go`（既有，加测试）：
  - ingestPDF happy（mock worker 输出 text+headings → source page 生成）
  - ingestImage happy（造个 1x1 PNG → source page 含 dimensions）
  - 不支持格式 → ErrUnsupportedRawFormat
- `internal/watcher/watcher_test.go`：见 B3
- `cmd/wikimind/command_test.go`：watch 命令存在 + --auto-ingest flag

目标测试总数：210（D12 后）→ ≥235（+25）

### E. 文档

`docs/demo/w2-watcher.md`：
- 装依赖：`pip install pypdf` (or 系统装 poppler)
- 跑 watcher：`wikimind watch --auto-ingest`
- 在另一终端 `cp paper.pdf <vault>/raw/inbox/`
- watcher 检测到 + 自动 ingest + log.md 增行 + query 立即命中

## Acceptance Criteria

- [ ] `worker/main.py` 真实 PDF 解析（pypdf）
- [ ] `wikimind ingest paper.pdf` 生成 wiki/sources/paper.md 含 headings outline
- [ ] `wikimind ingest photo.jpg` 生成 wiki/sources/photo.md 含 dimensions/EXIF
- [ ] `wikimind watch --auto-ingest` 检测 raw/inbox/ 新文件触发 ingest
- [ ] debounce 200ms：高频写不重复 ingest
- [ ] 跨平台 watcher：macOS FSEvents 优先；Linux/Windows fsnotify fallback
- [ ] 不支持格式 ingest → ErrUnsupportedRawFormat 友好错误
- [ ] 单测：≥ 25 个新测试
- [ ] `go build / vet / test ./...` 全绿；CI 5 OS 通过

## Definition of Done

- A-E done
- CI 5 OS 全绿（PDF/image 跨平台跑通；Windows watcher 至少不 crash）
- 测试 ≥ 235
- commit + push

## Out of Scope

- PDF OCR（扫描件 → 文字）—— W4+
- 图片 OCR—— W4+ 可选
- 音频 ingest (whisper)—— W4+
- HTML / EPUB / DOCX ingest—— W3+
- launchd 自启 `wikimind watch`—— W4 release 阶段
- 全 vault 启动时 reconcile—— W3 lint 一起做
- Watcher 高级特性（partial sync, exclude patterns）—— W3+
- 增量 reindex 单文件 update（D13 watcher 触发 full reindex；W3 D17 incremental）

## Decision (ADR-lite)

**Context**: PDF 解析有 5 选项（pdftotext binary / pypdf pure-py /
pdfplumber / pdfminer.six / poppler 直接 Go binding），每种 trade-off 不同。

**Decision**:
1. **PDF 走 Python worker + pypdf**：跨平台 pure-py，无系统依赖；Go 调
   Python 子进程通过 stdin/stdout JSON 协议（engineering-decisions §1）。
   性能限：> 50MB PDF 较慢但 MVP acceptable
2. **图片 dimensions 走 Go std lib**：image/png + image/jpeg DecodeConfig
   只读 header 不解码全图，毫秒级
3. **EXIF optional**：`rwcarlsen/goexif` 加 go.mod；非 JPEG 图片或无 EXIF
   silently 跳过
4. **Watcher 用 fsnotify**：跨平台抽象层（macOS FSEvents / Linux inotify /
   Windows RDCW），跟 cross-platform.md §2.2 一致；buffer overflow / sleep
   handling 留 W3 daemon 做
5. **D13 watcher 不进 daemon**：跑在 `wikimind watch` CLI 进程；Ctrl-C 退出。
   long-running daemon (launchd) 留 W4 release

**Consequences**:
- 优点：D13 三件套独立可测；user 在 macOS 立即用 `wikimind watch --auto-ingest`
  体验拖拽 ingest
- 缺点：Python worker 跨平台依赖（macOS/Linux pypdf 装 OK；Windows 需 user
  自己 pip install）。CI Windows 跑 PDF test 可能 skip（worker 缺）
- watcher 高级特性 W3 daemon 才完整；D13 watcher 是 MVP 单进程版本

## Technical Notes

- `pypdf` API：
  ```python
  from pypdf import PdfReader
  reader = PdfReader(path)
  text = "\n".join(p.extract_text() for p in reader.pages)
  meta = reader.metadata  # /Title /Author /CreationDate
  ```
- heading 识别简化版：
  ```python
  for line in text.split("\n"):
      if re.match(r"^(?:\d+\.?\s+|#+\s+|[A-Z][A-Z\s]{3,}$)", line):
          # 视为 heading
  ```
- Go 调 Python：`exec.Command("python3", workerPath)` + `cmd.Stdin = strings.NewReader(taskJSON)`
- 跨平台 python binary：macOS/Linux 用 `python3`；Windows 优先 `py -3` 或 `python`
- watcher debounce 实现：map 加 time.Timer.Reset()
- fsnotify v1.7+ 支持 `RECURSIVE` (Linux) 但 macOS/Windows 不支持—— D13
  显式 Add 每个子目录
- image DecodeConfig：`cfg, _, _ := image.DecodeConfig(file)` → `cfg.Width / cfg.Height`
- EXIF orientation 字段：可选；D13 不旋转图片，仅记录

## 实施建议顺序

1. **Python worker pypdf 抽取**（独立可测，pytest 跑）
2. **internal/service/ingest.go ingestPDF**（Go 端调 worker，测 mock subprocess）
3. **internal/service/ingest.go ingestImage**（独立小功能）
4. **internal/watcher/ 包**（独立可测）
5. **CLI: wikimind watch**（整合 watcher + ingest）
6. **docs/demo/w2-watcher.md**
7. **测试 + ≥ 235 验证**
