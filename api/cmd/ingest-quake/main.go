// ingest-quake は P2P地震情報の地震情報を取り込む Lambda（EventBridge Scheduler から1分ごとに呼ばれる）。
// 新しい情報は match-queue に送る。
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/masuzuki1028/saigai-portal/api/internal/awsenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
	"github.com/masuzuki1028/saigai-portal/api/internal/p2p"
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
	in := p2p.NewIngester(st, func(ctx context.Context, ev *domain.Event) error { return q.Send(ctx, ev) }, log)
	lambda.Start(func(ctx context.Context) error {
		n, err := in.Poll(ctx)
		log.Info("ingest-quake", "new", n)
		return err
	})
	return nil
}
