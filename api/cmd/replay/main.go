//go:build local

// replay は保存した電文を順に流し、取り込み → 突き合わせ → 通知 を手元で再現する（docs/08 6.2）。
// 通知のメールは Mailpit（http://localhost:8025）で確認できる。
//
//	go run -tags local ./cmd/replay niigata-heavy-rain
//	go run -tags local ./cmd/replay -list
package main

import (
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
	"slices"
	"strings"

	"github.com/masuzuki1028/saigai-portal/api/internal/localenv"
	"github.com/masuzuki1028/saigai-portal/api/internal/scenario"
)

func main() {
	list := flag.Bool("list", false, "シナリオの一覧を表示する")
	reset := flag.Bool("reset", false, "ローカルのテーブルを作り直してから流す（登録済みの地点も消える）")
	dir := flag.String("dir", "testdata/scenarios", "シナリオのディレクトリ")
	verbose := flag.Bool("v", false, "処理のログを表示する")
	flag.Parse()

	if *list {
		dirs, _ := filepath.Glob(filepath.Join(*dir, "*"))
		for _, d := range dirs {
			sc, _, err := scenario.Load(d)
			if err == nil {
				fmt.Printf("%-24s %s\n", filepath.Base(d), sc.Description)
			}
		}
		return
	}
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "使い方: go run -tags local ./cmd/replay [-reset] <シナリオ名>（一覧は -list）")
		os.Exit(2)
	}
	ctx := context.Background()
	out := io.Discard
	if *verbose {
		out = os.Stderr
	}
	log := slog.New(slog.NewTextHandler(out, nil))

	env := localenv.FromEnv()
	env.Geocoder = "static" // シナリオの地点は固定なので、外部の逆ジオコーダは呼ばない
	app, err := localenv.New(ctx, env, log)
	if err != nil {
		fail(err)
	}
	if *reset {
		if err := app.Store.DeleteTables(ctx); err != nil {
			fail(err)
		}
		if app, err = localenv.New(ctx, env, log); err != nil {
			fail(err)
		}
	}

	sc, files, err := scenario.Load(filepath.Join(*dir, flag.Arg(0)))
	if err != nil {
		fail(err)
	}
	fmt.Printf("# %s\n%s\n\n", flag.Arg(0), sc.Description)
	if err := seedOnce(ctx, app, sc); err != nil {
		fail(err)
	}
	res, err := scenario.Run(ctx, app.Pipeline, files)
	if err != nil {
		fail(err)
	}
	fmt.Print(res)
	users := []string{}
	for _, l := range sc.Locations {
		if !slices.Contains(users, l.User) {
			users = append(users, l.User)
		}
	}
	fmt.Printf("\n通知のメール: http://localhost:8025\n画面: %s/app （%s でログイン）\n", env.AppBaseURL, strings.Join(users, " / "))
}

// seedOnce は、まだ同じ名前の地点がない利用者にだけシナリオの地点を登録する（繰り返し実行しても増えない）。
func seedOnce(ctx context.Context, app *localenv.App, sc scenario.Scenario) error {
	var todo scenario.Scenario
	for _, l := range sc.Locations {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/locations", nil)
		req.Header.Set("Authorization", localenv.TokenFor(l.User))
		rec := httptest.NewRecorder()
		app.API.ServeHTTP(rec, req)
		var res struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		exists := false
		for _, it := range res.Items {
			exists = exists || it.Name == l.Name
		}
		if !exists {
			todo.Locations = append(todo.Locations, l)
		}
	}
	return scenario.Seed(ctx, app.API, todo, localenv.TokenFor)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "エラー:", err)
	os.Exit(1)
}
