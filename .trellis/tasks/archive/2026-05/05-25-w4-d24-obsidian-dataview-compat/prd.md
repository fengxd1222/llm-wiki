# W4 D24: Obsidian vault + Dataview 兼容验证

## Goal

Verify `wiki/` 子目录可作为 Obsidian vault 打开，所有 page 渲染正常，
backlink + Dataview 插件可用。

需求来源：
- `spec-v2/docs/roadmap-30d.md` W4 D24

## Requirements

### A. Obsidian compatibility checklist

`docs/compat/obsidian.md`：
- 用 demo vault 在 Obsidian 打开
- 测试 12 项：
  - [[wikilink]] 解析
  - Markdown 渲染（claim / entity / concept 三类）
  - frontmatter 显示
  - Tags 解析
  - Image preview (source page raw 路径)
  - Graph view（page links）
  - Backlinks pane
  - Daily notes 不冲突
  - Search 全文
  - 文件名 lower-kebab 符合
  - Chinese path 编码 (UTF-8 NFC)
  - log.md 表格渲染

### B. Dataview 测试

`docs/compat/dataview.md`：
- 5 个 Dataview query 示例（works in Dataview 0.5.x）：
  ```dataview
  LIST FROM "claims" WHERE confidence > 0.8
  ```
- 验证 frontmatter 字段（id / confidence / sources）Dataview 可读

### C. frontmatter schema 收敛

`internal/service/page.go` ParsePage 输出 frontmatter 默认值确保 Dataview
不报 "missing field"：
- `confidence: 0` 默认（不存在时填）
- `sources: []` 默认
- `status: "unverified"` 默认

### D. 测试

- frontmatter 默认值单测
- Obsidian 渲染只能 manual（截图存 docs/compat/screenshots/）

目标 ≥ 530（D23 后 515 + 15）。

## Acceptance Criteria

- [ ] 12 Obsidian 兼容项 check 全过
- [ ] 5 Dataview query 都返结果
- [ ] frontmatter 默认值确保 Dataview 不挂
- [ ] CI 5 OS 全绿；测试 ≥ 530

## Out of Scope

- Obsidian plugin 自动 install
- 其他笔记软件兼容（Logseq / Notion / Bear）

## Decision (ADR-lite)

**Decision**：保持 wiki/ 作为 source-of-truth 主 Markdown，Obsidian 是
secondary viewer。不为 Obsidian 改 frontmatter schema；仅加默认值不让插件挂。
