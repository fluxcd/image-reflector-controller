resource "aws_ecr_repository" "testrepo" {
  name                 = "flux-test-repo-${random_pet.suffix.id}"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

resource "aws_ecr_repository" "image_reflector_controller" {
  name                 = "flux-test-image-reflector-${random_pet.suffix.id}"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}
