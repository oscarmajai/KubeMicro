# GitLab CI authenticates to AWS with its OIDC ID token: no long-lived access keys in CI variables.
locals {
  gitlab_host = trimprefix(var.gitlab_url, "https://")
}

resource "aws_iam_openid_connect_provider" "gitlab" {
  url            = var.gitlab_url
  client_id_list = [var.gitlab_url]
}

data "aws_iam_policy_document" "ci_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.gitlab.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.gitlab_host}:aud"
      values   = [var.gitlab_url]
    }

    # Only pipelines on the protected main branch of this project may push images.
    condition {
      test     = "StringEquals"
      variable = "${local.gitlab_host}:sub"
      values   = ["project_path:${var.gitlab_project_path}:ref_type:branch:ref:main"]
    }
  }
}

resource "aws_iam_role" "ci" {
  name                 = "${var.cluster_name}-gitlab-ci"
  assume_role_policy   = data.aws_iam_policy_document.ci_trust.json
  max_session_duration = 3600
}

data "aws_iam_policy_document" "ci_ecr_push" {
  statement {
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  statement {
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [for r in aws_ecr_repository.service : r.arn]
  }
}

resource "aws_iam_role_policy" "ci_ecr_push" {
  name   = "ecr-push"
  role   = aws_iam_role.ci.id
  policy = data.aws_iam_policy_document.ci_ecr_push.json
}
