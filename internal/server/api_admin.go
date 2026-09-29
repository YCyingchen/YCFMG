package server

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ycyingchen/ycfmg/internal/auth"
	"github.com/ycyingchen/ycfmg/internal/util"
	"github.com/ycyingchen/ycfmg/internal/version"
)

func utilToken(n int) string                  { return util.Token(n) }
func nowUnix() int64                          { return time.Now().Unix() }
func clientIP(r *http.Request, t bool) string { return auth.ClientIP(r, t) }
func hashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func pathInt(s string) int64 {
	var n int64
	for _, c := range s {
		if c < 0x30 || c > 0x39 {
			return 0
		}
		n = n*10 + int64(c-0x30)
	}
	return n
}

func (s *Server) hHealth(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{
		"name": "YCFMG", "version": s.Version(), "uptime": int(time.Since(s.started).Seconds()),
		"time": nowUnix(),
	})
}

// gatewayBase 返回前端应使用的接口前缀：优先取网关注入的挂载前缀。
func gatewayBase(r *http.Request, fallback string) string {
	if v := r.Header.Get("X-YCFMG-Gateway-Prefix"); v != "" {
		return v
	}
	return fallback
}

func (s *Server) hPublic(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{
		"brand":    s.cfg.Share.BrandName,
		"subtitle": s.cfg.Share.BrandSubtitle,
		// 经飞牛统一网关访问时，前端必须知道自己的挂载前缀，
		// 否则 API 会打到站点根目录而 404（页面能开、数据全空）。
		"base_path":        gatewayBase(r, s.cfg.Server.BasePath),
		"need_login":       true,
		"allow_guest":      s.cfg.Auth.AllowGuest,
		"readonly":         s.cfg.Server.Readonly,
		"libraries":        s.fsapi.Dirs(),
		"semantic_enabled": s.eng.Semantic().Enabled(),
		"thumb_sizes":      s.cfg.Index.ThumbSizes,
		"urls":             s.cfg.Server.PublicURLs,
		"current_url":      s.sh.BaseURL(r),
	})
}

func (s *Server) hLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Username == "" {
		req.Username = s.cfg.Auth.Username
	}
	sess, err := s.au.Login(w, r, req.Username, req.Password)
	if err != nil {
		fail(w, http.StatusUnauthorized, "%v", err)
		return
	}
	okData(w, map[string]any{"username": sess.Username, "expire_at": sess.ExpireAt})
}

func (s *Server) hLogout(w http.ResponseWriter, r *http.Request) {
	s.au.Logout(w, r)
	okData(w, map[string]any{"logout": true})
}

func (s *Server) hMe(w http.ResponseWriter, r *http.Request) {
	sess := s.au.Current(r)
	if sess == nil {
		if s.cfg.Auth.AllowGuest {
			okData(w, map[string]any{"username": "访客", "guest": true})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "未登录"})
		return
	}
	okData(w, map[string]any{"username": sess.Username, "expire_at": sess.ExpireAt})
}

func (s *Server) hChangePassword(w http.ResponseWriter, r *http.Request) {
	sess := s.au.Current(r)
	if sess == nil {
		fail(w, http.StatusUnauthorized, "未登录")
		return
	}
	var req struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.au.ChangePassword(sess.Username, req.Old, req.New); err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"changed": true})
}

func (s *Server) hStats(w http.ResponseWriter, r *http.Request) {
	st := s.st.Stats()
	st["cache_size"] = s.th.Size()
	st["libraries"] = len(s.cfg.Libraries)
	st["semantic"] = map[string]any{"enabled": s.eng.Semantic().Enabled(), "model": s.eng.Semantic().Model()}
	st["version"] = s.Version()
	st["progress"] = s.ix.Progress()
	okData(w, st)
}

func (s *Server) hLibraries(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{"libraries": s.ix.LibraryStats()})
}

func (s *Server) hIndexStatus(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{
		"progress": s.ix.Progress(),
		"jobs":     s.st.RecentJobs(10),
		"stats":    s.st.Stats(),
	})
}

func (s *Server) hIndexScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Lib   string `json:"lib"`
		Path  string `json:"path"`
		Force bool   `json:"force"`
	}
	_ = decodeBody(r, &req)
	if p := r.URL.Query().Get("path"); p != "" {
		req.Path = p
	}
	if req.Path != "" {
		n, err := s.ix.IndexDir(req.Path, true)
		if err != nil {
			fail(w, http.StatusBadRequest, "%v", err)
			return
		}
		okData(w, map[string]any{"indexed": n})
		return
	}
	go s.ix.ScanAll(req.Force)
	okData(w, map[string]any{"started": true})
}

