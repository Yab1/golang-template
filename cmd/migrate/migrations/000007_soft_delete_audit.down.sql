DROP TABLE IF EXISTS audit_logs;

DROP INDEX IF EXISTS idx_posts_created_id_active;
DROP INDEX IF EXISTS idx_posts_not_deleted;

ALTER TABLE posts
  DROP COLUMN IF EXISTS deleted_at;
