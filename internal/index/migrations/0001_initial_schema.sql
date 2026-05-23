-- +goose Up
-- W1 D3: bootstrap meta + sources 表。
-- 完整 schema（pages / pages_fts / claim_sources / page_links / reviews /
-- bundles / change_log / locks / agent_sessions）见 spec-v2/docs/architecture.md
-- §4.2，按 roadmap D4+ 增量加新 migration。
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_bootstrap', '0001');

-- sources 表：资料追溯三件套（sha256 / mtime / size），见 architecture §4.2。
-- raw_id 是 vault-relative POSIX 路径（如 "raw/inbox/karpathy-llm-wiki.md"）。
-- status 状态机：pending → parsed → done | error，见 architecture §5.1。
CREATE TABLE IF NOT EXISTS sources (
    raw_id      TEXT PRIMARY KEY,
    sha256      TEXT NOT NULL,
    size        INTEGER NOT NULL,
    mtime       INTEGER NOT NULL,
    status      TEXT NOT NULL,
    ingested_at INTEGER,
    parser      TEXT,
    metadata    TEXT
);

CREATE INDEX IF NOT EXISTS idx_sources_sha256 ON sources(sha256);
CREATE INDEX IF NOT EXISTS idx_sources_status ON sources(status);

-- +goose Down
DROP INDEX IF EXISTS idx_sources_status;
DROP INDEX IF EXISTS idx_sources_sha256;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS meta;
