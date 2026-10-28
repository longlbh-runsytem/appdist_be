package services

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/appinfo"
	"github.com/longlbh/appdist-be/db/sqlc"
)

func TestCheckMatches(t *testing.T) {
	app := &sqlc.App{
		Name:        "GMO Delivery",
		IosBundleID: pgtype.Text{String: "com.gmo.ios", Valid: true},
	}
	ios := &appinfo.Info{Platform: appinfo.PlatformIOS, BundleID: "com.gmo.ios"}
	android := &appinfo.Info{Platform: appinfo.PlatformAndroid, BundleID: "com.gmo.android"}

	cases := []struct {
		name    string
		info    *appinfo.Info
		app     *sqlc.App
		hint    string
		wantKey string // rỗng = hợp lệ
	}{
		{"đúng platform và bundle id", ios, app, "", ""},
		{"platform khai khớp file", ios, app, "ios", ""},
		{"platform khai sai", ios, app, "android", "ErrPlatformMismatch"},
		{"App chưa cấu hình platform của file", android, app, "", "ErrPlatformNotConfigured"},
		{"bundle id khác", &appinfo.Info{Platform: appinfo.PlatformIOS, BundleID: "com.other"}, app, "", "ErrBundleIdMismatch"},
		{"quick share không kiểm bundle id", android, nil, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkMatches(c.info, c.app, c.hint)
			if c.wantKey == "" {
				if err != nil {
					t.Fatalf("lỗi không mong đợi: %v", err)
				}
				return
			}
			var appErr *common.AppError
			if !errors.As(err, &appErr) || appErr.Key != c.wantKey {
				t.Fatalf("err = %v, muốn key %s", err, c.wantKey)
			}
		})
	}
}

func TestOwnsUploadKey(t *testing.T) {
	s := &BuildService{}
	user := sqlc.User{}
	user.ID[0] = 1
	own := s.newObjectKey(&user, ".ipa")

	if !s.ownsUploadKey(user, own) {
		t.Fatal("key của chính mình bị từ chối")
	}
	other := sqlc.User{}
	other.ID[0] = 2
	for _, k := range []string{s.newObjectKey(&other, ".ipa"), s.newObjectKey(nil, ".ipa"), own + "/../x.ipa", "icons/" + own} {
		if s.ownsUploadKey(user, k) {
			t.Fatalf("chấp nhận key không thuộc user: %s", k)
		}
	}
}
