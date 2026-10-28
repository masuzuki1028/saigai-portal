output "app_url" {
  value = local.app_origin
}

output "web_bucket" {
  description = "画面の静的ファイルを置くバケット（deploy-prod.yml が使う）"
  value       = aws_s3_bucket.web.bucket
}

output "cloudfront_distribution_id" {
  value = aws_cloudfront_distribution.web.id
}

output "function_names" {
  value = { for k, f in aws_lambda_function.fn : k => f.function_name }
}

output "cognito" {
  description = "画面のビルドに渡す値（NEXT_PUBLIC_COGNITO_*）。秘密の値ではない"
  value = {
    user_pool_id = aws_cognito_user_pool.this.id
    client_id    = aws_cognito_user_pool_client.web.id
    login_origin = local.cognito_login_origin
  }
}
