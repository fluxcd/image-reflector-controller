output "gcr_repository_url" {
  value = data.google_container_registry_repository.test_repo.repository_url
}

output "gcp_kubeconfig" {
  value     = module.gke_auth.kubeconfig_raw
  sensitive = true
}

output "gcp_project" {
  value = data.google_client_config.current.project
}

output "gcp_region" {
  value = data.google_client_config.current.region
}

output "gcp_artifact_repository" {
  value = google_artifact_registry_repository.test_repo.repository_id
}
