package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ratuhin1122/gochat-assistant/internal/store"
)

func TestMemoryStore_CreateConversation(t *testing.T) {
	s := store.NewMemoryStore()
	defer s.Close()

	ctx := context.Background()
	conv, err := s.CreateConversation(ctx)
	if err != nil {
		t.Fatalf("unexpected error creating conversation: %v", err)
	}

	if !conv.ID.Valid {
		t.Fatal("expected valid UUID for conversation ID")
	}
	if !conv.CreatedAt.Valid {
		t.Fatal("expected valid CreatedAt for conversation")
	}

	conv2, err := s.CreateConversation(ctx)
	if err != nil {
		t.Fatalf("unexpected error creating second conversation: %v", err)
	}
	if conv.ID.Bytes == conv2.ID.Bytes {
		t.Errorf("expected unique conversation IDs, got identical: %v", conv.ID.Bytes)
	}
}

func TestMemoryStore_InsertAndListMessages(t *testing.T) {
	s := store.NewMemoryStore()
	defer s.Close()

	ctx := context.Background()
	conv, err := s.CreateConversation(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Insert 3 messages
	m1, err := s.InsertMessage(ctx, conv.ID, "user", "Hello")
	if err != nil {
		t.Fatalf("failed to insert m1: %v", err)
	}
	if !m1.ID.Valid || m1.Role != "user" || m1.Content != "Hello" {
		t.Errorf("unexpected m1: %+v", m1)
	}

	m2, err := s.InsertMessage(ctx, conv.ID, "assistant", "Hi there!")
	if err != nil {
		t.Fatalf("failed to insert m2: %v", err)
	}
	if m2.Role != "assistant" || m2.Content != "Hi there!" {
		t.Errorf("unexpected m2: %+v", m2)
	}

	m3, err := s.InsertMessage(ctx, conv.ID, "user", "How are you?")
	if err != nil {
		t.Fatalf("failed to insert m3: %v", err)
	}
	if m3.Role != "user" || m3.Content != "How are you?" {
		t.Errorf("unexpected m3: %+v", m3)
	}

	t.Run("list all messages when limit exceeds count", func(t *testing.T) {
		msgs, err := s.ListLastMessages(ctx, conv.ID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(msgs))
		}
		if msgs[0].Content != "Hello" || msgs[1].Content != "Hi there!" || msgs[2].Content != "How are you?" {
			t.Errorf("unexpected order or content: %+v", msgs)
		}
	})

	t.Run("list last 2 messages", func(t *testing.T) {
		msgs, err := s.ListLastMessages(ctx, conv.ID, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
		if msgs[0].Content != "Hi there!" || msgs[1].Content != "How are you?" {
			t.Errorf("expected last 2 messages, got: %+v", msgs)
		}
	})

	t.Run("list from non-existent conversation", func(t *testing.T) {
		var fakeID [16]byte
		fakeID[0] = 0xff
		msgs, err := s.ListLastMessages(ctx, pgtype.UUID{Bytes: fakeID, Valid: true}, 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 0 {
			t.Errorf("expected 0 messages for unknown conversation, got %d", len(msgs))
		}
	})
}

func TestNewStore_Factory(t *testing.T) {
	ctx := context.Background()

	t.Run("empty database URL returns MemoryStore", func(t *testing.T) {
		st, err := store.NewStore(ctx, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := st.(*store.MemoryStore); !ok {
			t.Errorf("expected *store.MemoryStore, got %T", st)
		}
	})

	t.Run("whitespace database URL returns MemoryStore", func(t *testing.T) {
		st, err := store.NewStore(ctx, "   ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := st.(*store.MemoryStore); !ok {
			t.Errorf("expected *store.MemoryStore, got %T", st)
		}
	})
}
