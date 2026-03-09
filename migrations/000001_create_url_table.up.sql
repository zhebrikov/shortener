CREATE TABLE IF NOT EXISTS links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url TEXT NOT NULL,
    short_url TEXT NOT NULL UNIQUE
);

CREATE INDEX idx_links_short_url ON links(short_url);
CREATE INDEX idx_links_url ON links(url);
