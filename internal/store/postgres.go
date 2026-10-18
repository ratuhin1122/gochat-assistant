package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/ratuhin1122/gochat-assistant/migrations"
)

// PostgresStore implements Store backed by a PostgreSQL database and sqlc generated queries.
type PostgresStore struct {
	pool    *pgxpool.Pool
	queries *Queries
}

// NewPostgresStore connects to PostgreSQL, runs any pending embedded migrations,
// and returns a PostgresStore ready for use.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping postgres: %w", err)
	}

	if err := RunMigrations(pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: run migrations: %w", err)
	}

	return &PostgresStore{
		pool:    pool,
		queries: New(pool),
	}, nil
}

// CreateConversation creates a new conversation with an auto-generated UUID.
func (s *PostgresStore) CreateConversation(ctx context.Context) (Conversation, error) {
	return s.queries.CreateConversation(ctx, pgtype.UUID{})
}

// InsertMessage inserts a new message into the messages table.
func (s *PostgresStore) InsertMessage(ctx context.Context, conversationID pgtype.UUID, role, content string) (Message, error) {
	return s.queries.InsertMessage(ctx, InsertMessageParams{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
	})
}

// ListLastMessages returns up to limit messages for the conversation, ordered chronologically.
func (s *PostgresStore) ListLastMessages(ctx context.Context, conversationID pgtype.UUID, limit int32) ([]Message, error) {
	return s.queries.ListLastMessagesByConversation(ctx, ListLastMessagesByConversationParams{
		ConversationID: conversationID,
		Limit:          limit,
	})
}

// Close closes the underlying pgxpool connection pool.
func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// RunMigrations executes embedded SQL migrations against the database.
func RunMigrations(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	sourceDriver, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("open iofs migration source: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create pgx migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx", dbDriver)
	if err != nil {
		return fmt.Errorf("initialize migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
