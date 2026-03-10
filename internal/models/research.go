package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ResearchProject represents a deep research investigation.
type ResearchProject struct {
	ID         uuid.UUID       `json:"id"`
	UserID     uuid.UUID       `json:"user_id"`
	Topic      string          `json:"topic"`
	Keywords   []string        `json:"keywords"`
	Status     string          `json:"status"`
	Phase      int             `json:"phase"`
	Progress   json.RawMessage `json:"progress"`
	Dossier    string          `json:"dossier"`
	Entities   json.RawMessage `json:"entities"`
	Timeline   json.RawMessage `json:"timeline"`
	ErrorMsg   string          `json:"error_msg"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ResearchFinding represents a single search result or scraped page.
type ResearchFinding struct {
	ID          uuid.UUID       `json:"id"`
	ProjectID   uuid.UUID       `json:"project_id"`
	URL         string          `json:"url"`
	URLHash     string          `json:"url_hash"`
	Title       string          `json:"title"`
	Snippet     string          `json:"snippet"`
	CleanText   string          `json:"clean_text,omitempty"`
	SourceType  string          `json:"source_type"`
	Sentiment   string          `json:"sentiment"`
	Relevance   float32         `json:"relevance"`
	ImageURL    *string         `json:"image_url,omitempty"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
	Scraped     bool            `json:"scraped"`
	Tags        json.RawMessage `json:"tags"`
	Entities    json.RawMessage `json:"entities"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ── ResearchProjectStore ─────────────────────────────────────────

type ResearchProjectStore struct {
	pool *pgxpool.Pool
}

func NewResearchProjectStore(pool *pgxpool.Pool) *ResearchProjectStore {
	return &ResearchProjectStore{pool: pool}
}

func (s *ResearchProjectStore) Create(ctx context.Context, p *ResearchProject) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	kwJSON, err := json.Marshal(p.Keywords)
	if err != nil {
		return fmt.Errorf("research project create: marshal keywords: %w", err)
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO research_projects (id, user_id, topic, keywords)
		VALUES ($1, $2, $3, $4)
		RETURNING status, phase, progress, dossier, entities, timeline, error_msg,
		          started_at, finished_at, created_at, updated_at
	`, p.ID, p.UserID, p.Topic, kwJSON).Scan(
		&p.Status, &p.Phase, &p.Progress, &p.Dossier, &p.Entities, &p.Timeline,
		&p.ErrorMsg, &p.StartedAt, &p.FinishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("research project create: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) GetByID(ctx context.Context, id uuid.UUID) (*ResearchProject, error) {
	var p ResearchProject
	var kwRaw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, topic, keywords, status, phase, progress, dossier,
		       entities, timeline, error_msg, started_at, finished_at, created_at, updated_at
		FROM research_projects
		WHERE id = $1
	`, id).Scan(
		&p.ID, &p.UserID, &p.Topic, &kwRaw, &p.Status, &p.Phase, &p.Progress,
		&p.Dossier, &p.Entities, &p.Timeline, &p.ErrorMsg,
		&p.StartedAt, &p.FinishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("research project get: %w", err)
	}
	p.Keywords = scanJSONStringSlice(kwRaw)
	return &p, nil
}

func (s *ResearchProjectStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]ResearchProject, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, topic, keywords, status, phase, progress, dossier,
		       entities, timeline, error_msg, started_at, finished_at, created_at, updated_at
		FROM research_projects
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("research projects list: %w", err)
	}
	defer rows.Close()

	var projects []ResearchProject
	for rows.Next() {
		var p ResearchProject
		var kwRaw []byte
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Topic, &kwRaw, &p.Status, &p.Phase, &p.Progress,
			&p.Dossier, &p.Entities, &p.Timeline, &p.ErrorMsg,
			&p.StartedAt, &p.FinishedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("research project scan: %w", err)
		}
		p.Keywords = scanJSONStringSlice(kwRaw)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *ResearchProjectStore) ListQueued(ctx context.Context) ([]ResearchProject, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, topic, keywords, status, phase, progress, dossier,
		       entities, timeline, error_msg, started_at, finished_at, created_at, updated_at
		FROM research_projects
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("research projects list queued: %w", err)
	}
	defer rows.Close()

	var projects []ResearchProject
	for rows.Next() {
		var p ResearchProject
		var kwRaw []byte
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Topic, &kwRaw, &p.Status, &p.Phase, &p.Progress,
			&p.Dossier, &p.Entities, &p.Timeline, &p.ErrorMsg,
			&p.StartedAt, &p.FinishedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("research project scan: %w", err)
		}
		p.Keywords = scanJSONStringSlice(kwRaw)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *ResearchProjectStore) UpdateStatus(ctx context.Context, id uuid.UUID, status string, phase int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET status = $2, phase = $3, updated_at = NOW()
		WHERE id = $1
	`, id, status, phase)
	if err != nil {
		return fmt.Errorf("research project update status: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) UpdateProgress(ctx context.Context, id uuid.UUID, progress json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET progress = $2, updated_at = NOW()
		WHERE id = $1
	`, id, progress)
	if err != nil {
		return fmt.Errorf("research project update progress: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) SetStarted(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET started_at = NOW(), status = 'searching', phase = 1, updated_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("research project set started: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) SetFinished(ctx context.Context, id uuid.UUID, dossier string, entities, timeline json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET status = 'done', phase = 3, dossier = $2, entities = $3, timeline = $4,
		    finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, dossier, entities, timeline)
	if err != nil {
		return fmt.Errorf("research project set finished: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) SetFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET status = 'failed', error_msg = $2, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("research project set failed: %w", err)
	}
	return nil
}

func (s *ResearchProjectStore) Cancel(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE research_projects
		SET status = 'cancelled', finished_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status NOT IN ('done', 'failed', 'cancelled')
	`, id)
	if err != nil {
		return fmt.Errorf("research project cancel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("research project not found or already finished: %s", id)
	}
	return nil
}

func (s *ResearchProjectStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM research_projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("research project delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("research project not found: %s", id)
	}
	return nil
}

// ── ResearchFindingStore ─────────────────────────────────────────

type ResearchFindingStore struct {
	pool *pgxpool.Pool
}

func NewResearchFindingStore(pool *pgxpool.Pool) *ResearchFindingStore {
	return &ResearchFindingStore{pool: pool}
}

func (s *ResearchFindingStore) Create(ctx context.Context, f *ResearchFinding) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.Tags == nil {
		f.Tags = json.RawMessage("[]")
	}
	if f.Entities == nil {
		f.Entities = json.RawMessage("{}")
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO research_findings (id, project_id, url, url_hash, title, snippet, clean_text,
		                               source_type, sentiment, relevance, image_url, published_at, scraped, tags, entities)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (project_id, url_hash) DO NOTHING
		RETURNING created_at
	`, f.ID, f.ProjectID, f.URL, f.URLHash, f.Title, f.Snippet, f.CleanText,
		f.SourceType, f.Sentiment, f.Relevance, f.ImageURL, f.PublishedAt, f.Scraped, f.Tags, f.Entities,
	).Scan(&f.CreatedAt)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows — duplicate
		f.ID = uuid.Nil
		return nil
	}
	return nil
}

func (s *ResearchFindingStore) ListByProject(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]ResearchFinding, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, url, url_hash, title, snippet, clean_text,
		       source_type, sentiment, relevance, image_url, published_at, scraped, tags, entities, created_at
		FROM research_findings
		WHERE project_id = $1
		ORDER BY relevance DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("research findings list: %w", err)
	}
	defer rows.Close()
	return scanFindingRows(rows)
}

func (s *ResearchFindingStore) ListByProjectAndSource(ctx context.Context, projectID uuid.UUID, sourceType string, limit, offset int) ([]ResearchFinding, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, url, url_hash, title, snippet, clean_text,
		       source_type, sentiment, relevance, image_url, published_at, scraped, tags, entities, created_at
		FROM research_findings
		WHERE project_id = $1 AND source_type = $2
		ORDER BY relevance DESC, created_at DESC
		LIMIT $3 OFFSET $4
	`, projectID, sourceType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("research findings list by source: %w", err)
	}
	defer rows.Close()
	return scanFindingRows(rows)
}

func (s *ResearchFindingStore) CountByProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM research_findings WHERE project_id = $1
	`, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("research findings count: %w", err)
	}
	return count, nil
}

func (s *ResearchFindingStore) ListTopUnscraped(ctx context.Context, projectID uuid.UUID, limit int) ([]ResearchFinding, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, url, url_hash, title, snippet, clean_text,
		       source_type, sentiment, relevance, image_url, published_at, scraped, tags, entities, created_at
		FROM research_findings
		WHERE project_id = $1 AND scraped = false
		ORDER BY created_at ASC
		LIMIT $2
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("research findings list unscraped: %w", err)
	}
	defer rows.Close()
	return scanFindingRows(rows)
}

func (s *ResearchFindingStore) UpdateScraped(ctx context.Context, id uuid.UUID, cleanText, sentiment string, tags json.RawMessage, entities json.RawMessage) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_findings
		SET clean_text = $2, scraped = true, sentiment = $3, tags = $4, entities = $5
		WHERE id = $1
	`, id, cleanText, sentiment, tags, entities)
	if err != nil {
		return fmt.Errorf("research finding update scraped: %w", err)
	}
	return nil
}

