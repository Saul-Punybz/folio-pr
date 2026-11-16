package models

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Article represents a collected news article or grant posting.
type Article struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Source            string     `json:"source"`
	SourceID          *uuid.UUID `json:"source_id,omitempty"`
	URL               string     `json:"url"`
	CanonicalURL      string     `json:"canonical_url,omitempty"`
	Region            string     `json:"region"`
	PublishedAt       *time.Time `json:"published_at,omitempty"`
	CleanText         string     `json:"clean_text,omitempty"`
	Summary           string     `json:"summary,omitempty"`
	ImageURL          string     `json:"image_url,omitempty"`
	Status            string     `json:"status"`
	Pinned            bool       `json:"pinned"`
	EvidencePolicy    string     `json:"evidence_policy,omitempty"`
	EvidenceExpiresAt *time.Time `json:"evidence_expires_at,omitempty"`
	Tags              []string   `json:"tags,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// scanTags unmarshals a JSONB tags column (scanned as []byte) into a []string.
func scanTags(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil
	}
	return tags
}

// ArticleStore provides data access methods for articles.
type ArticleStore struct {
	pool *pgxpool.Pool
}

// NewArticleStore creates a new ArticleStore.
func NewArticleStore(pool *pgxpool.Pool) *ArticleStore {
	return &ArticleStore{pool: pool}
}

// Pool returns the underlying connection pool for direct queries.
func (s *ArticleStore) Pool() *pgxpool.Pool {
	return s.pool
}

// ListByStatus returns articles filtered by status with pagination.
// Optional date filters narrow results to a specific time window.
func (s *ArticleStore) ListByStatus(ctx context.Context, status string, limit, offset int, dateFrom, dateTo *time.Time) ([]Article, error) {
	if limit <= 0 {
		limit = 50
	}

	var args []any
	args = append(args, status)
	where := "WHERE status = $1"
	argN := 2
	if status == "inbox" {
		// El Inbox muestra solo la última semana; lo anterior ya pasó a Archivo.
		where += " AND created_at >= now() - interval '7 days'"
	}

	if dateFrom != nil {
		where += fmt.Sprintf(" AND COALESCE(published_at, created_at) >= $%d", argN)
		args = append(args, *dateFrom)
		argN++
	}
	if dateTo != nil {
		where += fmt.Sprintf(" AND COALESCE(published_at, created_at) < $%d", argN)
		args = append(args, *dateTo)
		argN++
	}

	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT id, title, source, url, canonical_url, region, published_at,
		       clean_text, summary, image_url, status, pinned, evidence_policy,
		       evidence_expires_at, tags, created_at
		FROM articles
		%s
		ORDER BY pinned DESC, published_at DESC NULLS LAST, created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argN, argN+1)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("article list: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		a := scanArticleFromRow(rows)
		if a == nil {
			return nil, fmt.Errorf("article scan: failed")
		}
		articles = append(articles, *a)
	}

	return articles, rows.Err()
}

// scannable is an interface for pgx Row and Rows.
type scannable interface {
	Scan(dest ...any) error
}

// scanArticleFromRow scans a single article from a row, handling all nullable columns.
func scanArticleFromRow(row scannable) *Article {
	var a Article
	var tagsRaw []byte
	var imageURL, cleanText, summary, canonicalURL *string
	if err := row.Scan(
		&a.ID, &a.Title, &a.Source, &a.URL, &canonicalURL, &a.Region,
		&a.PublishedAt, &cleanText, &summary, &imageURL, &a.Status, &a.Pinned,
		&a.EvidencePolicy, &a.EvidenceExpiresAt, &tagsRaw, &a.CreatedAt,
	); err != nil {
		return nil
	}
	a.Tags = scanTags(tagsRaw)
	if imageURL != nil {
		a.ImageURL = *imageURL
	}
	if cleanText != nil {
		a.CleanText = *cleanText
	}
	if summary != nil {
		a.Summary = *summary
	}
	if canonicalURL != nil {
		a.CanonicalURL = *canonicalURL
	}
	return &a
}

// GetByID returns a single article by its UUID.
func (s *ArticleStore) GetByID(ctx context.Context, id uuid.UUID) (*Article, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, title, source, url, canonical_url, region, published_at,
		       clean_text, summary, image_url, status, pinned, evidence_policy,
		       evidence_expires_at, tags, created_at
		FROM articles
		WHERE id = $1
	`, id)
	a := scanArticleFromRow(row)
	if a == nil {
		return nil, fmt.Errorf("article get: scan failed")
	}
	return a, nil
}

// UpdateStatus changes an article's status.
func (s *ArticleStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE articles SET status = $1,
		       trashed_at = CASE WHEN $1 = 'trashed' THEN now() ELSE NULL END
		WHERE id = $2`, status, id)
	if err != nil {
		return fmt.Errorf("article update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("article not found: %s", id)
	}
	return nil
}

// SetPinned sets the pinned flag on an article.
func (s *ArticleStore) SetPinned(ctx context.Context, id uuid.UUID, pinned bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE articles SET pinned = $1 WHERE id = $2`, pinned, id)
	if err != nil {
		return fmt.Errorf("article set pinned: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("article not found: %s", id)
	}
	return nil
}

