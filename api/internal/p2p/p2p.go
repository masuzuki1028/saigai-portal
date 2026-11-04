// Package p2p は P2P地震情報 API（https://www.p2pquake.net/develop/json_api_v2/）から地震情報を取り込む。
// 気象庁 XML の取り込みが止まったときの補完・冗長化に使う（docs/05_system_architecture.md 3.2）。
//
// 同じ地震は domain.Quake.Key（発生時刻（分）＋震央地名）で識別するため、気象庁 XML と両方から届いても
// タイムライン・通知は重複しない。津波の情報は気象庁 XML だけで扱う。
//
// 出典: P2P地震情報（気象庁の発表を配信）
package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
)

const (
	Source = "p2p_quake"
	// HistoryURL は地震情報（code=551）の履歴。1分ごとの取得はレート制限（60回/分）の範囲内
	HistoryURL = "https://api.p2pquake.net/v2/history?codes=551&limit=20"
)

type item struct {
	Code       int    `json:"code"`
	ID         string `json:"id"`
	Earthquake struct {
		Time       string `json:"time"`
		MaxScale   int    `json:"maxScale"`
		Hypocenter struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Magnitude float64 `json:"magnitude"`
		} `json:"hypocenter"`
	} `json:"earthquake"`
	Issue struct {
		Time    string `json:"time"`
		Type    string `json:"type"`
		Correct string `json:"correct"`
	} `json:"issue"`
	Points []struct {
		Pref   string `json:"pref"`
		Addr   string `json:"addr"`
		IsArea bool   `json:"isArea"`
		Scale  int    `json:"scale"`
	} `json:"points"`
}

// scales は P2P地震情報の震度の値 → 震度。46（震度5弱以上と推定、未入電）と -1（不明）は扱わない。
var scales = map[int]domain.Intensity{10: "1", 20: "2", 30: "3", 40: "4", 45: "5-", 50: "5+", 55: "6-", 60: "6+", 70: "7"}

var jst = time.FixedZone("JST", 9*60*60)

// EventID は冪等化用の ID。
func EventID(p2pID string) string {
	sum := sha256.Sum256([]byte(Source + "\x00" + p2pID))
	return hex.EncodeToString(sum[:])
}

// Parse は履歴の JSON を domain.Event にする（古い順）。市区町村の震度がない情報（震度速報など）は除く。
func Parse(data []byte) ([]*domain.Event, error) {
	var items []item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode p2p history: %w", err)
	}
	var out []*domain.Event
	for _, it := range items {
		if it.Code != 551 {
			continue
		}
		ev, err := toEvent(it)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].IssuedAt.Before(out[j].IssuedAt) })
	return out, nil
}

func toEvent(it item) (*domain.Event, error) {
	issued, err := time.ParseInLocation("2006/01/02 15:04:05", it.Issue.Time, jst)
	if err != nil {
		return nil, fmt.Errorf("parse issue.time %q: %w", it.Issue.Time, err)
	}
	origin, err := time.ParseInLocation("2006/01/02 15:04:05", it.Earthquake.Time, jst)
	if err != nil {
		return nil, fmt.Errorf("parse earthquake.time %q: %w", it.Earthquake.Time, err)
	}
	h := it.Earthquake.Hypocenter
	q := &domain.Quake{
		EventID: "p2p:" + it.ID, Key: domain.QuakeKey(origin, h.Name), OriginTime: origin, Hypocenter: h.Name,
		Magnitude: "Ｍ不明", MaxInt: scales[it.Earthquake.MaxScale],
	}
	if h.Magnitude > 0 {
		q.Magnitude = fmt.Sprintf("Ｍ%.1f", h.Magnitude)
	}
	if h.Latitude > -90 && h.Longitude > -180 { // 不明のときは -200
		q.Epicenter = &domain.LatLon{Lat: h.Latitude, Lon: h.Longitude}
	}
	// 観測点ごとの震度を、市区町村ごとの最大震度にまとめる
	maxByMuni := map[string]domain.Intensity{}
	for _, p := range it.Points {
		in, ok := scales[p.Scale]
		if p.IsArea || !ok {
			continue
		}
		code, ok := area.MuniFromStation(p.Pref, p.Addr)
		if !ok {
			continue
		}
		if in.Rank() > maxByMuni[code].Rank() {
			maxByMuni[code] = in
		}
	}
	if len(maxByMuni) == 0 {
		return nil, nil
	}
	codes := make([]string, 0, len(maxByMuni))
	for c := range maxByMuni {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		a, err := area.FromMuniCode(c)
		if err != nil {
			continue
		}
		// 気象庁の市町村等コード（7桁）の形にそろえる
		q.Cities = append(q.Cities, domain.CityIntensity{Code: c + "00", Name: a.Name, MaxInt: maxByMuni[c]})
	}
	infoType := "発表"
	if it.Issue.Correct != "" && it.Issue.Correct != "None" {
		infoType = "訂正"
	}
	return &domain.Event{
		ID: EventID(it.ID), Source: Source, Kind: domain.EventEarthquake, Title: "震源・震度情報（P2P地震情報）",
		Headline: fmt.Sprintf("%sころ、地震がありました。", origin.Format("2日15時04分")),
		InfoType: infoType, IssuedAt: issued, ReportURL: "https://www.p2pquake.net/", Quake: q,
	}, nil
}

// Store は取り込みに使う store の操作。
type Store interface {
	PutEventIfAbsent(ctx context.Context, ev *domain.Event) (bool, error)
	DeleteEvent(ctx context.Context, id string) error
}

// Ingester は P2P地震情報の履歴を取得して取り込む。
type Ingester struct {
	Store   Store
	Client  *http.Client
	URL     string
	OnEvent func(ctx context.Context, ev *domain.Event) error
	Log     *slog.Logger
}

func NewIngester(s Store, onEvent func(context.Context, *domain.Event) error, log *slog.Logger) *Ingester {
	return &Ingester{Store: s, Client: &http.Client{Timeout: 10 * time.Second}, URL: HistoryURL, OnEvent: onEvent, Log: log}
}

// Poll は履歴を1回取得し、新しい情報を取り込んだ件数を返す。
func (in *Ingester) Poll(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return 0, err
	}
	res, err := in.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("p2p history: status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return 0, err
	}
	return in.Ingest(ctx, data)
}

// Ingest は履歴の JSON を取り込む。replay からも使う。
func (in *Ingester) Ingest(ctx context.Context, data []byte) (int, error) {
	events, err := Parse(data)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, ev := range events {
		created, err := in.Store.PutEventIfAbsent(ctx, ev)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !created {
			continue
		}
		n++
		if in.OnEvent != nil {
			if err := in.OnEvent(ctx, ev); err != nil {
				// 次の取り込みでもう一度処理する
				errs = append(errs, err, in.Store.DeleteEvent(ctx, ev.ID))
			}
		}
	}
	return n, errors.Join(errs...)
}
