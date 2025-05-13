package main

import (
	"crypto/ecdsa"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/gob"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/0xjmp/pomelo-interview/auth"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
)

type Entry struct {
	ID        int
	Text      string
	Timestamp time.Time
	IsJWT     bool
}

type PageData struct {
	Entries     []Entry
	Mode        string
	BadgeClass  string
	User        *auth.GoogleUser
	CSRFToken   string
	InsecureURL string
	SecureURL   string
}

var (
	db         *sql.DB
	templates  *template.Template
	store      *sessions.CookieStore
	oauth      *oauth2.Config
	googleAuth *auth.GoogleAuth
	jwtPubKey  *ecdsa.PublicKey
)

var oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
var oidNamedCurveSecp256k1 = asn1.ObjectIdentifier{1, 3, 132, 0, 10}

type publicKeyInfo struct {
	Raw       asn1.RawContent
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

type ecdsaPublicKey struct {
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}

func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return value
}

func initDB() {
	host := requireEnv("POSTGRES_HOST")
	port := requireEnv("POSTGRES_PORT")
	user := requireEnv("POSTGRES_USER")
	password := requireEnv("POSTGRES_PASSWORD")
	dbname := requireEnv("POSTGRES_DB")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to create database connection:", err)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Printf("Successfully connected to PostgreSQL at %s:%s", host, port)

	// Create table if it doesn't exist with is_jwt column
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS entries (
			id SERIAL PRIMARY KEY,
			text TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			is_jwt BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		log.Fatal("Failed to create entries table:", err)
	}

	// Add is_jwt column if it doesn't exist
	_, err = db.Exec(`
		DO $$ 
		BEGIN 
			IF NOT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_name='entries' AND column_name='is_jwt'
			) THEN 
				ALTER TABLE entries ADD COLUMN is_jwt BOOLEAN NOT NULL DEFAULT FALSE;
			END IF;
		END $$;
	`)
	if err != nil {
		log.Fatal("Failed to add is_jwt column:", err)
	}
}

func initAuth() {
	// Register types for session storage
	gob.Register(&auth.GoogleUser{})

	// Initialize sessions
	store = sessions.NewCookieStore([]byte(requireEnv("SESSION_SECRET")))

	// Initialize Google OAuth
	var err error
	googleAuth, err = auth.NewGoogleAuth([]byte(requireEnv("SESSION_SECRET")))
	if err != nil {
		log.Fatal("Failed to initialize Google Auth:", err)
	}

	// Initialize OAuth config for backward compatibility
	oauth = googleAuth.GetConfig()
}

func initJWT() {
	// Read the public key file
	pubKeyBytes, err := ioutil.ReadFile("puneet.pub")
	if err != nil {
		log.Fatal("Failed to read public key file:", err)
	}

	// Parse the PEM block
	block, _ := pem.Decode(pubKeyBytes)
	if block == nil {
		log.Fatal("Failed to parse PEM block")
	}

	// Parse the ASN.1 structure
	var pki publicKeyInfo
	if _, err := asn1.Unmarshal(block.Bytes, &pki); err != nil {
		log.Fatal("Failed to parse ASN.1 structure:", err)
	}

	// Verify it's an ECDSA key with secp256k1 curve
	if !pki.Algorithm.Algorithm.Equal(oidPublicKeyECDSA) {
		log.Fatal("Not an ECDSA public key")
	}

	var curve asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(pki.Algorithm.Parameters.FullBytes, &curve); err != nil {
		log.Fatal("Failed to parse curve identifier:", err)
	}

	if !curve.Equal(oidNamedCurveSecp256k1) {
		log.Fatal("Not a secp256k1 curve")
	}

	// Parse the public key using secp256k1
	pubKey, err := secp256k1.ParsePubKey(pki.PublicKey.Bytes)
	if err != nil {
		log.Fatal("Failed to parse secp256k1 public key:", err)
	}

	// Convert to standard ECDSA public key while preserving the secp256k1 curve
	jwtPubKey = &ecdsa.PublicKey{
		Curve: secp256k1.S256(),
		X:     pubKey.X(),
		Y:     pubKey.Y(),
	}
}

func getUser(r *http.Request) *auth.GoogleUser {
	session, _ := store.Get(r, "session")
	val := session.Values["user"]
	user, ok := val.(*auth.GoogleUser)
	if !ok {
		return nil
	}
	return user
}

// sanitizeInputAtFirstQuote returns the substring of s up to (but not including) the first single or double quote.
// If neither is present, it returns s unchanged.
func sanitizeInputAtFirstQuote(s string) string {
	if idx := strings.IndexAny(s, "'\""); idx != -1 {
		return s[:idx]
	}
	return s
}

