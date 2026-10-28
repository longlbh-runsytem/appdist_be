// Package dto chứa kiểu dữ liệu vào/ra của module builds.
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
)

// =============== REQUEST PAYLOAD ===============

// InitUploadPayload là body của POST /api/v1/builds/uploads — bước 1: xin URL upload.
//
// FileName chỉ dùng để lấy đuôi file đặt tên object, KHÔNG dùng để xác định
// loại file: loại thật đọc từ nội dung ở bước 3.
type InitUploadPayload struct {
	FileName string `json:"fileName" binding:"required,max=255" example:"app-release.apk"`
	FileSize int64  `json:"fileSize" binding:"required,gt=0"    example:"45000000"`
}

// CreateBuildPayload là body của POST /api/v1/builds — bước 3: sau khi PUT xong.
type CreateBuildPayload struct {
	UploadKey string    `json:"uploadKey" binding:"required,max=512"`
	AppID     uuid.UUID `json:"appId"     binding:"required"`
	Changelog string    `json:"changelog" binding:"max=2000" example:"Sửa lỗi đăng nhập"`
}

// ListBuildsQuery là query string của GET /api/v1/builds và
// GET /api/v1/apps/:id/builds (AppID lấy từ path).
// AppID là chuỗi vì gin không bind được query vào uuid.UUID; service parse.
type ListBuildsQuery struct {
	common.PageQuery
	AppID    string     `form:"appId"    binding:"omitempty,uuid"`
	Search   string     `form:"search"   binding:"max=100"`
	Platform string     `form:"platform" binding:"omitempty,oneof=ios android"                                     enums:"ios,android"`
	Status   string     `form:"status"   binding:"omitempty,oneof=processing ready pending_approval rejected archived" enums:"processing,ready,pending_approval,rejected,archived"`
	From     *time.Time `form:"from"     time_format:"2006-01-02T15:04:05Z07:00"`
	To       *time.Time `form:"to"       time_format:"2006-01-02T15:04:05Z07:00"`
	Sort     string     `form:"sort"     binding:"omitempty,oneof=uploadedAt version buildNumber"                  enums:"uploadedAt,version,buildNumber"`
	Order    string     `form:"order"    binding:"omitempty,oneof=asc desc"                                        enums:"asc,desc"`
}

// ReviewPayload là body của POST /api/v1/approvals/:id/review.
type ReviewPayload struct {
	Decision sqlc.ApprovalDecision `json:"decision" binding:"required,oneof=approve reject" enums:"approve,reject"`
	Note     string                `json:"note"     binding:"max=2000"`
}

// =============== RESPONSE ===============

// InitUploadResponse cho client biết PUT file lên đâu.
type InitUploadResponse struct {
	UploadKey string    `json:"uploadKey"`
	UploadURL string    `json:"uploadUrl"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// UploadContentResult là data trả về sau khi đẩy file qua server thành công.
type UploadContentResult struct {
	Uploaded  bool   `json:"uploaded"  example:"true"`
	UploadKey string `json:"uploadKey"`
}

// BuildDTO là một build trả ra API.
//
// IconURL copy từ icon của App cha để FE render được 1 dòng build độc lập.
// Không trả file_key / icon_key thô: bucket private, client chỉ dùng được URL
// đã ký. DownloadURL là trang tải bên FE, đã kèm vé (?t=) nếu App không public.
type BuildDTO struct {
	ID           int64            `json:"id,string" swaggertype:"string" example:"42"`
	AppID        *uuid.UUID       `json:"appId"`
	Platform     sqlc.Platform    `json:"platform" enums:"ios,android"`
	IconURL      *string          `json:"iconUrl"`
	Version      string           `json:"version"`
	BuildNumber  string           `json:"buildNumber"`
	Status       sqlc.BuildStatus `json:"status" enums:"processing,ready,pending_approval,rejected,archived"`
	SizeBytes    int64            `json:"sizeBytes"`
	Changelog    string           `json:"changelog"`
	UploadedAt   time.Time        `json:"uploadedAt"`
	UploadedBy   string           `json:"uploadedBy"`
	ExpiresAt    *time.Time       `json:"expiresAt"`
	TesterGroups []common.RefDTO  `json:"testerGroups"`
	ShareToken   string           `json:"shareToken"`

	// Thông tin đọc từ file, ngoài tài liệu FE.
	AppName       string         `json:"appName"`
	BundleID      string         `json:"bundleId"`
	MinOSVersion  string         `json:"minOsVersion,omitempty"`
	Metadata      map[string]any `json:"metadata"`
	DownloadCount int32          `json:"downloadCount"`
	DownloadURL   string         `json:"downloadUrl"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// FromSQLC chuyển bản ghi DB sang DTO. URL, email, tester group do service điền sau.
func FromSQLC(b sqlc.Build) BuildDTO {
	d := BuildDTO{
		ID:            b.ID,
		AppID:         b.AppID,
		Platform:      b.Platform,
		Version:       b.Version,
		BuildNumber:   b.BuildNumber,
		Status:        b.Status,
		SizeBytes:     b.FileSize,
		Changelog:     b.ReleaseNotes.String,
		UploadedAt:    b.CreatedAt,
		TesterGroups:  []common.RefDTO{},
		ShareToken:    b.Token,
		AppName:       b.AppName,
		BundleID:      b.BundleID,
		MinOSVersion:  b.MinOsVersion.String,
		Metadata:      map[string]any{},
		DownloadCount: b.DownloadCount,
	}
	if b.ExpiresAt.Valid {
		t := b.ExpiresAt.Time
		d.ExpiresAt = &t
	}
	// Metadata do chính server ghi nên luôn là JSON hợp lệ; lỗi thì để map rỗng.
	_ = json.Unmarshal(b.Metadata, &d.Metadata)
	return d
}

// ApprovalDTO là một yêu cầu duyệt build đang chờ.
type ApprovalDTO struct {
	ID          uuid.UUID `json:"id"`
	BuildID     int64     `json:"buildId,string" swaggertype:"string"`
	AppName     string    `json:"appName"`
	Version     string    `json:"version"`
	BuildNumber string    `json:"buildNumber"`
	RequestedBy string    `json:"requestedBy"`
	RequestedAt time.Time `json:"requestedAt"`
}

// ReviewResultDTO là kết quả sau khi duyệt/từ chối.
type ReviewResultDTO struct {
	ID         uuid.UUID             `json:"id"`
	BuildID    int64                 `json:"buildId,string" swaggertype:"string"`
	Decision   sqlc.ApprovalDecision `json:"decision" enums:"approve,reject"`
	Note       string                `json:"note"`
	ReviewedBy string                `json:"reviewedBy"`
	ReviewedAt time.Time             `json:"reviewedAt"`
}
