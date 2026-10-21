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
	PutAffected(ctx context.Context, a store.Affected) error
	ListAffected(ctx context.Context, group string) ([]store.Affected, error)
	CancelTimelineItem(ctx context.Context, locID, itemID string, issuedAt time.Time) error
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
		g := groupOf(ev)
		if g == "" {
			// 警報・注意報と土砂災害警戒情報は、毎回すべての区域の状態が入った電文が届くため、
			// 取消報のあとの電文で正しい状態に戻る。取消報そのものは記録だけ残す
			m.Log.Warn("取消報を記録した（次の電文で状態を戻す）", "eventId", ev.ID, "title", ev.Title)
			return Result{}, nil
		}
		return m.cancel(ctx, ev, g)
	}
	switch ev.Kind {
	case domain.EventWarning:
		return m.matchWarning(ctx, ev, []domain.Hazard{domain.Rain, domain.Flood, domain.Landslide}, "")
	case domain.EventLandslide:
		// 土砂災害警戒情報は、大雨警報（土砂災害）とは別の状態として持つ（互いの更新で上書きしない）
		return m.matchWarning(ctx, ev, []domain.Hazard{domain.Landslide}, ":alert")
	case domain.EventEarthquake:
		return m.matchQuake(ctx, ev)
	case domain.EventTsunami:
		return m.matchTsunami(ctx, ev)
	default:
		return Result{}, fmt.Errorf("unknown event kind %q", ev.Kind)
	}
}

// groupOf は、続報・取消報で共通の ID（地震・津波）。それ以外の電文は空。
func groupOf(ev *domain.Event) string {
	switch {
	case ev.Kind == domain.EventEarthquake && ev.Quake != nil && ev.Quake.EventID != "":
		return "quake:" + ev.Quake.EventID
	case ev.Kind == domain.EventTsunami && ev.Tsunami != nil && ev.Tsunami.EventID != "":
		return "tsunami:" + ev.Tsunami.EventID
	}
	return ""
}

// cancel は取消報を処理する。同じ地震・津波で出した項目を「取消」にし、現在のレベルから外す。
// 通知済みの地点には、取り消されたことを通知する（誤った情報のままにしない）。
func (m *Matcher) cancel(ctx context.Context, ev *domain.Event, group string) (Result, error) {
	var res Result
	affected, err := m.Store.ListAffected(ctx, group)
	if err != nil {
		return res, err
	}
	hz, stKey := domain.Tsunami, string(domain.Tsunami)
	if ev.Kind == domain.EventEarthquake {
		hz, stKey = domain.Earthquake, "Q#"+ev.Quake.EventID
	}
	notified := map[string]store.Affected{}
	reset := map[string]bool{}
	for _, a := range affected {
		if err := m.Store.CancelTimelineItem(ctx, a.LocationID, a.ItemID, a.IssuedAt); err != nil {
			return res, err
		}
		res.Items = append(res.Items, a.LocationName+": 取消: "+a.Title)
		if a.Notified {
			notified[a.LocationID] = a
		}
		if !reset[a.LocationID] {
			reset[a.LocationID] = true
			st := store.HazardState{Key: stKey, Hazard: hz, Severity: 0, Title: "取消", ReportTime: ev.IssuedAt}
			if _, err := m.Store.PutState(ctx, a.LocationID, st); err != nil {
				return res, err
			}
		}
	}
	for _, a := range sortedAffected(notified) {
		res.Jobs = append(res.Jobs, Job{
			OwnerKey: a.OwnerKey, LocationID: a.LocationID, LocationName: a.LocationName, Hazard: hz,
			Title: a.Title, Summary: "気象庁が「" + a.Title + "」の情報を取り消しました。" + ev.Headline,
			Status: domain.StatusCancelled, IssuedAt: ev.IssuedAt,
		})
	}
	m.Log.Info("cancelled", "eventId", ev.ID, "group", group, "items", len(affected), "notify", len(res.Jobs))
	return res, nil
}

func sortedAffected(m map[string]store.Affected) []store.Affected {
	out := make([]store.Affected, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LocationID < out[j].LocationID })
	return out
}

// areaLevel は1つの突き合わせキー（市町村）の種別ごとの状態。
type areaLevel struct {
	name     string
	severity map[domain.Hazard]domain.Severity
	kinds    map[domain.Hazard][]string // 発表中の種類名（表示用）
}

