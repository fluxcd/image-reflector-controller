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
  source = "git::https://github.com/somtochiama/test-infra.git//tf-modules/azure/aks?ref=az-workload"

  name     = local.name
  location = var.azure_location
}

module "acr" {
  source = "git::https://github.com/somtochiama/test-infra.git//tf-modules/azure/acr?ref=az-workload"

  name             = local.name
  location         = var.azure_location
  resource_group   = module.aks.resource_group
}

module "acr_flux" {
  source = "git::https://github.com/somtochiama/test-infra.git//tf-modules/azure/acr?ref=az-workload"

  name             = "manager${random_pet.suffix.id}"
  location         = var.azure_location
  resource_group   = module.aks.resource_group
  aks_principal_id = [module.aks.principal_id]

  depends_on = [module.aks]
}

resource "azuread_application" "flux" {
  display_name = "acr-sp"

  required_resource_access {
    resource_app_id = "00000003-0000-0000-c000-000000000000"

    resource_access {
      id   = "df021288-bdef-4463-88db-98f22de89214"
      type = "Role"
    }
  }

  required_resource_access {
    resource_app_id = "00000002-0000-0000-c000-000000000000"

    resource_access {
      id   = "1cda74f2-2616-4834-b122-5cb1b07f8a59"
      type = "Role"
    }
    resource_access {
      id   = "78c8a3c8-a07e-4b9e-af1b-b5ccab50a175"
      type = "Role"
    }
  }
}

resource "azuread_service_principal" "flux" {
  application_id = azuread_application.flux.application_id
}

resource "azurerm_role_assignment" "acr" {
  scope                =  module.acr.registry_id
  role_definition_name = "AcrPull"
  principal_id         = azuread_service_principal.flux.object_id
}


resource "azuread_application_federated_identity_credential" "example" {
  application_object_id = azuread_application.flux.object_id
  display_name          = "image-reflector-sa"
  description           = "Kubernetes service account federated credential"
  audiences             = ["api://AzureADTokenExchange"]
  issuer                = module.aks.cluster_oidc_url
  subject               = "system:serviceaccount:flux-system:image-reflector-controller"
}
