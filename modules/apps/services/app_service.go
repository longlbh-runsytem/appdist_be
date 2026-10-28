// Package services chứa nghiệp vụ App: cấu hình phân phối, nhóm quản trị,
// tester group mặc định, API key cho CI — và quy tắc scope dùng chung.
//
// Scope: owner thấy mọi App; role khác chỉ thấy App có ít nhất một nhóm quản
// trị đang active mà mình là thành viên. App ngoài scope trả 404 (không phải
// 403) để không lộ App tồn tại.
package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/apps/dto"
)

const iconURLExpiry = 24 * time.Hour

type AppService struct {
	store   db.Store
	storage storage.Storage
}

func NewAppService(store db.Store, st storage.Storage) *AppService {
	return &AppService{store: store, storage: st}
}

var (
	errAppNotFound         = common.ErrNotFound("App")
	errNoPlatform          = common.ErrBadRequest("App cần ít nhất một platform (ios hoặc android)", "ErrNoPlatform")
	errBundleIDTaken       = common.ErrConflict("bundle id đã thuộc về App khác", "ErrBundleIdTaken")
	errGroupInvalid        = common.ErrBadRequest("nhóm quản trị không tồn tại hoặc đã bị xoá", "ErrGroupNotFound")
	errGroupNotMine        = common.ErrForbidden("admin chỉ được gán nhóm quản trị mà mình là thành viên")
	errLastManagingGroup   = common.ErrBadRequest("App phải còn ít nhất một nhóm quản trị", "ErrLastManagingGroup")
	errTesterGroupNotFound = common.ErrNotFound("tester group")
)

// IsOwner: owner bỏ qua mọi kiểm tra scope.
func IsOwner(u sqlc.User) bool { return u.Role == sqlc.UserRoleOwner }

// ScopeUserID là tham số scope cho các query list: nil = không lọc (owner).
func ScopeUserID(u sqlc.User) *uuid.UUID {
	if IsOwner(u) {
		return nil
	}
	return &u.ID
}

// Access nạp App và kiểm tra user có quyền trên nó. Ngoài scope → 404.
// Module builds dùng hàm này qua interface của riêng nó.
func (s *AppService) Access(ctx context.Context, user sqlc.User, appID uuid.UUID) (sqlc.App, error) {
	app, err := s.store.GetApp(ctx, appID)
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.App{}, errAppNotFound
		}
		return sqlc.App{}, common.ErrDB(err)
	}
	if IsOwner(user) {
		return app, nil
	}

	ok, err := s.store.IsAppInScope(ctx, sqlc.IsAppInScopeParams{AppID: appID, UserID: user.ID})
	if err != nil {
		return sqlc.App{}, common.ErrDB(err)
	}
	if !ok {
		return sqlc.App{}, errAppNotFound
	}
	return app, nil
}

