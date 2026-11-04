package lambdahttp

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
)

const issuer = "https://cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_TEST"

func event(method, path, query, body string) events.APIGatewayV2HTTPRequest {
	ev := events.APIGatewayV2HTTPRequest{
		RawPath: path, RawQueryString: query, Body: body,
		Headers: map[string]string{"content-type": "application/json", "x-test": "a,b"},
	}
	ev.RequestContext.HTTP.Method = method
	ev.RequestContext.HTTP.SourceIP = "203.0.113.1"
	ev.RequestContext.DomainName = "api.example.com"
	return ev
}

func TestHandler(t *testing.T) {
	var got struct {
		method, path, query, body, header string
		principal                         auth.Principal
		authErr                           error
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method, got.path, got.query, got.body, got.header = r.Method, r.URL.Path, r.URL.Query().Get("limit"), string(b), r.Header.Get("X-Test")
		got.principal, got.authErr = auth.ClaimsVerifier{Issuer: issuer}.Verify(r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Set-Cookie", "a=1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	ev := event("POST", "/api/locations", "limit=20", `{"name":"自宅"}`)
	ev.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
		JWT: &events.APIGatewayV2HTTPRequestContextAuthorizerJWTDescription{Claims: map[string]string{"iss": issuer, "sub": "s1", "token_use": "access"}},
	}
	res, err := Handler(h)(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if got.method != "POST" || got.path != "/api/locations" || got.query != "20" || got.body != `{"name":"自宅"}` || got.header != "a,b" {
		t.Errorf("request = %+v", got)
	}
	if got.authErr != nil || got.principal.Subject != "s1" {
		t.Errorf("principal = %+v, %v", got.principal, got.authErr)
	}
	if res.StatusCode != 201 || res.Body != `{"ok":true}` || res.IsBase64Encoded || res.Headers["Content-Type"] != "application/json" || len(res.Cookies) != 1 {
		t.Errorf("response = %+v", res)
	}

	// オーソライザを通っていないリクエスト（/api/health）にはクレームがない
	if _, err := Handler(h)(context.Background(), event("GET", "/api/health", "", "")); err != nil {
		t.Fatal(err)
	}
	if got.authErr == nil {
		t.Error("クレームなしで認証された")
	}
}

func TestRequestBody(t *testing.T) {
	ev := event("PUT", "/api/x", "", base64.StdEncoding.EncodeToString([]byte("bin\x00ary")))
	ev.IsBase64Encoded = true
	r, err := Request(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(r.Body); string(b) != "bin\x00ary" {
		t.Errorf("body = %q", b)
	}
	if _, err := Request(context.Background(), event("PUT", "/api/x", "", strings.Repeat("a", maxBodyBytes+1))); err == nil {
		t.Error("大きすぎる本文を受け付けた")
	}
	res, _ := Handler(http.NotFoundHandler())(context.Background(), event("PUT", "/api/x", "", strings.Repeat("a", maxBodyBytes+1)))
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d", res.StatusCode)
	}
}

func TestResponseBinary(t *testing.T) {
	res := Response(200, http.Header{}, []byte{0xff, 0xfe})
	if !res.IsBase64Encoded || res.Body != base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe}) {
		t.Errorf("response = %+v", res)
	}
}
