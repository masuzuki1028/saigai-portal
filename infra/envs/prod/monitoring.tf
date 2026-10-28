# 監視（docs/05 4章）。アラームは SNS からメールで通知する（購読の確認メールのリンクを開く必要がある）
#trivy:ignore:AVD-AWS-0095 アラームの通知だけで秘密情報を含まないため、費用のかかる KMS のキーは使わない
resource "aws_sns_topic" "alarms" {
  name = "${local.prefix}-alarms"
}

resource "aws_sns_topic_subscription" "alarms_email" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.alarm_email
}

# 取り込みの停止（最重要）: 5分間、成功した実行がない
resource "aws_cloudwatch_metric_alarm" "ingest_stopped" {
  for_each            = var.ingest_enabled ? toset(["ingest-jma", "ingest-quake"]) : toset([])
  alarm_name          = "${local.prefix}-${each.key}-stopped"
  alarm_description   = "${each.key} が5分間正常に動いていない（取り込みの停止）"
  namespace           = "AWS/Lambda"
  metric_name         = "Invocations"
  dimensions          = { FunctionName = aws_lambda_function.fn[each.key].function_name }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
  ok_actions          = [aws_sns_topic.alarms.arn]
}

# 関数のエラー
resource "aws_cloudwatch_metric_alarm" "fn_errors" {
  for_each            = local.functions
  alarm_name          = "${local.prefix}-${each.key}-errors"
  alarm_description   = "${each.key} のエラーが続いている"
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  dimensions          = { FunctionName = aws_lambda_function.fn[each.key].function_name }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 3
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
}

# DLQ にメッセージが溜まった（突き合わせ・通知の失敗）
resource "aws_cloudwatch_metric_alarm" "dlq" {
  for_each            = local.queues
  alarm_name          = "${local.prefix}-${each.key}-dlq"
  alarm_description   = "${each.key} の処理に失敗したメッセージが DLQ にある"
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  dimensions          = { QueueName = aws_sqs_queue.dlq[each.key].name }
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
}

# API の 5xx
resource "aws_cloudwatch_metric_alarm" "api_5xx" {
  alarm_name          = "${local.prefix}-api-5xx"
  alarm_description   = "API の 5xx が増えている"
  namespace           = "AWS/ApiGateway"
  metric_name         = "5xx"
  dimensions          = { ApiId = aws_apigatewayv2_api.this.id }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 10
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]
}
