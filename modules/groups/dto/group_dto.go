// Package dto chứa kiểu dữ liệu vào/ra của module groups (nhóm quản trị).
package dto

import (
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/db/sqlc"
	userDTO "github.com/longlbh/appdist-be/modules/users/dto"
)

// GroupDTO là một nhóm quản trị. Members trả object rút gọn để FE hiển thị
// ngay, không phải gọi thêm GET /users rồi tự ghép.
type GroupDTO struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	ParentID *uuid.UUID             `json:"parentId"`
	Status   sqlc.EntityStatus      `json:"status" enums:"active,disabled,deleted"`
	Members  []userDTO.MemberRefDTO `json:"members"`
}

func FromSQLC(g sqlc.Group, members []userDTO.MemberRefDTO) GroupDTO {
	if members == nil {
		members = []userDTO.MemberRefDTO{}
	}
	return GroupDTO{
		ID:       g.ID,
		Name:     g.Name,
		ParentID: g.ParentID,
		Status:   g.Status,
		Members:  members,
	}
}

// CreateGroupPayload là body của POST /api/v1/groups.
type CreateGroupPayload struct {
	Name     string     `json:"name"     binding:"required,max=100" example:"Mobile Team"`
	ParentID *uuid.UUID `json:"parentId"`
}
