// Package notifier は通知を送る（docs/05_system_architecture.md 3.2）。MVP はメールのみ。
package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// Mailer はメールを送る。本番は SES、ローカルは Mailpit（SMTP）。
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Users は notifier が使う store の操作。
type Users interface {
	GetUser(ctx context.Context, id string) (store.User, error)
}

type Notifier struct {
	Users     Users
	Directory auth.Directory
	Mailer    Mailer
	// AppBaseURL は通知から開く画面の URL（例: http://localhost:3000）
	AppBaseURL string
	Log        *slog.Logger
}

// Handle は1つの地点への通知の依頼をまとめて1通のメールにする。メール通知をオフにしている利用者には送らない。
func (n *Notifier) Handle(ctx context.Context, jobs []matcher.Job) error {
	if len(jobs) == 0 {
		return nil
	}
	j := jobs[0]
	userID := store.UserIDFromOwnerKey(j.OwnerKey)
	if userID == "" {
		return nil // 組織の通知チャネルは Phase 3
	}
	u, err := n.Users.GetUser(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil // 退会済み
	}
	if err != nil {
		return err
	}
	if !u.EmailEnabled {
		return nil
	}
	to, err := n.Directory.Email(ctx, u.Principal())
	if err != nil {
		return fmt.Errorf("lookup email: %w", err)
	}
	titles := make([]string, 0, len(jobs))
	var lines strings.Builder
	for _, x := range jobs {
		titles = append(titles, statusLabel(x.Status, x.Title))
		fmt.Fprintf(&lines, "■ %s\n%s\n\n", statusLabel(x.Status, x.Title), x.Summary)
	}
	subject := fmt.Sprintf("【saigai-timeline】%s: %s", oneLine(j.LocationName), strings.Join(titles, "、"))
	body := fmt.Sprintf(`%s の周辺で、次の情報が出ています。

%s発表: %s
タイムラインを見る: %s/app/location?id=%s

出典: 気象庁
このメールは補助的な情報です。避難の判断は、自治体・気象庁の公式情報に従ってください。
通知の設定: %s/app/settings/notifications
`, oneLine(j.LocationName), lines.String(), j.IssuedAt.In(jst).Format("2006年1月2日 15:04"), n.AppBaseURL, j.LocationID, n.AppBaseURL)
	if err := n.Mailer.Send(ctx, to, subject, body); err != nil {
		return err
	}
	n.Log.Info("notified", "locationId", j.LocationID, "jobs", len(jobs), "to", logx.MaskEmail(to))
	return nil
}

// SendTest はテスト通知を送る。
func (n *Notifier) SendTest(ctx context.Context, u store.User) error {
	to, err := n.Directory.Email(ctx, u.Principal())
	if err != nil {
		return fmt.Errorf("lookup email: %w", err)
	}
	return n.Mailer.Send(ctx, to, "【saigai-timeline】テスト通知", fmt.Sprintf(`これはテスト通知です。災害の情報が出たときは、このアドレスに届きます。

通知の設定: %s/app/settings/notifications
`, n.AppBaseURL))
}

func statusLabel(s domain.ItemStatus, title string) string {
	switch s {
	case domain.StatusCancelled:
		return "【取消】" + title
	case domain.StatusCorrected:
		return title + "（訂正）"
	case domain.StatusContinued:
		return title + "（レベルが変わりました）"
	default:
		return title
	}
}

// oneLine は件名に改行が入らないようにする（メールヘッダーインジェクション対策）。
func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}
