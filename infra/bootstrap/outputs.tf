output "state_bucket" {
  description = "Terraform の state のバケット（infra/envs/prod と bootstrap の backend に指定する）"
  value       = aws_s3_bucket.tfstate.bucket
}

output "gha_plan_role_arn" {
  description = "GitHub の Repository secrets の PLAN_ROLE_ARN に登録する"
  value       = aws_iam_role.gha_plan.arn
}

output "gha_deploy_role_arn" {
  description = "GitHub の Environment「prod」の Environment secrets の DEPLOY_ROLE_ARN に登録する"
  value       = aws_iam_role.gha_deploy.arn
}

output "permissions_boundary_arn" {
  description = "本番のリソースのロールに付ける権限の上限"
  value       = aws_iam_policy.boundary.arn
}

output "aws_config" {
  description = "~/.aws/config に追記する内容（saigai は aws login で作るプロファイル）。aws login の認証情報には MFA の印が付かないため、ロールを引き受けるときに認証アプリのコードを入力する（mfa_serial）"
  value       = <<-EOT
    [profile saigai-readonly]
    source_profile = saigai
    role_arn = ${aws_iam_role.readonly.arn}
    region = ${var.region}
    mfa_serial = <aws iam list-mfa-devices --user-name ${var.operator_user_name} --profile saigai の SerialNumber>
    duration_seconds = 3600

    [profile saigai-admin]
    source_profile = saigai
    role_arn = ${aws_iam_role.admin.arn}
    region = ${var.region}
    mfa_serial = <同上>
    duration_seconds = 3600

    # Terraform 用。Terraform は MFA のコードを入力できないため、AWS CLI が保存したセッションを借りる
    [profile saigai-readonly-tf]
    credential_process = aws configure export-credentials --profile saigai-readonly --format process
    region = ${var.region}

    [profile saigai-admin-tf]
    credential_process = aws configure export-credentials --profile saigai-admin --format process
    region = ${var.region}
  EOT
}
