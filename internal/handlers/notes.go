package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
)

// NotesHandler groups note-related HTTP handlers.
type NotesHandler struct {
	Notes    *models.NoteStore
	Articles *models.ArticleStore
}

// ListNotes handles GET /api/items/{id}/notes.
// Returns all notes for an article, newest first.
func (h *NotesHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	notes, err := h.Notes.ListByArticle(r.Context(), articleID)
	if err != nil {
		slog.Error("list notes", "article_id", articleID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if notes == nil {
		notes = []models.Note{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notes": notes,
		"count": len(notes),
	})
}

type createNoteRequest struct {
	Content string `json:"content"`
}

// CreateNote handles POST /api/items/{id}/notes.
// Body: { "content": "note text" }
func (h *NotesHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	articleID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}

	// Verify the article exists.
	if _, err := h.Articles.GetByID(r.Context(), articleID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}

	note := &models.Note{
		ArticleID: articleID,
		UserID:    user.ID,
		Content:   req.Content,
	}

	if err := h.Notes.Create(r.Context(), note); err != nil {
		slog.Error("create note", "article_id", articleID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create note"})
		return
	}

	writeJSON(w, http.StatusCreated, note)
}

// DeleteNote handles DELETE /api/notes/{noteId}.
// Only the note author or an admin can delete.
func (h *NotesHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	noteID, err := uuid.Parse(chi.URLParam(r, "noteId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note id"})
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get the note to check ownership.
	note, err := h.Notes.GetByID(r.Context(), noteID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "note not found"})
		return
	}

	// Only the author or an admin can delete.
	if note.UserID != user.ID && user.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := h.Notes.Delete(r.Context(), noteID); err != nil {
		slog.Error("delete note", "note_id", noteID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete note"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
