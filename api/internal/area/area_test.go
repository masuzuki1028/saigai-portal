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
			w.Write([]byte(`{"results":{"muniCd":"15202","lv01Nm":"城内町一丁目"}}`))
		case "30.000000":
			w.Write([]byte(`{}`)) // 海上
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
