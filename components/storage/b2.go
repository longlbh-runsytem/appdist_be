package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/longlbh/appdist-be/utils"
)

type b2Storage struct {
	client *minio.Client
	bucket string
}

// NewB2 kết nối Backblaze B2 qua S3-compatible API và kiểm tra bucket truy
// cập được ngay lúc khởi động — sai key/bucket thì báo sớm, không đợi tới
// lúc người dùng upload mới lỗi.
func NewB2(cfg utils.Config) (Storage, error) {
	if cfg.B2Endpoint == "" || cfg.B2Bucket == "" || cfg.B2KeyID == "" || cfg.B2ApplicationKey == "" {
		return nil, errors.New("thiếu cấu hình B2_ENDPOINT / B2_BUCKET / B2_KEY_ID / B2_APPLICATION_KEY")
	}

	// Endpoint mặc định là HTTPS. Tiền tố "http://" chỉ dành cho dev với
	// MinIO chạy local — B2 thật luôn là HTTPS.
	endpoint, secure := cfg.B2Endpoint, true
	if rest, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint, secure = rest, false
	}
	endpoint = strings.TrimPrefix(endpoint, "https://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.B2KeyID, cfg.B2ApplicationKey, ""),
		Secure: secure,
		Region: cfg.B2Region,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create B2 client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.B2Bucket)
	if err != nil {
		return nil, fmt.Errorf("cannot reach B2 bucket %q: %w", cfg.B2Bucket, err)
	}
	if !exists {
		return nil, fmt.Errorf("B2 bucket %q không tồn tại hoặc key không có quyền", cfg.B2Bucket)
	}

	return &b2Storage{client: client, bucket: cfg.B2Bucket}, nil
}

func (s *b2Storage) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("presign put %q: %w", key, err)
	}
	return u.String(), nil
}

func (s *b2Storage) PresignGet(ctx context.Context, key string, expiry time.Duration, downloadName string) (string, error) {
	params := url.Values{}
	if downloadName != "" {
		params.Set("response-content-disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
	}

	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, params)
	if err != nil {
		return "", fmt.Errorf("presign get %q: %w", key, err)
	}
	return u.String(), nil
}

func (s *b2Storage) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, fmt.Errorf("stat %q: %w", key, err)
	}

	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		LastModified: info.LastModified,
	}, nil
}

// Open trả *minio.Object — nó đã hiện thực ReadAt bằng HTTP range request.
func (s *b2Storage) Open(ctx context.Context, key string) (ReadAtCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open %q: %w", key, err)
	}
	return obj, nil
}

func (s *b2Storage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opts := minio.PutObjectOptions{ContentType: contentType}
	if size < 0 {
		// Không biết trước dung lượng (stream multipart): minio mặc định cấp
		// buffer theo cỡ file tối đa 5 TB/10000 part ≈ 500 MB mỗi upload. Đặt
		// part 16 MB — đủ cho file 160 GB mà RAM chỉ tốn 16 MB.
		opts.PartSize = 16 << 20
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, opts)
	if err != nil {
		return fmt.Errorf("upload %q: %w", key, err)
	}
	return nil
}

func (s *b2Storage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}
