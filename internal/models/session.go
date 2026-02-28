package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Session represents an active user session (cookie-based auth).
type Session struct {
	ID        string    `json:"id"` // opaque token
	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionStore provides data access methods for sessions.
type SessionStore struct {
	pool *pgxpool.Pool
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(pool *pgxpool.Pool) *SessionStore {
	return &SessionStore{pool: pool}
}

// Create inserts a new session.
func (s *SessionStore) Create(ctx context.Context, session *Session) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO sessions (id, user_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`, session.ID, session.UserID, session.ExpiresAt).Scan(&session.CreatedAt)
	if err != nil {
		return fmt.Errorf("session create: %w", err)
	}
	return nil
}

// GetByToken returns a session by its token string.
func (s *SessionStore) GetByToken(ctx context.Context, token string) (*Session, error) {
	var sess Session
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, created_at
		FROM sessions
		WHERE id = $1
	`, token).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("session get: %w", err)
	}
	return &sess, nil
}

// Delete removes a session by its token.
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, token)
	if err != nil {
		return fmt.Errorf("session delete: %w", err)
	}
	return nil
}

// DeleteExpired removes all sessions that have passed their expiry time.
func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	if err != nil {
		return fmt.Errorf("session delete expired: %w", err)
	}
	return nil
}
