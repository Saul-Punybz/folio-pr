package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CrawlQueueItem represents a URL in the crawl frontier.
type CrawlQueueItem struct {
	ID           uuid.UUID  `json:"id"`
	URL          string     `json:"url"`
	URLHash      string     `json:"url_hash"`
	DomainID     uuid.UUID  `json:"domain_id"`
	Depth        int        `json:"depth"`
	Priority     int        `json:"priority"`
	Status       string     `json:"status"`
	DiscoveredBy *uuid.UUID `json:"discovered_by,omitempty"`
	Kind         string     `json:"kind"`        // "page" | "pdf"
	AnchorText   string     `json:"anchor_text"` // texto del enlace que lo trajo
	ErrorMsg     string     `json:"error_msg"`
	Attempts     int        `json:"attempts"`
	ScheduledAt  time.Time  `json:"scheduled_at"`
	ClaimedAt    *time.Time `json:"claimed_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type CrawlQueueStore struct {
	pool *pgxpool.Pool
}

func NewCrawlQueueStore(pool *pgxpool.Pool) *CrawlQueueStore {
	return &CrawlQueueStore{pool: pool}
}

// ClaimBatch atomically claims up to `limit` pending items from the queue using
// FOR UPDATE SKIP LOCKED to allow concurrent workers. Primero lo menos
// profundo (páginas de arranque) y, dentro de cada nivel, la prioridad más alta.
func (s *CrawlQueueStore) ClaimBatch(ctx context.Context, limit int) ([]CrawlQueueItem, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE crawl_queue
		SET status = 'in_progress', claimed_at = NOW(), attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM crawl_queue
			WHERE status = 'pending' AND scheduled_at <= NOW()
			ORDER BY depth ASC, priority DESC, scheduled_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, url, url_hash, domain_id, depth, priority, status,
		          discovered_by, kind, anchor_text, error_msg, attempts, scheduled_at, claimed_at, finished_at, created_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("crawl queue claim batch: %w", err)
	}
	defer rows.Close()

	var items []CrawlQueueItem
	for rows.Next() {
		var q CrawlQueueItem
		if err := rows.Scan(
			&q.ID, &q.URL, &q.URLHash, &q.DomainID, &q.Depth, &q.Priority, &q.Status,
			&q.DiscoveredBy, &q.Kind, &q.AnchorText, &q.ErrorMsg, &q.Attempts, &q.ScheduledAt, &q.ClaimedAt, &q.FinishedAt, &q.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("crawl queue scan: %w", err)
		}
		items = append(items, q)
	}
	return items, rows.Err()
}

// Enqueue adds a URL to the queue. Silently ignores duplicates.
func (s *CrawlQueueStore) Enqueue(ctx context.Context, q *CrawlQueueItem) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	if q.Kind == "" {
		q.Kind = "page"
	}
	if q.Priority < 1 {
		q.Priority = 1
	}
	if q.Priority > 10 {
		q.Priority = 10
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crawl_queue (id, url, url_hash, domain_id, depth, priority, discovered_by, kind, anchor_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (url_hash) DO NOTHING
	`, q.ID, q.URL, q.URLHash, q.DomainID, q.Depth, q.Priority, q.DiscoveredBy, q.Kind, q.AnchorText)
	if err != nil {
		return fmt.Errorf("crawl queue enqueue: %w", err)
	}
	return nil
}

// MarkDone marks a queue item as completed.
func (s *CrawlQueueStore) MarkDone(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_queue SET status = 'done', finished_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("crawl queue mark done: %w", err)
	}
	return nil
}

// MarkFailed marks a queue item as failed with an error message.
func (s *CrawlQueueStore) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_queue SET status = 'failed', error_msg = $2, finished_at = NOW() WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("crawl queue mark failed: %w", err)
	}
	return nil
}

// MarkSkipped marks a queue item as skipped (e.g., disallowed by robots.txt).
func (s *CrawlQueueStore) MarkSkipped(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawl_queue SET status = 'skipped', finished_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("crawl queue mark skipped: %w", err)
	}
	return nil
}

// ReenqueueForRecrawl vuelve a poner en cola (pending) las páginas HTML que
// pasaron su next_crawl_at. Antes hacía ON CONFLICT DO NOTHING y, como la
// fila vieja de la cola sigue ahí (done), nunca se volvía a rastrear nada.
// Los PDF no se vuelven a bajar: un PDF nuevo trae otra URL.
func (s *CrawlQueueStore) ReenqueueForRecrawl(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO crawl_queue (url, url_hash, domain_id, depth, priority)
		SELECT p.url, p.url_hash, p.domain_id, p.depth, d.priority
		FROM crawled_pages p
		JOIN crawl_domains d ON d.id = p.domain_id
		WHERE d.active = true
		  AND p.document_id IS NULL
		  AND p.content_type NOT ILIKE '%pdf%'
		  AND p.next_crawl_at IS NOT NULL
		  AND p.next_crawl_at <= NOW()
		ON CONFLICT (url_hash) DO UPDATE SET
		  status = 'pending', scheduled_at = NOW(), attempts = 0, error_msg = '',
		  claimed_at = NULL, finished_at = NULL
		WHERE crawl_queue.status IN ('done', 'failed', 'skipped')
	`)
	if err != nil {
		return 0, fmt.Errorf("crawl queue reenqueue: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// CountsByStatus returns the number of queue items grouped by status.
func (s *CrawlQueueStore) CountsByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM crawl_queue GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("crawl queue counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("crawl queue counts scan: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// EnqueueEntry pone en cola la URL de arranque de un dominio. Si ya estaba y
// falló o se saltó hace más de 6 horas, la vuelve a intentar (las que
// terminaron bien vuelven por ReenqueueForRecrawl cuando les toca).
// Con force, la vuelve a poner pending aunque haya terminado bien.
func (s *CrawlQueueStore) EnqueueEntry(ctx context.Context, q *CrawlQueueItem, force bool) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO crawl_queue (id, url, url_hash, domain_id, depth, priority, kind)
		VALUES ($1, $2, $3, $4, 0, $5, 'page')
		ON CONFLICT (url_hash) DO UPDATE SET
		  status = 'pending', scheduled_at = NOW(), attempts = 0, error_msg = '',
		  depth = 0, claimed_at = NULL, finished_at = NULL
		WHERE ($6 AND crawl_queue.status IN ('done', 'failed', 'skipped'))
		   OR (crawl_queue.status IN ('failed', 'skipped')
		       AND crawl_queue.finished_at < NOW() - INTERVAL '6 hours')
	`, q.ID, q.URL, q.URLHash, q.DomainID, q.Priority, force)
	if err != nil {
		return fmt.Errorf("crawl queue enqueue entry: %w", err)
	}
	return nil
}

// ReleaseStale devuelve a pending lo que quedó in_progress de una corrida
// que se cortó (apagado a mitad de un crawl). Solo corre un crawl a la vez
// (candado de la tarea), así que todo lo in_progress al empezar es viejo.
func (s *CrawlQueueStore) ReleaseStale(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE crawl_queue SET status = 'pending', claimed_at = NULL
		WHERE status = 'in_progress'`)
	if err != nil {
		return 0, fmt.Errorf("crawl queue release stale: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
