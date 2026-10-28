// Package services chứa nghiệp vụ của module builds.
//
// File lên storage bằng một trong hai đường, rồi đi chung một pipeline
// (createFromObject): đọc thông tin từ file ĐÃ nằm trên storage (range
// request, chỉ tải vài chục KB), kiểm tra, trích icon, tạo bản ghi.
//
//  1. Web, 3 bước — file KHÔNG đi qua server:
//     InitUpload (presigned PUT URL) → client PUT thẳng lên B2 → Create.
//  2. CI / quick share — multipart qua server: Upload / QuickShare stream
//     part file thẳng lên storage, không buffer cả file trong RAM hay đĩa.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/appinfo"
	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/components/ticket"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/builds/dto"
	"github.com/longlbh/appdist-be/utils"
)

const (
	uploadURLExpiry  = time.Hour
	iconURLExpiry    = 24 * time.Hour
	ticketTTL        = 24 * time.Hour
	quickShareTTL    = 7 * 24 * time.Hour
	tokenLength      = 10
	maxFormValueSize = 4 << 10
)

var allowedExts = map[string]bool{".ipa": true, ".apk": true}

// AppAccess kiểm tra quyền của user trên App — hiện thực bởi
// apps/services.AppService, khai báo ở đây để module không phụ thuộc ngược.
type AppAccess interface {
	Access(ctx context.Context, user sqlc.User, appID uuid.UUID) (sqlc.App, error)
}

type BuildService struct {
	store   db.Store
	storage storage.Storage
	apps    AppAccess
	tickets *ticket.Signer
	cfg     utils.Config
}

func NewBuildService(store db.Store, st storage.Storage, apps AppAccess, tickets *ticket.Signer, cfg utils.Config) *BuildService {
	return &BuildService{store: store, storage: st, apps: apps, tickets: tickets, cfg: cfg}
}

var (
	errUnsupportedExt = common.NewCustomError(
		errors.New("unsupported file extension"),
		"chỉ hỗ trợ file .ipa hoặc .apk",
		"ErrUnsupportedFile",
	)

	errUploadKeyForbidden = common.NewFullErrorResponse(
		http.StatusForbidden,
		errors.New("upload key not owned"),
		"uploadKey không thuộc về tài khoản này",
		"upload key not owned",
		common.ErrNoPermissionCode,
	)

	errUploadNotFound = common.NewCustomError(
		errors.New("uploaded object not found"),
		"chưa thấy file trên storage — hãy PUT file lên uploadUrl trước",
		"ErrUploadNotFound",
	)

	errAlreadyCreated = common.ErrConflict("file này đã được tạo bản ghi", "ErrBuildAlreadyCreated")

	errBuildNotFound = common.ErrNotFound("build")

	errMissingFile  = common.ErrBadRequest("thiếu field file", "ErrMissingFile")
	errMissingAppID = common.ErrBadRequest("thiếu appId (bắt buộc khi dùng Bearer token)", "ErrMissingAppId")
	errBadPlatform  = common.ErrBadRequest("platform chỉ nhận ios hoặc android", "ErrInvalidPlatform")

	errTesterGroupNotFound = common.ErrNotFound("tester group")
)

func errFileTooLarge(max int64) error {
	return common.NewFullErrorResponse(
		http.StatusRequestEntityTooLarge,
		errors.New("file too large"),
		fmt.Sprintf("file vượt quá giới hạn %d MB", max>>20),
		"file too large",
		"ErrFileTooLarge",
	)
}

func errInvalidApp(err error) error {
	return common.NewCustomError(err, err.Error(), "ErrInvalidAppFile")
}

// InitUpload — bước 1 của luồng web.
//
// Object key gắn userID ("builds/<userID>/..."): bước 3 chỉ cần so tiền tố là
// biết key có đúng của người này không, không phải lưu phiên upload vào DB.
func (s *BuildService) InitUpload(ctx context.Context, user sqlc.User, p dto.InitUploadPayload) (*dto.InitUploadResponse, error) {
	ext := strings.ToLower(path.Ext(p.FileName))
	if !allowedExts[ext] {
		return nil, errUnsupportedExt
	}
	if p.FileSize > s.cfg.MaxUploadSize() {
		return nil, errFileTooLarge(s.cfg.MaxUploadSize())
	}

	key := s.newObjectKey(&user, ext)
	url, err := s.storage.PresignPut(ctx, key, uploadURLExpiry)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	return &dto.InitUploadResponse{
		UploadKey: key,
		UploadURL: url,
		Method:    http.MethodPut,
		ExpiresAt: time.Now().Add(uploadURLExpiry),
	}, nil
}

