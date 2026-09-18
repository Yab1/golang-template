DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE users
  DROP COLUMN IF EXISTS token_version;
