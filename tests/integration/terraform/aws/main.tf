module "tags" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/utils/tags"

  tags = var.tags
}

provider "aws" {
  default_tags {
    tags = module.tags.tags
  }
}

resource "random_pet" "suffix" {}

locals {
  name = "flux-test-${var.rand}"
}

module "eks" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/eks"

  name = local.name
  tags = module.tags.tags
}

module "test_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-repo-${local.name}"
}

module "image_reflector_ecr" {
  source = "git::https://github.com/fluxcd/test-infra.git//tf-modules/aws/ecr"

  name = "test-image-reflector-${local.name}"
}
