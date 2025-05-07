# SQL Injection Demo App

This is a deliberately vulnerable web application designed to demonstrate SQL injection attacks in a controlled environment. **DO NOT USE THIS CODE IN PRODUCTION.**

## Purpose

This application serves as an educational tool to:
1. Demonstrate how SQL injection vulnerabilities occur
2. Show the impact of unsanitized user input
3. Help developers understand why parameterized queries are essential
4. Demonstrate how mod_security can block attacks even when the app contains vulnerable code

The application provides two endpoints to demonstrate ModSecurity's effectiveness:
   - Insecure endpoint (`http://localhost:8080`): Bypasses ModSecurity, allowing SQL injection attacks
   - Secure endpoint (`http://localhost:8081`): Protected by ModSecurity, blocks malicious requests

## The Vulnerability

The app contains a simple text input form that stores messages in a PostgreSQL database. The application deliberately uses string concatenation instead of parameterized queries, making it vulnerable to SQL injection:

```go
// Vulnerable code (DO NOT USE IN PRODUCTION):
query := fmt.Sprintf("INSERT INTO entries (text) VALUES ('%s')", userInput)
```

### Example Attacks

Normal input like "hello world" works as expected, but the application is vulnerable to SQL injection. For example, this input:
```
'), ('pwned'), ('pwned
```

Results in this SQL being executed:
```sql
INSERT INTO entries (text) VALUES (''), ('pwned'), ('pwned')
```

This allows an attacker to insert multiple unauthorized entries into the database.

### ModSecurity Protection

Here are some example payloads that were tested and blocked on the secure endpoint:

1. Basic SQL Injection:
```
Input: '), ('test
Response: 403 Forbidden
```

2. Concatenation-based Injection:
```
Input: test'||(SELECT 'x')||'
Response: 403 Forbidden
```

3. Case-mixing with URL Encoding:
```
Input: x' UniOn SeLeCt 'y
Response: 403 Forbidden
```

4. Variable assignments in SQL syntax:
```
Input: SET @var = 'test'; SELECT * FROM entries WHERE text = @var;
Response: 403 Forbidden
```

_Patched in: https://lists.debian.org/debian-lts-announce/2023/01/msg00033.html_

5. Using comment characters:
```
Input: '); -- comment
Response: 200 Ok
```

_Patched in: https://lists.debian.org/debian-lts-announce/2023/01/msg00033.html_

_While this was allowed by modsecurity, I was unable to use it for any kind of meaningful attack._

6. More complex comment characters:
```
'); /* multiline comment */ SELECT 'x'; --
Response: 403 Forbidden
```

_Patched in: https://lists.debian.org/debian-lts-announce/2023/01/msg00033.html_

7. Everyone's favorite SQL injection test:
```
' OR '1'='1
Response: 403 Forbidden
```

These attempts are blocked by ModSecurity with PARANOIA level 1, demonstrating the effectiveness of the WAF protection layer. Try the same payloads on both endpoints to see the difference in behavior.

## Secure Alternative

The way to completely mitigate this vulnerability would be to use parameterized queries:
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
   POSTGRES_DB=pomelodb                # Database name
   POSTGRES_HOST=db                    # Database host (use 'db' for Docker)
   POSTGRES_PORT=5432                  # Database port
   ```

## Running the Application

### Using Docker Compose (Recommended)

1. Start the application:
   ```bash
   docker-compose up
   ```

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
- `base.html` - Base HTML template used by both insecure and secure endpoint
- `schema.sql` - Database schema
- `Dockerfile` - Basic golang container configuration
- `docker-compose.yml` - Multi-container Docker configuration

Uses modsecurity CRS ruleset v4.14.0 (latest).

## License

MIT

## Author

Jake Peterson

### What I learned making this
- Modsecurity simply works, which is really cool. I was expecting to have to create my own ruleset. 
   - Based on my research, it appears nginx is a safer choice over apache2 as the latter has a history of CVEs.
   - ModSecurity v3 (which is what runs on Nginx) was rewritten from scratch to be more secure and performant than the Apache version (ModSecurity v2).
   - In a production setting, paranoia would likely be set to a higher value than 1. I kept it at 1 for this demo on purpose to increase my chances of finding interesting bypasses.
   - I couldn't find any published CVEs specific to CRS 4.1.0. This is the latest version and was released with significant security improvements.
   - Modsecurity detects SQL keywords so while I can use SQL comments I cannot put anything meaningful beyond the comment that would constitute an attack.
- I wanted to be fancy and make the go application compile its front-end to wasm. The project structure I initially created (following best practices) felt convoluted so I opted for a simpler setup.
- Not as much something I've learned, but reminded... building anything in Go is consistently a _good time_. It's really such an impressive language that nails simplicity at near-native runtime performance.
- This is the best take-home assessment I've been given. Bite-size, relevant to my work, and open-ended enough that I get to think for myself. I will definitely be using an interview question inspired by this one in the future :) 