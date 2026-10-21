// Package area は地点の座標から市区町村を求め、気象庁の情報と突き合わせるためのキーを作る
// （docs/04_data_model.md 7章）。
//
// 市区町村コード表（muni.json）は国土地理院の市区町村コード表から生成している（genmuni）。
package area

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

// Area は地点が属する市区町村と、突き合わせに使うキー。
type Area struct {
	MuniCode string // 5桁の市区町村コード（政令指定都市は区のコード）
	Name     string // 例: 横浜市中区
	Pref     string
	// WarnKey は警報・注意報の突き合わせキー。政令指定都市は市の単位で発表されるため市のコードにする
	WarnKey string
	// QuakeKey は地震情報（市町村ごとの震度）の突き合わせキー。政令指定都市は区の単位
	QuakeKey string
	// IsCoastal は沿岸の地点か（津波の設定を出すかどうか）。MVP では判定しない（docs/01 10章の未決事項）
	IsCoastal bool
}

// Resolver は座標から市区町村を求める。
type Resolver interface {
	Resolve(ctx context.Context, lat, lon float64) (Area, error)
}

// ErrNotFound は座標に対応する市区町村がない（海上など）。
var ErrNotFound = errors.New("municipality not found")

//go:embed muni.json
var muniJSON []byte

type muni struct {
	Pref string `json:"pref"`
	Name string `json:"name"`
	City string `json:"city"`
}

var munis = func() map[string]muni {
	m := map[string]muni{}
	if err := json.Unmarshal(muniJSON, &m); err != nil {
		panic(fmt.Sprintf("muni.json: %v", err))
	}
	return m
}()

// FromMuniCode は5桁の市区町村コードから Area を作る。
func FromMuniCode(code string) (Area, error) {
	if len(code) < 5 {
		code = fmt.Sprintf("%05s", code)
	}
	m, ok := munis[code]
	if !ok {
		return Area{}, fmt.Errorf("%w: %s", ErrNotFound, code)
	}
	warn := code
	if m.City != "" {
		warn = m.City
	}
	return Area{MuniCode: code, Name: m.Name, Pref: m.Pref, WarnKey: warn, QuakeKey: code}, nil
}

// KeyFromJMACode は気象庁の7桁の市町村等コード（例: 1520200、富山市平地 1620111）を
// 突き合わせキー（5桁）にする。二次細分（市町村をさらに分けた区域）は市町村の単位にまとめる。
//
// 富山市の「平地」と「山間部」のように市町村の中で分かれている場合、どちらの区域の情報も
// その市町村の全地点に届く（実際より広めに届く方向の誤差）。精度の改善は Phase 2 で行う。
func KeyFromJMACode(code string) string {
	if len(code) < 5 {
		return code
	}
	return code[:5]
}
