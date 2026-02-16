package repository

import (
	"context"
	"fmt"
	"time"

	"msg_server/server/chat/domain"
	"msg_server/server/common/infra/db"
)

type ChatRepository struct {
	router *db.TenantDBRouter
}

func NewChatRepository(router *db.TenantDBRouter) *ChatRepository {
	return &ChatRepository{router: router}
}

func (r *ChatRepository) CreateRoom(ctx context.Context, tenantID string, room domain.ChatRoom, memberIDs []string) (string, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var roomID string
	err = tx.QueryRow(ctx, `INSERT INTO chat_rooms(tenant_id, name, room_type, created_by) VALUES($1, $2, $3, $4) RETURNING chat_room_id`, tenantID, room.Name, room.RoomType, room.CreatedBy).Scan(&roomID)
	if err != nil {
		return "", err
	}

	for _, userID := range memberIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO room_members(tenant_id, room_id, user_id) VALUES($1, $2, $3) ON CONFLICT DO NOTHING`, tenantID, roomID, userID); err != nil {
			return "", err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO room_members(tenant_id, room_id, user_id) VALUES($1, $2, $3) ON CONFLICT DO NOTHING`, tenantID, roomID, room.CreatedBy); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return roomID, nil
}

func (r *ChatRepository) AddMember(ctx context.Context, tenantID, roomID, userID string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO room_members(tenant_id, room_id, user_id) VALUES($1, $2, $3) ON CONFLICT DO NOTHING`, tenantID, roomID, userID)
	return err
}

func (r *ChatRepository) IsRoomMember(ctx context.Context, tenantID, roomID, userID string) (bool, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	var exists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM room_members
			WHERE tenant_id=$1 AND room_id=$2 AND user_id=$3
		)
	`, tenantID, roomID, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *ChatRepository) CreateMessage(ctx context.Context, message domain.Message) (domain.Message, error) {
	pool, err := r.router.DBForTenant(ctx, message.TenantID)
	if err != nil {
		return message, err
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO messages(tenant_id, room_id, sender_id, body, meta_json)
		VALUES($1, $2, $3, $4, $5)
		RETURNING message_id, created_at
	`, message.TenantID, message.RoomID, message.SenderID, message.Body, message.MetaJSON).Scan(&message.ID, &message.CreatedAt)
	return message, err
}

func (r *ChatRepository) ListMessages(ctx context.Context, tenantID, roomID string, limit int, cursorID *string) ([]domain.Message, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	base := `
		SELECT message_id AS id, room_id, sender_id, body, meta_json, created_at
		FROM messages
		WHERE tenant_id=$1 AND room_id=$2`
	args := []any{tenantID, roomID}

	if cursorID != nil {
		base += ` AND message_id < $3`
		args = append(args, *cursorID)
		base += ` ORDER BY message_id DESC LIMIT $4`
		args = append(args, limit)
	} else {
		base += ` ORDER BY message_id DESC LIMIT $3`
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Body, &m.MetaJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *ChatRepository) SearchMessages(ctx context.Context, tenantID string, q string, roomID *string, limit int, cursorID *string) ([]domain.Message, error) {
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

	items := make([]domain.Message, 0)
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.SenderID, &m.Body, &m.MetaJSON, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *ChatRepository) MarkReadUpTo(ctx context.Context, tenantID, roomID, userID, messageID string) error {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	cmd, err := pool.Exec(ctx, `
		INSERT INTO message_reads(tenant_id, room_id, message_id, user_id, read_at)
		SELECT m.tenant_id, m.room_id, m.message_id, $3, NOW()
		FROM messages m
		WHERE m.tenant_id=$1 AND m.room_id=$2
		  AND (m.created_at, m.message_id) <= (
			SELECT m2.created_at, m2.message_id
			FROM messages m2
			WHERE m2.tenant_id=$1 AND m2.message_id=$4 AND m2.room_id=$2
		  )
		ON CONFLICT (message_id, user_id)
		DO UPDATE SET read_at=EXCLUDED.read_at
	`, tenantID, roomID, userID, messageID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("no messages marked as read")
	}
	return nil
}

func (r *ChatRepository) GetMessageReaders(ctx context.Context, tenantID, roomID, messageID string) ([]domain.MessageRead, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT room_id, message_id, user_id, read_at
		FROM message_reads
		WHERE tenant_id=$1 AND room_id=$2 AND message_id=$3
		ORDER BY read_at ASC
	`, tenantID, roomID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.MessageRead, 0)
	for rows.Next() {
		var item domain.MessageRead
		if err := rows.Scan(&item.RoomID, &item.MessageID, &item.UserID, &item.ReadAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ChatRepository) GetLastReadMessageID(ctx context.Context, tenantID, roomID, userID string) (string, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return "", err
	}
	var messageID string
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(
			(
				SELECT mr.message_id
				FROM message_reads mr
				WHERE mr.tenant_id=$1 AND mr.room_id=$2 AND mr.user_id=$3
				ORDER BY mr.read_at DESC, mr.message_id DESC
				LIMIT 1
			),
			''
		)
	`, tenantID, roomID, userID).Scan(&messageID)
	return messageID, err
}

func (r *ChatRepository) GetUnreadCount(ctx context.Context, tenantID, roomID, userID string) (int64, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	var count int64
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)::BIGINT
		FROM messages m
		WHERE m.tenant_id=$1
		  AND m.room_id=$2
		  AND m.sender_id <> $3
		  AND m.created_at > COALESCE(
			(SELECT MAX(mr.read_at) FROM message_reads mr WHERE mr.tenant_id=$1 AND mr.room_id=$2 AND mr.user_id=$3),
			TIMESTAMPTZ 'epoch'
		  )
	`, tenantID, roomID, userID).Scan(&count)
	return count, err
}

