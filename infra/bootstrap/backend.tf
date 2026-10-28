# 最初の apply が終わったら、このファイルを backend.tf に名前を変えて、state を S3 に移す（README.md の手順 4）。
# バケット名（アカウント ID を含む）は公開リポジトリに載せないため、init のときに -backend-config で渡す。
terraform {
  backend "s3" {
    key          = "bootstrap/terraform.tfstate"
    region       = "ap-northeast-1"
    encrypt      = true
    use_lockfile = true
  }
}
