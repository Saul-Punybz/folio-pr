package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// PDFKnown es lo que se sabe de un PDF ya bajado desde una URL: sirve para no
// volver a bajarlo si el servidor dice que no cambió.
type PDFKnown struct {
	DocumentID   uuid.UUID
	Path         string // vacía si el archivo ya se borró (el texto sigue)
	SizeBytes    int64
	ETag         string
	LastModified string
}

// KnownPDF busca el PDF guardado para una URL (por su huella). (nil, nil) si
// no se ha bajado nunca.
func (s *DocumentStore) KnownPDF(ctx context.Context, urlHash string) (*PDFKnown, error) {
	var k PDFKnown
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, d.path, d.size_bytes, cp.http_etag, cp.http_last_modified
		FROM crawled_pages cp JOIN documents d ON d.id = cp.document_id
		WHERE cp.url_hash = $1`, urlHash).
		Scan(&k.DocumentID, &k.Path, &k.SizeBytes, &k.ETag, &k.LastModified)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("document known pdf: %w", err)
	}
	return &k, nil
}

// SetValidators guarda el ETag y el Last-Modified con que se bajó el PDF.
func (s *DocumentStore) SetValidators(ctx context.Context, urlHash, etag, lastModified string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE crawled_pages SET http_etag = $2, http_last_modified = $3 WHERE url_hash = $1`,
		urlHash, etag, lastModified)
	return err
}

// DocState: un documento con lo que la política de retención necesita saber.
type DocState struct {
	ID        uuid.UUID
	Path      string // "" = el archivo ya no está
	SizeBytes int64
	CreatedAt time.Time
	// HasActive: algún fondo (por revisar o confirmado) lo usa y no está
	// cerrado hace más de ClosedGraceDays.
	HasActive bool
	// Pending: alguna página/PDF suya aún no pasó por la extracción.
	Pending bool
}

// ClosedGraceDays: un fondo cerrado conserva su PDF este tiempo.
const ClosedGraceDays = 90

// ListDocStates devuelve el estado de retención de los documentos con archivo.
func (s *DocumentStore) ListDocStates(ctx context.Context, now time.Time) ([]DocState, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id, d.path, d.size_bytes, d.created_at,
		  EXISTS (SELECT 1 FROM opportunities o WHERE o.document_id = d.id
		          AND NOT (o.status = 'closed' AND o.deadline IS NOT NULL
		                   AND o.deadline < ($1::timestamptz AT TIME ZONE 'America/Puerto_Rico')::date - $2::int)) AS has_active,
		  EXISTS (SELECT 1 FROM crawled_pages cp WHERE cp.document_id = d.id
		          AND cp.opportunity_checked = false) AS pending
		FROM documents d WHERE d.path <> ''`, now, ClosedGraceDays)
	if err != nil {
		return nil, fmt.Errorf("document states: %w", err)
	}
	defer rows.Close()
	var out []DocState
	for rows.Next() {
		var d DocState
		if err := rows.Scan(&d.ID, &d.Path, &d.SizeBytes, &d.CreatedAt, &d.HasActive, &d.Pending); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ReleaseFile deja el documento sin archivo (su fila y su texto extraído se
// quedan). Los fondos que lo apuntaban pasan a usar el enlace remoto.
func (s *DocumentStore) ReleaseFile(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `
		UPDATE opportunities o SET
		    document_remote_url = CASE WHEN o.document_remote_url = ''
		        THEN (SELECT source_url FROM documents WHERE id = $1) ELSE o.document_remote_url END,
		    document_id = NULL
		WHERE o.document_id = $1`, id); err != nil {
		return fmt.Errorf("document release: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE documents SET path = '' WHERE id = $1`, id); err != nil {
		return fmt.Errorf("document release: %w", err)
	}
	return tx.Commit(ctx)
}

// ReferencedPaths: las rutas (relativas) de los documentos con archivo.
func (s *DocumentStore) ReferencedPaths(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT path FROM documents WHERE path <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = true
	}
	return out, rows.Err()
}
