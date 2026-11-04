// Package logx はログの共通処理。住所・座標・メールアドレス・トークンをログに出さないためのマスキングを持つ
// （docs/07_security.md 3.5）。
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// New は JSON 形式の構造化ログを返す。出力するレベルは LOG_LEVEL（DEBUG / INFO / WARN / ERROR。初期値は INFO）。
func New() *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))) // 不正な値・未設定なら INFO のまま
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level, ReplaceAttr: redact}))
}

// sensitiveKeys に一致する属性は値を伏せる。
var sensitiveKeys = map[string]bool{
	"email": true, "address": true, "lat": true, "lon": true, "coord": true,
	"token": true, "authorization": true, "webhook": true,
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, "[REDACTED]")
	}
	return a
}

// MaskEmail はメールアドレスを「a***@example.com」の形にする。
func MaskEmail(s string) string {
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" {
		return "***"
	}
	return local[:1] + "***@" + domain
}
