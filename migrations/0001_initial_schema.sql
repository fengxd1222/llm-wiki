-- +goose Up
-- W0 占位 migration，验证 goose 迁移链路可用。
-- 完整 schema（sources / pages / pages_fts / claim_sources / page_links /
-- reviews / bundles / change_log / locks / agent_sessions）见
-- spec-v2/docs/architecture.md §4.2，在 roadmap D3 实现。
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_bootstrap', '0001');

-- +goose Down
DROP TABLE IF EXISTS meta;