// UploadContent — đường DỰ PHÒNG cho bước 2: nhận file qua server rồi đẩy lên
// storage. Chỉ dùng khi trình duyệt không PUT thẳng lên B2 được (bucket chưa
// bật CORS); client thường (curl, CI) luôn PUT thẳng vào uploadUrl.
func (s *BuildService) UploadContent(ctx context.Context, user sqlc.User, key string, body io.Reader, size int64) error {
	if !s.ownsUploadKey(user, key) {
		return errUploadKeyForbidden
	}
	if !allowedExts[strings.ToLower(path.Ext(key))] {
		return errUnsupportedExt
	}
	if size <= 0 {
		return common.NewFullErrorResponse(
			http.StatusLengthRequired,
			errors.New("content length required"),
			"thiếu Content-Length",
			"content length required",
			"ErrLengthRequired",
		)
	}
	if size > s.cfg.MaxUploadSize() {
		return errFileTooLarge(s.cfg.MaxUploadSize())
	}

	// LimitReader: client khai Content-Length nhỏ rồi gửi nhiều hơn cũng
	// không đẩy quá size lên storage.
	if err := s.storage.Upload(ctx, key, io.LimitReader(body, size), size, "application/octet-stream"); err != nil {
		return common.ErrInternal(err)
	}
	return nil
}

// Create — bước 3 của luồng web: đọc thông tin từ file và tạo build cho App.
func (s *BuildService) Create(ctx context.Context, user sqlc.User, p dto.CreateBuildPayload) (*dto.BuildDTO, error) {
	if !s.ownsUploadKey(user, p.UploadKey) {
		return nil, errUploadKeyForbidden
	}
	app, err := s.apps.Access(ctx, user, p.AppID)
	if err != nil {
		return nil, err
	}
	return s.createFromObject(ctx, &user, &app, p.UploadKey, p.Changelog, "")
}

// Upload — POST /builds/upload (multipart) cho CI hoặc web.
//
// apiKeyApp khác nil = xác thực bằng API key: App lấy từ key, field appId
// (nếu có) bị bỏ qua. Field version/buildNumber client gửi luôn bị bỏ qua —
// thông tin app chỉ đọc từ file. platform nếu gửi thì phải khớp file.
func (s *BuildService) Upload(ctx context.Context, user sqlc.User, apiKeyApp *uuid.UUID, mr *multipart.Reader) (*dto.BuildDTO, error) {
	form, err := s.receiveMultipart(ctx, &user, mr)
	if err != nil {
		return nil, err
	}

	app, err := s.resolveUploadApp(ctx, user, apiKeyApp, form.values["appId"])
	if err != nil {
		s.discard(form.key)
		return nil, err
	}
	return s.createFromObject(ctx, &user, &app, form.key, form.values["changelog"], form.values["platform"])
}

// QuickShare — POST /quick-share: chia sẻ nhanh không gắn App, hết hạn sau 7 ngày.
// Không cần đăng nhập: uploader nil = ẩn danh. Trả build token để module
// downloads dựng response giống GET /releases/:token.
func (s *BuildService) QuickShare(ctx context.Context, uploader *sqlc.User, mr *multipart.Reader) (string, error) {
	form, err := s.receiveMultipart(ctx, uploader, mr)
	if err != nil {
		return "", err
	}
	b, err := s.createFromObject(ctx, uploader, nil, form.key, form.values["changelog"], "")
	if err != nil {
		return "", err
	}
	return b.ShareToken, nil
}

