// Package handler là tầng duy nhất biết đến gin.
//
// Nhiệm vụ: bind + validate body, rút thông tin từ request (IP, user agent,
// path param), gọi service, trả kết quả. Không có nghiệp vụ ở đây.
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/modules/users/services"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login godoc
//
//	@Summary	Đăng nhập
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.LoginPayload	true	"Email và mật khẩu"
//	@Success	200		{object}	common.SuccessRes{data=dto.LoginResponse}
//	@Failure	401		{object}	common.AppError
//	@Failure	400		{object}	common.AppError
//	@Failure	500		{object}	common.AppError
//	@Router		/api/v1/auth/login [post]
func (h *AuthHandler) Login() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var p dto.LoginPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}

		// IP và user agent do server suy ra, không nhận từ body.
		client := services.ClientInfo{
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}

		return h.svc.Login(c.Request.Context(), p, client)
	})
}

// RenewAccessToken godoc
//
//	@Summary	Làm mới access token bằng refresh token
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.RenewAccessTokenPayload	true	"Refresh token"
//	@Success	200		{object}	common.SuccessRes{data=dto.RenewAccessTokenResponse}
//	@Failure	401		{object}	common.AppError
//	@Failure	400		{object}	common.AppError
//	@Failure	500		{object}	common.AppError
//	@Router		/api/v1/auth/refresh [post]
func (h *AuthHandler) RenewAccessToken() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var p dto.RenewAccessTokenPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.RenewAccessToken(c.Request.Context(), p)
	})
}

// Logout godoc
//
//	@Summary	Đăng xuất (chặn phiên của refresh token này)
//	@Tags		auth
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.LogoutPayload	true	"Refresh token của phiên cần đóng"
//	@Success	200		{object}	common.SuccessRes{data=SuccessResult}
//	@Failure	401		{object}	common.AppError
//	@Failure	400		{object}	common.AppError
//	@Failure	500		{object}	common.AppError
//	@Router		/api/v1/auth/logout [post]
func (h *AuthHandler) Logout() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var p dto.LogoutPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		if err := h.svc.Logout(c.Request.Context(), p); err != nil {
			return nil, err
		}
		return SuccessResult{Success: true}, nil
	})
}

// AcceptInvite godoc
//
//	@Summary		Nhận lời mời: đặt mật khẩu lần đầu và đăng nhập
//	@Description	token lấy từ link mời (inviteUrl) trả về khi tạo member/tester.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.AcceptInvitePayload	true	"Token mời, mật khẩu mới, tên hiển thị (tuỳ chọn)"
//	@Success		200		{object}	common.SuccessRes{data=dto.LoginResponse}
//	@Failure		400		{object}	common.AppError
//	@Failure		500		{object}	common.AppError
//	@Router			/api/v1/auth/accept-invite [post]
func (h *AuthHandler) AcceptInvite() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var p dto.AcceptInvitePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		client := services.ClientInfo{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
		return h.svc.AcceptInvite(c.Request.Context(), p, client)
	})
}

// SuccessResult là data của các thao tác không có gì để trả ngoài "đã xong".
type SuccessResult struct {
	Success bool `json:"success" example:"true"`
}
