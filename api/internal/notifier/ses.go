package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
)

// SESAPI は SES のうち使う操作。
type SESAPI interface {
	SendEmail(ctx context.Context, in *sesv2.SendEmailInput, opts ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SESMailer は SES でメールを送る（本番）。
type SESMailer struct {
	Client SESAPI
	From   string
}

func (m SESMailer) Send(ctx context.Context, to, subject, body string) error {
	_, err := m.Client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.From),
		Destination:      &types.Destination{ToAddresses: []string{to}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String(oneLine(subject)), Charset: aws.String("UTF-8")},
			Body:    &types.Body{Text: &types.Content{Data: aws.String(body), Charset: aws.String("UTF-8")}},
		}},
	})
	if err != nil {
		return fmt.Errorf("ses SendEmail: %w", err)
	}
	return nil
}

// DisabledMailer は SES を有効にする前の本番で使う。送らずにログだけ残す。
type DisabledMailer struct {
	Log *slog.Logger
}

func (m DisabledMailer) Send(_ context.Context, to, _, _ string) error {
	m.Log.Warn("SES が無効のためメールを送りませんでした", "to", logx.MaskEmail(to))
	return nil
}
