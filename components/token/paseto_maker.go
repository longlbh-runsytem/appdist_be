package token

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/o1egl/paseto"
	"golang.org/x/crypto/chacha20poly1305"
)

type pasetoMaker struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewPasetoMaker(symmetricKey string) (TokenMaker, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: must be exactly %d characters", chacha20poly1305.KeySize)
	}

	return &pasetoMaker{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}, nil
}

func (m *pasetoMaker) CreateToken(userID uuid.UUID, duration time.Duration) (string, *TokenPayload, error) {
	payload, err := NewPayload(userID, duration)
	if err != nil {
		return "", nil, err
	}

	token, err := m.paseto.Encrypt(m.symmetricKey, payload, nil)
	if err != nil {
		return "", nil, ErrEncodingToken
	}

	return token, payload, nil
}

func (m *pasetoMaker) VerifyToken(token string) (*TokenPayload, error) {
	payload := &TokenPayload{}

	if err := m.paseto.Decrypt(token, m.symmetricKey, payload, nil); err != nil {
		return nil, ErrInvalidToken
	}

	// Giải mã thành công mới chỉ chứng minh token chưa bị sửa.
	// Hạn dùng phải kiểm tra riêng.
	if err := payload.Valid(); err != nil {
		return nil, err
	}

	return payload, nil
}
