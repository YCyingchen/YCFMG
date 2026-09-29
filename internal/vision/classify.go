package vision

import (
	"math"
	"strconv"
	"strings"
)

var screenResolutions = map[string]bool{
	"1440x2560": true, "1080x1920": true, "1080x2340": true, "1170x2532": true,
	"1284x2778": true, "750x1334": true, "828x1792": true, "1125x2436": true,
	"1080x2400": true, "1220x2712": true, "1080x2160": true, "2560x1440": true,
	"1920x1080": true, "1366x768": true, "2880x1800": true, "2560x1600": true,
	"3840x2160": true, "3072x1920": true, "1280x720": true, "1600x900": true,
	"2160x3840": true, "1440x3040": true, "1080x2280": true, "1344x2992": true,
	"2400x1080": true, "2532x1170": true, "2778x1284": true, "1440x3200": true,
}

// Classify 用纯本地规则判断图片所属分类，不依赖任何模型。
func Classify(f *Features, name string, ex EXIF) string {
	lower := strings.ToLower(name)
	if containsAny(lower, "screenshot", "screen_shot", "screencap", "截图", "屏幕快照", "snipping") {
		if f.Aspect > 2.2 {
			return "长截图"
		}
		return "截图"
	}
	if containsAny(lower, "scan", "扫描", "发票", "身份证", "合同", "receipt", "invoice") {
		return "文档"
	}
	if containsAny(lower, "qrcode", "qr_code", "二维码", "barcode", "条形码") {
		return "二维码"
	}
	if containsAny(lower, "emoji", "表情", "sticker", "meme", "梗图") {
		return "表情包"
	}
	if containsAny(lower, "wallpaper", "壁纸", "background", "desktop") {
		return "壁纸"
	}
	if containsAny(lower, "avatar", "头像", "profile") {
		return "头像"
	}
	if containsAny(lower, "icon", "图标", "logo", "标志") {
		return "图标"
	}
	if containsAny(lower, "map", "地图", "导航", "route") {
		return "地图"
	}
	noCamera := strings.TrimSpace(ex.Make) == "" && strings.TrimSpace(ex.Model) == ""
	if noCamera {
		if f.Aspect > 2.3 {
			return "长截图"
		}
		key := strconv.Itoa(f.Width) + "x" + strconv.Itoa(f.Height)
		if screenResolutions[key] {
			return "截图"
		}
	}
	if f.Width <= 260 && f.Height <= 260 && math.Abs(f.Aspect-1) < 0.2 {
		return "图标"
	}
	if f.Aspect > 2.6 {
		return "长截图"
	}
	if f.Bright < 0.26 || f.DarkRatio > 0.62 {
		return "夜景"
	}
	if f.Saturation < 0.10 && f.Bright > 0.6 && f.Contrast > 0.14 {
		return "文档"
	}
	if f.SkinRatio > 0.18 {
		return "人物"
	}
	if f.GreenRatio > 0.30 && f.Bright > 0.32 {
		return "风景"
	}
	if f.BlueRatio > 0.38 && f.Bright > 0.34 {
		return "风景"
	}
	if f.WarmRatio > 0.55 && f.Saturation > 0.28 && f.Bright > 0.32 {
		return "美食"
	}
	if f.Saturation > 0.42 && f.Contrast > 0.18 && f.Sharp > 250 {
		return "动漫"
	}
	if f.Bright > 0.72 && f.Saturation < 0.18 {
		return "极简"
	}
	return "照片"
}

// AutoTags 依据特征与 EXIF 生成自动标签。
func AutoTags(f *Features, name string, ex EXIF) []string {
	if f.Classify == "" {
		f.Classify = Classify(f, name, ex)
	}
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		s = CleanTag(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(f.Classify)
	add(AspectLabel(f))
	add(ResolutionLabel(f))
	add(f.ColorName)
	if Grayscale(f) {
		add("黑白")
	} else if f.Saturation > 0.42 {
		add("高饱和")
	}
	if f.Bright < 0.3 {
		add("暗光")
	} else if f.Bright > 0.72 {
		add("明亮")
	}
	if f.Contrast < 0.12 {
		add("低对比")
	} else if f.Contrast > 0.28 {
		add("高对比")
	}
	if f.Sharp > 600 {
		add("清晰")
	} else if f.Sharp < 25 {
		add("模糊")
	}
	if IsPanorama(f) {
		add("全景")
	}
	if f.Width >= 3840 || f.Height >= 3840 {
		add("4K+")
	}
	if ex.Make != "" {
		add(ex.Make)
	}
	if ex.Model != "" {
		add(ex.Model)
	}
	if ex.Lens != "" {
		add(ex.Lens)
	}
	if ex.HasGPS {
		add("含地理信息")
	}
	if ex.Focal != "" {
		add("焦段 " + ex.Focal)
	}
	if ex.ISO >= 3200 {
		add("高感光")
	}
	return out
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
