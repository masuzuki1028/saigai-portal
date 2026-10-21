package domain

import "time"

// EventKind は正規化した公式情報の種類。
type EventKind string

const (
	EventWarning    EventKind = "warning"    // 気象警報・注意報
	EventEarthquake EventKind = "earthquake" // 震源・震度に関する情報
	EventLandslide  EventKind = "landslide"  // 土砂災害警戒情報（Warnings に入れる）
	EventTsunami    EventKind = "tsunami"    // 津波警報・注意報・予報
)

// Event は気象庁などの公式情報を正規化したもの（docs/04_data_model.md 5.3）。
type Event struct {
	ID        string    // sha256(source + 電文の ID)。冪等化に使う
	Source    string    // jma_xml など
	Kind      EventKind //
	Title     string    // 例: 新潟県気象警報・注意報
	Headline  string    // 見出し文
	InfoType  string    // 発表 / 訂正 / 取消
	IssuedAt  time.Time // 発表時刻（ReportDateTime）
	ReportURL string    // 電文の URL

	Warnings []AreaWarning // Kind=warning / landslide のとき
	Quake    *Quake        // Kind=earthquake のとき
	Tsunami  *TsunamiInfo  // Kind=tsunami のとき
}

// TsunamiInfo は津波予報区ごとの津波警報・注意報・予報。電文に含まれない予報区は変化なしとして扱う。
type TsunamiInfo struct {
	EventID string
	Areas   []TsunamiArea
}

// TsunamiArea は1つの津波予報区の情報。
type TsunamiArea struct {
	Code        string // 津波予報区コード（例: 340）
	Name        string
	KindCode    string // 例: 51 = 津波警報
	KindName    string
	Severity    Severity
	ArrivalTime string // 到達予想時刻・状況（表示用）
	MaxHeight   string // 予想される高さ（表示用。例: 3m、巨大）
}

// AreaWarning は1つの区域（市町村等）に対する警報・注意報の状態。
type AreaWarning struct {
	AreaCode string // 7桁の市町村等コード（例: 1520200）
	AreaName string
	Kinds    []WarningKind
}

// WarningKind は1つの警報・注意報の種類と状態。
type WarningKind struct {
	Code     string   // 気象庁のコード（例: 03 = 大雨警報）
	Name     string   // 例: 大雨警報
	Status   string   // 発表 / 継続 / 解除 など
	Notes    []string // 例: 土砂災害警戒、浸水害警戒
	Hazard   Hazard   // 対応する種別。対象外の種類は空
	Severity Severity // 対応する重要度
	Active   bool     // 解除されていないか
}

// Quake は地震の震源と市町村ごとの震度。
type Quake struct {
	EventID    string // 気象庁の地震の ID（同じ地震の続報で共通）
	Serial     string
	OriginTime time.Time
	Hypocenter string // 震央地名
	Magnitude  string // 表示用（例: M4.0）
	MaxInt     Intensity
	Cities     []CityIntensity
}

// CityIntensity は市町村ごとの最大震度。
type CityIntensity struct {
	Code   string // 7桁の市町村等コード（例: 0120600）
	Name   string
	MaxInt Intensity
}
