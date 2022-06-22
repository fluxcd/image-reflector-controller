data "google_client_config" "current" {}

resource "google_container_cluster" "primary" {
  name               = local.name
  location           = data.google_client_config.current.region
  initial_node_count = 1
  node_config {
    machine_type = "g1-small"
    disk_size_gb = 10

    # Set the scope to grant the nodes all the API access.
    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}

# auth module to retrieve kubeconfig of the created cluster.
module "gke_auth" {
  source  = "terraform-google-modules/kubernetes-engine/google//modules/auth"
  version = "~> 21"

  project_id   = data.google_client_config.current.project
  cluster_name = local.name
  location     = data.google_client_config.current.region

  depends_on = [google_container_cluster.primary]
}
