package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// ErrNoEmail は通知先のメールアドレスがない（未確認・削除済み）ことを表す。通知は送らない。
var ErrNoEmail = errors.New("no verified email")

// CognitoAPI は Cognito のうち使う操作（テストではフェイクに差し替える）。
type CognitoAPI interface {
	AdminGetUser(ctx context.Context, in *cip.AdminGetUserInput, opts ...func(*cip.Options)) (*cip.AdminGetUserOutput, error)
	AdminDeleteUser(ctx context.Context, in *cip.AdminDeleteUserInput, opts ...func(*cip.Options)) (*cip.AdminDeleteUserOutput, error)
}

// Cognito はユーザープールからメールアドレスを引き、退会時にユーザーを削除する。
// ユーザープールはメールアドレスをユーザー名にしているため、内部のユーザー名は sub と同じ値になる。
type Cognito struct {
	Client     CognitoAPI
	UserPoolID string
}

// Email は確認済みのメールアドレスを返す。未確認・削除済みなら ErrNoEmail。
func (c Cognito) Email(ctx context.Context, p Principal) (string, error) {
	out, err := c.Client.AdminGetUser(ctx, &cip.AdminGetUserInput{UserPoolId: &c.UserPoolID, Username: aws.String(p.Subject)})
	var nf *types.UserNotFoundException
	if errors.As(err, &nf) {
		return "", ErrNoEmail
	}
	if err != nil {
		return "", fmt.Errorf("cognito AdminGetUser: %w", err)
	}
	var email, verified string
	for _, a := range out.UserAttributes {
		switch aws.ToString(a.Name) {
		case "email":
			email = aws.ToString(a.Value)
		case "email_verified":
			verified = aws.ToString(a.Value)
		}
	}
	if email == "" || verified != "true" || !out.Enabled {
		return "", ErrNoEmail
	}
	return email, nil
}

// DeleteUser は退会した利用者を削除する。既に削除済みなら何もしない。
func (c Cognito) DeleteUser(ctx context.Context, p Principal) error {
	_, err := c.Client.AdminDeleteUser(ctx, &cip.AdminDeleteUserInput{UserPoolId: &c.UserPoolID, Username: aws.String(p.Subject)})
	var nf *types.UserNotFoundException
	if errors.As(err, &nf) {
		return nil
	}
	return err
}
