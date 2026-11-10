# 利用者の認証（docs/07 3.1）。Authorization Code + PKCE のマネージドログインを使う
resource "aws_cognito_user_pool" "this" {
  name                     = local.prefix
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  deletion_protection      = "ACTIVE"
  mfa_configuration        = "OPTIONAL"

  software_token_mfa_configuration {
    enabled = true
  }

  password_policy {
    minimum_length                   = 12
    require_lowercase                = true
    require_uppercase                = false
    require_numbers                  = true
    require_symbols                  = false
    temporary_password_validity_days = 3
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  # 新規登録の確認とパスワードの再設定は、同じ文面で確認コードを送る（Cognito の標準は英語）
  verification_message_template {
    default_email_option = "CONFIRM_WITH_CODE"
    email_subject        = "【saigai-timeline】確認コード"
    email_message        = <<-EOT
      saigai-timeline の確認コードです。

      確認コード: {####}

      画面に戻って、このコードを入力してください。
      心当たりがない場合は、このメールを無視してください。
    EOT
  }

  # SES の本番利用が承認されるまでは Cognito の標準の送信（1日 50 通まで）を使う（ses.tf）
  email_configuration {
    email_sending_account = local.ses_sending ? "DEVELOPER" : "COGNITO_DEFAULT"
    source_arn            = local.ses_sending ? aws_sesv2_email_identity.app[0].arn : null
    from_email_address    = local.ses_sending ? "saigai-timeline <${local.mail_from}>" : null
  }

  user_attribute_update_settings {
    attributes_require_verification_before_update = ["email"]
  }

  admin_create_user_config {
    allow_admin_create_user_only = false
  }

  # SES から送るときは、Cognito に送信を許可するポリシーを先に付ける
  depends_on = [aws_sesv2_email_identity_policy.cognito]

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_cognito_user_pool_client" "web" {
  name         = "${local.prefix}-web"
  user_pool_id = aws_cognito_user_pool.this.id

  generate_secret                      = false # ブラウザから使う（PKCE）
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email"]
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = ["${local.app_origin}/auth/callback"]
  logout_urls                          = ["${local.app_origin}/"]
  explicit_auth_flows                  = ["ALLOW_REFRESH_TOKEN_AUTH"]
  prevent_user_existence_errors        = "ENABLED"
  enable_token_revocation              = true

  access_token_validity  = 60
  id_token_validity      = 60
  refresh_token_validity = 30
  token_validity_units {
    access_token  = "minutes"
    id_token      = "minutes"
    refresh_token = "days"
  }
}

# マネージドログインのドメイン（プレフィックス方式。独自ドメインにすると us-east-1 の証明書が別に要る）
resource "aws_cognito_user_pool_domain" "this" {
  domain                = "${local.prefix}-${substr(sha256(var.account_id), 0, 8)}"
  user_pool_id          = aws_cognito_user_pool.this.id
  managed_login_version = 2
}

resource "aws_cognito_managed_login_branding" "web" {
  user_pool_id                = aws_cognito_user_pool.this.id
  client_id                   = aws_cognito_user_pool_client.web.id
  use_cognito_provided_values = true
}