func (r *ChatRepository) GetUnreadCounts(ctx context.Context, tenantID, userID string) ([]domain.RoomUnread, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT
			rm.room_id,
			COALESCE((
				SELECT COUNT(*)::BIGINT
				FROM messages m
				WHERE m.tenant_id = $1
				  AND m.room_id = rm.room_id
				  AND m.sender_id <> $2
				  AND m.created_at > COALESCE((
					SELECT MAX(mr.read_at)
					FROM message_reads mr
					WHERE mr.tenant_id = $1 AND mr.room_id = rm.room_id AND mr.user_id = $2
				  ), TIMESTAMPTZ 'epoch')
			), 0) AS unread_count
		FROM room_members rm
		WHERE rm.tenant_id = $1 AND rm.user_id = $2
		ORDER BY rm.room_id
	`, tenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.RoomUnread, 0)
	for rows.Next() {
		var item domain.RoomUnread
		if err := rows.Scan(&item.RoomID, &item.UnreadCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ChatRepository) ListMyRooms(ctx context.Context, tenantID, userID string, limit int, cursorCreatedAt *time.Time, cursorRoomID *string) ([]domain.ChatRoomSummary, error) {
	pool, err := r.router.DBForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT
			cr.chat_room_id,
			cr.name,
			cr.room_type,
			cr.created_by,
			cr.created_at,
			pu.user_id,
			pu.name,
			pu.status,
			pu.status_note,
			lm.id,
			lm.body,
			CASE
				WHEN COALESCE(lm.meta_json->>'file_id', '') <> ''
				  OR (jsonb_typeof(lm.meta_json->'file_ids') = 'array' AND jsonb_array_length(lm.meta_json->'file_ids') > 0)
				THEN 'file'
				WHEN jsonb_typeof(lm.meta_json->'emojis') = 'array' AND jsonb_array_length(lm.meta_json->'emojis') > 0 THEN 'emoji'
				ELSE 'text'
			END AS latest_message_kind,
			CASE
				WHEN COALESCE(lm.meta_json->>'file_id', '') <> ''
				  OR (jsonb_typeof(lm.meta_json->'file_ids') = 'array' AND jsonb_array_length(lm.meta_json->'file_ids') > 0)
				THEN '[파일]'
				WHEN jsonb_typeof(lm.meta_json->'emojis') = 'array' AND jsonb_array_length(lm.meta_json->'emojis') > 0 AND COALESCE(lm.body, '') = '' THEN '[이모지]'
				WHEN COALESCE(lm.body, '') = '' THEN ''
				ELSE LEFT(lm.body, 120)
			END AS latest_message_summary,
			CASE
				WHEN lm.id IS NULL THEN ARRAY[]::TEXT[]
				ELSE ARRAY(
					SELECT DISTINCT m[1]
					FROM regexp_matches(COALESCE(lm.body, ''), '@([[:alnum:]_가-힣]+)', 'g') AS m
				)
			END AS latest_message_mention_tokens,
			CASE
				WHEN lm.id IS NULL THEN false
				ELSE EXISTS (
					SELECT 1
					FROM regexp_matches(COALESCE(lm.body, ''), '@([[:alnum:]_가-힣]+)', 'g') AS m
					WHERE lower(m[1]) = lower(COALESCE(me.name, ''))
					   OR lower(m[1]) = lower(split_part(COALESCE(me.email, ''), '@', 1))
					   OR lower(m[1]) = ANY(ma.aliases)
				)
			END AS latest_message_is_mentioned,
			lm.created_at,
			lm.sender_id,
			COALESCE((
				SELECT COUNT(*)::BIGINT
				FROM messages m2
				WHERE m2.tenant_id = $1
				  AND m2.room_id = cr.chat_room_id
				  AND m2.sender_id <> $2
				  AND m2.created_at > COALESCE((
					SELECT MAX(mr.read_at)
					FROM message_reads mr
					WHERE mr.tenant_id = $1 AND mr.room_id = cr.chat_room_id AND mr.user_id = $2
				  ), TIMESTAMPTZ 'epoch')
			), 0) AS unread_count
		FROM room_members rm
		JOIN chat_rooms cr ON cr.tenant_id = $1 AND cr.chat_room_id = rm.room_id
		JOIN users me ON me.tenant_id = $1 AND me.user_id = $2
		LEFT JOIN LATERAL (
			SELECT COALESCE(array_agg(lower(ua.alias)), ARRAY[]::text[]) AS aliases
			FROM user_aliases ua
			WHERE ua.tenant_id = $1 AND ua.user_id = $2
		) ma ON true
		LEFT JOIN LATERAL (
			SELECT u.user_id AS user_id, u.name, u.status, u.status_note
			FROM room_members rm2
			JOIN users u ON u.tenant_id = $1 AND u.user_id = rm2.user_id
			WHERE rm2.tenant_id = $1 AND rm2.room_id = cr.chat_room_id AND rm2.user_id <> $2
			ORDER BY rm2.joined_at ASC
			LIMIT 1
		) pu ON cr.room_type = 'direct'
		LEFT JOIN LATERAL (
			SELECT m.message_id AS id, m.body, m.meta_json, m.created_at, m.sender_id
			FROM messages m
			WHERE m.tenant_id = $1 AND m.room_id = cr.chat_room_id
			ORDER BY m.message_id DESC
			LIMIT 1
		) lm ON true
		WHERE rm.tenant_id = $1 AND rm.user_id = $2`

	args := []any{tenantID, userID}
	if cursorCreatedAt != nil && cursorRoomID != nil {
		query += `
		  AND (
			COALESCE(lm.created_at, cr.created_at) < $3
			OR (COALESCE(lm.created_at, cr.created_at) = $3 AND cr.chat_room_id < $4)
		  )`
		args = append(args, *cursorCreatedAt, *cursorRoomID)
		query += `
		ORDER BY COALESCE(lm.created_at, cr.created_at) DESC, cr.chat_room_id DESC
		LIMIT $5`
		args = append(args, limit)
	} else {
		query += `
		ORDER BY COALESCE(lm.created_at, cr.created_at) DESC, cr.chat_room_id DESC
		LIMIT $3`
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ChatRoomSummary, 0)
	for rows.Next() {
		var item domain.ChatRoomSummary
		if err := rows.Scan(
			&item.RoomID,
			&item.Name,
			&item.RoomType,
			&item.CreatedBy,
			&item.CreatedAt,
			&item.PeerUserID,
			&item.PeerName,
			&item.PeerStatus,
			&item.PeerStatusNote,
			&item.LatestMessageID,
			&item.LatestMessageBody,
			&item.LatestMessageKind,
			&item.LatestMessageSummary,
			&item.LatestMessageMentionTokens,
			&item.LatestMessageIsMentioned,
			&item.LatestMessageAt,
			&item.LatestMessageSender,
			&item.UnreadCount,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
