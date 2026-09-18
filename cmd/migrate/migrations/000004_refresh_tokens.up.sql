CREATE TABLE IF NOT EXISTS refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  jti uuid NOT NULL UNIQUE,
  expires_at timestamp(0) with time zone NOT NULL,
  revoked_at timestamp(0) with time zone,
  replaced_by uuid,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_jti ON refresh_tokens (jti);

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS token_version int NOT NULL DEFAULT 1;
