package api

import (
	"msg_server/server/common/transport/httpresp"
)

const (
	ErrUnauthorized               = httpresp.ErrUnauthorized
	ErrInvalidCredentials         = httpresp.ErrInvalidCredentials
	ErrCannotUpdateOtherUserState = httpresp.ErrCannotUpdateOtherUserState
	ErrFromMustBeRFC3339          = httpresp.ErrFromMustBeRFC3339
	ErrToMustBeRFC3339            = httpresp.ErrToMustBeRFC3339
)

type PaginatedResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ErrorResponse = httpresp.ErrorResponse
type OKResponse = httpresp.OKResponse
type IDResponse = httpresp.IDResponse
type URLResponse = httpresp.URLResponse
type TokenResponse = httpresp.TokenResponse

type HealthResponse struct {
	Status string `json:"status"`
}

type AliasesResponse struct {
	Aliases []string `json:"aliases"`
}

type RoomUnreadCountResponse struct {
	RoomID      string `json:"room_id"`
	UserID      string `json:"user_id"`
	UnreadCount int64  `json:"unread_count"`
}

type ReadStateResponse struct {
	RoomID            string `json:"room_id"`
	UserID            string `json:"user_id"`
	LastReadMessageID string `json:"last_read_message_id"`
}

func NewPaginatedResponse[T any](items []T, nextCursor string) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Items:      items,
		NextCursor: nextCursor,
	}
}

func NewErrorResponse(message string) ErrorResponse {
	return httpresp.NewErrorResponse(message)
}

func NewOKResponse() OKResponse {
	return httpresp.NewOKResponse()
}

func NewIDResponse(id string) IDResponse {
	return httpresp.NewIDResponse(id)
}

func NewURLResponse(url string) URLResponse {
	return httpresp.NewURLResponse(url)
}

func NewTokenResponse(accessToken string, userID string, tenantID string, role string) TokenResponse {
	return httpresp.NewTokenResponse(accessToken, userID, tenantID, role)
}

func NewHealthResponse(status string) HealthResponse {
	return HealthResponse{Status: status}
}

func NewAliasesResponse(aliases []string) AliasesResponse {
	return AliasesResponse{Aliases: aliases}
}

func NewRoomUnreadCountResponse(roomID, userID string, unreadCount int64) RoomUnreadCountResponse {
	return RoomUnreadCountResponse{RoomID: roomID, UserID: userID, UnreadCount: unreadCount}
}

func NewReadStateResponse(roomID, userID, lastReadMessageID string) ReadStateResponse {
	return ReadStateResponse{RoomID: roomID, UserID: userID, LastReadMessageID: lastReadMessageID}
}
