// Package services chứa số liệu thống kê, lọc theo scope App như module apps.
package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/stats/dto"
)

const maxRangeDays = 366

type StatsService struct {
	store db.Store
}

func NewStatsService(store db.Store) *StatsService {
	return &StatsService{store: store}
}

var vnTime = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*3600)
	}
	return loc
}()

var errBadRange = common.ErrBadRequest("khoảng ngày không hợp lệ (from ≤ to, tối đa 366 ngày)", "ErrInvalidDateRange")

func (s *StatsService) Summary(ctx context.Context, user sqlc.User) (*dto.SummaryDTO, error) {
	r, err := s.store.StatsSummary(ctx, scope(user))
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return &dto.SummaryDTO{
		TotalApps:      r.TotalApps,
		TotalBuilds:    r.TotalBuilds,
		TotalDownloads: r.TotalDownloads,
		TotalTesters:   r.TotalTesters,
	}, nil
}

// Downloads trả số lượt tải theo ngày, đủ mọi ngày trong khoảng (ngày không
// có lượt tải = 0) để FE vẽ biểu đồ không phải tự điền.
func (s *StatsService) Downloads(ctx context.Context, user sqlc.User, q dto.DownloadsQuery) ([]dto.DailyDownloadsDTO, error) {
	today := time.Now().In(vnTime)
	to := today
	if q.To != "" {
		to, _ = time.ParseInLocation(time.DateOnly, q.To, vnTime)
	}
	from := to.AddDate(0, 0, -6)
	if q.From != "" {
		from, _ = time.ParseInLocation(time.DateOnly, q.From, vnTime)
	}
	if from.After(to) || to.Sub(from) > maxRangeDays*24*time.Hour {
		return nil, errBadRange
	}

	rows, err := s.store.StatsDownloadsByDay(ctx, sqlc.StatsDownloadsByDayParams{
		FromDate:    pgtype.Date{Time: from, Valid: true},
		ToDate:      pgtype.Date{Time: to, Valid: true},
		ScopeUserID: scope(user),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	out := make([]dto.DailyDownloadsDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.DailyDownloadsDTO{Date: r.Date.Time.Format(time.DateOnly), Count: r.Count})
	}
	return out, nil
}

func scope(u sqlc.User) *uuid.UUID {
	if u.Role == sqlc.UserRoleOwner {
		return nil
	}
	return &u.ID
}
