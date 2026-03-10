package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrawlLink represents a link from one crawled page to another URL.
type CrawlLink struct {
	SourcePageID uuid.UUID  `json:"source_page_id"`
	TargetURL    string     `json:"target_url"`
	TargetHash   string     `json:"target_hash"`
	TargetPageID *uuid.UUID `json:"target_page_id,omitempty"`
	AnchorText   string     `json:"anchor_text"`
	IsExternal   bool       `json:"is_external"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CrawlLinkStore struct {
	pool *pgxpool.Pool
}

func NewCrawlLinkStore(pool *pgxpool.Pool) *CrawlLinkStore {
	return &CrawlLinkStore{pool: pool}
}

// BulkInsert inserts multiple links for a page, ignoring duplicates.
func (s *CrawlLinkStore) BulkInsert(ctx context.Context, links []CrawlLink) error {
	if len(links) == 0 {
		return nil
	}
	for _, l := range links {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO crawl_links (source_page_id, target_url, target_hash, target_page_id, anchor_text, is_external)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (source_page_id, target_hash) DO NOTHING
		`, l.SourcePageID, l.TargetURL, l.TargetHash, l.TargetPageID, l.AnchorText, l.IsExternal)
		if err != nil {
			return fmt.Errorf("crawl link insert: %w", err)
		}
	}
	return nil
}

// ListOutbound returns all links from a given page.
func (s *CrawlLinkStore) ListOutbound(ctx context.Context, pageID uuid.UUID) ([]CrawlLink, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source_page_id, target_url, target_hash, target_page_id, anchor_text, is_external, created_at
		FROM crawl_links
		WHERE source_page_id = $1
		ORDER BY created_at
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("crawl links outbound: %w", err)
	}
	defer rows.Close()

	var links []CrawlLink
	for rows.Next() {
		var l CrawlLink
		if err := rows.Scan(
			&l.SourcePageID, &l.TargetURL, &l.TargetHash, &l.TargetPageID,
			&l.AnchorText, &l.IsExternal, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl link scan: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ListInbound returns all links pointing to a given page.
func (s *CrawlLinkStore) ListInbound(ctx context.Context, pageID uuid.UUID) ([]CrawlLink, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source_page_id, target_url, target_hash, target_page_id, anchor_text, is_external, created_at
		FROM crawl_links
		WHERE target_page_id = $1
		ORDER BY created_at
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("crawl links inbound: %w", err)
	}
	defer rows.Close()

	var links []CrawlLink
	for rows.Next() {
		var l CrawlLink
		if err := rows.Scan(
			&l.SourcePageID, &l.TargetURL, &l.TargetHash, &l.TargetPageID,
			&l.AnchorText, &l.IsExternal, &l.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl link scan: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// DeleteBySource removes all links from a given source page. Used before
// re-inserting links on re-crawl.
func (s *CrawlLinkStore) DeleteBySource(ctx context.Context, pageID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM crawl_links WHERE source_page_id = $1`, pageID)
	if err != nil {
		return fmt.Errorf("crawl links delete by source: %w", err)
	}
	return nil
}

// TotalCount returns the total number of link records.
func (s *CrawlLinkStore) TotalCount(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM crawl_links`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("crawl links total count: %w", err)
	}
	return count, nil
}
