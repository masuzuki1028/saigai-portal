// Package jma は気象庁防災情報 XML（PULL 型 Atom フィードと電文）を解析し、domain.Event に正規化する。
//
// 対応している電文:
//   - 気象警報・注意報（Ｈ２７）（VPWW54）
//   - 土砂災害警戒情報（VXWW50）
//   - 震源・震度に関する情報（VXSE53）
//   - 津波警報・注意報・予報a（VTSE41）
//
// 出典: 気象庁防災情報 XML（https://xml.kishou.go.jp/）
package jma

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
)

const Source = "jma_xml"

// 取り込む電文の表題（Control/Title）。
const (
	TitleWarning   = "気象警報・注意報（Ｈ２７）"
	TitleLandslide = "土砂災害警戒情報"
	TitleQuake     = "震源・震度に関する情報"
	TitleTsunami   = "津波警報・注意報・予報a"
)

// ErrUnsupported は対応していない電文。
var ErrUnsupported = errors.New("unsupported telegram")

// EventID は電文の ID（フィードの entry/id = 電文の URL）から冪等化用の ID を作る。
func EventID(telegramID string) string {
	sum := sha256.Sum256([]byte(Source + "\x00" + telegramID))
	return hex.EncodeToString(sum[:])
}

//go:embed severity_map.json
var severityMapJSON []byte

type severityMap struct {
	Warnings map[string]struct {
		Name              string          `json:"name"`
		Hazard            domain.Hazard   `json:"hazard"`
		Severity          domain.Severity `json:"severity"`
		LandslideSeverity domain.Severity `json:"landslideSeverity"`
	} `json:"warnings"`
	LandslideNote  string `json:"landslideNote"`
	LandslideAlert map[string]struct {
		Name     string          `json:"name"`
		Severity domain.Severity `json:"severity"`
	} `json:"landslideAlert"`
	Tsunami map[string]struct {
		Name     string          `json:"name"`
		Severity domain.Severity `json:"severity"`
	} `json:"tsunami"`
}

var sevMap = func() severityMap {
	var m severityMap
	if err := json.Unmarshal(severityMapJSON, &m); err != nil {
		panic(fmt.Sprintf("severity_map.json: %v", err))
	}
	return m
}()

type report struct {
	Control struct {
		Title    string `xml:"Title"`
		DateTime string `xml:"DateTime"`
		Status   string `xml:"Status"` // 通常 / 訓練 / 試験
	} `xml:"Control"`
	Head struct {
		Title          string `xml:"Title"`
		ReportDateTime string `xml:"ReportDateTime"`
		EventID        string `xml:"EventID"`
		InfoType       string `xml:"InfoType"`
		Serial         string `xml:"Serial"`
		Headline       struct {
			Text string `xml:"Text"`
		} `xml:"Headline"`
	} `xml:"Head"`
	Body struct {
		Warnings []struct {
			Type  string `xml:"type,attr"`
			Items []struct {
				Kinds []struct {
					Name      string   `xml:"Name"`
					Code      string   `xml:"Code"`
					Status    string   `xml:"Status"`
					Attention []string `xml:"Attention>Note"`
				} `xml:"Kind"`
				Area struct {
					Name string `xml:"Name"`
					Code string `xml:"Code"`
				} `xml:"Area"`
			} `xml:"Item"`
		} `xml:"Warning"`
		Tsunami struct {
			Items []struct {
				Area struct {
					Name string `xml:"Name"`
					Code string `xml:"Code"`
				} `xml:"Area"`
				Kind struct {
					Name string `xml:"Name"`
					Code string `xml:"Code"`
				} `xml:"Category>Kind"`
				FirstHeight struct {
					ArrivalTime string `xml:"ArrivalTime"`
					Condition   string `xml:"Condition"`
				} `xml:"FirstHeight"`
				MaxHeight struct {
					Description string `xml:"description,attr"`
				} `xml:"MaxHeight>TsunamiHeight"`
			} `xml:"Forecast>Item"`
		} `xml:"Tsunami"`
		Earthquake struct {
			OriginTime string `xml:"OriginTime"`
			Hypocenter struct {
				Area struct {
					Name string `xml:"Name"`
				} `xml:"Area"`
			} `xml:"Hypocenter"`
			Magnitude struct {
				Description string `xml:"description,attr"`
				Value       string `xml:",chardata"`
			} `xml:"Magnitude"`
		} `xml:"Earthquake"`
		Intensity struct {
			Observation struct {
				MaxInt string `xml:"MaxInt"`
				Prefs  []struct {
					Areas []struct {
						Cities []struct {
							Name   string `xml:"Name"`
							Code   string `xml:"Code"`
							MaxInt string `xml:"MaxInt"`
						} `xml:"City"`
					} `xml:"Area"`
				} `xml:"Pref"`
			} `xml:"Observation"`
		} `xml:"Intensity"`
	} `xml:"Body"`
}

// Parse は電文を解析する。telegramID はフィードの entry/id（電文の URL）。
// 訓練・試験の電文と、対応していない電文は ErrUnsupported を返す。
func Parse(data []byte, telegramID string) (*domain.Event, error) {
	return parse(data, telegramID, false)
}

