package store

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
)

// Location は登録地点（docs/04_data_model.md 5.2）。正確な座標と住所は暗号化して保存する。
type Location struct {
	OwnerKey       string                `dynamodbav:"PK"`
	ID             string                `dynamodbav:"locId"`
	Name           string                `dynamodbav:"name"`
	RadiusM        int                   `dynamodbav:"radiusM"`
	Geohash6       string                `dynamodbav:"geohash6"`
	CoordEnc       []byte                `dynamodbav:"coordEnc"`
	AddressEnc     []byte                `dynamodbav:"addressEnc,omitempty"`
	MuniCode       string                `dynamodbav:"muniCode"`
	AreaName       string                `dynamodbav:"areaName"`
	WarnKey        string                `dynamodbav:"warnKey"`
	QuakeKey       string                `dynamodbav:"quakeKey"`
	IsCoastal      bool                  `dynamodbav:"isCoastal"`
	HazardSettings domain.HazardSettings `dynamodbav:"hazardSettings"`
	Version        int                   `dynamodbav:"version"`
	CreatedAt      time.Time             `dynamodbav:"createdAt"`
	UpdatedAt      time.Time             `dynamodbav:"updatedAt"`
	WrittenBy      string                `dynamodbav:"writtenBy"`
}

func locSK(id string) string { return "LOC#" + id }

// AreaKind は逆引きインデックスの種類。
type AreaKind string

const (
	AreaWarn  AreaKind = "warn"
	AreaQuake AreaKind = "quake"
)

func areaPK(kind AreaKind, code string) string { return "AREA#" + string(kind) + "#" + code }

// AreaEntry は「エリアコード → 地点」の逆引き（AreaIndex）。突き合わせのときに Location 本体を
// 読まずに判定できるよう、設定のコピーを持つ（docs/04 5.1）。
type AreaEntry struct {
	LocationID     string                `dynamodbav:"locId"`
	OwnerKey       string                `dynamodbav:"ownerKey"`
	Name           string                `dynamodbav:"name"`
	HazardSettings domain.HazardSettings `dynamodbav:"hazardSettings"`
	WrittenBy      string                `dynamodbav:"writtenBy"`
}

func (l Location) areaEntries() map[AreaKind]string {
	return map[AreaKind]string{AreaWarn: l.WarnKey, AreaQuake: l.QuakeKey}
}

func (s *Store) areaPut(l Location) ([]types.TransactWriteItem, error) {
	var out []types.TransactWriteItem
	for kind, code := range l.areaEntries() {
		item, err := attributevalue.MarshalMap(AreaEntry{LocationID: l.ID, OwnerKey: l.OwnerKey, Name: l.Name, HazardSettings: l.HazardSettings, WrittenBy: s.WrittenBy})
		if err != nil {
			return nil, err
		}
		item["PK"] = &types.AttributeValueMemberS{Value: areaPK(kind, code)}
		item["SK"] = &types.AttributeValueMemberS{Value: locSK(l.ID)}
		out = append(out, types.TransactWriteItem{Put: &types.Put{TableName: &s.Tables.App, Item: item}})
	}
	return out, nil
}

func (s *Store) areaDelete(l Location) []types.TransactWriteItem {
	var out []types.TransactWriteItem
	for kind, code := range l.areaEntries() {
		out = append(out, types.TransactWriteItem{Delete: &types.Delete{TableName: &s.Tables.App, Key: key(areaPK(kind, code), locSK(l.ID))}})
	}
	return out
}

func (s *Store) marshalLocation(l Location) (map[string]types.AttributeValue, error) {
	item, err := attributevalue.MarshalMap(l)
	if err != nil {
		return nil, err
	}
	item["SK"] = &types.AttributeValueMemberS{Value: locSK(l.ID)}
	return item, nil
}

// CountLocations は所有者の地点の数。
func (s *Store) CountLocations(ctx context.Context, ownerKey string) (int, error) {
	out, err := s.DB.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.Tables.App,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: ownerKey},
			":sk": &types.AttributeValueMemberS{Value: "LOC#"},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, err
	}
	return int(out.Count), nil
}

