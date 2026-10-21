package crypto

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestAESGCM(t *testing.T) {
	ctx := context.Background()
	s, err := NewAESGCM(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := s.Seal(ctx, []byte("35.681200,139.767100"), []byte("LOC#A"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("35.6812")) {
		t.Fatal("暗号文に平文が含まれている")
	}
	pt, err := s.Open(ctx, ct, []byte("LOC#A"))
	if err != nil || string(pt) != "35.681200,139.767100" {
		t.Fatalf("Open = %q, %v", pt, err)
	}
	// 別の地点の暗号文としては復号できない
	if _, err := s.Open(ctx, ct, []byte("LOC#B")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("aad が違うのに復号できた: %v", err)
	}
	if _, err := NewAESGCM([]byte("short")); err == nil {
		t.Error("短い鍵を受け付けた")
	}
}
