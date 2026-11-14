package models

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents an authenticated user of the system.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialize to JSON
	Role         string    `json:"role"`
	FeedToken    *string   `json:"-"` // RSS feed token, never serialize
	CreatedAt    time.Time `json:"created_at"`
}

// UserStore provides data access methods for users.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore creates a new UserStore.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// NormalizeEmail recorta espacios y pasa a minúsculas; así se guardan y se buscan.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// List returns all users ordered by creation date.
func (s *UserStore) List(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, role, created_at FROM users ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("user list: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("user list scan: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetPassword replaces the password hash (and role) of an existing user.
func (s *UserStore) SetPassword(ctx context.Context, email, passwordHash, role string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE users SET password_hash = $2, role = $3 WHERE email = $1
	`, email, passwordHash, role)
	if err != nil {
		return fmt.Errorf("user set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", email)
	}
	return nil
}

// GetByEmail returns a user by their email address.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE lower(email) = lower($1)
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user get by email: %w", err)
	}
	return &u, nil
}

// GetByID returns a user by their UUID.
func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user get by id: %w", err)
	}
	return &u, nil
}

// Create inserts a new user. The ID is generated if not set.
func (s *UserStore) Create(ctx context.Context, user *User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.Role == "" {
		user.Role = "member"
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, user.ID, user.Email, user.PasswordHash, user.Role).Scan(&user.CreatedAt)
	if err != nil {
		return fmt.Errorf("user create: %w", err)
	}
	return nil
}

// GetByFeedToken returns a user by their RSS feed token.
func (s *UserStore) GetByFeedToken(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, feed_token, created_at
		FROM users
		WHERE feed_token = $1
	`, token).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.FeedToken, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("user get by feed token: %w", err)
	}
	return &u, nil
}

// SetFeedToken generates and stores a random feed token for the user.
// If the user already has a token, it returns it without generating a new one.
func (s *UserStore) SetFeedToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Check if user already has a token.
	var existing *string
	err := s.pool.QueryRow(ctx, `SELECT feed_token FROM users WHERE id = $1`, userID).Scan(&existing)
	if err != nil {
		return "", fmt.Errorf("user check feed token: %w", err)
	}
	if existing != nil && *existing != "" {
		return *existing, nil
	}

	// Generate a new 32-char hex token.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("user generate feed token: %w", err)
	}
	token := hex.EncodeToString(b)

	_, err = s.pool.Exec(ctx, `UPDATE users SET feed_token = $2 WHERE id = $1`, userID, token)
	if err != nil {
		return "", fmt.Errorf("user set feed token: %w", err)
	}
	return token, nil
}

// ResetFeedToken generates and stores a new feed token, replacing any existing one.
func (s *UserStore) ResetFeedToken(ctx context.Context, userID uuid.UUID) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("user generate feed token: %w", err)
	}
	token := hex.EncodeToString(b)

	_, err := s.pool.Exec(ctx, `UPDATE users SET feed_token = $2 WHERE id = $1`, userID, token)
	if err != nil {
		return "", fmt.Errorf("user reset feed token: %w", err)
	}
	return token, nil
}
