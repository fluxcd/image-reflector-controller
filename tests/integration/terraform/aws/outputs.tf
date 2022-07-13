output "eks_cluster_name" {
  value = module.eks.cluster_id
}

output "eks_cluster_ca_certificate" {
  value     = module.eks.cluster_ca_data
  sensitive = true
}

output "eks_cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "eks_cluster_arn" {
  value = module.eks.cluster_arn
}

output "region" {
  value = module.eks.region
}

output "ecr_repository_url" {
  value = module.test_ecr.repository_url
}

output "ecr_registry_id" {
  value = module.test_ecr.registry_id
}

output "ecr_image_reflector_controller_repo_url" {
  value = module.image_reflector_ecr.repository_url
}
