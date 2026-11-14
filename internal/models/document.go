package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Document es la copia local de un PDF (en <datos>/documentos/<sha256>.pdf).
type Document struct {
	ID        uuid.UUID
	SHA256    string
	Path      string // relativa a la carpeta de datos
	SizeBytes int64
	Pages     int
	Scanned   bool
	SourceURL string
	CreatedAt time.Time
}

type DocumentStore struct {
	pool *pgxpool.Pool
}

func NewDocumentStore(pool *pgxpool.Pool) *DocumentStore {
	return &DocumentStore{pool: pool}
}

// Upsert guarda el documento por su huella; si ya existía devuelve el mismo id.
func (s *DocumentStore) Upsert(ctx context.Context, d *Document) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO documents (id, sha256, path, size_bytes, pages, scanned, source_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (sha256) DO UPDATE SET path = EXCLUDED.path, pages = EXCLUDED.pages, scanned = EXCLUDED.scanned
		RETURNING id, created_at
	`, d.ID, d.SHA256, d.Path, d.SizeBytes, d.Pages, d.Scanned, d.SourceURL).Scan(&d.ID, &d.CreatedAt)
	if err != nil {
		return fmt.Errorf("document upsert: %w", err)
	}
	return nil
}

// Get devuelve un documento por id.
func (s *DocumentStore) Get(ctx context.Context, id uuid.UUID) (*Document, error) {
	var d Document
	err := s.pool.QueryRow(ctx, `
		SELECT id, sha256, path, size_bytes, pages, scanned, source_url, created_at
		FROM documents WHERE id = $1`, id).
		Scan(&d.ID, &d.SHA256, &d.Path, &d.SizeBytes, &d.Pages, &d.Scanned, &d.SourceURL, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("document get: %w", err)
	}
	return &d, nil
}

// DocumentCounts: cuántos PDF hay guardados, legibles y escaneados.
func (s *DocumentStore) Counts(ctx context.Context) (total, scanned int, err error) {
	err = s.pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE scanned) FROM documents`).Scan(&total, &scanned)
	return total, scanned, err
}
