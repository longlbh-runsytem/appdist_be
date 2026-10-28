package appinfo

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"howett.net/plist"
)

// buildZip dựng file zip trong bộ nhớ từ map tên → nội dung.
func buildZip(t *testing.T, files map[string][]byte) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func binaryPlist(t *testing.T, v any) []byte {
	t.Helper()
	data, err := plist.Marshal(v, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// fakeProvision giả lập embedded.mobileprovision: plist XML kẹp giữa rác nhị
// phân, giống cấu trúc CMS thật.
func fakeProvision(t *testing.T, v map[string]any) []byte {
	t.Helper()
	xml, err := plist.Marshal(v, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte{0x30, 0x82, 0x1f, 0x00, 0x06}, xml...)
	return append(out, 0xa0, 0x82, 0x0d)
}

// toCgBI mã hoá ảnh theo định dạng CgBI của Xcode: BGRA premultiplied, raw
// deflate, có chunk CgBI đứng trước IHDR.
func toCgBI(t *testing.T, img *image.NRGBA) []byte {
	t.Helper()
	w, h := img.Rect.Dx(), img.Rect.Dy()

	var raw bytes.Buffer
	for y := 0; y < h; y++ {
		raw.WriteByte(0) // filter None
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			pm := func(v uint8) uint8 { return uint8(int(v) * int(c.A) / 255) }
			raw.Write([]byte{pm(c.B), pm(c.G), pm(c.R), c.A})
		}
	}

	var idat bytes.Buffer
	fw, _ := flate.NewWriter(&idat, flate.DefaultCompression)
	fw.Write(raw.Bytes())
	fw.Close()

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, 6 // 8 bit, RGBA

	var out bytes.Buffer
	out.Write(pngSignature)
	writeChunk(&out, "CgBI", []byte{0x50, 0x00, 0x20, 0x02})
	writeChunk(&out, "IHDR", ihdr)
	writeChunk(&out, "IDAT", idat.Bytes())
	writeChunk(&out, "IEND", nil)
	return out.Bytes()
}

func testIcon() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 50, B: 10, A: 255})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 0, A: 128}) // pixel bán trong suốt
	return img
}

func TestParseIPAInHouse(t *testing.T) {
	expires := time.Now().Add(200 * 24 * time.Hour).UTC().Truncate(time.Second)

	r := buildZip(t, map[string][]byte{
		"Payload/Pharmago.app/Info.plist": binaryPlist(t, map[string]any{
			"CFBundleDisplayName":        "Pharmago Store",
			"CFBundleName":               "Pharmago",
			"CFBundleIdentifier":         "vn.idt.pharmago",
			"CFBundleShortVersionString": "1.4.2",
			"CFBundleVersion":            "87",
			"MinimumOSVersion":           "15.0",
			"UIDeviceFamily":             []int{1, 2},
			"DTSDKName":                  "iphoneos18.0",
			"CFBundleIcons": map[string]any{
				"CFBundlePrimaryIcon": map[string]any{"CFBundleIconFiles": []string{"AppIcon60x60"}},
			},
		}),
		// Info.plist của extension phải bị bỏ qua.
		"Payload/Pharmago.app/PlugIns/Notif.appex/Info.plist": binaryPlist(t, map[string]any{
			"CFBundleIdentifier": "vn.idt.pharmago.notif",
		}),
		"Payload/Pharmago.app/embedded.mobileprovision": fakeProvision(t, map[string]any{
			"Name":                 "Pharmago InHouse",
			"TeamName":             "IDT Co., Ltd",
			"TeamIdentifier":       []string{"ABCDE12345"},
			"ExpirationDate":       expires,
			"ProvisionsAllDevices": true,
		}),
		"Payload/Pharmago.app/AppIcon60x60@2x.png": toCgBI(t, testIcon()),
		"Payload/Pharmago.app/AppIcon60x60@3x.png": toCgBI(t, testIcon()),
	})

	info, err := Parse(r, r.Size())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	checks := map[string][2]string{
		"platform":     {string(info.Platform), "ios"},
		"app_name":     {info.AppName, "Pharmago Store"},
		"bundle_id":    {info.BundleID, "vn.idt.pharmago"},
		"version":      {info.Version, "1.4.2"},
		"build_number": {info.BuildNumber, "87"},
		"min_os":       {info.MinOSVersion, "15.0"},
		"distribution": {info.Extra["distribution"].(string), DistributionInHouse},
		"team_name":    {info.Extra["team_name"].(string), "IDT Co., Ltd"},
		"team_id":      {info.Extra["team_id"].(string), "ABCDE12345"},
		"profile_exp":  {info.Extra["profile_expires_at"].(string), expires.Format(time.RFC3339)},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}

	if info.Icon == nil {
		t.Fatal("icon nil — CgBI không giải mã được")
	}
	// Icon đã về RGBA chuẩn: màu đúng, alpha đã un-premultiply (sai số làm tròn ≤ 2).
	got := color.NRGBAModel.Convert(info.Icon.At(1, 1)).(color.NRGBA)
	if got != (color.NRGBA{R: 200, G: 50, B: 10, A: 255}) {
		t.Errorf("pixel đặc = %v, want R200 G50 B10", got)
	}
	half := color.NRGBAModel.Convert(info.Icon.At(0, 0)).(color.NRGBA)
	if half.A != 128 || absDiff(half.R, 200) > 2 || absDiff(half.G, 100) > 2 || half.B != 0 {
		t.Errorf("pixel bán trong suốt = %v, want ~R200 G100 B0 A128", half)
	}

	// Icon phải encode lại được thành PNG chuẩn để trình duyệt hiển thị.
	if err := png.Encode(&bytes.Buffer{}, info.Icon); err != nil {
		t.Errorf("encode icon: %v", err)
	}
}

