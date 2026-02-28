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

type ChatHandler struct {
	Sessions *models.ChatSessionStore
}

// ListSessions handles GET /api/chat/sessions.
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	sessions, err := h.Sessions.ListByUser(r.Context(), user.ID, 50)
	if err != nil {
		slog.Error("list chat sessions", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if sessions == nil {
		sessions = []models.ChatSession{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
	})
}

// GetSession handles GET /api/chat/sessions/{id}.
func (h *ChatHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session id"})
		return
	}

	session, err := h.Sessions.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	if session.UserID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	writeJSON(w, http.StatusOK, session)
}

type createSessionRequest struct {
	Title    string          `json:"title"`
	Messages json.RawMessage `json:"messages"`
}

// CreateSession handles POST /api/chat/sessions.
func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	session := &models.ChatSession{
		UserID:   user.ID,
		Title:    req.Title,
		Messages: req.Messages,
	}

	if err := h.Sessions.Create(r.Context(), session); err != nil {
		slog.Error("create chat session", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create session"})
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

type updateSessionRequest struct {
	Title    string          `json:"title"`
	Messages json.RawMessage `json:"messages"`
}

// UpdateSession handles PUT /api/chat/sessions/{id}.
func (h *ChatHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session id"})
		return
	}

	// Verify ownership.
	session, err := h.Sessions.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if session.UserID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	var req updateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.Sessions.Update(r.Context(), id, req.Title, req.Messages); err != nil {
		slog.Error("update chat session", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update session"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DeleteSession handles DELETE /api/chat/sessions/{id}.
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session id"})
		return
	}

	// Verify ownership.
	session, err := h.Sessions.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if session.UserID != user.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := h.Sessions.Delete(r.Context(), id); err != nil {
		slog.Error("delete chat session", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete session"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
