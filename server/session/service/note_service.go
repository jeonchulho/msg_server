package service

import (
	"context"
	"errors"
	"strings"

	"msg_server/server/session/domain"
)

type noteServiceStore interface {
	CreateSessionNote(ctx context.Context, note domain.Note) (domain.Note, error)
	ListSessionInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error)
	MarkSessionNoteRead(ctx context.Context, tenantID, userID, noteID string) error
}

type NoteService struct {
	store noteServiceStore
	hub   *Hub
}

func NewNoteService(store noteServiceStore, hub *Hub) *NoteService {
	return &NoteService{store: store, hub: hub}
}

func (s *NoteService) SendNote(ctx context.Context, tenantID, senderUserID string, input domain.NoteCreateInput) (domain.Note, error) {
	if strings.TrimSpace(input.Title) == "" {
		return domain.Note{}, errors.New("title is required")
	}
	if len(input.To)+len(input.CC)+len(input.BCC) == 0 {
		return domain.Note{}, errors.New("at least one recipient is required")
	}

	recipients := make([]domain.NoteRecipient, 0, len(input.To)+len(input.CC)+len(input.BCC))
	for _, userID := range dedupeAndTrim(input.To) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "to"})
	}
	for _, userID := range dedupeAndTrim(input.CC) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "cc"})
	}
	for _, userID := range dedupeAndTrim(input.BCC) {
		recipients = append(recipients, domain.NoteRecipient{UserID: userID, Type: "bcc"})
	}

	note, err := s.store.CreateSessionNote(ctx, domain.Note{
		TenantID:     tenantID,
		SenderUserID: senderUserID,
		Title:        strings.TrimSpace(input.Title),
		Body:         input.Body,
		Recipients:   recipients,
		Files:        input.Files,
	})
	if err != nil {
		return domain.Note{}, err
	}

	recipientTypeByUser := map[string]string{}
	for _, recipient := range note.Recipients {
		if _, ok := recipientTypeByUser[recipient.UserID]; !ok {
			recipientTypeByUser[recipient.UserID] = recipient.Type
		}
	}

	userIDs := make([]string, 0, len(recipientTypeByUser))
	for userID := range recipientTypeByUser {
		userIDs = append(userIDs, userID)
	}

	s.hub.NotifyUsers(tenantID, userIDs, func(userID string) any {
		return map[string]any{
			"type":           "note.received",
			"note_id":        note.NoteID,
			"tenant_id":      tenantID,
			"sender_user_id": senderUserID,
			"recipient_type": recipientTypeByUser[userID],
			"title":          note.Title,
			"body":           note.Body,
			"files":          note.Files,
			"created_at":     note.CreatedAt,
		}
	})

	return note, nil
}

func (s *NoteService) ListInbox(ctx context.Context, tenantID, userID string, limit int) ([]domain.NoteInboxItem, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListSessionInbox(ctx, tenantID, userID, limit)
}

func (s *NoteService) MarkNoteRead(ctx context.Context, tenantID, userID, noteID string) error {
	if strings.TrimSpace(noteID) == "" {
		return errors.New("note_id is required")
	}
	return s.store.MarkSessionNoteRead(ctx, tenantID, userID, noteID)
}
