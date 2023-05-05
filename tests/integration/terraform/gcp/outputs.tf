output "gcp_kubeconfig" {
  value     = module.gke.kubeconfig
  sensitive = true
}

output "gcp_project" {
  value = module.gke.project
}

output "gcp_region" {
  value = module.gke.region
}

output "gcp_artifact_repository" {
  value = module.gcr.artifact_repository_id
}
