package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Source represents a news or grants feed source configuration.
type Source struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"base_url"`
	Region        string    `json:"region"`
	FeedType      string    `json:"feed_type"`
	FeedURL       string    `json:"feed_url,omitempty"`
	ListURLs      []string  `json:"list_urls,omitempty"`
	LinkSelector  string    `json:"link_selector,omitempty"`
	TitleSelector string    `json:"title_selector,omitempty"`
	BodySelector  string    `json:"body_selector,omitempty"`
	DateSelector  string    `json:"date_selector,omitempty"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

// SourceStore provides data access methods for sources.
type SourceStore struct {
	pool *pgxpool.Pool
}

// NewSourceStore creates a new SourceStore.
func NewSourceStore(pool *pgxpool.Pool) *SourceStore {
	return &SourceStore{pool: pool}
}

// ListAll returns all sources regardless of active status.
func (s *SourceStore) ListAll(ctx context.Context) ([]Source, error) {
	return s.listSources(ctx, false)
}

// ListActive returns all sources where active = true.
func (s *SourceStore) ListActive(ctx context.Context) ([]Source, error) {
	return s.listSources(ctx, true)
}

func (s *SourceStore) listSources(ctx context.Context, activeOnly bool) ([]Source, error) {
	query := `
		SELECT id, name, base_url, region, feed_type, feed_url, list_urls,
		       link_selector, title_selector, body_selector, date_selector,
		       active, created_at
		FROM sources
	`
	if activeOnly {
		query += " WHERE active = true"
	}
	query += " ORDER BY name ASC"

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("source list active: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		var listURLsJSON []byte
		var feedURL, linkSel, titleSel, bodySel, dateSel *string
		if err := rows.Scan(
			&src.ID, &src.Name, &src.BaseURL, &src.Region, &src.FeedType,
			&feedURL, &listURLsJSON, &linkSel, &titleSel,
			&bodySel, &dateSel, &src.Active, &src.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("source scan: %w", err)
		}
		if feedURL != nil {
			src.FeedURL = *feedURL
		}
		if linkSel != nil {
			src.LinkSelector = *linkSel
		}
		if titleSel != nil {
			src.TitleSelector = *titleSel
		}
		if bodySel != nil {
			src.BodySelector = *bodySel
		}
		if dateSel != nil {
			src.DateSelector = *dateSel
		}
		if listURLsJSON != nil {
			if err := json.Unmarshal(listURLsJSON, &src.ListURLs); err != nil {
				return nil, fmt.Errorf("source unmarshal list_urls: %w", err)
			}
		}
		sources = append(sources, src)
	}

	return sources, rows.Err()
}

// Create inserts a new source.
func (s *SourceStore) Create(ctx context.Context, source *Source) error {
	if source.ID == uuid.Nil {
		source.ID = uuid.New()
	}

	listURLsJSON, err := json.Marshal(source.ListURLs)
	if err != nil {
		return fmt.Errorf("source marshal list_urls: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO sources (id, name, base_url, region, feed_type, feed_url,
		                     list_urls, link_selector, title_selector,
		                     body_selector, date_selector, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`,
		source.ID, source.Name, source.BaseURL, source.Region, source.FeedType,
		source.FeedURL, listURLsJSON, source.LinkSelector, source.TitleSelector,
		source.BodySelector, source.DateSelector, source.Active,
	).Scan(&source.CreatedAt)
	if err != nil {
		return fmt.Errorf("source create: %w", err)
	}
	return nil
}

// Update modifies an existing source.
func (s *SourceStore) Update(ctx context.Context, source *Source) error {
	listURLsJSON, err := json.Marshal(source.ListURLs)
	if err != nil {
		return fmt.Errorf("source marshal list_urls: %w", err)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE sources
		SET name = $1, base_url = $2, region = $3, feed_type = $4, feed_url = $5,
		    list_urls = $6, link_selector = $7, title_selector = $8,
		    body_selector = $9, date_selector = $10, active = $11
		WHERE id = $12
	`,
		source.Name, source.BaseURL, source.Region, source.FeedType,
		source.FeedURL, listURLsJSON, source.LinkSelector, source.TitleSelector,
		source.BodySelector, source.DateSelector, source.Active, source.ID,
	)
	if err != nil {
		return fmt.Errorf("source update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %s", source.ID)
	}
	return nil
}

// ToggleActive sets only the active flag on a source without modifying other fields.
func (s *SourceStore) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE sources SET active = $1 WHERE id = $2`, active, id)
	if err != nil {
		return fmt.Errorf("source toggle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %s", id)
	}
	return nil
}

// Delete removes a source by ID.
func (s *SourceStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("source delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("source not found: %s", id)
	}
	return nil
}
