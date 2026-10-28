# Lambda 関数（すべて Go、provided.al2023、arm64。docs/05 3.2）。
# コードは CI がデプロイするため、Terraform は最初の仮のコードだけを置き、以後のコードの変更を無視する（docs/06 6.1）
locals {
  functions = {
    api          = { timeout = 10, memory = 256, concurrency = -1, description = "REST API" }
    ingest-jma   = { timeout = 50, memory = 256, concurrency = 1, description = "気象庁 防災情報XML の取り込み" }
    ingest-quake = { timeout = 50, memory = 256, concurrency = 1, description = "P2P地震情報の取り込み" }
    matcher      = { timeout = 60, memory = 256, concurrency = -1, description = "公式情報と地点の突き合わせ" }
    notifier     = { timeout = 30, memory = 256, concurrency = -1, description = "通知の送信" }
  }

  table_arns = { for k, t in aws_dynamodb_table.this : k => t.arn }

  common_env = {
    TABLE_PREFIX = local.prefix
    APP_BASE_URL = local.app_origin
    LOG_LEVEL    = "INFO"
  }
}

data "archive_file" "placeholder" {
  type        = "zip"
  source_file = "${path.module}/placeholder/bootstrap"
  output_path = "${path.module}/.build/placeholder.zip"
}

data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "fn" {
  for_each             = local.functions
  name                 = "${local.prefix}-${each.key}"
  assume_role_policy   = data.aws_iam_policy_document.lambda_assume.json
  permissions_boundary = local.boundary_arn
}

resource "aws_cloudwatch_log_group" "fn" {
  for_each          = local.functions
  name              = "/aws/lambda/${local.prefix}-${each.key}"
  retention_in_days = 30
}

# ログの書き込み（関数ごとのロググループだけ）
data "aws_iam_policy_document" "fn_logs" {
  for_each = local.functions
  statement {
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.fn[each.key].arn}:*"]
  }
}

resource "aws_iam_role_policy" "fn_logs" {
  for_each = local.functions
  name     = "logs"
  role     = aws_iam_role.fn[each.key].id
  policy   = data.aws_iam_policy_document.fn_logs[each.key].json
}

resource "aws_lambda_function" "fn" {
  for_each = local.functions

  function_name                  = "${local.prefix}-${each.key}"
  description                    = each.value.description
  role                           = aws_iam_role.fn[each.key].arn
  runtime                        = "provided.al2023"
  architectures                  = ["arm64"]
  handler                        = "bootstrap"
  filename                       = data.archive_file.placeholder.output_path
  timeout                        = each.value.timeout
  memory_size                    = each.value.memory
  reserved_concurrent_executions = each.value.concurrency
  publish                        = false

  environment {
    variables = merge(local.common_env, local.function_env[each.key])
  }

  logging_config {
    log_format = "JSON"
    log_group  = aws_cloudwatch_log_group.fn[each.key].name
  }

  lifecycle {
    # コードは CI がデプロイする（Terraform では管理しない）
    ignore_changes = [filename, source_code_hash, s3_bucket, s3_key, s3_object_version]
  }

  depends_on = [aws_iam_role_policy.fn_logs]
}

# 呼び出し元（API Gateway・SQS・Scheduler）は live エイリアスを呼ぶ。CI が新しいバージョンを公開して切り替える
resource "aws_lambda_alias" "live" {
  for_each         = local.functions
  name             = "live"
  function_name    = aws_lambda_function.fn[each.key].function_name
  function_version = "$LATEST"

  lifecycle {
    ignore_changes = [function_version, routing_config]
  }
}

locals {
  function_env = {
    api = {
      KMS_KEY_ID           = aws_kms_alias.app.arn
      COGNITO_USER_POOL_ID = aws_cognito_user_pool.this.id
      COGNITO_ISSUER       = "https://cognito-idp.${var.region}.amazonaws.com/${aws_cognito_user_pool.this.id}"
      MAIL_FROM            = local.mail_from
      SES_ENABLED          = tostring(var.enable_ses)
    }
    ingest-jma = {
      MATCH_QUEUE_URL = aws_sqs_queue.this["match"].url
    }
    ingest-quake = {
      MATCH_QUEUE_URL = aws_sqs_queue.this["match"].url
    }
    matcher = {
      NOTIFY_QUEUE_URL = aws_sqs_queue.this["notify"].url
    }
    notifier = {
      COGNITO_USER_POOL_ID = aws_cognito_user_pool.this.id
      MAIL_FROM            = local.mail_from
      SES_ENABLED          = tostring(var.enable_ses)
    }
  }
}

