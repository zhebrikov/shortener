DROP INDEX IF EXISTS idx_links_user_id_is_deleted;
ALTER TABLE links DROP COLUMN IF EXISTS is_deleted;
