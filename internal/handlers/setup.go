package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/Saul-Punybz/folio/internal/models"
)

// MinPasswordLen es el largo mínimo de contraseña (runas) para crear usuarios.
const MinPasswordLen = 10

// SetupHandler atiende el primer arranque: crear el primer usuario admin.
type SetupHandler struct {
	Users *models.UserStore
}

// Status handles GET /api/setup → {"needs_setup": true|false}.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	n, err := h.Users.Count(r.Context())
	if err != nil {
		slog.Error("setup: count users", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

// Create handles POST /api/setup {email,password}. Crea el PRIMER usuario
// (admin) y solo mientras no exista ninguno; después responde 409.
func (h *SetupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	email := models.NormalizeEmail(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid email"})
		return
	}
	if len([]rune(req.Password)) < MinPasswordLen {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 10 characters"})
		return
	}

	// Respuesta rápida si ya hay usuarios (sin gastar un bcrypt).
	if n, err := h.Users.Count(r.Context()); err == nil && n > 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already set up"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("setup: hash", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	u, created, err := h.Users.CreateFirstAdmin(r.Context(), email, string(hash))
	if err != nil {
		slog.Error("setup: create first admin", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if !created {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already set up"})
		return
	}
	slog.Info("setup: first admin created", "email", u.Email)
	writeJSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}
