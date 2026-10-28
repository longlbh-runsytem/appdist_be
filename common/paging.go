package common

import "github.com/google/uuid"

// PageQuery là query phân trang dùng chung: ?page=1&pageSize=20.
type PageQuery struct {
	Page     int32 `form:"page"     binding:"omitempty,min=1"`
	PageSize int32 `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// Normalize điền mặc định page=1, pageSize=20.
func (q *PageQuery) Normalize() {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 20
	}
}

func (q PageQuery) Limit() int32  { return q.PageSize }
func (q PageQuery) Offset() int32 { return (q.Page - 1) * q.PageSize }

// Page là shape phân trang dùng chung cho mọi endpoint list có phân trang.
//
// HasMore để FE hiện/ẩn nút "Tải thêm" mà không phải tự tính từ Total.
type Page[T any] struct {
	Items      []T   `json:"items"`
	Page       int32 `json:"page"`
	PageSize   int32 `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int32 `json:"totalPages"`
	HasMore    bool  `json:"hasMore"`
}

func NewPage[T any](items []T, q PageQuery, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}
	pages := int32((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return Page[T]{
		Items:      items,
		Page:       q.Page,
		PageSize:   q.PageSize,
		Total:      total,
		TotalPages: pages,
		HasMore:    q.Page < pages,
	}
}

// RefDTO là object rút gọn {id, name} dùng khi một tài nguyên tham chiếu tài
// nguyên khác (managingGroups, testerGroups...) — FE hiển thị được ngay, không
// phải gọi thêm API để ghép tên.
type RefDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}
