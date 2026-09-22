CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS roles (
  id bigserial PRIMARY KEY,
  name varchar(255) NOT NULL UNIQUE,
  level int NOT NULL DEFAULT 0,
  description text
);

INSERT INTO roles (name, description, level)
VALUES
  ('user', 'Can create and manage own resources', 1),
  ('moderator', 'Can update other users resources', 2),
  ('admin', 'Can update and delete other users resources', 3)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reference_id varchar(16) NOT NULL,
  email citext UNIQUE NOT NULL,
  username varchar(255) UNIQUE NOT NULL,
  password bytea NOT NULL,
  is_active boolean NOT NULL DEFAULT false,
  is_visible boolean NOT NULL DEFAULT true,
  role_id bigint NOT NULL REFERENCES roles (id),
  token_version int NOT NULL DEFAULT 1,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS users_reference_id_key ON users (reference_id);
CREATE INDEX IF NOT EXISTS idx_users_created_by ON users (created_by);
CREATE INDEX IF NOT EXISTS idx_users_is_visible ON users (is_visible) WHERE is_visible = true;
