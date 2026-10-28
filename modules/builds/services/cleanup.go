package services

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/common"
)

// RunCleanup dọn định kỳ cho tới khi ctx bị huỷ:
//   - quick share hết hạn: xoá file + icon trên storage rồi xoá bản ghi
//   - session hết hạn
//
// Chạy ngay trong process server: chưa cần hạ tầng job riêng, và chạy trùng
// trên nhiều instance cũng vô hại (xoá cái đã xoá là no-op).
func (s *BuildService) RunCleanup(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		s.cleanupOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *BuildService) cleanupOnce(ctx context.Context) {
	if err := s.store.DeleteExpiredSessions(ctx); err != nil {
		log.Warn().Err(err).Msg("cleanup sessions failed")
	}

	rows, err := s.store.ListExpiredQuickShares(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("list expired quick shares failed")
		return
	}
	for _, r := range rows {
		// Xoá file trước, bản ghi sau: lỗi giữa chừng thì lần sau còn bản ghi
		// để thử lại, không để file mồ côi trên storage.
		if err := s.storage.Delete(ctx, r.FileKey); err != nil {
			log.Warn().Err(err).Str("key", r.FileKey).Msg("delete quick share file failed")
			continue
		}
		if r.IconKey.Valid {
			if err := s.storage.Delete(ctx, r.IconKey.String); err != nil {
				log.Warn().Err(err).Str("key", r.IconKey.String).Msg("delete quick share icon failed")
			}
		}
		if err := s.store.DeleteBuild(ctx, r.ID); err != nil {
			log.Warn().Err(err).Int64("build_id", r.ID).Msg("delete quick share failed")
		}
	}
	if len(rows) > 0 {
		log.Info().Int("count", len(rows)).Msg("cleaned expired quick shares")
	}
}

// wrapDB giữ nguyên lỗi nghiệp vụ (*AppError) trả từ trong transaction.
func wrapDB(err error) error {
	var appErr *common.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return common.ErrDB(err)
}
