provider "azurerm" {
  features {}
}

resource "random_pet" "suffix" {
  // Since azurerm doesn't allow "-" in registry name, use an alphabet as a
  // separator.
  separator = "o"
}

locals {
  name = "fluxTest${random_pet.suffix.id}"
}

module "aks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/azure/aks"

  name     = local.name
  location = var.azure_location
  tags     = var.tags
}

module "acr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/azure/acr"

  name             = local.name
  location         = var.azure_location
  aks_principal_id = module.aks.principal_id
  resource_group   = module.aks.resource_group
  tags             = var.tags

  depends_on = [module.aks]
}
