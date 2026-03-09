DROP INDEX IF EXISTS idx_links_url;
CREATE INDEX idx_links_url ON links(url);
