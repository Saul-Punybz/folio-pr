package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
)

const sessionDuration = 30 * 24 * time.Hour // 30 days

// AuthHandler groups authentication-related HTTP handlers.
type AuthHandler struct {
	Users    *models.UserStore
	Sessions *models.SessionStore
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/login.
// Accepts JSON {email, password}, validates credentials, creates a session,
// and sets an HttpOnly cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}

	user, err := h.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		slog.Debug("login: user not found", "email", req.Email)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		slog.Debug("login: bad password", "email", req.Email)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Generate a secure random session token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.Error("login: generate token", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	session := &models.Session{
		ID:        token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(sessionDuration),
	}

	if err := h.Sessions.Create(r.Context(), session); err != nil {
		slog.Error("login: create session", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// Logout handles POST /api/logout.
// Deletes the session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		if err := h.Sessions.Delete(r.Context(), cookie.Value); err != nil {
			slog.Error("logout: delete session", "err", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// Me handles GET /api/me.
// Returns the current authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}
