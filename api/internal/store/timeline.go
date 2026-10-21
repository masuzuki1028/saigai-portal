package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
)

// 保持期間（docs/04_data_model.md 5.3・5.4）。プラン別の期間は課金の導入時（Phase 4）に分ける。
const (
	EventTTL    = 400 * 24 * time.Hour
	TimelineTTL = 7 * 24 * time.Hour  // 無料プラン
	StateTTL    = 30 * 24 * time.Hour // 発表中の状態。更新のたびに延長する
)

// TimelineItem は地点ごとのタイムライン項目（docs/04 5.4）。
type TimelineItem struct {
	ID          string            `dynamodbav:"itemId"`
	Kind        string            `dynamodbav:"kind"` // official / sns / report
	EventID     string            `dynamodbav:"eventRef"`
	Hazard      domain.Hazard     `dynamodbav:"hazard"`
	Severity    domain.Severity   `dynamodbav:"severity"`
	Reliability string            `dynamodbav:"reliability"`
	Title       string            `dynamodbav:"title"`
	Summary     string            `dynamodbav:"summary"`
	Status      domain.ItemStatus `dynamodbav:"status"`
	Source      string            `dynamodbav:"source"`
	SourceURL   string            `dynamodbav:"sourceUrl"`
	IssuedAt    time.Time         `dynamodbav:"issuedAt"`
	Notified    bool              `dynamodbav:"notified"`
	WrittenBy   string            `dynamodbav:"writtenBy"`
}

func timelinePK(locID string) string { return "LOC#" + locID }

// PutTimelineItem はタイムライン項目を書き込む。同じ ID の項目は上書きしない（再実行しても重複しない）。
func (s *Store) PutTimelineItem(ctx context.Context, locID string, it TimelineItem) error {
	it.WrittenBy = s.WrittenBy
	item, err := attributevalue.MarshalMap(it)
	if err != nil {
		return err
	}
	item["PK"] = &types.AttributeValueMemberS{Value: timelinePK(locID)}
	item["SK"] = &types.AttributeValueMemberS{Value: "T#" + it.IssuedAt.UTC().Format(time.RFC3339) + "#" + it.ID}
	item["ttl"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlAfter(s.Now(), TimelineTTL), 10)}
	_, err = s.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: &s.Tables.Timeline, Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")})
	if isConditionFailed(err) {
		return nil
	}
	return err
}

// ErrBadCursor は不正なページングのカーソル。
var ErrBadCursor = errors.New("bad cursor")

type cursor struct {
	PK string `json:"p"`
	SK string `json:"s"`
}

// QueryTimeline は地点のタイムラインを新しい順に取得する（アクセスパターン #9）。
// カーソルは前のページの最後のキー。別の地点のキーを含むカーソルは拒否する（他人のタイムラインを読ませない）。
func (s *Store) QueryTimeline(ctx context.Context, locID string, limit int, after string) ([]TimelineItem, string, error) {
	pk := timelinePK(locID)
	in := &dynamodb.QueryInput{
		TableName:              &s.Tables.Timeline,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: "T#"},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(int32(limit)),
	}
	if after != "" {
		raw, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil {
			return nil, "", ErrBadCursor
		}
		var c cursor
		if err := json.Unmarshal(raw, &c); err != nil || c.PK != pk || len(c.SK) < 3 || c.SK[:2] != "T#" {
			return nil, "", ErrBadCursor
		}
		in.ExclusiveStartKey = key(c.PK, c.SK)
	}
	out, err := s.DB.Query(ctx, in)
	if err != nil {
		return nil, "", err
	}
	var items []TimelineItem
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return nil, "", err
	}
	next := ""
	if out.LastEvaluatedKey != nil {
		pkv, _ := out.LastEvaluatedKey["PK"].(*types.AttributeValueMemberS)
		skv, _ := out.LastEvaluatedKey["SK"].(*types.AttributeValueMemberS)
		if pkv == nil || skv == nil {
			return nil, "", errors.New("unexpected LastEvaluatedKey")
		}
		b, _ := json.Marshal(cursor{PK: pkv.Value, SK: skv.Value})
		next = base64.RawURLEncoding.EncodeToString(b)
	}
	return items, next, nil
}

// HazardState は地点・種別ごとの現在の状態。レベルが変わったときだけタイムラインと通知を出すために使う。
// Key は種別（警報・注意報）か、"Q#<地震のID>"（地震）。
type HazardState struct {
	Key        string           `dynamodbav:"stateKey"`
	Hazard     domain.Hazard    `dynamodbav:"hazard"`
	Severity   domain.Severity  `dynamodbav:"severity"`
	Title      string           `dynamodbav:"title"`
	Signature  string           `dynamodbav:"signature"` // 発表中の種類の組み合わせ（変化の検出用）
	Intensity  domain.Intensity `dynamodbav:"intensity,omitempty"`
	ReportTime time.Time        `dynamodbav:"reportTime"`
	Notified   bool             `dynamodbav:"notified"`
	WrittenBy  string           `dynamodbav:"writtenBy"`
}

func stateSK(k string) string { return "S#" + k }

// GetState は状態を取得する。なければ空の状態を返す。
func (s *Store) GetState(ctx context.Context, locID, k string) (HazardState, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.Timeline, Key: key(timelinePK(locID), stateSK(k)), ConsistentRead: aws.Bool(true)})
	if err != nil {
		return HazardState{}, err
	}
	st := HazardState{Key: k}
	if out.Item == nil {
		return st, nil
	}
	err = attributevalue.UnmarshalMap(out.Item, &st)
	return st, err
}