// matchWarning は市町村ごとに発表される情報（警報・注意報、土砂災害警戒情報）を処理する。
// stateSuffix は状態のキーの接尾辞（同じ種別を別の電文の系列で持つときに使う）。
func (m *Matcher) matchWarning(ctx context.Context, ev *domain.Event, hazards []domain.Hazard, stateSuffix string) (Result, error) {
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
	for _, k := range sortedKeys(levels) {
		al := levels[k]
		entries, err := m.Store.QueryArea(ctx, store.AreaWarn, k)
		if err != nil {
			return res, err
		}
		for _, e := range entries {
			for _, hz := range hazards {
				lc := levelChange{
					stateKey: string(hz) + stateSuffix, hazard: hz, severity: al.severity[hz], kinds: al.kinds[hz],
					areaName: al.name, clearedTitle: hazardLabel(hz) + "の警報・注意報の解除", sourceURL: "https://www.jma.go.jp/bosai/warning/",
				}
				if stateSuffix == ":alert" {
					lc.clearedTitle = "土砂災害警戒情報の解除"
					lc.sourceURL = "https://www.jma.go.jp/bosai/map.html#contents=landslide"
				}
				if err := m.applyLevel(ctx, ev, e, lc, &res); err != nil {
					return res, err
				}
			}
		}
	}
	return res, nil
}

