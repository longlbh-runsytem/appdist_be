package services

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/utils"
)

const (
	inviteTTL         = 7 * 24 * time.Hour
	inviteTokenLength = 40
)

// Inviter tạo tài khoản theo email và cấp link mời.
//
// Chưa có hạ tầng gửi email: link mời trả về trong response để người mời tự
// gửi. Module testers cũng dùng Inviter (qua interface của riêng nó) để mời
// tester có tài khoản đăng nhập.
type Inviter struct {
	cfg utils.Config
}

func NewInviter(cfg utils.Config) *Inviter {
	return &Inviter{cfg: cfg}
}

// EnsureUser trả user có email này; chưa có thì tạo với role cho trước kèm
// link mời. Tài khoản tạo rồi nhưng chưa nhận lời mời thì cấp link mới.
// inviteURL rỗng = tài khoản đã kích hoạt, không cần mời.
//
// Chạy trong transaction của caller (q).
func (i *Inviter) EnsureUser(ctx context.Context, q *sqlc.Queries, email string, role sqlc.UserRole, invitedBy uuid.UUID) (sqlc.User, string, error) {
	email = dto.NormalizeEmail(email)

	user, err := q.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		if user.HashedPassword != "" {
			return user, "", nil
		}
	case db.IsNotFound(err):
		user, err = q.CreateUser(ctx, sqlc.CreateUserParams{
			Email:          email,
			HashedPassword: "",
			FullName:       email,
			Role:           role,
			InvitedBy:      &invitedBy,
		})
		if err != nil {
			return sqlc.User{}, "", err
		}
	default:
		return sqlc.User{}, "", err
	}

	link, err := i.Issue(ctx, q, user.ID)
	if err != nil {
		return sqlc.User{}, "", err
	}
	return user, link, nil
}

// Issue đóng các lời mời cũ của user và cấp lời mời mới.
func (i *Inviter) Issue(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (string, error) {
	token, err := utils.RandomString(inviteTokenLength, utils.ReadableAlphabet)
	if err != nil {
		return "", common.ErrInternal(err)
	}

	if err := q.CloseInvitations(ctx, userID); err != nil {
		return "", err
	}
	if _, err := q.CreateInvitation(ctx, sqlc.CreateInvitationParams{
		UserID:    userID,
		TokenHash: utils.SHA256Hex(token),
		ExpiresAt: time.Now().Add(inviteTTL),
	}); err != nil {
		return "", err
	}

	return i.cfg.FrontendURL + "/accept-invite?token=" + url.QueryEscape(token), nil
}
