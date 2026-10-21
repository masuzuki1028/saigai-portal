//go:build integration

package scenario_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/masuzuki1028/saigai-portal/api/internal/area"
	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/crypto"
	"github.com/masuzuki1028/saigai-portal/api/internal/httpapi"
	"github.com/masuzuki1028/saigai-portal/api/internal/pipeline"
	"github.com/masuzuki1028/saigai-portal/api/internal/scenario"
	"github.com/masuzuki1028/saigai-portal/api/internal/store/storetest"
)

var update = flag.Bool("update", false, "ゴールデンファイルを更新する")

type verifier struct{}

func (verifier) Verify(r *http.Request) (auth.Principal, error) {
	sub, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return auth.Principal{}, auth.ErrUnauthenticated
	}
	return auth.Principal{Issuer: "test", Subject: sub}, nil
}

type dir struct{}

func (dir) Email(_ context.Context, p auth.Principal) (string, error) {
	return p.Subject + "@example.com", nil
}

type mailbox struct {
	mu   sync.Mutex
	logs []string
}

func (m *mailbox) Send(_ context.Context, to, subject, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, to+" | "+subject)
	return nil
}

// シナリオを流し、タイムライン・通知の結果とメールの件名をゴールデンファイルと比較する。
// 通知の連打や漏れ（同じレベルの継続で送る、解除で送らない など）を防ぐ。
func TestScenarios(t *testing.T) {
	dirs, _ := filepath.Glob("../../testdata/scenarios/*")
	for _, d := range dirs {
		t.Run(filepath.Base(d), func(t *testing.T) {
			ctx := context.Background()
			st := storetest.New(t)
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			sealer, _ := crypto.NewAESGCM(bytes.Repeat([]byte{1}, 32))
			api := &httpapi.Server{Store: st, Resolver: area.StaticResolver{Points: area.DefaultStaticPoints}, Sealer: sealer, Log: log}
			h, err := api.Handler(verifier{})
			if err != nil {
				t.Fatal(err)
			}
			sc, files, err := scenario.Load(d)
			if err != nil {
				t.Fatal(err)
			}
			if err := scenario.Seed(ctx, h, sc, func(u string) string { return "Bearer " + u }); err != nil {
				t.Fatal(err)
			}
			mb := &mailbox{}
			p := pipeline.New(st, dir{}, mb, "http://localhost:3000", log)
			got, err := scenario.Run(ctx, p, files)
			if err != nil {
				t.Fatal(err)
			}
			got += "\n# メール\n" + strings.Join(mb.logs, "\n") + "\n"
			got += finalState(t, h, sc)

			// 同じ電文をもう一度流しても、何も起きない（冪等）
			again, err := scenario.Run(ctx, p, files)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(again, "通知:") || strings.Contains(again, "タイムライン:") {
				t.Errorf("2回目の実行で処理された:\n%s", again)
			}

			golden := filepath.Join(d, "expected.txt")
			if *update {
				_ = os.WriteFile(golden, []byte(got), 0o644)
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("ゴールデンファイルがない（-update で作成）: %v", err)
			}
			if diff := cmp.Diff(string(want), got); diff != "" {
				t.Errorf("シナリオの結果が変わった (-want +got):\n%s", diff)
			}
		})
	}
}

// finalState は、シナリオの最後の時点の各地点の「現在のレベル」とタイムライン（状態つき）を API から読んで文章にする。
func finalState(t *testing.T, h http.Handler, sc scenario.Scenario) string {
	t.Helper()
	get := func(user, path string, v any) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+user)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatal(err)
		}
	}
	var out strings.Builder
	out.WriteString("\n# 最後の状態\n")
	seen := map[string]bool{}
	for _, l := range sc.Locations {
		if seen[l.User] {
			continue
		}
		seen[l.User] = true
		var locs struct {
			Items []struct {
				ID            string `json:"id"`
				Name          string `json:"name"`
				CurrentLevels []struct {
					Title string `json:"title"`
				} `json:"currentLevels"`
			} `json:"items"`
		}
		get(l.User, "/api/locations", &locs)
		for _, loc := range locs.Items {
			levels := []string{}
			for _, c := range loc.CurrentLevels {
				levels = append(levels, c.Title)
			}
			fmt.Fprintf(&out, "## %s（発表中: %s）\n", loc.Name, strings.Join(levels, "、"))
			var tl struct {
				Items []struct {
					Title  string `json:"title"`
					Status string `json:"status"`
				} `json:"items"`
			}
			get(l.User, "/api/locations/"+loc.ID+"/timeline?limit=100", &tl)
			for _, it := range tl.Items {
				fmt.Fprintf(&out, "- %s [%s]\n", it.Title, it.Status)
			}
		}
	}
	return out.String()
}
