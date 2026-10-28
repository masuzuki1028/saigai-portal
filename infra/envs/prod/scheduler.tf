# 取り込みを1分ごとに起動する（docs/05 3.4）
data "aws_iam_policy_document" "scheduler_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [var.account_id]
    }
  }
}

resource "aws_iam_role" "scheduler" {
  name                 = "${local.prefix}-scheduler"
  assume_role_policy   = data.aws_iam_policy_document.scheduler_assume.json
  permissions_boundary = local.boundary_arn
}

data "aws_iam_policy_document" "scheduler" {
  statement {
    actions   = ["lambda:InvokeFunction"]
    resources = [aws_lambda_alias.live["ingest-jma"].arn, aws_lambda_alias.live["ingest-quake"].arn]
  }
}

resource "aws_iam_role_policy" "scheduler" {
  name   = "invoke-ingest"
  role   = aws_iam_role.scheduler.id
  policy = data.aws_iam_policy_document.scheduler.json
}

resource "aws_scheduler_schedule" "ingest" {
  for_each = toset(["ingest-jma", "ingest-quake"])

  name                         = "${local.prefix}-${each.key}"
  schedule_expression          = "rate(1 minute)"
  schedule_expression_timezone = "Asia/Tokyo"
  # 最初はコードがデプロイされていないため止めておき、最初のデプロイのあとで ENABLED にする
  state = var.ingest_enabled ? "ENABLED" : "DISABLED"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    arn      = aws_lambda_alias.live[each.key].arn
    role_arn = aws_iam_role.scheduler.arn
    retry_policy {
      maximum_retry_attempts       = 0 # 次の1分後の実行で取り直す
      maximum_event_age_in_seconds = 60
    }
  }
}
