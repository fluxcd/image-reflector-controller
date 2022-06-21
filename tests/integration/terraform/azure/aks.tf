resource "azurerm_resource_group" "default" {
  name     = local.name
  location = var.azure_location
}

resource "azurerm_kubernetes_cluster" "default" {
  name                = "aks-${random_pet.suffix.id}"
  location            = azurerm_resource_group.default.location
  resource_group_name = azurerm_resource_group.default.name
  dns_prefix          = "aks-${random_pet.suffix.id}"
  default_node_pool {
    name            = "default"
    node_count      = 2
    vm_size         = "Standard_B2s"
    os_disk_size_gb = 30
  }
  identity {
    type = "SystemAssigned"
  }
  role_based_access_control_enabled = true
  network_profile {
    network_plugin = "kubenet"
    network_policy = "calico"
  }
}
