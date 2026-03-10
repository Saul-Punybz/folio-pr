package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrawlDomain represents an allowlisted domain the crawler can visit.
type CrawlDomain struct {
	ID            uuid.UUID  `json:"id"`
	Domain        string     `json:"domain"`
	Label         string     `json:"label"`
	Category      string     `json:"category"`
	MaxDepth      int        `json:"max_depth"`
	RecrawlHours  int        `json:"recrawl_hours"`
	Priority      int        `json:"priority"`
	Active        bool       `json:"active"`
	PageCount     int        `json:"page_count"`
	LastCrawledAt *time.Time `json:"last_crawled_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type CrawlDomainStore struct {
	pool *pgxpool.Pool
}

func NewCrawlDomainStore(pool *pgxpool.Pool) *CrawlDomainStore {
	return &CrawlDomainStore{pool: pool}
}

func (s *CrawlDomainStore) List(ctx context.Context) ([]CrawlDomain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, domain, label, category, max_depth, recrawl_hours, priority,
		       active, page_count, last_crawled_at, created_at
		FROM crawl_domains
		ORDER BY priority DESC, domain
	`)
	if err != nil {
		return nil, fmt.Errorf("crawl domains list: %w", err)
	}
	defer rows.Close()

	var domains []CrawlDomain
	for rows.Next() {
		var d CrawlDomain
		if err := rows.Scan(
			&d.ID, &d.Domain, &d.Label, &d.Category, &d.MaxDepth, &d.RecrawlHours,
			&d.Priority, &d.Active, &d.PageCount, &d.LastCrawledAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl domain scan: %w", err)
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func (s *CrawlDomainStore) ListActive(ctx context.Context) ([]CrawlDomain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, domain, label, category, max_depth, recrawl_hours, priority,
		       active, page_count, last_crawled_at, created_at
		FROM crawl_domains
		WHERE active = true
		ORDER BY priority DESC, domain
	`)
	if err != nil {
		return nil, fmt.Errorf("crawl domains list active: %w", err)
	}
	defer rows.Close()

	var domains []CrawlDomain
	for rows.Next() {
		var d CrawlDomain
		if err := rows.Scan(
			&d.ID, &d.Domain, &d.Label, &d.Category, &d.MaxDepth, &d.RecrawlHours,
			&d.Priority, &d.Active, &d.PageCount, &d.LastCrawledAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl domain scan: %w", err)
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

func (s *CrawlDomainStore) GetByID(ctx context.Context, id uuid.UUID) (*CrawlDomain, error) {
	var d CrawlDomain
	err := s.pool.QueryRow(ctx, `
		SELECT id, domain, label, category, max_depth, recrawl_hours, priority,
		       active, page_count, last_crawled_at, created_at
		FROM crawl_domains WHERE id = $1
	`, id).Scan(
		&d.ID, &d.Domain, &d.Label, &d.Category, &d.MaxDepth, &d.RecrawlHours,
		&d.Priority, &d.Active, &d.PageCount, &d.LastCrawledAt, &d.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("crawl domain get: %w", err)
	}
	return &d, nil
}

func (s *CrawlDomainStore) Create(ctx context.Context, d *CrawlDomain) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO crawl_domains (id, domain, label, category, max_depth, recrawl_hours, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING active, page_count, last_crawled_at, created_at
	`, d.ID, d.Domain, d.Label, d.Category, d.MaxDepth, d.RecrawlHours, d.Priority).Scan(
		&d.Active, &d.PageCount, &d.LastCrawledAt, &d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("crawl domain create: %w", err)
	}
	return nil
}

func (s *CrawlDomainStore) Update(ctx context.Context, d *CrawlDomain) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE crawl_domains
		SET label = $2, category = $3, max_depth = $4, recrawl_hours = $5, priority = $6
		WHERE id = $1
	`, d.ID, d.Label, d.Category, d.MaxDepth, d.RecrawlHours, d.Priority)
	if err != nil {
		return fmt.Errorf("crawl domain update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("crawl domain not found: %s", d.ID)
	}
	return nil
}

func (s *CrawlDomainStore) ToggleActive(ctx context.Context, id uuid.UUID, active bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE crawl_domains SET active = $2 WHERE id = $1
	`, id, active)
	if err != nil {
		return fmt.Errorf("crawl domain toggle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("crawl domain not found: %s", id)
	}
	return nil
}

func (s *CrawlDomainStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM crawl_domains WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("crawl domain delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("crawl domain not found: %s", id)
	}
	return nil
}

func (s *CrawlDomainStore) IncrementPageCount(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_domains
		SET page_count = page_count + 1, last_crawled_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("crawl domain increment page count: %w", err)
	}
	return nil
}
