// Package store provides persistence abstractions and implementations for conversations and messages.
package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// Store defines the persistence interface for conversations and messages.
type Store interface {
	// CreateConversation creates a new conversation record.
	CreateConversation(ctx context.Context) (Conversation, error)
	// InsertMessage persists a new message associated with a conversation.
	InsertMessage(ctx context.Context, conversationID pgtype.UUID, role, content string) (Message, error)
	// ListLastMessages returns up to limit messages for a conversation, ordered chronologically.
	ListLastMessages(ctx context.Context, conversationID pgtype.UUID, limit int32) ([]Message, error)
	// Close releases any resources held by the store.
	Close()
}

// NewStore returns a PostgresStore if databaseURL is non-empty (running embedded migrations),
// or a MemoryStore if databaseURL is empty.
func NewStore(ctx context.Context, databaseURL string) (Store, error) {
	if strings.TrimSpace(databaseURL) != "" {
		return NewPostgresStore(ctx, databaseURL)
	}
	return NewMemoryStore(), nil
}
