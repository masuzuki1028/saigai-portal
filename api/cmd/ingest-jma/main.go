// ingest-jma は気象庁の防災情報XML を取り込む Lambda（EventBridge Scheduler から1分ごとに呼ばれる）。
// 新しい電文は match-queue に送る。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/masuzuki1028/saigai-portal/api/internal/awsenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/ingest"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
)

func main() {
	log := logx.New()
	if err := run(context.Background(), log); err != nil {
		log.Error("起動できません", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	env, err := awsenv.Load(ctx)
	if err != nil {
		return err
	}
	st, err := env.Store()
	if err != nil {
		return err
	}
	q, err := env.Queue("MATCH_QUEUE_URL")
	if err != nil {
		return err
	}
	in := ingest.New(st, func(ctx context.Context, ev *domain.Event) error { return q.Send(ctx, ev) }, log)
	lambda.Start(func(ctx context.Context) error {
		s, err := in.Poll(ctx)
		log.Info("ingest-jma", "feeds", s.Feeds, "entries", s.Entries, "new", s.New, "skipped", s.Skipped, "failed", s.Failed)
		if s.Failed > 0 {
			// 失敗した電文は次の取り込みでもう一度処理する。続くようならアラームで気づけるようにエラーにする
			err = errors.Join(err, fmt.Errorf("%d telegrams failed", s.Failed))
		}
		return err
	})
	return nil
}
