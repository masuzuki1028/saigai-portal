// gentsunami は気象庁の津波予報区の GIS データ（シェープファイル、海岸線に沿った線）から
// internal/area/tsunami.json を生成する。データが更新されたときに手動で実行する。
//
//	curl -sO https://www.data.jma.go.jp/developer/gis/20240520_AreaTsunami_GIS.zip
//	unzip して 津波予報区.shp / .dbf を取り出す
//	go run ./internal/area/gentsunami -shp 津波予報区.shp -dbf 津波予報区.dbf -out internal/area/tsunami.json
//
// 元データは約 140MB あり、短い線が大量に集まった形になっている。沿岸かどうかの判定（海岸線から数 km 以内か）には
// 細かい形は不要なため、海岸線を約 2km 四方の格子（0.02 度）のセルに置き換えて持つ。
// 出典: 気象庁 防災情報XMLで用いるコードが示す地域の GIS データ（津波予報区）
package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"os"
	"sort"

	"github.com/masuzuki1028/saigai-portal/api/internal/area/shpfile"
)

type zone struct {
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Cells [][2]int `json:"cells"` // [緯度/Grid, 経度/Grid] の整数（四捨五入）
}

// Grid はセルの大きさ（度）。internal/area/tsunami.go と合わせる。
const Grid = 0.02

func main() {
	shp := flag.String("shp", "", "津波予報区.shp")
	dbf := flag.String("dbf", "", "津波予報区.dbf")
	out := flag.String("out", "internal/area/tsunami.json", "出力先")
	flag.Parse()

	recs, err := shpfile.ReadDBF(*dbf)
	if err != nil {
		log.Fatal(err)
	}
	shapes, err := shpfile.ReadShapes(*shp, 3)
	if err != nil {
		log.Fatal(err)
	}
	if len(recs) != len(shapes) {
		log.Fatalf("dbf %d 件と shp %d 件が一致しない", len(recs), len(shapes))
	}
	byCode := map[string]*zone{}
	var order []string
	total := 0
	for i, parts := range shapes {
		code := recs[i]["code"]
		z, ok := byCode[code]
		if !ok {
			z = &zone{Code: code, Name: recs[i]["name"]}
			byCode[code] = z
			order = append(order, code)
		}
		seen := map[[2]int]bool{}
		for _, c := range z.Cells {
			seen[c] = true
		}
		for _, line := range parts {
			for j := 0; j+1 < len(line); j++ {
				a, b := line[j], line[j+1]
				// セルを飛ばさないよう、セルの半分の間隔で補間する
				n := int(math.Ceil(math.Hypot(b[0]-a[0], b[1]-a[1])/(Grid/2))) + 1
				for k := 0; k <= n; k++ {
					t := float64(k) / float64(n)
					c := [2]int{int(math.Round((a[1] + t*(b[1]-a[1])) / Grid)), int(math.Round((a[0] + t*(b[0]-a[0])) / Grid))}
					if !seen[c] {
						seen[c] = true
						z.Cells = append(z.Cells, c)
					}
				}
			}
		}
	}
	zones := make([]zone, 0, len(order))
	for _, c := range order {
		sort.Slice(byCode[c].Cells, func(i, j int) bool {
			a, b := byCode[c].Cells[i], byCode[c].Cells[j]
			return a[0] < b[0] || (a[0] == b[0] && a[1] < b[1])
		})
		total += len(byCode[c].Cells)
		zones = append(zones, *byCode[c])
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].Code < zones[j].Code })
	b, err := json.Marshal(map[string]any{
		"source": "気象庁 防災情報XMLで用いるコードが示す地域の GIS データ（津波予報区, 20240520）を加工して作成",
		"grid":   Grid,
		"zones":  zones,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%d 区域・%d セル・%d バイト", len(zones), total, len(b))
}
