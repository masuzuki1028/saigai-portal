//go:build local

// Package localenv はローカル環境（DynamoDB Local・Mailpit・固定トークン）の部品を組み立てる。
// ビルドタグ local のときだけビルドされる。
package localenv

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/crypto"
	"github.com/masuzuki1028/saigai-portal/api/internal/httpapi"
	"github.com/masuzuki1028/saigai-portal/api/internal/notifier"
	"github.com/masuzuki1028/saigai-portal/api/internal/pipeline"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
	"github.com/masuzuki1028/saigai-portal/api/internal/store/storetest"
)

// devKey はローカル専用の暗号鍵。本番は KMS を使う。LOCAL_ENCRYPTION_KEY（64桁の16進数）で上書きできる。
const devKey = "6c6f63616c2d6f6e6c792d6465762d6b65792d646f2d6e6f742d7573652d2121"

// Env はローカル環境の設定（環境変数から読む）。
type Env struct {
	DynamoEndpoint string
	TablePrefix    string
	SMTPAddr       string
	MailFrom       string
	AppBaseURL     string
	Geocoder       string // gsi（国土地理院の逆ジオコーダ）/ static（外部を呼ばない）
}

func FromEnv() Env {
	return Env{
		DynamoEndpoint: get("DYNAMODB_ENDPOINT", storetest.DefaultEndpoint),
		TablePrefix:    get("TABLE_PREFIX", "saigai-local"),
		SMTPAddr:       get("SMTP_ADDR", "127.0.0.1:1025"),
		MailFrom:       get("MAIL_FROM", "noreply@saigai.local"),
		AppBaseURL:     get("APP_BASE_URL", "http://localhost:3000"),
		Geocoder:       get("GEOCODER", "gsi"),
	}
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// App はローカルで動かすための部品一式。
type App struct {
	Env      Env
	Store    *store.Store
	Pipeline *pipeline.Pipeline
	API      http.Handler // /api 配下
}

func New(ctx context.Context, env Env, log *slog.Logger) (*App, error) {
	st := store.New(storetest.Client(env.DynamoEndpoint), store.TablesWithPrefix(env.TablePrefix), "local")
	if err := st.EnsureTables(ctx); err != nil {
		return nil, fmt.Errorf("DynamoDB Local に接続できません（docker compose up -d を実行してください）: %w", err)
	}
	key, err := hex.DecodeString(get("LOCAL_ENCRYPTION_KEY", devKey))
	if err != nil {
		return nil, err
	}
	sealer, err := crypto.NewAESGCM(key)
	if err != nil {
		return nil, err
	}
	var resolver area.Resolver = area.NewGSIResolver()
	if env.Geocoder == "static" {
		resolver = area.StaticResolver{Points: area.DefaultStaticPoints}
	}
	mailer := notifier.SMTPMailer{Addr: env.SMTPAddr, From: env.MailFrom}
	p := pipeline.New(st, auth.LocalDirectory{}, mailer, env.AppBaseURL, log)
	srv := &httpapi.Server{Store: st, Resolver: resolver, Sealer: sealer, Notifier: p.Notifier, Log: log}
	h, err := srv.Handler(auth.LocalVerifier{})
	if err != nil {
		return nil, err
	}
	return &App{Env: env, Store: st, Pipeline: p, API: h}, nil
}

// TokenFor はローカルの利用者名（alice / bob）の Authorization ヘッダーの値。
func TokenFor(user string) string {
	for _, u := range auth.LocalUsers {
		if u.Subject == user {
			return "Bearer " + u.Token
		}
	}
	return ""
}
