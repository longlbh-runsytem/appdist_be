package common

import "testing"

func TestNewPage(t *testing.T) {
	cases := []struct {
		name      string
		page      int32
		size      int32
		total     int64
		wantPages int32
		wantMore  bool
	}{
		{"rỗng", 1, 20, 0, 0, false},
		{"vừa một trang", 1, 20, 20, 1, false},
		{"còn trang sau", 1, 20, 21, 2, true},
		{"trang cuối", 2, 20, 21, 2, false},
		{"vượt quá trang cuối", 5, 20, 21, 2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := NewPage[int](nil, PageQuery{Page: c.page, PageSize: c.size}, c.total)
			if p.TotalPages != c.wantPages || p.HasMore != c.wantMore {
				t.Errorf("totalPages=%d hasMore=%v, want %d %v", p.TotalPages, p.HasMore, c.wantPages, c.wantMore)
			}
			if p.Items == nil {
				t.Error("items nil — phải là mảng rỗng để JSON ra [] thay vì null")
			}
		})
	}
}

func TestPageQueryNormalize(t *testing.T) {
	q := PageQuery{}
	q.Normalize()
	if q.Page != 1 || q.PageSize != 20 || q.Offset() != 0 {
		t.Errorf("got %+v offset=%d", q, q.Offset())
	}
	q = PageQuery{Page: 3, PageSize: 10}
	if q.Offset() != 20 {
		t.Errorf("offset = %d, want 20", q.Offset())
	}
}
