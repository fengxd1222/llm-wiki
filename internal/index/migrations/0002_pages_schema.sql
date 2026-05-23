-- +goose Up
-- W1 D4: pages + pages_fts (trigram) + sync triggers.
-- 完整 schema 见 spec-v2/docs/architecture.md §4.2，本次只引入 pages 及其全文
-- 索引；其余表（claim_sources / page_links / reviews 等）按 roadmap D5+ 增量加。
--
-- frontmatter 字段存为 JSON 文本（SQLite 无原生 JSON 类型，TEXT + 应用层 JSON）。
-- body 列冗余存储 markdown 正文，让 pages_fts 同步可由 trigger 全权完成
-- （service 层不直接动 pages_fts，杜绝双写漏同步）。
CREATE TABLE IF NOT EXISTS pages (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    confidence  REAL,
    status      TEXT,
    schema_ver  TEXT NOT NULL,
    created_by  TEXT,
    updated_by  TEXT,
    created_at  INTEGER,
    updated_at  INTEGER,
    frontmatter TEXT
);

CREATE INDEX IF NOT EXISTS idx_pages_type ON pages(type);
CREATE INDEX IF NOT EXISTS idx_pages_status ON pages(status);

-- CJK-aware 全文索引：tokenize = 'trigram'，详见 spec-v2/docs/cjk-tokenizer.md §3。
-- id 用 UNINDEXED，节省空间；title / body 入索引。
CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
    id UNINDEXED,
    title,
    body,
    tokenize = 'trigram'
);

-- 同步 triggers：pages 写入即驱动 pages_fts，避免 service 层显式双写漏同步。
-- 注意：CREATE TRIGGER ... BEGIN ... END 内含分号，goose 默认按 ; 切语句会切坏，
-- 必须用 StatementBegin / StatementEnd 包起来（见下方 goose 指令）。
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
    INSERT INTO pages_fts(id, title, body) VALUES (new.id, new.title, new.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
    DELETE FROM pages_fts WHERE id = old.id;
    INSERT INTO pages_fts(id, title, body) VALUES (new.id, new.title, new.body);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
    DELETE FROM pages_fts WHERE id = old.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS pages_ad;
DROP TRIGGER IF EXISTS pages_au;
DROP TRIGGER IF EXISTS pages_ai;
DROP TABLE IF EXISTS pages_fts;
DROP INDEX IF EXISTS idx_pages_status;
DROP INDEX IF EXISTS idx_pages_type;
DROP TABLE IF EXISTS pages;
