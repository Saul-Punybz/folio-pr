// Package middleware provides HTTP middleware for the Folio API.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// SessionAuth returns middleware that validates the session_token cookie,
// looks up the session and user in the database, and injects the user
// into the request context. Requests without a valid session receive 401.
func SessionAuth(sessions *models.SessionStore, users *models.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessions.GetByToken(r.Context(), cookie.Value)
			if err != nil {
				slog.Debug("session lookup failed", "err", err)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if session.ExpiresAt.Before(time.Now()) {
				// Session expired — clean it up.
				_ = sessions.Delete(r.Context(), session.ID)
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}

			user, err := users.GetByID(r.Context(), session.UserID)
			if err != nil {
				slog.Error("user lookup failed for valid session", "user_id", session.UserID, "err", err)
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin returns middleware that checks the user has the "admin" role.
// Must be placed after SessionAuth in the middleware chain.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Role != "admin" {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is set (i.e., the request is unauthenticated).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}
