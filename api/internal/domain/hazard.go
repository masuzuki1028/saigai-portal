// Package domain は災害情報・地点・タイムラインのドメインの型と判定ロジック。
// AWS や HTTP には依存しない。
package domain

import (
	"errors"
	"fmt"
)

// Hazard は災害の種別（docs/04_data_model.md 4章）。
type Hazard string

const (
	Rain       Hazard = "rain"
	Flood      Hazard = "flood"
	Landslide  Hazard = "landslide"
	Earthquake Hazard = "earthquake"
	Tsunami    Hazard = "tsunami"
)

// Hazards は表示順に並べた全種別。
var Hazards = []Hazard{Rain, Flood, Landslide, Earthquake, Tsunami}

// Severity は共通の重要度。0 はなし（または OFF）。
type Severity int

const (
	SeverityNone      Severity = 0
	SeverityInfo      Severity = 1
	SeverityAdvisory  Severity = 2 // 注意報
	SeverityWarning   Severity = 3 // 警報
	SeverityEmergency Severity = 4 // 特別警報
	maxSeverity                = SeverityEmergency
)

// Intensity は震度。"5-" は5弱、"5+" は5強。"off" は OFF を表す。
type Intensity string

const IntensityOff Intensity = "off"

var intensityRanks = map[Intensity]int{
	"1": 1, "2": 2, "3": 3, "4": 4, "5-": 5, "5+": 6, "6-": 7, "6+": 8, "7": 9,
}

// Rank は震度の大小比較用の値。OFF と不明な値は 0。
func (i Intensity) Rank() int { return intensityRanks[i] }

// Valid は震度として有効な値（または OFF）か。
func (i Intensity) Valid() bool { return i == IntensityOff || i.Rank() > 0 }

// AtLeast は i が min 以上か。min が OFF のときは常に false。
func (i Intensity) AtLeast(min Intensity) bool {
	return min.Rank() > 0 && i.Rank() >= min.Rank()
}

// Severity は震度を共通の重要度に変換する（docs/04_data_model.md 4.1）。
func (i Intensity) Severity() Severity {
	switch r := i.Rank(); {
	case r >= intensityRanks["6-"]:
		return SeverityEmergency
	case r >= intensityRanks["5-"]:
		return SeverityWarning
	case r >= intensityRanks["3"]:
		return SeverityAdvisory
	case r >= 1:
		return SeverityInfo
	default:
		return SeverityNone
	}
}

// Label は画面・通知に出す震度の表記。
func (i Intensity) Label() string {
	switch i {
	case "5-":
		return "震度5弱"
	case "5+":
		return "震度5強"
	case "6-":
		return "震度6弱"
	case "6+":
		return "震度6強"
	case IntensityOff, "":
		return "なし"
	default:
		return "震度" + string(i)
	}
}

// LevelSetting は「この severity 以上を表示する / 通知する」の設定。0 は OFF。
type LevelSetting struct {
	Display Severity `dynamodbav:"display" json:"display"`
	Notify  Severity `dynamodbav:"notify" json:"notify"`
}

// EarthquakeSetting は地震のしきい値。severity より細かく震度で選べる。
type EarthquakeSetting struct {
	DisplayMinIntensity Intensity `dynamodbav:"displayMinIntensity" json:"displayMinIntensity"`
	NotifyMinIntensity  Intensity `dynamodbav:"notifyMinIntensity" json:"notifyMinIntensity"`
}

// HazardSettings は地点ごとの災害種別×レベルの設定（docs/01_concept_overview.md 4.2）。
type HazardSettings struct {
	Rain       LevelSetting      `dynamodbav:"rain" json:"rain"`
	Flood      LevelSetting      `dynamodbav:"flood" json:"flood"`
	Landslide  LevelSetting      `dynamodbav:"landslide" json:"landslide"`
	Earthquake EarthquakeSetting `dynamodbav:"earthquake" json:"earthquake"`
	Tsunami    LevelSetting      `dynamodbav:"tsunami" json:"tsunami"`
}

// DefaultHazardSettings は新しい地点の初期値。
func DefaultHazardSettings() HazardSettings {
	return HazardSettings{
		Rain:       LevelSetting{Display: SeverityAdvisory, Notify: SeverityWarning},
		Flood:      LevelSetting{Display: SeverityAdvisory, Notify: SeverityWarning},
		Landslide:  LevelSetting{Display: SeverityWarning, Notify: SeverityWarning},
		Earthquake: EarthquakeSetting{DisplayMinIntensity: "3", NotifyMinIntensity: "5-"},
		Tsunami:    LevelSetting{Display: SeverityAdvisory, Notify: SeverityWarning},
	}
}

// Level は地震以外の種別の設定を返す。
func (h HazardSettings) Level(hz Hazard) LevelSetting {
	switch hz {
	case Rain:
		return h.Rain
	case Flood:
		return h.Flood
	case Landslide:
		return h.Landslide
	case Tsunami:
		return h.Tsunami
	default:
		return LevelSetting{}
	}
}

var ErrInvalidSettings = errors.New("invalid hazard settings")

// Validate は設定値の範囲を確認する。
func (h HazardSettings) Validate() error {
	for _, hz := range []Hazard{Rain, Flood, Landslide, Tsunami} {
		l := h.Level(hz)
		if l.Display < 0 || l.Display > maxSeverity || l.Notify < 0 || l.Notify > maxSeverity {
			return fmt.Errorf("%w: %s", ErrInvalidSettings, hz)
		}
	}
	if !h.Earthquake.DisplayMinIntensity.Valid() || !h.Earthquake.NotifyMinIntensity.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidSettings, Earthquake)
	}
	return nil
}
