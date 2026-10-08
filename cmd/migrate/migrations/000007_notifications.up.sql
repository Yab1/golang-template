-- Notification templates, per-user channel preferences, and the delivery queue.
-- The API inserts a delivery row and returns. A background poller sends it.
-- Kafka is not required. External channels stay off until their env flag is on.

CREATE TABLE IF NOT EXISTS notification_templates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reference_id varchar(16) NOT NULL,

  channel varchar(16) NOT NULL
    CHECK (channel IN ('email', 'sms', 'push', 'in_app', 'telegram')),
  category varchar(32) NOT NULL
    CHECK (category IN ('welcome', 'alert', 'status', 'transactional')),
  locale varchar(16) NOT NULL DEFAULT 'en',
  subject varchar(200) NOT NULL DEFAULT '',
  body text NOT NULL,
  format varchar(16) NOT NULL DEFAULT 'text'
    CHECK (format IN ('text', 'html', 'markdown')),
  is_active boolean NOT NULL DEFAULT true,

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

CREATE UNIQUE INDEX IF NOT EXISTS notification_templates_reference_id_key
  ON notification_templates (reference_id);

CREATE UNIQUE INDEX IF NOT EXISTS notification_templates_active_key
  ON notification_templates (channel, category, locale)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS notification_preferences (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),

  user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  channel varchar(16) NOT NULL
    CHECK (channel IN ('email', 'sms', 'push', 'in_app', 'telegram')),
  category varchar(32) NOT NULL
    CHECK (category IN ('welcome', 'alert', 'status', 'transactional')),
  enabled boolean NOT NULL DEFAULT true,
  destination varchar(255) NOT NULL DEFAULT '',

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

CREATE UNIQUE INDEX IF NOT EXISTS notification_preferences_user_channel_category_key
  ON notification_preferences (user_id, channel, category)
  WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS notification_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reference_id varchar(16) NOT NULL,

  user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  channel varchar(16) NOT NULL
    CHECK (channel IN ('email', 'sms', 'push', 'in_app', 'telegram')),
  category varchar(32) NOT NULL
    CHECK (category IN ('welcome', 'alert', 'status', 'transactional')),
  template_id uuid REFERENCES notification_templates (id) ON DELETE SET NULL,

  destination varchar(255) NOT NULL DEFAULT '',
  subject varchar(200) NOT NULL DEFAULT '',
  body text NOT NULL DEFAULT '',
  format varchar(16) NOT NULL DEFAULT 'text'
    CHECK (format IN ('text', 'html', 'markdown')),

  status varchar(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
  attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  max_attempts int NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
  next_attempt_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  last_error text NOT NULL DEFAULT '',
  provider_ref varchar(128) NOT NULL DEFAULT '',
  idempotency_key varchar(128),
  read_at timestamp(0) with time zone,
  locked_until timestamp(0) with time zone,

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

CREATE UNIQUE INDEX IF NOT EXISTS notification_deliveries_reference_id_key
  ON notification_deliveries (reference_id);

CREATE UNIQUE INDEX IF NOT EXISTS notification_deliveries_idempotency_key
  ON notification_deliveries (idempotency_key, channel)
  WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_due
  ON notification_deliveries (next_attempt_at)
  WHERE deleted_at IS NULL AND status IN ('pending', 'sending');

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_inbox
  ON notification_deliveries (user_id, created_at DESC)
  WHERE deleted_at IS NULL AND channel = 'in_app' AND status = 'sent';

CREATE TABLE IF NOT EXISTS notification_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  delivery_id uuid NOT NULL REFERENCES notification_deliveries (id) ON DELETE CASCADE,
  attempt int NOT NULL CHECK (attempt >= 0),
  status varchar(16) NOT NULL
    CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
  error text NOT NULL DEFAULT '',
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_attempts_delivery
  ON notification_attempts (delivery_id, attempt);

INSERT INTO notification_templates (reference_id, channel, category, subject, body, format)
VALUES
  ('GTL-NTM-WEL01', 'email', 'welcome', 'Welcome', '<p>Hello {{.name}}</p><p>{{.message}}</p>', 'html'),
  ('GTL-NTM-WEL02', 'sms', 'welcome', '', 'Hello {{.name}}. {{.message}}', 'text'),
  ('GTL-NTM-WEL03', 'push', 'welcome', 'Welcome', '{{.message}}', 'text'),
  ('GTL-NTM-WEL04', 'in_app', 'welcome', 'Welcome', '{{.message}}', 'text'),
  ('GTL-NTM-WEL05', 'telegram', 'welcome', 'Welcome', 'Hello {{.name}}. {{.message}}', 'markdown'),
  ('GTL-NTM-ALT01', 'email', 'alert', 'Alert', '<p>{{.message}}</p>', 'html'),
  ('GTL-NTM-ALT02', 'sms', 'alert', '', '{{.message}}', 'text'),
  ('GTL-NTM-ALT03', 'push', 'alert', 'Alert', '{{.message}}', 'text'),
  ('GTL-NTM-ALT04', 'in_app', 'alert', 'Alert', '{{.message}}', 'text'),
  ('GTL-NTM-ALT05', 'telegram', 'alert', 'Alert', '{{.message}}', 'markdown'),
  ('GTL-NTM-STA01', 'email', 'status', 'Status update', '<p>{{.message}}</p>', 'html'),
  ('GTL-NTM-STA02', 'sms', 'status', '', '{{.message}}', 'text'),
  ('GTL-NTM-STA03', 'push', 'status', 'Status update', '{{.message}}', 'text'),
  ('GTL-NTM-STA04', 'in_app', 'status', 'Status update', '{{.message}}', 'text'),
  ('GTL-NTM-STA05', 'telegram', 'status', 'Status update', '{{.message}}', 'markdown'),
  ('GTL-NTM-TXN01', 'email', 'transactional', 'Notice', '<p>{{.message}}</p>', 'html'),
  ('GTL-NTM-TXN02', 'sms', 'transactional', '', '{{.message}}', 'text'),
  ('GTL-NTM-TXN03', 'push', 'transactional', 'Notice', '{{.message}}', 'text'),
  ('GTL-NTM-TXN04', 'in_app', 'transactional', 'Notice', '{{.message}}', 'text'),
  ('GTL-NTM-TXN05', 'telegram', 'transactional', 'Notice', '{{.message}}', 'markdown')
ON CONFLICT (reference_id) DO NOTHING;
