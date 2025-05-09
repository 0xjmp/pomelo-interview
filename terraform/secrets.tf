resource "random_password" "db_password" {
  length  = 16
  special = true
}

resource "random_password" "session_secret" {
  length  = 32
  special = true
}

resource "random_password" "csrf_key" {
  length  = 32
  special = false # CSRF key typically needs to be URL-safe
}

# Database password
resource "google_secret_manager_secret" "db_password" {
  secret_id = "pomelo-db-password"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = random_password.db_password.result
}

data "google_secret_manager_secret_version" "db_password" {
  secret = google_secret_manager_secret.db_password.id
  version = "latest"
  depends_on = [google_secret_manager_secret_version.db_password]
}

# Session secret
resource "google_secret_manager_secret" "session_secret" {
  secret_id = "pomelo-session-secret"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "session_secret" {
  secret      = google_secret_manager_secret.session_secret.id
  secret_data = random_password.session_secret.result
}

# CSRF key
resource "google_secret_manager_secret" "csrf_key" {
  secret_id = "pomelo-csrf-key"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "csrf_key" {
  secret      = google_secret_manager_secret.csrf_key.id
  secret_data = random_password.csrf_key.result
} 