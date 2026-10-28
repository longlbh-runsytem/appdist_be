// Package dto chứa kiểu dữ liệu vào/ra của module testers.
package dto

import (
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/db/sqlc"
)

// TesterGroupDTO là một nhóm tester. MemberCount chỉ đếm tester active.
type TesterGroupDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MemberCount int32     `json:"memberCount"`
}

// TesterDTO là một tester trong nhóm.
//
// InviteURL chỉ có khi vừa thêm một email chưa có tài khoản (hoặc chưa nhận
// lời mời) — gửi link này cho tester để họ đặt mật khẩu.
type TesterDTO struct {
	ID        uuid.UUID         `json:"id"`
	Email     string            `json:"email"`
	Status    sqlc.EntityStatus `json:"status" enums:"active,disabled,deleted"`
	InviteURL string            `json:"inviteUrl,omitempty"`
}

// CreateTesterGroupPayload là body của POST /api/v1/tester-groups.
type CreateTesterGroupPayload struct {
	Name string `json:"name" binding:"required,max=100" example:"Nhóm QA nội bộ"`
}

// AddTestersPayload là body của POST /api/v1/tester-groups/:id/testers.
type AddTestersPayload struct {
	Emails []string `json:"emails" binding:"required,min=1,max=100,dive,email,max=255" example:"qa1@company.com"`
}
