package vision

import (
	"encoding/binary"
	"os"
	"strconv"
	"strings"
	"time"
)

// EXIF 是从图片中解析出的拍摄信息。
type EXIF struct {
	Make        string  `json:"make"`
	Model       string  `json:"model"`
	Lens        string  `json:"lens"`
	DateTime    string  `json:"datetime"`
	TakenAt     int64   `json:"taken_at"`
	Exposure    string  `json:"exposure"`
	FNumber     string  `json:"fnum"`
	ISO         int     `json:"iso"`
	Focal       string  `json:"focal"`
	Orientation int     `json:"orientation"`
	HasGPS      bool    `json:"has_gps"`
	GPSLat      float64 `json:"gps_lat"`
	GPSLon      float64 `json:"gps_lon"`
}

// ReadEXIF 从文件头部解析 EXIF（JPEG APP1）。
func ReadEXIF(path string) EXIF {
	var e EXIF
	f, err := os.Open(path)
	if err != nil {
		return e
	}
	defer f.Close()
	buf := make([]byte, 2<<20)
	n, _ := f.Read(buf)
	if n <= 0 {
		return e
	}
	tiff := findExifTIFF(buf[:n])
	if tiff == nil {
		return e
	}
	e.parse(tiff)
	return e
}

func findExifTIFF(b []byte) []byte {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		// 非 JPEG：尝试直接按 TIFF 解析
		if len(b) > 8 && ((b[0] == 0x49 && b[1] == 0x49) || (b[0] == 0x4D && b[1] == 0x4D)) {
			return b
		}
		return nil
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			i++
			continue
		}
		marker := b[i+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		if marker == 0xDA {
			break
		}
		size := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if size < 2 {
			break
		}
		end := i + 2 + size
		if end > len(b) {
			end = len(b)
		}
		seg := b[i+4 : end]
		if marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return seg[6:]
		}
		i += 2 + size
	}
	return nil
}

func (e *EXIF) parse(t []byte) {
	if len(t) < 8 {
		return
	}
	var bo binary.ByteOrder
	switch {
	case t[0] == 0x49 && t[1] == 0x49:
		bo = binary.LittleEndian
	case t[0] == 0x4D && t[1] == 0x4D:
		bo = binary.BigEndian
	default:
		return
	}
	if bo.Uint16(t[2:4]) != 42 {
		return
	}
	e.readIFD(t, bo, int(bo.Uint32(t[4:8])), 0)
}

func (e *EXIF) readIFD(t []byte, bo binary.ByteOrder, off, depth int) {
	if off <= 0 || off+2 > len(t) || depth > 3 {
		return
	}
	count := int(bo.Uint16(t[off : off+2]))
	base := off + 2
	for i := 0; i < count; i++ {
		p := base + i*12
		if p+12 > len(t) {
			return
		}
		tag := bo.Uint16(t[p : p+2])
		typ := bo.Uint16(t[p+2 : p+4])
		cnt := int(bo.Uint32(t[p+4 : p+8]))
		vp := p + 8
		switch tag {
		case 0x8769:
			e.readIFD(t, bo, int(bo.Uint32(t[vp:vp+4])), depth+1)
		case 0x8825:
			e.readGPS(t, bo, int(bo.Uint32(t[vp:vp+4])))
		case 0x010F:
			e.Make = strings.TrimSpace(e.ascii(t, bo, typ, cnt, vp))
		case 0x0110:
			e.Model = strings.TrimSpace(e.ascii(t, bo, typ, cnt, vp))
		case 0x0112:
			e.Orientation = e.uintVal(t, bo, typ, cnt, vp)
		case 0x9003, 0x9004:
			s := strings.TrimSpace(e.ascii(t, bo, typ, cnt, vp))
			if e.DateTime == "" {
				e.DateTime = s
			}
			if e.TakenAt == 0 {
				e.TakenAt = parseExifTime(s)
			}
		case 0x829A:
			if r, ok := e.rational(t, bo, typ, cnt, vp, 0); ok && r > 0 {
				if r >= 1 {
					e.Exposure = trimFloat(r) + "s"
				} else {
					e.Exposure = "1/" + trimFloat(1/r) + "s"
				}
			}
		case 0x829D:
			if r, ok := e.rational(t, bo, typ, cnt, vp, 0); ok && r > 0 {
				e.FNumber = "f/" + trimFloat(r)
			}
		case 0x8827:
			e.ISO = e.uintVal(t, bo, typ, cnt, vp)
		case 0x920A:
			if r, ok := e.rational(t, bo, typ, cnt, vp, 0); ok && r > 0 {
				e.Focal = trimFloat(r) + "mm"
			}
		case 0xA434:
			e.Lens = strings.TrimSpace(e.ascii(t, bo, typ, cnt, vp))
		}
	}
}

