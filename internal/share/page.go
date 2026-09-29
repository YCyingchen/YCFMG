package share

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/util"
)

const shareCSS = ":root{--bg:#f6f7fb;--card:#fff;--fg:#16181d;--muted:#6b7280;--line:#e5e7eb;--brand:#2563eb;--brand2:#0ea5e9;--shadow:0 8px 28px rgba(16,24,40,.10)}" +
	"@media (prefers-color-scheme:dark){:root{--bg:#0e1116;--card:#161a21;--fg:#e8eaef;--muted:#9aa3b2;--line:#242a34;--shadow:0 8px 28px rgba(0,0,0,.45)}}" +
	"*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.6 -apple-system,BlinkMacSystemFont,Segoe UI,PingFang SC,Microsoft YaHei,sans-serif}" +
	"a{color:inherit;text-decoration:none}.wrap{max-width:1240px;margin:0 auto;padding:0 20px}" +
	".bar{position:sticky;top:0;z-index:30;background:var(--card);border-bottom:1px solid var(--line)}" +
	".bar .wrap{display:flex;align-items:center;gap:12px;height:60px}.logo{width:30px;height:30px}" +
	".brand{font-weight:800;letter-spacing:.3px}.brand small{display:block;font-weight:500;color:var(--muted);font-size:11px}" +
	".spacer{flex:1}.btn{display:inline-flex;align-items:center;gap:6px;padding:9px 15px;border-radius:10px;border:1px solid var(--line);background:var(--card);cursor:pointer;font-size:14px;color:var(--fg)}" +
	".btn:hover{border-color:var(--brand);color:var(--brand)}.btn.primary{background:linear-gradient(135deg,var(--brand),var(--brand2));border:0;color:#fff}" +
	".hero{padding:34px 0 22px}.hero h1{margin:0 0 8px;font-size:27px;line-height:1.25}" +
	".hero p{margin:0;color:var(--muted)}.meta{display:flex;flex-wrap:wrap;gap:10px;margin-top:16px}" +
	".chip{padding:5px 11px;border-radius:999px;background:var(--card);border:1px solid var(--line);font-size:12.5px;color:var(--muted)}" +
	".grid{columns:5 220px;column-gap:12px;padding-bottom:60px}" +
	"@media(max-width:1100px){.grid{columns:4 180px}}@media(max-width:820px){.grid{columns:3 150px}}@media(max-width:560px){.grid{columns:2 130px}}" +
	".cell{break-inside:avoid;margin:0 0 12px;border-radius:12px;overflow:hidden;background:var(--card);box-shadow:var(--shadow);cursor:zoom-in}" +
	".cell img{display:block;width:100%;height:auto}.cell .cap{padding:7px 9px;font-size:12px;color:var(--muted);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}" +
	".files{display:flex;flex-direction:column;gap:8px;margin:6px 0 40px}.frow{display:flex;align-items:center;gap:12px;padding:12px 14px;background:var(--card);border:1px solid var(--line);border-radius:12px}" +
	".frow .kind{width:38px;height:38px;border-radius:9px;display:grid;place-items:center;background:linear-gradient(135deg,var(--brand),var(--brand2));color:#fff;font-size:12px;font-weight:700}" +
	".frow .nm{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.frow .sz{color:var(--muted);font-size:12.5px}" +
	"footer{border-top:1px solid var(--line);padding:22px 0 40px;color:var(--muted);font-size:13px;text-align:center}" +
	".lb{position:fixed;inset:0;background:rgba(6,8,12,.94);display:none;align-items:center;justify-content:center;z-index:80;flex-direction:column}" +
	".lb.on{display:flex}.lb img{max-width:94vw;max-height:82vh;border-radius:8px;box-shadow:0 20px 60px rgba(0,0,0,.6)}" +
	".lb .tools{display:flex;gap:10px;margin-top:16px}.lb .tools .btn{background:#1b2029;color:#e8eaef;border-color:#2b3341}" +
	".pw{max-width:400px;margin:14vh auto;background:var(--card);border:1px solid var(--line);border-radius:16px;padding:28px;box-shadow:var(--shadow)}" +
	".pw h2{margin:0 0 6px;font-size:20px}.pw p{margin:0 0 18px;color:var(--muted);font-size:13.5px}" +
	".pw input{width:100%;padding:11px 13px;border-radius:10px;border:1px solid var(--line);background:transparent;color:var(--fg);font-size:15px;outline:none}" +
	".pw input:focus{border-color:var(--brand)}.pw .btn{margin-top:14px;width:100%;justify-content:center}" +
	".err{background:#fee2e2;color:#b91c1c;padding:9px 12px;border-radius:9px;font-size:13px;margin-bottom:14px}" +
	".toast{position:fixed;left:50%;bottom:32px;transform:translateX(-50%);background:#111827;color:#fff;padding:10px 18px;border-radius:10px;font-size:13.5px;opacity:0;transition:.25s;z-index:90}" +
	".toast.on{opacity:1}"

