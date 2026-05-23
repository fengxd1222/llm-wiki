// Package index 实现 SQLite 索引、FTS5、reconcile 与 rebuild。
//
// 当前（D3）只暴露最小能力：
//   - Open：打开 `.wikimind/index.db`，自动跑 goose up 到最新版本
//   - 事务入口（BeginTx）供 service 层使用
//   - sources 表（migrations/0001）：raw ingest 三件套（sha256 / mtime / size）
//
// 完整 schema（pages / pages_fts / claim_sources / page_links / reviews / ...）
// 按 roadmap D4+ 增量加新 migration。
package index
