# 画面の配信（docs/05 3.1）。S3（非公開）+ CloudFront（OAC）。/api/* は API Gateway に振り分け、同一オリジンにする

resource "aws_s3_bucket" "web" {
  bucket = "${local.prefix}-web-${var.account_id}"
}

resource "aws_s3_bucket_versioning" "web" {
  bucket = aws_s3_bucket.web.id
  versioning_configuration {
    status = "Enabled" # ロールバックで前の版に戻せるようにする（docs/06 6.2）
  }
}

#trivy:ignore:AVD-AWS-0132 静的な画面のファイル（公開する内容）のため、費用のかかる KMS のキーは使わない
resource "aws_s3_bucket_server_side_encryption_configuration" "web" {
  bucket = aws_s3_bucket.web.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "web" {
  bucket                  = aws_s3_bucket.web.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "web" {
  bucket = aws_s3_bucket.web.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "web" {
  bucket = aws_s3_bucket.web.id
  rule {
    id     = "expire-noncurrent"
    status = "Enabled"
    filter {}
    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

data "aws_iam_policy_document" "web_bucket" {
  statement {
    sid       = "CloudFrontOnly"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.web.arn}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.web.arn]
    }
  }
  statement {
    sid       = "DenyInsecureTransport"
    effect    = "Deny"
    actions   = ["s3:*"]
    resources = [aws_s3_bucket.web.arn, "${aws_s3_bucket.web.arn}/*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "web" {
  bucket = aws_s3_bucket.web.id
  policy = data.aws_iam_policy_document.web_bucket.json
}

resource "aws_cloudfront_origin_access_control" "web" {
  name                              = "${local.prefix}-web"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# --- 証明書（CloudFront 用は us-east-1） ---
resource "aws_acm_certificate" "web" {
  provider          = aws.us_east_1
  domain_name       = local.app_domain
  validation_method = "DNS"
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "web_cert" {
  for_each = {
    for o in aws_acm_certificate.web.domain_validation_options : o.domain_name => o
  }
  zone_id         = data.aws_route53_zone.root.zone_id
  name            = each.value.resource_record_name
  type            = each.value.resource_record_type
  records         = [each.value.resource_record_value]
  ttl             = 300
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "web" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.web.arn
  validation_record_fqdns = [for r in aws_route53_record.web_cert : r.fqdn]
}

# --- 拡張子のないパスを .html に書き換える（Next.js の静的出力。docs/06 6.2） ---
resource "aws_cloudfront_function" "rewrite" {
  name    = "${local.prefix}-rewrite"
  runtime = "cloudfront-js-2.0"
  comment = "/login -> /login.html, / -> /index.html"
  publish = true
  code    = <<-JS
    function handler(event) {
      var req = event.request;
      var uri = req.uri;
      if (uri === "/" || uri === "") {
        req.uri = "/index.html";
      } else if (uri.endsWith("/")) {
        req.uri = uri.slice(0, -1) + ".html";
      } else if (uri.lastIndexOf(".") <= uri.lastIndexOf("/")) {
        req.uri = uri + ".html";
      }
      return req;
    }
  JS
}

# --- セキュリティヘッダー（docs/07 3.6） ---
locals {
  cognito_login_origin = "https://${aws_cognito_user_pool_domain.this.domain}.auth.${var.region}.amazoncognito.com"
  csp = join("; ", [
    "default-src 'self'",
    # Next.js の静的出力はインラインのスクリプトを使うため 'unsafe-inline' が必要（nonce は静的出力では使えない）。
    # React のエスケープと dangerouslySetInnerHTML の禁止（lint）で XSS を防ぐ
    "script-src 'self' 'unsafe-inline'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https://cyberjapandata.gsi.go.jp",
    "font-src 'self'",
    "connect-src 'self' https://msearch.gsi.go.jp https://cyberjapandata.gsi.go.jp ${local.cognito_login_origin}",
    "worker-src 'self' blob:",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self' ${local.cognito_login_origin}",
    "object-src 'none'",
    "upgrade-insecure-requests",
  ])
}

resource "aws_cloudfront_response_headers_policy" "security" {
  name = "${local.prefix}-security"
  security_headers_config {
    content_security_policy {
      content_security_policy = local.csp
      override                = true
    }
    strict_transport_security {
      access_control_max_age_sec = 31536000
      include_subdomains         = true
      override                   = true
    }
    frame_options {
      frame_option = "DENY"
      override     = true
    }
    content_type_options {
      override = true
    }
    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = true
    }
  }
  custom_headers_config {
    items {
      header   = "Permissions-Policy"
      value    = "geolocation=(self), camera=(), microphone=(), payment=()"
      override = true
    }
  }
}

data "aws_cloudfront_cache_policy" "optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_except_host" {
  name = "Managed-AllViewerExceptHostHeader"
}

#trivy:ignore:AVD-AWS-0010 アクセスログは費用を抑えるため出さない（API Gateway のアクセスログで代替）
#trivy:ignore:AVD-AWS-0011 WAF は Phase 2 で導入する（docs/07 7章）
resource "aws_cloudfront_distribution" "web" {
  enabled             = true
  is_ipv6_enabled     = true
  http_version        = "http2and3"
  price_class         = "PriceClass_200" # 日本を含む地域。全地域より安い
  aliases             = [local.app_domain]
  default_root_object = "index.html"
  comment             = "saigai prod"

  origin {
    origin_id                = "web"
    domain_name              = aws_s3_bucket.web.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.web.id
  }

  origin {
    origin_id   = "api"
    domain_name = replace(aws_apigatewayv2_api.this.api_endpoint, "https://", "")
    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id           = "web"
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = data.aws_cloudfront_cache_policy.optimized.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.rewrite.arn
    }
  }

  ordered_cache_behavior {
    path_pattern               = "/api/*"
    target_origin_id           = "api"
    viewer_protocol_policy     = "https-only"
    allowed_methods            = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    cache_policy_id            = data.aws_cloudfront_cache_policy.disabled.id
    origin_request_policy_id   = data.aws_cloudfront_origin_request_policy.all_except_host.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.security.id
  }

  custom_error_response {
    error_code            = 404
    response_code         = 404
    response_page_path    = "/404.html"
    error_caching_min_ttl = 60
  }

  # S3 は存在しないキーに 403 を返すため、404 として扱う
  custom_error_response {
    error_code            = 403
    response_code         = 404
    response_page_path    = "/404.html"
    error_caching_min_ttl = 60
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate_validation.web.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }
}

resource "aws_route53_record" "web" {
  for_each = toset(["A", "AAAA"])
  zone_id  = data.aws_route53_zone.root.zone_id
  name     = local.app_domain
  type     = each.value
  alias {
    name                   = aws_cloudfront_distribution.web.domain_name
    zone_id                = aws_cloudfront_distribution.web.hosted_zone_id
    evaluate_target_health = false
  }
}
