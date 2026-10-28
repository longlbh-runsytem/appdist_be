package appinfo

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"howett.net/plist"
)

// Distribution là kiểu ký của IPA, suy ra từ embedded.mobileprovision.
const (
	DistributionInHouse     = "in-house"    // Enterprise: cài được trên mọi máy
	DistributionAdHoc       = "ad-hoc"      // chỉ các máy có UDID trong profile
	DistributionDevelopment = "development" // như ad-hoc, thêm cần bật Developer Mode
	DistributionAppStore    = "app-store"   // KHÔNG cài trực tiếp được
	DistributionUnsigned    = "unsigned"    // không có embedded.mobileprovision — KHÔNG cài được
	DistributionUnknown     = "unknown"     // có profile nhưng không đọc được
)

// Chỉ lấy Info.plist của app chính, đúng 2 cấp: Payload/X.app/Info.plist.
// Extension (PlugIns/*.appex) và framework cũng có Info.plist riêng, phải bỏ qua.
var ipaInfoPlistRe = regexp.MustCompile(`^Payload/[^/]+\.app/Info\.plist$`)

func findIPAInfoPlist(zr *zip.Reader) *zip.File {
	for _, f := range zr.File {
		if ipaInfoPlistRe.MatchString(f.Name) {
			return f
		}
	}
	return nil
}

type iosInfoPlist struct {
	DisplayName     string   `plist:"CFBundleDisplayName"`
	Name            string   `plist:"CFBundleName"`
	BundleID        string   `plist:"CFBundleIdentifier"`
	ShortVersion    string   `plist:"CFBundleShortVersionString"`
	Version         string   `plist:"CFBundleVersion"`
	MinOS           string   `plist:"MinimumOSVersion"`
	DeviceFamily    []int    `plist:"UIDeviceFamily"`
	SDKName         string   `plist:"DTSDKName"`
	Xcode           string   `plist:"DTXcode"`
	LegacyIconFiles []string `plist:"CFBundleIconFiles"`
	Icons           struct {
		Primary struct {
			Files []string `plist:"CFBundleIconFiles"`
		} `plist:"CFBundlePrimaryIcon"`
	} `plist:"CFBundleIcons"`
}

type mobileProvision struct {
	Name                 string         `plist:"Name"`
	TeamName             string         `plist:"TeamName"`
	TeamIdentifier       []string       `plist:"TeamIdentifier"`
	ExpirationDate       time.Time      `plist:"ExpirationDate"`
	ProvisionsAllDevices bool           `plist:"ProvisionsAllDevices"`
	ProvisionedDevices   []string       `plist:"ProvisionedDevices"`
	Entitlements         map[string]any `plist:"Entitlements"`
}

