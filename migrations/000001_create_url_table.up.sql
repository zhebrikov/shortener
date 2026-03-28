CREATE TABLE IF NOT EXISTS links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- лимит длины URL (во многих браузерах и системах ~2048 символов).
    url VARCHAR(2048) NOT NULL,
    short_url VARCHAR(100) NOT NULL UNIQUE
);

CREATE INDEX idx_links_short_url ON links(short_url);

DROP INDEX IF EXISTS idx_links_url;
CREATE UNIQUE INDEX idx_links_url ON links(url);
