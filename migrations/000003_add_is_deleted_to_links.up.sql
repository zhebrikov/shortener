ALTER TABLE links ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_links_user_id_is_deleted ON links(user_id, is_deleted);
