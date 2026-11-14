package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrawledPage represents a page that has been fetched by the crawler.
type CrawledPage struct {
	ID                 uuid.UUID       `json:"id"`
	URL                string          `json:"url"`
	URLHash            string          `json:"url_hash"`
	DomainID           uuid.UUID       `json:"domain_id"`
	Title              string          `json:"title"`
	CleanText          string          `json:"clean_text,omitempty"`
	ContentHash        string          `json:"content_hash"`
	Summary            string          `json:"summary"`
	Tags               json.RawMessage `json:"tags"`
	Entities           json.RawMessage `json:"entities"`
	Sentiment          string          `json:"sentiment"`
	LinksOut           int             `json:"links_out"`
	LinksIn            int             `json:"links_in"`
	Depth              int             `json:"depth"`
	StatusCode         int             `json:"status_code"`
	ContentType        string          `json:"content_type"`
	ContentLength      int             `json:"content_length"`
	Changed            bool            `json:"changed"`
	ChangeSummary      string          `json:"change_summary"`
	Enriched           bool            `json:"enriched"`
	OpportunityChecked bool            `json:"opportunity_checked"`
	CrawlCount         int             `json:"crawl_count"`
	FirstSeenAt        time.Time       `json:"first_seen_at"`
	LastCrawledAt      time.Time       `json:"last_crawled_at"`
	NextCrawlAt        *time.Time      `json:"next_crawl_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`

	// Solo para el rastreo y la extracción (no se listan en la API).
	RawHTML      string     `json:"-"`
	DocumentID   *uuid.UUID `json:"-"`
	AnchorText   string     `json:"-"`
	SourcePageID *uuid.UUID `json:"-"`
}

type CrawledPageStore struct {
	pool *pgxpool.Pool
}

func NewCrawledPageStore(pool *pgxpool.Pool) *CrawledPageStore {
	return &CrawledPageStore{pool: pool}
}

func (s *CrawledPageStore) Upsert(ctx context.Context, p *CrawledPage) (isNew bool, err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.Tags == nil {
		p.Tags = json.RawMessage("[]")
	}
	if p.Entities == nil {
		p.Entities = json.RawMessage("{}")
	}

	var existingHash string
	scanErr := s.pool.QueryRow(ctx, `
		SELECT content_hash FROM crawled_pages WHERE url_hash = $1
	`, p.URLHash).Scan(&existingHash)

	if scanErr != nil {
		// New page — insert
		err = s.pool.QueryRow(ctx, `
			INSERT INTO crawled_pages (id, url, url_hash, domain_id, title, clean_text, content_hash,
			                           links_out, depth, status_code, content_type, content_length,
			                           next_crawl_at, tags, entities, raw_html, document_id, anchor_text, source_page_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			RETURNING first_seen_at, last_crawled_at, created_at
		`, p.ID, p.URL, p.URLHash, p.DomainID, p.Title, p.CleanText, p.ContentHash,
			p.LinksOut, p.Depth, p.StatusCode, p.ContentType, p.ContentLength,
			p.NextCrawlAt, p.Tags, p.Entities, p.RawHTML, p.DocumentID, p.AnchorText, p.SourcePageID,
		).Scan(&p.FirstSeenAt, &p.LastCrawledAt, &p.CreatedAt)
		if err != nil {
			return false, fmt.Errorf("crawled page insert: %w", err)
		}
		return true, nil
	}

	// Existing page — update and detect change
	changed := existingHash != p.ContentHash
	p.Changed = changed

	err = s.pool.QueryRow(ctx, `
		UPDATE crawled_pages
		SET title = $2, clean_text = $3, content_hash = $4, links_out = $5,
		    status_code = $6, content_type = $7, content_length = $8,
		    changed = $9, enriched = CASE WHEN $9 THEN false ELSE enriched END,
		    opportunity_checked = CASE WHEN $9 OR raw_html = '' THEN false ELSE opportunity_checked END,
		    raw_html = $11, document_id = COALESCE($12, document_id),
		    anchor_text = CASE WHEN $13 <> '' THEN $13 ELSE anchor_text END,
		    source_page_id = COALESCE(source_page_id, $14),
		    crawl_count = crawl_count + 1, last_crawled_at = NOW(), next_crawl_at = $10
		WHERE url_hash = $1
		RETURNING id, first_seen_at, last_crawled_at, crawl_count, created_at
	`, p.URLHash, p.Title, p.CleanText, p.ContentHash, p.LinksOut,
		p.StatusCode, p.ContentType, p.ContentLength,
		changed, p.NextCrawlAt, p.RawHTML, p.DocumentID, p.AnchorText, p.SourcePageID,
	).Scan(&p.ID, &p.FirstSeenAt, &p.LastCrawledAt, &p.CrawlCount, &p.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("crawled page update: %w", err)
	}
	return false, nil
}

