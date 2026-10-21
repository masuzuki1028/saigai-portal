package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: redact}))
	l.Info("x", "email", "alice@example.com", "lat", 35.68, "lon", 139.76, "address", "東京都千代田区", "token", "secret", "locationId", "01J")
	out := buf.String()
	for _, leak := range []string{"alice@example.com", "35.68", "139.76", "東京都", "secret"} {
		if strings.Contains(out, leak) {
			t.Errorf("ログに %q が出ている: %s", leak, out)
		}
	}
	if !strings.Contains(out, "01J") {
		t.Errorf("伏せる必要のない値まで消えている: %s", out)
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{"alice@example.com": "a***@example.com", "x": "***", "@d": "***"} {
		if got := MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
