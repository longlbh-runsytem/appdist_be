// Package dto chứa kiểu dữ liệu vào/ra của module downloads.
package dto

import (
	"time"

	"github.com/longlbh/appdist-be/db/sqlc"
)

// ReleaseDTO là dữ liệu trang tải công khai (GET /releases/:token) — cũng là
// response của POST /quick-share.
//
// InstallURL: iOS là itms-services://..., Android là link file .apk. Với App
// không public, các URL đã kèm vé (?t=) hạn 24 giờ.
type ReleaseDTO struct {
	Token            string        `json:"token"`
	AppName          string        `json:"appName"`
	IconURL          *string       `json:"iconUrl"`
	Version          string        `json:"version"`
	BuildNumber      string        `json:"buildNumber"`
	Platform         sqlc.Platform `json:"platform" enums:"ios,android"`
	SizeBytes        int64         `json:"sizeBytes"`
	Changelog        string        `json:"changelog"`
	UploadedAt       time.Time     `json:"uploadedAt"`
	RequiresPassword bool          `json:"requiresPassword"`
	InstallURL       string        `json:"installUrl"`
	ExpiresAt        *time.Time    `json:"expiresAt"`

	// Ngoài tài liệu FE: link trang tải bên FE (<FRONTEND_URL>/d/<token>) để
	// chia sẻ / hiện QR; kèm vé nếu App không public.
	PageURL string `json:"pageUrl"`
}