// Create inserts a new article. The ID and CreatedAt fields are set by the database
// if left as zero values.
func (s *ArticleStore) Create(ctx context.Context, article *Article) error {
	if article.ID == uuid.Nil {
		article.ID = uuid.New()
	}

	tags := article.Tags
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("article create: marshal tags: %w", err)
	}

	// Use nil for empty image_url to store NULL in the database.
	var imageURL *string
	if article.ImageURL != "" {
		imageURL = &article.ImageURL
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO articles (id, title, source, source_id, url, canonical_url, region,
		                      published_at, clean_text, summary, image_url, status, pinned,
		                      evidence_policy, evidence_expires_at, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at
	`,
		article.ID, article.Title, article.Source, article.SourceID, article.URL,
		article.CanonicalURL, article.Region, article.PublishedAt,
		article.CleanText, article.Summary, imageURL, article.Status, article.Pinned,
		article.EvidencePolicy, article.EvidenceExpiresAt, tagsJSON,
	).Scan(&article.CreatedAt)
	if err != nil {
		return fmt.Errorf("article create: %w", err)
	}
	return nil
}

// SetSummaryIfEmpty sets the rule-based summary only when the article has none.
func (s *ArticleStore) SetSummaryIfEmpty(ctx context.Context, id uuid.UUID, summary string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE articles SET summary = $1
		WHERE id = $2 AND (summary IS NULL OR summary = '')
	`, summary, id)
	if err != nil {
		return fmt.Errorf("article set summary: %w", err)
	}
	return nil
}

// ListRecent returns articles created in the last N hours, ordered by creation time.
func (s *ArticleStore) ListRecent(ctx context.Context, hours int) ([]Article, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, source, url, canonical_url, region, published_at,
		       clean_text, summary, image_url, status, pinned, evidence_policy,
		       evidence_expires_at, tags, created_at
		FROM articles
		WHERE status <> 'trashed' AND created_at >= now() - make_interval(hours => $1)
		ORDER BY created_at DESC
	`, hours)
	if err != nil {
		return nil, fmt.Errorf("article list recent: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		a := scanArticleFromRow(rows)
		if a == nil {
			return nil, fmt.Errorf("article list recent scan: failed")
		}
		articles = append(articles, *a)
	}

	return articles, rows.Err()
}

// CountToday returns the number of articles created since the start of today (UTC).
func (s *ArticleStore) CountToday(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM articles
		WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("article count today: %w", err)
	}
	return count, nil
}

// SetImageURL updates the image_url field on an article.
func (s *ArticleStore) SetImageURL(ctx context.Context, id uuid.UUID, imageURL string) error {
	var val *string
	if imageURL != "" {
		val = &imageURL
	}
	_, err := s.pool.Exec(ctx, `UPDATE articles SET image_url = $1 WHERE id = $2`, val, id)
	if err != nil {
		return fmt.Errorf("article set image_url: %w", err)
	}
	return nil
}

// Search performs a full-text search on articles with optional filters.
// Uses 'simple' text search config which works for both English and Spanish content.
// Supports tag filtering via the tag parameter (matches articles containing the tag).
func (s *ArticleStore) Search(ctx context.Context, query string, from, to time.Time, region, status, tag string, limit, offset int) ([]Article, error) {
	if limit <= 0 {
		limit = 50
	}

	var conditions []string
	var args []any
	argN := 1
	hasQuery := query != ""

	if hasQuery {
		conditions = append(conditions, fmt.Sprintf(
			"to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(clean_text, '')) @@ plainto_tsquery('simple', $%d)", argN))
		args = append(args, query)
		argN++
	}

	if !from.IsZero() {
		conditions = append(conditions, fmt.Sprintf("published_at >= $%d", argN))
		args = append(args, from)
		argN++
	}
	if !to.IsZero() {
		conditions = append(conditions, fmt.Sprintf("published_at <= $%d", argN))
		args = append(args, to)
		argN++
	}
	if region != "" {
		conditions = append(conditions, fmt.Sprintf("region = $%d", argN))
		args = append(args, region)
		argN++
	}
	// Archivo busca en lo nuevo, lo guardado y lo archivado; nunca en lo borrado.
	switch status {
	case "inbox", "saved", "archived":
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
	default:
		conditions = append(conditions, "status IN ('inbox', 'saved', 'archived')")
	}
	if tag != "" {
		// Filter by tag using JSONB containment: tags @> '["politics"]'::jsonb
		conditions = append(conditions, fmt.Sprintf("tags @> to_jsonb(ARRAY[$%d::text])", argN))
		args = append(args, tag)
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Use ts_rank for relevance ordering when a search query is present.
	var orderBy string
	if hasQuery {
		orderBy = fmt.Sprintf(
			"ORDER BY ts_rank(to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(clean_text, '')), plainto_tsquery('simple', $1)) DESC, published_at DESC NULLS LAST, created_at DESC")
	} else {
		orderBy = "ORDER BY published_at DESC NULLS LAST, created_at DESC"
	}

	q := fmt.Sprintf(`
		SELECT id, title, source, url, canonical_url, region, published_at,
		       clean_text, summary, image_url, status, pinned, evidence_policy,
		       evidence_expires_at, tags, created_at
		FROM articles
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, argN, argN+1)

	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("article search: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		a := scanArticleFromRow(rows)
		if a == nil {
			return nil, fmt.Errorf("article search scan: failed")
		}
		articles = append(articles, *a)
	}

	return articles, rows.Err()
}

