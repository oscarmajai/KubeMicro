output "cluster_name" {
  value = module.eks.cluster_name
}

output "configure_kubectl" {
  description = "Run this to point kubectl at the cluster."
  value       = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}

output "ecr_registry" {
  description = "Set as the ECR_REGISTRY CI/CD variable in GitLab."
  value       = split("/", values(aws_ecr_repository.service)[0].repository_url)[0]
}

output "ecr_repositories" {
  value = { for k, r in aws_ecr_repository.service : k => r.repository_url }
}

output "ci_role_arn" {
  description = "Set as the AWS_ROLE_ARN CI/CD variable in GitLab."
  value       = aws_iam_role.ci.arn
}

output "argocd_admin_password" {
  description = "Initial Argo CD admin password."
  value       = "kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
}

output "grafana_admin_password" {
  value     = random_password.grafana_admin.result
  sensitive = true
}
