package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EntityRelationship represents a directed relationship between two entities.
type EntityRelationship struct {
	ID             uuid.UUID `json:"id"`
	SourceEntityID uuid.UUID `json:"source_entity_id"`
	TargetEntityID uuid.UUID `json:"target_entity_id"`
	RelationType   string    `json:"relation_type"`
	Strength       int       `json:"strength"`
	EvidenceCount  int       `json:"evidence_count"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// GraphNode represents an entity in the knowledge graph.
type GraphNode struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Type string    `json:"type"`
}

// GraphEdge represents a relationship edge in the knowledge graph.
type GraphEdge struct {
	Source       uuid.UUID `json:"source"`
	Target       uuid.UUID `json:"target"`
	RelationType string    `json:"relation_type"`
	Strength     int       `json:"strength"`
}

type EntityRelationshipStore struct {
	pool *pgxpool.Pool
}

func NewEntityRelationshipStore(pool *pgxpool.Pool) *EntityRelationshipStore {
	return &EntityRelationshipStore{pool: pool}
}

// Upsert inserts or increments the strength of an entity relationship.
func (s *EntityRelationshipStore) Upsert(ctx context.Context, sourceID, targetID uuid.UUID, relationType string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO entity_relationships (source_entity_id, target_entity_id, relation_type)
		VALUES ($1, $2, $3)
		ON CONFLICT (source_entity_id, target_entity_id, relation_type)
		DO UPDATE SET strength = entity_relationships.strength + 1,
		             evidence_count = entity_relationships.evidence_count + 1,
		             last_seen_at = NOW()
	`, sourceID, targetID, relationType)
	if err != nil {
		return fmt.Errorf("entity relationship upsert: %w", err)
	}
	return nil
}

// ListByEntity returns all relationships where the given entity is either
// source or target.
func (s *EntityRelationshipStore) ListByEntity(ctx context.Context, entityID uuid.UUID) ([]EntityRelationship, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, source_entity_id, target_entity_id, relation_type,
		       strength, evidence_count, last_seen_at, created_at
		FROM entity_relationships
		WHERE source_entity_id = $1 OR target_entity_id = $1
		ORDER BY strength DESC
	`, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity relationships list: %w", err)
	}
	defer rows.Close()

	var rels []EntityRelationship
	for rows.Next() {
		var r EntityRelationship
		if err := rows.Scan(
			&r.ID, &r.SourceEntityID, &r.TargetEntityID, &r.RelationType,
			&r.Strength, &r.EvidenceCount, &r.LastSeenAt, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("entity relationship scan: %w", err)
		}
		rels = append(rels, r)
	}
	return rels, rows.Err()
}

// GetGraph returns the knowledge graph: top entities and their relationships.
func (s *EntityRelationshipStore) GetGraph(ctx context.Context, limit int) ([]GraphNode, []GraphEdge, error) {
	if limit <= 0 {
		limit = 50
	}

	// Get top entities by relationship count
	nodeRows, err := s.pool.Query(ctx, `
		SELECT DISTINCT e.id, e.name, e.type
		FROM entities e
		JOIN (
			SELECT source_entity_id AS eid FROM entity_relationships
			UNION ALL
			SELECT target_entity_id FROM entity_relationships
		) r ON r.eid = e.id
		GROUP BY e.id, e.name, e.type
		ORDER BY COUNT(*) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("graph nodes: %w", err)
	}
	defer nodeRows.Close()

	var nodes []GraphNode
	nodeIDs := make(map[uuid.UUID]bool)
	for nodeRows.Next() {
		var n GraphNode
		if err := nodeRows.Scan(&n.ID, &n.Name, &n.Type); err != nil {
			return nil, nil, fmt.Errorf("graph node scan: %w", err)
		}
		nodes = append(nodes, n)
		nodeIDs[n.ID] = true
	}
	if err := nodeRows.Err(); err != nil {
		return nil, nil, err
	}

	if len(nodes) == 0 {
		return nodes, nil, nil
	}

	// Get edges between those nodes
	edgeRows, err := s.pool.Query(ctx, `
		SELECT source_entity_id, target_entity_id, relation_type, strength
		FROM entity_relationships
		ORDER BY strength DESC
		LIMIT $1
	`, limit*3)
	if err != nil {
		return nil, nil, fmt.Errorf("graph edges: %w", err)
	}
	defer edgeRows.Close()

	var edges []GraphEdge
	for edgeRows.Next() {
		var e GraphEdge
		if err := edgeRows.Scan(&e.Source, &e.Target, &e.RelationType, &e.Strength); err != nil {
			return nil, nil, fmt.Errorf("graph edge scan: %w", err)
		}
		// Only include edges where both nodes are in the result set
		if nodeIDs[e.Source] && nodeIDs[e.Target] {
			edges = append(edges, e)
		}
	}
	if err := edgeRows.Err(); err != nil {
		return nil, nil, err
	}

	return nodes, edges, nil
}

// GetEntityRelations returns relationships + entity details for a single entity.
func (s *EntityRelationshipStore) GetEntityRelations(ctx context.Context, entityID uuid.UUID) ([]EntityRelationship, []Entity, error) {
	rels, err := s.ListByEntity(ctx, entityID)
	if err != nil {
		return nil, nil, err
	}

	// Collect unique entity IDs from relationships
	entityIDs := make(map[uuid.UUID]bool)
	entityIDs[entityID] = true
	for _, r := range rels {
		entityIDs[r.SourceEntityID] = true
		entityIDs[r.TargetEntityID] = true
	}

	// Fetch entity details
	var entities []Entity
	for id := range entityIDs {
		var e Entity
		err := s.pool.QueryRow(ctx, `
			SELECT id, name, type, canonical, created_at
			FROM entities WHERE id = $1
		`, id).Scan(&e.ID, &e.Name, &e.Type, &e.Canonical, &e.CreatedAt)
		if err != nil {
			continue
		}
		entities = append(entities, e)
	}

	return rels, entities, nil
}
