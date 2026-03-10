package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Escrito represents an SEO-optimized article generated from news sources.
type Escrito struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	Topic           string          `json:"topic"`
	Slug            string          `json:"slug"`
	Title           string          `json:"title"`
	MetaDescription string          `json:"meta_description"`
	Keywords        []string        `json:"keywords"`
	Hashtags        []string        `json:"hashtags"`
	Content         string          `json:"content"`
	ArticlePlan     json.RawMessage `json:"article_plan"`
	SEOScore        json.RawMessage `json:"seo_score"`
	Status          string          `json:"status"`
	Phase           int             `json:"phase"`
	Progress        json.RawMessage `json:"progress"`
	PublishStatus   string          `json:"publish_status"`
	WordCount       int             `json:"word_count"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// EscritoSource links an escrito to a source article.
type EscritoSource struct {
	ID            uuid.UUID `json:"id"`
	EscritoID     uuid.UUID `json:"escrito_id"`
	ArticleID     uuid.UUID `json:"article_id"`
	Relevance     float32   `json:"relevance"`
	UsedInSection string    `json:"used_in_section"`
	// Joined fields from articles table.
	ArticleTitle  string `json:"article_title,omitempty"`
	ArticleSource string `json:"article_source,omitempty"`
	ArticleURL    string `json:"article_url,omitempty"`
}

// ── EscritoStore ─────────────────────────────────────────────────

type EscritoStore struct {
	pool *pgxpool.Pool
}

func NewEscritoStore(pool *pgxpool.Pool) *EscritoStore {
	return &EscritoStore{pool: pool}
}

func (s *EscritoStore) Create(ctx context.Context, e *Escrito) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	kwJSON, err := json.Marshal(e.Keywords)
	if err != nil {
		return fmt.Errorf("escrito create: marshal keywords: %w", err)
	}
	htJSON, err := json.Marshal(e.Hashtags)
	if err != nil {
		return fmt.Errorf("escrito create: marshal hashtags: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO escritos (id, user_id, topic, keywords, hashtags)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING slug, title, meta_description, content, article_plan, seo_score,
		          status, phase, progress, publish_status, word_count,
		          started_at, finished_at, created_at, updated_at
	`, e.ID, e.UserID, e.Topic, kwJSON, htJSON).Scan(
		&e.Slug, &e.Title, &e.MetaDescription, &e.Content, &e.ArticlePlan, &e.SEOScore,
		&e.Status, &e.Phase, &e.Progress, &e.PublishStatus, &e.WordCount,
		&e.StartedAt, &e.FinishedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("escrito create: %w", err)
	}
	return nil
}

