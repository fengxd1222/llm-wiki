// Package service 实现业务层：read / propose / review / lint / dream。
//
// 当前（D3）只有最小 ingest 能力：
//   - IngestFile：把外部文件复制进 vault/raw/inbox/，写 sources 表。
//
// 后续 D 增加：propose / review / accept / search / dream cycle。
package service
