CREATE TABLE IF NOT EXISTS roles (
  id bigserial PRIMARY KEY,
  name varchar(255) NOT NULL UNIQUE,
  level int NOT NULL DEFAULT 0,
  description text
);

INSERT INTO roles (name, description, level)
VALUES
  ('user', 'Can create and manage own posts', 1),
  ('moderator', 'Can update other users posts', 2),
  ('admin', 'Can update and delete other users posts', 3)
ON CONFLICT (name) DO NOTHING;

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS role_id bigint REFERENCES roles (id);

UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

ALTER TABLE users
  ALTER COLUMN role_id SET NOT NULL;
