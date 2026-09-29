// Package server 组装 HTTP 路由、中间件与各业务处理器。
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/auth"
	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/fsapi"
	"github.com/ycyingchen/ycfmg/internal/indexer"
	"github.com/ycyingchen/ycfmg/internal/logx"
	"github.com/ycyingchen/ycfmg/internal/media"
	"github.com/ycyingchen/ycfmg/internal/search"
	"github.com/ycyingchen/ycfmg/internal/share"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
	"github.com/ycyingchen/ycfmg/internal/util"
)

// Server 聚合全部依赖。
type Server struct {
	cfg     *config.Config
	st      *store.Store
	th      *thumbs.Cache
	ix      *indexer.Indexer
	eng     *search.Engine
	sh      *share.Manager
	au      *auth.Manager
	fsapi   *fsapi.FS
	webFS   fs.FS
	mux     *http.ServeMux
	started time.Time
}

// Options 是构造参数。
type Options struct {
	Config *config.Config
	Store  *store.Store
	Thumbs *thumbs.Cache
	Index  *indexer.Indexer
	Engine *search.Engine
	Share  *share.Manager
	Auth   *auth.Manager
	FS     *fsapi.FS
	WebFS  fs.FS
}

// New 创建服务器。
func New(o Options) *Server {
	s := &Server{
		cfg: o.Config, st: o.Store, th: o.Thumbs, ix: o.Index, eng: o.Engine,
		sh: o.Share, au: o.Auth, fsapi: o.FS, webFS: o.WebFS,
		mux: http.NewServeMux(), started: time.Now(),
	}
	s.routes()
	return s
}