func TestParseIPADistribution(t *testing.T) {
	cases := []struct {
		name string
		prov map[string]any
		want string
	}{
		{"ad-hoc", map[string]any{"ProvisionedDevices": []string{"UDID1", "UDID2"}}, DistributionAdHoc},
		{"development", map[string]any{
			"ProvisionedDevices": []string{"UDID1"},
			"Entitlements":       map[string]any{"get-task-allow": true},
		}, DistributionDevelopment},
		{"app-store", map[string]any{"Name": "AppStore"}, DistributionAppStore},
		{"unsigned", nil, DistributionUnsigned},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string][]byte{
				"Payload/A.app/Info.plist": binaryPlist(t, map[string]any{
					"CFBundleIdentifier": "a.b", "CFBundleShortVersionString": "1.0",
				}),
			}
			if tc.prov != nil {
				files["Payload/A.app/embedded.mobileprovision"] = fakeProvision(t, tc.prov)
			}
			r := buildZip(t, files)
			info, err := Parse(r, r.Size())
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Extra["distribution"]; got != tc.want {
				t.Errorf("distribution = %v, want %v", got, tc.want)
			}
			// Thiếu tên / build number thì lấy giá trị dự phòng.
			if info.AppName != "a.b" || info.BuildNumber != "1.0" {
				t.Errorf("fallback sai: app_name=%q build=%q", info.AppName, info.BuildNumber)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]map[string][]byte{
		"zip thường":    {"readme.txt": []byte("hi")},
		"aab":           {"BundleConfig.pb": []byte{1}, "base/manifest/AndroidManifest.xml": []byte{1}},
		"thiếu version": {"Payload/A.app/Info.plist": binaryPlistNoT(map[string]any{"CFBundleIdentifier": "a.b"})},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			r := buildZip(t, files)
			if _, err := Parse(r, r.Size()); !errors.Is(err, ErrUnsupported) {
				t.Errorf("err = %v, want ErrUnsupported", err)
			}
		})
	}

	t.Run("không phải zip", func(t *testing.T) {
		r := bytes.NewReader([]byte("definitely not a zip file"))
		if _, err := Parse(r, r.Size()); !errors.Is(err, ErrUnsupported) {
			t.Errorf("err = %v, want ErrUnsupported", err)
		}
	})
}

func binaryPlistNoT(v any) []byte {
	data, _ := plist.Marshal(v, plist.BinaryFormat)
	return data
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
