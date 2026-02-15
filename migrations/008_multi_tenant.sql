CREATE TABLE IF NOT EXISTS tenants (
  tenant_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  deployment_mode TEXT NOT NULL DEFAULT 'shared' CHECK (deployment_mode IN ('shared', 'dedicated')),
  dedicated_dsn TEXT NOT NULL DEFAULT '',
  user_count_threshold INTEGER NOT NULL DEFAULT 200,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO tenants (tenant_id, name, deployment_mode)
VALUES ('default', 'Default Tenant', 'shared')
ON CONFLICT (tenant_id) DO NOTHING;

ALTER TABLE org_units ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE chat_rooms ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE room_members ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE files ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE message_reads ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE user_aliases ADD COLUMN IF NOT EXISTS tenant_id TEXT;
ALTER TABLE alias_audit ADD COLUMN IF NOT EXISTS tenant_id TEXT;

UPDATE org_units
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE users u
SET tenant_id = COALESCE(ou.tenant_id, 'default')
FROM org_units ou
WHERE u.tenant_id IS NULL
  AND ou.org_id = u.org_id;

UPDATE users
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE chat_rooms cr
SET tenant_id = COALESCE(u.tenant_id, 'default')
FROM users u
WHERE cr.tenant_id IS NULL
  AND u.user_id = cr.created_by;

UPDATE chat_rooms
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE room_members rm
SET tenant_id = COALESCE(cr.tenant_id, 'default')
FROM chat_rooms cr
WHERE rm.tenant_id IS NULL
  AND cr.chat_room_id = rm.room_id;

UPDATE room_members
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE messages m
SET tenant_id = COALESCE(cr.tenant_id, 'default')
FROM chat_rooms cr
WHERE m.tenant_id IS NULL
  AND cr.chat_room_id = m.room_id;

UPDATE messages
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE files f
SET tenant_id = COALESCE(cr.tenant_id, 'default')
FROM chat_rooms cr
WHERE f.tenant_id IS NULL
  AND cr.chat_room_id = f.room_id;

UPDATE files
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE message_reads mr
SET tenant_id = COALESCE(m.tenant_id, 'default')
FROM messages m
WHERE mr.tenant_id IS NULL
  AND m.message_id = mr.message_id;

UPDATE message_reads
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE user_aliases ua
SET tenant_id = COALESCE(u.tenant_id, 'default')
FROM users u
WHERE ua.tenant_id IS NULL
  AND u.user_id = ua.user_id;

UPDATE user_aliases
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

UPDATE alias_audit aa
SET tenant_id = COALESCE(u.tenant_id, 'default')
FROM users u
WHERE aa.tenant_id IS NULL
  AND u.user_id = aa.user_id;

UPDATE alias_audit
SET tenant_id = 'default'
WHERE tenant_id IS NULL;

ALTER TABLE org_units ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE chat_rooms ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE room_members ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE messages ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE files ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE message_reads ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE user_aliases ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE alias_audit ALTER COLUMN tenant_id SET NOT NULL;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_org_units_tenant') THEN
    ALTER TABLE org_units ADD CONSTRAINT fk_org_units_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_users_tenant') THEN
    ALTER TABLE users ADD CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_chat_rooms_tenant') THEN
    ALTER TABLE chat_rooms ADD CONSTRAINT fk_chat_rooms_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_room_members_tenant') THEN
    ALTER TABLE room_members ADD CONSTRAINT fk_room_members_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_messages_tenant') THEN
    ALTER TABLE messages ADD CONSTRAINT fk_messages_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_files_tenant') THEN
    ALTER TABLE files ADD CONSTRAINT fk_files_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_message_reads_tenant') THEN
    ALTER TABLE message_reads ADD CONSTRAINT fk_message_reads_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_user_aliases_tenant') THEN
    ALTER TABLE user_aliases ADD CONSTRAINT fk_user_aliases_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_alias_audit_tenant') THEN
    ALTER TABLE alias_audit ADD CONSTRAINT fk_alias_audit_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE RESTRICT;
  END IF;
END $$;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_tenant_email ON users(tenant_id, email);

CREATE INDEX IF NOT EXISTS idx_org_units_tenant_id ON org_units(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_chat_rooms_tenant_id ON chat_rooms(tenant_id);
CREATE INDEX IF NOT EXISTS idx_room_members_tenant_id ON room_members(tenant_id);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_files_tenant_id ON files(tenant_id);
CREATE INDEX IF NOT EXISTS idx_message_reads_tenant_id ON message_reads(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_aliases_tenant_id ON user_aliases(tenant_id);
CREATE INDEX IF NOT EXISTS idx_alias_audit_tenant_id ON alias_audit(tenant_id);
