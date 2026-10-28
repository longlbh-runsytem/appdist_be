// Package db là ranh giới duy nhất giữa tầng nghiệp vụ và Postgres.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/longlbh/appdist-be/db/sqlc"
)

// Store gộp toàn bộ query do sqlc sinh (sqlc.Querier) với khả năng chạy
// nhiều query trong một transaction.
//
// Không có tầng repository viết tay: sqlc.Querier ĐÃ là interface repository,
// luôn đồng bộ với file .sql và không bao giờ lệch chữ ký. Thứ duy nhất sqlc
// không cho sẵn là ExecTx, nên đó là tất cả những gì file này bổ sung.
type Store interface {
	sqlc.Querier
	ExecTx(ctx context.Context, fn func(*sqlc.Queries) error) error
}

type sqlStore struct {
	*sqlc.Queries
	pool *pgxpool.Pool
}

// NewStore dựng Store từ connection pool.
func NewStore(pool *pgxpool.Pool) Store {
	return &sqlStore{
		Queries: sqlc.New(pool),
		pool:    pool,
	}
}

// ExecTx chạy fn trong một transaction: fn trả lỗi thì rollback, ngược lại commit.
//
// Dùng cho các thao tác phải nguyên tử, ví dụ thay profile (đánh dấu profile cũ
// hết hiệu lực + insert profile mới + chụp danh sách thiết bị).
func (s *sqlStore) ExecTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(s.Queries.WithTx(tx)); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx err: %v, rollback err: %w", err, rbErr)
		}
		return err
	}

	return tx.Commit(ctx)
}
