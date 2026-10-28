# 読み取り専用のロールでも、本番のデータ（DynamoDB の項目・暗号化したデータ・秘密情報）は読めないようにする
# （docs/07 3.4「本番データの閲覧は原則禁止」）。state のバケットだけは読める
data "aws_iam_policy_document" "deny_data_read" {
  statement {
    sid    = "DenyDataPlaneRead"
    effect = "Deny"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:BatchGetItem",
      "dynamodb:Query",
      "dynamodb:Scan",
      "dynamodb:GetRecords",
      "dynamodb:PartiQLSelect",
      "secretsmanager:GetSecretValue",
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["*"]
  }
  # 復号は拒否する。ただし Lambda を通した復号（関数の設定を見るときの環境変数。AWS 管理のキーで暗号化されている）は許可する。
  # 環境変数にはテーブル名やキューの URL しか入れない。利用者のデータ（アプリ層で暗号化した座標・住所）の復号は
  # 直接 KMS を呼ぶため、この拒否にかかる
  statement {
    sid       = "DenyDecryptExceptLambdaConfig"
    effect    = "Deny"
    actions   = ["kms:Decrypt"]
    resources = ["*"]
    condition {
      test     = "StringNotLike"
      variable = "kms:ViaService"
      values   = ["lambda.*.amazonaws.com"]
    }
  }
  statement {
    sid           = "DenyObjectReadExceptState"
    effect        = "Deny"
    actions       = ["s3:GetObject", "s3:GetObjectVersion"]
    not_resources = ["${aws_s3_bucket.tfstate.arn}/*"]
  }
}

# CI のデプロイ用ロールの IAM の操作は、本番のリソースの接頭辞の付いたロール・ポリシーだけにし、
# 作るロールには必ず権限の上限（saigai-prod-boundary）を付けさせる
data "aws_iam_policy_document" "gha_deploy_iam" {
  statement {
    sid = "ManagePrefixedRoles"
    actions = [
      "iam:CreateRole",
      "iam:PutRolePermissionsBoundary",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:PutRolePolicy",
      "iam:DeleteRolePolicy",
    ]
    resources = ["arn:aws:iam::${var.account_id}:role/${var.resource_prefix}-*"]
    condition {
      test     = "StringEquals"
      variable = "iam:PermissionsBoundary"
      values   = [aws_iam_policy.boundary.arn]
    }
  }
  statement {
    sid = "ManagePrefixedRolesNoBoundaryChange"
    actions = [
      "iam:DeleteRole",
      "iam:UpdateRole",
      "iam:UpdateAssumeRolePolicy",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:GetRole",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:ListAttachedRolePolicies",
      "iam:ListInstanceProfilesForRole",
    ]
    resources = ["arn:aws:iam::${var.account_id}:role/${var.resource_prefix}-*"]
  }
  statement {
    sid = "ManagePrefixedPolicies"
    actions = [
      "iam:CreatePolicy",
      "iam:DeletePolicy",
      "iam:CreatePolicyVersion",
      "iam:DeletePolicyVersion",
      "iam:GetPolicy",
      "iam:GetPolicyVersion",
      "iam:ListPolicyVersions",
      "iam:TagPolicy",
      "iam:UntagPolicy",
    ]
    resources = ["arn:aws:iam::${var.account_id}:policy/${var.resource_prefix}-*"]
  }
  statement {
    sid       = "PassPrefixedRoles"
    actions   = ["iam:PassRole"]
    resources = ["arn:aws:iam::${var.account_id}:role/${var.resource_prefix}-*"]
  }
  statement {
    sid       = "ServiceLinkedRoles"
    actions   = ["iam:CreateServiceLinkedRole"]
    resources = ["arn:aws:iam::${var.account_id}:role/aws-service-role/*"]
  }
  statement {
    sid       = "StateBucket"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
    resources = [aws_s3_bucket.tfstate.arn, "${aws_s3_bucket.tfstate.arn}/*"]
  }
}

# デプロイ用ロールが、自分自身や bootstrap のリソースを変えられないようにする
data "aws_iam_policy_document" "deploy_guard" {
  statement {
    sid     = "DenyBootstrapChanges"
    effect  = "Deny"
    actions = ["iam:*"]
    resources = [
      aws_iam_role.gha_plan.arn,
      aws_iam_role.gha_deploy.arn,
      aws_iam_role.readonly.arn,
      aws_iam_role.admin.arn,
      aws_iam_policy.boundary.arn,
      local.github_oidc_arn,
      data.aws_iam_user.operator.arn,
    ]
  }
  statement {
    sid       = "DenyStateBucketPolicyChanges"
    effect    = "Deny"
    actions   = ["s3:DeleteBucket", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy", "s3:PutBucketVersioning", "s3:PutLifecycleConfiguration"]
    resources = [aws_s3_bucket.tfstate.arn]
  }
  statement {
    sid       = "DenyAccountLevel"
    effect    = "Deny"
    actions   = ["organizations:*", "account:*", "iam:CreateUser", "iam:CreateAccessKey", "iam:CreateLoginProfile"]
    resources = ["*"]
  }
}

# 本番のリソースのロール（Lambda の実行ロールなど）に付ける権限の上限
data "aws_iam_policy_document" "boundary" {
  statement {
    sid       = "AllowWithinServices"
    actions   = ["*"]
    resources = ["*"]
  }
  statement {
    sid    = "DenyIdentityAndAccount"
    effect = "Deny"
    actions = [
      "iam:*",
      "organizations:*",
      "account:*",
      "sts:AssumeRole",
    ]
    resources = ["*"]
  }
  statement {
    sid       = "DenyStateBucket"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.tfstate.arn, "${aws_s3_bucket.tfstate.arn}/*"]
  }
}

resource "aws_iam_policy" "boundary" {
  name        = "${var.resource_prefix}-boundary"
  description = "saigai の本番のリソースのロールに付ける権限の上限"
  policy      = data.aws_iam_policy_document.boundary.json
}
