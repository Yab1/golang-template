-- Sample domain module. Delete this migration (and internal/modules/post) when forking.

CREATE TABLE IF NOT EXISTS posts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reference_id varchar(16) NOT NULL,
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  title varchar(255) NOT NULL,
  content text NOT NULL,
  tags varchar(100)[] NOT NULL DEFAULT '{}',
  version int NOT NULL DEFAULT 1,
  is_visible boolean NOT NULL DEFAULT true,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  deleted_by uuid REFERENCES users (id) ON DELETE SET NULL,
  deleted_at timestamp(0) with time zone,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS posts_reference_id_key ON posts (reference_id);
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts (user_id);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_created_by ON posts (created_by) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_posts_updated_by ON posts (updated_by) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_posts_not_deleted
  ON posts (created_at DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_posts_created_id_active
  ON posts (created_at DESC, id DESC)
  WHERE deleted_at IS NULL AND is_visible = true;
