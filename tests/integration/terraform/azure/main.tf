provider "azurerm" {
  features {}
}

resource "random_pet" "suffix" {
  // Since azurerm doesn't allow "-" in registry name, use an alphabet as a
  // separator.
  separator = "o"
}

locals {
  name = "flux-test-${random_pet.suffix.id}"
}
