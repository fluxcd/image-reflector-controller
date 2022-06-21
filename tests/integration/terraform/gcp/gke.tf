data "google_client_config" "default" {}

resource "google_container_cluster" "primary" {
  name                   = local.name
  location               = var.location
  initial_node_count = 1
  node_config {
    machine_type = "g1-small"
    disk_size_gb = 10

    oauth_scopes    = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}

module "gke_auth" {
  source               = "terraform-google-modules/kubernetes-engine/google//modules/auth"
  project_id           = var.project_id
  cluster_name         = local.name
  location             = var.location

  depends_on = [google_container_cluster.primary]
}