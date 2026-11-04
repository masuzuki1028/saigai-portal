# 公開リポジトリに載せない値（アカウント ID・ドメイン・メールアドレス）は terraform.tfvars（.gitignore 済み）か
# 環境変数 TF_VAR_*（CI では Secrets）で渡す

variable "account_id" {
  description = "AWS アカウント ID（12桁）"
  type        = string
  validation {
    condition     = can(regex("^[0-9]{12}$", var.account_id))
    error_message = "12桁の数字で指定してください。"
  }
}

variable "region" {
  description = "メインのリージョン"
  type        = string
  default     = "ap-northeast-1"
}

variable "root_domain" {
  description = "Route 53 で管理しているドメイン（ホストゾーン）。作品ごとにサブドメインを切る"
  type        = string
}

variable "app_subdomain" {
  description = "このサービスのサブドメイン（<app_subdomain>.<root_domain>）"
  type        = string
  default     = "saigai-portal"
}

variable "alarm_email" {
  description = "監視のアラームの通知先（SNS のメール。購読の確認メールが届く）"
  type        = string
}

variable "ses_mode" {
  description = <<-EOT
    SES の使い方（ses.tf）。
    off: 使わない（通知メールは送らずに記録だけ残す。Cognito の確認メールは Cognito の標準の送信）
    identity: ドメインの認証だけを作る。SES の本番利用（サンドボックスの解除）を申請できる状態にする。送信は off と同じ
    production: 本番利用が承認されてから。通知メールと Cognito の確認メールを SES から送る
  EOT
  type        = string
  default     = "off"

  validation {
    condition     = contains(["off", "identity", "production"], var.ses_mode)
    error_message = "ses_mode は off・identity・production のどれかにしてください"
  }
}

variable "permissions_boundary_name" {
  description = "Lambda などのロールに付ける権限の上限（infra/bootstrap で作成）"
  type        = string
  default     = "saigai-prod-boundary"
}

variable "ingest_enabled" {
  description = "取り込みの定期実行を動かす（最初のデプロイが終わってから true にする）"
  type        = bool
  default     = false
}

variable "ingest_interval_minutes" {
  description = "取り込みの間隔（分）。公開前は費用を抑えて5分、公開したら1分にする。気象庁の高頻度フィードには直近約10分の電文しか載らないため、5分より長くしない"
  type        = number
  default     = 5

  validation {
    condition     = var.ingest_interval_minutes >= 1 && var.ingest_interval_minutes <= 5 && floor(var.ingest_interval_minutes) == var.ingest_interval_minutes
    error_message = "ingest_interval_minutes は 1〜5 の整数にしてください"
  }
}
