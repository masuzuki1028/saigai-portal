terraform {
  required_version = ">= 1.16.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.67.0"
    }
  }
}

provider "aws" {
  region = var.region
  # 誤って別のアカウントに作らないようにする
  allowed_account_ids = [var.account_id]

  default_tags {
    tags = {
      Project   = "saigai"
      ManagedBy = "terraform"
      Stack     = "bootstrap"
    }
  }
}
