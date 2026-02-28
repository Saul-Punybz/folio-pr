package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WatchlistOrg represents an NGO or organization being monitored.
type WatchlistOrg struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Name            string    `json:"name"`
	Website         string    `json:"website"`
	Keywords        []string  `json:"keywords"`
	YouTubeChannels []string  `json:"youtube_channels"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// WatchlistHit represents a single mention found by a scanning agent.
type WatchlistHit struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	OrgName    string    `json:"org_name,omitempty"`
	SourceType string    `json:"source_type"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	URLHash    string    `json:"url_hash"`
	Snippet    string    `json:"snippet"`
	Sentiment  string    `json:"sentiment"`
	AIDraft    *string   `json:"ai_draft"`
	Seen       bool      `json:"seen"`
	CreatedAt  time.Time `json:"created_at"`
}

// ── WatchlistOrgStore ────────────────────────────────────────────

type WatchlistOrgStore struct {
	pool *pgxpool.Pool
}

func NewWatchlistOrgStore(pool *pgxpool.Pool) *WatchlistOrgStore {
	return &WatchlistOrgStore{pool: pool}
}

func (s *WatchlistOrgStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]WatchlistOrg, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, website, keywords, youtube_channels, active, created_at, updated_at
		FROM watchlist_orgs
		WHERE user_id = $1
		ORDER BY name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist orgs list: %w", err)
	}
	defer rows.Close()

	var orgs []WatchlistOrg
	for rows.Next() {
		var o WatchlistOrg
		var kwRaw, ytRaw []byte
		if err := rows.Scan(&o.ID, &o.UserID, &o.Name, &o.Website, &kwRaw, &ytRaw, &o.Active, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("watchlist orgs scan: %w", err)
		}
		o.Keywords = scanJSONStringSlice(kwRaw)
		o.YouTubeChannels = scanJSONStringSlice(ytRaw)
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

func (s *WatchlistOrgStore) ListActive(ctx context.Context) ([]WatchlistOrg, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, website, keywords, youtube_channels, active, created_at, updated_at
		FROM watchlist_orgs
		WHERE active = true
	`)
	if err != nil {
		return nil, fmt.Errorf("watchlist orgs list active: %w", err)
	}
	defer rows.Close()

	var orgs []WatchlistOrg
	for rows.Next() {
		var o WatchlistOrg
		var kwRaw, ytRaw []byte
		if err := rows.Scan(&o.ID, &o.UserID, &o.Name, &o.Website, &kwRaw, &ytRaw, &o.Active, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("watchlist orgs scan: %w", err)
		}
		o.Keywords = scanJSONStringSlice(kwRaw)
		o.YouTubeChannels = scanJSONStringSlice(ytRaw)
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

func (s *WatchlistOrgStore) Create(ctx context.Context, org *WatchlistOrg) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	kwJSON, err := json.Marshal(org.Keywords)
	if err != nil {
		return fmt.Errorf("watchlist org create: marshal keywords: %w", err)
	}
	ytJSON, err := json.Marshal(org.YouTubeChannels)
	if err != nil {
		return fmt.Errorf("watchlist org create: marshal youtube: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO watchlist_orgs (id, user_id, name, website, keywords, youtube_channels, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`, org.ID, org.UserID, org.Name, org.Website, kwJSON, ytJSON, org.Active).Scan(&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return fmt.Errorf("watchlist org create: %w", err)
	}
	return nil
}

func (s *WatchlistOrgStore) Update(ctx context.Context, org *WatchlistOrg) error {
	kwJSON, err := json.Marshal(org.Keywords)
	if err != nil {
		return fmt.Errorf("watchlist org update: marshal keywords: %w", err)
	}
	ytJSON, err := json.Marshal(org.YouTubeChannels)
	if err != nil {
		return fmt.Errorf("watchlist org update: marshal youtube: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_orgs
		SET name = $2, website = $3, keywords = $4, youtube_channels = $5, active = $6, updated_at = NOW()
		WHERE id = $1
	`, org.ID, org.Name, org.Website, kwJSON, ytJSON, org.Active)
	if err != nil {
		return fmt.Errorf("watchlist org update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist org not found: %s", org.ID)
	}
	return nil
}

func (s *WatchlistOrgStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM watchlist_orgs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("watchlist org delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist org not found: %s", id)
	}
	return nil
}

func (s *WatchlistOrgStore) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_orgs SET active = $2, updated_at = NOW() WHERE id = $1
	`, id, active)
	if err != nil {
		return fmt.Errorf("watchlist org toggle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist org not found: %s", id)
	}
	return nil
}

// ── WatchlistHitStore ────────────────────────────────────────────

type WatchlistHitStore struct {
	pool *pgxpool.Pool
}

func NewWatchlistHitStore(pool *pgxpool.Pool) *WatchlistHitStore {
	return &WatchlistHitStore{pool: pool}
}

func (s *WatchlistHitStore) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT wh.id, wh.org_id, wo.name, wh.source_type, wh.title, wh.url, wh.url_hash,
		       wh.snippet, wh.sentiment, wh.ai_draft, wh.seen, wh.created_at
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1
		ORDER BY wh.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

func (s *WatchlistHitStore) ListByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT wh.id, wh.org_id, wo.name, wh.source_type, wh.title, wh.url, wh.url_hash,
		       wh.snippet, wh.sentiment, wh.ai_draft, wh.seen, wh.created_at
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.org_id = $1
		ORDER BY wh.created_at DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list by org: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

func (s *WatchlistHitStore) CountUnseenByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1 AND wh.seen = false
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("watchlist hits count unseen: %w", err)
	}
	return count, nil
}

