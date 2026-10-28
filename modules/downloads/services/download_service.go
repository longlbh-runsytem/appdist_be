// Package services chứa nghiệp vụ phía NGƯỜI TẢI: dữ liệu trang tải dạng
// JSON cho FE, manifest.plist cho iOS, và ghi nhận lượt tải.
//
// Trang tải HTML nằm bên FE tại <FRONTEND_URL>/d/:token; BE chỉ phục vụ các
// URL mà thiết bị gọi thẳng (manifest, file, icon, QR) dưới <PUBLIC_BASE_URL>/d/:token.
//
// App public: ai có link cũng tải được. App không public: link chỉ chạy khi
// kèm vé (?t=) — vé do GET /releases/:token cấp cho người có quyền (member
// trong scope hoặc tester được gán build).
package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/skip2/go-qrcode"
	"howett.net/plist"

	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/components/ticket"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/downloads/dto"
	"github.com/longlbh/appdist-be/utils"
)

const (
	// Link tải trên storage chỉ sống ngắn: đủ để iOS/Android bắt đầu tải, lộ
	// ra ngoài cũng nhanh chết. Tải file lớn mất lâu hơn cũng không sao — hạn
	// chỉ kiểm tra lúc BẮT ĐẦU request.
	fileURLExpiry = 15 * time.Minute
	iconURLExpiry = time.Hour
	ticketTTL     = 24 * time.Hour
)

var (
	// ErrNotFound: token không tồn tại, hoặc App không public mà không có quyền.
	ErrNotFound = errors.New("build not found")
	// ErrUnavailable: link đã bị thu hồi, hết hạn, hết lượt tải, hoặc build chưa được duyệt.
	ErrUnavailable = errors.New("build unavailable")
)

type DownloadService struct {
	store   db.Store
	storage storage.Storage
	tickets *ticket.Signer
	cfg     utils.Config
}

func NewDownloadService(store db.Store, st storage.Storage, tickets *ticket.Signer, cfg utils.Config) *DownloadService {
	return &DownloadService{store: store, storage: st, tickets: tickets, cfg: cfg}
}

// ClientInfo do handler suy ra từ request, dùng để ghi lượt tải.
type ClientInfo struct {
	IP        string
	UserAgent string
}

// release là build kèm cấu hình phân phối của App cha và vé đang dùng.
type release struct {
	sqlc.GetReleaseByTokenRow
	ticket string // rỗng = App public, link không cần vé
}

func (r release) build() sqlc.Build { return r.GetReleaseByTokenRow.Build }

// public: quick share (không có App) luôn public.
func (r release) public() bool { return !r.AppIsPublic.Valid || r.AppIsPublic.Bool }

func (r release) appName() string {
	if r.AppDisplayName.Valid {
		return r.AppDisplayName.String
	}
	return r.build().AppName
}

func (r release) iconKey() string {
	if r.AppIconKey.Valid {
		return r.AppIconKey.String
	}
	return r.build().IconKey.String
}

// expiresAt là hạn sớm nhất giữa hạn của build và hạn của App.
func (r release) expiresAt() *time.Time {
	var out *time.Time
	for _, t := range []pgtype.Timestamptz{r.build().ExpiresAt, r.AppExpiresAt} {
		if t.Valid && (out == nil || t.Time.Before(*out)) {
			v := t.Time
			out = &v
		}
	}
	return out
}

