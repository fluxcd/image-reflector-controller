resource "aws_ecr_repository" "testrepo" {
  name                 = "flux-test-repo-${random_pet.suffix.id}"
  image_tag_mutability = "MUTABLE"
}
