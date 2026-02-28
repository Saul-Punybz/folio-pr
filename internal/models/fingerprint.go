package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Fingerprint tracks URL and content hashes for deduplication.
type Fingerprint struct {
	ID               uuid.UUID `json:"id"`
	CanonicalURLHash string    `json:"canonical_url_hash"`
	ContentHash      string    `json:"content_hash,omitempty"`
	Blocked          bool      `json:"blocked"`
	CreatedAt        time.Time `json:"created_at"`
}

// FingerprintStore provides data access methods for fingerprints.
type FingerprintStore struct {
	pool *pgxpool.Pool
}

// NewFingerprintStore creates a new FingerprintStore.
func NewFingerprintStore(pool *pgxpool.Pool) *FingerprintStore {
	return &FingerprintStore{pool: pool}
}

// Exists checks whether a fingerprint with the given URL hash already exists.
func (s *FingerprintStore) Exists(ctx context.Context, urlHash string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM fingerprints WHERE canonical_url_hash = $1)
	`, urlHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("fingerprint exists: %w", err)
	}
	return exists, nil
}

// ExistsOrBlocked checks whether a fingerprint exists and whether it is blocked.
// Returns (exists, blocked, error).
func (s *FingerprintStore) ExistsOrBlocked(ctx context.Context, urlHash string) (bool, bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx, `
		SELECT blocked FROM fingerprints WHERE canonical_url_hash = $1
	`, urlHash).Scan(&blocked)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, false, nil
		}
		return false, false, fmt.Errorf("fingerprint exists or blocked: %w", err)
	}
	return true, blocked, nil
}

// Create inserts a new fingerprint record.
func (s *FingerprintStore) Create(ctx context.Context, fp *Fingerprint) error {
	if fp.ID == uuid.Nil {
		fp.ID = uuid.New()
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO fingerprints (id, canonical_url_hash, content_hash, blocked)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, fp.ID, fp.CanonicalURLHash, fp.ContentHash, fp.Blocked).Scan(&fp.CreatedAt)
	if err != nil {
		return fmt.Errorf("fingerprint create: %w", err)
	}
	return nil
}

// Block marks a fingerprint as blocked so it will not be collected again.
func (s *FingerprintStore) Block(ctx context.Context, urlHash string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE fingerprints SET blocked = true WHERE canonical_url_hash = $1
	`, urlHash)
	if err != nil {
		return fmt.Errorf("fingerprint block: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("fingerprint not found: %s", urlHash)
	}
	return nil
}
