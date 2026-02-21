package orghub

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"msg_server/server/chat/domain"
)

type DBManClient interface {
	CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error)
	ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error)
	CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error)
	UpdateUserStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error
	SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error)
	AuthenticateUser(ctx context.Context, tenantID, email, password string) (domain.User, error)
	ListAliases(ctx context.Context, tenantID, userID string) ([]string, error)
	AddAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error
	DeleteAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error
	ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action string, cursorCreatedAt *time.Time, cursorID *string) ([]domain.AliasAudit, error)
}

type Service struct {
	dbman DBManClient
}

func NewService(dbman DBManClient) *Service {
	return &Service{dbman: dbman}
}

func (s *Service) CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error) {
	return s.dbman.CreateOrgUnit(ctx, tenantID, parentID, name)
}

func (s *Service) ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error) {
	return s.dbman.ListOrgUnits(ctx, tenantID)
}

func (s *Service) CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error) {
	return s.dbman.CreateUser(ctx, tenantID, user)
}

func (s *Service) UpdateStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error {
	return s.dbman.UpdateUserStatus(ctx, tenantID, userID, status, note)
}

func (s *Service) SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.dbman.SearchUsers(ctx, tenantID, q, limit)
}

func (s *Service) Authenticate(ctx context.Context, tenantID, email, password string) (domain.User, error) {
	return s.dbman.AuthenticateUser(ctx, tenantID, email, password)
}

func (s *Service) ListAliases(ctx context.Context, tenantID, userID string) ([]string, error) {
	return s.dbman.ListAliases(ctx, tenantID, userID)
}

func (s *Service) AddAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	return s.dbman.AddAlias(ctx, tenantID, userID, alias, ip, userAgent)
}

func (s *Service) DeleteAlias(ctx context.Context, tenantID, userID string, alias, ip, userAgent string) error {
	return s.dbman.DeleteAlias(ctx, tenantID, userID, alias, ip, userAgent)
}

func (s *Service) ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action, cursor string) ([]domain.AliasAudit, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "" && action != "add" && action != "delete" {
		return nil, "", errors.New("action must be add or delete")
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, "", errors.New("from must be before to")
	}

	var cursorCreatedAt *time.Time
	var cursorID *string
	if strings.TrimSpace(cursor) != "" {
		parsedTime, parsedID, err := decodeAliasAuditCursor(cursor)
		if err != nil {
			return nil, "", errors.New("cursor is invalid")
		}
		cursorCreatedAt = &parsedTime
		cursorID = &parsedID
	}

	items, err := s.dbman.ListAliasAudit(ctx, tenantID, userID, limit+1, from, to, action, cursorCreatedAt, cursorID)
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeAliasAuditCursor(last.CreatedAt, last.ID)
	}
	return items, nextCursor, nil
}

func encodeAliasAuditCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%d:%s", createdAt.UnixNano(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeAliasAuditCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", errors.New("invalid cursor format")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}
	id := strings.TrimSpace(parts[1])
	if id == "" {
		return time.Time{}, "", errors.New("invalid cursor id")
	}
	return time.Unix(0, nanos).UTC(), id, nil
}
