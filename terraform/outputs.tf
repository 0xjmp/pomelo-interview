output "pomelo_web_url" {
  value       = google_cloud_run_service.pomelo_web.status[0].url
  description = "URL of the web service"
}

output "pomelo_web_secure_url" {
  value       = google_cloud_run_service.pomelo_web_secure.status[0].url
  description = "URL of the ModSecurity proxy"
}

output "database_name" {
  value = var.db_name
}

output "database_user" {
  value = var.db_user
}

output "registry_location" {
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo"
  description = "Docker registry location"
}

output "image_web" {
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo/pomelo-web:latest"
  description = "Web service image location"
}

output "image_modsecurity" {
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo/modsecurity:latest"
  description = "ModSecurity proxy image location"
} 