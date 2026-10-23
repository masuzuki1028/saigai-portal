// Package pipeline は 取り込み → 突き合わせ → 通知 をプロセス内でつなぐ（ローカル・replay・結合テスト用）。
// 本番ではそれぞれ別の Lambda で動き、SQS（match-queue / notify-queue）でつながる（docs/05 3.2）。
package pipeline

import (
	"context"
	"log/slog"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/ingest"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
	"github.com/masuzuki1028/saigai-portal/api/internal/notifier"
	"github.com/masuzuki1028/saigai-portal/api/internal/p2p"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

type Pipeline struct {
	Ingester *ingest.Ingester
	P2P      *p2p.Ingester
	Matcher  *matcher.Matcher
	Notifier *notifier.Notifier
	// OnResult は電文ごとの突き合わせの結果を受け取る（replay の出力用）。
	OnResult func(ev *domain.Event, r matcher.Result)
}

func New(st *store.Store, dir auth.Directory, mailer notifier.Mailer, appBaseURL string, log *slog.Logger) *Pipeline {
	p := &Pipeline{}
	p.Notifier = &notifier.Notifier{Users: st, Directory: dir, Mailer: mailer, AppBaseURL: appBaseURL, Log: log}
	p.Matcher = &matcher.Matcher{Store: st, Notify: p.Notifier.Handle, Log: log}
	onEvent := func(ctx context.Context, ev *domain.Event) error {
		r, err := p.Matcher.Match(ctx, ev)
		if p.OnResult != nil {
			p.OnResult(ev, r)
		}
		return err
	}
	p.Ingester = ingest.New(st, onEvent, log)
	p.P2P = p2p.NewIngester(st, onEvent, log)
	return p
}