func (s *AppService) Create(ctx context.Context, actor sqlc.User, p dto.CreateAppPayload) (*dto.AppDTO, error) {
	ios, android, err := bundleIDs(p.Platforms)
	if err != nil {
		return nil, err
	}
	for _, gid := range p.ManagingGroupIDs {
		if err := s.checkManagingGroup(ctx, actor, gid); err != nil {
			return nil, err
		}
	}

	var app sqlc.App
	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		var err error
		app, err = q.CreateApp(ctx, sqlc.CreateAppParams{
			Name:            strings.TrimSpace(p.Name),
			IosBundleID:     ios,
			AndroidBundleID: android,
			CreatedBy:       actor.ID,
		})
		if err != nil {
			return err
		}
		for _, gid := range p.ManagingGroupIDs {
			if err := q.AddAppManagingGroup(ctx, sqlc.AddAppManagingGroupParams{AppID: app.ID, GroupID: gid}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) List(ctx context.Context, user sqlc.User, q dto.ListAppsQuery) (*common.Page[dto.AppDTO], error) {
	q.Normalize()

	params := sqlc.ListAppsParams{
		ScopeUserID: ScopeUserID(user),
		Search:      db.Text(q.Search),
		FromTime:    db.Time(q.From),
		ToTime:      db.Time(q.To),
		PageLimit:   q.Limit(),
		PageOffset:  q.Offset(),
	}
	if q.Platform != "" {
		params.Platform = sqlc.NullPlatform{Platform: sqlc.Platform(q.Platform), Valid: true}
	}

	rows, err := s.store.ListApps(ctx, params)
	if err != nil {
		return nil, common.ErrDB(err)
	}

	apps := make([]sqlc.App, 0, len(rows))
	var total int64
	for _, r := range rows {
		apps = append(apps, r.App)
		total = r.Total
	}
	items, err := s.toDTOs(ctx, apps)
	if err != nil {
		return nil, err
	}
	page := common.NewPage(items, q.PageQuery, total)
	return &page, nil
}

func (s *AppService) Get(ctx context.Context, user sqlc.User, id uuid.UUID) (*dto.AppDTO, error) {
	app, err := s.Access(ctx, user, id)
	if err != nil {
		return nil, err
	}
	return s.toDTO(ctx, app)
}

// GetPublic trả trang App công khai kèm danh sách build (lọc / sắp xếp /
// phân trang giống GET /tester/my-apps/:appId/builds). App private, hết hạn
// hoặc không tồn tại đều trả CÙNG một 404 — không để lộ App private có tồn tại hay không.
func (s *AppService) GetPublic(ctx context.Context, id uuid.UUID, q dto.ListAppBuildsQuery) (*dto.PublicAppDTO, error) {
	q.Normalize()

	app, err := s.store.GetApp(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errAppNotFound
		}
		return nil, common.ErrDB(err)
	}
	if !app.IsPublic || (app.ExpiresAt.Valid && time.Now().After(app.ExpiresAt.Time)) {
		return nil, errAppNotFound
	}

	countRows, err := s.store.CountPublicAppBuildsByPlatform(ctx, sqlc.CountPublicAppBuildsByPlatformParams{
		AppID:         &app.ID,
		DownloadLimit: app.DownloadLimit,
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	counts := PlatformCounts(countRows, func(r sqlc.CountPublicAppBuildsByPlatformRow) (sqlc.Platform, int64) {
		return r.Platform, r.Total
	})

	sortBy, asc := BuildListOrder(q)
	builds, err := s.store.ListPublicAppBuilds(ctx, sqlc.ListPublicAppBuildsParams{
		AppID:         &app.ID,
		DownloadLimit: app.DownloadLimit,
		Platform:      BuildListPlatform(q),
		SortBy:        sortBy,
		SortAsc:       asc,
		PageLimit:     q.Limit(),
		PageOffset:    q.Offset(),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	icon := IconURL(ctx, s.storage, app.IconKey.String)
	return &dto.PublicAppDTO{
		ID:        app.ID,
		Name:      app.Name,
		IconURL:   icon,
		Platforms: PlatformsOf(app),
		Builds:    NewBuildPage(builds, app.ID, icon, q, counts),
	}, nil
}

func (s *AppService) UpdateInfo(ctx context.Context, user sqlc.User, id uuid.UUID, p dto.UpdateAppInfoPayload) (*dto.AppDTO, error) {
	if _, err := s.Access(ctx, user, id); err != nil {
		return nil, err
	}
	ios, android, err := bundleIDs(p.Platforms)
	if err != nil {
		return nil, err
	}
	app, err := s.store.UpdateAppInfo(ctx, sqlc.UpdateAppInfoParams{
		ID:              id,
		Name:            strings.TrimSpace(p.Name),
		IosBundleID:     ios,
		AndroidBundleID: android,
	})
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) AddManagingGroup(ctx context.Context, user sqlc.User, id, groupID uuid.UUID) (*dto.AppDTO, error) {
	app, err := s.Access(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if err := s.checkManagingGroup(ctx, user, groupID); err != nil {
		return nil, err
	}
	if err := s.store.AddAppManagingGroup(ctx, sqlc.AddAppManagingGroupParams{AppID: id, GroupID: groupID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

// RemoveManagingGroup gỡ 1 nhóm. Không cho gỡ nhóm cuối: App mất hết nhóm thì
// chỉ owner còn thấy, dễ thành App "mồ côi".
func (s *AppService) RemoveManagingGroup(ctx context.Context, user sqlc.User, id, groupID uuid.UUID) (*dto.AppDTO, error) {
	app, err := s.Access(ctx, user, id)
	if err != nil {
		return nil, err
	}
	err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
		if err := q.RemoveAppManagingGroup(ctx, sqlc.RemoveAppManagingGroupParams{AppID: id, GroupID: groupID}); err != nil {
			return err
		}
		n, err := q.CountAppManagingGroups(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return errLastManagingGroup
		}
		return nil
	})
	if err != nil {
		return nil, mapWriteErr(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) UpdateVisibility(ctx context.Context, user sqlc.User, id uuid.UUID, p dto.VisibilityPayload) (*dto.AppDTO, error) {
	if _, err := s.Access(ctx, user, id); err != nil {
		return nil, err
	}
	app, err := s.store.UpdateAppVisibility(ctx, sqlc.UpdateAppVisibilityParams{
		ID:            id,
		IsPublic:      *p.IsPublic,
		ExpiresAt:     db.Time(p.ExpiresAt),
		DownloadLimit: db.Int4(p.DownloadLimit),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) UpdateApproval(ctx context.Context, user sqlc.User, id uuid.UUID, p dto.ApprovalPayload) (*dto.AppDTO, error) {
	if _, err := s.Access(ctx, user, id); err != nil {
		return nil, err
	}
	app, err := s.store.UpdateAppApproval(ctx, sqlc.UpdateAppApprovalParams{ID: id, RequiresApproval: *p.RequiresApproval})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) UpdateAutoAssign(ctx context.Context, user sqlc.User, id uuid.UUID, p dto.AutoAssignPayload) (*dto.AppDTO, error) {
	if _, err := s.Access(ctx, user, id); err != nil {
		return nil, err
	}
	app, err := s.store.UpdateAppAutoAssign(ctx, sqlc.UpdateAppAutoAssignParams{ID: id, AutoAssignTesterGroups: *p.AutoAssignTesterGroups})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) AddDefaultTesterGroup(ctx context.Context, user sqlc.User, id, tgID uuid.UUID) (*dto.AppDTO, error) {
	app, err := s.Access(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.GetTesterGroup(ctx, tgID); err != nil {
		if db.IsNotFound(err) {
			return nil, errTesterGroupNotFound
		}
		return nil, common.ErrDB(err)
	}
	if err := s.store.AddAppDefaultTesterGroup(ctx, sqlc.AddAppDefaultTesterGroupParams{AppID: id, TesterGroupID: tgID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

func (s *AppService) RemoveDefaultTesterGroup(ctx context.Context, user sqlc.User, id, tgID uuid.UUID) (*dto.AppDTO, error) {
	app, err := s.Access(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if err := s.store.RemoveAppDefaultTesterGroup(ctx, sqlc.RemoveAppDefaultTesterGroupParams{AppID: id, TesterGroupID: tgID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.toDTO(ctx, app)
}

// checkManagingGroup: nhóm phải còn tồn tại; admin chỉ gán được nhóm mình là
// thành viên — không thì admin tự tạo App rồi giao cho nhóm khác là mất scope.
func (s *AppService) checkManagingGroup(ctx context.Context, actor sqlc.User, groupID uuid.UUID) error {
	g, err := s.store.GetGroup(ctx, groupID)
	if err != nil {
		if db.IsNotFound(err) {
			return errGroupInvalid
		}
		return common.ErrDB(err)
	}
	if g.Status == sqlc.EntityStatusDeleted {
		return errGroupInvalid
	}
	if IsOwner(actor) {
		return nil
	}
	ok, err := s.store.IsGroupMember(ctx, sqlc.IsGroupMemberParams{GroupID: groupID, UserID: actor.ID})
	if err != nil {
		return common.ErrDB(err)
	}
	if !ok {
		return errGroupNotMine
	}
	return nil
}

func (s *AppService) toDTO(ctx context.Context, app sqlc.App) (*dto.AppDTO, error) {
	out, err := s.toDTOs(ctx, []sqlc.App{app})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

// toDTOs ghép nhóm quản trị và tester group mặc định cho nhiều App bằng 2 query.
func (s *AppService) toDTOs(ctx context.Context, apps []sqlc.App) ([]dto.AppDTO, error) {
	out := make([]dto.AppDTO, 0, len(apps))
	if len(apps) == 0 {
		return out, nil
	}

	ids := make([]uuid.UUID, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.ID)
	}

	managing := map[uuid.UUID][]common.RefDTO{}
	mRows, err := s.store.ListAppManagingGroups(ctx, ids)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	for _, r := range mRows {
		managing[r.AppID] = append(managing[r.AppID], common.RefDTO{ID: r.ID, Name: r.Name})
	}

	defaults := map[uuid.UUID][]common.RefDTO{}
	dRows, err := s.store.ListAppDefaultTesterGroups(ctx, ids)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	for _, r := range dRows {
		defaults[r.AppID] = append(defaults[r.AppID], common.RefDTO{ID: r.ID, Name: r.Name})
	}

	for _, a := range apps {
		out = append(out, dto.AppDTO{
			ID:                     a.ID,
			Name:                   a.Name,
			Platforms:              PlatformsOf(a),
			IconURL:                IconURL(ctx, s.storage, a.IconKey.String),
			IsPublic:               a.IsPublic,
			ManagingGroups:         nonNil(managing[a.ID]),
			CreatedAt:              a.CreatedAt,
			ExpiresAt:              db.TimePtr(a.ExpiresAt),
			DownloadLimit:          db.Int4Ptr(a.DownloadLimit),
			RequiresApproval:       a.RequiresApproval,
			AutoAssignTesterGroups: a.AutoAssignTesterGroups,
			DefaultTesterGroups:    nonNil(defaults[a.ID]),
		})
	}
	return out, nil
}

// PlatformsOf dựng {ios, android} từ 2 cột bundle id.
func PlatformsOf(a sqlc.App) dto.PlatformsDTO {
	var p dto.PlatformsDTO
	if a.IosBundleID.Valid {
		p.IOS = &dto.PlatformDTO{BundleID: a.IosBundleID.String}
	}
	if a.AndroidBundleID.Valid {
		p.Android = &dto.PlatformDTO{BundleID: a.AndroidBundleID.String}
	}
	return p
}

// IconURL ký link tạm tới icon; key rỗng hoặc ký lỗi → nil (FE hiện chữ cái đầu).
func IconURL(ctx context.Context, st storage.Storage, key string) *string {
	if key == "" {
		return nil
	}
	u, err := st.PresignGet(ctx, key, iconURLExpiry, "")
	if err != nil {
		return nil
	}
	return &u
}

func bundleIDs(p dto.PlatformsDTO) (ios, android pgtype.Text, err error) {
	if p.IOS != nil {
		ios = db.Text(p.IOS.BundleID)
	}
	if p.Android != nil {
		android = db.Text(p.Android.BundleID)
	}
	if !ios.Valid && !android.Valid {
		return ios, android, errNoPlatform
	}
	return ios, android, nil
}

func nonNil(refs []common.RefDTO) []common.RefDTO {
	if refs == nil {
		return []common.RefDTO{}
	}
	return refs
}

// mapWriteErr: trùng bundle id → 409; lỗi nghiệp vụ trả từ transaction giữ nguyên.
func mapWriteErr(err error) error {
	switch db.UniqueViolation(err) {
	case "uq_apps_ios_bundle_id", "uq_apps_android_bundle_id":
		return errBundleIDTaken
	}
	var appErr *common.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return common.ErrDB(err)
}
