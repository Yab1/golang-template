-- Actor stamps on mutable resources (enterprise: who created/updated/deleted).

ALTER TABLE posts
  ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES users (id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS deleted_by uuid REFERENCES users (id) ON DELETE SET NULL;

UPDATE posts
SET created_by = user_id,
    updated_by = user_id
WHERE created_by IS NULL;

CREATE INDEX IF NOT EXISTS idx_posts_created_by ON posts (created_by) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_posts_updated_by ON posts (updated_by) WHERE deleted_at IS NULL;

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS created_by uuid REFERENCES users (id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS updated_by uuid REFERENCES users (id) ON DELETE SET NULL;

UPDATE users
SET created_by = id,
    updated_by = id
WHERE created_by IS NULL;
