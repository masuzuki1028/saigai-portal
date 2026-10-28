package crypto

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// fakeKMS はデータキーを作り、「暗号化したデータキー」から元のデータキーを返す。
type fakeKMS struct {
	keys              map[string][]byte
	generate, decrypt int
}

func (f *fakeKMS) GenerateDataKey(_ context.Context, in *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	if !maps.Equal(in.EncryptionContext, encryptionContext) || in.KeySpec != types.DataKeySpecAes256 {
		return nil, errors.New("unexpected input")
	}
	f.generate++
	pt, blob := make([]byte, 32), make([]byte, 40)
	_, _ = rand.Read(pt)
	_, _ = rand.Read(blob)
	f.keys[string(blob)] = append([]byte(nil), pt...)
	return &kms.GenerateDataKeyOutput{Plaintext: pt, CiphertextBlob: blob}, nil
}

func (f *fakeKMS) Decrypt(_ context.Context, in *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	f.decrypt++
	k, ok := f.keys[string(in.CiphertextBlob)]
	if !ok || !maps.Equal(in.EncryptionContext, encryptionContext) {
		return nil, &types.InvalidCiphertextException{}
	}
	return &kms.DecryptOutput{Plaintext: append([]byte(nil), k...)}, nil
}

func TestKMSRoundTrip(t *testing.T) {
	f := &fakeKMS{keys: map[string][]byte{}}
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	s := NewKMS(f, "alias/test")
	s.Now = func() time.Time { return now }
	ctx := context.Background()

	ct, err := s.Seal(ctx, []byte("35.681236,139.767125"), []byte("LOC1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, []byte("35.681236")) {
		t.Fatal("暗号文に平文が含まれる")
	}
	pt, err := s.Open(ctx, ct, []byte("LOC1"))
	if err != nil || string(pt) != "35.681236,139.767125" {
		t.Fatalf("Open = %q, %v", pt, err)
	}
	if _, err := s.Open(ctx, ct, []byte("LOC2")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("別の地点の ID で復号できた: %v", err)
	}
	bad := append([]byte(nil), ct...)
	bad[len(bad)-1] ^= 1
	if _, err := s.Open(ctx, bad, []byte("LOC1")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("改ざんした暗号文を復号できた: %v", err)
	}
	for _, b := range [][]byte{nil, {formatKMSv1}, {formatKMSv1, 0, 0}, {formatKMSv1, 0xff, 0xff, 1}, {formatAESGCMv1, 0, 1, 2}} {
		if _, err := s.Open(ctx, b, nil); !errors.Is(err, ErrDecrypt) {
			t.Errorf("Open(%v) = %v, want ErrDecrypt", b, err)
		}
	}

	// 別のプロセス（キャッシュなし）でも復号できる。2回目からは KMS を呼ばない
	s2 := NewKMS(f, "alias/test")
	for range 3 {
		if _, err := s2.Open(ctx, ct, []byte("LOC1")); err != nil {
			t.Fatal(err)
		}
	}
	if f.decrypt != 1 {
		t.Errorf("Decrypt を %d 回呼んだ（キャッシュが効いていない）", f.decrypt)
	}
}

func TestKMSDataKeyRotation(t *testing.T) {
	f := &fakeKMS{keys: map[string][]byte{}}
	now := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	s := NewKMS(f, "alias/test")
	s.Now = func() time.Time { return now }
	s.DataKeyUses = 3
	ctx := context.Background()
	for range 3 {
		if _, err := s.Seal(ctx, []byte("x"), nil); err != nil {
			t.Fatal(err)
		}
	}
	if f.generate != 1 {
		t.Fatalf("3回の暗号化で GenerateDataKey を %d 回呼んだ", f.generate)
	}
	_, _ = s.Seal(ctx, []byte("x"), nil) // 回数の上限
	now = now.Add(6 * time.Minute)
	_, _ = s.Seal(ctx, []byte("x"), nil) // 時間の上限
	if f.generate != 3 {
		t.Errorf("GenerateDataKey = %d 回, want 3", f.generate)
	}
}