// Release trả dữ liệu trang tải dạng JSON cho FE.
//
// App không public: cần vé t hợp lệ (từ link chia sẻ, dùng lại nguyên vé
// nên không gia hạn được), hoặc viewer là owner, member có App trong scope,
// hay tester được gán build này — khi đó các URL trả về kèm vé mới. Không đủ
// quyền trả ErrNotFound (không để lộ App tồn tại).
func (s *DownloadService) Release(ctx context.Context, token, t string, viewer *sqlc.User) (*dto.ReleaseDTO, error) {
	r, err := s.load(ctx, token)
	if err != nil {
		return nil, err
	}
	if !r.public() && t != "" && s.tickets.Verify(token, t) {
		r.ticket = t
	} else if !r.public() {
		ok, err := s.canView(ctx, r, viewer)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotFound
		}
		r.ticket = s.tickets.Sign(token, ticketTTL)
	}
	if err := checkAvailable(r); err != nil {
		return nil, err
	}

	b := r.build()
	var icon *string
	if key := r.iconKey(); key != "" {
		if u, err := s.storage.PresignGet(ctx, key, iconURLExpiry, ""); err == nil {
			icon = &u
		}
	}

	return &dto.ReleaseDTO{
		Token:            b.Token,
		AppName:          r.appName(),
		IconURL:          icon,
		Version:          b.Version,
		BuildNumber:      b.BuildNumber,
		Platform:         b.Platform,
		SizeBytes:        b.FileSize,
		Changelog:        b.ReleaseNotes.String,
		UploadedAt:       b.CreatedAt,
		RequiresPassword: false,
		InstallURL:       s.installURL(r),
		ExpiresAt:        r.expiresAt(),
		PageURL:          s.pageURL(r),
	}, nil
}

// Manifest sinh manifest.plist mà iOS đọc khi bấm link itms-services://.
//
// Dùng thư viện plist thay vì ghép chuỗi: tên app có ký tự như "&" hay "<"
// sẽ làm hỏng XML nếu không escape.
func (s *DownloadService) Manifest(ctx context.Context, token, t string) ([]byte, error) {
	r, err := s.loadWithTicket(ctx, token, t)
	if err != nil {
		return nil, err
	}
	b := r.build()
	if b.Platform != sqlc.PlatformIos {
		return nil, ErrNotFound
	}

	assets := []map[string]string{
		{"kind": "software-package", "url": s.link(r, "/app.ipa")},
	}
	if r.iconKey() != "" {
		// Icon hiện trên màn hình chính trong lúc đang cài.
		icon := s.cfg.PublicBaseURL + "/d/" + b.Token + "/icon.png"
		assets = append(assets,
			map[string]string{"kind": "display-image", "url": icon},
			map[string]string{"kind": "full-size-image", "url": icon},
		)
	}

	manifest := map[string]any{
		"items": []map[string]any{{
			"assets": assets,
			"metadata": map[string]string{
				"bundle-identifier": b.BundleID,
				"bundle-version":    b.Version,
				"kind":              "software",
				"title":             r.appName(),
			},
		}},
	}
	return plist.MarshalIndent(manifest, plist.XMLFormat, "  ")
}

// FileURL trả link tải tạm trên storage và — nếu count — ghi một lượt tải.
func (s *DownloadService) FileURL(ctx context.Context, token, t string, client ClientInfo, count bool) (string, error) {
	r, err := s.loadWithTicket(ctx, token, t)
	if err != nil {
		return "", err
	}
	b := r.build()

	if count {
		err := s.store.RecordDownload(ctx, sqlc.RecordDownloadParams{
			BuildID:   b.ID,
			ClientIp:  pgtype.Text{String: client.IP, Valid: client.IP != ""},
			UserAgent: pgtype.Text{String: truncate(client.UserAgent, 512), Valid: client.UserAgent != ""},
		})
		if err != nil {
			return "", fmt.Errorf("record download: %w", err)
		}
	}

	return s.storage.PresignGet(ctx, b.FileKey, fileURLExpiry, fileName(b))
}

// IconURL trả link tạm tới icon (icon của App nếu có). Không kiểm tra quyền
// hay hạn: icon không nhạy cảm, và thiếu icon trên trang báo lỗi trông hỏng
// hơn là để lộ icon.
func (s *DownloadService) IconURL(ctx context.Context, token string) (string, error) {
	r, err := s.load(ctx, token)
	if err != nil {
		return "", err
	}
	key := r.iconKey()
	if key == "" {
		return "", ErrNotFound
	}
	return s.storage.PresignGet(ctx, key, iconURLExpiry, "")
}

