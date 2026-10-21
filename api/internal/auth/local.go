//go:build local

// ローカル専用の認証。ビルドタグ local を付けたときだけビルドされ、Lambda のビルドには含まれない
// （CLAUDE.md「ローカル専用の認証バイパスはビルドタグ local の中にだけ置く」）。

package auth

import (
	"context"
	"net/http"
	"strings"
)

// LocalIssuer はローカルの固定トークンの発行元。
const LocalIssuer = "local"

// LocalUser はローカルで使える利用者。
type LocalUser struct {
	Token string
	Principal
	Email string
}

// LocalUsers は固定トークンの一覧。画面のローカル用ログインと認可テストの確認に使う。
var LocalUsers = []LocalUser{
	{Token: "local-alice", Principal: Principal{Issuer: LocalIssuer, Subject: "alice"}, Email: "alice@example.com"},
	{Token: "local-bob", Principal: Principal{Issuer: LocalIssuer, Subject: "bob"}, Email: "bob@example.com"},
}

// LocalVerifier は Authorization: Bearer <固定トークン> を受け付ける。
type LocalVerifier struct{}

func (LocalVerifier) Verify(r *http.Request) (Principal, error) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	for _, u := range LocalUsers {
		if u.Token == token {
			return u.Principal, nil
		}
	}
	return Principal{}, ErrUnauthenticated
}

// LocalDirectory は LocalUsers のメールアドレスを返す。
type LocalDirectory struct{}

func (LocalDirectory) Email(_ context.Context, p Principal) (string, error) {
	for _, u := range LocalUsers {
		if u.Principal == p {
			return u.Email, nil
		}
	}
	return "", ErrUnauthenticated
}
