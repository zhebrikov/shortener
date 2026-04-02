ALTER TABLE links ADD COLUMN IF NOT EXISTS user_id UUID NULL;

CREATE INDEX idx_links_user_id ON links(user_id);
