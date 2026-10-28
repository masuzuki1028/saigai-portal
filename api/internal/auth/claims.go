package auth

import (
	"context"
	"net/http"
)

type claimsKey struct{}

// WithClaims は API Gateway の JWT オーソライザが検証したクレームをコンテキストに入れる（lambdahttp が呼ぶ）。
func WithClaims(ctx context.Context, claims map[string]string) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// ClaimsVerifier は API Gateway の JWT オーソライザが検証したクレームから Principal を作る。
// 署名・有効期限・発行元・クライアント ID は API Gateway が検証済み。ここでは発行元とトークンの種類を確かめ直す
// （オーソライザの設定を誤っても、別のユーザープールや ID トークンで呼べないようにする）。
type ClaimsVerifier struct {
	Issuer string // 例: https://cognito-idp.ap-northeast-1.amazonaws.com/<ユーザープールID>
}

func (v ClaimsVerifier) Verify(r *http.Request) (Principal, error) {
	c, _ := r.Context().Value(claimsKey{}).(map[string]string)
	if c == nil || v.Issuer == "" || c["iss"] != v.Issuer || c["token_use"] != "access" || c["sub"] == "" {
		return Principal{}, ErrUnauthenticated
	}
	return Principal{Issuer: c["iss"], Subject: c["sub"]}, nil
}
