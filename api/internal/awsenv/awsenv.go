// Package awsenv は Lambda（本番）の部品を、Terraform が設定する環境変数から組み立てる
// （infra/envs/prod/lambda.tf の common_env・function_env）。ローカルの組み立ては localenv。
package awsenv

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/crypto"
	"github.com/masuzuki1028/saigai-portal/api/internal/notifier"
	"github.com/masuzuki1028/saigai-portal/api/internal/queue"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// Env は Lambda の環境変数と AWS の設定。
type Env struct {
	AWS aws.Config
}

// Load は AWS の設定を読む（Lambda の実行ロールの一時クレデンシャルを使う）。
func Load(ctx context.Context) (*Env, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Env{AWS: cfg}, nil
}

// Must は必須の環境変数の値を返す。未設定ならエラー。
func Must(names ...string) (map[string]string, error) {
	out := map[string]string{}
	var missing []string
	for _, n := range names {
		v := os.Getenv(n)
		if v == "" {
			missing = append(missing, n)
		}
		out[n] = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("環境変数が設定されていません: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// Revision はビルドしたコミット（go build が埋め込む vcs.revision の先頭12桁）。
// 書き込みに付ける writtenBy に使う（CLAUDE.md「新旧バージョンの共存」）。
func Revision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 12 {
				return s.Value[:12]
			}
		}
	}
	return "unknown"
}

// Store は TABLE_PREFIX のテーブルを使う Store。
func (e *Env) Store() (*store.Store, error) {
	v, err := Must("TABLE_PREFIX")
	if err != nil {
		return nil, err
	}
	return store.New(dynamodb.NewFromConfig(e.AWS), store.TablesWithPrefix(v["TABLE_PREFIX"]), Revision()), nil
}

// Sealer は KMS_KEY_ID のキーでエンベロープ暗号化する。
func (e *Env) Sealer() (crypto.Sealer, error) {
	v, err := Must("KMS_KEY_ID")
	if err != nil {
		return nil, err
	}
	return crypto.NewKMS(kms.NewFromConfig(e.AWS), v["KMS_KEY_ID"]), nil
}

// Cognito は COGNITO_USER_POOL_ID のユーザープール。
func (e *Env) Cognito() (auth.Cognito, error) {
	v, err := Must("COGNITO_USER_POOL_ID")
	if err != nil {
		return auth.Cognito{}, err
	}
	return auth.Cognito{Client: cip.NewFromConfig(e.AWS), UserPoolID: v["COGNITO_USER_POOL_ID"]}, nil
}

// Mailer は SES_ENABLED=true なら SES、そうでなければ送らずにログだけ残す Mailer。
func (e *Env) Mailer(log *slog.Logger) (notifier.Mailer, error) {
	if os.Getenv("SES_ENABLED") != "true" {
		return notifier.DisabledMailer{Log: log}, nil
	}
	v, err := Must("MAIL_FROM")
	if err != nil {
		return nil, err
	}
	return notifier.SESMailer{Client: sesv2.NewFromConfig(e.AWS), From: v["MAIL_FROM"]}, nil
}

// Queue は環境変数 name の URL のキューに送る Sender。
func (e *Env) Queue(name string) (queue.Sender, error) {
	v, err := Must(name)
	if err != nil {
		return queue.Sender{}, err
	}
	return queue.Sender{Client: sqs.NewFromConfig(e.AWS), URL: v[name]}, nil
}
