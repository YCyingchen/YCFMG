package search

import (
	"bytes"
	"fmt"
	"image"
	"sort"
	"strings"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/vision"
)

// Engine 是检索引擎。
type Engine struct {
	cfg *config.Config
	st  *store.Store
	sem *Semantic
}

// New 创建检索引擎。
func New(cfg *config.Config, st *store.Store) *Engine {
	return &Engine{cfg: cfg, st: st, sem: NewSemantic(cfg.Semantic)}
}

// Semantic 返回语义检索适配器。
func (e *Engine) Semantic() *Semantic { return e.sem }

// Hit 是一条检索结果。
type Hit struct {
	File   store.File `json:"file"`
	Score  float64    `json:"score"`
	Reason string     `json:"reason"`
}

// Options 是检索参数。
type Options struct {
	Q         string
	Lib       string
	Kind      string
	Color     string
	Classify  string
	Tag       string
	Camera    string
	Favorite  bool
	MinRating int
	From      int64
	To        int64
	MinWidth  int
	MinHeight int
	Sort      string
	Desc      bool
	Limit     int
	Offset    int
	MinScore  float64
	Tolerance int
}

func (o *Options) normalize() {
	if o.Limit <= 0 {
		o.Limit = 60
	}
	if o.Limit > 500 {
		o.Limit = 500
	}
	if o.MinScore <= 0 {
		o.MinScore = 0.5
	}
	if o.Tolerance <= 0 {
		o.Tolerance = 6
	}
}

// TextSearch 以文搜图：分词 + 同义词扩展 + 字段加权打分。
func (e *Engine) TextSearch(opt Options) ([]Hit, int, error) {
	opt.normalize()
	q := strings.TrimSpace(opt.Q)
	tokens := Tokenize(q)
	if len(tokens) == 0 {
		files, total, err := e.st.QueryFiles(toFileQuery(opt))
		if err != nil {
			return nil, 0, err
		}
		hits := make([]Hit, 0, len(files))
		for _, f := range files {
			hits = append(hits, Hit{File: f, Score: 1, Reason: "列表"})
		}
		return hits, total, nil
	}
	patterns, _ := Expand(tokens)
	cand, err := e.st.SearchTextRows(patterns, opt.Lib, opt.Kind, 3000)
	if err != nil {
		return nil, 0, err
	}
	phrase := strings.ToLower(q)
	scored := make([]Hit, 0, len(cand))
	for i := range cand {
		f := cand[i]
		if !passFilters(&f, opt) {
			continue
		}
		sc, reason := textScore(&f, tokens, patterns, phrase)
		if sc <= 0 {
			continue
		}
		scored = append(scored, Hit{File: f, Score: sc, Reason: reason})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].File.MTime > scored[j].File.MTime
		}
		return scored[i].Score > scored[j].Score
	})
	total := len(scored)
	hits := paginate(scored, opt.Offset, opt.Limit)
	return hits, total, nil
}

func toFileQuery(opt Options) store.FileQuery {
	q := store.FileQuery{
		Lib:       opt.Lib,
		Kind:      opt.Kind,
		Color:     opt.Color,
		Classify:  opt.Classify,
		Tag:       opt.Tag,
		Camera:    opt.Camera,
		MinRating: opt.MinRating,
		TakenFrom: opt.From,
		TakenTo:   opt.To,
		Sort:      opt.Sort,
		Desc:      opt.Desc,
		Limit:     opt.Limit,
		Offset:    opt.Offset,
	}
	if opt.Favorite {
		t := true
		q.Favorite = &t
	}
	return q
}

func passFilters(f *store.File, opt Options) bool {
	if opt.Color != "" && !strings.EqualFold(f.Color, opt.Color) {
		if !strings.Contains(strings.ToLower(f.Tags), strings.ToLower(opt.Color)) {
			return false
		}
	}
	if opt.Classify != "" && !strings.EqualFold(f.Classify, opt.Classify) {
		return false
	}
	if opt.Camera != "" && !strings.Contains(strings.ToLower(f.Camera), strings.ToLower(opt.Camera)) {
		return false
	}
	if opt.Favorite && !f.Favorite {
		return false
	}
	if opt.MinRating > 0 && f.Rating < opt.MinRating {
		return false
	}
	if opt.MinWidth > 0 && f.Width < opt.MinWidth {
		return false
	}
	if opt.MinHeight > 0 && f.Height < opt.MinHeight {
		return false
	}
	t := f.TakenAt
	if t == 0 {
		t = f.MTime
	}
	if opt.From > 0 && t < opt.From {
		return false
	}
	if opt.To > 0 && t > opt.To {
		return false
	}
	return true
}

