data "google_container_registry_repository" "test_repo" {
  region = var.gcr_region
}

resource "google_artifact_registry_repository" "test_repo" {
  provider = google-beta

  project       = data.google_client_config.current.project
  location      = data.google_client_config.current.region
  repository_id = "flux-test-repo-${random_pet.suffix.id}"
  description   = "example docker repository"
  format        = "DOCKER"
}