const logoSVG = "<svg viewBox=\"0 0 512 512\" class=\"logo\" xmlns=\"http://www.w3.org/2000/svg\"><defs><linearGradient id=\"g\" x1=\"0\" y1=\"0\" x2=\"1\" y2=\"1\"><stop offset=\"0\" stop-color=\"#3B82F6\"/><stop offset=\"1\" stop-color=\"#0EA5E9\"/></linearGradient></defs><rect width=\"512\" height=\"512\" rx=\"120\" fill=\"url(#g)\"/><path fill=\"#fff\" fill-rule=\"evenodd\" d=\"M96 152h118l34 42h168a30 30 0 0 1 30 30v156a30 30 0 0 1-30 30H96a30 30 0 0 1-30-30V182a30 30 0 0 1 30-30zM320 208a28 28 0 1 0 0 56 28 28 0 0 0 0-56zM110 366l86-116 62 80 42-48 72 84z\"/></svg>"

// PageData 是渲染分享页所需的数据。
type PageData struct {
	Brand     string
	Subtitle  string
	Title     string
	Descr     string
	Token     string
	Base      string
	Items     []Item
	NeedPass  bool
	Message   string
	ViewCount int
	ExpiresAt int64
	CreatedAt int64
	Creator   string
	ShowWater bool
	AllowDL   bool
}

// RenderPage 生成专属分享页 HTML。
func RenderPage(d PageData) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	b.WriteString("<meta name=\"robots\" content=\"noindex,nofollow\">")
	b.WriteString("<title>" + html.EscapeString(d.Title) + " - " + html.EscapeString(d.Brand) + "</title>")
	b.WriteString("<style>" + shareCSS + "</style></head><body>")
	b.WriteString("<header class=\"bar\"><div class=\"wrap\">" + logoSVG)
	b.WriteString("<div class=\"brand\">" + html.EscapeString(d.Brand) + "<small>" + html.EscapeString(d.Subtitle) + "</small></div>")
	b.WriteString("<div class=\"spacer\"></div>")
	if d.AllowDL && !d.NeedPass {
		b.WriteString("<button class=\"btn\" onclick=\"copyLink()\">复制链接</button>")
		b.WriteString("<a class=\"btn primary\" href=\"" + d.Base + "/s/" + d.Token + "/zip\">下载全部</a>")
	}
	b.WriteString("</div></header>")
	if d.NeedPass {
		b.WriteString(renderPassword(d))
	} else {
		b.WriteString(renderGallery(d))
	}
	b.WriteString("<footer><div class=\"wrap\">由 " + html.EscapeString(d.Brand) + " 提供文件分享服务 · 文件管理 + 智能图库</div></footer>")
	b.WriteString("<div class=\"lb\" id=\"lb\" onclick=\"closeLb()\"><img id=\"lbImg\" src=\"\" alt=\"\"><div class=\"tools\">")
	b.WriteString("<a class=\"btn\" id=\"lbDl\" href=\"#\">下载原图</a><button class=\"btn\" onclick=\"closeLb()\">关闭</button></div></div>")
	b.WriteString("<div class=\"toast\" id=\"toast\"></div>")
	b.WriteString("<script>" + shareJS + "</script></body></html>")
	return b.String()
}

