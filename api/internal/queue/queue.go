// Package queue は取り込み → 突き合わせ → 通知 をつなぐ SQS のメッセージを扱う（docs/05 3.2）。
//
// match-queue には domain.Event、notify-queue には 1つの地点への []matcher.Job を JSON で入れる。
// 受け取る側（matcher・notifier）は、同じメッセージを2回受け取っても結果が重複しないようにしてある。
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// MaxMessageBytes は SQS のメッセージの上限（標準の 256KiB）。
const MaxMessageBytes = 256 << 10

// SQSAPI は SQS のうち使う操作。
type SQSAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, opts ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Sender は1つのキューに JSON のメッセージを送る。
type Sender struct {
	Client SQSAPI
	URL    string
}

func (s Sender) Send(ctx context.Context, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > MaxMessageBytes {
		return fmt.Errorf("message too large: %d bytes", len(b))
	}
	_, err = s.Client.SendMessage(ctx, &sqs.SendMessageInput{QueueUrl: aws.String(s.URL), MessageBody: aws.String(string(b))})
	if err != nil {
		return fmt.Errorf("sqs SendMessage: %w", err)
	}
	return nil
}

// HandleBatch はメッセージを1件ずつ処理し、失敗したものだけを返す（ReportBatchItemFailures）。
// 失敗したメッセージは可視性タイムアウトのあとに再び届き、上限の回数を超えると DLQ に移る。
func HandleBatch[T any](ctx context.Context, ev events.SQSEvent, log *slog.Logger, handle func(context.Context, T) error) events.SQSEventResponse {
	var res events.SQSEventResponse
	for _, m := range ev.Records {
		var v T
		err := json.Unmarshal([]byte(m.Body), &v)
		if err == nil {
			err = handle(ctx, v)
		}
		if err != nil {
			log.Error("message failed", "messageId", m.MessageId, "receiveCount", m.Attributes["ApproximateReceiveCount"], "err", err)
			res.BatchItemFailures = append(res.BatchItemFailures, events.SQSBatchItemFailure{ItemIdentifier: m.MessageId})
		}
	}
	return res
}
