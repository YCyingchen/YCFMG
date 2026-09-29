package vision

import (
	"image"
	"math"
	"strings"
)

// Features 是一张图片的全部可检索特征。
type Features struct {
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Aspect     float64  `json:"aspect"`
	HashA      uint64   `json:"-"`
	HashD      uint64   `json:"-"`
	HashP      uint64   `json:"-"`
	Color      string   `json:"color"`
	ColorName  string   `json:"color_name"`
	TopColors  []string `json:"top_colors"`
	Hist       []byte   `json:"-"`
	Bright     float64  `json:"bright"`
	Contrast   float64  `json:"contrast"`
	Saturation float64  `json:"saturation"`
	Sharp      float64  `json:"sharp"`
	SkinRatio  float64  `json:"skin_ratio"`
	GreenRatio float64  `json:"green_ratio"`
	BlueRatio  float64  `json:"blue_ratio"`
	WarmRatio  float64  `json:"warm_ratio"`
	DarkRatio  float64  `json:"dark_ratio"`
	Classify   string   `json:"classify"`
	Tags       []string `json:"tags"`
}

// Extract 计算图片特征。
func Extract(img image.Image, name string, ex EXIF) Features {
	b := img.Bounds()
	f := Features{
		Width:  b.Dx(),
		Height: b.Dy(),
	}
	if f.Height > 0 {
		f.Aspect = float64(f.Width) / float64(f.Height)
	}
	f.HashA = AHash(img)
	f.HashD = DHash(img)
	f.HashP = PHash(img)
	f.Color = DominantColor(img)
	f.ColorName = ColorName(f.Color)
	f.TopColors = TopColors(img, 3)
	f.Hist = Histogram(img, 4)
	f.Bright, f.Contrast, f.Saturation = Stats(img)
	f.Sharp = Sharpness(img)
	f.SkinRatio, f.GreenRatio, f.BlueRatio, f.WarmRatio, f.DarkRatio = Palette(img)
	f.Classify = Classify(&f, name, ex)
	f.Tags = AutoTags(&f, name, ex)
	return f
}

// DefaultTagColor 返回标签的默认颜色。
func DefaultTagColor(name string) string {
	palette := []string{"#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6", "#06B6D4", "#EC4899", "#84CC16"}
	var sum int
	for _, r := range name {
		sum += int(r)
	}
	if sum < 0 {
		sum = -sum
	}
	return palette[sum%len(palette)]
}

// ScoreParts 是两图相似度的各分量。
type ScoreParts struct {
	PHash     int
	DHash     int
	AHash     int
	Hist      float64
	Tolerance int
}

// Compare 计算两个特征的相似度分量。
func Compare(a, b Features, tolerance int) ScoreParts {
	if tolerance <= 0 {
		tolerance = 6
	}
	return ScoreParts{
		PHash:     Hamming(a.HashP, b.HashP),
		DHash:     Hamming(a.HashD, b.HashD),
		AHash:     Hamming(a.HashA, b.HashA),
		Hist:      HistDistance(a.Hist, b.Hist),
		Tolerance: tolerance,
	}
}

// Normalized 返回 0..1 的相似度（1 表示几乎相同）。
func (s ScoreParts) Normalized() float64 {
	hp := 1 - float64(s.PHash)/64.0
	hd := 1 - float64(s.DHash)/64.0
	ha := 1 - float64(s.AHash)/64.0
	hh := 1 - s.Hist
	v := 0.45*hp + 0.30*hd + 0.10*ha + 0.15*hh
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return v
}

// Duplicate 判断是否可视为重复图。
func (s ScoreParts) Duplicate() bool {
	return s.PHash <= s.Tolerance && s.Hist < 0.35
}

// Grayscale 判断是否近似黑白图。
func Grayscale(f *Features) bool { return f.Saturation < 0.08 }

// IsPanorama 判断是否全景/长图。
func IsPanorama(f *Features) bool { return f.Aspect > 2.2 || f.Aspect < 0.45 }

// AspectLabel 返回画幅描述。
func AspectLabel(f *Features) string {
	a := f.Aspect
	switch {
	case a > 2.2:
		return "长图"
	case a > 1.6:
		return "宽幅"
	case a > 1.2:
		return "横向"
	case a > 0.85:
		return "方形"
	case a > 0.6:
		return "竖向"
	default:
		return "长竖图"
	}
}

// ResolutionLabel 返回分辨率档位。
func ResolutionLabel(f *Features) string {
	px := f.Width * f.Height
	switch {
	case px >= 33000000:
		return "8K+"
	case px >= 8000000:
		return "4K+"
	case px >= 2000000:
		return "1080P+"
	case px >= 800000:
		return "高清"
	default:
		return "标清"
	}
}

// SquaredDistance 计算两个向量的欧氏距离平方。
func SquaredDistance(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return sum
}

// NormalizeVector 归一化向量。
func NormalizeVector(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}

// CleanTag 规范化标签文本。
func CleanTag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", " ")
	return s
}
