package models

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Opportunity represents a grant/RFP/funding opportunity extracted from a crawled page.
type Opportunity struct {
	ID            uuid.UUID       `json:"id"`
	PageID        *uuid.UUID      `json:"page_id,omitempty"`
	DomainID      uuid.UUID       `json:"domain_id"`
	Title         string          `json:"title"`
	Agency        string          `json:"agency"`
	Summary       string          `json:"summary"`
	Amount        string          `json:"amount"`
	AmountCents   int64           `json:"amount_cents"`
	Deadline      *time.Time      `json:"deadline,omitempty"`
	ContactName   string          `json:"contact_name"`
	ContactEmail  string          `json:"contact_email"`
	Category      string          `json:"category"`
	Status        string          `json:"status"`
	SourceURL     string          `json:"source_url"`
	Confidence    int             `json:"confidence"`
	RawExtraction json.RawMessage `json:"raw_extraction"`
	Reviewed      bool            `json:"reviewed"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type OpportunityStore struct {
	pool *pgxpool.Pool
}

func NewOpportunityStore(pool *pgxpool.Pool) *OpportunityStore {
	return &OpportunityStore{pool: pool}
}

func (s *OpportunityStore) Upsert(ctx context.Context, o *Opportunity) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	if o.RawExtraction == nil {
		o.RawExtraction = json.RawMessage("{}")
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO opportunities (id, page_id, domain_id, title, agency, summary, amount,
		    amount_cents, deadline, contact_name, contact_email, category, status,
		    source_url, confidence, raw_extraction, reviewed)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (page_id) DO UPDATE SET
		    title = EXCLUDED.title, agency = EXCLUDED.agency, summary = EXCLUDED.summary,
		    amount = EXCLUDED.amount, amount_cents = EXCLUDED.amount_cents,
		    deadline = EXCLUDED.deadline, contact_name = EXCLUDED.contact_name,
		    contact_email = EXCLUDED.contact_email, category = EXCLUDED.category,
		    status = EXCLUDED.status, source_url = EXCLUDED.source_url,
		    confidence = EXCLUDED.confidence, raw_extraction = EXCLUDED.raw_extraction,
		    updated_at = NOW()
	`, o.ID, o.PageID, o.DomainID, o.Title, o.Agency, o.Summary, o.Amount,
		o.AmountCents, o.Deadline, o.ContactName, o.ContactEmail, o.Category, o.Status,
		o.SourceURL, o.Confidence, o.RawExtraction, o.Reviewed)
	if err != nil {
		return fmt.Errorf("opportunity upsert: %w", err)
	}
	return nil
}

func (s *OpportunityStore) GetByID(ctx context.Context, id uuid.UUID) (*Opportunity, error) {
	var o Opportunity
	err := s.pool.QueryRow(ctx, `
		SELECT id, page_id, domain_id, title, agency, summary, amount, amount_cents,
		       deadline, contact_name, contact_email, category, status, source_url,
		       confidence, raw_extraction, reviewed, created_at, updated_at
		FROM opportunities WHERE id = $1
	`, id).Scan(
		&o.ID, &o.PageID, &o.DomainID, &o.Title, &o.Agency, &o.Summary, &o.Amount,
		&o.AmountCents, &o.Deadline, &o.ContactName, &o.ContactEmail, &o.Category,
		&o.Status, &o.SourceURL, &o.Confidence, &o.RawExtraction, &o.Reviewed,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("opportunity get: %w", err)
	}
	return &o, nil
}

