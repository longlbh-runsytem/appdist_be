package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	appDTO "github.com/longlbh/appdist-be/modules/apps/dto"
	appServices "github.com/longlbh/appdist-be/modules/apps/services"
)

// MyAppsService là phía TESTER: App và build mình được mời xem.
//
// Tester thấy build khi: build status=ready, được gán cho một tester group
// có mình, và bản ghi tester của mình trong nhóm đó đang active.
type MyAppsService struct {
	store   db.Store
	storage storage.Storage
}

func NewMyAppsService(store db.Store, st storage.Storage) *MyAppsService {
	return &MyAppsService{store: store, storage: st}
}

var errMyAppNotFound = common.ErrNotFound("App")

func (s *MyAppsService) ListApps(ctx context.Context, user sqlc.User, q appDTO.ListMyAppsQuery) (*common.Page[appDTO.AppSummaryDTO], error) {
	q.Normalize()
	rows, err := s.store.ListTesterApps(ctx, sqlc.ListTesterAppsParams{
		UserID:     user.ID,
		Search:     db.Text(q.Search),
		FromTime:   db.Time(q.From),
		ToTime:     db.Time(q.To),
		PageLimit:  q.Limit(),
		PageOffset: q.Offset(),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	items := make([]appDTO.AppSummaryDTO, 0, len(rows))
	var total int64
	for _, r := range rows {
		total = r.Total
		items = append(items, appDTO.AppSummaryDTO{
			ID:        r.App.ID,
			Name:      r.App.Name,
			Platforms: appServices.PlatformsOf(r.App),
			IconURL:   appServices.IconURL(ctx, s.storage, r.App.IconKey.String),
		})
	}
	page := common.NewPage(items, q.PageQuery, total)
	return &page, nil
}

// ListBuilds trả trang build ready của App mà tester được mời, kèm số build
// theo từng platform. Tester không thấy build nào của App (ở mọi platform) =
// App ngoài phạm vi của tester → 404; lọc platform ra rỗng thì vẫn 200.
func (s *MyAppsService) ListBuilds(ctx context.Context, user sqlc.User, appID uuid.UUID, q appDTO.ListAppBuildsQuery) (*appDTO.AppBuildPage, error) {
	q.Normalize()

	countRows, err := s.store.CountTesterAppBuildsByPlatform(ctx, sqlc.CountTesterAppBuildsByPlatformParams{AppID: &appID, UserID: user.ID})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	counts := appServices.PlatformCounts(countRows, func(r sqlc.CountTesterAppBuildsByPlatformRow) (sqlc.Platform, int64) {
		return r.Platform, r.Total
	})
	if counts.All == 0 {
		return nil, errMyAppNotFound
	}

	sortBy, asc := appServices.BuildListOrder(q)
	builds, err := s.store.ListTesterAppBuilds(ctx, sqlc.ListTesterAppBuildsParams{
		AppID:      &appID,
		UserID:     user.ID,
		Platform:   appServices.BuildListPlatform(q),
		SortBy:     sortBy,
		SortAsc:    asc,
		PageLimit:  q.Limit(),
		PageOffset: q.Offset(),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	app, err := s.store.GetApp(ctx, appID)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	icon := appServices.IconURL(ctx, s.storage, app.IconKey.String)

	page := appServices.NewBuildPage(builds, appID, icon, q, counts)
	return &page, nil
}
