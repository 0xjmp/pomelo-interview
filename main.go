package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
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
}

var db *sql.DB
var templates *template.Template

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
	if r.Method == http.MethodPost {
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
	}

	templates.ExecuteTemplate(w, "base.html", data)
}

func main() {
	initDB()
	defer db.Close()

	// Parse template
	templates = template.Must(template.ParseFiles("base.html"))

	// Create separate mux for each server
	insecureMux := http.NewServeMux()
	secureMux := http.NewServeMux()

	// Set up routes
	insecureMux.HandleFunc("/", handleInsecure)
	secureMux.HandleFunc("/", handleSecure)

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
