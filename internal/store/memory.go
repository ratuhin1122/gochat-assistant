package store

import (
	"context"
	"crypto/rand"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// MemoryStore implements Store in memory for when no DATABASE_URL is configured.
type MemoryStore struct {
	mu            sync.RWMutex
	conversations map[[16]byte]Conversation
	messages      map[[16]byte][]Message
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[[16]byte]Conversation),
		messages:      make(map[[16]byte][]Message),
	}
}

// CreateConversation generates a new Conversation with a UUID.
func (s *MemoryStore) CreateConversation(_ context.Context) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateUUID()
	conv := Conversation{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	s.conversations[id] = conv
	return conv, nil
}

// InsertMessage appends a Message to the conversation's history.
func (s *MemoryStore) InsertMessage(_ context.Context, conversationID pgtype.UUID, role, content string) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgID := generateUUID()
	msg := Message{
		ID:             pgtype.UUID{Bytes: msgID, Valid: true},
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	s.messages[conversationID.Bytes] = append(s.messages[conversationID.Bytes], msg)
	return msg, nil
}

// ListLastMessages returns up to limit messages for the conversation, ordered chronologically.
func (s *MemoryStore) ListLastMessages(_ context.Context, conversationID pgtype.UUID, limit int32) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.messages[conversationID.Bytes]
	if len(msgs) == 0 {
		return []Message{}, nil
	}

	start := 0
	if limit > 0 && len(msgs) > int(limit) {
		start = len(msgs) - int(limit)
	}

	res := make([]Message, len(msgs[start:]))
	copy(res, msgs[start:])
	return res, nil
}

// Close is a no-op for MemoryStore.
func (s *MemoryStore) Close() {}

func generateUUID() [16]byte {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4 RFC 4122
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return b
}
