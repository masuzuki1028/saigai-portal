# 運営者（人）の操作。IAM ユーザーはロールを引き受けることしかできず、
# 普段は saigai-readonly、最初の構築と緊急時だけ saigai-admin を使う（docs/07 3.4）
data "aws_iam_user" "operator" {
  user_name = var.operator_user_name
}

data "aws_iam_policy_document" "operator_trust" {
  statement {
    actions = ["sts:AssumeRole", "sts:SetSourceIdentity"]
    principals {
      type        = "AWS"
      identifiers = [data.aws_iam_user.operator.arn]
    }
    dynamic "condition" {
      for_each = var.require_mfa ? [1] : []
      content {
        test     = "Bool"
        variable = "aws:MultiFactorAuthPresent"
        values   = ["true"]
      }
    }
  }
}

resource "aws_iam_role" "readonly" {
  name                 = "saigai-readonly"
  assume_role_policy   = data.aws_iam_policy_document.operator_trust.json
  max_session_duration = 43200
}

resource "aws_iam_role_policy_attachment" "readonly" {
  role       = aws_iam_role.readonly.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

resource "aws_iam_role_policy" "readonly_deny_data" {
  name   = "deny-data-read"
  role   = aws_iam_role.readonly.id
  policy = data.aws_iam_policy_document.deny_data_read.json
}

resource "aws_iam_role" "admin" {
  name                 = "saigai-admin"
  assume_role_policy   = data.aws_iam_policy_document.operator_trust.json
  max_session_duration = 3600
}

resource "aws_iam_role_policy_attachment" "admin" {
  role       = aws_iam_role.admin.name
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
}

# IAM ユーザー本体の権限: 2つのロールの引き受けと、自分のパスワード・MFA の管理だけ
data "aws_iam_policy_document" "operator_user" {
  statement {
    sid       = "AssumeOperatorRoles"
    actions   = ["sts:AssumeRole", "sts:SetSourceIdentity"]
    resources = [aws_iam_role.readonly.arn, aws_iam_role.admin.arn]
  }
  statement {
    sid = "ManageOwnCredentials"
    actions = [
      "iam:GetUser",
      "iam:ChangePassword",
      "iam:GetLoginProfile",
      "iam:ListMFADevices",
      "iam:ListVirtualMFADevices",
      "iam:CreateVirtualMFADevice",
      "iam:EnableMFADevice",
      "iam:ResyncMFADevice",
    ]
    resources = [data.aws_iam_user.operator.arn, "arn:aws:iam::${var.account_id}:mfa/*"]
  }
  statement {
    sid       = "GetAccountPasswordPolicy"
    actions   = ["iam:GetAccountPasswordPolicy"]
    resources = ["*"]
  }
}

resource "aws_iam_user_policy" "operator" {
  name   = "saigai-operator"
  user   = data.aws_iam_user.operator.user_name
  policy = data.aws_iam_policy_document.operator_user.json
}
