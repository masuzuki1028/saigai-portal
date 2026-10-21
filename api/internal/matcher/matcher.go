// Package matcher は正規化した公式情報を地点と突き合わせ、タイムラインへの追加と通知を判定する
// （docs/05_system_architecture.md 3.3）。
package matcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// Job は notifier に渡す通知の依頼。座標・住所は含めない。
type Job struct {
	OwnerKey     string            `json:"ownerKey"`
	LocationID   string            `json:"locationId"`
	LocationName string            `json:"locationName"`
	Hazard       domain.Hazard     `json:"hazard"`
	Severity     domain.Severity   `json:"severity"`
	Title        string            `json:"title"`
	Summary      string            `json:"summary"`
	Status       domain.ItemStatus `json:"status"`
	IssuedAt     time.Time         `json:"issuedAt"`
	ItemID       string            `json:"itemId"`
}

// Store は matcher が使う store の操作。
type Store interface {
	QueryArea(ctx context.Context, kind store.AreaKind, code string) ([]store.AreaEntry, error)
	GetState(ctx context.Context, locID, key string) (store.HazardState, error)
	PutState(ctx context.Context, locID string, st store.HazardState) (bool, error)
	PutTimelineItem(ctx context.Context, locID string, it store.TimelineItem) error
}

// Matcher は Event を地点に突き合わせる。Notify には地点ごとにまとめた通知の依頼を渡す
// （本番は notify-queue への送信、ローカルは直接呼び出し）。1つの電文で大雨警報と洪水警報が同時に出ても、
// 1つの地点への通知は1通にまとめる。
type Matcher struct {
	Store  Store
	Notify func(ctx context.Context, jobs []Job) error
	Log    *slog.Logger
}

const sourceJMA = "気象庁"

// Result は1つの電文の処理結果（replay とテストで使う）。
type Result struct {
	Items []string // 追加したタイムライン項目（地点名: タイトル）
	Jobs  []Job
}

