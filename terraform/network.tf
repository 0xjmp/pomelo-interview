# Create VPC network
resource "google_compute_network" "private" {
  name                    = "pomelo-private"
  auto_create_subnetworks = false

  depends_on = [
    google_project_service.compute
  ]
}

# Create subnet for the VPC connector
resource "google_compute_subnetwork" "private" {
  name          = "pomelo-private"
  ip_cidr_range = "10.0.0.0/16"
  network       = google_compute_network.private.id
  region        = var.region

  # Enable private Google access for Cloud SQL
  private_ip_google_access = true

  depends_on = [
    google_project_service.compute
  ]
}

# Create serverless VPC access connector
resource "google_vpc_access_connector" "connector" {
  name          = "pomelo-vpc"
  region        = var.region
  ip_cidr_range = "10.8.0.0/28"
  network       = google_compute_network.private.name
  machine_type  = "e2-micro" # Free tier machine type
  min_instances = 2
  max_instances = 3
  
  depends_on = [
    google_compute_subnetwork.private,
    google_project_service.vpc_access
  ]
}

# Allocate private IP range for services
resource "google_compute_global_address" "private_ip_address" {
  name          = "pomelo-private-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.private.id

  depends_on = [
    google_project_service.compute
  ]
}

# Create VPC peering connection
resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.private.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]

  depends_on = [
    google_project_service.service_networking
  ]
} 