// PutState は状態を保存する。保存済みの状態より古い電文による更新は行わず false を返す
// （電文の到着順が入れ替わっても、古い状態に戻らないようにする）。
func (s *Store) PutState(ctx context.Context, locID string, st HazardState) (bool, error) {
	st.WrittenBy = s.WrittenBy
	st.ReportTime = st.ReportTime.UTC() // 文字列で大小比較するため UTC に揃える
	item, err := attributevalue.MarshalMap(st)
	if err != nil {
		return false, err
	}
	item["PK"] = &types.AttributeValueMemberS{Value: timelinePK(locID)}
	item["SK"] = &types.AttributeValueMemberS{Value: stateSK(st.Key)}
	item["ttl"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlAfter(s.Now(), StateTTL), 10)}
	rt, err := attributevalue.Marshal(st.ReportTime)
	if err != nil {
		return false, err
	}
	_, err = s.DB.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                 &s.Tables.Timeline,
		Item:                      item,
		ConditionExpression:       aws.String("attribute_not_exists(PK) OR reportTime <= :rt"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":rt": rt},
	})
	if isConditionFailed(err) {
		return false, nil
	}
	return err == nil, err
}

// ListStates は地点の状態をすべて取得する（ホーム画面の「現在のレベル」用）。
func (s *Store) ListStates(ctx context.Context, locID string) ([]HazardState, error) {
	out, err := s.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.Tables.Timeline,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: timelinePK(locID)},
			":sk": &types.AttributeValueMemberS{Value: "S#"},
		},
	})
	if err != nil {
		return nil, err
	}
	var sts []HazardState
	err = attributevalue.UnmarshalListOfMaps(out.Items, &sts)
	return sts, err
}

// EventRecord は events テーブルに保存する公式情報の要約（docs/04 5.3）。
type EventRecord struct {
	ID        string           `dynamodbav:"eventId"`
	Source    string           `dynamodbav:"source"`
	Kind      domain.EventKind `dynamodbav:"kind"`
	Title     string           `dynamodbav:"title"`
	Headline  string           `dynamodbav:"headline"`
	InfoType  string           `dynamodbav:"infoType"`
	IssuedAt  time.Time        `dynamodbav:"issuedAt"`
	ReportURL string           `dynamodbav:"reportUrl"`
	WrittenBy string           `dynamodbav:"writtenBy"`
}

func eventPK(id string) string { return "EVT#" + id }

// EventExists は取り込み済みの電文か（同じ電文を二度ダウンロードしない）。
func (s *Store) EventExists(ctx context.Context, id string) (bool, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.Events, Key: key(eventPK(id), "V#1"), ProjectionExpression: aws.String("PK")})
	if err != nil {
		return false, err
	}
	return out.Item != nil, nil
}

// PutEventIfAbsent は電文を保存する。既に保存済みなら false を返す（eventId による冪等化）。
func (s *Store) PutEventIfAbsent(ctx context.Context, ev *domain.Event) (bool, error) {
	item, err := attributevalue.MarshalMap(EventRecord{
		ID: ev.ID, Source: ev.Source, Kind: ev.Kind, Title: ev.Title, Headline: ev.Headline,
		InfoType: ev.InfoType, IssuedAt: ev.IssuedAt, ReportURL: ev.ReportURL, WrittenBy: s.WrittenBy,
	})
	if err != nil {
		return false, err
	}
	item["PK"] = &types.AttributeValueMemberS{Value: eventPK(ev.ID)}
	item["SK"] = &types.AttributeValueMemberS{Value: "V#1"}
	item["ttl"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(ttlAfter(s.Now(), EventTTL), 10)}
	_, err = s.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: &s.Tables.Events, Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")})
	if isConditionFailed(err) {
		return false, nil
	}
	return err == nil, err
}

// IngestCursor は取り込み処理の進捗（docs/04 5.1）。
type IngestCursor struct {
	Source       string    `dynamodbav:"source"`
	LastModified string    `dynamodbav:"lastModified"` // フィードの Last-Modified（If-Modified-Since に使う）
	LastFetched  time.Time `dynamodbav:"lastFetchedAt"`
}

func (s *Store) GetCursor(ctx context.Context, source string) (IngestCursor, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.App, Key: key("INGEST#"+source, "CURSOR")})
	if err != nil {
		return IngestCursor{}, err
	}
	c := IngestCursor{Source: source}
	if out.Item == nil {
		return c, nil
	}
	err = attributevalue.UnmarshalMap(out.Item, &c)
	return c, err
}

func (s *Store) PutCursor(ctx context.Context, c IngestCursor) error {
	item, err := attributevalue.MarshalMap(c)
	if err != nil {
		return err
	}
	item["PK"] = &types.AttributeValueMemberS{Value: "INGEST#" + c.Source}
	item["SK"] = &types.AttributeValueMemberS{Value: "CURSOR"}
	item["writtenBy"] = &types.AttributeValueMemberS{Value: s.WrittenBy}
	_, err = s.DB.PutItem(ctx, &dynamodb.PutItemInput{TableName: &s.Tables.App, Item: item})
	return err
}

func itoa(i int) string { return strconv.Itoa(i) }
