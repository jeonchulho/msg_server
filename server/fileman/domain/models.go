package domain

import "time"

type FileObject struct {
	TenantID     string    `json:"tenant_id"`
	ID           string    `json:"id"`
	RoomID       string    `json:"room_id"`
	UploaderID   string    `json:"uploader_id"`
	ObjectKey    string    `json:"object_key"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	ThumbnailKey string    `json:"thumbnail_key"`
	OriginalName string    `json:"original_name"`
	CreatedAt    time.Time `json:"created_at"`
}