func renderPassword(d PageData) string {
	var b strings.Builder
	b.WriteString("<div class=\"pw\">")
	b.WriteString("<h2>" + html.EscapeString(d.Title) + "</h2>")
	b.WriteString("<p>该分享已加密，请输入提取密码</p>")
	if d.Message != "" {
		b.WriteString("<div class=\"err\">" + html.EscapeString(d.Message) + "</div>")
	}
	b.WriteString("<form method=\"post\" action=\"" + d.Base + "/s/" + d.Token + "\">")
	b.WriteString("<input type=\"password\" name=\"password\" placeholder=\"提取密码\" autofocus>")
	b.WriteString("<button class=\"btn primary\" type=\"submit\">进入分享</button></form></div>")
	return b.String()
}
func renderGallery(d PageData) string {
	var b strings.Builder
	b.WriteString("<main class=\"wrap\">")
	b.WriteString("<section class=\"hero\"><h1>" + html.EscapeString(d.Title) + "</h1>")
	if d.Descr != "" {
		b.WriteString("<p>" + html.EscapeString(d.Descr) + "</p>")
	}
	b.WriteString("<div class=\"meta\">")
	b.WriteString("<span class=\"chip\">" + fmt.Sprintf("%d 个条目", len(d.Items)) + "</span>")
	b.WriteString("<span class=\"chip\">" + util.HumanSize(totalSize(d.Items)) + "</span>")
	b.WriteString("<span class=\"chip\">" + fmt.Sprintf("%d 次浏览", d.ViewCount) + "</span>")
	if d.ExpiresAt > 0 {
		b.WriteString("<span class=\"chip\">有效至 " + time.Unix(d.ExpiresAt, 0).Format("2006-01-02 15:04") + "</span>")
	} else {
		b.WriteString("<span class=\"chip\">长期有效</span>")
	}
	if d.Creator != "" {
		b.WriteString("<span class=\"chip\">分享者 " + html.EscapeString(d.Creator) + "</span>")
	}
	b.WriteString("</div></section>")

	images := []Item{}
	others := []Item{}
	for _, it := range d.Items {
		if it.Kind == "image" && it.Size >= 0 {
			images = append(images, it)
		} else {
			others = append(others, it)
		}
	}
	sort.SliceStable(images, func(i, j int) bool { return images[i].MTime > images[j].MTime })
	if len(images) > 0 {
		b.WriteString("<section class=\"grid\">")
		for _, it := range images {
			thumb := d.Base + "/s/" + d.Token + "/thumb?p=" + url.QueryEscape(it.Path)
			raw := d.Base + "/s/" + d.Token + "/raw?p=" + url.QueryEscape(it.Path)
			b.WriteString("<figure class=\"cell\" data-img=\"" + html.EscapeString(raw) + "\" data-name=\"" + html.EscapeString(it.Name) + "\">")
			b.WriteString("<img loading=\"lazy\" src=\"" + html.EscapeString(thumb) + "\" alt=\"" + html.EscapeString(it.Name) + "\">")
			b.WriteString("<figcaption class=\"cap\">" + html.EscapeString(it.Name) + "</figcaption></figure>")
		}
		b.WriteString("</section>")
	}
	if len(others) > 0 {
		b.WriteString("<section class=\"files\">")
		for _, it := range others {
			raw := d.Base + "/s/" + d.Token + "/raw?p=" + url.QueryEscape(it.Path) + "&download=1"
			label := strings.ToUpper(strings.TrimPrefix(it.Kind, "."))
			if it.IsDir {
				label = "DIR"
			}
			if len(label) > 4 {
				label = label[:4]
			}
			b.WriteString("<div class=\"frow\"><div class=\"kind\">" + html.EscapeString(label) + "</div>")
			b.WriteString("<div class=\"nm\">" + html.EscapeString(it.Name) + "</div>")
			size := util.HumanSize(it.Size)
			if it.Size < 0 {
				size = "已失效"
			}
			b.WriteString("<div class=\"sz\">" + size + "</div>")
			if d.AllowDL {
				b.WriteString("<a class=\"btn\" href=\"" + html.EscapeString(raw) + "\">下载</a>")
			}
			b.WriteString("</div>")
		}
		b.WriteString("</section>")
	}
	if len(d.Items) == 0 {
		b.WriteString("<section class=\"files\"><div class=\"frow\"><div class=\"nm\">分享内容为空或已失效</div></div></section>")
	}
	b.WriteString("</main>")
	return b.String()
}

func totalSize(items []Item) int64 {
	var n int64
	for _, it := range items {
		if it.Size > 0 {
			n += it.Size
		}
	}
	return n
}

// KindLabel 返回中文类型名。
func KindLabel(kind string) string {
	switch kind {
	case "image":
		return "图片"
	case "video":
		return "视频"
	case "audio":
		return "音频"
	case "archive":
		return "压缩包"
	case "doc":
		return "文档"
	case "folder":
		return "文件夹"
	}
	return "文件"
}

// ShareURL 拼出分享页地址。
func ShareURL(base, token string) string {
	return strings.TrimRight(base, "/") + "/s/" + token
}

const shareJS = "function openLb(u,n){var i=document.getElementById(\"lbImg\");i.src=u;i.alt=n||\"\";document.getElementById(\"lbDl\").href=u+\"&download=1\";document.getElementById(\"lb\").classList.add(\"on\");}" +
	"function closeLb(){document.getElementById(\"lb\").classList.remove(\"on\");document.getElementById(\"lbImg\").src=\"\";}" +
	"function toast(m){var t=document.getElementById(\"toast\");t.textContent=m;t.classList.add(\"on\");setTimeout(function(){t.classList.remove(\"on\");},1800);}" +
	"function copyLink(){if(navigator.clipboard){navigator.clipboard.writeText(location.href).then(function(){toast(\"链接已复制\");});}else{toast(location.href);}}" +
	"document.addEventListener(\"click\",function(e){var c=e.target.closest(\".cell\");if(c&&c.dataset.img){openLb(c.dataset.img,c.dataset.name);}});" +
	"document.addEventListener(\"keydown\",function(e){if(e.key===\"Escape\"){closeLb();}});"
