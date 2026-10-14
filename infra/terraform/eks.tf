module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 21.0"

  name               = var.cluster_name
  kubernetes_version = var.kubernetes_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  endpoint_public_access       = true
  endpoint_public_access_cidrs = var.admin_cidrs
  endpoint_private_access      = true

  # Whoever runs terraform apply gets cluster-admin through an EKS access entry.
  enable_cluster_creator_admin_permissions = true

  addons = {
    vpc-cni                = { before_compute = true }
    eks-pod-identity-agent = { before_compute = true }
    coredns                = {}
    kube-proxy             = {}
  }

  eks_managed_node_groups = {
    default = {
      ami_type       = "AL2023_x86_64_STANDARD"
      instance_types = var.node_instance_types
      capacity_type  = "ON_DEMAND"

      min_size     = var.node_group_size.min
      desired_size = var.node_group_size.desired
      max_size     = var.node_group_size.max

      # Replace nodes one at a time so the PDBs are always honoured.
      update_config = { max_unavailable = 1 }
    }
  }
}
