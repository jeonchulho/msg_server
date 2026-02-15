package domain

import "time"

type UserStatus string
type UserRole string

type Tenant struct {
	TenantID                string    `json:"tenant_id"`
	Name                    string    `json:"name"`
	DeploymentMode          string    `json:"deployment_mode"`
	DedicatedDSN            string    `json:"dedicated_dsn"`
	DedicatedRedisAddr      string    `json:"dedicated_redis_addr"`
	DedicatedLavinMQURL     string    `json:"dedicated_lavinmq_url"`
	DedicatedMinIOEndpoint  string    `json:"dedicated_minio_endpoint"`
	DedicatedMinIOAccessKey string    `json:"dedicated_minio_access_key"`
	DedicatedMinIOSecretKey string    `json:"dedicated_minio_secret_key"`
	DedicatedMinIOBucket    string    `json:"dedicated_minio_bucket"`
	DedicatedMinIOUseSSL    bool      `json:"dedicated_minio_use_ssl"`
	UserCountThreshold      int       `json:"user_count_threshold"`
	IsActive                bool      `json:"is_active"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

const (
	UserStatusOffline UserStatus = "offline"
	UserStatusOnline  UserStatus = "online"
	UserStatusBusy    UserStatus = "busy"
	UserStatusMeeting UserStatus = "meeting"
	UserStatusAway    UserStatus = "away"
)

const (
	UserRoleAdmin   UserRole = "admin"
	UserRoleManager UserRole = "manager"
	UserRoleUser    UserRole = "user"
)

type OrgUnit struct {
	TenantID  string    `json:"tenant_id"`
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	TenantID     string     `json:"tenant_id"`
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Title        string     `json:"title"`
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
	StatusNote   string     `json:"status_note"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ChatRoom struct {
	TenantID  string    `json:"tenant_id"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	RoomType  string    `json:"room_type"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	TenantID  string    `json:"tenant_id"`
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	SenderID  string    `json:"sender_id"`
	Body      string    `json:"body"`
	MetaJSON  string    `json:"meta_json"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageRead struct {
	TenantID  string    `json:"tenant_id"`
	RoomID    string    `json:"room_id"`
	MessageID string    `json:"message_id"`
	UserID    string    `json:"user_id"`
	ReadAt    time.Time `json:"read_at"`
}

type RoomUnread struct {
	RoomID      string `json:"room_id"`
	UnreadCount int64  `json:"unread_count"`
}

type AliasAudit struct {
	TenantID  string    `json:"tenant_id"`
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Alias     string    `json:"alias"`
	Action    string    `json:"action"`
	ActedBy   string    `json:"acted_by"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatRoomSummary struct {
	RoomID                     string     `json:"room_id"`
	Name                       string     `json:"name"`
	RoomType                   string     `json:"room_type"`
	CreatedBy                  string     `json:"created_by"`
	CreatedAt                  time.Time  `json:"created_at"`
	PeerUserID                 *string    `json:"peer_user_id,omitempty"`
	PeerName                   *string    `json:"peer_name,omitempty"`
	PeerStatus                 *string    `json:"peer_status,omitempty"`
	PeerStatusNote             *string    `json:"peer_status_note,omitempty"`
	LatestMessageID            *string    `json:"latest_message_id,omitempty"`
	LatestMessageBody          *string    `json:"latest_message_body,omitempty"`
	LatestMessageKind          *string    `json:"latest_message_kind,omitempty"`
	LatestMessageSummary       *string    `json:"latest_message_summary,omitempty"`
	LatestMessageMentionTokens []string   `json:"latest_message_mention_tokens,omitempty"`
	LatestMessageIsMentioned   bool       `json:"latest_message_is_mentioned"`
	LatestMessageAt            *time.Time `json:"latest_message_at,omitempty"`
	LatestMessageSender        *string    `json:"latest_message_sender,omitempty"`
	UnreadCount                int64      `json:"unread_count"`
}
