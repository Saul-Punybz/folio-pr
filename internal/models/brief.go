package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Brief represents a daily intelligence summary.
type Brief struct {
	ID           uuid.UUID `json:"id"`
	Date         time.Time `json:"date"`
	Summary      string    `json:"summary"`
	TopTags      []string  `json:"top_tags"`
	ArticleCount int       `json:"article_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// BriefStore provides data access methods for daily briefs.
type BriefStore struct {
	pool *pgxpool.Pool
}

// NewBriefStore creates a new BriefStore.
func NewBriefStore(pool *pgxpool.Pool) *BriefStore {
	return &BriefStore{pool: pool}
}

// GetLatest returns the most recent daily brief.
func (s *BriefStore) GetLatest(ctx context.Context) (*Brief, error) {
	var b Brief
	var tagsRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, date, summary, top_tags, article_count, created_at
		FROM briefs
		ORDER BY date DESC
		LIMIT 1
	`).Scan(&b.ID, &b.Date, &b.Summary, &tagsRaw, &b.ArticleCount, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("brief get latest: %w", err)
	}
	b.TopTags = scanBriefTags(tagsRaw)
	return &b, nil
}

// GetByDate returns the brief for a specific date.
func (s *BriefStore) GetByDate(ctx context.Context, date time.Time) (*Brief, error) {
	var b Brief
	var tagsRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, date, summary, top_tags, article_count, created_at
		FROM briefs
		WHERE date = $1
	`, date).Scan(&b.ID, &b.Date, &b.Summary, &tagsRaw, &b.ArticleCount, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("brief get by date: %w", err)
	}
	b.TopTags = scanBriefTags(tagsRaw)
	return &b, nil
}

// Create inserts a new daily brief.
func (s *BriefStore) Create(ctx context.Context, brief *Brief) error {
	if brief.ID == uuid.Nil {
		brief.ID = uuid.New()
	}

	tagsJSON, err := json.Marshal(brief.TopTags)
	if err != nil {
		return fmt.Errorf("brief create: marshal tags: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO briefs (id, date, summary, top_tags, article_count)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (date) DO UPDATE SET
			summary = EXCLUDED.summary,
			top_tags = EXCLUDED.top_tags,
			article_count = EXCLUDED.article_count,
			created_at = now()
		RETURNING created_at
	`, brief.ID, brief.Date, brief.Summary, tagsJSON, brief.ArticleCount).Scan(&brief.CreatedAt)
	if err != nil {
		return fmt.Errorf("brief create: %w", err)
	}
	return nil
}

// List returns the most recent briefs up to the given limit.
func (s *BriefStore) List(ctx context.Context, limit int) ([]Brief, error) {
	if limit <= 0 {
		limit = 7
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, date, summary, top_tags, article_count, created_at
		FROM briefs
		ORDER BY date DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("brief list: %w", err)
	}
	defer rows.Close()

	var briefs []Brief
	for rows.Next() {
		var b Brief
		var tagsRaw []byte
		if err := rows.Scan(&b.ID, &b.Date, &b.Summary, &tagsRaw, &b.ArticleCount, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("brief scan: %w", err)
		}
		b.TopTags = scanBriefTags(tagsRaw)
		briefs = append(briefs, b)
	}

	return briefs, rows.Err()
}

// scanBriefTags unmarshals a JSONB tags column into a []string.
func scanBriefTags(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil
	}
	return tags
}
