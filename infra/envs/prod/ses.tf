# メールの送信（docs/07 3.6）。ドメインの認証（DKIM）・SPF（MAIL FROM）・DMARC を設定する。
# 1. ses_mode = "identity" でドメインの認証を作り、SES の本番利用（サンドボックスの解除）を申請する
# 2. 承認されたら ses_mode = "production" にして、通知メールと Cognito の確認メールを SES から送る
# バウンス（宛先不明）と苦情があった宛先には、以後は送らない。
# アカウント全体の抑制リストの設定はほかのプロジェクトと共有しているため変えず、このドメインの送信だけに設定セットで付ける
resource "aws_sesv2_configuration_set" "app" {
  count                  = local.ses_identity ? 1 : 0
  configuration_set_name = "${local.prefix}-mail"

  suppression_options {
    suppressed_reasons = ["BOUNCE", "COMPLAINT"]
  }

  delivery_options {
    tls_policy = "REQUIRE"
  }
}

resource "aws_sesv2_email_identity" "app" {
  count                  = local.ses_identity ? 1 : 0
  email_identity         = local.app_domain
  configuration_set_name = aws_sesv2_configuration_set.app[0].configuration_set_name
}

resource "aws_route53_record" "ses_dkim" {
  count   = local.ses_identity ? 3 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "${aws_sesv2_email_identity.app[0].dkim_signing_attributes[0].tokens[count.index]}._domainkey.${local.app_domain}"
  type    = "CNAME"
  ttl     = 1800
  records = ["${aws_sesv2_email_identity.app[0].dkim_signing_attributes[0].tokens[count.index]}.dkim.amazonses.com"]
}

resource "aws_sesv2_email_identity_mail_from_attributes" "app" {
  count                  = local.ses_identity ? 1 : 0
  email_identity         = aws_sesv2_email_identity.app[0].email_identity
  mail_from_domain       = "mail.${local.app_domain}"
  behavior_on_mx_failure = "USE_DEFAULT_VALUE"
}

resource "aws_route53_record" "ses_mail_from_mx" {
  count   = local.ses_identity ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "mail.${local.app_domain}"
  type    = "MX"
  ttl     = 1800
  records = ["10 feedback-smtp.${var.region}.amazonses.com"]
}

resource "aws_route53_record" "ses_mail_from_spf" {
  count   = local.ses_identity ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "mail.${local.app_domain}"
  type    = "TXT"
  ttl     = 1800
  records = ["v=spf1 include:amazonses.com ~all"]
}

resource "aws_route53_record" "ses_dmarc" {
  count   = local.ses_identity ? 1 : 0
  zone_id = data.aws_route53_zone.root.zone_id
  name    = "_dmarc.${local.app_domain}"
  type    = "TXT"
  ttl     = 1800
  records = ["v=DMARC1; p=quarantine; adkim=s; aspf=r"]
}

# Cognito がこのドメインの送信元でメールを送ることを許可する（API で DEVELOPER にするときは自分で付ける。
# 付けないと、Cognito の設定は成功するのにメールが届かない）。
# ユーザープールがこのポリシーに依存するため、ユーザープールの ARN ではなくアカウントで絞る
data "aws_iam_policy_document" "ses_cognito" {
  statement {
    sid       = "AllowCognitoSend"
    actions   = ["ses:SendEmail", "ses:SendRawEmail"]
    resources = local.ses_identity ? [aws_sesv2_email_identity.app[0].arn] : ["*"]
    principals {
      type        = "Service"
      identifiers = ["cognito-idp.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [var.account_id]
    }
    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = ["arn:aws:cognito-idp:${var.region}:${var.account_id}:userpool/*"]
    }
  }
}

resource "aws_sesv2_email_identity_policy" "cognito" {
  count          = local.ses_sending ? 1 : 0
  email_identity = aws_sesv2_email_identity.app[0].email_identity
  policy_name    = "cognito-send"
  policy         = data.aws_iam_policy_document.ses_cognito.json
}
