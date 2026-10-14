variable "region" {
  description = "AWS region."
  type        = string
  default     = "us-east-1"
}

variable "cluster_name" {
  description = "EKS cluster name; also prefixes the VPC and IAM resources."
  type        = string
  default     = "kubemicro"
}

variable "kubernetes_version" {
  description = "EKS Kubernetes version."
  type        = string
  default     = "1.36"
}

variable "vpc_cidr" {
  description = "VPC CIDR. Split into 3 public /24 and 3 private /20 subnets, one of each per AZ."
  type        = string
  default     = "10.40.0.0/16"
}

variable "single_nat_gateway" {
  description = "true = one shared NAT (cheaper, loses egress if its AZ fails). false = one NAT per AZ (HA)."
  type        = bool
  default     = false
}

variable "admin_cidrs" {
  description = "CIDRs allowed to reach the public EKS API endpoint. Restrict to your own IP."
  type        = list(string)

  validation {
    condition     = length(var.admin_cidrs) > 0 && !contains(var.admin_cidrs, "0.0.0.0/0")
    error_message = "Set admin_cidrs to your own IP range(s); 0.0.0.0/0 is not allowed."
  }
}

variable "node_instance_types" {
  description = "Instance types for the managed node group."
  type        = list(string)
  default     = ["t3.medium"]
}

variable "node_group_size" {
  description = "Managed node group size. min_size 3 keeps one node per AZ."
  type        = object({ min = number, desired = number, max = number })
  default     = { min = 3, desired = 3, max = 6 }
}

variable "services" {
  description = "Services that get an ECR repository and push rights for CI."
  type        = set(string)
  default     = ["catalog", "orders"]
}

variable "gitlab_url" {
  description = "GitLab instance that runs the pipeline (OIDC issuer)."
  type        = string
  default     = "https://gitlab.com"
}

variable "gitlab_project_path" {
  description = "GitLab project allowed to assume the CI role, e.g. oscarmajai/kubemicro."
  type        = string
}

variable "gitops_repo_url" {
  description = "Git repository Argo CD syncs from."
  type        = string
  default     = "https://github.com/oscarmajai/KubeMicro.git"
}

variable "gitops_revision" {
  description = "Branch or tag Argo CD tracks."
  type        = string
  default     = "main"
}
