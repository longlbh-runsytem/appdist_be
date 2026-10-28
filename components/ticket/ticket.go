// Package ticket ký "vé" ngắn hạn cho link tải của App không public.
//
// iOS tải manifest.plist và file IPA bằng chính hệ điều hành, không mang theo
// token đăng nhập của web — nên quyền tải phải nằm ngay trong URL. Vé là
// HMAC(token build + hạn) — không cần lưu DB, hết hạn là tự vô hiệu.
package ticket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// QueryParam là tên tham số mang vé trên URL.
const QueryParam = "t"

type Signer struct {
	key []byte
}

func NewSigner(key string) *Signer {
	// Tách khoá ký vé khỏi khoá PASETO: cùng secret nhưng khác ngữ cảnh.
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("appdist/download-ticket"))
	return &Signer{key: mac.Sum(nil)}
}

// Sign cấp vé cho build token, hết hạn sau ttl.
func (s *Signer) Sign(buildToken string, ttl time.Duration) string {
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 36)
	return exp + "." + s.sig(buildToken, exp)
}

// Verify kiểm tra vé đúng của build này và chưa hết hạn.
func (s *Signer) Verify(buildToken, t string) bool {
	exp, sig, ok := strings.Cut(t, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 36, 64)
	if err != nil || time.Now().Unix() > unix {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(s.sig(buildToken, exp)))
}

func (s *Signer) sig(buildToken, exp string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(buildToken + "|" + exp))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}
