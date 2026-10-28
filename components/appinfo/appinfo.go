// Package appinfo đọc thông tin app từ BÊN TRONG file IPA/APK.
//
// Cả IPA lẫn APK đều là file zip. Loại file được suy ra từ nội dung chứ không
// từ đuôi file: đổi tên .apk thành .ipa không lừa được parser.
//
// Parser chỉ cần io.ReaderAt nên đọc được thẳng từ B2 qua range request:
// zip đặt mục lục ở CUỐI file, nên để lấy Info.plist + icon chỉ phải tải vài
// chục KB thay vì cả file vài trăm MB.
package appinfo

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	"io"
)

type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Info là mọi thứ đọc được từ file, map thẳng vào các cột của bảng builds.
type Info struct {
	Platform     Platform
	AppName      string
	BundleID     string
	Version      string
	BuildNumber  string
	MinOSVersion string

	// Icon đã giải mã, nil nếu không trích được — thiếu icon không phải lỗi.
	Icon image.Image

	// Extra là thông tin phụ, lưu vào builds.metadata (jsonb).
	Extra map[string]any
}

var ErrUnsupported = errors.New("file không phải IPA hoặc APK hợp lệ")

// Giới hạn dung lượng GIẢI NÉN của từng file nhỏ đọc ra (plist, icon...),
// chặn zip bomb: một entry vài KB nén có thể bung ra vài GB.
const maxSmallFileSize = 10 << 20

// Parse nhận diện và đọc thông tin từ file IPA/APK.
func Parse(r io.ReaderAt, size int64) (*Info, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: không đọc được zip: %v", ErrUnsupported, err)
	}

	var info *Info
	switch {
	case findIPAInfoPlist(zr) != nil:
		info, err = parseIPA(zr)
	case findFile(zr, "AndroidManifest.xml") != nil:
		info, err = parseAPK(r, size)
	case findFile(zr, "BundleConfig.pb") != nil:
		return nil, fmt.Errorf("%w: file .aab không cài trực tiếp được, hãy build ra .apk", ErrUnsupported)
	default:
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, err
	}

	if info.BundleID == "" || info.Version == "" {
		return nil, fmt.Errorf("%w: thiếu bundle id hoặc version", ErrUnsupported)
	}
	if info.AppName == "" {
		info.AppName = info.BundleID
	}
	if info.BuildNumber == "" {
		info.BuildNumber = info.Version
	}
	return info, nil
}

func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// readZipFile đọc trọn một entry, từ chối nếu vượt maxSmallFileSize.
func readZipFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxSmallFileSize {
		return nil, fmt.Errorf("%s quá lớn (%d byte)", f.Name, f.UncompressedSize64)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Không tin UncompressedSize64 trong header (có thể bị làm giả): giới hạn
	// thêm lúc đọc thật.
	data, err := io.ReadAll(io.LimitReader(rc, maxSmallFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSmallFileSize {
		return nil, fmt.Errorf("%s quá lớn", f.Name)
	}
	return data, nil
}
