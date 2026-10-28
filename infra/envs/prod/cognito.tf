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

  # SES を有効にするまでは Cognito の標準の送信（1日 50 通まで）を使う
  email_configuration {
    email_sending_account = var.enable_ses ? "DEVELOPER" : "COGNITO_DEFAULT"
    source_arn            = var.enable_ses ? aws_sesv2_email_identity.app[0].arn : null
    from_email_address    = var.enable_ses ? "saigai-timeline <${local.mail_from}>" : null
  }

  user_attribute_update_settings {
    attributes_require_verification_before_update = ["email"]
  }

  admin_create_user_config {
    allow_admin_create_user_only = false
  }

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
