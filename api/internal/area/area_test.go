package area

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFromMuniCode(t *testing.T) {
	tests := []struct {
		code              string
		name, warn, quake string
	}{
		{"15202", "長岡市", "15202", "15202"},
		{"14104", "横浜市中区", "14100", "14104"}, // 政令指定都市の区: 警報は市の単位
		{"13101", "千代田区", "13101", "13101"},  // 東京23区は区の単位
		{"1206", "釧路市", "01206", "01206"},    // 先頭の0が落ちたコード
	}
	for _, tt := range tests {
		a, err := FromMuniCode(tt.code)
		if err != nil {
			t.Fatalf("FromMuniCode(%q): %v", tt.code, err)
		}
		if a.Name != tt.name || a.WarnKey != tt.warn || a.QuakeKey != tt.quake {
			t.Errorf("FromMuniCode(%q) = %+v", tt.code, a)
		}
	}
	if _, err := FromMuniCode("99999"); !errors.Is(err, ErrNotFound) {
		t.Errorf("存在しないコード: err = %v", err)
	}
}

func TestKeyFromJMACode(t *testing.T) {
	for in, want := range map[string]string{"1520200": "15202", "1620111": "16201", "1410000": "14100", "0120600": "01206"} {
		if got := KeyFromJMACode(in); got != want {
			t.Errorf("KeyFromJMACode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGSIResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("lat") {
		case "37.446200":
			_, _ = w.Write([]byte(`{"results":{"muniCd":"15202","lv01Nm":"城内町一丁目"}}`))
		case "30.000000":
			_, _ = w.Write([]byte(`{}`)) // 海上
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	g := &GSIResolver{BaseURL: srv.URL, Client: srv.Client()}

	a, err := g.Resolve(context.Background(), 37.4462, 138.8512)
	if err != nil || a.Name != "長岡市" {
		t.Fatalf("Resolve = %+v, %v", a, err)
	}
	if _, err := g.Resolve(context.Background(), 30, 140); !errors.Is(err, ErrNotFound) {
		t.Errorf("海上: err = %v", err)
	}
	if _, err := g.Resolve(context.Background(), 1, 1); err == nil {
		t.Error("5xx でエラーにならない")
	}
}

func TestStaticResolver(t *testing.T) {
	s := StaticResolver{Points: DefaultStaticPoints}
	a, err := s.Resolve(context.Background(), 37.45, 138.85)
	if err != nil || a.MuniCode != "15202" {
		t.Errorf("Resolve = %+v, %v", a, err)
	}
	if _, err := s.Resolve(context.Background(), 26.2, 127.7); !errors.Is(err, ErrNotFound) {
		t.Errorf("遠い地点: err = %v", err)
	}
}

func TestNearestTsunamiZone(t *testing.T) {
	tests := []struct {
		name     string
		lat, lon float64
		coastal  bool
		zone     string
	}{
		{"新潟市中央区（海沿い）", 37.9161, 139.0364, true, "新潟県上中下越"},
		{"長岡市の中心部（内陸）", 37.4462, 138.8512, false, ""},
		{"鎌倉駅（海沿い）", 35.3192, 139.5467, true, "相模湾・三浦半島"},
		{"宮古市（海沿い）", 39.6414, 141.9570, true, "岩手県"},
		{"東京都千代田区（東京湾から約 3km）", 35.6812, 139.7671, true, "東京湾内湾"},
		{"長野市（内陸）", 36.6485, 138.1949, false, ""},
	}
	for _, tt := range tests {
		z, d, ok := NearestTsunamiZone(tt.lat, tt.lon)
		if ok != tt.coastal || z.Name != tt.zone {
			t.Errorf("%s: zone=%+v dist=%.1fkm coastal=%v", tt.name, z, d, ok)
		}
	}
}
