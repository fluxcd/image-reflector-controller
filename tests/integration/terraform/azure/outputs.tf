output "aks_kubeconfig" {
  value     = module.aks.kubeconfig
  sensitive = true
}

output "acr_registry_url" {
  value = module.acr.registry_url
}

output "acr_registry_id" {
  value = module.acr.registry_id
}
