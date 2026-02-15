package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"msg_server/server/chat/domain"
	"msg_server/server/dbman/repository"
)

type UserService struct {
	repo *repository.UserRepository
}

var aliasPattern = regexp.MustCompile(`^[A-Za-z0-9_가-힣]{1,40}$`)

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) CreateOrgUnit(ctx context.Context, tenantID string, parentID *string, name string) (string, error) {
	return s.repo.CreateOrgUnit(ctx, tenantID, parentID, name)
}

func (s *UserService) ListOrgUnits(ctx context.Context, tenantID string) ([]domain.OrgUnit, error) {
	return s.repo.ListOrgUnits(ctx, tenantID)
}

func (s *UserService) CreateUser(ctx context.Context, tenantID string, user domain.User) (string, error) {
	if user.Status == "" {
		user.Status = domain.UserStatusOffline
	}
	if user.Role == "" {
		user.Role = domain.UserRoleUser
	}
	if user.PasswordHash == "" {
		return "", errors.New("password is required")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(user.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	user.PasswordHash = string(hashed)
	return s.repo.CreateUser(ctx, tenantID, user)
}

func (s *UserService) UpdateStatus(ctx context.Context, tenantID, userID string, status domain.UserStatus, note string) error {
	return s.repo.UpdateStatus(ctx, tenantID, userID, status, note)
}

func (s *UserService) SearchUsers(ctx context.Context, tenantID, q string, limit int) ([]domain.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.SearchUsers(ctx, tenantID, q, limit)
}

func (s *UserService) Authenticate(ctx context.Context, tenantID, email, password string) (domain.User, error) {
	user, err := s.repo.GetByEmail(ctx, tenantID, email)
	if err != nil {
		return domain.User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, errors.New("invalid credentials")
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *UserService) ListAliases(ctx context.Context, tenantID, userID string) ([]string, error) {
	return s.repo.ListAliases(ctx, tenantID, userID)
}

func (s *UserService) AddAlias(ctx context.Context, tenantID, userID, alias, ip, userAgent string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return errors.New("alias is required")
	}
	if len(alias) > 40 {
		return errors.New("alias is too long")
	}
	if !aliasPattern.MatchString(alias) {
		return errors.New("alias may only contain letters, digits, underscore, and Korean characters")
	}
	alias = strings.ToLower(alias)
	return s.repo.AddAlias(ctx, tenantID, userID, alias, ip, userAgent)
}

func (s *UserService) DeleteAlias(ctx context.Context, tenantID, userID, alias, ip, userAgent string) error {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return errors.New("alias is required")
	}
	if !aliasPattern.MatchString(alias) {
		return errors.New("alias format is invalid")
	}
	alias = strings.ToLower(alias)
	return s.repo.DeleteAlias(ctx, tenantID, userID, alias, ip, userAgent)
}

func (s *UserService) ListAliasAudit(ctx context.Context, tenantID, userID string, limit int, from, to *time.Time, action string, cursorCreatedAt *time.Time, cursorID *string) ([]domain.AliasAudit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "" && action != "add" && action != "delete" {
		return nil, errors.New("action must be add or delete")
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, errors.New("from must be before to")
	}
	return s.repo.ListAliasAudit(ctx, tenantID, userID, limit, from, to, action, cursorCreatedAt, cursorID)
}
