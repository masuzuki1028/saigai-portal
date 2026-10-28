# DynamoDB のテーブル（docs/04 5章。定義は api/internal/store/tables.json と合わせる）
locals {
  tables = {
    app      = { pitr = true, ttl = false, gsi1 = true }
    events   = { pitr = false, ttl = true, gsi1 = false }
    timeline = { pitr = false, ttl = true, gsi1 = false }
    audit    = { pitr = true, ttl = true, gsi1 = false }
  }
}

resource "aws_dynamodb_table" "this" {
  for_each = local.tables

  name                        = "${local.prefix}-${each.key}"
  billing_mode                = "PAY_PER_REQUEST"
  hash_key                    = "PK"
  range_key                   = "SK"
  deletion_protection_enabled = true

  attribute {
    name = "PK"
    type = "S"
  }
  attribute {
    name = "SK"
    type = "S"
  }

  dynamic "attribute" {
    for_each = each.value.gsi1 ? ["GSI1PK", "GSI1SK"] : []
    content {
      name = attribute.value
      type = "S"
    }
  }

  dynamic "global_secondary_index" {
    for_each = each.value.gsi1 ? [1] : []
    content {
      name            = "GSI1"
      projection_type = "ALL"
      key_schema {
        attribute_name = "GSI1PK"
        key_type       = "HASH"
      }
      key_schema {
        attribute_name = "GSI1SK"
        key_type       = "RANGE"
      }
    }
  }

  ttl {
    enabled        = each.value.ttl
    attribute_name = each.value.ttl ? "ttl" : ""
  }

  point_in_time_recovery {
    enabled = each.value.pitr
  }

  server_side_encryption {
    enabled = true # AWS 管理のキー（追加費用なし）。座標・住所はアプリ層でも KMS で暗号化している
  }

  lifecycle {
    prevent_destroy = true
  }
}