func (s *EscritoStore) GetByID(ctx context.Context, id uuid.UUID) (*Escrito, error) {
	var e Escrito
	var kwRaw, htRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, topic, slug, title, meta_description, keywords, hashtags,
		       content, article_plan, seo_score, status, phase, progress,
		       publish_status, word_count, started_at, finished_at, created_at, updated_at
		FROM escritos
		WHERE id = $1
	`, id).Scan(
		&e.ID, &e.UserID, &e.Topic, &e.Slug, &e.Title, &e.MetaDescription,
		&kwRaw, &htRaw, &e.Content, &e.ArticlePlan, &e.SEOScore,
		&e.Status, &e.Phase, &e.Progress, &e.PublishStatus, &e.WordCount,
		&e.StartedAt, &e.FinishedAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("escrito get: %w", err)
	}
	e.Keywords = scanJSONStringSlice(kwRaw)
	e.Hashtags = scanJSONStringSlice(htRaw)
	return &e, nil
}

func (s *EscritoStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]Escrito, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, topic, slug, title, meta_description, keywords, hashtags,
		       content, article_plan, seo_score, status, phase, progress,
		       publish_status, word_count, started_at, finished_at, created_at, updated_at
		FROM escritos
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("escritos list: %w", err)
	}
	defer rows.Close()

	var escritos []Escrito
	for rows.Next() {
		var e Escrito
		var kwRaw, htRaw []byte
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Topic, &e.Slug, &e.Title, &e.MetaDescription,
			&kwRaw, &htRaw, &e.Content, &e.ArticlePlan, &e.SEOScore,
			&e.Status, &e.Phase, &e.Progress, &e.PublishStatus, &e.WordCount,
			&e.StartedAt, &e.FinishedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("escrito scan: %w", err)
		}
		e.Keywords = scanJSONStringSlice(kwRaw)
		e.Hashtags = scanJSONStringSlice(htRaw)
		escritos = append(escritos, e)
	}
	return escritos, rows.Err()
}

func (s *EscritoStore) ListQueued(ctx context.Context) ([]Escrito, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, topic, slug, title, meta_description, keywords, hashtags,
		       content, article_plan, seo_score, status, phase, progress,
		       publish_status, word_count, started_at, finished_at, created_at, updated_at
		FROM escritos
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("escritos list queued: %w", err)
	}
	defer rows.Close()

	var escritos []Escrito
	for rows.Next() {
		var e Escrito
		var kwRaw, htRaw []byte
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Topic, &e.Slug, &e.Title, &e.MetaDescription,
			&kwRaw, &htRaw, &e.Content, &e.ArticlePlan, &e.SEOScore,
			&e.Status, &e.Phase, &e.Progress, &e.PublishStatus, &e.WordCount,
			&e.StartedAt, &e.FinishedAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("escrito scan: %w", err)
		}
		e.Keywords = scanJSONStringSlice(kwRaw)
		e.Hashtags = scanJSONStringSlice(htRaw)
		escritos = append(escritos, e)
	}
	return escritos, rows.Err()
}

func (s *EscritoStore) UpdateContent(ctx context.Context, id uuid.UUID, title, slug, metaDesc, content string, keywords, hashtags []string, wordCount int, plan json.RawMessage) error {
	kwJSON, err := json.Marshal(keywords)
	if err != nil {
		return fmt.Errorf("escrito update content: marshal keywords: %w", err)
	}
	htJSON, err := json.Marshal(hashtags)
	if err != nil {
		return fmt.Errorf("escrito update content: marshal hashtags: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE escritos
		SET title = $2, slug = $3, meta_description = $4, content = $5,
		    keywords = $6, hashtags = $7, word_count = $8, article_plan = $9,
		    updated_at = NOW()
		WHERE id = $1
	`, id, title, slug, metaDesc, content, kwJSON, htJSON, wordCount, plan)
	if err != nil {
		return fmt.Errorf("escrito update content: %w", err)
	}
	return nil
}

func (s *EscritoStore) UpdateSEOScore(ctx context.Context, id uuid.UUID, score json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos SET seo_score = $2, updated_at = NOW() WHERE id = $1
	`, id, score)
	if err != nil {
		return fmt.Errorf("escrito update seo score: %w", err)
	}
	return nil
}

func (s *EscritoStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string, phase int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos SET status = $2, phase = $3, updated_at = NOW() WHERE id = $1
	`, id, status, phase)
	if err != nil {
		return fmt.Errorf("escrito update status: %w", err)
	}
	return nil
}

func (s *EscritoStore) UpdateProgress(ctx context.Context, id uuid.UUID, progress json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos SET progress = $2, updated_at = NOW() WHERE id = $1
	`, id, progress)
	if err != nil {
		return fmt.Errorf("escrito update progress: %w", err)
	}
	return nil
}

func (s *EscritoStore) SetStarted(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos
		SET started_at = NOW(), status = 'planning', phase = 1, updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("escrito set started: %w", err)
	}
	return nil
}

func (s *EscritoStore) SetFinished(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos
		SET status = 'done', finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("escrito set finished: %w", err)
	}
	return nil
}

func (s *EscritoStore) SetFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos
		SET status = 'failed', progress = jsonb_set(COALESCE(progress, '{}'), '{error}', to_jsonb($2::text)),
		    finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("escrito set failed: %w", err)
	}
	return nil
}

func (s *EscritoStore) UpdatePublishStatus(ctx context.Context, id uuid.UUID, publishStatus string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos SET publish_status = $2, updated_at = NOW() WHERE id = $1
	`, id, publishStatus)
	if err != nil {
		return fmt.Errorf("escrito update publish status: %w", err)
	}
	return nil
}

