# 取り込み → 突き合わせ → 通知 を SQS でつなぐ（docs/05 3.2）。失敗したメッセージは DLQ に移す
locals {
  queues = {
    match  = { visibility = 360 } # matcher の timeout（60秒）の6倍
    notify = { visibility = 180 } # notifier の timeout（30秒）の6倍
  }
}

resource "aws_sqs_queue" "dlq" {
  for_each                  = local.queues
  name                      = "${local.prefix}-${each.key}-dlq"
  message_retention_seconds = 1209600 # 14日
  sqs_managed_sse_enabled   = true
}

resource "aws_sqs_queue" "this" {
  for_each                   = local.queues
  name                       = "${local.prefix}-${each.key}"
  visibility_timeout_seconds = each.value.visibility
  message_retention_seconds  = 345600 # 4日
  sqs_managed_sse_enabled    = true
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[each.key].arn
    maxReceiveCount     = 5
  })
}
