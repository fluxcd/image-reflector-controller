provider "azurerm" {
  features {}
}

resource "random_pet" "suffix" {}

locals {
    name = "flux-test-${random_pet.suffix.id}"
}

variable "region" {
    type = string
    default = "eastus"
}
