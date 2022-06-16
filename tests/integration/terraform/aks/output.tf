output "aks_kubeconfig" {
    value = azurerm_kubernetes_cluster.default.kube_config_raw
}

output "repository_url" {
    value = azurerm_container_registry.acr.login_server
}

output "acr_registry_id" {
    value = azurerm_container_registry.acr.id
}
