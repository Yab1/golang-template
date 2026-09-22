ALTER TABLE users
  DROP COLUMN IF EXISTS updated_by,
  DROP COLUMN IF EXISTS created_by;

DROP INDEX IF EXISTS idx_posts_updated_by;
DROP INDEX IF EXISTS idx_posts_created_by;

ALTER TABLE posts
  DROP COLUMN IF EXISTS deleted_by,
  DROP COLUMN IF EXISTS updated_by,
  DROP COLUMN IF EXISTS created_by;