func (s *Server) hIndexJobs(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{"jobs": s.st.RecentJobs(30)})
}

func (s *Server) hSettings(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{
		"server": map[string]any{
			"host": s.cfg.Server.Host, "port": s.cfg.Server.Port,
			"base_path": s.cfg.Server.BasePath, "public_urls": s.cfg.Server.PublicURLs,
			"trust_proxy": s.cfg.Server.TrustProxy, "readonly": s.cfg.Server.Readonly,
		},
		"data_dir":  s.cfg.DataDir,
		"libraries": s.cfg.Libraries,
		"index": map[string]any{
			"enabled": s.cfg.Index.Enabled, "scan_interval_minutes": s.cfg.Index.ScanIntervalMinutes,
			"thumb_sizes": s.cfg.Index.ThumbSizes, "auto_tags": s.cfg.Index.AutoTags,
			"hash_tolerance": s.cfg.Index.HashTolerance, "exclude": s.cfg.Index.Exclude,
		},
		"share": map[string]any{
			"default_expire_days": s.cfg.Share.DefaultExpireDays, "max_items": s.cfg.Share.MaxItems,
			"allow_download": s.cfg.Share.AllowDownload, "brand_name": s.cfg.Share.BrandName,
			"brand_subtitle": s.cfg.Share.BrandSubtitle,
		},
		"semantic": map[string]any{
			"enabled": s.eng.Semantic().Enabled(), "model": s.eng.Semantic().Model(),
		},
		"config_path": s.cfg.Path(),
	})
}

func (s *Server) hSettingsSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicURLs []string `json:"public_urls"`
		Brand      string   `json:"brand_name"`
		Subtitle   string   `json:"brand_subtitle"`
		AutoTags   *bool    `json:"auto_tags"`
		Tolerance  *int     `json:"hash_tolerance"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if len(req.PublicURLs) > 0 {
		clean := []string{}
		for _, u := range req.PublicURLs {
			u = strings.TrimRight(strings.TrimSpace(u), "/")
			if u != "" {
				clean = append(clean, u)
			}
		}
		raw := strings.Join(clean, ",")
		if err := s.st.SetSetting("public_urls", raw); err != nil {
			fail(w, http.StatusInternalServerError, "%v", err)
			return
		}
		s.cfg.Server.PublicURLs = clean
	}
	if req.Brand != "" {
		_ = s.st.SetSetting("brand_name", req.Brand)
		s.cfg.Share.BrandName = req.Brand
	}
	if req.Subtitle != "" {
		_ = s.st.SetSetting("brand_subtitle", req.Subtitle)
		s.cfg.Share.BrandSubtitle = req.Subtitle
	}
	if req.AutoTags != nil {
		v := "0"
		if *req.AutoTags {
			v = "1"
		}
		_ = s.st.SetSetting("auto_tags", v)
		s.cfg.Index.AutoTags = *req.AutoTags
	}
	if req.Tolerance != nil && *req.Tolerance > 0 {
		_ = s.st.SetSetting("hash_tolerance", itoaInt(*req.Tolerance))
		s.cfg.Index.HashTolerance = *req.Tolerance
	}
	okData(w, map[string]any{"saved": true})
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := []byte{}
	for n > 0 {
		buf = append([]byte{byte(0x30 + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{0x2D}, buf...)
	}
	return string(buf)
}

func (s *Server) hThumbClean(w http.ResponseWriter, r *http.Request) {
	if err := s.th.Cleanup(); err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	okData(w, map[string]any{"cleaned": true})
}

// Version 返回当前版本号。
func (s *Server) Version() string { return version.Version }

func (s *Server) staticHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.webFS, p); err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				fail(w, http.StatusNotFound, "接口不存在")
				return
			}
			if path.Ext(r.URL.Path) != "" {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/"
		}
		// 页面与脚本一律不缓存：应用升级后浏览器必须立刻拿到新资源，
		// 否则旧 JS 配新 HTML 会直接白屏。图片等静态资源可以长缓存。
		switch {
		case strings.HasSuffix(p, ".html") || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") || strings.HasSuffix(p, ".json"):
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		case strings.HasSuffix(p, ".svg") || strings.HasSuffix(p, ".png") || strings.HasSuffix(p, ".jpg") || strings.HasSuffix(p, ".ico"):
			w.Header().Set("Cache-Control", "public, max-age=604800")
		}
		fileServer.ServeHTTP(w, r)
	})
}

var _ = os.Stat
