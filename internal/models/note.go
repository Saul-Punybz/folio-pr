package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Note represents a user's annotation attached to an article.
type Note struct {
	ID        uuid.UUID `json:"id"`
	ArticleID uuid.UUID `json:"article_id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// NoteStore provides data access methods for notes.
type NoteStore struct {
	pool *pgxpool.Pool
}

// NewNoteStore creates a new NoteStore.
func NewNoteStore(pool *pgxpool.Pool) *NoteStore {
	return &NoteStore{pool: pool}
}

// ListByArticle returns all notes for a given article, newest first.
func (s *NoteStore) ListByArticle(ctx context.Context, articleID uuid.UUID) ([]Note, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, article_id, user_id, content, created_at
		FROM notes
		WHERE article_id = $1
		ORDER BY created_at DESC
	`, articleID)
	if err != nil {
		return nil, fmt.Errorf("note list: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ArticleID, &n.UserID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("note scan: %w", err)
		}
		notes = append(notes, n)
	}

	return notes, rows.Err()
}

// GetByID returns a single note by its UUID.
func (s *NoteStore) GetByID(ctx context.Context, id uuid.UUID) (*Note, error) {
	var n Note
	err := s.pool.QueryRow(ctx, `
		SELECT id, article_id, user_id, content, created_at
		FROM notes
		WHERE id = $1
	`, id).Scan(&n.ID, &n.ArticleID, &n.UserID, &n.Content, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("note get: %w", err)
	}
	return &n, nil
}

// Delete removes a note by its UUID.
func (s *NoteStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM notes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("note delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("note not found: %s", id)
	}
	return nil
}

// Create inserts a new note.
func (s *NoteStore) Create(ctx context.Context, note *Note) error {
	if note.ID == uuid.Nil {
		note.ID = uuid.New()
	}

	err := s.pool.QueryRow(ctx, `
		INSERT INTO notes (id, article_id, user_id, content)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, note.ID, note.ArticleID, note.UserID, note.Content).Scan(&note.CreatedAt)
	if err != nil {
		return fmt.Errorf("note create: %w", err)
	}
	return nil
}
