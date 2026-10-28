package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// KMSAPI は KMS のうち使う操作（テストではフェイクに差し替える）。
type KMSAPI interface {
	GenerateDataKey(ctx context.Context, in *kms.GenerateDataKeyInput, opts ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error)
	Decrypt(ctx context.Context, in *kms.DecryptInput, opts ...func(*kms.Options)) (*kms.DecryptOutput, error)
}

const formatKMSv1 byte = 2

// encryptionContext は KMS に渡す暗号化コンテキスト。データキーをこのアプリの用途以外で復号できないようにする。
var encryptionContext = map[string]string{"app": "saigai-timeline", "purpose": "field"}

// KMS は KMS のデータキーによるエンベロープ暗号化（docs/07 3.5）。
//
// 暗号文の形式: [2][暗号化したデータキーの長さ(2バイト)][暗号化したデータキー][nonce][AES-GCM の暗号文]
//
// KMS の呼び出し回数（料金と遅延）を減らすため、データキーは一定の時間・回数だけ使い回し、
// 復号したデータキーはプロセス内に短時間キャッシュする。平文のデータキーはメモリにだけ置く。
type KMS struct {
	Client KMSAPI
	KeyID  string // キーの ARN またはエイリアスの ARN

	// DataKeyTTL と DataKeyUses は、1つのデータキーで暗号化する時間と回数の上限
	DataKeyTTL  time.Duration
	DataKeyUses int
	// CacheTTL は復号したデータキーをキャッシュする時間
	CacheTTL time.Duration
	Now      func() time.Time

	mu    sync.Mutex
	cur   *dataKey
	cache map[string]*dataKey // 暗号化したデータキー → 復号したデータキー
}

type dataKey struct {
	encrypted []byte
	aead      cipher.AEAD
	created   time.Time
	uses      int
}

// NewKMS は既定の設定で KMS を作る。
func NewKMS(client KMSAPI, keyID string) *KMS {
	return &KMS{Client: client, KeyID: keyID, DataKeyTTL: 5 * time.Minute, DataKeyUses: 1000, CacheTTL: 5 * time.Minute, Now: time.Now}
}

const maxCachedKeys = 256

func (s *KMS) Seal(ctx context.Context, plaintext, aad []byte) ([]byte, error) {
	dk, err := s.encryptKey(ctx)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, dk.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 3+len(dk.encrypted)+len(nonce)+len(plaintext)+dk.aead.Overhead())
	out = append(out, formatKMSv1)
	out = binary.BigEndian.AppendUint16(out, uint16(len(dk.encrypted))) //nolint:gosec // KMS の暗号化したデータキーは数百バイト
	out = append(out, dk.encrypted...)
	out = append(out, nonce...)
	return dk.aead.Seal(out, nonce, plaintext, aad), nil
}

func (s *KMS) Open(ctx context.Context, ciphertext, aad []byte) ([]byte, error) {
	if len(ciphertext) < 3 || ciphertext[0] != formatKMSv1 {
		return nil, ErrDecrypt
	}
	n := int(binary.BigEndian.Uint16(ciphertext[1:3]))
	rest := ciphertext[3:]
	if n == 0 || len(rest) < n {
		return nil, ErrDecrypt
	}
	dk, err := s.decryptKey(ctx, rest[:n])
	if err != nil {
		return nil, err
	}
	rest = rest[n:]
	ns := dk.aead.NonceSize()
	if len(rest) < ns {
		return nil, ErrDecrypt
	}
	pt, err := dk.aead.Open(nil, rest[:ns], rest[ns:], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return pt, nil
}

// encryptKey は暗号化に使うデータキーを返す。期限か回数の上限に達していたら新しく作る。
func (s *KMS) encryptKey(ctx context.Context) (*dataKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	if s.cur != nil && now.Sub(s.cur.created) < s.DataKeyTTL && s.cur.uses < s.DataKeyUses {
		s.cur.uses++
		return s.cur, nil
	}
	out, err := s.Client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId: aws.String(s.KeyID), KeySpec: types.DataKeySpecAes256, EncryptionContext: encryptionContext,
	})
	if err != nil {
		return nil, fmt.Errorf("kms GenerateDataKey: %w", err)
	}
	aead, err := newAEAD(out.Plaintext)
	clear(out.Plaintext)
	if err != nil {
		return nil, err
	}
	s.cur = &dataKey{encrypted: out.CiphertextBlob, aead: aead, created: now, uses: 1}
	s.putCache(s.cur)
	return s.cur, nil
}

// decryptKey は暗号化したデータキーを復号する。キャッシュにあれば KMS を呼ばない。
func (s *KMS) decryptKey(ctx context.Context, encrypted []byte) (*dataKey, error) {
	s.mu.Lock()
	if dk, ok := s.cache[string(encrypted)]; ok && s.Now().Sub(dk.created) < s.CacheTTL {
		s.mu.Unlock()
		return dk, nil
	}
	s.mu.Unlock()

	out, err := s.Client.Decrypt(ctx, &kms.DecryptInput{
		KeyId: aws.String(s.KeyID), CiphertextBlob: encrypted, EncryptionContext: encryptionContext,
	})
	if err != nil {
		var ic *types.InvalidCiphertextException
		if errors.As(err, &ic) {
			return nil, ErrDecrypt
		}
		return nil, fmt.Errorf("kms Decrypt: %w", err)
	}
	aead, err := newAEAD(out.Plaintext)
	clear(out.Plaintext)
	if err != nil {
		return nil, err
	}
	dk := &dataKey{encrypted: append([]byte(nil), encrypted...), aead: aead, created: s.Now()}
	s.mu.Lock()
	s.putCache(dk)
	s.mu.Unlock()
	return dk, nil
}

// putCache はキャッシュに入れる。上限を超えたら期限切れのものを消し、それでも多ければすべて消す。
// 呼び出し元が mu を持っていること。
func (s *KMS) putCache(dk *dataKey) {
	if s.cache == nil {
		s.cache = map[string]*dataKey{}
	}
	if len(s.cache) >= maxCachedKeys {
		now := s.Now()
		for k, v := range s.cache {
			if now.Sub(v.created) >= s.CacheTTL {
				delete(s.cache, k)
			}
		}
		if len(s.cache) >= maxCachedKeys {
			clear(s.cache)
		}
	}
	s.cache[string(dk.encrypted)] = dk
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("data key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
