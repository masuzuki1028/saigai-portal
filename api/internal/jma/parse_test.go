package jma

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
)

var update = flag.Bool("update", false, "ゴールデンファイルを更新する")

// 実際の電文（testdata/*.xml）を解析し、結果をゴールデンファイルと比較する。
func TestParseGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/*_V*.xml")
	if err != nil || len(files) == 0 {
		t.Fatalf("fixture がない: %v", err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".xml")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			// sample_* は気象庁のサンプル電文（Status が訓練のものを含む）
			ev, err := parse(data, "https://www.data.jma.go.jp/developer/xml/data/"+name+".xml", strings.HasPrefix(name, "sample_"))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := json.MarshalIndent(ev, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, append(got, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("ゴールデンファイルがない（go test ./internal/jma -update で作成）: %v", err)
			}
			if diff := cmp.Diff(strings.TrimSpace(string(want)), strings.TrimSpace(string(got))); diff != "" {
				t.Errorf("解析結果がゴールデンファイルと異なる (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseWarningMapping(t *testing.T) {
	data, err := os.ReadFile("testdata/20261020071933_0_VPWW54_150000.xml")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := Parse(data, "x")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Kind != domain.EventWarning {
		t.Fatalf("Kind = %q", ev.Kind)
	}
	var nagaoka *domain.AreaWarning
	for i := range ev.Warnings {
		if ev.Warnings[i].AreaCode == "1520200" {
			nagaoka = &ev.Warnings[i]
		}
	}
	if nagaoka == nil {
		t.Fatal("長岡市の区域がない")
	}
	var rain, thunder *domain.WarningKind
	for i, k := range nagaoka.Kinds {
		switch k.Name {
		case "大雨注意報":
			rain = &nagaoka.Kinds[i]
		case "雷注意報":
			thunder = &nagaoka.Kinds[i]
		}
	}
	if rain == nil || rain.Hazard != domain.Rain || rain.Severity != domain.SeverityAdvisory || !rain.Active {
		t.Errorf("大雨注意報の変換が正しくない: %+v", rain)
	}
	if thunder == nil || thunder.Hazard != "" || thunder.Active {
		t.Errorf("雷注意報（解除・対象外）の変換が正しくない: %+v", thunder)
	}
}

func TestParseLandslide(t *testing.T) {
	// 大雨警報（土砂災害）は大雨と土砂災害の両方になる
	xml := strings.Replace(minimalWarning, "{{KIND}}", `<Kind><Name>大雨警報</Name><Code>03</Code><Status>発表</Status><Attention><Note>土砂災害警戒</Note><Note>浸水害警戒</Note></Attention></Kind>`, 1)
	ev, err := Parse([]byte(xml), "x")
	if err != nil {
		t.Fatal(err)
	}
	got := map[domain.Hazard]domain.Severity{}
	for _, k := range ev.Warnings[0].Kinds {
		got[k.Hazard] = k.Severity
	}
	want := map[domain.Hazard]domain.Severity{domain.Rain: 3, domain.Landslide: 3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestParseRejectsTraining(t *testing.T) {
	xml := strings.Replace(strings.Replace(minimalWarning, "{{KIND}}", "", 1), "<Status>通常</Status>", "<Status>訓練</Status>", 1)
	if _, err := Parse([]byte(xml), "x"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("訓練の電文を受け付けた: %v", err)
	}
}

func TestParseFeed(t *testing.T) {
	f, err := os.Open("testdata/feed_extra.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	entries, err := ParseFeed(f)
	if err != nil {
		t.Fatal(err)
	}
	var wanted int
	for _, e := range entries {
		if Wanted(e.Title) {
			wanted++
		}
	}
	if len(entries) != 3 || wanted != 2 {
		t.Errorf("entries = %d, wanted = %d", len(entries), wanted)
	}
	if entries[0].URL == "" || entries[0].Updated.IsZero() {
		t.Errorf("entry の値が空: %+v", entries[0])
	}
}

func FuzzParse(f *testing.F) {
	files, _ := filepath.Glob("testdata/*.xml")
	for _, p := range files {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(b)
		}
	}
	f.Add([]byte(minimalWarning))
	f.Fuzz(func(t *testing.T, data []byte) {
		// 不正な入力でも panic しないこと
		_, _ = Parse(data, "fuzz")
	})
}

const minimalWarning = `<?xml version="1.0" encoding="UTF-8"?>
<Report xmlns="http://xml.kishou.go.jp/jmaxml1/">
<Control><Title>気象警報・注意報（Ｈ２７）</Title><Status>通常</Status></Control>
<Head xmlns="http://xml.kishou.go.jp/jmaxml1/informationBasis1/"><Title>テスト県気象警報・注意報</Title><ReportDateTime>2026-10-20T10:00:00+09:00</ReportDateTime><InfoType>発表</InfoType></Head>
<Body xmlns="http://xml.kishou.go.jp/jmaxml1/body/meteorology1/">
<Warning type="気象警報・注意報（市町村等）"><Item>{{KIND}}<Area><Name>テスト市</Name><Code>1520200</Code></Area></Item></Warning>
</Body></Report>`
