// Package services chứa nghiệp vụ của module users.
//
// Service phụ thuộc db.Store (= sqlc.Querier + ExecTx), không có tầng
// repository viết tay ở giữa. Service không biết gì về gin: mọi thứ nó cần
// đều đến qua tham số.
package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/token"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/utils"
)

type AuthService struct {
	store      db.Store
	tokenMaker token.TokenMaker
	cfg        utils.Config
}

func NewAuthService(store db.Store, tokenMaker token.TokenMaker, cfg utils.Config) *AuthService {
	return &AuthService{store: store, tokenMaker: tokenMaker, cfg: cfg}
}

// ClientInfo là thông tin do server suy ra từ request, không phải do client gửi.
type ClientInfo struct {
	IP        string
	UserAgent string
}

var (
	// Dùng CHUNG cho cả "không có user" lẫn "sai mật khẩu" để endpoint login
	// không thành công cụ dò tài khoản tồn tại.
	errInvalidCredentials = common.NewUnauthorized(
		errors.New("invalid credentials"),
		"tài khoản hoặc mật khẩu không đúng",
		"ErrInvalidCredentials",
	)

	errInviteInvalid = common.ErrBadRequest("link mời không hợp lệ hoặc đã hết hạn", "ErrInviteInvalid")

	errAccountInactive = common.NewFullErrorResponse(
		http.StatusForbidden,
		errors.New("account inactive"),
		"tài khoản đã bị khóa",
		"account inactive",
		common.ErrNoPermissionCode,
	)

	errSessionInvalid = common.NewUnauthorized(
		errors.New("session invalid"),
		"phiên đăng nhập không hợp lệ",
		"ErrSessionInvalid",
	)
)

// Login xác thực mật khẩu, mở session mới và cấp cặp access/refresh token.
func (s *AuthService) Login(ctx context.Context, p dto.LoginPayload, client ClientInfo) (*dto.LoginResponse, error) {
	user, err := s.store.GetUserByEmail(ctx, dto.NormalizeEmail(p.Email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errInvalidCredentials
		}
		return nil, common.ErrDB(err)
	}

	if err := utils.CheckPassword(p.Password, user.HashedPassword); err != nil {
		return nil, errInvalidCredentials
	}

	if user.Status != sqlc.EntityStatusActive {
		return nil, errAccountInactive
	}

	return s.issueSession(ctx, user, client)
}

func (s *AuthService) issueSession(ctx context.Context, user sqlc.User, client ClientInfo) (*dto.LoginResponse, error) {
	accessToken, accessPayload, err := s.tokenMaker.CreateToken(user.ID, s.cfg.AccessTokenDuration)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	refreshToken, refreshPayload, err := s.tokenMaker.CreateToken(user.ID, s.cfg.RefreshTokenDuration)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	// Session dùng chính ID của refresh token, nên khi refresh chỉ cần giải mã
	// token là tra được đúng bản ghi session.
	sessionPayload := dto.CreateSessionPayload{
		ID:           refreshPayload.ID,
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    client.UserAgent,
		ClientIP:     client.IP,
		ExpiresAt:    refreshPayload.ExpireAt,
	}

	session, err := s.store.CreateSession(ctx, sessionPayload.ToCreateParams())
	if err != nil {
		return nil, common.ErrDB(err)
	}

	res := dto.NewLoginResponse(
		dto.SessionFromSQLC(session),
		dto.FromSQLC(user),
		accessToken,
		accessPayload.ExpireAt,
	)
	return &res, nil
}

// AcceptInvite đặt mật khẩu lần đầu từ link mời rồi đăng nhập luôn.
//
// Mọi trường hợp link không dùng được (sai, đã dùng, hết hạn, tài khoản đã
// bị khoá/xoá) trả CÙNG một lỗi — không để link mời thành công cụ dò tài khoản.
func (s *AuthService) AcceptInvite(ctx context.Context, p dto.AcceptInvitePayload, client ClientInfo) (*dto.LoginResponse, error) {
	inv, err := s.store.GetInvitationByTokenHash(ctx, utils.SHA256Hex(p.Token))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errInviteInvalid
		}
		return nil, common.ErrDB(err)
	}
	if inv.AcceptedAt.Valid || time.Now().After(inv.ExpiresAt) {
		return nil, errInviteInvalid
	}

	hashed, err := utils.HashPassword(p.Password)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	var user sqlc.User
	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		current, err := q.GetUserByID(ctx, inv.UserID)
		if err != nil {
			return err
		}
		if current.Status != sqlc.EntityStatusActive {
			return errInviteInvalid
		}

		user, err = q.ActivateInvitedUser(ctx, sqlc.ActivateInvitedUserParams{
			ID:             inv.UserID,
			HashedPassword: hashed,
			FullName:       db.Text(p.Name),
		})
		if err != nil {
			return err
		}
		return q.CloseInvitations(ctx, inv.UserID)
	})
	if err != nil {
		return nil, wrapDB(err)
	}

	return s.issueSession(ctx, user, client)
}

// RenewAccessToken cấp access token mới từ refresh token còn hiệu lực.
func (s *AuthService) RenewAccessToken(ctx context.Context, p dto.RenewAccessTokenPayload) (*dto.RenewAccessTokenResponse, error) {
	session, err := s.loadUsableSession(ctx, p.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Tài khoản bị khoá sau khi đăng nhập thì refresh token cũng phải chết theo.
	user, err := s.store.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errSessionInvalid
		}
		return nil, common.ErrDB(err)
	}
	if user.Status != sqlc.EntityStatusActive {
		return nil, errAccountInactive
	}

	accessToken, accessPayload, err := s.tokenMaker.CreateToken(user.ID, s.cfg.AccessTokenDuration)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	return &dto.RenewAccessTokenResponse{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessPayload.ExpireAt,
	}, nil
}

// Logout chặn phiên ứng với refresh token.
func (s *AuthService) Logout(ctx context.Context, p dto.LogoutPayload) error {
	session, err := s.loadUsableSession(ctx, p.RefreshToken)
	if err != nil {
		return err
	}

	if err := s.store.BlockSession(ctx, session.ID); err != nil {
		return common.ErrDB(err)
	}
	return nil
}

// loadUsableSession giải mã refresh token và đối chiếu với bản ghi session.
//
// So khớp cả chuỗi token chứ không chỉ ID: token cũ bị lộ nhưng cùng ID
// (không xảy ra với uuid ngẫu nhiên, nhưng rẻ để chặn) vẫn bị từ chối.
func (s *AuthService) loadUsableSession(ctx context.Context, refreshToken string) (dto.SessionDTO, error) {
	payload, err := s.tokenMaker.VerifyToken(refreshToken)
	if err != nil {
		return dto.SessionDTO{}, err
	}

	row, err := s.store.GetSession(ctx, payload.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.SessionDTO{}, errSessionInvalid
		}
		return dto.SessionDTO{}, common.ErrDB(err)
	}

	session := dto.SessionFromSQLC(row)
	if session.RefreshToken != refreshToken || session.UserID != payload.UserID || !session.IsUsable() {
		return dto.SessionDTO{}, errSessionInvalid
	}
	return session, nil
}
