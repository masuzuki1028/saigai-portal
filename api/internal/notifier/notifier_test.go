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

type fakeSent map[string]bool

func (f fakeSent) WasSent(_ context.Context, locID, k string) (bool, error) {
	return f[locID+"/"+k], nil
}
func (f fakeSent) MarkSent(_ context.Context, locID, k string) error {
	f[locID+"/"+k] = true
	return nil
}

// matcher のやり直しや SQS の再試行で同じ依頼が届いても、送った通知はもう一度送らない。
// 取消は、同じ項目でも別の通知として送る。
func TestHandleSkipsSentJobs(t *testing.T) {
	users := fakeUsers{"U1": {ID: "U1", Issuer: "iss", Subject: "s1", EmailEnabled: true}}
	m := &fakeMailer{}
	sentLog := fakeSent{}
	n := &Notifier{Users: users, Directory: fakeDir{}, Mailer: m, Sent: sentLog, AppBaseURL: "https://example.com", Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	rain := matcher.Job{OwnerKey: "OWNER#U#U1", LocationID: "L1", LocationName: "自宅", Hazard: domain.Rain, Severity: 3, Title: "大雨警報", Status: domain.StatusIssued, IssuedAt: at, ItemID: "I1"}
	flood := matcher.Job{OwnerKey: "OWNER#U#U1", LocationID: "L1", LocationName: "自宅", Hazard: domain.Flood, Severity: 3, Title: "洪水警報", Status: domain.StatusIssued, IssuedAt: at, ItemID: "I2"}
	ctx := context.Background()

	if err := n.Handle(ctx, []matcher.Job{rain}); err != nil {
		t.Fatal(err)
	}
	// 同じ依頼のやり直し: 送らない
	if err := n.Handle(ctx, []matcher.Job{rain}); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("送った回数 = %d, want 1", len(m.sent))
	}
	// やり直しで組み合わせが変わった: まだ送っていない洪水警報だけを送る
	if err := n.Handle(ctx, []matcher.Job{rain, flood}); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 2 || strings.Contains(m.sent[1].subject, "大雨警報") || !strings.Contains(m.sent[1].subject, "洪水警報") {
		t.Fatalf("2通目 = %+v", m.sent)
	}
	// 同じ項目の取消は別の通知
	cancel := rain
	cancel.Status = domain.StatusCancelled
	if err := n.Handle(ctx, []matcher.Job{cancel}); err != nil {
		t.Fatal(err)
	}
	if len(m.sent) != 3 {
		t.Errorf("取消を送っていない: %d", len(m.sent))
	}
	// 項目の ID のない依頼（古い版の matcher）は、記録せずに送る
	old := rain
	old.ItemID = ""
	for range 2 {
		if err := n.Handle(ctx, []matcher.Job{old}); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.sent) != 5 {
		t.Errorf("項目の ID のない依頼の送信回数 = %d, want 5", len(m.sent))
	}
}
