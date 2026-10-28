package token

import (
	"time"

	"github.com/google/uuid"
)

// TokenPayload là nội dung nằm trong token.
//
// ID vừa là định danh của token, vừa là id của session khi đây là refresh
// token — nhờ vậy handler refresh tra thẳng được bảng sessions mà không cần
// mang thêm trường nào.
type TokenPayload struct {
	ID       uuid.UUID `json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	IssuedAt time.Time `json:"issued_at"`
	ExpireAt time.Time `json:"expire_at"`
}

func NewPayload(userID uuid.UUID, duration time.Duration) (*TokenPayload, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &TokenPayload{
		ID:       id,
		UserID:   userID,
		IssuedAt: now,
		ExpireAt: now.Add(duration),
	}, nil
}

// Valid kiểm tra token còn hạn.
//
// Bắt buộc gọi sau khi giải mã: PASETO chỉ đảm bảo payload không bị sửa,
// nó KHÔNG tự kiểm tra thời hạn. Thiếu bước này thì token hết hạn vẫn dùng
// được vĩnh viễn.
func (p *TokenPayload) Valid() error {
	if time.Now().After(p.ExpireAt) {
		return ErrExpiredToken
	}
	return nil
}
