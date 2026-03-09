DROP INDEX IF EXISTS idx_links_url;
CREATE UNIQUE INDEX idx_links_url ON links(url);
