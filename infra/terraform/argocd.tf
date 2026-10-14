# Terraform only bootstraps Argo CD and the root app; from there, Git is the source of truth
# for everything that runs in the cluster (gitops/apps).

resource "helm_release" "argocd" {
  name             = "argocd"
  namespace        = "argocd"
  create_namespace = true
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  version          = "10.10.3"

  values = [yamlencode({
    dex = { enabled = false }
    server = {
      service = { type = "ClusterIP" } # reach it with kubectl port-forward, nothing exposed publicly
    }
  })]

  depends_on = [module.eks]
}

resource "helm_release" "argocd_root" {
  name       = "argocd-root"
  namespace  = "argocd"
  repository = "https://argoproj.github.io/argo-helm"
  chart      = "argocd-apps"
  version    = "2.0.6"

  values = [yamlencode({
    applications = {
      kubemicro = {
        namespace = "argocd"
        project   = "default"
        source = {
          repoURL        = var.gitops_repo_url
          targetRevision = var.gitops_revision
          path           = "gitops/apps"
        }
        destination = { server = "https://kubernetes.default.svc", namespace = "argocd" }
        syncPolicy  = { automated = { prune = true, selfHeal = true } }
      }
    }
  })]

  depends_on = [helm_release.argocd, kubernetes_secret_v1.grafana_admin]
}

# Grafana's admin password: generated here, stored only in state and in the cluster secret.
resource "random_password" "grafana_admin" {
  length  = 24
  special = false
}

resource "kubernetes_namespace_v1" "monitoring" {
  metadata {
    name = "monitoring"
  }
  depends_on = [module.eks]
}

resource "kubernetes_secret_v1" "grafana_admin" {
  metadata {
    name      = "grafana-admin"
    namespace = kubernetes_namespace_v1.monitoring.metadata[0].name
  }
  data = {
    admin-user     = "admin"
    admin-password = random_password.grafana_admin.result
  }
}