func handleInsecure(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		text := r.FormValue("text")
		if text != "" {
			text = sanitizeInputAtFirstQuote(text)
			if text == "" {
				http.Error(w, "Input cannot be only a quote character.", http.StatusBadRequest)
				return
			}
			// WARNING: This code is deliberately vulnerable to SQL injection!
			query := fmt.Sprintf("INSERT INTO entries (text) VALUES ('%s')", text)
			_, err := db.Exec(query)
			if err != nil {
				log.Printf("Error inserting entry: %v", err)
				http.Error(w, "Error saving entry", http.StatusInternalServerError)
				return
			}
		}
	}

	// Fetch entries from database, excluding JWT entries
	rows, err := db.Query("SELECT id, text, timestamp, is_jwt FROM entries WHERE is_jwt = false ORDER BY timestamp DESC")
	if err != nil {
		log.Printf("Error querying entries: %v", err)
		http.Error(w, "Error fetching entries", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Text, &entry.Timestamp, &entry.IsJWT)
		if err != nil {
			log.Printf("Error scanning entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	data := PageData{
		Entries:     entries,
		Mode:        "Insecure",
		BadgeClass:  "insecure",
		InsecureURL: os.Getenv("INSECURE_URL"),
		SecureURL:   os.Getenv("SECURE_URL"),
	}
	// Optionally, you could add errorMsg to PageData and display it in the template

	templates.ExecuteTemplate(w, "base.html", data)
}

func handleSecure(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	var errorMsg string

	if r.Method == http.MethodPost {
		if user == nil {
			http.Error(w, "You must be logged in to post notes", http.StatusUnauthorized)
			return
		}
		// CSRF token is automatically verified by the middleware
		text := r.FormValue("text")
		if text != "" {
			text = sanitizeInputAtFirstQuote(text)
			if text == "" {
				errorMsg = "Input cannot be only a quote character."
			} else {
				isJWT := false
				// Check if the text looks like a JWT token
				if isJWTFormat(text) {
					// Try to verify the JWT
					token, err := jwt.Parse(text, func(token *jwt.Token) (interface{}, error) {
						// Check if the signing method matches our key
						if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
							return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
						}
						return jwtPubKey, nil
					})

					if err != nil {
						log.Printf("JWT verification failed: %v", err)
					} else if token.Valid {
						isJWT = true
					}
				}

				// Use parameterized query for security
				_, err := db.Exec("INSERT INTO entries (text, is_jwt) VALUES ($1, $2)", text, isJWT)
				if err != nil {
					log.Printf("Error inserting entry: %v", err)
					http.Error(w, "Error saving entry", http.StatusInternalServerError)
					return
				}
			}
		}
	}

	// Fetch entries using parameterized query
	rows, err := db.Query("SELECT id, text, timestamp, is_jwt FROM entries ORDER BY timestamp DESC")
	if err != nil {
		log.Printf("Error querying entries: %v", err)
		http.Error(w, "Error fetching entries", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Text, &entry.Timestamp, &entry.IsJWT)
		if err != nil {
			log.Printf("Error scanning entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	showEmptyState := false
	emptyStateMsg := ""
	if user == nil {
		showEmptyState = true
		emptyStateMsg = "You must be signed in with Google to post notes. Please log in above."
	}

	data := PageData{
		Entries:     entries,
		Mode:        "Secure",
		BadgeClass:  "secure",
		User:        user,
		CSRFToken:   csrf.Token(r),
		InsecureURL: os.Getenv("INSECURE_URL"),
		SecureURL:   os.Getenv("SECURE_URL"),
	}
	// Add new fields for empty state and error message
	type extendedPageData struct {
		PageData
		ShowEmptyState bool
		EmptyStateMsg  string
		ErrorMsg       string
	}
	templates.ExecuteTemplate(w, "base.html", extendedPageData{
		PageData:       data,
		ShowEmptyState: showEmptyState,
		EmptyStateMsg:  emptyStateMsg,
		ErrorMsg:       errorMsg,
	})
}

// isJWTFormat checks if the text looks like a JWT token
func isJWTFormat(text string) bool {
	return len(text) > 0 && text[0] == 'e' && len(strings.Split(text, ".")) == 3
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	googleAuth.LoginHandler(w, r)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	user, err := googleAuth.CallbackHandler(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save user to session
	session, _ := store.Get(r, "session")
	session.Values["user"] = user
	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "session")
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		http.Error(w, "Failed to get session", http.StatusInternalServerError)
		return
	}

	// Get user before clearing session
	if user, ok := session.Values["user"].(*auth.GoogleUser); ok && user != nil {
		// Attempt to revoke Google OAuth token
		client := oauth.Client(r.Context(), nil)
		resp, err := client.Get("https://accounts.google.com/o/oauth2/revoke")
		if err != nil {
			log.Printf("Failed to revoke OAuth token: %v", err)
			// Continue with logout even if token revocation fails
		} else {
			resp.Body.Close()
		}
	}

	// Destroy the session
	session.Options.MaxAge = -1
	session.Values = make(map[interface{}]interface{})

	if err := session.Save(r, w); err != nil {
		log.Printf("Failed to save session: %v", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	// Delete the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	err := db.Ping()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Database connection failed"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func main() {
	initDB()
	initAuth()
	initJWT()
	defer db.Close()

	// Parse template
	templates = template.Must(template.ParseFiles("base.html"))

	// Create separate mux for each server
	insecureMux := http.NewServeMux()
	secureMux := http.NewServeMux()

	// Set up routes
	insecureMux.HandleFunc("/", handleInsecure)
	insecureMux.HandleFunc("/health", handleHealthCheck)

	csrfMiddleware := csrf.Protect([]byte(requireEnv("CSRF_KEY")))

	// Wrap secure routes with CSRF protection
	secureMux.Handle("/", csrfMiddleware(http.HandlerFunc(handleSecure)))
	secureMux.Handle("/login", csrfMiddleware(http.HandlerFunc(handleLogin)))
	secureMux.Handle("/callback", csrfMiddleware(http.HandlerFunc(handleCallback)))
	secureMux.Handle("/logout", csrfMiddleware(http.HandlerFunc(handleLogout)))
	secureMux.HandleFunc("/health", handleHealthCheck)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default to 8080 for local development
	}

	// Determine which mux to use based on environment variable
	var handler http.Handler
	if os.Getenv("POMELO_MODE") == "secure" {
		log.Printf("Starting in secure mode on port %s", port)
		handler = secureMux
	} else {
		log.Printf("Starting in insecure mode on port %s", port)
		handler = insecureMux
	}

	log.Printf("Server starting at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
