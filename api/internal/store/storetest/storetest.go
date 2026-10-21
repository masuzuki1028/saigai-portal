//go:build integration || local

// Package storetest は DynamoDB Local に接続する Store を作る（結合テスト・ローカル用）。
package storetest

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// DefaultEndpoint は compose.yaml の DynamoDB Local。DYNAMODB_ENDPOINT で上書きできる。
const DefaultEndpoint = "http://127.0.0.1:8000"

// Client は DynamoDB Local のクライアントを作る。認証情報はダミー。
func Client(endpoint string) *dynamodb.Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return dynamodb.New(dynamodb.Options{
		Region:       "ap-northeast-1",
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
	})
}

// New はテストごとに別の接頭辞でテーブルを作り、テストの終わりに削除する。
func New(t testing.TB) *store.Store {
	t.Helper()
	ctx := context.Background()
	prefix := fmt.Sprintf("test-%d", time.Now().UnixNano())
	s := store.New(Client(os.Getenv("DYNAMODB_ENDPOINT")), store.TablesWithPrefix(prefix), "test")
	if err := s.EnsureTables(ctx); err != nil {
		t.Fatalf("DynamoDB Local に接続できない（docker compose up -d が必要）: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteTables(context.Background()) })
	return s
}
