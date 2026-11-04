// matcher は match-queue の公式情報を地点と突き合わせる Lambda。通知の依頼は地点ごとに notify-queue に送る。
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/masuzuki1028/saigai-portal/api/internal/awsenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
	"github.com/masuzuki1028/saigai-portal/api/internal/queue"
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
	q, err := env.Queue("NOTIFY_QUEUE_URL")
	if err != nil {
		return err
	}
	m := &matcher.Matcher{Store: st, Notify: func(ctx context.Context, jobs []matcher.Job) error { return q.Send(ctx, jobs) }, Log: log}
	lambda.Start(func(ctx context.Context, ev events.SQSEvent) (events.SQSEventResponse, error) {
		return queue.HandleBatch(ctx, ev, log, func(ctx context.Context, e domain.Event) error {
			_, err := m.Match(ctx, &e)
			return err
		}), nil
	})
	return nil
}
