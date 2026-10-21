package notifier

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

type fakeUsers map[string]store.User

func (f fakeUsers) GetUser(_ context.Context, id string) (store.User, error) {
	u, ok := f[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

type fakeDir struct{}

func (fakeDir) Email(_ context.Context, p auth.Principal) (string, error) {
	return p.Subject + "@example.com", nil
}

type sent struct{ to, subject, body string }

type fakeMailer struct{ sent []sent }

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.sent = append(f.sent, sent{to, subject, body})
	return nil
}

func TestHandle(t *testing.T) {
	users := fakeUsers{
		"U1": {ID: "U1", Issuer: "local", Subject: "alice", EmailEnabled: true},
		"U2": {ID: "U2", Issuer: "local", Subject: "bob", EmailEnabled: false},
	}
	m := &fakeMailer{}
	n := &Notifier{Users: users, Directory: fakeDir{}, Mailer: m, AppBaseURL: "http://localhost:3000", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	job := matcher.Job{
		OwnerKey: "OWNER#U#U1", LocationID: "L1", LocationName: "自宅\r\nBcc: evil@example.com", Hazard: domain.Rain,
		Severity: 3, Title: "大雨警報", Summary: "大雨警報（長岡市）", Status: domain.StatusIssued, IssuedAt: time.Now(),
	}
	job2 := job
	job2.Title, job2.Hazard = "洪水警報", domain.Flood
	ctx := context.Background()
	if err := n.Handle(ctx, []matcher.Job{job, job2}); err != nil {
		t.Fatal(err)
	}
	job.OwnerKey = "OWNER#U#U2" // メール通知オフ
	_ = n.Handle(ctx, []matcher.Job{job})
	job.OwnerKey = "OWNER#U#UNKNOWN" // 退会済み
	if err := n.Handle(ctx, []matcher.Job{job}); err != nil {
		t.Errorf("退会済みの利用者でエラー: %v", err)
	}

	if len(m.sent) != 1 {
		t.Fatalf("送信数 = %d, want 1", len(m.sent))
	}
	got := m.sent[0]
	if got.to != "alice@example.com" {
		t.Errorf("宛先 = %q", got.to)
	}
	if strings.ContainsAny(got.subject, "\r\n") {
		t.Errorf("件名に改行が含まれる: %q", got.subject)
	}
	if !strings.Contains(got.subject, "大雨警報、洪水警報") {
		t.Errorf("1通にまとまっていない: %q", got.subject)
	}
	for _, want := range []string{"/app/location?id=L1", "出典: 気象庁", "避難の判断は"} {
		if !strings.Contains(got.body, want) {
			t.Errorf("本文に %q がない", want)
		}
	}
}
