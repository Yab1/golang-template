DROP INDEX IF EXISTS posts_reference_id_key;
ALTER TABLE posts DROP COLUMN IF EXISTS reference_id;

DROP INDEX IF EXISTS users_reference_id_key;
ALTER TABLE users DROP COLUMN IF EXISTS reference_id;
