provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "kubemicro"
      ManagedBy = "terraform"
      Repo      = "github.com/oscarmajai/KubeMicro"
    }
  }
}

# Both Kubernetes-facing providers authenticate with short-lived tokens from the AWS CLI,
# so no kubeconfig or static credentials are ever written to state.
locals {
  cluster_exec = {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name, "--region", var.region]
  }
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = local.cluster_exec.api_version
    command     = local.cluster_exec.command
    args        = local.cluster_exec.args
  }
}

provider "helm" {
  kubernetes = {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
    exec                   = local.cluster_exec
  }
}
