package appinfo

import (
	"fmt"
	"io"
	"strconv"

	"github.com/shogo82148/androidbinary/apk"
)

// parseAPK đọc AndroidManifest.xml. File này là "binary XML" của Android,
// tên app và icon lại nằm trong resources.arsc — thư viện androidbinary lo
// cả hai.
func parseAPK(r io.ReaderAt, size int64) (*Info, error) {
	pkg, err := apk.OpenZipReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: đọc APK: %v", ErrUnsupported, err)
	}
	defer pkg.Close()

	m := pkg.Manifest()
	info := &Info{
		Platform: PlatformAndroid,
		BundleID: pkg.PackageName(),
		Extra:    map[string]any{},
	}

	if v, err := m.VersionName.String(); err == nil {
		info.Version = v
	}
	if code, err := m.VersionCode.Int32(); err == nil {
		info.BuildNumber = strconv.Itoa(int(code))
	}
	if minSDK, err := m.SDK.Min.Int32(); err == nil && minSDK > 0 {
		info.MinOSVersion = fmt.Sprintf("API %d", minSDK)
		info.Extra["min_sdk"] = minSDK
	}
	if target, err := m.SDK.Target.Int32(); err == nil && target > 0 {
		info.Extra["target_sdk"] = target
	}

	// nil = cấu hình resource mặc định. Label/icon lỗi (vd adaptive icon dạng
	// XML) không làm hỏng upload.
	if label, err := pkg.Label(nil); err == nil {
		info.AppName = label
	}
	if icon, err := pkg.Icon(nil); err == nil {
		info.Icon = icon
	}

	return info, nil
}
