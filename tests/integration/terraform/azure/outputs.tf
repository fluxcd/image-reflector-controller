output "aks_kubeconfig" {
  value     = azurerm_kubernetes_cluster.default.kube_config_raw
  sensitive = true
}

output "acr_registry_url" {
  value = azurerm_container_registry.acr.login_server
}

output "acr_registry_id" {
  value = azurerm_container_registry.acr.id
}