// matchTsunami は津波予報区ごとの津波警報・注意報・予報を、沿岸の地点に突き合わせる。
// 電文に含まれない予報区は変化なしとして扱う（解除は「警報解除」などの区分で届く）。
func (m *Matcher) matchTsunami(ctx context.Context, ev *domain.Event) (Result, error) {
	var res Result
	for _, ta := range ev.Tsunami.Areas {
		entries, err := m.Store.QueryArea(ctx, store.AreaTsunami, ta.Code)
		if err != nil {
			return res, err
		}
		var kinds []string
		if ta.Severity > 0 {
			kinds = []string{ta.KindName}
		}
		detail := ""
		if ta.ArrivalTime != "" || ta.MaxHeight != "" {
			detail = strings.TrimSpace(fmt.Sprintf("%s。予想される津波の高さ: %s", ta.ArrivalTime, orUnknown(ta.MaxHeight)))
		}
		for _, e := range entries {
			lc := levelChange{
				stateKey: string(domain.Tsunami), hazard: domain.Tsunami, severity: ta.Severity, kinds: kinds,
				areaName: ta.Name, clearedTitle: "津波警報・注意報の解除", detail: detail,
				sourceURL: "https://www.jma.go.jp/bosai/map.html#contents=tsunami",
			}
			if err := m.applyLevel(ctx, ev, e, lc, &res); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

func orUnknown(s string) string {
	if s == "" {
		return "不明"
	}
	return s
}

// levelChange は、ある地点・種別の新しいレベル。
type levelChange struct {
	stateKey     string
	hazard       domain.Hazard
	severity     domain.Severity
	kinds        []string // 発表中の種類名
	areaName     string   // 発表区域の名前
	clearedTitle string   // 解除のときの表題
	detail       string   // 要約に付け加える内容（津波の到達予想時刻など）
	sourceURL    string
}

// applyLevel は状態の変化を判定し、タイムラインへの追加と通知の依頼を行う（docs/05 3.3）。
func (m *Matcher) applyLevel(ctx context.Context, ev *domain.Event, e store.AreaEntry, lc levelChange, res *Result) error {
	old, err := m.Store.GetState(ctx, e.LocationID, lc.stateKey)
	if err != nil {
		return err
	}
	if !old.ReportTime.IsZero() && ev.IssuedAt.Before(old.ReportTime) {
		return nil // 古い電文（到着順の入れ替わり）
	}
	kinds := append([]string(nil), lc.kinds...)
	sort.Strings(kinds)
	sig := strings.Join(kinds, ",")
	if lc.severity == old.Severity && sig == old.Signature {
		return nil // 変化なし（継続の電文）。書き込みも行わない
	}
	d := domain.Decide(old.Severity, lc.severity, sig != old.Signature && lc.severity > 0, e.HazardSettings.Level(lc.hazard))

	title := strings.Join(kinds, "・")
	if lc.severity == 0 {
		title = lc.clearedTitle
	}
	st := store.HazardState{Key: lc.stateKey, Hazard: lc.hazard, Severity: lc.severity, Title: title, Signature: sig, ReportTime: ev.IssuedAt}
	applied, err := m.Store.PutState(ctx, e.LocationID, st)
	if err != nil || !applied {
		return err
	}
	if !d.AddItem && !d.Notify {
		return nil
	}
	sev := lc.severity
	if d.Status == domain.StatusCleared {
		sev = old.Severity // 解除の項目は、解除前のレベルの色で出す
	}
	summary := fmt.Sprintf("%s（%s）。", title, lc.areaName)
	if lc.detail != "" && lc.severity > 0 {
		summary += lc.detail + "。"
	}
	summary += ev.Headline
	item := store.TimelineItem{
		ID: itemID(ev.ID, e.LocationID, lc.stateKey), Kind: "official", EventID: ev.ID, Hazard: lc.hazard, Severity: sev,
		Reliability: "official", Title: title, Summary: summary, Status: d.Status, Source: sourceJMA,
		SourceURL: lc.sourceURL, IssuedAt: ev.IssuedAt, Notified: d.Notify,
	}
	return m.emit(ctx, ev, e, item, d, res)
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
// 訂正報では震度が下がる場合も反映し、「訂正」の項目を出す。通知済みの地点には訂正も通知する。
func (m *Matcher) applyQuake(ctx context.Context, ev *domain.Event, e store.AreaEntry, c domain.CityIntensity, res *Result) error {
	set := e.HazardSettings.Earthquake
	stKey := "Q#" + ev.Quake.EventID
	old, err := m.Store.GetState(ctx, e.LocationID, stKey)
	if err != nil {
		return err
	}
	correction := ev.InfoType == "訂正" && old.Intensity != "" && old.Intensity != c.MaxInt
	if old.Intensity.Rank() >= c.MaxInt.Rank() && !correction {
		return nil
	}
	display := c.MaxInt.AtLeast(set.DisplayMinIntensity)
	notify := c.MaxInt.AtLeast(set.NotifyMinIntensity) && (!old.Notified || c.MaxInt.Severity() > old.Severity)
	status := domain.StatusIssued
	if correction {
		status = domain.StatusCorrected
		display = display || old.Intensity.AtLeast(set.DisplayMinIntensity)
		notify = notify || old.Notified
	}
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
		Severity: max(c.MaxInt.Severity(), domain.SeverityInfo), Reliability: "official", Title: title, Summary: summary, Status: status,
		Source: sourceJMA, SourceURL: "https://www.jma.go.jp/bosai/map.html#contents=earthquake_map", IssuedAt: ev.IssuedAt, Notified: notify,
	}
	return m.emit(ctx, ev, e, item, domain.Decision{AddItem: display, Status: status, Notify: notify}, res)
}

func (m *Matcher) emit(ctx context.Context, ev *domain.Event, e store.AreaEntry, item store.TimelineItem, d domain.Decision, res *Result) error {
	if d.AddItem {
		if err := m.Store.PutTimelineItem(ctx, e.LocationID, item); err != nil {
			return err
		}
		res.Items = append(res.Items, e.Name+": "+item.Title+statusSuffix(item.Status))
		// 取消報で取り消せるよう、地震・津波の項目を出した地点を記録する
		if g := groupOf(ev); g != "" {
			a := store.Affected{Group: g, LocationID: e.LocationID, OwnerKey: e.OwnerKey, LocationName: e.Name, ItemID: item.ID, IssuedAt: item.IssuedAt, Title: item.Title, Notified: d.Notify}
			if err := m.Store.PutAffected(ctx, a); err != nil {
				return err
			}
		}
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

func statusSuffix(s domain.ItemStatus) string {
	if s == domain.StatusCorrected {
		return "（訂正）"
	}
	return ""
}

// itemID は電文・地点・種別から決まる ID。再処理しても同じ ID になり、重複して書き込まない。
func itemID(eventID, locID, part string) string {
	sum := sha256.Sum256([]byte(eventID + "|" + locID + "|" + part))
	return hex.EncodeToString(sum[:])[:26]
}

func kindLabel(wk domain.WarningKind) string {
	if wk.Hazard == domain.Landslide && wk.Code != "3" { // 3 は土砂災害警戒情報（名前のまま）
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
