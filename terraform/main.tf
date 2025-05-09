terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "google" {
  project     = var.project_id
  region      = var.region
  credentials = file(var.credentials_file)
}

provider "random" {}

data "google_project" "current" {
  project_id = var.project_id
}

# Enable required APIs
resource "google_project_service" "cloud_run" {
  service = "run.googleapis.com"
}

resource "google_project_service" "secret_manager" {
  service = "secretmanager.googleapis.com"
}

resource "google_project_service" "sql_admin" {
  service = "sqladmin.googleapis.com"
}

resource "google_project_service" "artifact_registry" {
  service = "artifactregistry.googleapis.com"
}

resource "google_project_service" "container_registry" {
  service = "containerregistry.googleapis.com"
}

resource "google_project_service" "cloudbuild" {
  service = "cloudbuild.googleapis.com"
}

resource "google_project_service" "vpc_access" {
  service = "vpcaccess.googleapis.com"
}

resource "google_project_service" "service_networking" {
  service = "servicenetworking.googleapis.com"
}

resource "google_project_service" "compute" {
  service = "compute.googleapis.com"
} 