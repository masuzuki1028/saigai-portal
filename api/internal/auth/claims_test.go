package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

const issuer = "https://cognito-idp.ap-northeast-1.amazonaws.com/ap-northeast-1_TEST"

func TestClaimsVerifier(t *testing.T) {
	v := ClaimsVerifier{Issuer: issuer}
	ok := map[string]string{"iss": issuer, "sub": "s1", "token_use": "access"}
	tests := []struct {
		name   string
		claims map[string]string
		want   error
	}{
		{"アクセストークン", ok, nil},
		{"クレームがない（オーソライザを通っていない）", nil, ErrUnauthenticated},
		{"別のユーザープール", map[string]string{"iss": issuer + "X", "sub": "s1", "token_use": "access"}, ErrUnauthenticated},
		{"ID トークン", map[string]string{"iss": issuer, "sub": "s1", "token_use": "id"}, ErrUnauthenticated},
		{"sub がない", map[string]string{"iss": issuer, "token_use": "access"}, ErrUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/me", nil)
			if tt.claims != nil {
				r = r.WithContext(WithClaims(r.Context(), tt.claims))
			}
			p, err := v.Verify(r)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if err == nil && (p.Issuer != issuer || p.Subject != "s1") {
				t.Errorf("principal = %+v", p)
			}
		})
	}
	// Authorization ヘッダーだけでは認証しない（検証済みのクレームだけを信用する）
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.Header.Set("Authorization", "Bearer local-alice")
	if _, err := v.Verify(r); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("ヘッダーだけで認証された: %v", err)
	}
	if _, err := (ClaimsVerifier{}).Verify(httptest.NewRequest("GET", "/", nil).WithContext(WithClaims(context.Background(), ok))); err == nil {
		t.Error("発行元が未設定なのに認証された")
	}
}

type fakeCognito struct {
	users   map[string][]types.AttributeType
	enabled bool
	deleted []string
}

func (f *fakeCognito) AdminGetUser(_ context.Context, in *cip.AdminGetUserInput, _ ...func(*cip.Options)) (*cip.AdminGetUserOutput, error) {
	attrs, ok := f.users[*in.Username]
	if !ok {
		return nil, &types.UserNotFoundException{}
	}
	return &cip.AdminGetUserOutput{UserAttributes: attrs, Enabled: f.enabled}, nil
}

func (f *fakeCognito) AdminDeleteUser(_ context.Context, in *cip.AdminDeleteUserInput, _ ...func(*cip.Options)) (*cip.AdminDeleteUserOutput, error) {
	if _, ok := f.users[*in.Username]; !ok {
		return nil, &types.UserNotFoundException{}
	}
	f.deleted = append(f.deleted, *in.Username)
	delete(f.users, *in.Username)
	return &cip.AdminDeleteUserOutput{}, nil
}

func attr(k, v string) types.AttributeType {
	return types.AttributeType{Name: aws.String(k), Value: aws.String(v)}
}

func TestCognito(t *testing.T) {
	f := &fakeCognito{enabled: true, users: map[string][]types.AttributeType{
		"s1": {attr("email", "a@example.com"), attr("email_verified", "true")},
		"s2": {attr("email", "b@example.com"), attr("email_verified", "false")},
	}}
	c := Cognito{Client: f, UserPoolID: "pool"}
	ctx := context.Background()
	if got, err := c.Email(ctx, Principal{Subject: "s1"}); err != nil || got != "a@example.com" {
		t.Errorf("確認済み: %q %v", got, err)
	}
	for _, sub := range []string{"s2", "nobody"} {
		if _, err := c.Email(ctx, Principal{Subject: sub}); !errors.Is(err, ErrNoEmail) {
			t.Errorf("%s: err = %v, want ErrNoEmail", sub, err)
		}
	}
	f.enabled = false
	if _, err := c.Email(ctx, Principal{Subject: "s1"}); !errors.Is(err, ErrNoEmail) {
		t.Errorf("無効なユーザー: err = %v", err)
	}
	if err := c.DeleteUser(ctx, Principal{Subject: "s1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteUser(ctx, Principal{Subject: "s1"}); err != nil {
		t.Errorf("削除済みのユーザーの削除でエラー: %v", err)
	}
}
