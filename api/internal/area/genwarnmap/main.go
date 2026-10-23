// genwarnmap は気象庁の「市町村等（気象警報等）」の GIS データから、次の2つを生成する。データが更新されたときに手動で実行する。
//
//   - web/public/geo/warn/<都道府県コード>.json: 画面の地図に警報の区域を塗るための区域の形（GeoJSON）。
//     元データは約 230MB あるため、約 200m の誤差まで間引き、座標を小数4桁に丸め、都道府県ごとに分ける
//
//   - internal/area/warnkeys.json: 警報・注意報の発表区域（5桁の市区町村コード）の一覧。
//     政令指定都市を市の単位で発表するか区の単位で発表するか（例: 2026年3月から神戸市は区の単位）を判定するのに使う
//
//     curl -sO https://www.data.jma.go.jp/developer/gis/20260226_AreaInformationCity_weather_GIS.zip
//     unzip して 市町村等（気象警報等）.shp / .dbf を取り出す
//     go run ./internal/area/genwarnmap -shp ... -dbf ... -geo ../web/public/geo/warn -keys internal/area/warnkeys.json
//
// 出典: 気象庁 防災情報XMLで用いるコードが示す地域の GIS データ（市町村等（気象警報等））
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/masuzuki1028/saigai-portal/api/internal/area/shpfile"
)

type feature struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
	Geometry   struct {
		Type        string              `json:"type"`
		Coordinates [][][]shpfile.Point `json:"coordinates"`
	} `json:"geometry"`
}

func main() {
	shp := flag.String("shp", "", "市町村等（気象警報等）.shp")
	dbf := flag.String("dbf", "", "市町村等（気象警報等）.dbf")
	geoDir := flag.String("geo", "../web/public/geo/warn", "GeoJSON の出力先")
	keysOut := flag.String("keys", "internal/area/warnkeys.json", "発表区域の一覧の出力先")
	tol := flag.Float64("tol", 0.002, "間引きの許容誤差（度）")
	flag.Parse()

	recs, err := shpfile.ReadDBF(*dbf)
	if err != nil {
		log.Fatal(err)
	}
	shapes, err := shpfile.ReadShapes(*shp, 5)
	if err != nil {
		log.Fatal(err)
	}
	if len(recs) != len(shapes) {
		log.Fatalf("dbf %d 件と shp %d 件が一致しない", len(recs), len(shapes))
	}
	byPref := map[string][]feature{}
	keys := map[string]bool{}
	for i, rings := range shapes {
		code := recs[i]["regioncode"]
		if len(code) != 7 {
			continue // 削除済みなどのレコード
		}
		keys[code[:5]] = true
		f := feature{Type: "Feature", Properties: map[string]string{"code": code, "name": recs[i]["regionname"]}}
		f.Geometry.Type = "MultiPolygon"
		for _, ring := range rings {
			s := shpfile.Simplify(ring, *tol)
			if len(s) < 4 {
				continue // 間引いて点になった小さな島などは描かない
			}
			for j := range s {
				s[j] = shpfile.Point{round4(s[j][0]), round4(s[j][1])}
			}
			// シェープファイルでは外側の輪が時計回り、穴が反時計回り。穴は直前の外側の輪に付ける
			if shpfile.Clockwise(ring) || len(f.Geometry.Coordinates) == 0 {
				f.Geometry.Coordinates = append(f.Geometry.Coordinates, [][]shpfile.Point{s})
			} else {
				last := len(f.Geometry.Coordinates) - 1
				f.Geometry.Coordinates[last] = append(f.Geometry.Coordinates[last], s)
			}
		}
		if len(f.Geometry.Coordinates) > 0 {
			byPref[code[:2]] = append(byPref[code[:2]], f)
		}
	}
	if err := os.MkdirAll(*geoDir, 0o755); err != nil {
		log.Fatal(err)
	}
	total := 0
	for pref, fs := range byPref {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Properties["code"] < fs[j].Properties["code"] })
		b, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": fs})
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*geoDir, pref+".json"), b, 0o644); err != nil {
			log.Fatal(err)
		}
		total += len(b)
	}
	ks := make([]string, 0, len(keys))
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	b, _ := json.Marshal(map[string]any{
		"source": "気象庁 防災情報XMLで用いるコードが示す地域の GIS データ（市町村等（気象警報等）, 20260226）から作成",
		"keys":   ks,
	})
	if err := os.WriteFile(*keysOut, b, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d 都道府県・合計 %s・発表区域 %d", len(byPref), human(total), len(ks))
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func human(n int) string { return fmt.Sprintf("%.1fMB", float64(n)/1e6) }