# --- 関数ごとの権限（最小権限。docs/07 3.3） ---

data "aws_iam_policy_document" "api" {
  statement {
    sid = "AppAndTimeline"
    actions = [
      "dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem",
      "dynamodb:Query", "dynamodb:BatchWriteItem", "dynamodb:ConditionCheckItem",
    ]
    resources = [local.table_arns["app"], "${local.table_arns["app"]}/index/*", local.table_arns["timeline"]]
  }
  statement {
    sid       = "Transactions"
    actions   = ["dynamodb:TransactWriteItems"]
    resources = ["*"]
  }
  statement {
    sid       = "EncryptLocations"
    actions   = ["kms:Encrypt", "kms:Decrypt", "kms:GenerateDataKey"]
    resources = [aws_kms_key.app.arn]
  }
  statement {
    sid       = "LookupEmail"
    actions   = ["cognito-idp:AdminGetUser", "cognito-idp:AdminDeleteUser"]
    resources = [aws_cognito_user_pool.this.arn]
  }
  dynamic "statement" {
    for_each = var.enable_ses ? [1] : []
    content {
      sid       = "SendTestMail"
      actions   = ["ses:SendEmail"]
      resources = ["*"]
      condition {
        test     = "StringEquals"
        variable = "ses:FromAddress"
        values   = [local.mail_from]
      }
    }
  }
}

data "aws_iam_policy_document" "ingest" {
  statement {
    sid       = "Events"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem"]
    resources = [local.table_arns["events"]]
  }
  statement {
    sid       = "Cursor"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem"]
    resources = [local.table_arns["app"]]
    condition {
      test     = "ForAllValues:StringLike"
      variable = "dynamodb:LeadingKeys"
      values   = ["INGEST#*"]
    }
  }
  statement {
    sid       = "EnqueueMatch"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.this["match"].arn]
  }
}

data "aws_iam_policy_document" "matcher" {
  statement {
    sid       = "AreaIndex"
    actions   = ["dynamodb:Query"]
    resources = [local.table_arns["app"]]
    condition {
      test     = "ForAllValues:StringLike"
      variable = "dynamodb:LeadingKeys"
      values   = ["AREA#*"]
    }
  }
  statement {
    sid       = "Timeline"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:Query"]
    resources = [local.table_arns["timeline"]]
  }
  statement {
    sid       = "EventsIndex"
    actions   = ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:Query"]
    resources = [local.table_arns["events"]]
  }
  statement {
    sid       = "Queues"
    actions   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
    resources = [aws_sqs_queue.this["match"].arn]
  }
  statement {
    sid       = "EnqueueNotify"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.this["notify"].arn]
  }
}

data "aws_iam_policy_document" "notifier" {
  statement {
    sid       = "Users"
    actions   = ["dynamodb:GetItem"]
    resources = [local.table_arns["app"]]
    condition {
      test     = "ForAllValues:StringLike"
      variable = "dynamodb:LeadingKeys"
      values   = ["USER#*"]
    }
  }
  statement {
    sid       = "LookupEmail"
    actions   = ["cognito-idp:AdminGetUser"]
    resources = [aws_cognito_user_pool.this.arn]
  }
  statement {
    sid       = "Queue"
    actions   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
    resources = [aws_sqs_queue.this["notify"].arn]
  }
  dynamic "statement" {
    for_each = var.enable_ses ? [1] : []
    content {
      sid       = "SendMail"
      actions   = ["ses:SendEmail"]
      resources = ["*"]
      condition {
        test     = "StringEquals"
        variable = "ses:FromAddress"
        values   = [local.mail_from]
      }
    }
  }
}

locals {
  function_policies = {
    api          = data.aws_iam_policy_document.api.json
    ingest-jma   = data.aws_iam_policy_document.ingest.json
    ingest-quake = data.aws_iam_policy_document.ingest.json
    matcher      = data.aws_iam_policy_document.matcher.json
    notifier     = data.aws_iam_policy_document.notifier.json
  }
}

resource "aws_iam_role_policy" "fn" {
  for_each = local.functions
  name     = "app"
  role     = aws_iam_role.fn[each.key].id
  policy   = local.function_policies[each.key]
}

# --- SQS からの起動（matcher・notifier） ---
resource "aws_lambda_event_source_mapping" "queue" {
  for_each = {
    matcher  = "match"
    notifier = "notify"
  }
  event_source_arn        = aws_sqs_queue.this[each.value].arn
  function_name           = aws_lambda_alias.live[each.key].arn
  batch_size              = 10
  function_response_types = ["ReportBatchItemFailures"]
  scaling_config {
    maximum_concurrency = 10
  }
}
