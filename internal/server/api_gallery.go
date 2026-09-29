package server

import (
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/ycyingchen/ycfmg/internal/search"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/util"
	"github.com/ycyingchen/ycfmg/internal/vision"
)

func (s *Server) galleryOptions(r *http.Request) search.Options {
	q := r.URL.Query()
	return search.Options{
		Q:         strings.TrimSpace(q.Get("q")),
		Lib:       q.Get("lib"),
		Kind:      q.Get("kind"),
		Color:     q.Get("color"),
		Classify:  q.Get("classify"),
		Tag:       q.Get("tag"),
		Camera:    q.Get("camera"),
		Favorite:  queryBool(r, "favorite"),
		MinRating: queryInt(r, "rating", 0),
		From:      int64(queryInt(r, "from", 0)),
		To:        int64(queryInt(r, "to", 0)),
		MinWidth:  queryInt(r, "min_w", 0),
		MinHeight: queryInt(r, "min_h", 0),
		Sort:      q.Get("sort"),
		Desc:      queryBool(r, "desc"),
		Limit:     queryInt(r, "limit", 60),
		Offset:    queryInt(r, "offset", 0),
	}
}

func (s *Server) hGallery(w http.ResponseWriter, r *http.Request) {
	opt := s.galleryOptions(r)
	if opt.Kind == "" {
		opt.Kind = "image"
	}
	if q := r.URL.Query().Get("hidden"); q == "1" {
	}
	hits, total, err := s.eng.TextSearch(opt)
	if err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	items := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		items = append(items, fileToJSON(&h.File, h.Score, h.Reason))
	}
	okData(w, map[string]any{"items": items, "total": total, "offset": opt.Offset, "limit": opt.Limit})
}

func fileToJSON(f *store.File, score float64, reason string) map[string]any {
	ext := f.Ext
	return map[string]any{
		"id": f.ID, "name": f.Name, "path": f.Path, "parent": f.Parent, "lib": f.Lib,
		"ext": ext, "kind": f.Kind, "size": f.Size, "mtime": f.MTime, "taken_at": f.TakenAt,
		"width": f.Width, "height": f.Height, "aspect": f.Aspect, "color": f.Color,
		"classify": f.Classify, "tags": splitTags(f.Tags), "camera": f.Camera, "lens": f.Lens,
		"iso": f.ISO, "fnum": f.FNum, "exposure": f.Exposure, "focal": f.Focal,
		"gps_lat": f.GPSLat, "gps_lon": f.GPSLon, "favorite": f.Favorite, "rating": f.Rating,
		"hidden": f.Hidden, "note": f.Note, "missing": f.Missing,
		"score": score, "reason": reason,
		"size_text": util.HumanSize(f.Size),
	}
}

func splitTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Server) hFacets(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "image"
	}
	fields := []string{"year", "month", "classify", "color", "camera", "lib", "ext", "lens"}
	out := map[string]any{}
	for _, f := range fields {
		out[f] = s.st.Facets(f, kind, 120)
	}
	okData(w, out)
}

func (s *Server) hMapPoints(w http.ResponseWriter, r *http.Request) {
	files, _, err := s.st.QueryFiles(store.FileQuery{
		Kind:   "image",
		Limit:  2000,
		Sort:   "taken",
		Desc:   true,
		Hidden: nil,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	pts := []map[string]any{}
	for i := range files {
		f := files[i]
		if f.GPSLat == 0 && f.GPSLon == 0 {
			continue
		}
		pts = append(pts, map[string]any{
			"id": f.ID, "name": f.Name, "path": f.Path, "lat": f.GPSLat, "lon": f.GPSLon,
			"taken_at": f.TakenAt, "classify": f.Classify,
		})
	}
	okData(w, map[string]any{"points": pts})
}

func (s *Server) hDuplicates(w http.ResponseWriter, r *http.Request) {
	lib := r.URL.Query().Get("lib")
	tol := queryInt(r, "tolerance", s.cfg.Index.HashTolerance)
	groups := s.eng.DuplicateGroups(lib, tol)
	out := []map[string]any{}
	for _, g := range groups {
		items := []map[string]any{}
		var total int64
		for i := range g {
			items = append(items, fileToJSON(&g[i], 0, ""))
			total += g[i].Size
		}
		out = append(out, map[string]any{"count": len(items), "size": total, "size_text": util.HumanSize(total), "items": items})
	}
	okData(w, map[string]any{"groups": out, "total": len(out)})
}

func (s *Server) hSearch(w http.ResponseWriter, r *http.Request) {
	opt := s.galleryOptions(r)
	hits, total, err := s.eng.TextSearch(opt)
	if err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	items := []map[string]any{}
	for _, h := range hits {
		items = append(items, fileToJSON(&h.File, h.Score, h.Reason))
	}
	mode := "text"
	if s.eng.Semantic().Enabled() && opt.Q != "" {
		mode = "text+semantic-available"
	}
	okData(w, map[string]any{"items": items, "total": total, "mode": mode, "query": opt.Q})
}

func (s *Server) hSearchImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		fail(w, http.StatusBadRequest, "请以上传图片的方式提交")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "缺少图片文件")
		return
	}
	defer file.Close()
	buf := make([]byte, 0, 1<<20)
	tmp := make([]byte, 64<<10)
	for {
		n, rerr := file.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if rerr != nil {
			break
		}
		if len(buf) > 24<<20 {
			break
		}
	}
	opt := s.galleryOptions(r)
	opt.MinScore = 0.45
	hits, total, err := s.eng.SimilarByImageData(buf, opt)
	if err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}
	items := []map[string]any{}
	for _, h := range hits {
		items = append(items, fileToJSON(&h.File, h.Score, h.Reason))
	}
	okData(w, map[string]any{"items": items, "total": total, "mode": "image"})
}

