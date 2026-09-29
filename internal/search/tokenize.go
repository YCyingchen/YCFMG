// Package search 实现以文搜图、以图搜图与相似聚合。
package search

import (
	"strings"
	"unicode"
)

var stopWords = map[string]bool{
	"的": true, "了": true, "和": true, "与": true, "在": true, "是": true, "我": true,
	"有": true, "就": true, "不": true, "人": true, "都": true, "一": true, "上": true,
	"也": true, "很": true, "到": true, "说": true, "要": true, "去": true, "你": true,
	"会": true, "着": true, "看": true, "好": true, "这": true, "那": true, "图片": true,
	"照片": true, "找": true, "搜": true, "搜索": true, "文件": true, "the": true,
	"a": true, "an": true, "of": true, "and": true, "to": true, "find": true,
	"show": true, "me": true, "photo": true, "photos": true, "image": true,
	"images": true, "picture": true, "pictures": true, "file": true, "files": true,
}

// synonyms 是中文语义扩展词典，让「以文搜图」在不依赖模型时也有语义能力。
var synonyms = map[string][]string{
	"猫":   {"猫", "猫咪", "喵", "cat", "kitty", "kitten"},
	"狗":   {"狗", "狗狗", "汪", "dog", "puppy"},
	"鸟":   {"鸟", "鸟类", "bird"},
	"鱼":   {"鱼", "鱼类", "fish"},
	"花":   {"花", "花卉", "花朵", "flower", "blossom"},
	"树":   {"树", "树木", "森林", "tree", "forest"},
	"山":   {"山", "山脉", "山峰", "mountain", "hill"},
	"海":   {"海", "海洋", "大海", "海滩", "sea", "ocean", "beach"},
	"天空":  {"天空", "天空", "云", "云朵", "sky", "cloud"},
	"日落":  {"日落", "日出", "夕阳", "朝霞", "晚霞", "sunset", "sunrise"},
	"风景":  {"风景", "景色", "自然", "风光", "landscape", "scenery", "nature"},
	"人物":  {"人物", "人像", "自拍", "合影", "portrait", "selfie", "people"},
	"美食":  {"美食", "食物", "菜", "餐", "饭", "food", "meal", "dish"},
	"夜景":  {"夜景", "晚上", "夜间", "黑夜", "night", "nightscape"},
	"建筑":  {"建筑", "大楼", "房子", "桥梁", "building", "architecture"},
	"车":   {"车", "汽车", "轿车", "car", "vehicle"},
	"截图":  {"截图", "屏幕", "screenshot", "screen"},
	"文档":  {"文档", "扫描", "票据", "发票", "document", "scan", "receipt"},
	"表情包": {"表情包", "表情", "梗图", "meme", "sticker", "emoji"},
	"动漫":  {"动漫", "二次元", "漫画", "动画", "anime", "manga", "cartoon"},
	"壁纸":  {"壁纸", "桌面", "wallpaper"},
	"头像":  {"头像", "avatar", "profile"},
	"图标":  {"图标", "logo", "icon"},
	"二维码": {"二维码", "qr", "qrcode", "barcode"},
	"地图":  {"地图", "导航", "map", "navigation"},
	"黑白":  {"黑白", "灰度", "单色", "monochrome", "grayscale"},
	"全景":  {"全景", "长图", "panorama"},
	"模糊":  {"模糊", "失焦", "blur", "blurry"},
	"清晰":  {"清晰", "锐利", "sharp"},
	"红色":  {"红色", "红", "red"},
	"橙色":  {"橙色", "橙", "orange"},
	"黄色":  {"黄色", "黄", "yellow"},
	"绿色":  {"绿色", "绿", "green"},
	"青色":  {"青色", "青", "cyan"},
	"蓝色":  {"蓝色", "蓝", "blue"},
	"紫色":  {"紫色", "紫", "purple"},
	"粉色":  {"粉色", "粉", "pink"},
	"白色":  {"白色", "白", "white"},
	"黑色":  {"黑色", "黑", "black"},
	"灰色":  {"灰色", "灰", "gray", "grey"},
	"收藏":  {"收藏", "喜欢", "favorite"},
	"今天":  {"今天", "today"},
	"昨天":  {"昨天", "yesterday"},
	"本周":  {"本周", "这周", "week"},
	"本月":  {"本月", "这个月", "month"},
	"今年":  {"今年", "year"},
}

// Tokenize 把查询串切成检索词元：中文按二元切分，英文数字按词切分。
func Tokenize(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	push := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] || stopWords[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	cur := strings.Builder{}
	flushLatin := func() {
		if cur.Len() > 0 {
			push(cur.String())
			cur.Reset()
		}
	}
	hanBuf := []rune{}
	flushHan := func() {
		if len(hanBuf) == 0 {
			return
		}
		if len(hanBuf) == 1 {
			push(string(hanBuf[0]))
		} else {
			for i := 0; i < len(hanBuf); i++ {
				push(string(hanBuf[i]))
			}
			for i := 0; i+1 < len(hanBuf); i++ {
				push(string(hanBuf[i : i+2]))
			}
			push(string(hanBuf))
		}
		hanBuf = hanBuf[:0]
	}
	for _, r := range q {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			hanBuf = append(hanBuf, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == 0x2D || r == 0x5F:
			flushHan()
			cur.WriteRune(r)
		default:
			flushLatin()
			flushHan()
		}
	}
	flushLatin()
	flushHan()
	return out
}

// Expand 对词元做同义词扩展，返回用于 SQL 粗筛的模式串。
func Expand(tokens []string) (patterns []string, raw []string) {
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		patterns = append(patterns, s)
	}
	for _, t := range tokens {
		add(t)
		if list, ok := synonyms[t]; ok {
			for _, s := range list {
				add(s)
			}
			if !contains(raw, t) {
				raw = append(raw, t)
			}
		}
	}
	return patterns, raw
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
