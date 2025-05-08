package auth

import (
	"net/http"
)

// AuthMiddleware wraps handlers requiring authentication
type AuthMiddleware struct {
	sessions *SessionManager
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(sessions *SessionManager) *AuthMiddleware {
	return &AuthMiddleware{
		sessions: sessions,
	}
}

// RequireAuth is a middleware that ensures a user is authenticated
func (am *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := am.sessions.GetUserFromSession(r)
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		if user == nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		next(w, r)
	}
}
