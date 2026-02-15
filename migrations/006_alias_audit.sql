CREATE TABLE IF NOT EXISTS alias_audit (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  alias TEXT NOT NULL,
  action TEXT NOT NULL CHECK (action IN ('add', 'delete')),
  acted_by TEXT NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alias_audit_user_created ON alias_audit(user_id, created_at DESC);
