// Package dto chứa kiểu dữ liệu vào/ra của module stats.
package dto

// SummaryDTO là số liệu tổng quan trong scope của người xem.
//
// totalDownloads = số lần tải file thật (bấm cài / tải apk), không tính mở
// trang tải. totalTesters đếm tester active toàn hệ thống (tester group dùng chung).
type SummaryDTO struct {
	TotalApps      int64 `json:"totalApps"`
	TotalBuilds    int64 `json:"totalBuilds"`
	TotalDownloads int64 `json:"totalDownloads"`
	TotalTesters   int64 `json:"totalTesters"`
}

// DailyDownloadsDTO là số lượt tải trong một ngày (giờ Việt Nam).
type DailyDownloadsDTO struct {
	Date  string `json:"date"  example:"2026-09-25"`
	Count int64  `json:"count" example:"120"`
}

// DownloadsQuery là query của GET /api/v1/stats/downloads (YYYY-MM-DD).
// Bỏ trống = 7 ngày gần nhất.
type DownloadsQuery struct {
	From string `form:"from" binding:"omitempty,datetime=2006-01-02" example:"2026-09-25"`
	To   string `form:"to"   binding:"omitempty,datetime=2006-01-02" example:"2026-10-02"`
}
