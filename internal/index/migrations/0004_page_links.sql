-- +goose Up
-- W2 D14: page_links table for graph inbound queries and health score orphan detection.
CREATE TABLE IF NOT EXISTS page_links (
    source_id   TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    link_type   TEXT NOT NULL DEFAULT 'ref',
    PRIMARY KEY (source_id, target_id, link_type)
);

CREATE INDEX IF NOT EXISTS idx_page_links_target ON page_links(target_id);

-- +goose Down
DROP INDEX IF EXISTS idx_page_links_target;
DROP TABLE IF EXISTS page_links;
