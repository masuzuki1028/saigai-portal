// Package crypto は正確な座標・住所などをアプリ層で暗号化する（docs/07_security.md 3.5）。
//
// 本番は KMS のデータキーによるエンベロープ暗号化（KMS）を使う。
// ローカルとテストでは、固定の鍵で AES-GCM を使う AESGCM を使う。
package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// Sealer は値を暗号化・復号する。aad（追加認証データ）には地点の ID などを渡し、
// 暗号文を別の項目にコピーしても復号できないようにする。
type Sealer interface {
	Seal(ctx context.Context, plaintext, aad []byte) ([]byte, error)
	Open(ctx context.Context, ciphertext, aad []byte) ([]byte, error)
}

const formatAESGCMv1 byte = 1

// AESGCM は鍵を直接持つ Sealer。ローカル・テスト用。
type AESGCM struct {
	aead cipher.AEAD
}

// NewAESGCM は 32 バイトの鍵から AESGCM を作る。
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCM{aead: aead}, nil
}

func (s *AESGCM) Seal(_ context.Context, plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte{formatAESGCMv1}, nonce...)
	return s.aead.Seal(out, nonce, plaintext, aad), nil
}

var ErrDecrypt = errors.New("decrypt failed")

func (s *AESGCM) Open(_ context.Context, ciphertext, aad []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(ciphertext) < 1+n || ciphertext[0] != formatAESGCMv1 {
		return nil, ErrDecrypt
	}
	pt, err := s.aead.Open(nil, ciphertext[1:1+n], ciphertext[1+n:], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return pt, nil
}
