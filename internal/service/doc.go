// Package service 实现业务层：read / propose / review / lint / dream。
//
// 当前（D5）能力：
//   - IngestFile：把外部文件复制进 vault/raw/inbox/，写 sources 表
//   - ParsePage / ReindexWiki：解析 markdown frontmatter + heading + outbound 链接
//   - Search：FTS5 trigram BM25 → ripgrep → SQL LIKE 三路兜底
//
// 后续 D 增加：propose / review / accept / dream cycle。
package service
