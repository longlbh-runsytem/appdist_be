package services

import (
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/apps/dto"
)

// Các hàm dựng danh sách build dùng chung cho GET /tester/my-apps/:appId/builds
// (module testers) và GET /apps/:id/public — để hai API luôn cùng query
// params và cùng dạng response.

// BuildListOrder đổi query sang tham số sắp xếp của SQL. Mặc định build
// number giảm dần (bản mới nhất lên đầu).
func BuildListOrder(q dto.ListAppBuildsQuery) (sortBy string, asc bool) {
	sortBy = q.Sort
	if sortBy == "" {
		sortBy = "buildNumber"
	}
	return sortBy, q.Order == "asc"
}

// BuildListPlatform đổi bộ lọc platform sang tham số SQL (NULL = mọi platform).
func BuildListPlatform(q dto.ListAppBuildsQuery) sqlc.NullPlatform {
	if q.Platform == "" {
		return sqlc.NullPlatform{}
	}
	return sqlc.NullPlatform{Platform: sqlc.Platform(q.Platform), Valid: true}
}

// PlatformCounts gom kết quả "đếm theo platform" (mỗi query sqlc sinh một
// kiểu row riêng nên nhận hàm lấy giá trị).
func PlatformCounts[T any](rows []T, get func(T) (sqlc.Platform, int64)) dto.PlatformCountsDTO {
	var c dto.PlatformCountsDTO
	for _, r := range rows {
		p, n := get(r)
		switch p {
		case sqlc.PlatformIos:
			c.IOS = n
		case sqlc.PlatformAndroid:
			c.Android = n
		}
	}
	c.All = c.IOS + c.Android
	return c
}

// NewBuildPage dựng trang build; total lấy từ counts theo bộ lọc platform
// (không cần thêm query đếm, và đúng cả khi trang vượt quá trang cuối).
func NewBuildPage(builds []sqlc.Build, appID uuid.UUID, icon *string, q dto.ListAppBuildsQuery, counts dto.PlatformCountsDTO) dto.AppBuildPage {
	items := make([]dto.AppBuildDTO, 0, len(builds))
	for _, b := range builds {
		items = append(items, dto.AppBuildDTO{
			ID:          b.ID,
			AppID:       appID,
			Platform:    b.Platform,
			IconURL:     icon,
			Version:     b.Version,
			BuildNumber: b.BuildNumber,
			Status:      string(b.Status),
			SizeBytes:   b.FileSize,
			Changelog:   b.ReleaseNotes.String,
			UploadedAt:  b.CreatedAt,
			ShareToken:  b.Token,
		})
	}

	total := counts.All
	switch q.Platform {
	case string(sqlc.PlatformIos):
		total = counts.IOS
	case string(sqlc.PlatformAndroid):
		total = counts.Android
	}

	p := common.NewPage(items, q.PageQuery, total)
	return dto.AppBuildPage{
		Items:      p.Items,
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      p.Total,
		TotalPages: p.TotalPages,
		HasMore:    p.HasMore,
		Counts:     counts,
	}
}