func (s *WatchlistHitStore) Create(ctx context.Context, hit *WatchlistHit) error {
	if hit.ID == uuid.Nil {
		hit.ID = uuid.New()
	}
	// ON CONFLICT DO NOTHING — deduplication via url_hash.
	err := s.pool.QueryRow(ctx, `
		INSERT INTO watchlist_hits (id, org_id, source_type, title, url, url_hash, snippet, sentiment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (url_hash) DO NOTHING
		RETURNING created_at
	`, hit.ID, hit.OrgID, hit.SourceType, hit.Title, hit.URL, hit.URLHash, hit.Snippet, hit.Sentiment).Scan(&hit.CreatedAt)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows — not an error, just a duplicate.
		hit.ID = uuid.Nil // Signal that it was a duplicate.
		return nil
	}
	return nil
}

func (s *WatchlistHitStore) MarkSeen(ctx context.Context, hitID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE watchlist_hits SET seen = true WHERE id = $1`, hitID)
	if err != nil {
		return fmt.Errorf("watchlist hit mark seen: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist hit not found: %s", hitID)
	}
	return nil
}

func (s *WatchlistHitStore) MarkAllSeenByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET seen = true
		FROM watchlist_orgs wo
		WHERE wh.org_id = wo.id AND wo.user_id = $1 AND wh.seen = false
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("watchlist hits mark all seen: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (s *WatchlistHitStore) UpdateSentiment(ctx context.Context, hitID uuid.UUID, sentiment string) error {
	_, err := s.pool.Exec(ctx, `UPDATE watchlist_hits SET sentiment = $2 WHERE id = $1`, hitID, sentiment)
	if err != nil {
		return fmt.Errorf("watchlist hit update sentiment: %w", err)
	}
	return nil
}

func (s *WatchlistHitStore) UpdateAIDraft(ctx context.Context, hitID uuid.UUID, draft string) error {
	_, err := s.pool.Exec(ctx, `UPDATE watchlist_hits SET ai_draft = $2 WHERE id = $1`, hitID, draft)
	if err != nil {
		return fmt.Errorf("watchlist hit update draft: %w", err)
	}
	return nil
}

func (s *WatchlistHitStore) Delete(ctx context.Context, hitID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM watchlist_hits WHERE id = $1`, hitID)
	if err != nil {
		return fmt.Errorf("watchlist hit delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist hit not found: %s", hitID)
	}
	return nil
}

func (s *WatchlistHitStore) ListRecentByUser(ctx context.Context, userID uuid.UUID, limit int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT wh.id, wh.org_id, wo.name, wh.source_type, wh.title, wh.url, wh.url_hash,
		       wh.snippet, wh.sentiment, wh.ai_draft, wh.seen, wh.created_at
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1
		ORDER BY wh.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list recent: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

func (s *WatchlistHitStore) ListBySentiment(ctx context.Context, sentiment string, limit int) ([]WatchlistHit, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT wh.id, wh.org_id, wo.name, wh.source_type, wh.title, wh.url, wh.url_hash,
		       wh.snippet, wh.sentiment, wh.ai_draft, wh.seen, wh.created_at
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.sentiment = $1
		ORDER BY wh.created_at DESC
		LIMIT $2
	`, sentiment, limit)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits by sentiment: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

// ── Helpers ──────────────────────────────────────────────────────

func scanHitRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]WatchlistHit, error) {
	var hits []WatchlistHit
	for rows.Next() {
		var h WatchlistHit
		if err := rows.Scan(
			&h.ID, &h.OrgID, &h.OrgName, &h.SourceType, &h.Title, &h.URL, &h.URLHash,
			&h.Snippet, &h.Sentiment, &h.AIDraft, &h.Seen, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("watchlist hit scan: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// scanJSONStringSlice unmarshals a JSONB column into a []string.
func scanJSONStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return s
}
