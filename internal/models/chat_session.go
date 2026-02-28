package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatSession struct {
	ID        uuid.UUID       `json:"id"`
	UserID    uuid.UUID       `json:"user_id"`
	Title     string          `json:"title"`
	Messages  json.RawMessage `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type ChatSessionStore struct {
	pool *pgxpool.Pool
}

func NewChatSessionStore(pool *pgxpool.Pool) *ChatSessionStore {
	return &ChatSessionStore{pool: pool}
}

func (s *ChatSessionStore) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]ChatSession, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, title, messages, created_at, updated_at
		FROM chat_sessions
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("chat sessions list: %w", err)
	}
	defer rows.Close()

	var sessions []ChatSession
	for rows.Next() {
		var cs ChatSession
		if err := rows.Scan(&cs.ID, &cs.UserID, &cs.Title, &cs.Messages, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, fmt.Errorf("chat session scan: %w", err)
		}
		sessions = append(sessions, cs)
	}
	return sessions, rows.Err()
}

func (s *ChatSessionStore) GetByID(ctx context.Context, id uuid.UUID) (*ChatSession, error) {
	var cs ChatSession
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, title, messages, created_at, updated_at
		FROM chat_sessions
		WHERE id = $1
	`, id).Scan(&cs.ID, &cs.UserID, &cs.Title, &cs.Messages, &cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("chat session get: %w", err)
	}
	return &cs, nil
}

func (s *ChatSessionStore) Create(ctx context.Context, cs *ChatSession) error {
	if cs.ID == uuid.Nil {
		cs.ID = uuid.New()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO chat_sessions (id, user_id, title, messages)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at
	`, cs.ID, cs.UserID, cs.Title, cs.Messages).Scan(&cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return fmt.Errorf("chat session create: %w", err)
	}
	return nil
}

func (s *ChatSessionStore) Update(ctx context.Context, id uuid.UUID, title string, messages json.RawMessage) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE chat_sessions
		SET title = $2, messages = $3, updated_at = NOW()
		WHERE id = $1
	`, id, title, messages)
	if err != nil {
		return fmt.Errorf("chat session update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chat session not found: %s", id)
	}
	return nil
}

func (s *ChatSessionStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM chat_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("chat session delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("chat session not found: %s", id)
	}
	return nil
}
