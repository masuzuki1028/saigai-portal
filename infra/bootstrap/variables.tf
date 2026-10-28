variable "account_id" {
  description = "AWS アカウント ID（12桁）。公開リポジトリに載せないよう、terraform.tfvars（.gitignore 済み）か環境変数 TF_VAR_account_id で渡す"
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

variable "operator_user_name" {
  description = "運営者用の IAM ユーザー名（コンソールで作成済みのもの。パスワード＋MFA、アクセスキーなし）"
  type        = string
}

variable "github_repository" {
  description = "GitHub のリポジトリ（owner/name）"
  type        = string
  default     = "masuzuki1028/saigai-portal"
}

variable "create_github_oidc_provider" {
  description = "GitHub Actions の OIDC プロバイダを作る。アカウントにすでにある（ほかのプロジェクトで作成済み）なら false にして、既存のものを参照する"
  type        = bool
  default     = true
}

variable "require_mfa" {
  description = "運営者がロールを引き受けるときに MFA を必須にする。aws login の一時クレデンシャルで条件を満たせない場合だけ false にする（IAM ユーザーのログイン自体には MFA が必須）"
  type        = bool
  default     = true
}

variable "resource_prefix" {
  description = "本番のリソースの名前の接頭辞。CI のデプロイ用ロールが変更できる範囲をこの接頭辞に限定する"
  type        = string
  default     = "saigai-prod"
}