// ListLocations は所有者の地点の一覧（アクセスパターン #1）。
func (s *Store) ListLocations(ctx context.Context, ownerKey string) ([]Location, error) {
	p := dynamodb.NewQueryPaginator(s.DB, &dynamodb.QueryInput{
		TableName:              &s.Tables.App,
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: ownerKey},
			":sk": &types.AttributeValueMemberS{Value: "LOC#"},
		},
	})
	var out []Location
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		var ls []Location
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &ls); err != nil {
			return nil, err
		}
		out = append(out, ls...)
	}
	return out, nil
}

// GetLocation は所有者の地点を1件取得する。他人の地点は ErrNotFound になる（キーに所有者を含むため）。
func (s *Store) GetLocation(ctx context.Context, ownerKey, id string) (Location, error) {
	out, err := s.DB.GetItem(ctx, &dynamodb.GetItemInput{TableName: &s.Tables.App, Key: key(ownerKey, locSK(id))})
	if err != nil {
		return Location{}, err
	}
	if out.Item == nil {
		return Location{}, ErrNotFound
	}
	var l Location
	err = attributevalue.UnmarshalMap(out.Item, &l)
	return l, err
}

// CreateLocation は地点と逆引きインデックスを同時に書き込む。
func (s *Store) CreateLocation(ctx context.Context, l Location) error {
	item, err := s.marshalLocation(l)
	if err != nil {
		return err
	}
	areas, err := s.areaPut(l)
	if err != nil {
		return err
	}
	tx := append([]types.TransactWriteItem{{Put: &types.Put{TableName: &s.Tables.App, Item: item, ConditionExpression: aws.String("attribute_not_exists(PK)")}}}, areas...)
	_, err = s.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: tx})
	if isConditionFailed(err) {
		return ErrConflict
	}
	return err
}

// UpdateLocation は version が一致するときだけ更新する（楽観ロック）。
// エリアが変わったときは古い逆引きを消し、新しい設定のコピーで逆引きを書き直す。
func (s *Store) UpdateLocation(ctx context.Context, old, l Location) error {
	item, err := s.marshalLocation(l)
	if err != nil {
		return err
	}
	tx := []types.TransactWriteItem{{Put: &types.Put{
		TableName:                 &s.Tables.App,
		Item:                      item,
		ConditionExpression:       aws.String("version = :v"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberN{Value: itoa(old.Version)}},
	}}}
	for kind, code := range old.areaEntries() {
		if l.areaEntries()[kind] != code {
			tx = append(tx, types.TransactWriteItem{Delete: &types.Delete{TableName: &s.Tables.App, Key: key(areaPK(kind, code), locSK(l.ID))}})
		}
	}
	areas, err := s.areaPut(l)
	if err != nil {
		return err
	}
	tx = append(tx, areas...)
	_, err = s.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: tx})
	if isConditionFailed(err) {
		return ErrConflict
	}
	return err
}

// DeleteLocation は地点と逆引きインデックスを消す。タイムラインは TTL で消える。
func (s *Store) DeleteLocation(ctx context.Context, l Location) error {
	tx := append([]types.TransactWriteItem{{Delete: &types.Delete{
		TableName:           &s.Tables.App,
		Key:                 key(l.OwnerKey, locSK(l.ID)),
		ConditionExpression: aws.String("attribute_exists(PK)"),
	}}}, s.areaDelete(l)...)
	_, err := s.DB.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: tx})
	if isConditionFailed(err) {
		return ErrNotFound
	}
	return err
}

// QueryArea はエリアコードに該当する地点を引く（アクセスパターン #5・#6）。
func (s *Store) QueryArea(ctx context.Context, kind AreaKind, code string) ([]AreaEntry, error) {
	p := dynamodb.NewQueryPaginator(s.DB, &dynamodb.QueryInput{
		TableName:                 &s.Tables.App,
		KeyConditionExpression:    aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: areaPK(kind, code)}},
	})
	var out []AreaEntry
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		var es []AreaEntry
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &es); err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}

// UserIDFromOwnerKey は OWNER#U#<userId> から userId を取り出す。組織の所有なら空を返す。
func UserIDFromOwnerKey(ownerKey string) string {
	id, ok := strings.CutPrefix(ownerKey, "OWNER#U#")
	if !ok {
		return ""
	}
	return id
}
