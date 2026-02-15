package domain

import "time"

type DeviceSession struct {
	SessionID      string    `json:"session_id"`
	TenantID       string    `json:"tenant_id"`
	UserID         string    `json:"user_id"`
	DeviceID       string    `json:"device_id"`
	DeviceName     string    `json:"device_name"`
	AuthToken      string    `json:"auth_token,omitempty"`
	SessionToken   string    `json:"session_token"`
	AllowedTenants []string  `json:"allowed_tenants"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type UserStatus struct {
	TenantID   string    `json:"tenant_id"`
	UserID     string    `json:"user_id"`
	Status     string    `json:"status"`
	StatusNote string    `json:"status_note"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type NoteFile struct {
	FileName    string `json:"file_name"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

type NoteCreateInput struct {
	Title string     `json:"title"`
	Body  string     `json:"body"`
	To    []string   `json:"to"`
	CC    []string   `json:"cc"`
	BCC   []string   `json:"bcc"`
	Files []NoteFile `json:"files"`
}

type NoteRecipient struct {
	UserID string `json:"user_id"`
	Type   string `json:"type"`
}

type Note struct {
	NoteID       string          `json:"note_id"`
	TenantID     string          `json:"tenant_id"`
	SenderUserID string          `json:"sender_user_id"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Recipients   []NoteRecipient `json:"recipients"`
	Files        []NoteFile      `json:"files"`
	CreatedAt    time.Time       `json:"created_at"`
}

type NoteInboxItem struct {
	NoteID        string    `json:"note_id"`
	SenderUserID  string    `json:"sender_user_id"`
	RecipientType string    `json:"recipient_type"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	FileCount     int       `json:"file_count"`
	IsRead        bool      `json:"is_read"`
	CreatedAt     time.Time `json:"created_at"`
}

type ChatNotifyInput struct {
	RoomID       string   `json:"room_id"`
	MessageID    string   `json:"message_id"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	RecipientIDs []string `json:"recipient_ids"`
}
