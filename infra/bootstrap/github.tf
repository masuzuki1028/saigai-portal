# GitHub Actions から OIDC で一時クレデンシャルを取得する（長期のアクセスキーを作らない。docs/06 4章）。
# OIDC プロバイダは1つのアカウントに1つだけ作れる。ほかのプロジェクトで作成済みなら、作らずに参照する
# （共有しているプロバイダを、このスタックから変更・削除しないようにする）
resource "aws_iam_openid_connect_provider" "github" {
  count          = var.create_github_oidc_provider ? 1 : 0
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

data "aws_iam_openid_connect_provider" "github" {
  count = var.create_github_oidc_provider ? 0 : 1
  url   = "https://token.actions.githubusercontent.com"
}

locals {
  github_sub      = "repo:${var.github_repository}"
  github_oidc_arn = var.create_github_oidc_provider ? aws_iam_openid_connect_provider.github[0].arn : data.aws_iam_openid_connect_provider.github[0].arn
}

# plan 用（PR・main・deploy-prod の plan ジョブ）。読み取り専用
data "aws_iam_policy_document" "gha_plan_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values = [
        "${local.github_sub}:pull_request",
        "${local.github_sub}:ref:refs/heads/main",
        "${local.github_sub}:environment:prod-plan",
      ]
    }
  }
}

resource "aws_iam_role" "gha_plan" {
  name                 = "saigai-gha-plan"
  assume_role_policy   = data.aws_iam_policy_document.gha_plan_trust.json
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "gha_plan_readonly" {
  role       = aws_iam_role.gha_plan.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

resource "aws_iam_role_policy" "gha_plan_deny_data" {
  name   = "deny-data-read"
  role   = aws_iam_role.gha_plan.id
  policy = data.aws_iam_policy_document.deny_data_read.json
}

# デプロイ用（deploy-prod.yml の deploy ジョブ。GitHub Environment「prod」の承認後だけ）
data "aws_iam_policy_document" "gha_deploy_trust" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [local.github_oidc_arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["${local.github_sub}:environment:prod"]
    }
  }
}

resource "aws_iam_role" "gha_deploy" {
  name                 = "saigai-gha-deploy"
  assume_role_policy   = data.aws_iam_policy_document.gha_deploy_trust.json
  max_session_duration = 3600
}

# IAM 以外の操作（Lambda・DynamoDB・S3・CloudFront など）
resource "aws_iam_role_policy_attachment" "gha_deploy_poweruser" {
  role       = aws_iam_role.gha_deploy.name
  policy_arn = "arn:aws:iam::aws:policy/PowerUserAccess"
}

resource "aws_iam_role_policy" "gha_deploy_iam" {
  name   = "scoped-iam"
  role   = aws_iam_role.gha_deploy.id
  policy = data.aws_iam_policy_document.gha_deploy_iam.json
}

resource "aws_iam_role_policy" "gha_deploy_guard" {
  name   = "guard"
  role   = aws_iam_role.gha_deploy.id
  policy = data.aws_iam_policy_document.deploy_guard.json
}
