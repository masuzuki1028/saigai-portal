// Package shpfile は、気象庁の GIS データ（シェープファイル）を読む最小限の実装。データの生成ツール専用。
package shpfile

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strings"
)

// Point は [経度, 緯度]。
type Point = [2]float64

// ReadShapes は PolyLine（3）または Polygon（5）のシェープファイルを読み、図形ごとのパート（線・輪）を返す。
func ReadShapes(path string, wantType uint32) ([][][]Point, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if t := binary.LittleEndian.Uint32(b[32:36]); t != wantType {
		return nil, fmt.Errorf("shapeType=%d（%d を期待）", t, wantType)
	}
	var out [][][]Point
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
		var parts [][]Point
		for i := 0; i < numParts; i++ {
			var line []Point
			for j := starts[i]; j < starts[i+1]; j++ {
				x := math.Float64frombits(binary.LittleEndian.Uint64(rec[pts+16*j:]))
				y := math.Float64frombits(binary.LittleEndian.Uint64(rec[pts+16*j+8:]))
				line = append(line, Point{x, y})
			}
			parts = append(parts, line)
		}
		out = append(out, parts)
	}
	return out, nil
}

// ReadDBF は dBase ファイルの文字列フィールドを読む（文字コードは UTF-8）。
func ReadDBF(path string) ([]map[string]string, error) {
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

// Simplify は Douglas-Peucker 法で線を間引く（tol は度）。
func Simplify(pts []Point, tol float64) []Point {
	if len(pts) < 3 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	type span struct{ a, b int }
	stack := []span{{0, len(pts) - 1}}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		maxD, idx := 0.0, -1
		for i := s.a + 1; i < s.b; i++ {
			if d := segDist(pts[i], pts[s.a], pts[s.b]); d > maxD {
				maxD, idx = d, i
			}
		}
		if idx >= 0 && maxD > tol {
			keep[idx] = true
			stack = append(stack, span{s.a, idx}, span{idx, s.b})
		}
	}
	var out []Point
	for i, k := range keep {
		if k {
			out = append(out, pts[i])
		}
	}
	return out
}

func segDist(p, a, b Point) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p[0]-a[0]-t*dx, p[1]-a[1]-t*dy)
}

// Clockwise は輪が時計回りか（シェープファイルでは外側の輪が時計回り、穴が反時計回り）。
func Clockwise(ring []Point) bool {
	sum := 0.0
	for i := 0; i+1 < len(ring); i++ {
		sum += (ring[i+1][0] - ring[i][0]) * (ring[i+1][1] + ring[i][1])
	}
	return sum > 0
}
