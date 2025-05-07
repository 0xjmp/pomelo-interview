# SQL Injection Demo App

This is a deliberately vulnerable web application designed to demonstrate SQL injection attacks in a controlled environment. **DO NOT USE THIS CODE IN PRODUCTION.**

## Purpose

This application serves as an educational tool to:
1. Demonstrate how SQL injection vulnerabilities occur
2. Show the impact of unsanitized user input
3. Help developers understand why parameterized queries are essential

## The Vulnerability

The app contains a simple text input form that stores messages in a PostgreSQL database. The application deliberately uses string concatenation instead of parameterized queries, making it vulnerable to SQL injection:

```go
// Vulnerable code (DO NOT USE IN PRODUCTION):
query := fmt.Sprintf("INSERT INTO entries (text) VALUES ('%s')", userInput)
```

### Example Attack

Normal input like "hello world" works as expected, but the application is vulnerable to SQL injection. For example, this input:
```
'), ('pwned'), ('pwned
```

Results in this SQL being executed:
```sql
INSERT INTO entries (text) VALUES (''), ('pwned'), ('pwned')
```

This allows an attacker to insert multiple unauthorized entries into the database.

## Secure Alternative

The secure way to handle this would be to use parameterized queries:
```go
db.Exec("INSERT INTO entries (text) VALUES ($1)", userInput)
```

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- Go 1.x (for local development without Docker)

## Environment Setup

1. Clone the repository
2. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
3. Update the `.env` file with your desired configuration:
   ```
   PORT=8080                           # Application port
   POSTGRES_USER=pomelo                # Database user
   POSTGRES_PASSWORD=your_password     # Set a secure password
   POSTGRES_DB=pomelodb               # Database name
   POSTGRES_HOST=db                    # Database host (use 'db' for Docker)
   POSTGRES_PORT=5432                  # Database port
   ```

## Running the Application

### Using Docker Compose (Recommended)

1. Start the application:
   ```bash
   docker-compose up
   ```
2. The application will be available at `http://localhost:8080`

### Local Development

1. Install dependencies:
   ```bash
   go mod download
   ```
2. Update the `POSTGRES_HOST` in `.env` to point to your local PostgreSQL instance
3. Run the application:
   ```bash
   go run main.go
   ```

## Project Structure

- `main.go` - Main application entry point
- `schema.sql` - Database schema
- `Dockerfile` - Container configuration
- `docker-compose.yml` - Multi-container Docker configuration
- `.air.toml` - Live reload configuration

## Development

The application uses Docker Compose for development, which includes:
- Hot reload functionality
- PostgreSQL database
- Volume mounting for real-time code changes

## License

MIT

## Author

Jake Peterson 