func (e *EXIF) readGPS(t []byte, bo binary.ByteOrder, off int) {
	if off <= 0 || off+2 > len(t) {
		return
	}
	count := int(bo.Uint16(t[off : off+2]))
	base := off + 2
	latRef, lonRef := "", ""
	var lat, lon float64
	for i := 0; i < count; i++ {
		p := base + i*12
		if p+12 > len(t) {
			return
		}
		tag := bo.Uint16(t[p : p+2])
		typ := bo.Uint16(t[p+2 : p+4])
		cnt := int(bo.Uint32(t[p+4 : p+8]))
		vp := p + 8
		switch tag {
		case 0x0001:
			latRef = e.ascii(t, bo, typ, cnt, vp)
		case 0x0002:
			lat = e.gpsCoord(t, bo, typ, cnt, vp)
		case 0x0003:
			lonRef = e.ascii(t, bo, typ, cnt, vp)
		case 0x0004:
			lon = e.gpsCoord(t, bo, typ, cnt, vp)
		}
	}
	if lat == 0 && lon == 0 {
		return
	}
	if strings.HasPrefix(strings.ToUpper(latRef), "S") {
		lat = -lat
	}
	if strings.HasPrefix(strings.ToUpper(lonRef), "W") {
		lon = -lon
	}
	e.GPSLat, e.GPSLon, e.HasGPS = lat, lon, true
}

func (e *EXIF) dataPtr(t []byte, typ uint16, cnt int, vp int) (int, int) {
	size := typeSize(typ) * cnt
	if size <= 4 {
		return vp, size
	}
	off := int(binary.LittleEndian.Uint32(t[vp : vp+4]))
	return off, size
}

func (e *EXIF) ascii(t []byte, bo binary.ByteOrder, typ uint16, cnt int, vp int) string {
	if typ != 2 && typ != 7 {
		return ""
	}
	var off, size int
	if typeSize(typ)*cnt <= 4 {
		off, size = vp, typeSize(typ)*cnt
	} else {
		off = int(bo.Uint32(t[vp : vp+4]))
		size = cnt
	}
	if off < 0 || off+size > len(t) || size <= 0 {
		return ""
	}
	s := string(t[off : off+size])
	return strings.TrimRight(s, "\x00")
}

func (e *EXIF) uintVal(t []byte, bo binary.ByteOrder, typ uint16, cnt int, vp int) int {
	off, _ := e.ptrOf(t, bo, typ, cnt, vp)
	if off < 0 {
		return 0
	}
	switch typ {
	case 3:
		if off+2 <= len(t) {
			return int(bo.Uint16(t[off : off+2]))
		}
	case 4:
		if off+4 <= len(t) {
			return int(bo.Uint32(t[off : off+4]))
		}
	case 1:
		if off < len(t) {
			return int(t[off])
		}
	}
	return 0
}

func (e *EXIF) ptrOf(t []byte, bo binary.ByteOrder, typ uint16, cnt int, vp int) (int, int) {
	if vp+4 > len(t) {
		return -1, 0
	}
	size := typeSize(typ) * cnt
	if size <= 0 {
		return -1, 0
	}
	if size <= 4 {
		return vp, size
	}
	off := int(bo.Uint32(t[vp : vp+4]))
	if off < 0 || off >= len(t) {
		return -1, 0
	}
	return off, size
}

func (e *EXIF) rational(t []byte, bo binary.ByteOrder, typ uint16, cnt int, vp int, idx int) (float64, bool) {
	off, size := e.ptrOf(t, bo, typ, cnt, vp)
	if off < 0 || idx < 0 {
		return 0, false
	}
	if typ == 5 || typ == 10 {
		p := off + idx*8
		if p+8 > len(t) || (off+size) > len(t) {
			return 0, false
		}
		num := bo.Uint32(t[p : p+4])
		den := bo.Uint32(t[p+4 : p+8])
		if den == 0 {
			return 0, false
		}
		return float64(num) / float64(den), true
	}
	if typ == 3 {
		p := off + idx*2
		if p+2 > len(t) {
			return 0, false
		}
		return float64(bo.Uint16(t[p : p+2])), true
	}
	if typ == 4 {
		p := off + idx*4
		if p+4 > len(t) {
			return 0, false
		}
		return float64(bo.Uint32(t[p : p+4])), true
	}
	return 0, false
}

func (e *EXIF) gpsCoord(t []byte, bo binary.ByteOrder, typ uint16, cnt int, vp int) float64 {
	if cnt < 3 {
		return 0
	}
	d, ok1 := e.rational(t, bo, typ, cnt, vp, 0)
	m, ok2 := e.rational(t, bo, typ, cnt, vp, 1)
	s, ok3 := e.rational(t, bo, typ, cnt, vp, 2)
	if !ok1 || !ok2 || !ok3 {
		return 0
	}
	return d + m/60 + s/3600
}

func typeSize(typ uint16) int {
	switch typ {
	case 1, 2, 6, 7:
		return 1
	case 3, 8:
		return 2
	case 4, 9, 11:
		return 4
	case 5, 10, 12:
		return 8
	}
	return 0
}

func parseExifTime(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	layouts := []string{"2006:01:02 15:04:05", "2006-01-02 15:04:05", "2006:01:02 15:04", "2006-01-02T15:04:05"}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func trimFloat(v float64) string {
	s := strings.TrimRight(strings.TrimRight(formatFloat(v), "0"), ".")
	if s == "" {
		return "0"
	}
	return s
}

func formatFloat(v float64) string {
	if v >= 100 {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
