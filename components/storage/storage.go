// Package storage là ranh giới duy nhất với object storage (Backblaze B2).
//
// Service chỉ biết interface Storage, không biết B2 hay S3: đổi nhà cung cấp
// (R2, S3...) chỉ cần viết thêm một implementation.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotFound = errors.New("object not found")

// ObjectInfo là metadata của một object đã nằm trên storage.
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	LastModified time.Time
}

// ReadAtCloser đọc ngẫu nhiên theo offset — mỗi lần ReadAt là một HTTP range
// request. Đủ để archive/zip đọc Info.plist trong IPA mà không tải cả file.
type ReadAtCloser interface {
	io.ReaderAt
	io.Closer
}

type Storage interface {
	// PresignPut tạo URL để client PUT file thẳng lên storage, không đi qua server.
	PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error)

	// PresignGet tạo URL tải tạm thời cho object private. downloadName rỗng =
	// giữ tên theo key.
	PresignGet(ctx context.Context, key string, expiry time.Duration, downloadName string) (string, error)

	// Stat trả ErrNotFound nếu object chưa tồn tại.
	Stat(ctx context.Context, key string) (ObjectInfo, error)

	// Open mở object để đọc theo offset. Người gọi phải Close.
	Open(ctx context.Context, key string) (ReadAtCloser, error)

	// Upload đẩy dữ liệu từ server lên storage. size = -1 nếu không biết trước
	// (khi đó thư viện tự chia multipart, tốn RAM hơn).
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error

	Delete(ctx context.Context, key string) error
}
