output "gcr_repository_url" {
  value = data.google_container_registry_repository.test-repo.repository_url
}

output "gcp_kubeconfig" {
    value = module.gke_auth.kubeconfig_raw
}

output "artifact_location" {
    value = var.location
}

output "artifact_project" {
    value = var.project_id
}

output "artifact_repository" {
    value = random_pet.suffix.id
}