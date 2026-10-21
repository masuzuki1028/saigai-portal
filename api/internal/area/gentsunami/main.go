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
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
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

	recs, err := readDBF(*dbf)
	if err != nil {
		log.Fatal(err)
	}
	shapes, err := readPolylines(*shp)
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

// readPolylines は PolyLine（shapeType=3）のシェープファイルを読む。
func readPolylines(path string) ([][][][2]float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if t := binary.LittleEndian.Uint32(b[32:36]); t != 3 {
		return nil, fmt.Errorf("PolyLine ではない: shapeType=%d", t)
	}
	var out [][][][2]float64
	for p := 100; p+8 <= len(b); {
		length := int(binary.BigEndian.Uint32(b[p+4:p+8])) * 2
		rec := b[p+8 : p+8+length]
		p += 8 + length
		if binary.LittleEndian.Uint32(rec[0:4]) == 0 { // Null shape
			out = append(out, nil)
			continue
		}
		numParts := int(binary.LittleEndian.Uint32(rec[36:40]))
		numPoints := int(binary.LittleEndian.Uint32(rec[40:44]))
		starts := make([]int, numParts+1)
		for i := 0; i < numParts; i++ {
			starts[i] = int(binary.LittleEndian.Uint32(rec[44+4*i:]))
		}
		starts[numParts] = numPoints
		pts := 44 + 4*numParts
		var parts [][][2]float64
		for i := 0; i < numParts; i++ {
			var line [][2]float64
			for j := starts[i]; j < starts[i+1]; j++ {
				x := math.Float64frombits(binary.LittleEndian.Uint64(rec[pts+16*j:]))
				y := math.Float64frombits(binary.LittleEndian.Uint64(rec[pts+16*j+8:]))
				line = append(line, [2]float64{x, y})
			}
			parts = append(parts, line)
		}
		out = append(out, parts)
	}
	return out, nil
}

// readDBF は dBase ファイルの文字列フィールドを読む（文字コードは UTF-8）。
func readDBF(path string) ([]map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint32(b[4:8]))
	headerLen := int(binary.LittleEndian.Uint16(b[8:10]))
	recLen := int(binary.LittleEndian.Uint16(b[10:12]))
	type field struct {
		name string
		len  int
	}
	var fields []field
	for p := 32; b[p] != 0x0d; p += 32 {
		fields = append(fields, field{strings.TrimRight(string(b[p:p+11]), "\x00"), int(b[p+16])})
	}
	var out []map[string]string
	for i := 0; i < n; i++ {
		rec := b[headerLen+i*recLen : headerLen+(i+1)*recLen]
		m := map[string]string{}
		o := 1
		for _, f := range fields {
			m[f.name] = strings.TrimSpace(strings.TrimRight(string(rec[o:o+f.len]), "\x00"))
			o += f.len
		}
		out = append(out, m)
	}
	return out, nil
}
