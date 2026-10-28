package token

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
)

// TokenMaker ký và xác thực token.
//
// duration là tham số vì access token và refresh token có vòng đời rất khác
// nhau (phút so với ngày) nhưng dùng chung một cơ chế ký.
type TokenMaker interface {
	CreateToken(userID uuid.UUID, duration time.Duration) (string, *TokenPayload, error)
	VerifyToken(token string) (*TokenPayload, error)
}

var ErrInvalidToken = common.NewUnauthorized(
	errors.New("token invalid"),
	"token invalid",
	"ErrInvalidToken",
)

var ErrExpiredToken = common.NewUnauthorized(
	errors.New("token expired"),
	"token expired",
	"ErrExpiredToken",
)

var ErrEncodingToken = common.NewCustomError(
	errors.New("error encoding the token"),
	"error encoding the token",
	"ErrEncodingToken",
)
