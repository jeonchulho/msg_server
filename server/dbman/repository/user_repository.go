package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"msg_server/server/chat/domain"
	"msg_server/server/common/infra/db"
)

type UserRepository struct {
	router *db.TenantDBRouter
}

func NewUserRepository(router *db.TenantDBRouter) *UserRepository {
	return &UserRepository{router: router}
}

func (r *UserRepository) CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `INSERT INTO org_units(tenant_id, org_parent_id, name) VALUES($1, $2, $3) RETURNING org_id`, tenantID, parentID, name).Scan(&id)
	return id, err
}

func (r *UserRepository) ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT org_id AS id, org_parent_id AS parent_id, name, created_at FROM org_units WHERE tenant_id=$1 ORDER BY org_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrgUnit, 0)
	for rows.Next() {
		var item domain.OrgUnit
		if err := rows.Scan(&item.ID, &item.ParentID, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *UserRepository) CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO users(tenant_id, org_id, email, name, title, role, status, status_note, password_hash)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING user_id
	`, tenantID, user.OrgID, user.Email, user.Name, user.Title, user.Role, user.Status, user.StatusNote, user.PasswordHash).Scan(&id)
	return id, err
}

func (r *UserRepository) UpdateStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	cmd, err := pool.Exec(ctx, `UPDATE users SET status=$1, status_note=$2, updated_at=NOW() WHERE tenant_id=$3 AND user_id=$4`, status, note, tenantID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT user_id AS id, org_id, email, name, title, role, status, status_note, created_at, updated_at
		FROM users
		WHERE tenant_id=$1
		  AND (
			to_tsvector('simple', coalesce(name,'') || ' ' || coalesce(email,'') || ' ' || coalesce(title,'')) @@ plainto_tsquery('simple', $2)
			OR name ILIKE '%' || $2 || '%' OR email ILIKE '%' || $2 || '%'
		  )
		ORDER BY updated_at DESC
		LIMIT $3
	`, tenantID, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.User, 0)
	for rows.Next() {
		var item domain.User
		if err := rows.Scan(&item.ID, &item.OrgID, &item.Email, &item.Name, &item.Title, &item.Role, &item.Status, &item.StatusNote, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *UserRepository) GetByEmail(ctx context.Context, tenantID, email string) (domain.User, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return domain.User{}, err
	}
	var user domain.User
	err = pool.QueryRow(ctx, `
		SELECT user_id AS id, org_id, email, name, title, role, status, status_note, password_hash, created_at, updated_at
		FROM users
		WHERE tenant_id=$1 AND email=$2
	`, tenantID, email).Scan(
		&user.ID,
		&user.OrgID,
		&user.Email,
		&user.Name,
		&user.Title,
		&user.Role,
		&user.Status,
		&user.StatusNote,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}

func (r *UserRepository) ListAliases(ctx context.Context, tenantID, userID string) ([]string, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `SELECT alias FROM user_aliases WHERE tenant_id=$1 AND user_id=$2 ORDER BY alias`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		items = append(items, alias)
	}
	return items, rows.Err()
}

func (r *UserRepository) AddAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		WITH inserted AS (
			INSERT INTO user_aliases(tenant_id, user_id, alias)
			VALUES($1, $2, $3)
			ON CONFLICT DO NOTHING
			RETURNING tenant_id, user_id, alias
		)
		INSERT INTO alias_audit(tenant_id, user_id, alias, action, acted_by, ip, user_agent)
		SELECT tenant_id, user_id, alias, 'add', $2, $4, $5 FROM inserted
	`, tenantID, userID, alias, ip, userAgent)
	return err
}

func (r *UserRepository) DeleteAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		WITH deleted AS (
			DELETE FROM user_aliases
			WHERE tenant_id=$1 AND user_id=$2 AND alias=$3
			RETURNING tenant_id, user_id, alias
		)
		INSERT INTO alias_audit(tenant_id, user_id, alias, action, acted_by, ip, user_agent)
		SELECT tenant_id, user_id, alias, 'delete', $2, $4, $5 FROM deleted
	`, tenantID, userID, alias, ip, userAgent)
	return err
}

func (r *UserRepository) ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action string, cursorCreatedAt *time.Time, cursorID *string) ([]domain.AliasAudit, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	base := `
		SELECT id, user_id, alias, action, acted_by, ip, user_agent, created_at
		FROM alias_audit
		WHERE tenant_id=$1 AND user_id=$2`
	args := []any{tenantID, userID}
	index := 3

	if from != nil {
		base += fmt.Sprintf(" AND created_at >= $%d", index)
		args = append(args, *from)
		index++
	}
	if to != nil {
		base += fmt.Sprintf(" AND created_at <= $%d", index)
		args = append(args, *to)
		index++
	}
	if strings.TrimSpace(action) != "" {
		base += fmt.Sprintf(" AND action = $%d", index)
		args = append(args, strings.ToLower(strings.TrimSpace(action)))
		index++
	}
	if cursorCreatedAt != nil && cursorID != nil {
		base += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id < $%d))", index, index, index+1)
		args = append(args, *cursorCreatedAt, *cursorID)
		index += 2
	}

	base += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", index)
	args = append(args, limit)

	rows, err := pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.AliasAudit, 0)
	for rows.Next() {
		var item domain.AliasAudit
		if err := rows.Scan(&item.ID, &item.UserID, &item.Alias, &item.Action, &item.ActedBy, &item.IP, &item.UserAgent, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