func (s *ResearchFindingStore) UpdateRelevance(ctx context.Context, id uuid.UUID, relevance float32) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE research_findings SET relevance = $2 WHERE id = $1
	`, id, relevance)
	if err != nil {
		return fmt.Errorf("research finding update relevance: %w", err)
	}
	return nil
}

func (s *ResearchFindingStore) ListAllByProject(ctx context.Context, projectID uuid.UUID) ([]ResearchFinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, url, url_hash, title, snippet, clean_text,
		       source_type, sentiment, relevance, image_url, published_at, scraped, tags, entities, created_at
		FROM research_findings
		WHERE project_id = $1
		ORDER BY relevance DESC, created_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("research findings list all: %w", err)
	}
	defer rows.Close()
	return scanFindingRows(rows)
}

func scanFindingRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]ResearchFinding, error) {
	var findings []ResearchFinding
	for rows.Next() {
		var f ResearchFinding
		if err := rows.Scan(
			&f.ID, &f.ProjectID, &f.URL, &f.URLHash, &f.Title, &f.Snippet, &f.CleanText,
			&f.SourceType, &f.Sentiment, &f.Relevance, &f.ImageURL, &f.PublishedAt,
			&f.Scraped, &f.Tags, &f.Entities, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("research finding scan: %w", err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}
