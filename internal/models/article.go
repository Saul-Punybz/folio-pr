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
	tag, err := s.pool.Exec(ctx, `UPDATE articles SET status = $1 WHERE id = $2`, status, id)
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
		WHERE created_at >= now() - make_interval(hours => $1)
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

// UpdateRetention updates the evidence policy and recalculates the expiry date.
func (s *ArticleStore) UpdateRetention(ctx context.Context, id uuid.UUID, policy string) error {
	var expiresAt *time.Time
	now := time.Now().UTC()

	switch policy {
	case "ret_6m":
		t := now.AddDate(0, 6, 0)
		expiresAt = &t
	case "ret_12m":
		t := now.AddDate(1, 0, 0)
		expiresAt = &t
	case "keep":
		expiresAt = nil
	default:
		return fmt.Errorf("article update retention: invalid policy %q", policy)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE articles SET evidence_policy = $1, evidence_expires_at = $2 WHERE id = $3
	`, policy, expiresAt, id)
	if err != nil {
		return fmt.Errorf("article update retention: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("article not found: %s", id)
	}
	return nil
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

// ListExpiredEvidence returns articles whose evidence has expired and should be cleaned.
func (s *ArticleStore) ListExpiredEvidence(ctx context.Context) ([]Article, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, source, url, canonical_url, region, published_at,
		       clean_text, summary, image_url, status, pinned, evidence_policy,
		       evidence_expires_at, tags, created_at
		FROM articles
		WHERE evidence_expires_at < now()
		  AND evidence_policy != 'keep'
		  AND evidence_expires_at IS NOT NULL
		ORDER BY evidence_expires_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("article list expired evidence: %w", err)
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		a := scanArticleFromRow(rows)
		if a == nil {
			return nil, fmt.Errorf("article expired evidence scan: failed")
		}
		articles = append(articles, *a)
	}

	return articles, rows.Err()
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

// ClearEvidenceExpiry sets evidence_expires_at to NULL for the given article.
func (s *ArticleStore) ClearEvidenceExpiry(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE articles SET evidence_expires_at = NULL WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("article clear evidence expiry: %w", err)
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
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
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
