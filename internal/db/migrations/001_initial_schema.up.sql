-- 001_initial_schema.up.sql

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Main torrents table with JSONB file list and category
CREATE TABLE IF NOT EXISTS torrents (
    id              BIGINT PRIMARY KEY,
    title           TEXT NOT NULL,
    hash            TEXT,
    tracker_id      INTEGER,
    size            BIGINT,
    registered_at   TIMESTAMP,
    forum_id        INTEGER,
    forum_name      TEXT,
    category        TEXT,
    content         TEXT,
    file_list       JSONB,
    search_vector   tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('russian', coalesce(title, '')), 'A')
    ) STORED
);

-- GIN index for title full-text search
CREATE INDEX IF NOT EXISTS idx_torrents_search ON torrents USING GIN (search_vector);

-- GIN index for trigram-based fuzzy search on title
CREATE INDEX IF NOT EXISTS idx_torrents_title_trgm ON torrents USING GIN (title gin_trgm_ops);

-- Index for category filtering
CREATE INDEX IF NOT EXISTS idx_torrents_category ON torrents(category);

-- Old versions
CREATE TABLE IF NOT EXISTS torrent_old_versions (
    id          BIGSERIAL PRIMARY KEY,
    torrent_id  BIGINT NOT NULL REFERENCES torrents(id) ON DELETE CASCADE,
    hash        TEXT,
    title       TEXT,
    version_time TEXT,
    unixts      BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_old_ver_torrent_hash ON torrent_old_versions (torrent_id, hash, title);
CREATE INDEX IF NOT EXISTS idx_old_versions_torrent_id ON torrent_old_versions(torrent_id);

-- Duplicate candidates
CREATE TABLE IF NOT EXISTS torrent_dups (
    id              BIGSERIAL PRIMARY KEY,
    torrent_id      BIGINT NOT NULL REFERENCES torrents(id) ON DELETE CASCADE,
    dup_id          BIGINT,
    confidence      INTEGER,
    dup_title       TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dups_torrent_dup ON torrent_dups (torrent_id, dup_id);
CREATE INDEX IF NOT EXISTS idx_dups_torrent_id ON torrent_dups(torrent_id);

-- Migration checkpoint
CREATE TABLE IF NOT EXISTS migration_checkpoint (
    id              INTEGER PRIMARY KEY DEFAULT 1,
    last_torrent_id BIGINT DEFAULT 0,
    processed_count BIGINT DEFAULT 0,
    updated_at      TIMESTAMP DEFAULT NOW()
);

INSERT INTO migration_checkpoint (id, last_torrent_id, processed_count) VALUES (1, 0, 0)
ON CONFLICT (id) DO NOTHING;
