variable "project_id" {
  description = "The GCP project ID"
  type        = string
}

variable "region" {
  description = "The GCP region"
  type        = string
  default     = "us-west1"
}

variable "db_name" {
  description = "The database name"
  type        = string
  default     = "pomelodb"
  sensitive   = true
}

variable "db_user" {
  description = "The database user"
  type        = string
  default     = "pomelo"
  sensitive   = true
}

variable "env_file" {
  description = "Path to the .env file containing environment variables"
  type        = string
  default     = "../.env.production"
}

variable "google_client_id" {
  description = "The Google OAuth client ID"
  type        = string
  sensitive   = true
}

variable "google_client_secret" {
  description = "The Google OAuth client secret"
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "GitHub repository owner"
  type        = string
  default     = "0xjmp"
}

variable "github_repo" {
  description = "GitHub repository name"
  type        = string
  default     = "pomelo-interview"
}

variable "credentials_file" {
  description = "Path to the Google Cloud credentials file"
  type        = string
  default     = "~/.config/gcloud/application_default_credentials.json"
  sensitive   = true
}