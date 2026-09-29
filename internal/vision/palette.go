package vision

import "image"

// Palette 统计肤色、绿色、蓝色、暖色与暗部像素占比，用于规则分类。
func Palette(img image.Image) (skin, green, blue, warm, dark float64) {
	small := Scale(img, 32, 32)
	n := 0
	for i := 0; i+3 < len(small.Pix); i += 4 {
		r := float64(small.Pix[i])
		g := float64(small.Pix[i+1])
		b := float64(small.Pix[i+2])
		n++
		if isSkinTone(r, g, b) {
			skin++
		}
		if g > r+12 && g > b+8 {
			green++
		}
		if b > r+12 && b > g+6 {
			blue++
		}
		if r > b+28 && r > g {
			warm++
		}
		if (0.299*r+0.587*g+0.114*b)/255.0 < 0.25 {
			dark++
		}
	}
	if n == 0 {
		return 0, 0, 0, 0, 0
	}
	f := float64(n)
	return skin / f, green / f, blue / f, warm / f, dark / f
}

func isSkinTone(r, g, b float64) bool {
	mx := maxF(r, maxF(g, b))
	mn := minF(r, minF(g, b))
	if mx-mn <= 12 {
		return false
	}
	if !(r > 88 && g > 38 && b > 18 && r > g && g > b) {
		return false
	}
	return (mx - mn) > 14
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
