-- +goose Up
-- W3 D16: claim_sources table for quote_hash drift detection.
CREATE TABLE IF NOT EXISTS claim_sources (
    claim_id          TEXT NOT NULL,
    raw_id            TEXT NOT NULL,
    anchor            TEXT NOT NULL,
    stored_quote_hash TEXT NOT NULL,
    quote_preview     TEXT NOT NULL DEFAULT '',
    span_start        INTEGER,
    span_end          INTEGER,
    last_verified_at  INTEGER DEFAULT 0,
    cached_status     TEXT DEFAULT 'unknown',
    PRIMARY KEY (claim_id, raw_id, anchor)
);
CREATE INDEX IF NOT EXISTS idx_claim_sources_raw ON claim_sources(raw_id);

-- +goose Down
DROP INDEX IF EXISTS idx_claim_sources_raw;
DROP TABLE IF EXISTS claim_sources;
