package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/0xjmp/pomelo-interview/auth"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
)

type Entry struct {
	ID        int
	Text      string
	Timestamp time.Time
}

type PageData struct {
	Entries    []Entry
	Mode       string
	BadgeClass string
	User       *auth.GoogleUser
	CSRFToken  string
}

var (
	db         *sql.DB
	templates  *template.Template
	store      *sessions.CookieStore
	oauth      *oauth2.Config
	googleAuth *auth.GoogleAuth
)

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

	// Create table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS entries (
			id SERIAL PRIMARY KEY,
			text TEXT NOT NULL,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal("Failed to create entries table:", err)
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

func getUser(r *http.Request) *auth.GoogleUser {
	session, _ := store.Get(r, "session")
	val := session.Values["user"]
	user, ok := val.(*auth.GoogleUser)
	if !ok {
		return nil
	}
	return user
}

func handleInsecure(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		text := r.FormValue("text")
		if text != "" {
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

	// Fetch entries from database
	rows, err := db.Query("SELECT id, text, timestamp FROM entries ORDER BY timestamp DESC")
	if err != nil {
		log.Printf("Error querying entries: %v", err)
		http.Error(w, "Error fetching entries", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Text, &entry.Timestamp)
		if err != nil {
			log.Printf("Error scanning entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	data := PageData{
		Entries:    entries,
		Mode:       "Insecure",
		BadgeClass: "insecure",
	}

	templates.ExecuteTemplate(w, "base.html", data)
}

func handleSecure(w http.ResponseWriter, r *http.Request) {
	// Get user info if logged in, but don't require it
	user := getUser(r)

	if r.Method == http.MethodPost {
		// CSRF token is automatically verified by the middleware
		text := r.FormValue("text")
		if text != "" {
			// Use parameterized query for security
			_, err := db.Exec("INSERT INTO entries (text) VALUES ($1)", text)
			if err != nil {
				log.Printf("Error inserting entry: %v", err)
				http.Error(w, "Error saving entry", http.StatusInternalServerError)
				return
			}
		}
	}

	// Fetch entries using parameterized query
	rows, err := db.Query("SELECT id, text, timestamp FROM entries ORDER BY timestamp DESC")
	if err != nil {
		log.Printf("Error querying entries: %v", err)
		http.Error(w, "Error fetching entries", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		err := rows.Scan(&entry.ID, &entry.Text, &entry.Timestamp)
		if err != nil {
			log.Printf("Error scanning entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	data := PageData{
		Entries:    entries,
		Mode:       "Secure",
		BadgeClass: "secure",
		User:       user,
		CSRFToken:  csrf.Token(r),
	}

	templates.ExecuteTemplate(w, "base.html", data)
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

func main() {
	initDB()
	initAuth()
	defer db.Close()

	// Parse template
	templates = template.Must(template.ParseFiles("base.html"))

	// Create separate mux for each server
	insecureMux := http.NewServeMux()
	secureMux := http.NewServeMux()

	// Set up routes
	insecureMux.HandleFunc("/", handleInsecure)

	csrfMiddleware := csrf.Protect([]byte(requireEnv("CSRF_KEY")))

	// Wrap secure routes with CSRF protection
	secureMux.Handle("/", csrfMiddleware(http.HandlerFunc(handleSecure)))
	secureMux.Handle("/login", csrfMiddleware(http.HandlerFunc(handleLogin)))
	secureMux.Handle("/callback", csrfMiddleware(http.HandlerFunc(handleCallback)))
	secureMux.Handle("/logout", csrfMiddleware(http.HandlerFunc(handleLogout)))

	// Get ports from environment
	insecurePort := requireEnv("INSECURE_PORT")
	securePort := requireEnv("SECURE_PORT")

	// Start servers
	go func() {
		log.Printf("Insecure server starting at http://localhost:%s", insecurePort)
		log.Fatal(http.ListenAndServe(":"+insecurePort, insecureMux))
	}()

	log.Printf("Secure server starting at http://localhost:%s", securePort)
	log.Fatal(http.ListenAndServe(":"+securePort, secureMux))
}
