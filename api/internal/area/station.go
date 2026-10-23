package area

import (
	"sort"
	"strings"
)

// stationIndex は都道府県ごとの「市区町村名の候補 → 市区町村コード」（長い名前から順に並べる）。
var stationIndex = func() map[string][]nameCode {
	idx := map[string][]nameCode{}
	for code, m := range munis {
		names := []string{m.Name}
		// 政令指定都市の区は、震度観測点の名前では「市」が省かれる（例: 横浜市中区 → 横浜中区）
		if m.City != "" {
			if city, ok := munis[m.City]; ok && strings.HasPrefix(m.Name, city.Name) {
				names = append(names, strings.TrimSuffix(city.Name, "市")+strings.TrimPrefix(m.Name, city.Name))
			}
		}
		for _, n := range names {
			idx[m.Pref] = append(idx[m.Pref], nameCode{n, code})
		}
	}
	for p := range idx {
		sort.Slice(idx[p], func(i, j int) bool {
			if len(idx[p][i].name) != len(idx[p][j].name) {
				return len(idx[p][i].name) > len(idx[p][j].name)
			}
			return idx[p][i].code < idx[p][j].code
		})
	}
	return idx
}()

type nameCode struct{ name, code string }

// MuniFromStation は震度観測点の名前（例: 長岡市城内町、茨城古河市下大野、横浜中区山手町）から
// 5桁の市区町村コードを求める。P2P地震情報は観測点を名前で返すため、市区町村の名前の前方一致で引く。
func MuniFromStation(pref, station string) (string, bool) {
	s := strings.TrimRight(strings.TrimSpace(station), "＊*")
	if code, ok := matchPrefix(pref, s); ok {
		return code, true
	}
	// 同じ名前の市区町村がある場合、観測点の名前の先頭に都道府県名（「県」などを除く）が付く（例: 茨城古河市）。
	// 「新潟中央区」のように市の名前が都道府県名と同じ場合があるため、取り除くのは一致しなかったときだけ
	if short := strings.TrimRight(pref, "都府県"); pref != "北海道" && strings.HasPrefix(s, short) {
		return matchPrefix(pref, strings.TrimPrefix(s, short))
	}
	return "", false
}

func matchPrefix(pref, s string) (string, bool) {
	for _, nc := range stationIndex[pref] {
		if strings.HasPrefix(s, nc.name) {
			return nc.code, true
		}
	}
	return "", false
}
