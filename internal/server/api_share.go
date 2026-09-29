package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ycyingchen/ycfmg/internal/media"
	"github.com/ycyingchen/ycfmg/internal/share"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
)

const shareCookie = "ycfmg_share"

func (s *Server) shareSecret() string {
	sec := s.st.GetSetting("share_secret", "")
	if sec == "" {
		sec = hex.EncodeToString([]byte(utilToken(32)))
		_ = s.st.SetSetting("share_secret", sec)
	}
	return sec
}

func (s *Server) shareSignature(token string) string {
	h := hmac.New(sha256.New, []byte(s.shareSecret()))
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (s *Server) sharePassed(r *http.Request, sh *store.Share) bool {
	if sh.PasswordHash == "" {
		return true
	}
	c, err := r.Cookie(shareCookie)
	if err != nil {
		return false
	}
	parts := strings.SplitN(c.Value, ":", 2)
	if len(parts) != 2 {
		return false
	}
	return parts[0] == sh.Token && parts[1] == s.shareSignature(sh.Token)
}

func (s *Server) loadShare(w http.ResponseWriter, r *http.Request) *store.Share {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(token)
	if err != nil || sh == nil {
		s.renderShareError(w, r, "分享不存在", "该分享链接无效或已被删除")
		return nil
	}
	if err := s.sh.Check(sh, ""); err != nil && err != share.ErrNeedPassword {
		if !s.sharePassed(r, sh) {
			s.renderShareError(w, r, "分享不可用", err.Error())
			return nil
		}
	}
	return sh
}

func (s *Server) renderShareError(w http.ResponseWriter, r *http.Request, title, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	body := share.RenderPage(share.PageData{
		Brand: s.cfg.Share.BrandName, Subtitle: s.cfg.Share.BrandSubtitle,
		Title: title, Descr: msg, Base: s.sh.BaseURL(r), Token: "", Items: nil,
	})
	_, _ = w.Write([]byte(body))
}

func (s *Server) hSharePage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(token)
	if err != nil || sh == nil {
		s.renderShareError(w, r, "分享不存在", "该分享链接无效或已被删除")
		return
	}
	if sh.Disabled {
		s.renderShareError(w, r, "分享已停用", "分享者已关闭该分享")
		return
	}
	if sh.ExpireAt > 0 && sh.ExpireAt < nowUnix() {
		s.renderShareError(w, r, "分享已过期", "请联系分享者重新生成链接")
		return
	}
	passed := s.sharePassed(r, sh)
	data := share.PageData{
		Brand: s.cfg.Share.BrandName, Subtitle: s.cfg.Share.BrandSubtitle,
		Title: sh.Title, Descr: sh.Descr, Token: sh.Token, Base: s.sh.BaseURL(r),
		ViewCount: sh.ViewCount, ExpiresAt: sh.ExpireAt, CreatedAt: sh.CreatedAt,
		Creator: sh.Creator, ShowWater: sh.ShowWatermark,
		AllowDL: sh.AllowDownload && s.cfg.Share.AllowDownload,
	}
	if !passed {
		data.NeedPass = true
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(share.RenderPage(data)))
		return
	}
	data.Items = s.sh.ParseItems(sh)
	_ = s.st.IncShareView(sh.ID)
	_ = s.st.AddVisit(sh.ID, clientIP(r, s.cfg.Server.TrustProxy), r.UserAgent(), "view")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(share.RenderPage(data)))
}

func (s *Server) hShareAuth(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	sh, err := s.st.GetShareByToken(token)
	if err != nil || sh == nil {
		s.renderShareError(w, r, "分享不存在", "该分享链接无效或已被删除")
		return
	}
	_ = r.ParseForm()
	pwd := r.FormValue("password")
	if err := s.sh.Check(sh, pwd); err != nil {
		if err == share.ErrNeedPassword {
			err = nil
		}
		data := share.PageData{
			Brand: s.cfg.Share.BrandName, Subtitle: s.cfg.Share.BrandSubtitle,
			Title: sh.Title, Token: sh.Token, Base: s.sh.BaseURL(r),
			NeedPass: true, Message: "密码错误，请重试", ViewCount: sh.ViewCount, ExpiresAt: sh.ExpireAt,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(share.RenderPage(data)))
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: shareCookie, Value: token + ":" + s.shareSignature(token),
		Path: s.shareCookiePath(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 86400 * 7,
	})
	http.Redirect(w, r, s.cfg.Server.BasePath+"/s/"+token, http.StatusSeeOther)
}

func (s *Server) shareCookiePath() string {
	if s.cfg.Server.BasePath == "" {
		return "/s"
	}
	return s.cfg.Server.BasePath + "/s"
}