func textScore(f *store.File, tokens, patterns []string, phrase string) (float64, string) {
	name := strings.ToLower(f.Name)
	dir := strings.ToLower(f.Parent)
	tags := strings.ToLower(f.Tags + "," + f.Classify)
	camera := strings.ToLower(f.Camera + " " + f.Lens)
	var score float64
	hits := []string{}
	for _, t := range tokens {
		hit := false
		if strings.Contains(name, t) {
			score += 3.0
			hit = true
		}
		if strings.Contains(tags, t) {
			score += 3.5
			hit = true
		}
		if strings.Contains(dir, t) {
			score += 1.2
			hit = true
		}
		if strings.Contains(camera, t) {
			score += 1.0
			hit = true
		}
		if hit {
			hits = append(hits, t)
		}
	}
	if len(hits) == 0 {
		// 仅同义词命中，降权计入
		for _, p := range patterns {
			if strings.Contains(tags, p) || strings.Contains(name, p) {
				score += 1.0
				hits = append(hits, p)
				break
			}
		}
	}
	if phrase != "" && strings.Contains(name, phrase) {
		score += 5.0
	}
	if phrase != "" && strings.EqualFold(strings.TrimSuffix(f.Name, f.Ext), phrase) {
		score += 8.0
	}
	if f.Favorite {
		score += 0.4
	}
	score += float64(f.Rating) * 0.2
	return score, strings.Join(dedupe(hits), ",")
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func paginate(hits []Hit, offset, limit int) []Hit {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(hits) {
		return []Hit{}
	}
	end := offset + limit
	if end > len(hits) {
		end = len(hits)
	}
	return hits[offset:end]
}

// SimilarByFeatures 以图搜图：用特征与全库比对后排序。
func (e *Engine) SimilarByFeatures(ref vision.Features, opt Options, excludeID int64) ([]Hit, int, error) {
	opt.normalize()
	hits := make([]Hit, 0, 256)
	err := e.st.StreamImageFeatures(opt.Lib, func(it store.ImageFeature) error {
		if it.ID == excludeID {
			return nil
		}
		other := vision.Features{HashA: it.HashA, HashD: it.HashD, HashP: it.HashP, Hist: it.Hist}
		sp := vision.Compare(ref, other, opt.Tolerance)
		sc := sp.Normalized()
		if sc < opt.MinScore {
			return nil
		}
		f := store.File{
			ID: it.ID, Name: it.Name, Parent: it.Parent, Kind: it.Kind, Size: it.Size,
			MTime: it.MTime, TakenAt: it.Taken, Width: it.Width, Height: it.Height,
			Color: it.Color, Classify: it.Class, Tags: it.Tags, Camera: it.Camera,
		}
		reason := fmt.Sprintf("相似度 %.0f%%", sc*100)
		if sp.Duplicate() {
			reason = "疑似重复"
		}
		hits = append(hits, Hit{File: f, Score: sc, Reason: reason})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	total := len(hits)
	return paginate(hits, opt.Offset, opt.Limit), total, nil
}

// SimilarByImageData 以图搜图：直接分析上传的图片字节。
func (e *Engine) SimilarByImageData(data []byte, opt Options) ([]Hit, int, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("无法识别上传的图片: %w", err)
	}
	ref := vision.Extract(img, "query", vision.EXIF{})
	return e.SimilarByFeatures(ref, opt, 0)
}

// SimilarById 以图搜图：以库内某张图为基准。
func (e *Engine) SimilarById(id int64, opt Options) ([]Hit, int, error) {
	f, err := e.st.GetFileByID(id)
	if err != nil {
		return nil, 0, err
	}
	ref := vision.Features{HashA: f.HashA, HashD: f.HashD, HashP: f.HashP, Hist: f.Hist}
	return e.SimilarByFeatures(ref, opt, id)
}

// DuplicateGroups 用感知哈希分桶找出重复图组。
func (e *Engine) DuplicateGroups(lib string, tolerance int) [][]store.File {
	if tolerance <= 0 {
		tolerance = 6
	}
	type item struct {
		f  store.File
		hp uint64
		hd uint64
		hi []byte
	}
	items := []item{}
	_ = e.st.StreamImageFeatures(lib, func(it store.ImageFeature) error {
		if it.HashP == 0 {
			return nil
		}
		items = append(items, item{
			f: store.File{
				ID: it.ID, Name: it.Name, Parent: it.Parent, Kind: it.Kind, Size: it.Size,
				MTime: it.MTime, TakenAt: it.Taken, Width: it.Width, Height: it.Height,
				Color: it.Color, Classify: it.Class, Tags: it.Tags,
			},
			hp: it.HashP, hd: it.HashD, hi: it.Hist,
		})
		return nil
	})
	parent := make([]int, len(items))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	buckets := map[uint16][]int{}
	for i := range items {
		key := uint16(items[i].hp >> 48)
		for _, j := range buckets[key] {
			if vision.Hamming(items[i].hp, items[j].hp) <= tolerance &&
				vision.HistDistance(items[i].hi, items[j].hi) < 0.35 {
				union(i, j)
			}
		}
		buckets[key] = append(buckets[key], i)
	}
	groups := map[int][]store.File{}
	for i := range items {
		r := find(i)
		groups[r] = append(groups[r], items[i].f)
	}
	out := [][]store.File{}
	for _, g := range groups {
		if len(g) > 1 {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}
