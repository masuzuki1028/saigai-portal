package store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/oklog/ulid/v2"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
)

// User はアプリの利用者。データのキーには Cognito の sub ではなく UserID を使う（docs/04 5.1）。
type User struct {
	ID           string    `dynamodbav:"userId"`
	Issuer       string    `dynamodbav:"issuer"`
	Subject      string    `dynamodbav:"sub"`
	Plan         string    `dynamodbav:"plan"`
	EmailEnabled bool      `dynamodbav:"emailEnabled"`
	CreatedAt    time.Time `dynamodbav:"createdAt"`
	WrittenBy    string    `dynamodbav:"writtenBy"`
}

// Principal は通知先を引くための認証情報。
func (u User) Principal() auth.Principal { return auth.Principal{Issuer: u.Issuer, Subject: u.Subject} }

// OwnerKey は個人所有のデータのキー（OWNER#U#<userId>）。
func (u User) OwnerKey() string { return "OWNER#U#" + u.ID }

func subPK(p auth.Principal) string { return "SUB#" + p.Issuer + "#" + p.Subject }
func userPK(id string) string       { return "USER#" + id }

// ResolveUser はトークンの sub から利用者を引く。初めてのログインなら userId を発行して作る。
func (s *Store) ResolveUser(ctx context.Context, p auth.Principal) (User, error) {
	if p.Issuer == "" || p.Subject == "" {
		return User{}, fmt.Errorf("empty principal")
	}
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.App, Key: key(subPK(p), "META"), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return User{}, err
	}
	if out.Item != nil {
		var m struct {
			UserID string `dynamodbav:"userId"`
		}
		if err := attributevalue.UnmarshalMap(out.Item, &m); err != nil {
			return User{}, err
		}
		return s.GetUser(ctx, m.UserID)
	}

	u := User{ID: ulid.Make().String(), Issuer: p.Issuer, Subject: p.Subject, Plan: "free", EmailEnabled: true, CreatedAt: s.Now().UTC(), WrittenBy: s.WrittenBy}
	profile, err := attributevalue.MarshalMap(u)
	if err != nil {
		return User{}, err
	}
	profile["PK"] = &types.AttributeValueMemberS{Value: userPK(u.ID)}
	profile["SK"] = &types.AttributeValueMemberS{Value: "PROFILE"}
	index := key(subPK(p), "META")
	index["userId"] = &types.AttributeValueMemberS{Value: u.ID}
	index["writtenBy"] = &types.AttributeValueMemberS{Value: s.WrittenBy}
	_, err = s.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{
		// 同時に初回ログインしても userId を二重に発行しない
		{Put: &types.Put{TableName: &s.Tables.App, Item: index, ConditionExpression: aws.String("attribute_not_exists(PK)")}},
		{Put: &types.Put{TableName: &s.Tables.App, Item: profile, ConditionExpression: aws.String("attribute_not_exists(PK)")}},
	}})
	if isConditionFailed(err) {
		return s.ResolveUser(ctx, p)
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// GetUser は userId から利用者を引く。
func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.App, Key: key(userPK(id), "PROFILE")})
	if err != nil {
		return User{}, err
	}
	if out.Item == nil {
		return User{}, ErrNotFound
	}
	var u User
	err = attributevalue.UnmarshalMap(out.Item, &u)
	return u, err
}

// SetEmailEnabled はメール通知のオン・オフを変える。
func (s *Store) SetEmailEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.DB.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           &s.Tables.App,
		Key:                 key(userPK(id), "PROFILE"),
		UpdateExpression:    aws.String("SET emailEnabled = :e, writtenBy = :w"),
		ConditionExpression: aws.String("attribute_exists(PK)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":e": &types.AttributeValueMemberBOOL{Value: enabled},
			":w": &types.AttributeValueMemberS{Value: s.WrittenBy},
		},
	})
	if isConditionFailed(err) {
		return ErrNotFound
	}
	return err
}
