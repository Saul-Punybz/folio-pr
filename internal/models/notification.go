package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BotNotification represents a notification to be delivered via the Telegram bot.
type BotNotification struct {
	ID          uuid.UUID       `json:"id"`
	UserID      uuid.UUID       `json:"user_id"`
	Type        string          `json:"type"` // "digest", "watchlist_hit", "system"
	Payload     json.RawMessage `json:"payload"`
	Delivered   bool            `json:"delivered"`
	CreatedAt   time.Time       `json:"created_at"`
	DeliveredAt *time.Time      `json:"delivered_at,omitempty"`
}

// NotificationStore provides data access methods for bot notifications.
type NotificationStore struct {
	pool *pgxpool.Pool
}

// NewNotificationStore creates a new NotificationStore.
func NewNotificationStore(pool *pgxpool.Pool) *NotificationStore {
	return &NotificationStore{pool: pool}
}

// ListPending returns undelivered notifications ordered by creation time, up to limit.
func (s *NotificationStore) ListPending(ctx context.Context, limit int) ([]BotNotification, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, type, payload, delivered, created_at, delivered_at
		FROM bot_notifications
		WHERE delivered = false
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("notification list pending: %w", err)
	}
	defer rows.Close()

	var notifications []BotNotification
	for rows.Next() {
		var n BotNotification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.Delivered, &n.CreatedAt, &n.DeliveredAt); err != nil {
			return nil, fmt.Errorf("notification scan: %w", err)
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

// MarkDelivered marks a notification as delivered with the current timestamp.
func (s *NotificationStore) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE bot_notifications
		SET delivered = true, delivered_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("notification mark delivered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}
	return nil
}

// CreateDigest creates a digest notification for the given user with a brief summary.
func (s *NotificationStore) CreateDigest(ctx context.Context, userID uuid.UUID, briefSummary string) error {
	id := uuid.New()
	payload, err := json.Marshal(map[string]string{
		"summary": briefSummary,
	})
	if err != nil {
		return fmt.Errorf("notification create digest: marshal payload: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO bot_notifications (id, user_id, type, payload)
		VALUES ($1, $2, 'digest', $3)
	`, id, userID, payload)
	if err != nil {
		return fmt.Errorf("notification create digest: %w", err)
	}
	return nil
}

// CreateWatchlistHit creates a watchlist hit notification for the given user.
func (s *NotificationStore) CreateWatchlistHit(ctx context.Context, userID uuid.UUID, orgName, title, url string) error {
	id := uuid.New()
	payload, err := json.Marshal(map[string]string{
		"org_name": orgName,
		"title":    title,
		"url":      url,
	})
	if err != nil {
		return fmt.Errorf("notification create watchlist hit: marshal payload: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO bot_notifications (id, user_id, type, payload)
		VALUES ($1, $2, 'watchlist_hit', $3)
	`, id, userID, payload)
	if err != nil {
		return fmt.Errorf("notification create watchlist hit: %w", err)
	}
	return nil
}

// Cleanup deletes delivered notifications older than the specified number of days.
// Returns the number of rows deleted.
func (s *NotificationStore) Cleanup(ctx context.Context, olderThanDays int) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM bot_notifications
		WHERE delivered = true
		  AND created_at < NOW() - make_interval(days => $1)
	`, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("notification cleanup: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