// ExistsByURL checks whether an article with the given URL already exists.
func (s *ArticleStore) ExistsByURL(ctx context.Context, rawURL string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM articles WHERE url = $1)`, rawURL).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("article exists by url: %w", err)
	}
	return exists, nil
}

// ArchiveAfter es la edad a partir de la cual lo no tocado del Inbox pasa a Archivo.
const ArchiveAfter = 7 * 24 * time.Hour

// TrashGrace es lo que se espera antes de eliminar de verdad lo borrado
// (el «Deshacer» dura 8 s; la hora de margen cubre cierres y reintentos).
const TrashGrace = time.Hour

// Trash oculta las noticias de golpe: status 'trashed', recuerda el estado
// anterior para poder deshacer. Devuelve cuántas cambió.
func (s *ArticleStore) Trash(ctx context.Context, ids []uuid.UUID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE articles SET prev_status = status, status = 'trashed', trashed_at = now()
		WHERE id = ANY($1) AND status <> 'trashed'
	`, ids)
	if err != nil {
		return 0, fmt.Errorf("article trash: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Restore deshace un borrado: devuelve cada noticia a su estado anterior
// (o a fallback si no se recuerda). Solo toca lo que sigue en 'trashed'.
func (s *ArticleStore) Restore(ctx context.Context, ids []uuid.UUID, fallback string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if fallback != "saved" && fallback != "archived" {
		fallback = "inbox"
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE articles
		SET status = CASE WHEN prev_status IN ('inbox', 'saved', 'archived') THEN prev_status ELSE $2 END,
		    prev_status = NULL, trashed_at = NULL
		WHERE id = ANY($1) AND status = 'trashed'
	`, ids, fallback)
	if err != nil {
		return 0, fmt.Errorf("article restore: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ArchiveOld pasa a 'archived' lo del Inbox con más de 7 días. Lo guardado y
// lo archivado se conserva para siempre.
func (s *ArticleStore) ArchiveOld(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE articles SET status = 'archived'
		WHERE status = 'inbox' AND created_at < now() - make_interval(secs => $1)
	`, ArchiveAfter.Seconds())
	if err != nil {
		return 0, fmt.Errorf("article archive old: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// PurgeTrashed elimina de verdad (con sus notas, por cascada) lo borrado hace
// más de olderThan, y deja su URL y su huella de contenido bloqueadas para que
// la ingesta no lo vuelva a traer. hashURL/hashContent son los de scraper.
func (s *ArticleStore) PurgeTrashed(ctx context.Context, olderThan time.Duration, hashURL, hashContent func(string) string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("article purge: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
		SELECT id, url, COALESCE(canonical_url, ''), COALESCE(clean_text, '')
		FROM articles
		WHERE status = 'trashed'
		  AND (trashed_at IS NULL OR trashed_at < now() - make_interval(secs => $1))
	`, olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("article purge: list: %w", err)
	}
	type victim struct {
		id                   uuid.UUID
		url, canonical, text string
	}
	var victims []victim
	for rows.Next() {
		var v victim
		if err := rows.Scan(&v.id, &v.url, &v.canonical, &v.text); err != nil {
			rows.Close()
			return 0, fmt.Errorf("article purge: scan: %w", err)
		}
		victims = append(victims, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("article purge: rows: %w", err)
	}
	if len(victims) == 0 {
		return 0, nil
	}

	ids := make([]uuid.UUID, 0, len(victims))
	for _, v := range victims {
		ids = append(ids, v.id)
		contentHash := ""
		if strings.TrimSpace(v.text) != "" {
			contentHash = hashContent(v.text)
		}
		for _, u := range []string{v.url, v.canonical} {
			if strings.TrimSpace(u) == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO fingerprints (id, canonical_url_hash, content_hash, blocked)
				VALUES ($1, $2, NULLIF($3, ''), true)
				ON CONFLICT (canonical_url_hash) DO UPDATE
				SET blocked = true,
				    content_hash = COALESCE(NULLIF($3, ''), fingerprints.content_hash)
			`, uuid.New(), hashURL(u), contentHash); err != nil {
				return 0, fmt.Errorf("article purge: block: %w", err)
			}
		}
	}
	// Se borra solo lo que sigue borrado (un Deshacer a medias lo salva).
	tag, err := tx.Exec(ctx, `DELETE FROM articles WHERE id = ANY($1) AND status = 'trashed'`, ids)
	if err != nil {
		return 0, fmt.Errorf("article purge: delete: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("article purge: commit: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