// QRCode trả ảnh PNG mã QR của trang tải — để quét khi mở link trên máy tính.
func (s *DownloadService) QRCode(ctx context.Context, token, t string) ([]byte, error) {
	r, err := s.loadWithTicket(ctx, token, t)
	if err != nil {
		return nil, err
	}
	return qrcode.Encode(s.pageURL(r), qrcode.Medium, 512)
}

func (s *DownloadService) load(ctx context.Context, token string) (release, error) {
	row, err := s.store.GetReleaseByToken(ctx, token)
	if err != nil {
		if db.IsNotFound(err) {
			return release{}, ErrNotFound
		}
		return release{}, err
	}
	return release{GetReleaseByTokenRow: row}, nil
}

// loadWithTicket dùng cho các route /d/:token: App không public thì vé phải hợp lệ.
func (s *DownloadService) loadWithTicket(ctx context.Context, token, t string) (release, error) {
	r, err := s.load(ctx, token)
	if err != nil {
		return release{}, err
	}
	if !r.public() {
		if !s.tickets.Verify(token, t) {
			return release{}, ErrNotFound
		}
		r.ticket = t
	}
	if err := checkAvailable(r); err != nil {
		return release{}, err
	}
	return r, nil
}

// checkAvailable: build phải ready, còn hạn (của build và App), còn lượt tải.
func checkAvailable(r release) error {
	b := r.build()
	if b.Status != sqlc.BuildStatusReady {
		return ErrUnavailable
	}
	if exp := r.expiresAt(); exp != nil && time.Now().After(*exp) {
		return ErrUnavailable
	}
	if r.AppDownloadLimit.Valid && b.DownloadCount >= r.AppDownloadLimit.Int32 {
		return ErrUnavailable
	}
	return nil
}

func (s *DownloadService) canView(ctx context.Context, r release, viewer *sqlc.User) (bool, error) {
	if viewer == nil {
		return false, nil
	}
	b := r.build()
	switch viewer.Role {
	case sqlc.UserRoleOwner:
		return true, nil
	case sqlc.UserRoleAdmin, sqlc.UserRoleMember:
		ok, err := s.store.IsAppInScope(ctx, sqlc.IsAppInScopeParams{AppID: *b.AppID, UserID: viewer.ID})
		if err != nil || ok {
			return ok, err
		}
	}
	return s.store.CanTesterAccessBuild(ctx, sqlc.CanTesterAccessBuildParams{BuildID: b.ID, UserID: viewer.ID})
}

// installURL: iOS mở itms-services trỏ tới manifest; Android tải thẳng apk.
func (s *DownloadService) installURL(r release) string {
	if r.build().Platform == sqlc.PlatformIos {
		return "itms-services://?action=download-manifest&url=" + url.QueryEscape(s.link(r, "/manifest.plist"))
	}
	return s.link(r, "/app.apk")
}

// pageURL là link trang tải bên FE — link đưa cho khách và mã hoá trong QR.
func (s *DownloadService) pageURL(r release) string {
	return s.cfg.FrontendURL + "/d/" + r.build().Token + ticketQuery(r.ticket)
}

// link dựng URL tuyệt đối tới BE dưới /d/:token, kèm vé nếu có.
func (s *DownloadService) link(r release, suffix string) string {
	return s.cfg.PublicBaseURL + "/d/" + r.build().Token + suffix + ticketQuery(r.ticket)
}

func ticketQuery(t string) string {
	if t == "" {
		return ""
	}
	return "?" + ticket.QueryParam + "=" + url.QueryEscape(t)
}

// fileName là tên file khi tải về, vd "Pharmago-Store-2.3.0.apk".
func fileName(b sqlc.Build) string {
	ext := ".ipa"
	if b.Platform == sqlc.PlatformAndroid {
		ext = ".apk"
	}

	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		case r == ' ':
			return '-'
		default:
			return -1 // bỏ dấu tiếng Việt và ký tự đặc biệt khỏi tên file
		}
	}, b.AppName)
	if name == "" {
		name = "app"
	}
	return name + "-" + b.Version + ext
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Cắt theo byte có thể chẻ đôi một ký tự UTF-8 — Postgres sẽ từ chối chuỗi đó.
	return strings.ToValidUTF8(s[:n], "")
}