func (s *OpportunityStore) ListAll(ctx context.Context, status, category string, domainID *uuid.UUID, sort string, limit, offset int) ([]Opportunity, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, page_id, domain_id, title, agency, summary, amount, amount_cents,
	                 deadline, contact_name, contact_email, category, status, source_url,
	                 confidence, raw_extraction, reviewed, created_at, updated_at
	          FROM opportunities WHERE 1=1`
	args := []any{}
	argIdx := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}
	if domainID != nil {
		query += fmt.Sprintf(" AND domain_id = $%d", argIdx)
		args = append(args, *domainID)
		argIdx++
	}

	switch sort {
	case "deadline":
		query += " ORDER BY deadline ASC NULLS LAST"
	case "amount":
		query += " ORDER BY amount_cents DESC"
	case "confidence":
		query += " ORDER BY confidence DESC"
	default:
		query += " ORDER BY created_at DESC"
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("opportunities list: %w", err)
	}
	defer rows.Close()
	return scanOpportunityRows(rows)
}

func (s *OpportunityStore) SearchFTS(ctx context.Context, query string, limit int) ([]Opportunity, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, page_id, domain_id, title, agency, summary, amount, amount_cents,
		       deadline, contact_name, contact_email, category, status, source_url,
		       confidence, raw_extraction, reviewed, created_at, updated_at
		FROM opportunities
		WHERE to_tsvector('simple', title || ' ' || agency || ' ' || summary) @@ plainto_tsquery('simple', $1)
		ORDER BY confidence DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("opportunities search: %w", err)
	}
	defer rows.Close()
	return scanOpportunityRows(rows)
}

func (s *OpportunityStore) TotalCount(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM opportunities`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("opportunities count: %w", err)
	}
	return count, nil
}

func (s *OpportunityStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM opportunities GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("opportunities count by status: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

func (s *OpportunityStore) Update(ctx context.Context, o *Opportunity) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE opportunities SET
		    title = $2, agency = $3, summary = $4, amount = $5, amount_cents = $6,
		    deadline = $7, contact_name = $8, contact_email = $9, category = $10,
		    status = $11, confidence = $12, reviewed = $13, updated_at = NOW()
		WHERE id = $1
	`, o.ID, o.Title, o.Agency, o.Summary, o.Amount, o.AmountCents,
		o.Deadline, o.ContactName, o.ContactEmail, o.Category,
		o.Status, o.Confidence, o.Reviewed)
	if err != nil {
		return fmt.Errorf("opportunity update: %w", err)
	}
	return nil
}

func (s *OpportunityStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM opportunities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("opportunity delete: %w", err)
	}
	return nil
}

func (s *OpportunityStore) MarkReviewed(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE opportunities SET reviewed = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("opportunity mark reviewed: %w", err)
	}
	return nil
}

// RefreshStatuses updates opportunity statuses based on deadlines.
// open → closing_soon if deadline within 7 days, open/closing_soon → closed if past deadline.
func (s *OpportunityStore) RefreshStatuses(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE opportunities SET
		    status = CASE
		        WHEN deadline IS NOT NULL AND deadline < CURRENT_DATE THEN 'closed'
		        WHEN deadline IS NOT NULL AND deadline <= CURRENT_DATE + INTERVAL '7 days' THEN 'closing_soon'
		        ELSE status
		    END,
		    updated_at = NOW()
		WHERE status IN ('open','closing_soon','unknown')
		  AND deadline IS NOT NULL
		  AND (
		      (deadline < CURRENT_DATE AND status != 'closed')
		      OR (deadline <= CURRENT_DATE + INTERVAL '7 days' AND status = 'open')
		  )
	`)
	if err != nil {
		return 0, fmt.Errorf("opportunities refresh statuses: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func scanOpportunityRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]Opportunity, error) {
	var opps []Opportunity
	for rows.Next() {
		var o Opportunity
		if err := rows.Scan(
			&o.ID, &o.PageID, &o.DomainID, &o.Title, &o.Agency, &o.Summary, &o.Amount,
			&o.AmountCents, &o.Deadline, &o.ContactName, &o.ContactEmail, &o.Category,
			&o.Status, &o.SourceURL, &o.Confidence, &o.RawExtraction, &o.Reviewed,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("opportunity scan: %w", err)
		}
		opps = append(opps, o)
	}
	return opps, rows.Err()
}
