// Package auth はリクエストの認証（誰からのリクエストか）を扱う。
// 認可（何をしてよいか）は httpapi のミドルウェアで、トークンの sub から引いた userId で判定する。
package auth

import (
	"context"
	"errors"
	"net/http"
)

// Principal は認証済みの利用者。Issuer と Subject は Cognito（ローカルでは固定値）の値。
type Principal struct {
	Issuer  string
	Subject string
}

var ErrUnauthenticated = errors.New("unauthenticated")

// Verifier はリクエストから Principal を取り出す。
// 本番では API Gateway の JWT オーソライザが検証したクレームを使う（AWS への構築時に実装する）。
type Verifier interface {
	Verify(r *http.Request) (Principal, error)
}

// Directory は通知先のメールアドレスを引く。メールアドレスは DynamoDB に保存せず、
// 認証基盤（Cognito）にだけ置く（docs/04_data_model.md 8章）。
type Directory interface {
	Email(ctx context.Context, p Principal) (string, error)
}
