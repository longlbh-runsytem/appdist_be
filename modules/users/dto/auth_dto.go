package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/db/sqlc"
)

// =============== SESSION ===============

// SessionDTO là một phiên đăng nhập (một refresh token còn sống).
//
// RefreshToken gắn `json:"-"`: nó chỉ rời server đúng một lần trong
// LoginResponse lúc tạo phiên.
type SessionDTO struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"userId"`
	UserAgent string    `json:"userAgent"`
	ClientIP  string    `json:"clientIp"`
	IsBlocked bool      `json:"isBlocked"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`

	RefreshToken string `json:"-"`
}

// IsUsable gộp mọi điều kiện để một phiên còn dùng được.
func (s SessionDTO) IsUsable() bool {
	return !s.IsBlocked && time.Now().Before(s.ExpiresAt)
}

func SessionFromSQLC(s sqlc.Session) SessionDTO {
	return SessionDTO{
		ID:           s.ID,
		UserID:       s.UserID,
		UserAgent:    s.UserAgent,
		ClientIP:     s.ClientIp,
		IsBlocked:    s.IsBlocked,
		ExpiresAt:    s.ExpiresAt,
		CreatedAt:    s.CreatedAt,
		RefreshToken: s.RefreshToken,
	}
}

// CreateSessionPayload là dữ liệu nội bộ để mở một phiên — service tự dựng,
// không đến từ body.
type CreateSessionPayload struct {
	ID           uuid.UUID // trùng với ID trong payload của refresh token
	UserID       uuid.UUID
	RefreshToken string
	UserAgent    string
	ClientIP     string
	ExpiresAt    time.Time
}

func (p CreateSessionPayload) ToCreateParams() sqlc.CreateSessionParams {
	return sqlc.CreateSessionParams{
		ID:           p.ID,
		UserID:       p.UserID,
		RefreshToken: p.RefreshToken,
		UserAgent:    p.UserAgent,
		ClientIp:     p.ClientIP,
		ExpiresAt:    p.ExpiresAt,
	}
}

// =============== REQUEST PAYLOAD ===============

// LoginPayload là body của POST /api/v1/auth/login.
//
// Example là tài khoản test, để "Try it out" trên Swagger đăng nhập được ngay.
type LoginPayload struct {
	Email    string `json:"email"    binding:"required,email,max=255" example:"tester@appdist.local"`
	Password string `json:"password" binding:"required,max=72"        example:"Test@e41ca7"`
}

// RenewAccessTokenPayload là body của POST /api/v1/auth/refresh.
type RenewAccessTokenPayload struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// LogoutPayload là body của POST /api/v1/auth/logout.
// Chặn đúng phiên đang dùng thay vì mọi phiên của user.
type LogoutPayload struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// =============== RESPONSE ===============

// LoginResponse trả về sau khi đăng nhập thành công.
// Đây là lần duy nhất RefreshToken rời khỏi server.
type LoginResponse struct {
	SessionID             uuid.UUID `json:"sessionId"`
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	User                  UserDTO   `json:"user"`
}

func NewLoginResponse(session SessionDTO, user UserDTO, accessToken string, accessTokenExpiresAt time.Time) LoginResponse {
	return LoginResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessTokenExpiresAt,
		RefreshToken:          session.RefreshToken,
		RefreshTokenExpiresAt: session.ExpiresAt,
		User:                  user,
	}
}

// RenewAccessTokenResponse trả về sau khi làm mới access token.
type RenewAccessTokenResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
}
