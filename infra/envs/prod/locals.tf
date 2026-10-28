locals {
  prefix     = "saigai-prod"
  app_domain = "${var.app_subdomain}.${var.root_domain}"
  app_origin = "https://${local.app_domain}"
  # SES を有効にするまでは、Cognito の標準の送信元と同じ扱いにする（通知メールは送らない）
  mail_from = "notify@${local.app_domain}"

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
