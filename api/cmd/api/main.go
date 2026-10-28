// api は REST API の Lambda（API Gateway HTTP API から呼ばれる）。ハンドラはローカルの API サーバー（cmd/local）と共通。
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/awsenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/httpapi"
	"github.com/masuzuki1028/saigai-portal/api/internal/lambdahttp"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
	"github.com/masuzuki1028/saigai-portal/api/internal/notifier"
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
	v, err := awsenv.Must("APP_BASE_URL", "COGNITO_ISSUER")
	if err != nil {
		return err
	}
	st, err := env.Store()
	if err != nil {
		return err
	}
	sealer, err := env.Sealer()
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
	srv := &httpapi.Server{Store: st, Resolver: area.NewGSIResolver(), Sealer: sealer, Notifier: n, Accounts: cognito, Log: log}
	h, err := srv.Handler(auth.ClaimsVerifier{Issuer: v["COGNITO_ISSUER"]})
	if err != nil {
		return err
	}
	log.Info("api started", "revision", awsenv.Revision())
	lambda.Start(lambdahttp.Handler(h))
	return nil
}
