package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
)

// SenderName は差出人の名前。Cognito の確認メール（infra/envs/prod/cognito.tf の from_email_address）とそろえる。
const SenderName = "saigai-timeline"

// fromHeader は差出人の名前付きの From（"saigai-timeline" <addr>）。
// SES の IAM の条件 ses:FromAddress はアドレスの部分だけと比べるため、名前を付けても条件に合う
func fromHeader(addr string) string {
	return (&mail.Address{Name: SenderName, Address: addr}).String()
}

// SESAPI は SES のうち使う操作。
type SESAPI interface {
	SendEmail(ctx context.Context, in *sesv2.SendEmailInput, opts ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// SESMailer は SES でメールを送る（本番）。
type SESMailer struct {
	Client SESAPI
	From   string // 差出人のアドレス（名前は付けない）
}

func (m SESMailer) Send(ctx context.Context, to, subject, body string) error {
	_, err := m.Client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(fromHeader(m.From)),
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
