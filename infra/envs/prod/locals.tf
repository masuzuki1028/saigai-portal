locals {
  prefix     = "saigai-prod"
  app_domain = "${var.app_subdomain}.${var.root_domain}"
  app_origin = "https://${local.app_domain}"
  mail_from  = "notify@${local.app_domain}"
  # サンドボックスの間は確認済みのアドレスにしか送れないため、送信（通知・Cognito）は本番利用の承認後にだけ切り替える
  ses_identity = var.ses_mode != "off"
  ses_sending  = var.ses_mode == "production"

  tags = {
    Project   = "saigai"
    Env       = "prod"
    ManagedBy = "terraform"
  }

  boundary_arn = "arn:aws:iam::${var.account_id}:policy/${var.permissions_boundary_name}"
}

data "aws_route53_zone" "root" {
  name         = var.root_domain
  private_zone = false
}
