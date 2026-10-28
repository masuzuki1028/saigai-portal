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

variable "enable_ses" {
  description = "SES でメールを送る（ドメインの認証と本番利用の申請のあとで true にする）。false の間は通知メールを送らずに記録だけ残す"
  type        = bool
  default     = false
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
