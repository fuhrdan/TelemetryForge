resource "google_project_service" "required" {
  for_each = toset([
    "compute.googleapis.com",
    "container.googleapis.com",
  ])

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_network" "this" {
  name                    = "${var.name}-vpc"
  auto_create_subnetworks = false

  depends_on = [google_project_service.required]
}

resource "google_compute_subnetwork" "this" {
  name          = "${var.name}-${var.region}"
  ip_cidr_range = var.subnet_cidr
  region        = var.region
  network       = google_compute_network.this.id
}

resource "google_container_cluster" "this" {
  name                     = var.name
  location                 = var.region
  network                  = google_compute_network.this.id
  subnetwork               = google_compute_subnetwork.this.id
  remove_default_node_pool = true
  initial_node_count       = 1
  deletion_protection      = false

  ip_allocation_policy {}

  release_channel {
    channel = "REGULAR"
  }
}

resource "google_container_node_pool" "this" {
  name       = "${var.name}-general"
  location   = var.region
  cluster    = google_container_cluster.this.name
  node_count = var.node_count

  node_config {
    machine_type = var.node_machine_type
    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]

    labels = {
      workload = "telemetryforge"
    }
  }
}