// Handler 返回带基础路径前缀的处理器。
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.logMiddleware(h)
	if bp := s.cfg.Server.BasePath; bp != "" {
		h = http.StripPrefix(bp, h)
	}
	return h
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/health", s.hHealth)
	m.HandleFunc("GET /api/public", s.hPublic)
	m.HandleFunc("POST /api/login", s.hLogin)
	m.HandleFunc("POST /api/logout", s.hLogout)
	m.HandleFunc("GET /api/me", s.hMe)
	m.HandleFunc("POST /api/password", s.hChangePassword)

	m.HandleFunc("GET /s/{token}", s.hSharePage)
	m.HandleFunc("POST /s/{token}", s.hShareAuth)
	m.HandleFunc("GET /s/{token}/thumb", s.hShareThumb)
	m.HandleFunc("GET /s/{token}/raw", s.hShareRaw)
	m.HandleFunc("GET /s/{token}/zip", s.hShareZip)

	m.HandleFunc("GET /api/fs/list", s.auth(s.hFSList))
	m.HandleFunc("GET /api/fs/tree", s.auth(s.hFSTree))
	m.HandleFunc("POST /api/fs/mkdir", s.auth(s.hFSMkdir))
	m.HandleFunc("POST /api/fs/rename", s.auth(s.hFSRename))
	m.HandleFunc("POST /api/fs/move", s.auth(s.hFSMove))
	m.HandleFunc("POST /api/fs/copy", s.auth(s.hFSCopy))
	m.HandleFunc("POST /api/fs/delete", s.auth(s.hFSDelete))
	m.HandleFunc("POST /api/fs/upload", s.auth(s.hFSUpload))
	m.HandleFunc("GET /api/fs/trash", s.auth(s.hFSTrashList))
	m.HandleFunc("POST /api/fs/trash/restore", s.auth(s.hFSTrashRestore))
	m.HandleFunc("POST /api/fs/trash/purge", s.auth(s.hFSTrashPurge))
	m.HandleFunc("GET /api/fs/index", s.auth(s.hFSIndexNow))

	m.HandleFunc("GET /api/file", s.allowGuest(s.hFileRaw))
	m.HandleFunc("GET /api/thumb", s.allowGuest(s.hThumb))

	m.HandleFunc("GET /api/gallery", s.auth(s.hGallery))
	m.HandleFunc("GET /api/gallery/facets", s.auth(s.hFacets))
	m.HandleFunc("GET /api/gallery/map", s.auth(s.hMapPoints))
	m.HandleFunc("GET /api/gallery/duplicates", s.auth(s.hDuplicates))
	m.HandleFunc("GET /api/search", s.auth(s.hSearch))
	m.HandleFunc("POST /api/search/image", s.auth(s.hSearchImage))
	m.HandleFunc("GET /api/search/similar", s.auth(s.hSimilar))
	m.HandleFunc("GET /api/search/suggest", s.auth(s.hSuggest))
	m.HandleFunc("POST /api/file/meta", s.auth(s.hFileMeta))
	m.HandleFunc("GET /api/tags", s.auth(s.hTags))

	m.HandleFunc("GET /api/shares", s.auth(s.hShareList))
	m.HandleFunc("POST /api/shares", s.auth(s.hShareCreate))
	m.HandleFunc("GET /api/shares/{id}", s.auth(s.hShareGet))
	m.HandleFunc("PATCH /api/shares/{id}", s.auth(s.hShareUpdate))
	m.HandleFunc("DELETE /api/shares/{id}", s.auth(s.hShareDelete))
	m.HandleFunc("GET /api/shares/{id}/links", s.auth(s.hShareLinks))

	m.HandleFunc("GET /api/update", s.auth(s.hUpdateCheck))
	m.HandleFunc("POST /api/update/check", s.auth(s.hUpdateCheck))
	m.HandleFunc("GET /api/update/config", s.auth(s.hUpdateConfig))
	m.HandleFunc("POST /api/update/config", s.auth(s.hUpdateConfigSave))

	m.HandleFunc("GET /api/stats", s.auth(s.hStats))
	m.HandleFunc("GET /api/libraries", s.auth(s.hLibraries))
	m.HandleFunc("GET /api/index/status", s.auth(s.hIndexStatus))
	m.HandleFunc("POST /api/index/scan", s.auth(s.hIndexScan))
	m.HandleFunc("GET /api/index/jobs", s.auth(s.hIndexJobs))
	m.HandleFunc("GET /api/settings", s.auth(s.hSettings))
	m.HandleFunc("POST /api/settings", s.auth(s.hSettingsSave))
	m.HandleFunc("POST /api/thumbs/clean", s.auth(s.hThumbClean))

	if s.webFS != nil {
		m.Handle("/", s.staticHandler())
	}
}

// --- 中间件 ---

func (s *Server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 免登录模式：fpk 单机场景默认开启，按配置直接放行
		if s.cfg.Auth.AllowGuest {
			h(w, r)
			return
		}
		if s.au.Current(r) == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "未登录或登录已过期"})
			return
		}
		h(w, r)
	}
}

func (s *Server) allowGuest(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.au.Current(r) != nil || s.cfg.Auth.AllowGuest {
			h(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "未登录"})
	}
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && sw.status >= 400 {
			logx.Debugf("%s %s -> %d (%s) %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond), auth.ClientIP(r, s.cfg.Server.TrustProxy))
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// --- 通用辅助 ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func ok(w http.ResponseWriter, v any) { writeJSON(w, http.StatusOK, v) }

func fail(w http.ResponseWriter, code int, format string, args ...any) {
	writeJSON(w, code, map[string]any{"ok": false, "error": fmt.Sprintf(format, args...)})
}

func okData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func decodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 4<<20))
	return dec.Decode(v)
}

func queryInt(r *http.Request, key string, def int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c < 0x30 || c > 0x39 {
			return def
		}
		n = n*10 + int(c-0x30)
		if n > 1<<30 {
			return def
		}
	}
	return n
}

func queryBool(r *http.Request, key string) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

var _ = media.ContentDisposition
var _ = util.HumanSize
