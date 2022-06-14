provider "aws" {}

resource "random_pet" "suffix" {}

locals {
  name = "flux-test-${random_pet.suffix.id}"
}
