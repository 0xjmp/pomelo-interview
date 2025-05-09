resource "google_service_account" "cloud_run_service" {
  account_id   = "cloud-run-service"
  display_name = "Cloud Run Service Account"
}

# Grant Secret Manager access to the Cloud Run service account
resource "google_project_iam_member" "secret_accessor" {
  project = data.google_project.current.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${google_service_account.cloud_run_service.email}"
}

# Grant Cloud SQL access to the Cloud Run service account
resource "google_project_iam_member" "cloudsql_client" {
  project = data.google_project.current.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.cloud_run_service.email}"
}

# Create Cloud SQL instance
resource "google_sql_database_instance" "pomelo" {
  name             = "pomelo-db"
  database_version = "POSTGRES_15"
  region          = var.region

  settings {
    tier = "db-f1-micro"  # Free tier
    disk_size = 10
    
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.private.id
    }

    backup_configuration {
      enabled = false
    }

    maintenance_window {
      day  = 7  # Sunday
      hour = 3  # 3 AM
    }

    insights_config {
      query_insights_enabled = true
      query_string_length = 1024
      record_application_tags = false
      record_client_address = false
    }
  }

  deletion_protection = false  # Set to true for production

  depends_on = [
    google_service_networking_connection.private_vpc_connection
  ]
}

resource "google_sql_database" "pomelo" {
  name     = var.db_name
  instance = google_sql_database_instance.pomelo.name
}

resource "google_sql_user" "pomelo" {
  name     = var.db_user
  instance = google_sql_database_instance.pomelo.name
  password = data.google_secret_manager_secret_version.db_password.secret_data
}

locals {
  common_env_vars = {
    POSTGRES_USER     = var.db_user
    POSTGRES_DB       = var.db_name
    POSTGRES_HOST     = google_sql_database_instance.pomelo.private_ip_address
    POSTGRES_PORT     = "5432"
    GOOGLE_CLIENT_ID  = var.google_client_id
    GOOGLE_CLIENT_SECRET = var.google_client_secret
    INSECURE_URL     = "https://pomelo-web-463525203440.us-west1.run.app"
    SECURE_URL       = "https://pomelo-web-secure-463525203440.us-west1.run.app"
  }

  common_secret_vars = {
    POSTGRES_PASSWORD = {
      name = google_secret_manager_secret.db_password.secret_id
      key  = "latest"
    }
    SESSION_SECRET = {
      name = google_secret_manager_secret.session_secret.secret_id
      key  = "latest"
    }
    CSRF_KEY = {
      name = google_secret_manager_secret.csrf_key.secret_id
      key  = "latest"
    }
  }

  vpc_annotations = {
    "run.googleapis.com/vpc-access-connector" = google_vpc_access_connector.connector.name
    "run.googleapis.com/vpc-access-egress"    = "private-ranges-only"
  }
}

resource "google_cloud_run_service" "pomelo_web" {
  name     = "pomelo-web"
  location = var.region

  template {
    metadata {
      annotations = merge(local.vpc_annotations, {
        "run.googleapis.com/cpu-throttling" = "true"
        "autoscaling.knative.dev/maxScale" = "1"
      })
    }
    spec {
      service_account_name = google_service_account.cloud_run_service.email
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo/pomelo-web:latest"
        
        ports {
          container_port = 8080
          name          = "http1"
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = "256Mi"
          }
        }

        env {
          name  = "POMELO_MODE"
          value = "insecure"
        }

        dynamic "env" {
          for_each = local.common_env_vars
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.common_secret_vars
          content {
            name = env.key
            value_from {
              secret_key_ref {
                name = env.value.name
                key  = env.value.key
              }
            }
          }
        }
      }
    }
  }

  depends_on = [
    google_secret_manager_secret.db_password,
    google_secret_manager_secret.session_secret,
    google_secret_manager_secret.csrf_key,
    google_project_iam_member.secret_accessor,
    google_project_iam_member.cloudsql_client,
    google_artifact_registry_repository.pomelo,
    google_sql_database_instance.pomelo,
    google_vpc_access_connector.connector
  ]
}

resource "google_cloud_run_service" "pomelo_web_internal" {
  name     = "pomelo-web-internal"
  location = var.region

  template {
    metadata {
      annotations = merge(local.vpc_annotations, {
        "run.googleapis.com/cpu-throttling" = "true"
        "autoscaling.knative.dev/maxScale" = "1"
        "run.googleapis.com/ingress" = "all"
      })
    }
    spec {
      service_account_name = google_service_account.cloud_run_service.email
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo/pomelo-web:latest"
        
        ports {
          container_port = 8080
          name          = "http1"
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = "256Mi"
          }
        }

        env {
          name  = "POMELO_MODE"
          value = "secure"
        }

        dynamic "env" {
          for_each = local.common_env_vars
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.common_secret_vars
          content {
            name = env.key
            value_from {
              secret_key_ref {
                name = env.value.name
                key  = env.value.key
              }
            }
          }
        }
      }
    }
  }

  depends_on = [
    google_secret_manager_secret.db_password,
    google_secret_manager_secret.session_secret,
    google_secret_manager_secret.csrf_key,
    google_project_iam_member.secret_accessor,
    google_project_iam_member.cloudsql_client,
    google_artifact_registry_repository.pomelo,
    google_sql_database_instance.pomelo,
    google_vpc_access_connector.connector
  ]
}

resource "google_cloud_run_service" "pomelo_web_secure" {
  name     = "pomelo-web-secure"
  location = var.region

  template {
    metadata {
      annotations = merge(local.vpc_annotations, {
        "run.googleapis.com/cpu-throttling" = "true"
        "autoscaling.knative.dev/maxScale" = "1"
        "run.googleapis.com/container-health-metrics" = "true"
        "run.googleapis.com/ingress" = "all"
      })
    }
    spec {
      service_account_name = google_service_account.modsecurity_proxy.email
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/pomelo/modsecurity:latest"
        
        ports {
          container_port = 8080
          name          = "http1"
        }

        resources {
          limits = {
            cpu    = "1000m"
            memory = "512Mi"
          }
        }

        env {
          name  = "BACKEND"
          value = "https://${google_cloud_run_service.pomelo_web_internal.name}.run.internal"
        }
        env {
          name  = "PROXY_SSL"
          value = "on"
        }
        env {
          name  = "SSL_VERIFY"
          value = "off"
        }
      }
    }
  }

  depends_on = [
    google_cloud_run_service.pomelo_web_internal,
    google_artifact_registry_repository.pomelo,
    google_vpc_access_connector.connector
  ]
}

resource "google_service_account" "modsecurity_proxy" {
  account_id   = "modsecurity-proxy"
  display_name = "ModSecurity Proxy Service Account"
}

resource "google_cloud_run_service_iam_member" "pomelo_web_secure_public" {
  location = google_cloud_run_service.pomelo_web_secure.location
  service  = google_cloud_run_service.pomelo_web_secure.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_service_iam_member" "pomelo_web_public" {
  location = google_cloud_run_service.pomelo_web.location
  service  = google_cloud_run_service.pomelo_web.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_service_iam_member" "pomelo_web_internal_access" {
  location = google_cloud_run_service.pomelo_web_internal.location
  service  = google_cloud_run_service.pomelo_web_internal.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.modsecurity_proxy.email}"
} 