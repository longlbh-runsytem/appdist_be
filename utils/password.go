package utils

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword băm mật khẩu thô bằng bcrypt.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("cannot hash password: %w", err)
	}
	return string(hashed), nil
}

// CheckPassword so khớp mật khẩu thô với hash đã lưu.
// Trả nil nếu khớp.
func CheckPassword(password, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