func (s *Server) shareItem(sh *store.Share, p string) (share.Item, bool) {
	items := s.sh.ParseItems(sh)
	clean := filepath.Clean(p)
	for _, it := range items {
		if filepath.Clean(it.Path) == clean {
			return it, true
		}
		if it.IsDir && strings.HasPrefix(clean, filepath.Clean(it.Path)+string(os.PathSeparator)) {
			return it, true
		}
	}
	return share.Item{}, false
}

func (s *Server) hShareThumb(w http.ResponseWriter, r *http.Request) {
	sh, err := s.st.GetShareByToken(r.PathValue("token"))
	if err != nil || sh == nil || sh.Disabled || !s.sharePassed(r, sh) {
		http.Error(w, "无权访问", http.StatusForbidden)
		return
	}
	p := r.URL.Query().Get("p")
	if _, ok := s.shareItem(sh, p); !ok {
		http.Error(w, "条目不在分享范围内", http.StatusForbidden)
		return
	}
	st, err := os.Stat(p)
	if err != nil {
		s.placeholder(w, r)
		return
	}
	size := thumbs.ParseSize(r.URL.Query().Get("size"), 512)
	thumb, err := s.th.Get(p, size, st.ModTime().Unix())
	if err != nil {
		s.placeholder(w, r)
		return
	}
	media.ServeThumb(w, r, thumb)
}

func (s *Server) hShareRaw(w http.ResponseWriter, r *http.Request) {
	sh, err := s.st.GetShareByToken(r.PathValue("token"))
	if err != nil || sh == nil || sh.Disabled || !s.sharePassed(r, sh) {
		http.Error(w, "无权访问", http.StatusForbidden)
		return
	}
	p := r.URL.Query().Get("p")
	it, ok := s.shareItem(sh, p)
	if !ok {
		http.Error(w, "条目不在分享范围内", http.StatusForbidden)
		return
	}
	if it.IsDir {
		http.Error(w, "目录请使用打包下载", http.StatusBadRequest)
		return
	}
	if !sh.AllowDownload && r.URL.Query().Get("download") == "1" {
		http.Error(w, "分享者已关闭下载", http.StatusForbidden)
		return
	}
	if r.URL.Query().Get("download") == "1" {
		if sh.MaxDownload > 0 && sh.DownloadCount >= sh.MaxDownload {
			http.Error(w, "下载次数已达上限", http.StatusForbidden)
			return
		}
		_, _ = s.st.IncShareDownload(sh.ID)
		_ = s.st.AddVisit(sh.ID, clientIP(r, s.cfg.Server.TrustProxy), r.UserAgent(), "download")
	}
	media.Serve(w, r, p, filepath.Base(p))
}

func (s *Server) hShareZip(w http.ResponseWriter, r *http.Request) {
	sh, err := s.st.GetShareByToken(r.PathValue("token"))
	if err != nil || sh == nil || sh.Disabled || !s.sharePassed(r, sh) {
		http.Error(w, "无权访问", http.StatusForbidden)
		return
	}
	if !sh.AllowDownload || !s.cfg.Share.AllowDownload {
		http.Error(w, "分享者已关闭下载", http.StatusForbidden)
		return
	}
	if sh.MaxDownload > 0 && sh.DownloadCount >= sh.MaxDownload {
		http.Error(w, "下载次数已达上限", http.StatusForbidden)
		return
	}
	items := s.sh.ParseItems(sh)
	paths := make([]string, 0, len(items))
	for _, it := range items {
		paths = append(paths, it.Path)
	}
	_, _ = s.st.IncShareDownload(sh.ID)
	_ = s.st.AddVisit(sh.ID, clientIP(r, s.cfg.Server.TrustProxy), r.UserAgent(), "zip")
	name := sh.Title
	if name == "" {
		name = "ycfmg-share"
	}
	if err := media.Zip(w, r, paths, name); err != nil {
		return
	}
}

func (s *Server) hShareList(w http.ResponseWriter, r *http.Request) {
	list, total, err := s.st.ListShares(r.URL.Query().Get("q"), queryInt(r, "limit", 50), queryInt(r, "offset", 0))
	if err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	out := []map[string]any{}
	for i := range list {
		out = append(out, s.shareJSON(r, &list[i]))
	}
	okData(w, map[string]any{"items": out, "total": total, "overview": s.st.ShareOverview()})
}