// List trả build của các App trong scope, mới nhất trước.
func (s *BuildService) List(ctx context.Context, user sqlc.User, q dto.ListBuildsQuery) (*common.Page[dto.BuildDTO], error) {
	q.Normalize()

	var appID *uuid.UUID
	if q.AppID != "" {
		id := uuid.MustParse(q.AppID) // binding đã kiểm tra định dạng uuid
		if _, err := s.apps.Access(ctx, user, id); err != nil {
			return nil, err
		}
		appID = &id
	}

	var scope *uuid.UUID
	if user.Role != sqlc.UserRoleOwner {
		scope = &user.ID
	}
	var status sqlc.NullBuildStatus
	if q.Status != "" {
		status = sqlc.NullBuildStatus{BuildStatus: sqlc.BuildStatus(q.Status), Valid: true}
	}
	var platform sqlc.NullPlatform
	if q.Platform != "" {
		platform = sqlc.NullPlatform{Platform: sqlc.Platform(q.Platform), Valid: true}
	}
	sortBy := q.Sort
	if sortBy == "" {
		sortBy = "uploadedAt"
	}

	rows, err := s.store.ListBuilds(ctx, sqlc.ListBuildsParams{
		AppID:       appID,
		Status:      status,
		Platform:    platform,
		FromTime:    db.Time(q.From),
		ToTime:      db.Time(q.To),
		Search:      db.Text(q.Search),
		ScopeUserID: scope,
		SortBy:      sortBy,
		SortAsc:     q.Order == "asc",
		PageLimit:   q.Limit(),
		PageOffset:  q.Offset(),
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}
	// Đếm riêng để total vẫn đúng khi trang vượt quá trang cuối.
	total, err := s.store.CountBuilds(ctx, sqlc.CountBuildsParams{
		AppID:       appID,
		Status:      status,
		Platform:    platform,
		FromTime:    db.Time(q.From),
		ToTime:      db.Time(q.To),
		Search:      db.Text(q.Search),
		ScopeUserID: scope,
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	views := make([]buildView, 0, len(rows))
	for _, r := range rows {
		views = append(views, buildView{r.Build, r.UploaderEmail.String, r.AppIconKey.String, r.AppIsPublic})
	}
	items, err := s.toDTOs(ctx, views)
	if err != nil {
		return nil, err
	}
	page := common.NewPage(items, q.PageQuery, total)
	return &page, nil
}

// Archive ngừng phân phối build: link tải trả 410.
func (s *BuildService) Archive(ctx context.Context, user sqlc.User, id int64) (*dto.BuildDTO, error) {
	if _, err := s.loadInScope(ctx, user, id); err != nil {
		return nil, err
	}
	if err := s.store.UpdateBuildStatus(ctx, sqlc.UpdateBuildStatusParams{ID: id, Status: sqlc.BuildStatusArchived}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.Get(ctx, user, id)
}

func (s *BuildService) AddTesterGroup(ctx context.Context, user sqlc.User, id int64, tgID uuid.UUID) (*dto.BuildDTO, error) {
	if _, err := s.loadInScope(ctx, user, id); err != nil {
		return nil, err
	}
	if _, err := s.store.GetTesterGroup(ctx, tgID); err != nil {
		if db.IsNotFound(err) {
			return nil, errTesterGroupNotFound
		}
		return nil, common.ErrDB(err)
	}
	if err := s.store.AddBuildTesterGroup(ctx, sqlc.AddBuildTesterGroupParams{BuildID: id, TesterGroupID: tgID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.Get(ctx, user, id)
}

func (s *BuildService) RemoveTesterGroup(ctx context.Context, user sqlc.User, id int64, tgID uuid.UUID) (*dto.BuildDTO, error) {
	if _, err := s.loadInScope(ctx, user, id); err != nil {
		return nil, err
	}
	if err := s.store.RemoveBuildTesterGroup(ctx, sqlc.RemoveBuildTesterGroupParams{BuildID: id, TesterGroupID: tgID}); err != nil {
		return nil, common.ErrDB(err)
	}
	return s.Get(ctx, user, id)
}

// Get trả một build trong scope.
func (s *BuildService) Get(ctx context.Context, user sqlc.User, id int64) (*dto.BuildDTO, error) {
	row, err := s.loadInScope(ctx, user, id)
	if err != nil {
		return nil, err
	}
	out, err := s.toDTOs(ctx, []buildView{{row.Build, row.UploaderEmail.String, row.AppIconKey.String, row.AppIsPublic}})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

// loadInScope nạp build của App; quick share hoặc App ngoài scope → 404.
func (s *BuildService) loadInScope(ctx context.Context, user sqlc.User, id int64) (sqlc.GetBuildDetailRow, error) {
	row, err := s.store.GetBuildDetail(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.GetBuildDetailRow{}, errBuildNotFound
		}
		return sqlc.GetBuildDetailRow{}, common.ErrDB(err)
	}
	if row.Build.AppID == nil {
		return sqlc.GetBuildDetailRow{}, errBuildNotFound
	}
	if _, err := s.apps.Access(ctx, user, *row.Build.AppID); err != nil {
		return sqlc.GetBuildDetailRow{}, errBuildNotFound
	}
	return row, nil
}

// createFromObject là pipeline chung sau khi file đã nằm trên storage. File
// không hợp lệ bị xoá khỏi storage để không thành rác.
//
// app nil = quick share. uploader nil = quick share ẩn danh (build của App
// luôn có uploader — DB cũng chặn bằng CHECK).
func (s *BuildService) createFromObject(ctx context.Context, uploader *sqlc.User, app *sqlc.App, key, changelog, platformHint string) (*dto.BuildDTO, error) {
	obj, err := s.storage.Stat(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, errUploadNotFound
		}
		return nil, common.ErrInternal(err)
	}

	// Presigned PUT không giới hạn được dung lượng, nên kiểm tra lại ở đây.
	if obj.Size > s.cfg.MaxUploadSize() {
		s.discard(key)
		return nil, errFileTooLarge(s.cfg.MaxUploadSize())
	}

	info, err := s.readAppInfo(ctx, key, obj.Size)
	if err != nil {
		if errors.Is(err, appinfo.ErrUnsupported) {
			s.discard(key)
			return nil, errInvalidApp(err)
		}
		return nil, common.ErrInternal(err)
	}

	warnings, err := checkInstallable(info)
	if err == nil {
		err = checkMatches(info, app, platformHint)
	}
	if err != nil {
		s.discard(key)
		return nil, err
	}

	metadata, err := json.Marshal(info.Extra)
	if err != nil {
		return nil, common.ErrInternal(err)
	}

	params := sqlc.CreateBuildParams{
		Source:       sqlc.BuildSourceQuickShare,
		Status:       sqlc.BuildStatusReady,
		Platform:     sqlc.Platform(info.Platform),
		AppName:      info.AppName,
		BundleID:     info.BundleID,
		Version:      info.Version,
		BuildNumber:  info.BuildNumber,
		MinOsVersion: db.Text(info.MinOSVersion),
		ReleaseNotes: db.Text(changelog),
		FileKey:      key,
		FileSize:     obj.Size,
		IconKey:      db.Text(s.uploadIcon(ctx, uploader, info)),
		Metadata:     metadata,
	}
	if uploader != nil {
		params.UploadedBy = &uploader.ID
	}
	if app != nil {
		params.Source = sqlc.BuildSourceApp
		params.AppID = &app.ID
		if app.RequiresApproval {
			params.Status = sqlc.BuildStatusPendingApproval
		}
	} else {
		params.ExpiresAt = db.Time(new(time.Now().Add(quickShareTTL)))
	}

	build, err := s.insertBuild(ctx, params, app)
	if err != nil {
		return nil, err
	}

	view := buildView{build: build}
	if uploader != nil {
		view.uploaderEmail = uploader.Email
	}
	if app != nil {
		view.appIconKey = build.IconKey.String
		if !build.IconKey.Valid {
			view.appIconKey = app.IconKey.String
		}
		view.appIsPublic.Bool, view.appIsPublic.Valid = app.IsPublic, true
	}
	out, err := s.toDTOs(ctx, []buildView{view})
	if err != nil {
		return nil, err
	}
	out[0].Warnings = warnings
	return &out[0], nil
}

// insertBuild tạo build và các side effect trong CÙNG transaction:
//   - autoAssignTesterGroups → kế thừa tester group mặc định của App
//   - requiresApproval       → tạo yêu cầu duyệt
//   - icon mới               → thành icon của App
//
// Token sinh ngẫu nhiên, trùng thì làm lại cả transaction (xác suất cực nhỏ
// với 10 ký tự trên bảng 56 chữ, nhưng rẻ để xử lý đúng).
func (s *BuildService) insertBuild(ctx context.Context, params sqlc.CreateBuildParams, app *sqlc.App) (sqlc.Build, error) {
	for range 3 {
		token, err := utils.RandomString(tokenLength, utils.ReadableAlphabet)
		if err != nil {
			return sqlc.Build{}, common.ErrInternal(err)
		}
		params.Token = token

		var build sqlc.Build
		err = s.store.ExecTx(ctx, func(q *sqlc.Queries) error {
			var err error
			build, err = q.CreateBuild(ctx, params)
			if err != nil || app == nil {
				return err
			}
			if app.AutoAssignTesterGroups {
				if err := q.CopyDefaultTesterGroups(ctx, sqlc.CopyDefaultTesterGroupsParams{BuildID: build.ID, AppID: app.ID}); err != nil {
					return err
				}
			}
			if app.RequiresApproval {
				if err := q.CreateApprovalRequest(ctx, sqlc.CreateApprovalRequestParams{BuildID: build.ID, RequestedBy: *params.UploadedBy}); err != nil {
					return err
				}
			}
			if params.IconKey.Valid {
				return q.UpdateAppIcon(ctx, sqlc.UpdateAppIconParams{ID: app.ID, IconKey: params.IconKey})
			}
			return nil
		})
		if err == nil {
			return build, nil
		}

		switch db.UniqueViolation(err) {
		case "builds_token_key":
			continue
		case "builds_file_key_key":
			return sqlc.Build{}, errAlreadyCreated
		}
		return sqlc.Build{}, common.ErrDB(err)
	}
	return sqlc.Build{}, common.ErrInternal(errors.New("cannot generate unique token"))
}

// checkMatches: file phải đúng platform và bundle id đã cấu hình cho App —
// chặn upload nhầm file của app khác.
func checkMatches(info *appinfo.Info, app *sqlc.App, platformHint string) error {
	if platformHint != "" && platformHint != string(info.Platform) {
		return common.ErrBadRequest(
			fmt.Sprintf("platform=%s nhưng file là %s", platformHint, info.Platform),
			"ErrPlatformMismatch",
		)
	}
	if app == nil {
		return nil
	}

	want := app.AndroidBundleID
	if info.Platform == appinfo.PlatformIOS {
		want = app.IosBundleID
	}
	if !want.Valid {
		return common.ErrBadRequest(
			fmt.Sprintf("App %q chưa cấu hình platform %s", app.Name, info.Platform),
			"ErrPlatformNotConfigured",
		)
	}
	if want.String != info.BundleID {
		return common.ErrBadRequest(
			fmt.Sprintf("bundle id trong file (%s) không khớp App (%s)", info.BundleID, want.String),
			"ErrBundleIdMismatch",
		)
	}
	return nil
}

// checkInstallable chặn file chắc chắn không cài được và cảnh báo các trường
// hợp cài được nhưng có giới hạn.
func checkInstallable(info *appinfo.Info) ([]string, error) {
	if info.Platform != appinfo.PlatformIOS {
		return nil, nil
	}

	var warnings []string

	switch info.Extra["distribution"] {
	case appinfo.DistributionUnsigned:
		return nil, errInvalidApp(errors.New("IPA chưa được ký (không có embedded.mobileprovision) — hãy archive và export bằng profile in-house"))
	case appinfo.DistributionAppStore:
		return nil, errInvalidApp(errors.New("IPA được ký cho App Store, không cài trực tiếp được — hãy export bằng profile in-house hoặc ad-hoc"))
	case appinfo.DistributionAdHoc, appinfo.DistributionDevelopment:
		warnings = append(warnings, fmt.Sprintf(
			"IPA ký %s: chỉ cài được trên %v máy đã đăng ký UDID trong profile",
			info.Extra["distribution"], info.Extra["provisioned_device_count"],
		))
	case appinfo.DistributionUnknown:
		warnings = append(warnings, "không đọc được provisioning profile — có thể không cài được")
	}

	if raw, ok := info.Extra["profile_expires_at"].(string); ok {
		if exp, err := time.Parse(time.RFC3339, raw); err == nil {
			if time.Now().After(exp) {
				return nil, errInvalidApp(fmt.Errorf("provisioning profile đã hết hạn từ %s", exp.Format("02/01/2006")))
			}
			if time.Until(exp) < 30*24*time.Hour {
				warnings = append(warnings, fmt.Sprintf("provisioning profile hết hạn ngày %s — app sẽ không mở được sau ngày này", exp.Format("02/01/2006")))
			}
		}
	}

	return warnings, nil
}

// uploadForm là kết quả đọc multipart: file đã nằm trên storage ở key.
type uploadForm struct {
	key    string
	values map[string]string
}

// receiveMultipart stream part "file" thẳng lên storage và gom các field chữ.
//
// Thứ tự part tuỳ client (curl -F file=@... -F appId=... gửi file trước) nên
// không chờ đủ field mới upload; caller kiểm tra field sau và xoá file nếu sai.
func (s *BuildService) receiveMultipart(ctx context.Context, uploader *sqlc.User, mr *multipart.Reader) (uploadForm, error) {
	form := uploadForm{values: map[string]string{}}
	fail := func(err error) (uploadForm, error) {
		if form.key != "" {
			s.discard(form.key)
		}
		return uploadForm{}, err
	}

	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(common.ErrInvalidRequest(err))
		}

		name := part.FormName()
		if name != "file" {
			v, err := io.ReadAll(io.LimitReader(part, maxFormValueSize))
			if err != nil {
				return fail(common.ErrInvalidRequest(err))
			}
			form.values[name] = strings.TrimSpace(string(v))
			continue
		}

		if form.key != "" {
			return fail(common.ErrBadRequest("chỉ gửi một file mỗi lần", "ErrMultipleFiles"))
		}
		ext := strings.ToLower(path.Ext(part.FileName()))
		if !allowedExts[ext] {
			return fail(errUnsupportedExt)
		}

		form.key = s.newObjectKey(uploader, ext)
		// Đọc tối đa max+1 byte: vượt giới hạn thì createFromObject thấy
		// size > max và xoá, không đẩy vô hạn lên storage.
		body := io.LimitReader(part, s.cfg.MaxUploadSize()+1)
		if err := s.storage.Upload(ctx, form.key, body, -1, "application/octet-stream"); err != nil {
			return fail(common.ErrInternal(err))
		}
	}

	if form.key == "" {
		return uploadForm{}, errMissingFile
	}
	if p := form.values["platform"]; p != "" && p != string(sqlc.PlatformIos) && p != string(sqlc.PlatformAndroid) {
		return fail(errBadPlatform)
	}
	return form, nil
}

func (s *BuildService) resolveUploadApp(ctx context.Context, user sqlc.User, apiKeyApp *uuid.UUID, rawAppID string) (sqlc.App, error) {
	// API key đã gắn sẵn App: không cần (và không xét) scope của người tạo key.
	if apiKeyApp != nil {
		app, err := s.store.GetApp(ctx, *apiKeyApp)
		if err != nil {
			return sqlc.App{}, common.ErrDB(err)
		}
		return app, nil
	}

	if rawAppID == "" {
		return sqlc.App{}, errMissingAppID
	}
	appID, err := uuid.Parse(rawAppID)
	if err != nil {
		return sqlc.App{}, common.ErrNotFound("App")
	}
	return s.apps.Access(ctx, user, appID)
}

func (s *BuildService) newObjectKey(uploader *sqlc.User, ext string) string {
	return fmt.Sprintf("builds/%s/%s%s", ownerDir(uploader), uuid.NewString(), ext)
}

// ownerDir là thư mục theo người upload trên storage; quick share ẩn danh
// gom vào "anonymous". ownsUploadKey dựa vào thư mục này để biết key của ai.
func ownerDir(uploader *sqlc.User) string {
	if uploader == nil {
		return "anonymous"
	}
	return uploader.ID.String()
}

func (s *BuildService) ownsUploadKey(user sqlc.User, key string) bool {
	prefix := fmt.Sprintf("builds/%s/", user.ID)
	return strings.HasPrefix(key, prefix) && !strings.Contains(key, "..") && path.Clean(key) == key
}

func (s *BuildService) readAppInfo(ctx context.Context, key string, size int64) (*appinfo.Info, error) {
	r, err := s.storage.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return appinfo.Parse(r, size)
}

// uploadIcon lưu icon dạng PNG chuẩn. Lỗi chỉ ghi log: thiếu icon không đáng
// để bắt người dùng upload lại cả file.
func (s *BuildService) uploadIcon(ctx context.Context, uploader *sqlc.User, info *appinfo.Info) string {
	if info.Icon == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, info.Icon); err != nil {
		log.Warn().Err(err).Str("bundle_id", info.BundleID).Msg("encode icon failed")
		return ""
	}

	key := fmt.Sprintf("icons/%s/%s.png", ownerDir(uploader), uuid.NewString())
	if err := s.storage.Upload(ctx, key, &buf, int64(buf.Len()), "image/png"); err != nil {
		log.Warn().Err(err).Str("bundle_id", info.BundleID).Msg("upload icon failed")
		return ""
	}
	return key
}

// discard xoá file không hợp lệ khỏi storage. Chạy nền với context riêng:
// request có thể đã bị huỷ nhưng file rác vẫn phải dọn.
func (s *BuildService) discard(key string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.storage.Delete(ctx, key); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("discard upload failed")
		}
	}()
}
