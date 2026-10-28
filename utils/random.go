package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// Bảng chữ bỏ 0/O/1/l/I dễ nhầm khi đọc hoặc gõ lại.
const ReadableAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// RandomString sinh chuỗi ngẫu nhiên an toàn mật mã từ alphabet.
//
// Dùng rejection sampling: bỏ byte rơi vào phần dư để mọi ký tự có xác suất
// đúng bằng nhau — chuỗi này còn dùng làm API key và token mời.
func RandomString(n int, alphabet string) (string, error) {
	limit := byte(256 - 256%len(alphabet))
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}

// SHA256Hex băm token trước khi lưu DB: lộ DB không lộ token dùng được.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
