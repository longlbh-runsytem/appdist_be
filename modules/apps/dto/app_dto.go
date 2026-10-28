// Package dto chứa kiểu dữ liệu vào/ra của module apps.
package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
)

// PlatformDTO là cấu hình một platform của App.
type PlatformDTO struct {
	BundleID string `json:"bundleId" binding:"required,max=255" example:"com.gmo.delivery.ios"`
}

// PlatformsDTO: platform không dùng thì bỏ trống (null).
type PlatformsDTO struct {
	IOS     *PlatformDTO `json:"ios,omitempty"`
	Android *PlatformDTO `json:"android,omitempty"`
}

// AppDTO là một App. managingGroups/defaultTesterGroups trả object rút gọn
// {id, name}, không phải mảng id trần.
type AppDTO struct {
	ID                     uuid.UUID       `json:"id"`
	Name                   string          `json:"name"`
	Platforms              PlatformsDTO    `json:"platforms"`
	IconURL                *string         `json:"iconUrl"`
	IsPublic               bool            `json:"isPublic"`
	ManagingGroups         []common.RefDTO `json:"managingGroups"`
	CreatedAt              time.Time       `json:"createdAt"`
	ExpiresAt              *time.Time      `json:"expiresAt"`
	DownloadLimit          *int32          `json:"downloadLimit"`
	RequiresApproval       bool            `json:"requiresApproval"`
	AutoAssignTesterGroups bool            `json:"autoAssignTesterGroups"`
	DefaultTesterGroups    []common.RefDTO `json:"defaultTesterGroups"`
}

// AppSummaryDTO là App rút gọn cho phía tester (GET /tester/my-apps).
type AppSummaryDTO struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	Platforms PlatformsDTO `json:"platforms"`
	IconURL   *string      `json:"iconUrl"`
}

// PublicAppDTO là App trên trang công khai (GET /apps/:id/public) — không
// có thông tin quản trị vì ai có link cũng xem được.
type PublicAppDTO struct {
	ID        uuid.UUID    `json:"id"`
	Name      string       `json:"name"`
	IconURL   *string      `json:"iconUrl"`
	Platforms PlatformsDTO `json:"platforms"`
	Builds    AppBuildPage `json:"builds"`
}

// =============== DANH SÁCH BUILD (tester / trang công khai) ===============
//
// Dùng chung cho GET /tester/my-apps/:appId/builds và GET /apps/:id/public
// để hai API có cùng query params và cùng dạng response.

// ListAppBuildsQuery là query lọc / sắp xếp / phân trang danh sách build.
type ListAppBuildsQuery struct {
	common.PageQuery
	Platform string `form:"platform" binding:"omitempty,oneof=ios android"            enums:"ios,android"`
	Sort     string `form:"sort"     binding:"omitempty,oneof=buildNumber uploadedAt" enums:"buildNumber,uploadedAt"`
	Order    string `form:"order"    binding:"omitempty,oneof=asc desc"               enums:"asc,desc"`
}

// PlatformCountsDTO là số build theo từng platform (không phụ thuộc bộ lọc platform).
type PlatformCountsDTO struct {
	IOS     int64 `json:"ios"`
	Android int64 `json:"android"`
	All     int64 `json:"all"`
}

// AppBuildPage là một trang build kèm số lượng theo platform.
// total/totalPages/hasMore tính theo bộ lọc platform đang áp.
type AppBuildPage struct {
	Items      []AppBuildDTO     `json:"items"`
	Page       int32             `json:"page"`
	PageSize   int32             `json:"pageSize"`
	Total      int64             `json:"total"`
	TotalPages int32             `json:"totalPages"`
	HasMore    bool              `json:"hasMore"`
	Counts     PlatformCountsDTO `json:"counts"`
}

