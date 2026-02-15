CREATE TABLE IF NOT EXISTS device_sessions (
  session_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  user_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  device_name TEXT NOT NULL DEFAULT '',
  session_token TEXT NOT NULL,
  allowed_tenants JSONB NOT NULL DEFAULT '[]'::jsonb,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (tenant_id, user_id, device_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_device_sessions_token ON device_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_device_sessions_tenant_user ON device_sessions(tenant_id, user_id, is_active);

CREATE TABLE IF NOT EXISTS user_presence (
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  user_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'offline',
  status_note TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, user_id)
);

CREATE TABLE IF NOT EXISTS notes (
  note_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  sender_user_id TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notes_tenant_created ON notes(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS note_recipients (
  note_id TEXT NOT NULL REFERENCES notes(note_id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  recipient_user_id TEXT NOT NULL,
  recipient_type TEXT NOT NULL CHECK (recipient_type IN ('to','cc','bcc')),
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (note_id, recipient_user_id)
);

CREATE INDEX IF NOT EXISTS idx_note_recipients_inbox ON note_recipients(tenant_id, recipient_user_id, is_read, created_at DESC);

CREATE TABLE IF NOT EXISTS note_files (
  note_file_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  note_id TEXT NOT NULL REFERENCES notes(note_id) ON DELETE CASCADE,
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  file_name TEXT NOT NULL,
  object_key TEXT NOT NULL,
  content_type TEXT NOT NULL DEFAULT '',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_note_files_note ON note_files(note_id, tenant_id);

CREATE TABLE IF NOT EXISTS chat_notifications (
  notification_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id) ON DELETE RESTRICT,
  room_id TEXT NOT NULL,
  message_id TEXT NOT NULL DEFAULT '',
  sender_user_id TEXT NOT NULL,
  recipient_user_id TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_notifications_user ON chat_notifications(tenant_id, recipient_user_id, is_read, created_at DESC);
