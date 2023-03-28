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

output "flux_acr_registry_url" {
  value = module.acr_flux.registry_url
}

output "flux_acr_registry_id" {
  value = module.acr_flux.registry_id
}

output "spn_id" {
  value = azuread_application.flux.application_id
}
