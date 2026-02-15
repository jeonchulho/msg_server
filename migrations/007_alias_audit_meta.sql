ALTER TABLE alias_audit
  ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_alias_audit_created_at ON alias_audit(created_at DESC);
