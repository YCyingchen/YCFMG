package vision

import (
	"image"
	"math"
	"math/bits"
)

// AHash 均值哈希：缩放为 8x8 灰度后与均值比较。
func AHash(img image.Image) uint64 {
	g := ScaleGray(img, 8, 8)
	var sum float64
	for _, v := range g.Pix {
		sum += float64(v)
	}
	mean := sum / 64.0
	var h uint64
	for i, v := range g.Pix {
		if v < 0 {
			v = 0
		}
		if float64(v) >= mean {
			h |= 1 << uint(i)
		}
	}
	return h
}

// DHash 差值哈希：9x8 灰度逐行比较相邻像素。
func DHash(img image.Image) uint64 {
	g := ScaleGray(img, 9, 8)
	var h uint64
	bit := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			l := g.GrayAt(x, y).Y
			r := g.GrayAt(x+1, y).Y
			if l > r {
				h |= 1 << uint(bit)
			}
			bit++
		}
	}
	return h
}

// PHash 感知哈希：32x32 DCT 变换后取左上 8x8 低频系数与均值比较。
func PHash(img image.Image) uint64 {
	const n = 32
	g := ScaleGray(img, n, n)
	m := make([]float64, n*n)
	for i, v := range g.Pix {
		m[i] = float64(v)
	}
	d := dct2d(m, n)
	var sum float64
	count := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x == 0 && y == 0 {
				continue
			}
			sum += d[y*n+x]
			count++
		}
	}
	mean := 0.0
	if count > 0 {
		mean = sum / float64(count)
	}
	var h uint64
	bit := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x == 0 && y == 0 {
				continue
			}
			if d[y*n+x] > mean {
				h |= 1 << uint(bit)
			}
			bit++
		}
	}
	return h
}

// dct2d 计算二维 DCT-II。
func dct2d(in []float64, n int) []float64 {
	cos := make([]float64, n*n)
	for u := 0; u < n; u++ {
		for x := 0; x < n; x++ {
			cos[u*n+x] = math.Cos(math.Pi * float64(2*x+1) * float64(u) / float64(2*n))
		}
	}
	tmp := make([]float64, n*n)
	for y := 0; y < n; y++ {
		for u := 0; u < n; u++ {
			var s float64
			for x := 0; x < n; x++ {
				s += in[y*n+x] * cos[u*n+x]
			}
			tmp[y*n+u] = s
		}
	}
	out := make([]float64, n*n)
	for x := 0; x < n; x++ {
		for v := 0; v < n; v++ {
			var s float64
			for y := 0; y < n; y++ {
				s += tmp[y*n+x] * cos[v*n+y]
			}
			out[v*n+x] = s
		}
	}
	return out
}

// Hamming 计算两个 64 位哈希的汉明距离。
func Hamming(a, b uint64) int { return bits.OnesCount64(a ^ b) }