// Match は Event を処理する。同じ Event を2回処理しても、タイムラインと通知は重複しない。
func (m *Matcher) Match(ctx context.Context, ev *domain.Event) (Result, error) {
	res, err := m.match(ctx, ev)
	if err != nil || m.Notify == nil {
		return res, err
	}
	byLoc := map[string][]Job{}
	var order []string
	for _, j := range res.Jobs {
		if _, ok := byLoc[j.LocationID]; !ok {
			order = append(order, j.LocationID)
		}
		byLoc[j.LocationID] = append(byLoc[j.LocationID], j)
	}
	for _, id := range order {
		if err := m.Notify(ctx, byLoc[id]); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (m *Matcher) match(ctx context.Context, ev *domain.Event) (Result, error) {
	if ev.InfoType == "取消" {
		// 取消報の扱い（取り消された情報の表示を消すか）は Phase 2 で決める。MVP では無視して記録だけ残す
		m.Log.Warn("取消報を無視した", "eventId", ev.ID, "title", ev.Title)
		return Result{}, nil
	}
	switch ev.Kind {
	case domain.EventWarning:
		return m.matchWarning(ctx, ev)
	case domain.EventEarthquake:
		return m.matchQuake(ctx, ev)
	default:
		return Result{}, fmt.Errorf("unknown event kind %q", ev.Kind)
	}
}

// areaLevel は1つの突き合わせキー（市町村）の種別ごとの状態。
type areaLevel struct {
	name     string
	severity map[domain.Hazard]domain.Severity
	kinds    map[domain.Hazard][]string // 発表中の種類名（表示用）
}

func (m *Matcher) matchWarning(ctx context.Context, ev *domain.Event) (Result, error) {
	// 市町村の中の区域（富山市平地・山間部など）を市町村にまとめ、種別ごとに最大のレベルを取る
	levels := map[string]*areaLevel{}
	for _, aw := range ev.Warnings {
		k := area.KeyFromJMACode(aw.AreaCode)
		al, ok := levels[k]
		if !ok {
			al = &areaLevel{name: aw.AreaName, severity: map[domain.Hazard]domain.Severity{}, kinds: map[domain.Hazard][]string{}}
			levels[k] = al
		}
		for _, wk := range aw.Kinds {
			if wk.Hazard == "" || !wk.Active {
				continue
			}
			if wk.Severity > al.severity[wk.Hazard] {
				al.severity[wk.Hazard] = wk.Severity
			}
			if !contains(al.kinds[wk.Hazard], kindLabel(wk)) {
				al.kinds[wk.Hazard] = append(al.kinds[wk.Hazard], kindLabel(wk))
			}
		}
	}

	var res Result
	keys := sortedKeys(levels)
	for _, k := range keys {
		al := levels[k]
		entries, err := m.Store.QueryArea(ctx, store.AreaWarn, k)
		if err != nil {
			return res, err
		}
		for _, e := range entries {
			for _, hz := range []domain.Hazard{domain.Rain, domain.Flood, domain.Landslide} {
				if err := m.applyWarning(ctx, ev, e, al, hz, &res); err != nil {
					return res, err
				}
			}
		}
	}
	return res, nil
}

func (m *Matcher) applyWarning(ctx context.Context, ev *domain.Event, e store.AreaEntry, al *areaLevel, hz domain.Hazard, res *Result) error {
	old, err := m.Store.GetState(ctx, e.LocationID, string(hz))
	if err != nil {
		return err
	}
	if !old.ReportTime.IsZero() && ev.IssuedAt.Before(old.ReportTime) {
		return nil // 古い電文（到着順の入れ替わり）
	}
	newSev := al.severity[hz]
	kinds := al.kinds[hz]
	sort.Strings(kinds)
	sig := strings.Join(kinds, ",")
	if newSev == old.Severity && sig == old.Signature {
		return nil // 変化なし（継続の電文）。書き込みも行わない
	}
	d := domain.Decide(old.Severity, newSev, sig != old.Signature && newSev > 0, e.HazardSettings.Level(hz))

	title := strings.Join(kinds, "・")
	if newSev == 0 {
		title = hazardLabel(hz) + "の警報・注意報の解除"
	}
	st := store.HazardState{Key: string(hz), Hazard: hz, Severity: newSev, Title: title, Signature: sig, ReportTime: ev.IssuedAt}
	applied, err := m.Store.PutState(ctx, e.LocationID, st)
	if err != nil || !applied {
		return err
	}
	if !d.AddItem && !d.Notify {
		return nil
	}
	sev := newSev
	if d.Status == domain.StatusCleared {
		sev = old.Severity // 解除の項目は、解除前のレベルの色で出す
	}
	summary := fmt.Sprintf("%s（%s）。%s", title, al.name, ev.Headline)
	item := store.TimelineItem{
		ID: itemID(ev.ID, e.LocationID, string(hz)), Kind: "official", EventID: ev.ID, Hazard: hz, Severity: sev,
		Reliability: "official", Title: title, Summary: summary, Status: d.Status, Source: sourceJMA,
		SourceURL: "https://www.jma.go.jp/bosai/warning/", IssuedAt: ev.IssuedAt, Notified: d.Notify,
	}
	return m.emit(ctx, e, item, d, res)
}

func (m *Matcher) matchQuake(ctx context.Context, ev *domain.Event) (Result, error) {
	q := ev.Quake
	var res Result
	for _, c := range q.Cities {
		entries, err := m.Store.QueryArea(ctx, store.AreaQuake, area.KeyFromJMACode(c.Code))
		if err != nil {
			return res, err
		}
		for _, e := range entries {
			if err := m.applyQuake(ctx, ev, e, c, &res); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// applyQuake は地点の市町村の震度を判定する。同じ地震の続報（震度の追加・修正）では、
// 震度が大きくなったときだけ項目を追加する。通知は、初めて通知のしきい値に達したときと、
// レベル（severity）が上がったとき（例: 震度5強 → 6弱）に送る。
func (m *Matcher) applyQuake(ctx context.Context, ev *domain.Event, e store.AreaEntry, c domain.CityIntensity, res *Result) error {
	set := e.HazardSettings.Earthquake
	stKey := "Q#" + ev.Quake.EventID
	old, err := m.Store.GetState(ctx, e.LocationID, stKey)
	if err != nil {
		return err
	}
	if old.Intensity.Rank() >= c.MaxInt.Rank() {
		return nil
	}
	display := c.MaxInt.AtLeast(set.DisplayMinIntensity)
	notify := c.MaxInt.AtLeast(set.NotifyMinIntensity) && (!old.Notified || c.MaxInt.Severity() > old.Severity)
	title := fmt.Sprintf("地震（%s %s）", c.Name, c.MaxInt.Label())
	st := store.HazardState{
		Key: stKey, Hazard: domain.Earthquake, Severity: c.MaxInt.Severity(), Title: title,
		Intensity: c.MaxInt, ReportTime: ev.IssuedAt, Notified: old.Notified || notify,
	}
	applied, err := m.Store.PutState(ctx, e.LocationID, st)
	if err != nil || !applied {
		return err
	}
	if !display && !notify {
		return nil
	}
	summary := fmt.Sprintf("%s ころ、%s を震源とする地震がありました（%s）。%s で%sを観測しました。",
		ev.Quake.OriginTime.In(jst).Format("1月2日 15時04分"), ev.Quake.Hypocenter, ev.Quake.Magnitude, c.Name, c.MaxInt.Label())
	item := store.TimelineItem{
		ID: itemID(ev.ID, e.LocationID, string(c.MaxInt)), Kind: "official", EventID: ev.ID, Hazard: domain.Earthquake,
		Severity: c.MaxInt.Severity(), Reliability: "official", Title: title, Summary: summary, Status: domain.StatusIssued,
		Source: sourceJMA, SourceURL: "https://www.jma.go.jp/bosai/map.html#contents=earthquake_map", IssuedAt: ev.IssuedAt, Notified: notify,
	}
	return m.emit(ctx, e, item, domain.Decision{AddItem: display, Status: domain.StatusIssued, Notify: notify}, res)
}

func (m *Matcher) emit(ctx context.Context, e store.AreaEntry, item store.TimelineItem, d domain.Decision, res *Result) error {
	if d.AddItem {
		if err := m.Store.PutTimelineItem(ctx, e.LocationID, item); err != nil {
			return err
		}
		res.Items = append(res.Items, e.Name+": "+item.Title)
	}
	if d.Notify {
		j := Job{
			OwnerKey: e.OwnerKey, LocationID: e.LocationID, LocationName: e.Name, Hazard: item.Hazard, Severity: item.Severity,
			Title: item.Title, Summary: item.Summary, Status: item.Status, IssuedAt: item.IssuedAt, ItemID: item.ID,
		}
		res.Jobs = append(res.Jobs, j)
	}
	m.Log.Info("matched", "eventId", item.EventID, "locationId", e.LocationID, "hazard", item.Hazard, "severity", item.Severity, "status", item.Status, "notify", d.Notify)
	return nil
}

var jst = time.FixedZone("JST", 9*60*60)

// itemID は電文・地点・種別から決まる ID。再処理しても同じ ID になり、重複して書き込まない。
func itemID(eventID, locID, part string) string {
	sum := sha256.Sum256([]byte(eventID + "|" + locID + "|" + part))
	return hex.EncodeToString(sum[:])[:26]
}

func kindLabel(wk domain.WarningKind) string {
	if wk.Hazard == domain.Landslide {
		return wk.Name + "（土砂災害）"
	}
	return wk.Name
}

func hazardLabel(hz domain.Hazard) string {
	switch hz {
	case domain.Rain:
		return "大雨"
	case domain.Flood:
		return "洪水"
	case domain.Landslide:
		return "土砂災害"
	case domain.Earthquake:
		return "地震"
	case domain.Tsunami:
		return "津波"
	}
	return string(hz)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]*areaLevel) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
