package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrawlRun tracks a single crawl session.
type CrawlRun struct {
	ID              uuid.UUID  `json:"id"`
	Status          string     `json:"status"`
	PagesCrawled    int        `json:"pages_crawled"`
	PagesNew        int        `json:"pages_new"`
	PagesChanged    int        `json:"pages_changed"`
	PagesEnriched   int        `json:"pages_enriched"`
	PagesFailed     int        `json:"pages_failed"`
	DomainsVisited  int        `json:"domains_visited"`
	LinksDiscovered int        `json:"links_discovered"`
	ErrorMsg        string     `json:"error_msg"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type CrawlRunStore struct {
	pool *pgxpool.Pool
}

func NewCrawlRunStore(pool *pgxpool.Pool) *CrawlRunStore {
	return &CrawlRunStore{pool: pool}
}

func (s *CrawlRunStore) Create(ctx context.Context) (*CrawlRun, error) {
	var r CrawlRun
	err := s.pool.QueryRow(ctx, `
		INSERT INTO crawl_runs DEFAULT VALUES
		RETURNING id, status, pages_crawled, pages_new, pages_changed, pages_enriched,
		          pages_failed, domains_visited, links_discovered, error_msg, started_at, finished_at
	`).Scan(
		&r.ID, &r.Status, &r.PagesCrawled, &r.PagesNew, &r.PagesChanged, &r.PagesEnriched,
		&r.PagesFailed, &r.DomainsVisited, &r.LinksDiscovered, &r.ErrorMsg, &r.StartedAt, &r.FinishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("crawl run create: %w", err)
	}
	return &r, nil
}

func (s *CrawlRunStore) Update(ctx context.Context, r *CrawlRun) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_runs
		SET status = $2, pages_crawled = $3, pages_new = $4, pages_changed = $5,
		    pages_enriched = $6, pages_failed = $7, domains_visited = $8,
		    links_discovered = $9, error_msg = $10, finished_at = $11
		WHERE id = $1
	`, r.ID, r.Status, r.PagesCrawled, r.PagesNew, r.PagesChanged,
		r.PagesEnriched, r.PagesFailed, r.DomainsVisited,
		r.LinksDiscovered, r.ErrorMsg, r.FinishedAt)
	if err != nil {
		return fmt.Errorf("crawl run update: %w", err)
	}
	return nil
}

func (s *CrawlRunStore) GetLatest(ctx context.Context) (*CrawlRun, error) {
	var r CrawlRun
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, pages_crawled, pages_new, pages_changed, pages_enriched,
		       pages_failed, domains_visited, links_discovered, error_msg, started_at, finished_at
		FROM crawl_runs
		ORDER BY started_at DESC
		LIMIT 1
	`).Scan(
		&r.ID, &r.Status, &r.PagesCrawled, &r.PagesNew, &r.PagesChanged, &r.PagesEnriched,
		&r.PagesFailed, &r.DomainsVisited, &r.LinksDiscovered, &r.ErrorMsg, &r.StartedAt, &r.FinishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("crawl run get latest: %w", err)
	}
	return &r, nil
}

func (s *CrawlRunStore) ListRecent(ctx context.Context, limit int) ([]CrawlRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, pages_crawled, pages_new, pages_changed, pages_enriched,
		       pages_failed, domains_visited, links_discovered, error_msg, started_at, finished_at
		FROM crawl_runs
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("crawl runs list: %w", err)
	}
	defer rows.Close()

	var runs []CrawlRun
	for rows.Next() {
		var r CrawlRun
		if err := rows.Scan(
			&r.ID, &r.Status, &r.PagesCrawled, &r.PagesNew, &r.PagesChanged, &r.PagesEnriched,
			&r.PagesFailed, &r.DomainsVisited, &r.LinksDiscovered, &r.ErrorMsg, &r.StartedAt, &r.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl run scan: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
