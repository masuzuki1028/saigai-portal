// Package httpapi は api/openapi.yaml の API を実装する。Lambda とローカルサーバー（cmd/local）で共通のハンドラ。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/masuzuki1028/saigai-portal/api/internal/apigen"
	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/crypto"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// TestSender はテスト通知を送る（notifier.Notifier）。
type TestSender interface {
	SendTest(ctx context.Context, u store.User) error
}

// Server は StrictServerInterface の実装。
type Server struct {
	Store    *store.Store
	Resolver area.Resolver
	Sealer   crypto.Sealer
	Notifier TestSender
	Log      *slog.Logger
}

var _ apigen.StrictServerInterface = (*Server)(nil)

// PlanLimits はプランごとの地点の上限（docs/01_concept_overview.md 2.1）。
var PlanLimits = map[string]int{"free": 3, "personal_plus": 10, "business": 500}

// Handler は認証・入力の検証・ルーティングを組み合わせた http.Handler を返す。/api 配下で受ける。
func (s *Server) Handler(v auth.Verifier) (http.Handler, error) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		return nil, err
	}
	validate := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		DoNotValidateServers: true,
		Options: openapi3filter.Options{
			// 認証は authenticate で行う。ここでは入力の形だけを検証する
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			writeError(w, status, "bad_request", "入力が正しくありません")
		},
	})
	strict := apigen.NewStrictHandlerWithOptions(s, nil, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			writeError(w, http.StatusBadRequest, "bad_request", "入力が正しくありません")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			s.Log.Error("handler error", "path", r.URL.Path, "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "エラーが発生しました")
		},
	})
	routes := apigen.HandlerWithOptions(strict, apigen.StdHTTPServerOptions{BaseRouter: http.NewServeMux()})
	h := s.authenticate(v, validate(routes))
	return http.StripPrefix("/api", securityHeaders(h)), nil
}

type ctxKey struct{}

// UserFrom はミドルウェアが解決した利用者を返す。
func UserFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(store.User)
	return u, ok
}

// publicPaths は認証が不要なパス。
var publicPaths = map[string]bool{"/health": true}

// authenticate はトークンを検証し、トークンの sub から userId を引いてコンテキストに入れる。
// 以降の処理は、リクエストに含まれる ID ではなく、ここで引いた利用者で認可する（docs/07 3.2）。
func (s *Server) authenticate(v auth.Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		p, err := v.Verify(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "ログインが必要です")
			return
		}
		u, err := s.Store.ResolveUser(r.Context(), p)
		if err != nil {
			s.Log.Error("resolve user", "err", err)
			writeError(w, http.StatusInternalServerError, "internal", "エラーが発生しました")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apigen.Error{Code: code, Message: msg})
}

func errBody(code, msg string) apigen.Error { return apigen.Error{Code: code, Message: msg} }

var errNoUser = errors.New("no user in context")

func mustUser(ctx context.Context) (store.User, error) {
	u, ok := UserFrom(ctx)
	if !ok {
		return store.User{}, errNoUser
	}
	return u, nil
}

// trimName は前後の空白を除く。
func trimName(s string) string { return strings.TrimSpace(s) }
