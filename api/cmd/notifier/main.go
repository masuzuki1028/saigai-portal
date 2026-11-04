// notifier は notify-queue の通知の依頼を送る Lambda。MVP はメールのみ（SES を有効にするまでは送らずにログだけ残す）。
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/masuzuki1028/saigai-portal/api/internal/awsenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
	"github.com/masuzuki1028/saigai-portal/api/internal/notifier"
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
	v, err := awsenv.Must("APP_BASE_URL")
	if err != nil {
		return err
	}
	st, err := env.Store()
	if err != nil {
		return err
	}
	cognito, err := env.Cognito()
	if err != nil {
		return err
	}
	mailer, err := env.Mailer(log)
	if err != nil {
		return err
	}
	n := &notifier.Notifier{Users: st, Directory: cognito, Mailer: mailer, AppBaseURL: v["APP_BASE_URL"], Log: log}
	lambda.Start(func(ctx context.Context, ev events.SQSEvent) (events.SQSEventResponse, error) {
		return queue.HandleBatch(ctx, ev, log, n.Handle), nil
	})
	return nil
}
