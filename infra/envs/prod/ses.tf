# メールの送信（docs/07 3.6）。ドメインの認証（DKIM）・SPF（MAIL FROM）・DMARC を設定する。
# 新しいドメインを取得してから enable_ses = true にする。そのあと SES の本番利用（サンドボックスの解除）を申請する
resource "aws_sesv2_email_identity" "app" {
  count          = var.enable_ses ? 1 : 0
  email_identity = local.app_domain
}

resource "aws_route53_record" "ses_dkim" {
  count   = var.enable_ses ? 3 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "${aws_sesv2_email_identity.app[0].dkim_signing_attributes[0].tokens[count.index]}._domainkey.${local.app_domain}"
  type    = "CNAME"
  ttl     = 1800
  records = ["${aws_sesv2_email_identity.app[0].dkim_signing_attributes[0].tokens[count.index]}.dkim.amazonses.com"]
}

resource "aws_sesv2_email_identity_mail_from_attributes" "app" {
  count                  = var.enable_ses ? 1 : 0
  email_identity         = aws_sesv2_email_identity.app[0].email_identity
  mail_from_domain       = "mail.${local.app_domain}"
  behavior_on_mx_failure = "USE_DEFAULT_VALUE"
}

resource "aws_route53_record" "ses_mail_from_mx" {
  count   = var.enable_ses ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "mail.${local.app_domain}"
  type    = "MX"
  ttl     = 1800
  records = ["10 feedback-smtp.${var.region}.amazonses.com"]
}

resource "aws_route53_record" "ses_mail_from_spf" {
  count   = var.enable_ses ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "mail.${local.app_domain}"
  type    = "TXT"
  ttl     = 1800
  records = ["v=spf1 include:amazonses.com ~all"]
}

resource "aws_route53_record" "ses_dmarc" {
  count   = var.enable_ses ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "_dmarc.${local.app_domain}"
  type    = "TXT"
  ttl     = 1800
  records = ["v=DMARC1; p=quarantine; adkim=s; aspf=r"]
}
