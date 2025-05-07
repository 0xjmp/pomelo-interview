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

var db *sql.DB

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

func main() {
	initDB()
	defer db.Close()

	tmpl := template.Must(template.ParseFiles("index.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			text := r.FormValue("text")
			if text != "" {
				// WARNING: This code is deliberately vulnerable to SQL injection!
				// DO NOT USE THIS IN PRODUCTION!
				//
				// The secure way to handle this is to use parameterized queries with db.Exec
				// and placeholders ($1, $2, etc) like this:
				//     db.Exec("INSERT INTO entries (text) VALUES ($1)", text)
				//
				// Direct string interpolation of user input into SQL queries is extremely dangerous
				// as it allows attackers to inject malicious SQL commands.
				// Example attack input: '), ('pwned'), ('pwned

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

		tmpl.Execute(w, entries)
	})

	port := requireEnv("PORT")
	log.Printf("Server starting at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
