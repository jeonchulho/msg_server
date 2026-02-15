WITH ranked AS (
  SELECT ctid,
         user_id,
         alias,
         row_number() OVER (
           PARTITION BY user_id, lower(alias)
           ORDER BY created_at ASC, alias ASC
         ) AS rn
  FROM user_aliases
)
DELETE FROM user_aliases ua
USING ranked r
WHERE ua.ctid = r.ctid
  AND r.rn > 1;

UPDATE user_aliases
SET alias = lower(alias)
WHERE alias <> lower(alias);

CREATE UNIQUE INDEX IF NOT EXISTS ux_user_aliases_user_lower_alias
  ON user_aliases(user_id, lower(alias));
