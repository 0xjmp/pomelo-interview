resource "google_artifact_registry_repository" "pomelo" {
  location      = var.region
  repository_id = "pomelo"
  description   = "Docker repository for Pomelo services"
  format        = "DOCKER"
} 