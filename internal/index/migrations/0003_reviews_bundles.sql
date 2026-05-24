-- +goose Up
-- W2 D10: review queue persistence foundation.
--
-- D10 only creates the base reviews / bundles tables and indexes. propose_*
-- writes and accept/reject state transitions are wired in D11/D12.
CREATE TABLE IF NOT EXISTS reviews (
    id             TEXT PRIMARY KEY,
    seq            INTEGER NOT NULL UNIQUE,
    bundle_id      TEXT,
    agent          TEXT NOT NULL,
    session_id     TEXT NOT NULL,
    op             TEXT NOT NULL,
    target_page_id TEXT,
    patch_path     TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    created_at     TEXT NOT NULL,
    decided_at     TEXT,
    decided_by     TEXT,
    meta_json      TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews(status);
CREATE INDEX IF NOT EXISTS idx_reviews_bundle ON reviews(bundle_id);
CREATE INDEX IF NOT EXISTS idx_reviews_agent_session ON reviews(agent, session_id);

CREATE TABLE IF NOT EXISTS bundles (
    id           TEXT PRIMARY KEY,
    seq          INTEGER NOT NULL UNIQUE,
    agent        TEXT NOT NULL,
    session_id   TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'open',
    created_at   TEXT NOT NULL,
    submitted_at TEXT,
    decided_at   TEXT
);

CREATE INDEX IF NOT EXISTS idx_bundles_status ON bundles(status);
CREATE INDEX IF NOT EXISTS idx_bundles_agent_session ON bundles(agent, session_id);

-- +goose Down
DROP INDEX IF EXISTS idx_bundles_agent_session;
DROP INDEX IF EXISTS idx_bundles_status;
DROP TABLE IF EXISTS bundles;
DROP INDEX IF EXISTS idx_reviews_agent_session;
DROP INDEX IF EXISTS idx_reviews_bundle;
DROP INDEX IF EXISTS idx_reviews_status;
DROP TABLE IF EXISTS reviews;
