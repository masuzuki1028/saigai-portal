package notifier

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
)

type fakeSES struct{ in []*sesv2.SendEmailInput }

func (f *fakeSES) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.in = append(f.in, in)
	return &sesv2.SendEmailOutput{}, nil
}

func TestSESMailer(t *testing.T) {
	f := &fakeSES{}
	m := SESMailer{Client: f, From: "notify@example.com"}
	if err := m.Send(context.Background(), "a@example.com", "件名\r\nBcc: evil@example.com", "本文"); err != nil {
		t.Fatal(err)
	}
	in := f.in[0]
	if *in.FromEmailAddress != "notify@example.com" || in.Destination.ToAddresses[0] != "a@example.com" {
		t.Errorf("from/to = %s %v", *in.FromEmailAddress, in.Destination.ToAddresses)
	}
	if got := *in.Content.Simple.Subject.Data; got != "件名  Bcc: evil@example.com" {
		t.Errorf("件名に改行が残っている: %q", got)
	}
	if *in.Content.Simple.Body.Text.Data != "本文" || *in.Content.Simple.Body.Text.Charset != "UTF-8" {
		t.Errorf("body = %+v", in.Content.Simple.Body.Text)
	}
}

type noEmailDir struct{}

func (noEmailDir) Email(context.Context, auth.Principal) (string, error) { return "", auth.ErrNoEmail }

// 確認済みのメールアドレスがない利用者には送らず、エラーにもしない（再試行しても届かないため）。
func TestHandleSkipsUnverifiedEmail(t *testing.T) {
	users := fakeUsers{"U1": {ID: "U1", Issuer: "iss", Subject: "s1", EmailEnabled: true}}
	m := &fakeMailer{}
	n := &Notifier{Users: users, Directory: noEmailDir{}, Mailer: m, AppBaseURL: "https://example.com", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	job := matcher.Job{OwnerKey: "OWNER#U#U1", LocationID: "L1", LocationName: "自宅", Hazard: domain.Rain, Severity: 3, Title: "大雨警報", IssuedAt: time.Now()}
	if err := n.Handle(context.Background(), []matcher.Job{job}); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 0 {
		t.Errorf("送った: %v", m.sent)
	}
}

func TestDisabledMailer(t *testing.T) {
	if err := (DisabledMailer{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}).Send(context.Background(), "a@example.com", "s", "b"); err != nil {
		t.Fatal(err)
	}
}
