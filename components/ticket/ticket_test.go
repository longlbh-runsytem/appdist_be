package ticket

import (
	"testing"
	"time"
)

func TestTicket(t *testing.T) {
	s := NewSigner("12345678901234567890123456789012")

	v := s.Sign("AbC123", time.Hour)
	if !s.Verify("AbC123", v) {
		t.Fatal("vé hợp lệ bị từ chối")
	}
	if s.Verify("Other1", v) {
		t.Fatal("vé của build khác được chấp nhận")
	}
	if s.Verify("AbC123", s.Sign("AbC123", -time.Second)) {
		t.Fatal("vé hết hạn được chấp nhận")
	}
	if NewSigner("another-key-another-key-another-k").Verify("AbC123", v) {
		t.Fatal("vé ký bằng khoá khác được chấp nhận")
	}
	for _, bad := range []string{"", "abc", "zz.zz", v + "x"} {
		if s.Verify("AbC123", bad) {
			t.Fatalf("vé %q được chấp nhận", bad)
		}
	}
}