// AppBuildDTO là build cài được (status=ready) — không có thông tin nội bộ
// như người upload. Mở bằng GET /api/v1/releases/{shareToken} để lấy installUrl.
type AppBuildDTO struct {
	ID          int64         `json:"id,string" swaggertype:"string" example:"9"`
	AppID       uuid.UUID     `json:"appId"`
	Platform    sqlc.Platform `json:"platform" enums:"ios,android"`
	IconURL     *string       `json:"iconUrl"`
	Version     string        `json:"version"`
	BuildNumber string        `json:"buildNumber"`
	Status      string        `json:"status" example:"ready"`
	SizeBytes   int64         `json:"sizeBytes"`
	Changelog   string        `json:"changelog"`
	UploadedAt  time.Time     `json:"uploadedAt"`
	ShareToken  string        `json:"shareToken"`
}

// =============== REQUEST PAYLOAD ===============

// CreateAppPayload là body của POST /api/v1/apps.
type CreateAppPayload struct {
	Name             string       `json:"name"             binding:"required,max=100" example:"GMO Delivery"`
	Platforms        PlatformsDTO `json:"platforms"`
	ManagingGroupIDs []uuid.UUID  `json:"managingGroupIds" binding:"required,min=1,max=20"`
}

// UpdateAppInfoPayload là body của PATCH /api/v1/apps/:id/info.
type UpdateAppInfoPayload struct {
	Name      string       `json:"name"      binding:"required,max=100" example:"GMO Delivery"`
	Platforms PlatformsDTO `json:"platforms"`
}

// VisibilityPayload là body của PATCH /api/v1/apps/:id/visibility.
// Ghi đè cả 3 trường: expiresAt/downloadLimit null = bỏ giới hạn.
type VisibilityPayload struct {
	IsPublic      *bool      `json:"isPublic"      binding:"required"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	DownloadLimit *int32     `json:"downloadLimit" binding:"omitempty,min=1" example:"500"`
}

// ApprovalPayload là body của PATCH /api/v1/apps/:id/approval.
type ApprovalPayload struct {
	RequiresApproval *bool `json:"requiresApproval" binding:"required"`
}

// AutoAssignPayload là body của PATCH /api/v1/apps/:id/auto-assign-tester-groups.
type AutoAssignPayload struct {
	AutoAssignTesterGroups *bool `json:"autoAssignTesterGroups" binding:"required"`
}

// ListAppsQuery là query của GET /api/v1/apps.
type ListAppsQuery struct {
	common.PageQuery
	Search   string     `form:"search"   binding:"max=100"`
	Platform string     `form:"platform" binding:"omitempty,oneof=ios android" enums:"ios,android"`
	From     *time.Time `form:"from"     time_format:"2006-01-02T15:04:05Z07:00"`
	To       *time.Time `form:"to"       time_format:"2006-01-02T15:04:05Z07:00"`
}

// ListMyAppsQuery là query của GET /api/v1/tester/my-apps.
type ListMyAppsQuery struct {
	common.PageQuery
	Search string     `form:"search" binding:"max=100"`
	From   *time.Time `form:"from"   time_format:"2006-01-02T15:04:05Z07:00"`
	To     *time.Time `form:"to"     time_format:"2006-01-02T15:04:05Z07:00"`
}

// =============== API KEY ===============

// APIKeyDTO là một API key của App. Token (plaintext) chỉ có trong response
// lúc tạo — không lưu lại, không trả lại ở GET.
type APIKeyDTO struct {
	ID           uuid.UUID  `json:"id"`
	AppID        uuid.UUID  `json:"appId"`
	Label        string     `json:"label"`
	TokenPreview string     `json:"tokenPreview"`
	CreatedAt    time.Time  `json:"createdAt"`
	CreatedBy    string     `json:"createdBy"`
	LastUsedAt   *time.Time `json:"lastUsedAt"`
	Token        string     `json:"token,omitempty"`
}

// CreateAPIKeyPayload là body của POST /api/v1/apps/:appId/tokens.
type CreateAPIKeyPayload struct {
	Label string `json:"label" binding:"required,max=100" example:"GitHub Actions"`
}