func (s *Server) hSimilar(w http.ResponseWriter, r *http.Request) {
	id := int64(queryInt(r, "id", 0))
	if id <= 0 {
		fail(w, http.StatusBadRequest, "缺少 id")
		return
	}
	opt := s.galleryOptions(r)
	opt.MinScore = float64(queryInt(r, "min_score", 45)) / 100.0
	hits, total, err := s.eng.SimilarById(id, opt)
	if err != nil {
		fail(w, http.StatusInternalServerError, "%v", err)
		return
	}
	items := []map[string]any{}
	for _, h := range hits {
		items = append(items, fileToJSON(&h.File, h.Score, h.Reason))
	}
	okData(w, map[string]any{"items": items, "total": total, "mode": "similar"})
}

func (s *Server) hSuggest(w http.ResponseWriter, r *http.Request) {
	classes := s.st.Facets("classify", "image", 30)
	cams := s.st.Facets("camera", "image", 20)
	years := s.st.Facets("year", "image", 20)
	tags := s.st.ListTags(nil)
	tagNames := []string{}
	for i, t := range tags {
		if i >= 40 {
			break
		}
		tagNames = append(tagNames, t.Name)
	}
	colors := []string{}
	for _, f := range s.st.Facets("color", "image", 200) {
		if n := vision.ColorName(f.Key); n != "" {
			colors = append(colors, n)
		}
	}
	sort.Strings(colors)
	okData(w, map[string]any{
		"classify": classes, "camera": cams, "year": years,
		"tags": tagNames, "colors": dedupeStrings(colors),
		"keywords": baseSuggestions,
	})
}

var baseSuggestions = []string{
	"风景", "人物", "美食", "夜景", "截图", "文档", "动漫", "表情包",
	"长截图", "黑白", "全景", "高清", "竖图", "横图", "收藏", "最近一周",
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func (s *Server) hFileMeta(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64    `json:"id"`
		Path     string   `json:"path"`
		Favorite *bool    `json:"favorite"`
		Rating   *int     `json:"rating"`
		Hidden   *bool    `json:"hidden"`
		Note     *string  `json:"note"`
		AddTags  []string `json:"add_tags"`
		DelTags  []string `json:"del_tags"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	var rec *store.File
	var err error
	if req.ID > 0 {
		rec, err = s.st.GetFileByID(req.ID)
	} else if req.Path != "" {
		rec, err = s.st.GetFileByPath(req.Path)
	} else {
		fail(w, http.StatusBadRequest, "缺少 id 或 path")
		return
	}
	if err != nil || rec == nil {
		fail(w, http.StatusNotFound, "记录不存在")
		return
	}
	fields := map[string]any{}
	if req.Favorite != nil {
		fields["favorite"] = *req.Favorite
	}
	if req.Rating != nil {
		fields["rating"] = *req.Rating
	}
	if req.Hidden != nil {
		fields["hidden"] = *req.Hidden
	}
	if req.Note != nil {
		fields["note"] = *req.Note
	}
	if len(fields) > 0 {
		if err := s.st.UpdateFileMeta(rec.ID, fields); err != nil {
			fail(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	for _, t := range req.AddTags {
		if t = strings.TrimSpace(t); t != "" {
			_ = s.st.AddFileTag(rec.ID, t, vision.DefaultTagColor(t), false)
		}
	}
	for _, t := range req.DelTags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		for _, tag := range s.st.FileTags(rec.ID) {
			if tag.Name == t {
				_ = s.st.UntagFile(rec.ID, tag.ID)
			}
		}
	}
	_ = s.st.RebuildTagCounts()
	if updated, err := s.st.GetFileByID(rec.ID); err == nil {
		js := fileToJSON(updated, 0, "")
		js["tags_full"] = s.st.FileTags(updated.ID)
		okData(w, js)
		return
	}
	okData(w, map[string]any{"id": rec.ID})
}

func (s *Server) hTags(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{"tags": s.st.ListTags(nil)})
}

var _ = os.Stat
