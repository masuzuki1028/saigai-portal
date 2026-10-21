//go:build local

// local はローカル用の API サーバー。Lambda と同じハンドラを net/http で動かす（docs/08 6.1）。
//
//	docker compose up -d
//	go run -tags local ./cmd/local            # http://localhost:8080/api
//	go run -tags local ./cmd/local -ingest    # 気象庁のフィードを1分ごとに取り込む
package main

import (
	"context"
	"flag"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/localenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/logx"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "待ち受けるアドレス")
	ingest := flag.Bool("ingest", false, "気象庁のフィードを1分ごとに取り込む（外部の気象庁サーバーにアクセスする）")
	flag.Parse()

	log := logx.New()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	env := localenv.FromEnv()
	app, err := localenv.New(ctx, env, log)
	if err != nil {
		log.Error("起動できません", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", app.API)
	srv := &http.Server{Addr: *addr, Handler: cors(env.AppBaseURL, mux), ReadHeaderTimeout: 5 * time.Second}

	if *ingest {
		go poll(ctx, app, log)
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Info("ローカル API サーバーを起動しました", "addr", "http://"+*addr+"/api", "geocoder", env.Geocoder, "ingest", *ingest, "tablePrefix", env.TablePrefix)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}

// poll は気象庁のフィードを約1分ごとに取得する。本番の EventBridge Scheduler の代わり。
func poll(ctx context.Context, app *localenv.App, log *slog.Logger) {
	for {
		st, err := app.Pipeline.Ingester.Poll(ctx)
		if err != nil {
			log.Error("ingest", "err", err)
		} else {
			log.Info("ingest", "new", st.New, "skipped", st.Skipped, "failed", st.Failed)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(60*time.Second + time.Duration(rand.IntN(5000))*time.Millisecond):
		}
	}
}

// cors は Next.js の開発サーバー（別オリジン）からの呼び出しを許可する。
// 本番は CloudFront で同一オリジンにするため CORS は開けない（docs/07 3.6）。ローカル専用。
func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
