package utils

import (
	"strings"
	"testing"
)

func TestRandomString(t *testing.T) {
	s, err := RandomString(64, ReadableAlphabet)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 64 {
		t.Fatalf("len = %d", len(s))
	}
	for _, r := range s {
		if !strings.ContainsRune(ReadableAlphabet, r) {
			t.Fatalf("ký tự %q ngoài alphabet", r)
		}
	}
	other, _ := RandomString(64, ReadableAlphabet)
	if s == other {
		t.Fatal("hai lần sinh trùng nhau")
	}
}
