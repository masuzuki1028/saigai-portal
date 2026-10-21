package area

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GSIBaseURL は国土地理院の逆ジオコーダ。
const GSIBaseURL = "https://mreversegeocoder.gsi.go.jp"

// GSIResolver は国土地理院の逆ジオコーダで市区町村を求める。
// 地点の登録時にだけ呼び、結果は地点に保存して繰り返し呼ばない（docs/05 3.4）。
type GSIResolver struct {
	BaseURL string // テストでは httptest.Server の URL に差し替える
	Client  *http.Client
}

func NewGSIResolver() *GSIResolver {
	return &GSIResolver{BaseURL: GSIBaseURL, Client: &http.Client{Timeout: 5 * time.Second}}
}

func (g *GSIResolver) Resolve(ctx context.Context, lat, lon float64) (Area, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.BaseURL+"/reverse-geocoder/LonLatToAddress?"+q.Encode(), nil)
	if err != nil {
		return Area{}, err
	}
	res, err := g.Client.Do(req)
	if err != nil {
		return Area{}, fmt.Errorf("reverse geocoder: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return Area{}, fmt.Errorf("reverse geocoder: status %d", res.StatusCode)
	}
	var body struct {
		Results *struct {
			MuniCd string `json:"muniCd"`
		} `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Area{}, fmt.Errorf("reverse geocoder: decode: %w", err)
	}
	if body.Results == nil || body.Results.MuniCd == "" {
		return Area{}, ErrNotFound
	}
	return FromMuniCode(body.Results.MuniCd)
}

// StaticResolver は、あらかじめ決めた地点の近くなら決まった市区町村を返す。
// 外部 API を呼ばずに動かしたいとき（テスト・オフラインでのローカル開発）に使う。
type StaticResolver struct {
	Points []StaticPoint
}

type StaticPoint struct {
	Lat, Lon float64
	MuniCode string
}

// DefaultStaticPoints は災害シナリオ（testdata/scenarios）で使う市区町村。
var DefaultStaticPoints = []StaticPoint{
	{Lat: 37.4462, Lon: 138.8512, MuniCode: "15202"}, // 新潟県長岡市
	{Lat: 37.9161, Lon: 139.0364, MuniCode: "15103"}, // 新潟市中央区
	{Lat: 36.6959, Lon: 137.2137, MuniCode: "16201"}, // 富山市
	{Lat: 35.6812, Lon: 139.7671, MuniCode: "13101"}, // 東京都千代田区
	{Lat: 35.4437, Lon: 139.6380, MuniCode: "14104"}, // 横浜市中区
	{Lat: 42.9849, Lon: 144.3820, MuniCode: "01206"}, // 釧路市
}

func (s StaticResolver) Resolve(_ context.Context, lat, lon float64) (Area, error) {
	best, bestD := "", 0.0
	for _, p := range s.Points {
		d := (p.Lat-lat)*(p.Lat-lat) + (p.Lon-lon)*(p.Lon-lon)
		if best == "" || d < bestD {
			best, bestD = p.MuniCode, d
		}
	}
	// 約 0.3 度（30km 程度）より離れていたら見つからない扱いにする
	if best == "" || bestD > 0.09 {
		return Area{}, ErrNotFound
	}
	return FromMuniCode(best)
}
