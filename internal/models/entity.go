package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entity represents a named entity (person, organization, place) extracted from
// articles by the AI pipeline.
type Entity struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Canonical string    `json:"canonical"`
	CreatedAt time.Time `json:"created_at"`
}

// EntityCount holds an entity name, type, and its mention count for aggregation
// queries like TopEntities and CoOccurrences.
type EntityCount struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// EntityStore provides data access methods for the entities and article_entities
// tables.
type EntityStore struct {
	pool *pgxpool.Pool
}

// NewEntityStore creates a new EntityStore.
func NewEntityStore(pool *pgxpool.Pool) *EntityStore {
	return &EntityStore{pool: pool}
}

// Upsert inserts or updates an entity by its canonical name and type. The
// canonical form is the lowercased, trimmed version of the name. If an entity
// with the same canonical name and type already exists, the display name is
// updated. Returns the entity UUID.
func (s *EntityStore) Upsert(ctx context.Context, name, entityType string) (uuid.UUID, error) {
	canonical := strings.ToLower(strings.TrimSpace(name))
	if canonical == "" {
		return uuid.Nil, fmt.Errorf("entity upsert: empty name")
	}

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO entities (id, name, type, canonical)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (canonical, type) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, uuid.New(), strings.TrimSpace(name), entityType, canonical).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("entity upsert: %w", err)
	}
	return id, nil
}

// LinkToArticle creates an association between an article and an entity. If the
// link already exists, it is silently ignored.
func (s *EntityStore) LinkToArticle(ctx context.Context, articleID, entityID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO article_entities (article_id, entity_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, articleID, entityID)
	if err != nil {
		return fmt.Errorf("entity link to article: %w", err)
	}
	return nil
}

// TopEntities returns the most frequently mentioned entities of the given type
// within the last N days. Results are ordered by mention count descending.
func (s *EntityStore) TopEntities(ctx context.Context, entityType string, days, limit int) ([]EntityCount, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT e.name, e.type, COUNT(*) AS cnt
		FROM entities e
		JOIN article_entities ae ON ae.entity_id = e.id
		JOIN articles a ON a.id = ae.article_id
		WHERE e.type = $1
		  AND a.created_at >= now() - make_interval(days => $2)
		GROUP BY e.id, e.name, e.type
		ORDER BY cnt DESC
		LIMIT $3
	`, entityType, days, limit)
	if err != nil {
		return nil, fmt.Errorf("entity top: %w", err)
	}
	defer rows.Close()

	var results []EntityCount
	for rows.Next() {
		var ec EntityCount
		if err := rows.Scan(&ec.Name, &ec.Type, &ec.Count); err != nil {
			return nil, fmt.Errorf("entity top scan: %w", err)
		}
		results = append(results, ec)
	}
	return results, rows.Err()
}

// CoOccurrences returns entities that appear alongside the given entity in
// articles, ordered by how many articles they share. The given entity itself is
// excluded from the results.
func (s *EntityStore) CoOccurrences(ctx context.Context, entityID uuid.UUID, limit int) ([]EntityCount, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT e.name, e.type, COUNT(*) AS cnt
		FROM article_entities ae1
		JOIN article_entities ae2 ON ae2.article_id = ae1.article_id AND ae2.entity_id != ae1.entity_id
		JOIN entities e ON e.id = ae2.entity_id
		WHERE ae1.entity_id = $1
		GROUP BY e.id, e.name, e.type
		ORDER BY cnt DESC
		LIMIT $2
	`, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("entity co-occurrences: %w", err)
	}
	defer rows.Close()

	var results []EntityCount
	for rows.Next() {
		var ec EntityCount
		if err := rows.Scan(&ec.Name, &ec.Type, &ec.Count); err != nil {
			return nil, fmt.Errorf("entity co-occurrences scan: %w", err)
		}
		results = append(results, ec)
	}
	return results, rows.Err()
}

// EntitiesForArticle returns all entities linked to the given article.
func (s *EntityStore) EntitiesForArticle(ctx context.Context, articleID uuid.UUID) ([]Entity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, e.canonical, e.created_at
		FROM entities e
		JOIN article_entities ae ON ae.entity_id = e.id
		WHERE ae.article_id = $1
		ORDER BY e.type, e.name
	`, articleID)
	if err != nil {
		return nil, fmt.Errorf("entities for article: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Canonical, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("entities for article scan: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}