func (s *Server) shareJSON(r *http.Request, sh *store.Share) map[string]any {
	items := s.sh.ParseItems(sh)
	imgs := 0
	var size int64
	for _, it := range items {
		if it.Kind == "image" {
			imgs++
		}
		if it.Size > 0 {
			size += it.Size
		}
	}
	links := s.sh.Links(r, sh.Token)
	return map[string]any{
		"id": sh.ID, "token": sh.Token, "title": sh.Title, "descr": sh.Descr, "kind": sh.Kind,
		"expire_at": sh.ExpireAt, "max_download": sh.MaxDownload, "download_count": sh.DownloadCount,
		"view_count": sh.ViewCount, "allow_download": sh.AllowDownload, "show_watermark": sh.ShowWatermark,
		"created_at": sh.CreatedAt, "creator": sh.Creator, "disabled": sh.Disabled,
		"has_password": sh.HasPassword, "expired": sh.Expired, "count": len(items), "images": imgs,
		"size": size, "url": links[0].URL, "links": links, "items": items,
	}
}

func (s *Server) hShareGet(w http.ResponseWriter, r *http.Request) {
	sh, err := s.st.GetShareByID(int64(queryInt(r, "id", 0)))
	if err != nil {
		if v := r.PathValue("id"); v != "" {
			var id int64
			for _, c := range v {
				if c < 0x30 || c > 0x39 {
					id = 0
					break
				}
				id = id*10 + int64(c-0x30)
			}
			sh, err = s.st.GetShareByID(id)
		}
	}
	if err != nil || sh == nil {
		fail(w, http.StatusNotFound, "分享不存在")
		return
	}
	okData(w, s.shareJSON(r, sh))
}

func (s *Server) hShareCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title         string   `json:"title"`
		Descr         string   `json:"descr"`
		Paths         []string `json:"paths"`
		Kind          string   `json:"kind"`
		Password      string   `json:"password"`
		ExpireDays    int      `json:"expire_days"`
		MaxDownload   int      `json:"max_download"`
		AllowDownload *bool    `json:"allow_download"`
		ShowWatermark bool     `json:"show_watermark"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	allow := s.cfg.Share.AllowDownload
	if req.AllowDownload != nil {
		allow = *req.AllowDownload
	}
	creator := ""
	if sess := s.au.Current(r); sess != nil {
		creator = sess.Username
	}
	sh, err := s.sh.Create(share.CreateRequest{
		Title: req.Title, Descr: req.Descr, Paths: req.Paths, Kind: req.Kind,
		Password: req.Password, ExpireDays: req.ExpireDays, MaxDownload: req.MaxDownload,
		AllowDownload: allow, ShowWatermark: req.ShowWatermark, Creator: creator,
	}, r)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, s.shareJSON(r, sh))
}

func (s *Server) hShareUpdate(w http.ResponseWriter, r *http.Request) {
	id := pathInt(r.PathValue("id"))
	sh, err := s.st.GetShareByID(id)
	if err != nil {
		fail(w, http.StatusNotFound, "分享不存在")
		return
	}
	var req struct {
		Title         *string `json:"title"`
		Descr         *string `json:"descr"`
		Password      *string `json:"password"`
		ExpireDays    *int    `json:"expire_days"`
		MaxDownload   *int    `json:"max_download"`
		AllowDownload *bool   `json:"allow_download"`
		ShowWatermark *bool   `json:"show_watermark"`
		Disabled      *bool   `json:"disabled"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	fields := map[string]any{}
	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Descr != nil {
		fields["descr"] = *req.Descr
	}
	if req.Password != nil {
		if *req.Password == "" {
			fields["password_hash"] = ""
		} else {
			if h, err := hashPassword(*req.Password); err == nil {
				fields["password_hash"] = h
			}
		}
	}
	if req.ExpireDays != nil {
		if *req.ExpireDays <= 0 {
			fields["expire_at"] = 0
		} else {
			fields["expire_at"] = nowUnix() + int64(*req.ExpireDays)*86400
		}
	}
	if req.MaxDownload != nil {
		fields["max_download"] = *req.MaxDownload
	}
	if req.AllowDownload != nil {
		fields["allow_download"] = *req.AllowDownload
	}
	if req.ShowWatermark != nil {
		fields["show_watermark"] = *req.ShowWatermark
	}
	if req.Disabled != nil {
		fields["disabled"] = *req.Disabled
	}
	if err := s.st.UpdateShare(sh.ID, fields); err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	updated, _ := s.st.GetShareByID(sh.ID)
	okData(w, s.shareJSON(r, updated))
}

func (s *Server) hShareDelete(w http.ResponseWriter, r *http.Request) {
	id := pathInt(r.PathValue("id"))
	if err := s.st.DeleteShare(id); err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	okData(w, map[string]any{"deleted": id})
}

func (s *Server) hShareLinks(w http.ResponseWriter, r *http.Request) {
	id := pathInt(r.PathValue("id"))
	sh, err := s.st.GetShareByID(id)
	if err != nil {
		fail(w, http.StatusNotFound, "分享不存在")
		return
	}
	okData(w, map[string]any{
		"token":  sh.Token,
		"links":  s.sh.Links(r, sh.Token),
		"visits": s.st.ListVisits(sh.ID, 50),
	})
}
