// Package vision 负责图像特征提取：感知哈希、颜色直方图、EXIF 与规则分类。
// 该包不依赖任何外部模型，纯本地计算，保证在 NAS 上零配置可用。
package vision

import (
	"bufio"
	"fmt"
	"image"
	"os"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"golang.org/x/image/tiff"
	"golang.org/x/image/webp"
)

func init() {
	image.RegisterFormat("bmp", "BM", bmp.Decode, bmp.DecodeConfig)
	image.RegisterFormat("tiff", "II*\x00", tiff.Decode, tiff.DecodeConfig)
	image.RegisterFormat("tiff", "MM\x00*", tiff.Decode, tiff.DecodeConfig)
	image.RegisterFormat("webp", "RIFF????WEBPVP8", webp.Decode, webp.DecodeConfig)
}

// Decode 解码图片文件。
func Decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(bufio.NewReaderSize(f, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("解码失败: %w", err)
	}
	return img, nil
}

// Size 只读取图片头部的宽高，避免整图解码。
func Size(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(bufio.NewReaderSize(f, 1<<16))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// Scale 用 CatmullRom 高质量缩放为 RGBA。
func Scale(img image.Image, w, h int) *image.RGBA {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// ScaleGray 把图片缩放为灰度图。
func ScaleGray(img image.Image, w, h int) *image.Gray {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewGray(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// FitSize 计算保持比例的缩略尺寸。
func FitSize(w, h, max int) (int, int) {
	if max <= 0 || (w <= max && h <= max) {
		return w, h
	}
	if w >= h {
		nh := h * max / w
		if nh < 1 {
			nh = 1
		}
		return max, nh
	}
	nw := w * max / h
	if nw < 1 {
		nw = 1
	}
	return nw, max
}
