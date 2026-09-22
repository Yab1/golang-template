-- Opaque auth tokens (email verify + password reset).

CREATE TABLE IF NOT EXISTS user_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  purpose text NOT NULL CHECK (purpose IN ('email_verify', 'password_reset')),
  expires_at timestamp(0) with time zone NOT NULL,
  used_at timestamp(0) with time zone,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_tokens_user_purpose
  ON user_tokens (user_id, purpose)
  WHERE used_at IS NULL;
