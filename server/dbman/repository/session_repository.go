package repository

import (
	"context"
	"encoding/json"
	"errors"

	"msg_server/server/common/infra/db"
	"msg_server/server/session/domain"

	"github.com/jackc/pgx/v5"
)

type SessionRepository struct {
	router *db.TenantDBRouter
}

func NewSessionRepository(router *db.TenantDBRouter) *SessionRepository {
	return &SessionRepository{router: router}
}

func (r *SessionRepository) UpsertDeviceSession(ctx context.Context, session domain.DeviceSession) (domain.DeviceSession, error) {
	pool, err := r.router.DBForTenant(ctx, session.TenantID)
	if err != nil {
		return domain.DeviceSession{}, err
	}
	allowed, err := json.Marshal(session.AllowedTenants)
	if err != nil {
		return domain.DeviceSession{}, err
	}

	var out domain.DeviceSession
	var allowedRaw []byte
	err = pool.QueryRow(ctx, `
		INSERT INTO device_sessions(tenant_id, user_id, device_id, device_name, session_token, allowed_tenants)
		VALUES($1, $2, $3, $4, $5, $6::jsonb)
		ON CONFLICT (tenant_id, user_id, device_id)
		DO UPDATE SET
			device_name = EXCLUDED.device_name,
			session_token = EXCLUDED.session_token,
			allowed_tenants = EXCLUDED.allowed_tenants,
			is_active = TRUE,
			last_seen_at = NOW(),
			updated_at = NOW()
		RETURNING session_id, tenant_id, user_id, device_id, device_name, session_token, allowed_tenants, last_seen_at, created_at
	`, session.TenantID, session.UserID, session.DeviceID, session.DeviceName, session.SessionToken, string(allowed)).
		Scan(
			&out.SessionID,
			&out.TenantID,
			&out.UserID,
			&out.DeviceID,
			&out.DeviceName,
			&out.SessionToken,
			&allowedRaw,
			&out.LastSeenAt,
			&out.CreatedAt,
		)
	if err != nil {
		return domain.DeviceSession{}, err
	}
	if len(allowedRaw) > 0 {
		_ = json.Unmarshal(allowedRaw, &out.AllowedTenants)
	}
	return out, nil
}

func (r *SessionRepository) ValidateAndTouchSession(ctx context.Context, tenantID, userID, sessionID, sessionToken string) (bool, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	var matched bool
	err = pool.QueryRow(ctx, `
		UPDATE device_sessions
		SET last_seen_at = NOW(), updated_at = NOW()
		WHERE tenant_id=$1
		  AND user_id=$2
		  AND session_id=$3
		  AND session_token=$4
		  AND is_active=TRUE
		RETURNING TRUE
	`, tenantID, userID, sessionID, sessionToken).Scan(&matched)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return matched, nil
}

func (r *SessionRepository) UpdateUserStatus(ctx context.Context, status domain.UserStatus) error {
	pool, err := r.router.DBForTenant(ctx, status.TenantID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO user_presence(tenant_id, user_id, status, status_note)
		VALUES($1, $2, $3, $4)
		ON CONFLICT (tenant_id, user_id)
		DO UPDATE SET status=EXCLUDED.status, status_note=EXCLUDED.status_note, updated_at=NOW()
	`, status.TenantID, status.UserID, status.Status, status.StatusNote)
	return err
}

func (r *SessionRepository) CreateNote(ctx context.Context, note domain.Note) (domain.Note, error) {
	pool, err := r.router.DBForTenant(ctx, note.TenantID)
	if err != nil {
		return domain.Note{}, err
	}
	if len(note.Recipients) == 0 {
		return domain.Note{}, errors.New("at least one recipient is required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return domain.Note{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		INSERT INTO notes(tenant_id, sender_user_id, title, body)
		VALUES($1, $2, $3, $4)
		RETURNING note_id, created_at
	`, note.TenantID, note.SenderUserID, note.Title, note.Body).Scan(&note.NoteID, &note.CreatedAt)
	if err != nil {
		return domain.Note{}, err
	}

	for _, recipient := range note.Recipients {
		_, err := tx.Exec(ctx, `
			INSERT INTO note_recipients(note_id, tenant_id, recipient_user_id, recipient_type)
			VALUES($1, $2, $3, $4)
		`, note.NoteID, note.TenantID, recipient.UserID, recipient.Type)
		if err != nil {
			return domain.Note{}, err
		}
	}

	for _, file := range note.Files {
		_, err := tx.Exec(ctx, `
			INSERT INTO note_files(note_id, tenant_id, file_name, object_key, content_type, size_bytes)
			VALUES($1, $2, $3, $4, $5, $6)
		`, note.NoteID, note.TenantID, file.FileName, file.ObjectKey, file.ContentType, file.SizeBytes)
		if err != nil {
			return domain.Note{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Note{}, err
	}
	return note, nil
}

func (r *SessionRepository) ListInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT n.note_id, n.sender_user_id, nr.recipient_type, n.title, n.body,
		       COALESCE((SELECT COUNT(*) FROM note_files nf WHERE nf.note_id = n.note_id AND nf.tenant_id = n.tenant_id), 0) AS file_count,
		       nr.is_read, n.created_at
		FROM note_recipients nr
		JOIN notes n ON n.note_id = nr.note_id AND n.tenant_id = nr.tenant_id
		WHERE nr.tenant_id = $1 AND nr.recipient_user_id = $2
		ORDER BY n.created_at DESC
		LIMIT $3
	`, tenantID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.NoteInboxItem, 0)
	for rows.Next() {
		var item domain.NoteInboxItem
		if err := rows.Scan(&item.NoteID, &item.SenderUserID, &item.RecipientType, &item.Title, &item.Body, &item.FileCount, &item.IsRead, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SessionRepository) MarkNoteRead(ctx context.Context, tenantID, userID, noteID string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		UPDATE note_recipients
		SET is_read = TRUE, read_at = NOW()
		WHERE tenant_id=$1 AND recipient_user_id=$2 AND note_id=$3
	`, tenantID, userID, noteID)
	return err
}

func (r *SessionRepository) SaveChatNotifications(ctx context.Context, tenantID, senderUserID string, input domain.ChatNotifyInput) error {
	if len(input.RecipientIDs) == 0 {
		return nil
	}
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, recipientID := range input.RecipientIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO chat_notifications(tenant_id, room_id, message_id, sender_user_id, recipient_user_id, title, body)
			VALUES($1, $2, $3, $4, $5, $6, $7)
		`, tenantID, input.RoomID, input.MessageID, senderUserID, recipientID, input.Title, input.Body)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