func (s *CrawledPageStore) GetByID(ctx context.Context, id uuid.UUID) (*CrawledPage, error) {
	var p CrawledPage
	err := s.pool.QueryRow(ctx, `
		SELECT id, url, url_hash, domain_id, title, clean_text, content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages WHERE id = $1
	`, id).Scan(
		&p.ID, &p.URL, &p.URLHash, &p.DomainID, &p.Title, &p.CleanText, &p.ContentHash,
		&p.Summary, &p.Tags, &p.Entities, &p.Sentiment, &p.LinksOut, &p.LinksIn,
		&p.Depth, &p.StatusCode, &p.ContentType, &p.ContentLength,
		&p.Changed, &p.ChangeSummary, &p.Enriched, &p.OpportunityChecked, &p.CrawlCount,
		&p.FirstSeenAt, &p.LastCrawledAt, &p.NextCrawlAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("crawled page get: %w", err)
	}
	return &p, nil
}

func (s *CrawledPageStore) GetByURLHash(ctx context.Context, urlHash string) (*CrawledPage, error) {
	var p CrawledPage
	err := s.pool.QueryRow(ctx, `
		SELECT id, url, url_hash, domain_id, title, clean_text, content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages WHERE url_hash = $1
	`, urlHash).Scan(
		&p.ID, &p.URL, &p.URLHash, &p.DomainID, &p.Title, &p.CleanText, &p.ContentHash,
		&p.Summary, &p.Tags, &p.Entities, &p.Sentiment, &p.LinksOut, &p.LinksIn,
		&p.Depth, &p.StatusCode, &p.ContentType, &p.ContentLength,
		&p.Changed, &p.ChangeSummary, &p.Enriched, &p.OpportunityChecked, &p.CrawlCount,
		&p.FirstSeenAt, &p.LastCrawledAt, &p.NextCrawlAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("crawled page get by hash: %w", err)
	}
	return &p, nil
}

func (s *CrawledPageStore) ListByDomain(ctx context.Context, domainID uuid.UUID, limit, offset int) ([]CrawledPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, url_hash, domain_id, title, '', content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages
		WHERE domain_id = $1
		ORDER BY last_crawled_at DESC
		LIMIT $2 OFFSET $3
	`, domainID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("crawled pages list by domain: %w", err)
	}
	defer rows.Close()
	return scanCrawledPageRows(rows)
}

func (s *CrawledPageStore) ListAll(ctx context.Context, limit, offset int) ([]CrawledPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, url_hash, domain_id, title, '', content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages
		ORDER BY last_crawled_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("crawled pages list: %w", err)
	}
	defer rows.Close()
	return scanCrawledPageRows(rows)
}

func (s *CrawledPageStore) SearchFTS(ctx context.Context, query string, limit int) ([]CrawledPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, url_hash, domain_id, title, '', content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages
		WHERE to_tsvector('simple', title || ' ' || clean_text) @@ plainto_tsquery('simple', $1)
		ORDER BY last_crawled_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("crawled pages search: %w", err)
	}
	defer rows.Close()
	return scanCrawledPageRows(rows)
}

func (s *CrawledPageStore) ListChanged(ctx context.Context, limit int) ([]CrawledPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, url_hash, domain_id, title, '', content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages
		WHERE changed = true
		ORDER BY last_crawled_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("crawled pages list changed: %w", err)
	}
	defer rows.Close()
	return scanCrawledPageRows(rows)
}

func (s *CrawledPageStore) UpdateChangeSummary(ctx context.Context, id uuid.UUID, changeSummary string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawled_pages SET change_summary = $2 WHERE id = $1
	`, id, changeSummary)
	if err != nil {
		return fmt.Errorf("crawled page update change summary: %w", err)
	}
	return nil
}