func (s *EscritoStore) UpdateEdited(ctx context.Context, id uuid.UUID, title, metaDesc, content string, keywords, hashtags []string) error {
	kwJSON, err := json.Marshal(keywords)
	if err != nil {
		return fmt.Errorf("escrito update edited: marshal keywords: %w", err)
	}
	htJSON, err := json.Marshal(hashtags)
	if err != nil {
		return fmt.Errorf("escrito update edited: marshal hashtags: %w", err)
	}
	wordCount := countWords(content)
	_, err = s.pool.Exec(ctx, `
		UPDATE escritos
		SET title = $2, meta_description = $3, content = $4, keywords = $5,
		    hashtags = $6, word_count = $7, updated_at = NOW()
		WHERE id = $1
	`, id, title, metaDesc, content, kwJSON, htJSON, wordCount)
	if err != nil {
		return fmt.Errorf("escrito update edited: %w", err)
	}
	return nil
}

func (s *EscritoStore) SetArticlePlan(ctx context.Context, id uuid.UUID, plan json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE escritos SET article_plan = $2, updated_at = NOW() WHERE id = $1
	`, id, plan)
	if err != nil {
		return fmt.Errorf("escrito set article plan: %w", err)
	}
	return nil
}

func (s *EscritoStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM escritos WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("escrito delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("escrito not found: %s", id)
	}
	return nil
}

// ── EscritoSourceStore ───────────────────────────────────────────

type EscritoSourceStore struct {
	pool *pgxpool.Pool
}

func NewEscritoSourceStore(pool *pgxpool.Pool) *EscritoSourceStore {
	return &EscritoSourceStore{pool: pool}
}

func (s *EscritoSourceStore) Create(ctx context.Context, src *EscritoSource) error {
	if src.ID == uuid.Nil {
		src.ID = uuid.New()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO escrito_sources (id, escrito_id, article_id, relevance, used_in_section)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (escrito_id, article_id) DO NOTHING
	`, src.ID, src.EscritoID, src.ArticleID, src.Relevance, src.UsedInSection)
	if err != nil {
		return fmt.Errorf("escrito source create: %w", err)
	}
	return nil
}

func (s *EscritoSourceStore) ListByEscrito(ctx context.Context, escritoID uuid.UUID) ([]EscritoSource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT es.id, es.escrito_id, es.article_id, es.relevance, es.used_in_section,
		       a.title, a.source, a.url
		FROM escrito_sources es
		JOIN articles a ON a.id = es.article_id
		WHERE es.escrito_id = $1
		ORDER BY es.relevance DESC
	`, escritoID)
	if err != nil {
		return nil, fmt.Errorf("escrito sources list: %w", err)
	}
	defer rows.Close()

	var sources []EscritoSource
	for rows.Next() {
		var src EscritoSource
		if err := rows.Scan(
			&src.ID, &src.EscritoID, &src.ArticleID, &src.Relevance, &src.UsedInSection,
			&src.ArticleTitle, &src.ArticleSource, &src.ArticleURL,
		); err != nil {
			return nil, fmt.Errorf("escrito source scan: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func (s *EscritoSourceStore) DeleteByEscrito(ctx context.Context, escritoID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM escrito_sources WHERE escrito_id = $1`, escritoID)
	if err != nil {
		return fmt.Errorf("escrito sources delete: %w", err)
	}
	return nil
}

// countWords counts whitespace-separated tokens in text.
func countWords(text string) int {
	if text == "" {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}
