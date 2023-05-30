provider "aws" {}

resource "random_pet" "suffix" {}

locals {
  name = "flux-test-${var.rand}"
}

module "eks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/eks"

  name = local.name
  tags = var.tags
}

module "test_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-repo-${local.name}"
  tags = var.tags
}

module "image_reflector_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-image-reflector-${local.name}"
  tags = var.tags
}
