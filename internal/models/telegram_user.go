package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TelegramUser maps a Telegram account to a Folio user.
type TelegramUser struct {
	TelegramID  int64     `json:"telegram_id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// TelegramUserStore provides data access methods for telegram user mappings.
type TelegramUserStore struct {
	pool *pgxpool.Pool
}

// NewTelegramUserStore creates a new TelegramUserStore.
func NewTelegramUserStore(pool *pgxpool.Pool) *TelegramUserStore {
	return &TelegramUserStore{pool: pool}
}

// GetByTelegramID returns the telegram user mapping for the given Telegram ID.
func (s *TelegramUserStore) GetByTelegramID(ctx context.Context, telegramID int64) (*TelegramUser, error) {
	var tu TelegramUser
	err := s.pool.QueryRow(ctx, `
		SELECT telegram_id, user_id, username, display_name, created_at
		FROM telegram_users
		WHERE telegram_id = $1
	`, telegramID).Scan(&tu.TelegramID, &tu.UserID, &tu.Username, &tu.DisplayName, &tu.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("telegram user get by telegram_id: %w", err)
	}
	return &tu, nil
}

// GetByUserID returns the telegram user mapping for the given Folio user ID.
func (s *TelegramUserStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*TelegramUser, error) {
	var tu TelegramUser
	err := s.pool.QueryRow(ctx, `
		SELECT telegram_id, user_id, username, display_name, created_at
		FROM telegram_users
		WHERE user_id = $1
	`, userID).Scan(&tu.TelegramID, &tu.UserID, &tu.Username, &tu.DisplayName, &tu.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("telegram user get by user_id: %w", err)
	}
	return &tu, nil
}

// Upsert inserts or updates a telegram user mapping. On conflict by telegram_id,
// the username and display_name are updated.
func (s *TelegramUserStore) Upsert(ctx context.Context, tu *TelegramUser) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO telegram_users (telegram_id, user_id, username, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name
	`, tu.TelegramID, tu.UserID, tu.Username, tu.DisplayName)
	if err != nil {
		return fmt.Errorf("telegram user upsert: %w", err)
	}
	return nil
}

// ListAll returns all telegram user mappings.
func (s *TelegramUserStore) ListAll(ctx context.Context) ([]TelegramUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT telegram_id, user_id, username, display_name, created_at
		FROM telegram_users
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("telegram user list all: %w", err)
	}
	defer rows.Close()

	var users []TelegramUser
	for rows.Next() {
		var tu TelegramUser
		if err := rows.Scan(&tu.TelegramID, &tu.UserID, &tu.Username, &tu.DisplayName, &tu.CreatedAt); err != nil {
			return nil, fmt.Errorf("telegram user scan: %w", err)
		}
		users = append(users, tu)
	}
	return users, rows.Err()
}
