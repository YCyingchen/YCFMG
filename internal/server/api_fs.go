package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ycyingchen/ycfmg/internal/media"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
	"github.com/ycyingchen/ycfmg/internal/util"
)

func (s *Server) hFSList(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if strings.TrimSpace(p) == "" {
		libs := s.cfg.Libraries
		if len(libs) > 0 {
			p = libs[0].Path
		}
	}
	entries, err := s.fsapi.List(p, queryBool(r, "hidden"))
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	abs, _ := filepath.Abs(p)
	parent := filepath.Dir(abs)
	if _, ok := s.cfg.LibraryByPath(parent); !ok {
		parent = ""
	}
	okData(w, map[string]any{
		"path":     abs,
		"parent":   parent,
		"entries":  entries,
		"writable": s.cfg.Writable(abs),
		"crumbs":   crumbs(abs),
	})
	s.ix.RequestScan(abs)
}

func crumbs(p string) []map[string]string {
	out := []map[string]string{}
	if p == "" {
		return out
	}
	parts := strings.Split(strings.Trim(p, "/"), "/")
	cur := ""
	for _, part := range parts {
		cur += "/" + part
		out = append(out, map[string]string{"name": part, "path": cur})
	}
	return out
}

func (s *Server) hFSTree(w http.ResponseWriter, r *http.Request) {
	type node struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Lib  bool   `json:"lib"`
		Kids []node `json:"kids,omitempty"`
	}
	libs := []node{}
	for _, l := range s.cfg.Libraries {
		n := node{Name: l.Name, Path: l.Path, Lib: true}
		if entries, err := os.ReadDir(l.Path); err == nil {
			count := 0
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				n.Kids = append(n.Kids, node{Name: e.Name(), Path: filepath.Join(l.Path, e.Name())})
				count++
				if count >= 60 {
					break
				}
			}
		}
		libs = append(libs, n)
	}
	okData(w, map[string]any{"libraries": libs, "home": s.cfg.DataDir})
}

func (s *Server) hFSMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	target, err := s.fsapi.Mkdir(req.Path, req.Name)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"path": target})
}

func (s *Server) hFSRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		NewName string `json:"new_name"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	target, err := s.fsapi.Rename(req.Path, req.NewName)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"path": target})
}

func (s *Server) hFSMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
		Dst   string   `json:"dst"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	n, err := s.fsapi.Move(req.Paths, req.Dst)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"moved": n})
}

func (s *Server) hFSCopy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
		Dst   string   `json:"dst"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	n, err := s.fsapi.Copy(req.Paths, req.Dst)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"copied": n})
}

func (s *Server) hFSDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths   []string `json:"paths"`
		Forever bool     `json:"forever"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	n, err := s.fsapi.Delete(req.Paths, !req.Forever)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"deleted": n})
}

func (s *Server) hFSUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			fail(w, http.StatusBadRequest, "上传解析失败")
			return
		}
	}
	dir := r.FormValue("path")
	if dir == "" {
		dir = r.URL.Query().Get("path")
	}
	uploadID := r.FormValue("upload_id")
	index := queryInt(r, "chunk", 0)
	total := queryInt(r, "total", 0)
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "缺少文件字段")
		return
	}
	defer file.Close()
	if uploadID != "" && total > 0 {
		target, done, err := s.fsapi.SaveChunk(dir, header.Filename, uploadID, index, total, file)
		if err != nil {
			fail(w, http.StatusBadRequest, "%v", err)
			return
		}
		okData(w, map[string]any{"path": target, "done": done, "chunk": index})
		return
	}
	target, err := s.fsapi.Save(dir, header.Filename, file)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	if util.IsImage(target) {
		go func(p string) { _, _ = s.ix.IndexFile(p) }(target)
	}
	okData(w, map[string]any{"path": target})
}

func (s *Server) hFSTrashList(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{"items": s.fsapi.TrashList()})
}

func (s *Server) hFSTrashRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if err := s.fsapi.TrashRestore(req.ID); err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"restored": true})
}

func (s *Server) hFSTrashPurge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	_ = decodeBody(r, &req)
	if err := s.fsapi.TrashPurge(req.IDs); err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"purged": true})
}

func (s *Server) hFSIndexNow(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		fail(w, http.StatusBadRequest, "缺少 path")
		return
	}
	n, err := s.ix.IndexDir(p, queryBool(r, "recursive"))
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	okData(w, map[string]any{"indexed": n})
}

func (s *Server) hFileRaw(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	abs, err := s.fsapi.Guard(p, false)
	if err != nil {
		fail(w, http.StatusForbidden, "%v", err)
		return
	}
	media.Serve(w, r, abs, filepath.Base(abs))
}

func (s *Server) hThumb(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	abs, err := s.fsapi.Guard(p, false)
	if err != nil {
		fail(w, http.StatusForbidden, "%v", err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		fail(w, http.StatusNotFound, "文件不存在")
		return
	}
	size := thumbs.ParseSize(r.URL.Query().Get("size"), 256)
	thumb, err := s.th.Get(abs, size, info.ModTime().Unix())
	if err != nil {
		s.placeholder(w, r)
		return
	}
	media.ServeThumb(w, r, thumb)
}

func (s *Server) placeholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 64 64\"><rect width=\"64\" height=\"64\" fill=\"#e5e7eb\"/><path d=\"M14 44l12-16 9 11 6-7 9 12z\" fill=\"#9aa3b2\"/><circle cx=\"42\" cy=\"22\" r=\"5\" fill=\"#9aa3b2\"/></svg>"))
}
