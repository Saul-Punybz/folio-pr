package models

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PageEntityStore manages the junction table between crawled pages and entities.
type PageEntityStore struct {
	pool *pgxpool.Pool
}

func NewPageEntityStore(pool *pgxpool.Pool) *PageEntityStore {
	return &PageEntityStore{pool: pool}
}

// Link creates an association between a crawled page and an entity. Ignores
// duplicates.
func (s *PageEntityStore) Link(ctx context.Context, pageID, entityID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO page_entities (page_id, entity_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, pageID, entityID)
	if err != nil {
		return fmt.Errorf("page entity link: %w", err)
	}
	return nil
}

// EntitiesForPage returns all entities linked to a crawled page.
func (s *PageEntityStore) EntitiesForPage(ctx context.Context, pageID uuid.UUID) ([]Entity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, e.canonical, e.created_at
		FROM entities e
		JOIN page_entities pe ON pe.entity_id = e.id
		WHERE pe.page_id = $1
		ORDER BY e.type, e.name
	`, pageID)
	if err != nil {
		return nil, fmt.Errorf("page entities list: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Canonical, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("page entity scan: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

// PagesForEntity returns crawled page IDs linked to a given entity.
func (s *PageEntityStore) PagesForEntity(ctx context.Context, entityID uuid.UUID, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT page_id FROM page_entities
		WHERE entity_id = $1
		LIMIT $2
	`, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("pages for entity: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("pages for entity scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