// parse は電文を解析する。acceptTraining は気象庁のサンプル電文（訓練）をテストで解析するときだけ true にする。
func parse(data []byte, telegramID string, acceptTraining bool) (*domain.Event, error) {
	var r report
	dec := xml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("decode telegram: %w", err)
	}
	accepted := r.Control.Status == "通常" || (acceptTraining && r.Control.Status == "訓練")
	if !accepted {
		return nil, fmt.Errorf("%w: status %q", ErrUnsupported, r.Control.Status)
	}
	issuedAt, err := time.Parse(time.RFC3339, r.Head.ReportDateTime)
	if err != nil {
		return nil, fmt.Errorf("parse ReportDateTime: %w", err)
	}
	ev := &domain.Event{
		ID:        EventID(telegramID),
		Source:    Source,
		Title:     strings.TrimSpace(r.Head.Title),
		Headline:  strings.TrimSpace(r.Head.Headline.Text),
		InfoType:  r.Head.InfoType,
		IssuedAt:  issuedAt,
		ReportURL: telegramID,
	}
	switch r.Control.Title {
	case TitleWarning:
		ev.Kind = domain.EventWarning
		ev.Warnings = parseWarnings(&r)
	case TitleLandslide:
		ev.Kind = domain.EventLandslide
		ev.Warnings = parseLandslide(&r)
	case TitleTsunami:
		ev.Kind = domain.EventTsunami
		ev.Tsunami = parseTsunami(&r)
	case TitleQuake:
		ev.Kind = domain.EventEarthquake
		q, err := parseQuake(&r)
		if err != nil {
			return nil, err
		}
		ev.Quake = q
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupported, r.Control.Title)
	}
	return ev, nil
}

const municipalWarningType = "気象警報・注意報（市町村等）"

func parseWarnings(r *report) []domain.AreaWarning {
	var out []domain.AreaWarning
	for _, w := range r.Body.Warnings {
		if w.Type != municipalWarningType {
			continue
		}
		for _, it := range w.Items {
			aw := domain.AreaWarning{AreaCode: it.Area.Code, AreaName: it.Area.Name}
			for _, k := range it.Kinds {
				wk := domain.WarningKind{
					Code:   k.Code,
					Name:   k.Name,
					Status: k.Status,
					Notes:  k.Attention,
					Active: isActive(k.Code, k.Status),
				}
				if m, ok := sevMap.Warnings[k.Code]; ok {
					wk.Hazard, wk.Severity = m.Hazard, m.Severity
					if m.LandslideSeverity > 0 && hasNote(k.Attention, sevMap.LandslideNote) {
						// 大雨警報（土砂災害）は、大雨と土砂災害の両方の種別として扱う
						ls := wk
						ls.Hazard, ls.Severity = domain.Landslide, m.LandslideSeverity
						aw.Kinds = append(aw.Kinds, ls)
					}
				}
				aw.Kinds = append(aw.Kinds, wk)
			}
			out = append(out, aw)
		}
	}
	return out
}

const landslideWarningType = "土砂災害警戒情報"

// parseLandslide は土砂災害警戒情報を、土砂災害の種別の警報として Warnings に入れる。
func parseLandslide(r *report) []domain.AreaWarning {
	var out []domain.AreaWarning
	for _, w := range r.Body.Warnings {
		if w.Type != landslideWarningType {
			continue
		}
		for _, it := range w.Items {
			aw := domain.AreaWarning{AreaCode: it.Area.Code, AreaName: it.Area.Name}
			for _, k := range it.Kinds {
				m, ok := sevMap.LandslideAlert[k.Code]
				if !ok {
					continue // なし
				}
				aw.Kinds = append(aw.Kinds, domain.WarningKind{
					Code: k.Code, Name: m.Name, Status: k.Status, Hazard: domain.Landslide, Severity: m.Severity,
					Active: k.Status != "解除" && k.Status != "なし",
				})
			}
			out = append(out, aw)
		}
	}
	return out
}

func parseTsunami(r *report) *domain.TsunamiInfo {
	t := &domain.TsunamiInfo{EventID: r.Head.EventID}
	for _, it := range r.Body.Tsunami.Items {
		m, ok := sevMap.Tsunami[it.Kind.Code]
		if !ok {
			continue // 対応表にない種類（新しい体系のコードなど）は扱わない
		}
		arrival := it.FirstHeight.Condition
		if at, err := time.Parse(time.RFC3339, it.FirstHeight.ArrivalTime); err == nil {
			arrival = at.Format("15時04分") + "ころ到達予想"
		}
		t.Areas = append(t.Areas, domain.TsunamiArea{
			Code: it.Area.Code, Name: it.Area.Name, KindCode: it.Kind.Code, KindName: m.Name,
			Severity: m.Severity, ArrivalTime: arrival, MaxHeight: it.MaxHeight.Description,
		})
	}
	return t
}

func isActive(code, status string) bool {
	if code == "00" {
		return false
	}
	switch status {
	case "解除", "発表警報・注意報はなし":
		return false
	}
	return true
}

func hasNote(notes []string, word string) bool {
	for _, n := range notes {
		if strings.Contains(n, word) {
			return true
		}
	}
	return false
}

func parseQuake(r *report) (*domain.Quake, error) {
	q := &domain.Quake{
		EventID:    r.Head.EventID,
		Serial:     r.Head.Serial,
		Hypocenter: r.Body.Earthquake.Hypocenter.Area.Name,
		Magnitude:  r.Body.Earthquake.Magnitude.Description,
		MaxInt:     domain.Intensity(r.Body.Intensity.Observation.MaxInt),
	}
	if q.EventID == "" {
		return nil, errors.New("earthquake telegram without EventID")
	}
	if t := r.Body.Earthquake.OriginTime; t != "" {
		ot, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return nil, fmt.Errorf("parse OriginTime: %w", err)
		}
		q.OriginTime = ot
	}
	for _, p := range r.Body.Intensity.Observation.Prefs {
		for _, a := range p.Areas {
			for _, c := range a.Cities {
				q.Cities = append(q.Cities, domain.CityIntensity{Code: c.Code, Name: c.Name, MaxInt: domain.Intensity(c.MaxInt)})
			}
		}
	}
	return q, nil
}
