package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	sessionName = "pomelo-session"
	userKey     = "user"
)

// SessionManager handles user sessions
type SessionManager struct {
	store sessions.Store
}

// NewSessionManager creates a new session manager
func NewSessionManager(secret string) *SessionManager {
	return &SessionManager{
		store: sessions.NewCookieStore([]byte(secret)),
	}
}

// GetSession retrieves the current session
func (sm *SessionManager) GetSession(r *http.Request) (*sessions.Session, error) {
	return sm.store.Get(r, sessionName)
}

// SaveUserToSession saves the user data to the session
func (sm *SessionManager) SaveUserToSession(w http.ResponseWriter, r *http.Request, user *GoogleUser) error {
	session, err := sm.GetSession(r)
	if err != nil {
		return err
	}

	session.Values[userKey] = user
	return session.Save(r, w)
}

// GetUserFromSession retrieves the user data from the session
func (sm *SessionManager) GetUserFromSession(r *http.Request) (*GoogleUser, error) {
	session, err := sm.GetSession(r)
	if err != nil {
		return nil, err
	}

	val, ok := session.Values[userKey]
	if !ok {
		return nil, nil
	}

	user, ok := val.(*GoogleUser)
	if !ok {
		return nil, nil
	}

	return user, nil
}

// ClearSession removes all session data
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) error {
	session, err := sm.GetSession(r)
	if err != nil {
		return err
	}

	session.Options.MaxAge = -1
	return session.Save(r, w)
}
