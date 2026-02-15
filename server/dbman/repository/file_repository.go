package repository

import (
	"context"
	"fmt"

	chatdomain "msg_server/server/chat/domain"
	"msg_server/server/common/infra/db"
	"msg_server/server/dbman/domain"
)

type FileRepository struct {
	router *db.TenantDBRouter
}

func NewFileRepository(router *db.TenantDBRouter) *FileRepository {
	return &FileRepository{router: router}
}

func (r *FileRepository) Create(ctx context.Context, item domain.FileObject) (domain.FileObject, error) {
	pool, err := r.router.DBForTenant(ctx, item.TenantID)
	if err != nil {
		return item, err
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO files(tenant_id, room_id, uploader_id, object_key, content_type, size_bytes, thumbnail_key, original_name)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, item.TenantID, item.RoomID, item.UploaderID, item.ObjectKey, item.ContentType, item.SizeBytes, item.ThumbnailKey, item.OriginalName).Scan(&item.ID, &item.CreatedAt)
	return item, err
}

func (r *FileRepository) SearchByRoom(ctx context.Context, tenantID, roomID string, limit int) ([]domain.FileObject, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT tenant_id, id, room_id, uploader_id, object_key, content_type, size_bytes, thumbnail_key, original_name, created_at
		FROM files
		WHERE tenant_id=$1 AND room_id=$2
		ORDER BY created_at DESC
		LIMIT $3
	`, tenantID, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.FileObject, 0)
	for rows.Next() {
		var item domain.FileObject
		if err := rows.Scan(&item.TenantID, &item.ID, &item.RoomID, &item.UploaderID, &item.ObjectKey, &item.ContentType, &item.SizeBytes, &item.ThumbnailKey, &item.OriginalName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *FileRepository) SearchMessages(ctx context.Context, tenantID string, q string, roomID *string, limit int, cursorID *string) ([]chatdomain.Message, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	base := `
		SELECT message_id AS id, room_id, sender_id, body, meta_json, created_at
		FROM messages
		WHERE tenant_id=$1
		  AND (to_tsvector('simple', coalesce(body,'')) @@ plainto_tsquery('simple', $2) OR body ILIKE '%' || $2 || '%')`
	args := []any{tenantID, q}
	idx := 3

	if roomID != nil {
		base += fmt.Sprintf(` AND room_id=$%d`, idx)
		args = append(args, *roomID)
		idx++
	}

	if cursorID != nil {
		base += fmt.Sprintf(` AND message_id < $%d`, idx)
		args = append(args, *cursorID)
		idx++
	}

	base += fmt.Sprintf(` ORDER BY message_id DESC LIMIT $%d`, idx)
	args = append(args, limit)

	rows, err := pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]chatdomain.Message, 0)
	for rows.Next() {
		var m chatdomain.Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Body, &m.MetaJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
