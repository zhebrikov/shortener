DROP INDEX IF EXISTS idx_links_user_id;

ALTER TABLE links DROP COLUMN IF EXISTS user_id;
