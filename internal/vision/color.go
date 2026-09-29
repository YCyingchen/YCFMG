package vision

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// Histogram 计算 bins^3 维颜色直方图并归一化到 0..255。
func Histogram(img image.Image, bins int) []byte {
	if bins < 2 {
		bins = 4
	}
	small := Scale(img, 64, 64)
	hist := make([]int, bins*bins*bins)
	for i := 0; i+3 < len(small.Pix); i += 4 {
		r := int(small.Pix[i]) * bins / 256
		g := int(small.Pix[i+1]) * bins / 256
		b := int(small.Pix[i+2]) * bins / 256
		hist[(r*bins+g)*bins+b]++
	}
	max := 1
	for _, v := range hist {
		if v > max {
			max = v
		}
	}
	out := make([]byte, len(hist))
	for i, v := range hist {
		out[i] = byte(v * 255 / max)
	}
	return out
}

// HistDistance 计算两个归一化直方图的 L1 距离（0 表示完全相同）。
func HistDistance(a, b []byte) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 1
	}
	var sum float64
	for i := range a {
		d := float64(int(a[i]) - int(b[i]))
		if d < 0 {
			d = -d
		}
		sum += d
	}
	return sum / (255.0 * float64(len(a)))
}

// DominantColor 返回图像主色（十六进制）。
func DominantColor(img image.Image) string {
	small := Scale(img, 32, 32)
	counts := map[int]int{}
	for i := 0; i+3 < len(small.Pix); i += 4 {
		r := int(small.Pix[i]) >> 5
		g := int(small.Pix[i+1]) >> 5
		b := int(small.Pix[i+2]) >> 5
		counts[(r<<6)|(g<<3)|b]++
	}
	best, bestN := 0, -1
	for k, v := range counts {
		if v > bestN {
			bestN = v
			best = k
		}
	}
	r := (best>>6)&7*32 + 16
	g := (best>>3)&7*32 + 16
	b := best&7*32 + 16
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// ColorName 把十六进制颜色归入中文色系。
func ColorName(hex string) string {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02X%02X%02X", &r, &g, &b); err != nil {
		return "其他"
	}
	mx := max3(r, g, b)
	mn := min3(r, g, b)
	v := float64(mx) / 255.0
	s := 0.0
	if mx > 0 {
		s = float64(mx-mn) / float64(mx)
	}
	h := 0.0
	if mx != mn {
		rf, gf, bf := float64(r), float64(g), float64(b)
		switch mx {
		case r:
			h = math.Mod((gf-bf)/float64(mx-mn), 6)
		case g:
			h = (bf-rf)/float64(mx-mn) + 2
		default:
			h = (rf-gf)/float64(mx-mn) + 4
		}
		h *= 60
		if h < 0 {
			h += 360
		}
	}
	if v < 0.16 {
		return "黑色"
	}
	if s < 0.12 {
		if v > 0.85 {
			return "白色"
		}
		return "灰色"
	}
	switch {
	case h < 15 || h >= 345:
		return "红色"
	case h < 40:
		return "橙色"
	case h < 70:
		return "黄色"
	case h < 165:
		return "绿色"
	case h < 200:
		return "青色"
	case h < 255:
		return "蓝色"
	case h < 290:
		return "紫色"
	case h < 345:
		return "粉色"
	}
	return "其他"
}

// Stats 返回平均亮度、对比度与饱和度。
func Stats(img image.Image) (float64, float64, float64) {
	small := Scale(img, 32, 32)
	n := 0
	var sum, sumSq, sat float64
	for i := 0; i+3 < len(small.Pix); i += 4 {
		r := float64(small.Pix[i])
		g := float64(small.Pix[i+1])
		b := float64(small.Pix[i+2])
		lum := (0.299*r + 0.587*g + 0.114*b) / 255.0
		mx := math.Max(r, math.Max(g, b))
		mn := math.Min(r, math.Min(g, b))
		if mx > 0 {
			sat += (mx - mn) / mx
		}
		sum += lum
		sumSq += lum * lum
		n++
	}
	if n == 0 {
		return 0, 0, 0
	}
	mean := sum / float64(n)
	variance := sumSq/float64(n) - mean*mean
	if variance < 0 {
		variance = 0
	}
	return mean, math.Sqrt(variance), sat / float64(n)
}

// Sharpness 用拉普拉斯响应方差近似清晰度。
func Sharpness(img image.Image) float64 {
	const n = 128
	g := ScaleGray(img, n, n)
	vals := make([]float64, 0, n*n)
	for y := 1; y < n-1; y++ {
		for x := 1; x < n-1; x++ {
			c := float64(g.GrayAt(x, y).Y)
			l := float64(g.GrayAt(x-1, y).Y)
			r := float64(g.GrayAt(x+1, y).Y)
			u := float64(g.GrayAt(x, y-1).Y)
			d := float64(g.GrayAt(x, y+1).Y)
			vals = append(vals, 4*c-l-r-u-d)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	var mean float64
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	var varsum float64
	for _, v := range vals {
		d := v - mean
		varsum += d * d
	}
	return varsum / float64(len(vals))
}

// TopColors 返回占比最高的若干主色。
func TopColors(img image.Image, k int) []string {
	if k <= 0 {
		k = 3
	}
	small := Scale(img, 48, 48)
	counts := map[int]int{}
	for i := 0; i+3 < len(small.Pix); i += 4 {
		r := int(small.Pix[i]) >> 4
		g := int(small.Pix[i+1]) >> 4
		b := int(small.Pix[i+2]) >> 4
		counts[(r<<8)|(g<<4)|b]++
	}
	type kv struct {
		k int
		v int
	}
	arr := make([]kv, 0, len(counts))
	for kk, vv := range counts {
		arr = append(arr, kv{kk, vv})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	out := []string{}
	for i := 0; i < len(arr) && i < k; i++ {
		r := (arr[i].k >> 8) & 15 * 16
		g := (arr[i].k >> 4) & 15 * 16
		b := arr[i].k & 15 * 16
		out = append(out, fmt.Sprintf("#%02X%02X%02X", r+8, g+8, b+8))
	}
	return out
}

func max3(a, b, c int) int {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
