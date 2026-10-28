package appinfo

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"io"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// decodeIOSPNG giải mã PNG lấy từ IPA.
//
// Xcode mặc định "tối ưu" PNG sang định dạng riêng của Apple (CgBI): có thêm
// chunk CgBI, dữ liệu IDAT là raw deflate (thiếu header zlib), pixel theo thứ
// tự BGRA và alpha đã premultiply. Trình duyệt và image/png đều không đọc
// được bản gốc, nên phải chuyển về PNG chuẩn trước.
func decodeIOSPNG(data []byte) (image.Image, error) {
	if !isCgBI(data) {
		return png.Decode(bytes.NewReader(data))
	}

	fixed, err := rewrapCgBI(data)
	if err != nil {
		return nil, err
	}

	img, err := png.Decode(bytes.NewReader(fixed))
	if err != nil {
		return nil, err
	}
	return unpremultiplyBGRA(img), nil
}

func isCgBI(data []byte) bool {
	return len(data) >= 16 && bytes.Equal(data[:8], pngSignature) && string(data[12:16]) == "CgBI"
}

// rewrapCgBI dựng lại một PNG hợp lệ: bỏ chunk CgBI, gộp IDAT, bọc lại bằng
// zlib. Thứ tự kênh màu vẫn là BGRA — sửa sau khi decode, vì lúc đó image/png
// đã lo phần unfilter từng dòng giúp.
func rewrapCgBI(data []byte) ([]byte, error) {
	var ihdr []byte
	var idat bytes.Buffer

	for pos := len(pngSignature); pos+12 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		typ := string(data[pos+4 : pos+8])
		if length < 0 || pos+12+length > len(data) {
			return nil, errors.New("cgbi: chunk hỏng")
		}
		body := data[pos+8 : pos+8+length]

		switch typ {
		case "IHDR":
			ihdr = body
		case "IDAT":
			idat.Write(body)
		case "IEND":
			pos = len(data)
			continue
		}
		pos += 12 + length
	}
	if ihdr == nil || idat.Len() == 0 {
		return nil, errors.New("cgbi: thiếu IHDR hoặc IDAT")
	}

	raw, err := io.ReadAll(io.LimitReader(flate.NewReader(&idat), maxSmallFileSize*4))
	if err != nil {
		return nil, err
	}

	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.Write(pngSignature)
	writeChunk(&out, "IHDR", ihdr)
	writeChunk(&out, "IDAT", zbuf.Bytes())
	writeChunk(&out, "IEND", nil)
	return out.Bytes(), nil
}

func writeChunk(w *bytes.Buffer, typ string, body []byte) {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[:4], uint32(len(body)))
	copy(hdr[4:], typ)
	w.Write(hdr[:])
	w.Write(body)

	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(body)
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc.Sum32())
	w.Write(sum[:])
}

// unpremultiplyBGRA đổi BGRA premultiplied thành RGBA thường.
func unpremultiplyBGRA(img image.Image) image.Image {
	switch src := img.(type) {
	case *image.NRGBA:
		out := image.NewNRGBA(src.Rect)
		for i := 0; i+3 < len(src.Pix); i += 4 {
			b, g, r, a := src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3]
			if a != 0 && a != 255 {
				r, g, b = unpremul(r, a), unpremul(g, a), unpremul(b, a)
			}
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = r, g, b, a
		}
		return out
	case *image.RGBA:
		// Ảnh không có kênh alpha: chỉ cần đảo B và R.
		out := image.NewRGBA(src.Rect)
		for i := 0; i+3 < len(src.Pix); i += 4 {
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = src.Pix[i+2], src.Pix[i+1], src.Pix[i], src.Pix[i+3]
		}
		return out
	default:
		return img
	}
}

func unpremul(c, a uint8) uint8 {
	v := int(c) * 255 / int(a)
	if v > 255 {
		return 255
	}
	return uint8(v)
}
