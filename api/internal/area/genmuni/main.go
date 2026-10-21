// genmuni は国土地理院の市区町村コード表（https://maps.gsi.go.jp/js/muni.js）から
// internal/area/muni.json を生成する。コード表が更新されたときに手動で実行する。
//
//	curl -sO https://maps.gsi.go.jp/js/muni.js
//	go run ./internal/area/genmuni -in muni.js -out internal/area/muni.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

type muni struct {
	Pref string `json:"pref"`
	Name string `json:"name"`
	// City は政令指定都市の区のとき、その市のコード。気象庁の警報・注意報は市の単位で発表されるため
	City string `json:"city,omitempty"`
}

var line = regexp.MustCompile(`GSI\.MUNI_ARRAY\["(\d+)"\] = '\d+,([^,]+),\d+,([^']+)'`)

func main() {
	in := flag.String("in", "muni.js", "muni.js のパス")
	out := flag.String("out", "internal/area/muni.json", "出力先")
	flag.Parse()

	b, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	munis := map[string]muni{}
	byName := map[string]string{}
	for _, m := range line.FindAllStringSubmatch(string(b), -1) {
		code := fmt.Sprintf("%05s", m[1])
		name := strings.TrimSpace(m[3])
		munis[code] = muni{Pref: m[2], Name: strings.ReplaceAll(name, "　", "")}
		byName[m[2]+"/"+name] = code
	}
	// 「札幌市　中央区」のような区は、市のコードを持たせる
	for code, mu := range munis {
		for full, c := range byName {
			if c != code {
				continue
			}
			parts := strings.SplitN(strings.TrimPrefix(full, mu.Pref+"/"), "　", 2)
			if len(parts) == 2 {
				if city, ok := byName[mu.Pref+"/"+parts[0]]; ok {
					mu.City = city
					munis[code] = mu
				}
			}
		}
	}
	codes := make([]string, 0, len(munis))
	for c := range munis {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	ordered := make([][2]any, 0, len(codes))
	for _, c := range codes {
		ordered = append(ordered, [2]any{c, munis[c]})
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// キーの順序を固定して差分を見やすくする
	fmt.Fprintln(f, "{")
	for i, kv := range ordered {
		v, _ := json.Marshal(kv[1])
		sep := ","
		if i == len(ordered)-1 {
			sep = ""
		}
		fmt.Fprintf(f, "  %q: %s%s\n", kv[0], v, sep)
	}
	fmt.Fprintln(f, "}")
	log.Printf("%d 件を書き出した", len(ordered))
}
