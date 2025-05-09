# Enable Secret Manager API
resource "google_project_service" "secretmanager_api" {
  project = var.project_id
  service = "secretmanager.googleapis.com"
}

# Grant Cloud Run service account access to Cloud Run
resource "google_project_iam_member" "run_admin" {
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.cloud_run_service.email}"
}

# Grant service account access to deploy to Cloud Run
resource "google_project_iam_member" "serviceaccount_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.cloud_run_service.email}"
}

# Grant ModSecurity proxy service account permission to use service-to-service auth
resource "google_project_iam_member" "modsecurity_service_auth" {
  project = var.project_id
  role    = "roles/iam.serviceAccountTokenCreator"
  member  = "serviceAccount:${google_service_account.modsecurity_proxy.email}"
}

# Ensure ModSecurity can invoke the internal service
resource "google_cloud_run_service_iam_member" "modsecurity_internal_invoker" {
  location = var.region
  service  = google_cloud_run_service.pomelo_web_internal.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.modsecurity_proxy.email}"
} 