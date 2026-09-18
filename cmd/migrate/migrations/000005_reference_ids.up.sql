ALTER TABLE users
  ADD COLUMN IF NOT EXISTS reference_id varchar(16);

UPDATE users
SET reference_id = 'GTL-USR-' || upper(substr(md5(id::text), 1, 5))
WHERE reference_id IS NULL;

ALTER TABLE users
  ALTER COLUMN reference_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_reference_id_key ON users (reference_id);

ALTER TABLE posts
  ADD COLUMN IF NOT EXISTS reference_id varchar(16);

UPDATE posts
SET reference_id = 'GTL-PST-' || upper(substr(md5(id::text), 1, 5))
WHERE reference_id IS NULL;

ALTER TABLE posts
  ALTER COLUMN reference_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS posts_reference_id_key ON posts (reference_id);
