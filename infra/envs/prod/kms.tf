# アプリ層の暗号化（正確な座標・住所）のキー（docs/07 3.5）。
# 後から変えられないため、最初からマルチリージョンキーにする（大阪のレプリカは DR の構築時に作る。docs/10 11章）
resource "aws_kms_key" "app" {
  description             = "saigai prod: アプリ層の暗号化（エンベロープ暗号化のデータキー）"
  multi_region            = true
  enable_key_rotation     = true
  deletion_window_in_days = 30

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_kms_alias" "app" {
  name          = "alias/${local.prefix}-app"
  target_key_id = aws_kms_key.app.key_id
}