func (s *CrawledPageStore) UpdateLinksIn(ctx context.Context, id uuid.UUID, linksIn int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawled_pages SET links_in = $2 WHERE id = $1
	`, id, linksIn)
	if err != nil {
		return fmt.Errorf("crawled page update links in: %w", err)
	}
	return nil
}

func (s *CrawledPageStore) TotalCount(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM crawled_pages`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("crawled pages total count: %w", err)
	}
	return count, nil
}

// MarkOpportunityChecked sets the opportunity_checked flag to true for a page.
func (s *CrawledPageStore) MarkOpportunityChecked(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE crawled_pages SET opportunity_checked = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("crawled page mark opportunity checked: %w", err)
	}
	return nil
}

// ListUncheckedForOpportunities returns pages with text that haven't been checked for opportunities.
func (s *CrawledPageStore) ListUncheckedForOpportunities(ctx context.Context, limit int) ([]CrawledPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, url_hash, domain_id, title, clean_text, content_hash,
		       summary, tags, entities, sentiment, links_out, links_in,
		       depth, status_code, content_type, content_length,
		       changed, change_summary, enriched, opportunity_checked, crawl_count,
		       first_seen_at, last_crawled_at, next_crawl_at, created_at
		FROM crawled_pages
		WHERE opportunity_checked = false AND length(clean_text) > 100
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("crawled pages list unchecked: %w", err)
	}
	defer rows.Close()
	return scanCrawledPageRows(rows)
}

func scanCrawledPageRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]CrawledPage, error) {
	var pages []CrawledPage
	for rows.Next() {
		var p CrawledPage
		if err := rows.Scan(
			&p.ID, &p.URL, &p.URLHash, &p.DomainID, &p.Title, &p.CleanText, &p.ContentHash,
			&p.Summary, &p.Tags, &p.Entities, &p.Sentiment, &p.LinksOut, &p.LinksIn,
			&p.Depth, &p.StatusCode, &p.ContentType, &p.ContentLength,
			&p.Changed, &p.ChangeSummary, &p.Enriched, &p.OpportunityChecked, &p.CrawlCount,
			&p.FirstSeenAt, &p.LastCrawledAt, &p.NextCrawlAt, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawled page scan: %w", err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// PageForExtraction es lo que la extracción de fondos necesita de una página.
type PageForExtraction struct {
	ID           uuid.UUID
	URL          string
	DomainID     uuid.UUID
	Title        string
	CleanText    string
	RawHTML      string
	ContentType  string
	DocumentID   *uuid.UUID
	AnchorText   string
	SourcePageID *uuid.UUID
}

// ListPendingExtraction: páginas nuevas o cambiadas que la extracción de
// fondos todavía no revisó (opportunity_checked=false).
func (s *CrawledPageStore) ListPendingExtraction(ctx context.Context, limit int) ([]PageForExtraction, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, domain_id, title, clean_text, raw_html, content_type, document_id, anchor_text, source_page_id
		FROM crawled_pages
		WHERE opportunity_checked = false
		ORDER BY created_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("crawled pages pending extraction: %w", err)
	}
	defer rows.Close()
	var out []PageForExtraction
	for rows.Next() {
		var p PageForExtraction
		if err := rows.Scan(&p.ID, &p.URL, &p.DomainID, &p.Title, &p.CleanText, &p.RawHTML,
			&p.ContentType, &p.DocumentID, &p.AnchorText, &p.SourcePageID); err != nil {
			return nil, fmt.Errorf("crawled pages pending extraction scan: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TitleAndURL devuelve título y URL de una página (contexto de un PDF).
func (s *CrawledPageStore) TitleAndURL(ctx context.Context, id uuid.UUID) (title, url string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT title, url FROM crawled_pages WHERE id = $1`, id).Scan(&title, &url)
	return title, url, err
}

// MarkOpportunityUnchecked vuelve a poner una página en la cola de la extracción.
func (s *CrawledPageStore) MarkOpportunityUnchecked(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE crawled_pages SET opportunity_checked = false WHERE id = $1`, id)
	return err
}
