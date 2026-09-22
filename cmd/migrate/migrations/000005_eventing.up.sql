CREATE TABLE IF NOT EXISTS event_outbox (
  id uuid PRIMARY KEY,
  topic text NOT NULL,
  event_key text NOT NULL,
  event_type text NOT NULL,
  aggregate_type text NOT NULL,
  aggregate_id text NOT NULL,
  aggregate_version bigint NOT NULL DEFAULT 1 CHECK (aggregate_version > 0),
  schema_uri text NOT NULL,
  payload jsonb NOT NULL,
  headers jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamp(0) with time zone NOT NULL,
  available_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts int NOT NULL DEFAULT 20 CHECK (max_attempts > 0),
  published_at timestamp(0) with time zone,
  locked_at timestamp(0) with time zone,
  locked_by text,
  last_error text,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_outbox_pending
  ON event_outbox (available_at, occurred_at)
  WHERE published_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_event_outbox_aggregate
  ON event_outbox (aggregate_type, aggregate_id, aggregate_version);

CREATE TABLE IF NOT EXISTS event_inbox (
  consumer_name text NOT NULL,
  event_id uuid NOT NULL,
  topic text NOT NULL,
  partition_id int NOT NULL,
  offset_id bigint NOT NULL,
  processed_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  PRIMARY KEY (consumer_name, event_id)
);

CREATE INDEX IF NOT EXISTS idx_event_inbox_processed
  ON event_inbox (processed_at);

CREATE TABLE IF NOT EXISTS event_inbox_watermarks (
  consumer_name text NOT NULL,
  subject text NOT NULL,
  last_version bigint NOT NULL CHECK (last_version > 0),
  updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  PRIMARY KEY (consumer_name, subject)
);

CREATE TABLE IF NOT EXISTS file_objects (
  key text PRIMARY KEY,
  owner_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  driver text NOT NULL,
  version bigint NOT NULL DEFAULT 1,
  is_visible boolean NOT NULL DEFAULT true,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users (id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
  deleted_by uuid REFERENCES users (id) ON DELETE SET NULL,
  deleted_at timestamp(0) with time zone,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_file_objects_owner
  ON file_objects (owner_id)
  WHERE deleted_at IS NULL;