func parseIPA(zr *zip.Reader) (*Info, error) {
	plistFile := findIPAInfoPlist(zr)
	appDir := strings.TrimSuffix(plistFile.Name, "Info.plist") // "Payload/X.app/"

	data, err := readZipFile(plistFile)
	if err != nil {
		return nil, fmt.Errorf("%w: đọc Info.plist: %v", ErrUnsupported, err)
	}

	// Info.plist trong IPA thường là binary plist; thư viện đọc được cả XML lẫn binary.
	var p iosInfoPlist
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%w: parse Info.plist: %v", ErrUnsupported, err)
	}

	info := &Info{
		Platform:     PlatformIOS,
		AppName:      firstNonEmpty(p.DisplayName, p.Name),
		BundleID:     p.BundleID,
		Version:      p.ShortVersion,
		BuildNumber:  p.Version,
		MinOSVersion: p.MinOS,
		Extra:        map[string]any{},
	}

	if families := deviceFamilies(p.DeviceFamily); len(families) > 0 {
		info.Extra["device_family"] = families
	}
	if p.SDKName != "" {
		info.Extra["sdk"] = p.SDKName
	}
	if p.Xcode != "" {
		info.Extra["xcode"] = p.Xcode
	}

	// Parser chỉ ghi nhận kiểu ký; chặn hay cho qua là quyết định của service.
	prov, found, ok := readProvision(zr, appDir)
	switch {
	case !found:
		info.Extra["distribution"] = DistributionUnsigned
	case ok:
		info.Extra["distribution"] = distributionOf(prov)
		info.Extra["profile_name"] = prov.Name
		info.Extra["team_name"] = prov.TeamName
		if len(prov.TeamIdentifier) > 0 {
			info.Extra["team_id"] = prov.TeamIdentifier[0]
		}
		if !prov.ExpirationDate.IsZero() {
			info.Extra["profile_expires_at"] = prov.ExpirationDate.UTC().Format(time.RFC3339)
		}
		if n := len(prov.ProvisionedDevices); n > 0 {
			info.Extra["provisioned_device_count"] = n
		}
	default:
		info.Extra["distribution"] = DistributionUnknown
	}

	iconNames := append(p.Icons.Primary.Files, p.LegacyIconFiles...)
	if f := pickIOSIcon(zr, appDir, iconNames); f != nil {
		if raw, err := readZipFile(f); err == nil {
			if img, err := decodeIOSPNG(raw); err == nil {
				info.Icon = img
			}
		}
	}

	return info, nil
}

// readProvision đọc embedded.mobileprovision. File này là plist bọc trong chữ
// ký CMS (PKCS#7); thay vì kéo thêm thư viện pkcs7, cắt thẳng đoạn XML nằm
// giữa — nội dung plist luôn ở dạng text trong đó.
//
// found = có file hay không; ok = đọc được nội dung hay không.
func readProvision(zr *zip.Reader, appDir string) (prov mobileProvision, found, ok bool) {
	f := findFile(zr, appDir+"embedded.mobileprovision")
	if f == nil {
		return prov, false, false
	}
	raw, err := readZipFile(f)
	if err != nil {
		return prov, true, false
	}

	start := bytes.Index(raw, []byte("<?xml"))
	end := bytes.LastIndex(raw, []byte("</plist>"))
	if start < 0 || end < start {
		return prov, true, false
	}

	if _, err := plist.Unmarshal(raw[start:end+len("</plist>")], &prov); err != nil {
		return prov, true, false
	}
	return prov, true, true
}

func distributionOf(p mobileProvision) string {
	switch {
	case p.ProvisionsAllDevices:
		return DistributionInHouse
	case len(p.ProvisionedDevices) > 0:
		if allow, _ := p.Entitlements["get-task-allow"].(bool); allow {
			return DistributionDevelopment
		}
		return DistributionAdHoc
	default:
		return DistributionAppStore
	}
}

// pickIOSIcon chọn icon lớn nhất của app. Info.plist chỉ ghi TIỀN TỐ tên
// (vd "AppIcon60x60"), file thật là "AppIcon60x60@2x.png", "AppIcon60x60@3x.png".
func pickIOSIcon(zr *zip.Reader, appDir string, prefixes []string) *zip.File {
	if len(prefixes) == 0 {
		prefixes = []string{"AppIcon"}
	}

	var best *zip.File
	for _, f := range zr.File {
		// Chỉ file nằm TRỰC TIẾP trong thư mục .app, không lấy trong extension.
		if path.Dir(f.Name)+"/" != appDir || !strings.HasSuffix(f.Name, ".png") {
			continue
		}
		base := path.Base(f.Name)
		for _, prefix := range prefixes {
			if prefix != "" && strings.HasPrefix(base, prefix) {
				if best == nil || f.UncompressedSize64 > best.UncompressedSize64 {
					best = f
				}
				break
			}
		}
	}
	return best
}

func deviceFamilies(codes []int) []string {
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		switch c {
		case 1:
			out = append(out, "iphone")
		case 2:
			out = append(out, "ipad")
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
