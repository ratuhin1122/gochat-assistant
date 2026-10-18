package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ratuhin1122/gochat-assistant/internal/store"
)

func TestPostgresStore_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := store.NewPostgresStore(ctx, "postgres://invalid:invalid@127.0.0.1:54321/nonexistent?sslmode=disable")
	if err == nil {
		t.Fatal("expected error connecting to non-existent database, got nil")
	}
}

func TestPostgresStore_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping postgres integration test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := store.NewPostgresStore(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer s.Close()

	conv, err := s.CreateConversation(ctx)
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}
	if !conv.ID.Valid {
		t.Fatal("expected valid conversation UUID")
	}

	msg, err := s.InsertMessage(ctx, conv.ID, "user", "Integration test message")
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}
	if msg.Content != "Integration test message" {
		t.Errorf("unexpected content: %q", msg.Content)
	}

	msgs, err := s.ListLastMessages(ctx, conv.ID, 5)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least 1 message")
	}
}
