// Package lambdahttp は API Gateway HTTP API（ペイロード 2.0）のイベントを net/http のハンドラにつなぐ。
// ローカルの API サーバー（cmd/local）と同じハンドラを Lambda で動かすために使う（docs/08 6.1）。
package lambdahttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
)

// maxBodyBytes はリクエストの本文の上限。API Gateway の上限（10MB）より小さくし、大きな入力を受け付けない。
const maxBodyBytes = 1 << 20

var errBodyTooLarge = errors.New("request body too large")

// Handler は h を API Gateway のイベントで呼ぶ関数を返す（lambda.Start に渡す）。
func Handler(h http.Handler) func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return func(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		r, err := Request(ctx, ev)
		if err != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: http.StatusBadRequest, Body: `{"code":"bad_request","message":"入力が正しくありません"}`,
				Headers: map[string]string{"Content-Type": "application/json"}}, nil
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return Response(w.Result().StatusCode, w.Header(), w.Body.Bytes()), nil
	}
}

// Request はイベントを http.Request にする。JWT オーソライザが検証したクレームはコンテキストに入れる。
func Request(ctx context.Context, ev events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	body := []byte(ev.Body)
	if ev.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(ev.Body)
		if err != nil {
			return nil, err
		}
		body = b
	}
	if len(body) > maxBodyBytes {
		return nil, errBodyTooLarge
	}
	if ja := ev.RequestContext.Authorizer; ja != nil && ja.JWT != nil {
		ctx = auth.WithClaims(ctx, ja.JWT.Claims)
	}
	u := ev.RawPath
	if ev.RawQueryString != "" {
		u += "?" + ev.RawQueryString
	}
	r, err := http.NewRequestWithContext(ctx, ev.RequestContext.HTTP.Method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range ev.Headers {
		r.Header.Set(k, v) // API Gateway は同じ名前のヘッダーをカンマでつないで渡す
	}
	if len(ev.Cookies) > 0 {
		r.Header.Set("Cookie", strings.Join(ev.Cookies, "; "))
	}
	r.Host = ev.RequestContext.DomainName
	r.RemoteAddr = ev.RequestContext.HTTP.SourceIP
	r.RequestURI = u
	return r, nil
}

// Response は http のレスポンスを API Gateway の形にする。文字列でない本文は base64 にする。
func Response(status int, header http.Header, body []byte) events.APIGatewayV2HTTPResponse {
	res := events.APIGatewayV2HTTPResponse{StatusCode: status, Headers: map[string]string{}}
	for k, vs := range header {
		if http.CanonicalHeaderKey(k) == "Set-Cookie" {
			res.Cookies = append(res.Cookies, vs...)
			continue
		}
		res.Headers[k] = strings.Join(vs, ",")
	}
	if utf8.Valid(body) {
		res.Body = string(body)
	} else {
		res.Body = base64.StdEncoding.EncodeToString(body)
		res.IsBase64Encoded = true
	}
	return res
}
