package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	oauthSessionName = "google_auth"
	stateKey         = "oauth_state"
	maxAge           = 300 // 5 minutes in seconds
)

// GoogleUser represents the user info received from Google
type GoogleUser struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

// GoogleAuth handles Google OAuth2 authentication
type GoogleAuth struct {
	config *oauth2.Config
	store  *sessions.CookieStore
}

// GetConfig returns the OAuth2 config
func (ga *GoogleAuth) GetConfig() *oauth2.Config {
	return ga.config
}

// generateState creates a random state token
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// NewGoogleAuth creates a new GoogleAuth instance
func NewGoogleAuth(sessionKey []byte) (*GoogleAuth, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set")
	}

	// Always use secure URL for OAuth
	secureURL := os.Getenv("SECURE_URL")
	if secureURL == "" {
		return nil, fmt.Errorf("SECURE_URL must be set")
	}

	// Use secure URL for OAuth callback
	redirectURL := secureURL + "/callback"

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	store := sessions.NewCookieStore(sessionKey)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   false, // Set to false for localhost
		SameSite: http.SameSiteLaxMode,
	}

	return &GoogleAuth{
		config: config,
		store:  store,
	}, nil
}

// LoginHandler initiates the Google OAuth flow
func (ga *GoogleAuth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		http.Error(w, "Failed to generate state token", http.StatusInternalServerError)
		return
	}

	// Store state in session
	session, err := ga.store.Get(r, oauthSessionName)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	session.Values[stateKey] = state
	session.Options.MaxAge = maxAge

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Debug print the URL
	url := ga.config.AuthCodeURL(state)
	fmt.Printf("Debug - Generated OAuth URL: %s\n", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// CallbackHandler handles the OAuth callback from Google
func (ga *GoogleAuth) CallbackHandler(w http.ResponseWriter, r *http.Request) (*GoogleUser, error) {
	// Verify state parameter
	session, err := ga.store.Get(r, oauthSessionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %v", err)
	}

	state := r.FormValue("state")
	if state == "" {
		return nil, fmt.Errorf("state parameter missing")
	}

	expectedState, ok := session.Values[stateKey].(string)
	if !ok || state != expectedState {
		return nil, fmt.Errorf("invalid state parameter")
	}

	// Clear the state from session immediately
	delete(session.Values, stateKey)
	if err := session.Save(r, w); err != nil {
		return nil, fmt.Errorf("failed to save session: %v", err)
	}

	code := r.FormValue("code")
	if code == "" {
		return nil, fmt.Errorf("code parameter missing")
	}

	token, err := ga.config.Exchange(r.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %v", err)
	}

	client := ga.config.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %v", err)
	}

	return &user, nil
}
