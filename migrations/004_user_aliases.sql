CREATE TABLE IF NOT EXISTS user_aliases (
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  alias TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, alias)
);

CREATE INDEX IF NOT EXISTS idx_user_aliases_user_id ON user_aliases(user_id);
CREATE INDEX IF NOT EXISTS idx_user_aliases_alias_lower ON user_aliases((lower(alias)));
