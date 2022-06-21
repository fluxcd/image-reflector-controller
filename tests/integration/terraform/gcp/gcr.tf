resource "google_container_registry" "registry" {
  project  = "dx-somtoxhi"
}

data "google_container_registry_repository" "test-repo" {
  region = "us"
}

resource "google_artifact_registry_repository" "test-repo" {
  provider = google-beta

  project = var.project_id
  location = var.location
  repository_id = random_pet.suffix.id
  description = "example docker repository"
  format = "DOCKER"
}

