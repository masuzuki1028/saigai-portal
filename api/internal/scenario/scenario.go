// Package scenario は災害シナリオ（testdata/scenarios/<名前>/）を再現する。
// cmd/replay と結合テストで同じ処理を使い、通知の回数と内容をゴールデンファイルで確認する（docs/08 6.2）。
package scenario

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/ingest"
	"github.com/masuzuki1028/saigai-portal/api/internal/matcher"
	"github.com/masuzuki1028/saigai-portal/api/internal/pipeline"
)

// Scenario は scenario.json の内容。
type Scenario struct {
	Description string `json:"description"`
	Locations   []struct {
		User string  `json:"user"`
		Name string  `json:"name"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
	} `json:"locations"`
}

// Load はシナリオと、時刻順に並べた電文のパスを読む。
func Load(dir string) (Scenario, []string, error) {
	var sc Scenario
	b, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	if err != nil {
		return sc, nil, err
	}
	if err := json.Unmarshal(b, &sc); err != nil {
		return sc, nil, err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	sort.Strings(files)
	return sc, files, err
}

// Seed はシナリオの地点を API 経由で登録する。token は利用者名から Authorization ヘッダーの値を作る。
func Seed(ctx context.Context, api http.Handler, sc Scenario, token func(user string) string) error {
	for _, l := range sc.Locations {
		body, _ := json.Marshal(map[string]any{"name": l.Name, "coord": map[string]any{"lat": l.Lat, "lon": l.Lon}, "radiusM": 3000})
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/locations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", token(l.User))
		rec := httptest.NewRecorder()
		api.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			return fmt.Errorf("seed %s/%s: %d %s", l.User, l.Name, rec.Code, rec.Body.String())
		}
	}
	return nil
}

// Run は電文を順に流し、電文ごとのタイムライン・通知の結果を文章にして返す。
func Run(ctx context.Context, p *pipeline.Pipeline, files []string) (string, error) {
	var out strings.Builder
	p.OnResult = func(ev *domain.Event, r matcher.Result) {
		fmt.Fprintf(&out, "## %s %s\n", ev.IssuedAt.Format("01/02 15:04"), ev.Title)
		if len(r.Items) == 0 && len(r.Jobs) == 0 {
			out.WriteString("（変化なし）\n")
		}
		for _, it := range r.Items {
			fmt.Fprintf(&out, "- タイムライン: %s\n", it)
		}
		for _, j := range r.Jobs {
			fmt.Fprintf(&out, "- 通知: %s: %s [%s]\n", j.LocationName, j.Title, j.Status)
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		var st ingest.Stats
		// 電文の ID はファイル名にする（同じシナリオを流し直しても重複して取り込まない）
		if err := p.Ingester.Ingest(ctx, data, "scenario:"+filepath.Base(filepath.Dir(f))+"/"+filepath.Base(f), &st); err != nil {
			return "", fmt.Errorf("%s: %w", f, err)
		}
		if st.Skipped > 0 {
			fmt.Fprintf(&out, "## %s（取り込み済みのためスキップ）\n", filepath.Base(f))
		}
	}
	return out.String(), nil
}
