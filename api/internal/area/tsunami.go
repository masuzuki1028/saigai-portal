package area

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

// CoastalDistanceKM は「沿岸の地点」とみなす海岸線からの距離。津波の設定はこの範囲の地点にだけ出す
// （docs/01_concept_overview.md 4.2）。海岸線は約 2km 四方のセルで持つため、±1.5km 程度の誤差がある。
const CoastalDistanceKM = 5.0

// TsunamiZone は津波予報区。
type TsunamiZone struct {
	Code string // 例: 340（新潟県上中下越）
	Name string
}

//go:embed tsunami.json
var tsunamiJSON []byte

type tsunamiData struct {
	Grid  float64 `json:"grid"`
	Zones []struct {
		Code  string   `json:"code"`
		Name  string   `json:"name"`
		Cells [][2]int `json:"cells"`
	} `json:"zones"`
}

type cellZone struct{ code, name string }

var tsunamiGrid, tsunamiCells = func() (float64, map[[2]int][]cellZone) {
	var d tsunamiData
	if err := json.Unmarshal(tsunamiJSON, &d); err != nil {
		panic(fmt.Sprintf("tsunami.json: %v", err))
	}
	cells := map[[2]int][]cellZone{}
	for _, z := range d.Zones {
		for _, c := range z.Cells {
			cells[c] = append(cells[c], cellZone{z.Code, z.Name})
		}
	}
	return d.Grid, cells
}()

// NearestTsunamiZone は地点から CoastalDistanceKM 以内にある一番近い津波予報区を返す。なければ ok=false（内陸の地点）。
func NearestTsunamiZone(lat, lon float64) (z TsunamiZone, distKM float64, ok bool) {
	ci, cj := int(math.Round(lat/tsunamiGrid)), int(math.Round(lon/tsunamiGrid))
	// 経度方向のセルは緯度によって短くなるため、探す範囲を広げる
	ri := int(math.Ceil(CoastalDistanceKM/(tsunamiGrid*111.0))) + 1
	rj := int(math.Ceil(CoastalDistanceKM/(tsunamiGrid*111.0*math.Cos(lat*math.Pi/180)))) + 1
	best := math.Inf(1)
	for i := ci - ri; i <= ci+ri; i++ {
		for j := cj - rj; j <= cj+rj; j++ {
			zs, found := tsunamiCells[[2]int{i, j}]
			if !found {
				continue
			}
			d := haversineKM(lat, lon, float64(i)*tsunamiGrid, float64(j)*tsunamiGrid)
			if d < best {
				best = d
				z = TsunamiZone{Code: zs[0].code, Name: zs[0].name}
			}
		}
	}
	if best > CoastalDistanceKM {
		return TsunamiZone{}, 0, false
	}
	return z, best, true
}

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}
