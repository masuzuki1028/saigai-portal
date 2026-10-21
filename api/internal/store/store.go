// Package store は DynamoDB へのアクセス（docs/04_data_model.md 5章）。
package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Tables はテーブル名。環境ごとに接頭辞を付ける（例: saigai-local-app）。
type Tables struct {
	App, Events, Timeline, Audit string
}

// TablesWithPrefix は "<prefix>-app" などのテーブル名を作る。
func TablesWithPrefix(prefix string) Tables {
	return Tables{App: prefix + "-app", Events: prefix + "-events", Timeline: prefix + "-timeline", Audit: prefix + "-audit"}
}

// Store は DynamoDB のクライアントとテーブル名をまとめたもの。
type Store struct {
	DB     *dynamodb.Client
	Tables Tables
	// WrittenBy は書き込みに付けるリリースの識別子（CLAUDE.md「新旧バージョンの共存」）
	WrittenBy string
	Now       func() time.Time
}

func New(db *dynamodb.Client, tables Tables, writtenBy string) *Store {
	return &Store{DB: db, Tables: tables, WrittenBy: writtenBy, Now: time.Now}
}

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

//go:embed tables.json
var tablesJSON []byte

type tableDef struct {
	Name       string      `json:"name"`
	Attributes [][2]string `json:"attributes"`
	KeySchema  []string    `json:"keySchema"`
	GSI        []struct {
		Name      string   `json:"name"`
		KeySchema []string `json:"keySchema"`
	} `json:"gsi"`
	TTL string `json:"ttl"`
}

// EnsureTables は tables.json の定義でテーブルを作る（ローカル・結合テスト用）。既にあれば何もしない。
func (s *Store) EnsureTables(ctx context.Context) error {
	var defs struct {
		Tables []tableDef `json:"tables"`
	}
	if err := json.Unmarshal(tablesJSON, &defs); err != nil {
		return err
	}
	names := map[string]string{"app": s.Tables.App, "events": s.Tables.Events, "timeline": s.Tables.Timeline, "audit": s.Tables.Audit}
	for _, d := range defs.Tables {
		name := names[d.Name]
		if _, err := s.DB.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &name}); err == nil {
			continue
		}
		in := &dynamodb.CreateTableInput{TableName: aws.String(name), BillingMode: types.BillingModePayPerRequest}
		for _, a := range d.Attributes {
			in.AttributeDefinitions = append(in.AttributeDefinitions, types.AttributeDefinition{AttributeName: aws.String(a[0]), AttributeType: types.ScalarAttributeType(a[1])})
		}
		in.KeySchema = keySchema(d.KeySchema)
		for _, g := range d.GSI {
			in.GlobalSecondaryIndexes = append(in.GlobalSecondaryIndexes, types.GlobalSecondaryIndex{
				IndexName:  aws.String(g.Name),
				KeySchema:  keySchema(g.KeySchema),
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			})
		}
		if _, err := s.DB.CreateTable(ctx, in); err != nil {
			return fmt.Errorf("create table %s: %w", name, err)
		}
		if d.TTL != "" {
			_, err := s.DB.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
				TableName:               aws.String(name),
				TimeToLiveSpecification: &types.TimeToLiveSpecification{AttributeName: aws.String(d.TTL), Enabled: aws.Bool(true)},
			})
			if err != nil {
				return fmt.Errorf("update ttl %s: %w", name, err)
			}
		}
	}
	return nil
}

// DeleteTables はテーブルを削除する（結合テストの後片付け用）。
func (s *Store) DeleteTables(ctx context.Context) error {
	for _, n := range []string{s.Tables.App, s.Tables.Events, s.Tables.Timeline, s.Tables.Audit} {
		if _, err := s.DB.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(n)}); err != nil {
			var nf *types.ResourceNotFoundException
			if !errors.As(err, &nf) {
				return err
			}
		}
	}
	return nil
}

func keySchema(keys []string) []types.KeySchemaElement {
	out := []types.KeySchemaElement{{AttributeName: aws.String(keys[0]), KeyType: types.KeyTypeHash}}
	if len(keys) > 1 {
		out = append(out, types.KeySchemaElement{AttributeName: aws.String(keys[1]), KeyType: types.KeyTypeRange})
	}
	return out
}

func isConditionFailed(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return true
	}
	var tc *types.TransactionCanceledException
	if errors.As(err, &tc) {
		for _, r := range tc.CancellationReasons {
			if aws.ToString(r.Code) == "ConditionalCheckFailed" {
				return true
			}
		}
	}
	return false
}

func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: pk},
		"SK": &types.AttributeValueMemberS{Value: sk},
	}
}

func ttlAfter(now time.Time, d time.Duration) int64 { return now.Add(d).Unix() }
