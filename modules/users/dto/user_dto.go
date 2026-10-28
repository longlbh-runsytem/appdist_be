// Package dto chứa kiểu dữ liệu vào/ra của module users.
//
// Quy ước:
//
//	Payload  — dữ liệu vào từ HTTP, có binding tag.
//	*DTO     — kiểu dùng xuyên suốt project, kiểu Go thuần.
//	sqlc.*   — chỉ xuất hiện ở ranh giới repository.
package dto

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
)

// UserDTO là một tài khoản trả ra API (nội bộ hoặc tester).
// Không bao giờ chứa hashed_password.
//
// Role là quyền toàn hệ thống của tài khoản, KHÁC với việc là thành viên
// (member) của một nhóm — 1 user là thành viên được nhiều nhóm.
type UserDTO struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Role      sqlc.UserRole     `json:"role"   enums:"owner,admin,member,tester"`
	Status    sqlc.EntityStatus `json:"status" enums:"active,disabled,deleted"`
	CreatedAt time.Time         `json:"createdAt"`
}

func FromSQLC(u sqlc.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		Name:      u.FullName,
		Email:     u.Email,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
	}
}

func FromSQLCList(users []sqlc.User) []UserDTO {
	out := make([]UserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, FromSQLC(u))
	}
	return out
}

// MemberRefDTO là thành viên nhóm rút gọn, dùng trong Group.members.
type MemberRefDTO struct {
	ID    uuid.UUID     `json:"id"`
	Name  string        `json:"name"`
	Email string        `json:"email"`
	Role  sqlc.UserRole `json:"role" enums:"owner,admin,member"`
}

// UserGroupDTO là một nhóm mà user là thành viên.
type UserGroupDTO struct {
	ID     uuid.UUID         `json:"id"`
	Name   string            `json:"name"`
	Status sqlc.EntityStatus `json:"status" enums:"active,disabled"`
}

// NormalizeEmail đưa email về một dạng duy nhất trước khi ghi hoặc tra cứu.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// =============== REQUEST PAYLOAD ===============

// CreateUserPayload là body của POST /api/v1/users/create — tạo tài khoản dùng được ngay.
// Muốn user vào nhóm thì thêm sau qua POST /api/v1/groups/:id/members/:userId.
type CreateUserPayload struct {
	Email    string        `json:"email"    binding:"required,email,max=255" example:"dev1@company.com"`
	Name     string        `json:"name"     binding:"required,max=100"        example:"Dev One"`
	Password string        `json:"password" binding:"required,min=8,max=72"   example:"Dev@123456"`
	Role     sqlc.UserRole `json:"role"     binding:"required,oneof=owner admin member tester" enums:"owner,admin,member,tester"`
}

// ListUsersQuery là query của GET /api/v1/users và /users/deleted.
type ListUsersQuery struct {
	common.PageQuery
	Search string `form:"search" binding:"max=100"`
	Role   string `form:"role"   binding:"omitempty,oneof=owner admin member tester"     enums:"owner,admin,member,tester"`
	Sort   string `form:"sort"   binding:"omitempty,oneof=createdAt name email role"     enums:"createdAt,name,email,role"`
	Order  string `form:"order"  binding:"omitempty,oneof=asc desc"                      enums:"asc,desc"`
}

// UpdateRolePayload là body của PATCH /api/v1/users/:id/role.
type UpdateRolePayload struct {
	Role sqlc.UserRole `json:"role" binding:"required,oneof=owner admin member tester" enums:"owner,admin,member,tester"`
}

// UpdateStatusPayload là body của các endpoint PATCH .../status.
// Xoá đi qua DELETE, khôi phục qua .../restore — không đặt "deleted" ở đây.
type UpdateStatusPayload struct {
	Status sqlc.EntityStatus `json:"status" binding:"required,oneof=active disabled" enums:"active,disabled"`
}

// ChangePasswordPayload là body của POST /api/v1/users/me/password.
type ChangePasswordPayload struct {
	CurrentPassword string `json:"currentPassword" binding:"required,max=72"`
	NewPassword     string `json:"newPassword"     binding:"required,min=8,max=72"`
}

// AcceptInvitePayload là body của POST /api/v1/auth/accept-invite — dùng cho
// tester được thêm qua tester group (tài khoản nội bộ tạo kèm mật khẩu).
type AcceptInvitePayload struct {
	Token    string `json:"token"    binding:"required,max=128"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name"     binding:"max=100"`
}
