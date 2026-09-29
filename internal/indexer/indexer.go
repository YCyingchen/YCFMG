// Package indexer 负责扫描文件系统、抽取特征并写入索引。
package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/logx"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
	"github.com/ycyingchen/ycfmg/internal/util"
	"github.com/ycyingchen/ycfmg/internal/vision"
)

// Progress 是索引进度快照。
type Progress struct {
	Running    bool   `json:"running"`
	Lib        string `json:"lib"`
	Root       string `json:"root"`
	Current    string `json:"current"`
	Scanned    int    `json:"scanned"`
	Added      int    `json:"added"`
	Updated    int    `json:"updated"`
	Skipped    int    `json:"skipped"`
	Removed    int    `json:"removed"`
	Failed     int    `json:"failed"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	Message    string `json:"message"`
}

// Indexer 是索引引擎。
type Indexer struct {
	cfg      *config.Config
	st       *store.Store
	th       *thumbs.Cache
	mu       sync.RWMutex
	prog     Progress
	queue    chan string
	queued   map[string]bool
	stopOnce sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
}

// New 创建索引器。
func New(cfg *config.Config, st *store.Store, th *thumbs.Cache) *Indexer {
	ix := &Indexer{
		cfg:    cfg,
		st:     st,
		th:     th,
		queue:  make(chan string, 256),
		queued: map[string]bool{},
		stop:   make(chan struct{}),
	}
	ix.wg.Add(1)
	go ix.worker()
	return ix
}

// Progress 返回当前进度。
func (ix *Indexer) Progress() Progress {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.prog
}

func (ix *Indexer) setProgress(fn func(p *Progress)) {
	ix.mu.Lock()
	fn(&ix.prog)
	ix.mu.Unlock()
}

// Stop 停止索引器。
func (ix *Indexer) Stop() {
	ix.stopOnce.Do(func() { close(ix.stop) })
	ix.wg.Wait()
}

// RequestScan 把目录加入异步索引队列（去重）。
func (ix *Indexer) RequestScan(dir string) {
	if dir == "" {
		return
	}
	ix.mu.Lock()
	if ix.queued[dir] {
		ix.mu.Unlock()
		return
	}
	ix.queued[dir] = true
	ix.mu.Unlock()
	select {
	case ix.queue <- dir:
	default:
		ix.mu.Lock()
		delete(ix.queued, dir)
		ix.mu.Unlock()
	}
}

func (ix *Indexer) worker() {
	defer ix.wg.Done()
	for {
		select {
		case <-ix.stop:
			return
		case dir := <-ix.queue:
			ix.mu.Lock()
			delete(ix.queued, dir)
			ix.mu.Unlock()
			if _, err := ix.IndexDir(dir, false); err != nil {
				logx.Warnf("按需索引失败 %s: %v", dir, err)
			}
		}
	}
}

// Loop 按配置的间隔周期性全量索引。
func (ix *Indexer) Loop() {
	if !ix.cfg.Index.Enabled {
		return
	}
	interval := time.Duration(ix.cfg.Index.ScanIntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}
	ix.wg.Add(1)
	go func() {
		defer ix.wg.Done()
		for {
			select {
			case <-ix.stop:
				return
			case <-time.After(interval):
				ix.ScanAll(false)
			}
		}
	}()
}

// ScanAll 扫描全部文件库。force 为真时忽略增量，重新提取特征。
func (ix *Indexer) ScanAll(force bool) {
	for _, lib := range ix.cfg.Libraries {
		if !lib.Index {
			continue
		}
		if !util.IsDir(lib.Path) {
			logx.Warnf("文件库不存在，跳过: %s", lib.Path)
			continue
		}
		ix.scanRoot(lib.Name, lib.Path, force)
	}
}

func (ix *Indexer) scanRoot(libName, root string, force bool) {
	start := time.Now()
	jobID, _ := ix.st.StartJob(libName, root)
	ix.setProgress(func(p *Progress) {
		*p = Progress{Running: true, Lib: libName, Root: root, StartedAt: time.Now().Unix(), Message: "扫描中"}
	})
	logx.Infof("开始索引 %s (%s)", libName, root)
	stat := &scanStat{}
	err := ix.walk(root, libName, force, stat, true)
	if err != nil {
		logx.Errorf("索引出错 %s: %v", root, err)
	}
	status := "done"
	msg := fmt.Sprintf("耗时 %s", time.Since(start).Round(time.Second))
	if err != nil {
		status = "failed"
		msg = err.Error()
	}
	if jobID > 0 {
		_ = ix.st.FinishJob(jobID, status, stat.scanned, stat.added, stat.updated, stat.removed, stat.failed, msg)
	}
	ix.setProgress(func(p *Progress) {
		p.Running = false
		p.FinishedAt = time.Now().Unix()
		p.Current = ""
		p.Message = msg
	})
	_ = ix.st.RebuildTagCounts()
	logx.Infof("索引完成 %s: 扫描 %d，新增 %d，更新 %d，丢失 %d，失败 %d，耗时 %s",
		libName, stat.scanned, stat.added, stat.updated, stat.removed, stat.failed, time.Since(start).Round(time.Second))
}

type scanStat struct {
	scanned int
	added   int
	updated int
	removed int
	failed  int
	skipped int
}

func (ix *Indexer) walk(root, libName string, force bool, stat *scanStat, prune bool) error {
	exclude := map[string]bool{}
	for _, e := range ix.cfg.Index.Exclude {
		exclude[e] = true
	}
	batch := make([]*store.File, 0, 128)
	tagBatch := map[string][]string{}
	keep := map[string]bool{}
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := ix.st.UpsertFiles(batch); err != nil {
			logx.Warnf("批量写入索引失败: %v", err)
		}
		for _, f := range batch {
			if tags, ok := tagBatch[f.Path]; ok {
				id, err := ix.st.GetFileByPath(f.Path)
				if err == nil {
					for _, t := range tags {
						_ = ix.st.AddFileTag(id.ID, t, vision.DefaultTagColor(t), true)
					}
				}
			}
		}
		batch = batch[:0]
		tagBatch = map[string][]string{}
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			stat.failed++
			return nil
		}
		select {
		case <-ix.stop:
			return filepath.SkipAll
		default:
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && exclude[name] {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && path != root && name != ".trash" {
				if exclude[name] {
					return filepath.SkipDir
				}
			}
			info, ierr := d.Info()
			mt := int64(0)
			if ierr == nil {
				mt = info.ModTime().Unix()
			}
			counts := ix.dirCounts(path)
			_ = ix.st.UpsertDir(&store.DirEntry{
				Path: path, Lib: libName, Parent: filepath.Dir(path), Name: name,
				MTime: mt, FileCount: counts[0], ImageCount: counts[1],
			})
			return nil
		}
		stat.scanned++
		keep[path] = true
		info, ierr := d.Info()
		if ierr != nil {
			stat.failed++
			return nil
		}
		if !force {
			if old, gerr := ix.st.GetFileByPath(path); gerr == nil {
				if old.Size == info.Size() && old.MTime == info.ModTime().Unix() && !old.Missing {
					stat.skipped++
					ix.setProgress(func(p *Progress) { p.Skipped = stat.skipped; p.Scanned = stat.scanned })
					return nil
				}
			}
		}
		f, tags, ferr := ix.buildFile(path, info, libName)
		if ferr != nil {
			stat.failed++
			return nil
		}
		batch = append(batch, f)
		if len(tags) > 0 {
			tagBatch[path] = tags
		}
		if len(batch) >= 128 {
			flush()
		}
		ix.setProgress(func(p *Progress) {
			p.Scanned = stat.scanned
			p.Added = stat.added
			p.Current = path
		})
		return nil
	})
	flush()
	if err == nil && prune {
		before := len(keep)
		_ = before
		if gerr := ix.st.MarkMissingUnder(root, keep); gerr == nil {
			if n, derr := ix.st.DeleteMissingUnder(root); derr == nil {
				stat.removed = int(n)
			}
		}
	}
	return err
}

// dirCounts 统计目录下的文件数与图片数（仅当前层）。
func (ix *Indexer) dirCounts(dir string) [2]int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return [2]int{0, 0}
	}
	files, images := 0, 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files++
		if util.IsImage(e.Name()) {
			images++
		}
		if files > 5000 {
			break
		}
	}
	return [2]int{files, images}
}

// buildFile 抽取单个文件的元数据与图像特征。
func (ix *Indexer) buildFile(path string, info os.FileInfo, libName string) (*store.File, []string, error) {
	kind := util.KindOf(info.Name(), false)
	f := &store.File{
		Lib:    libName,
		Path:   path,
		Parent: filepath.Dir(path),
		Name:   info.Name(),
		Ext:    util.Ext(info.Name()),
		Kind:   kind,
		Size:   info.Size(),
		MTime:  info.ModTime().Unix(),
	}
	var tags []string
	switch kind {
	case "image":
		w, h, err := vision.Size(path)
		if err == nil {
			f.Width, f.Height = w, h
			if h > 0 {
				f.Aspect = float64(w) / float64(h)
			}
		}
		ex := vision.ReadEXIF(path)
		f.Camera = strings.TrimSpace(ex.Make + " " + ex.Model)
		f.Lens = ex.Lens
		f.ISO = ex.ISO
		f.FNum = ex.FNumber
		f.Exposure = ex.Exposure
		f.Focal = ex.Focal
		f.TakenAt = ex.TakenAt
		f.Orientation = ex.Orientation
		if ex.HasGPS {
			f.GPSLat, f.GPSLon = ex.GPSLat, ex.GPSLon
		}
		maxPixels := int64(80_000_000)
		if w > 0 && h > 0 && int64(w)*int64(h) <= maxPixels {
			if img, err := vision.Decode(path); err == nil {
				feat := vision.Extract(img, info.Name(), ex)
				f.Width, f.Height = feat.Width, feat.Height
				f.HashA, f.HashD, f.HashP = feat.HashA, feat.HashD, feat.HashP
				f.Color = feat.Color
				f.Hist = feat.Hist
				f.Aspect = feat.Aspect
				f.Bright = feat.Bright
				f.Sharp = feat.Sharp
				f.Classify = feat.Classify
				f.Tags = strings.Join(feat.Tags, ",")
				tags = feat.Tags
			}
		}
		f.QuickHash = util.QuickHash(path, info.Size())
	case "video":
		f.QuickHash = util.QuickHash(path, info.Size())
		w, h, err := vision.Size(path)
		if err == nil {
			f.Width, f.Height = w, h
		}
	default:
		if info.Size() > 0 && info.Size() < 64*1024*1024 {
			f.QuickHash = util.QuickHash(path, info.Size())
		}
	}
	if f.TakenAt == 0 {
		f.TakenAt = 0
	}
	return f, tags, nil
}

// IndexDir 索引单个目录（depth 为 true 时递归）。
func (ix *Indexer) IndexDir(dir string, recursive bool) (int, error) {
	lib, ok := ix.cfg.LibraryByPath(dir)
	if !ok {
		if util.IsDir(dir) {
			lib = config.Library{Name: filepath.Base(dir), Path: dir}
		} else {
			return 0, fmt.Errorf("目录不在任何文件库内: %s", dir)
		}
	}
	if !util.IsDir(dir) {
		return 0, fmt.Errorf("目录不存在: %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	batch := make([]*store.File, 0, 64)
	tagBatch := map[string][]string{}
	n := 0
	files, images := 0, 0
	for _, e := range entries {
		if e.IsDir() {
			if recursive {
				sub := filepath.Join(dir, e.Name())
				_, _ = ix.IndexDir(sub, true)
			}
			continue
		}
		files++
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		if util.IsImage(e.Name()) {
			images++
		}
		if old, gerr := ix.st.GetFileByPath(filepath.Join(dir, e.Name())); gerr == nil && !old.Missing {
			if old.Size == info.Size() && old.MTime == info.ModTime().Unix() && old.Kind != "image" {
				continue
			}
			if old.Size == info.Size() && old.MTime == info.ModTime().Unix() && old.HashP != 0 {
				continue
			}
		}
		f, tags, ferr := ix.buildFile(filepath.Join(dir, e.Name()), info, lib.Name)
		if ferr != nil {
			continue
		}
		batch = append(batch, f)
		if len(tags) > 0 {
			tagBatch[f.Path] = tags
		}
		n++
	}
	if len(batch) > 0 {
		if err := ix.st.UpsertFiles(batch); err != nil {
			return n, err
		}
		for path, tags := range tagBatch {
			if rec, err := ix.st.GetFileByPath(path); err == nil {
				for _, t := range tags {
					_ = ix.st.AddFileTag(rec.ID, t, vision.DefaultTagColor(t), true)
				}
			}
		}
	}
	_ = ix.st.RebuildTagCounts()

	parent := filepath.Dir(dir)
	counts := [2]int{files, images}
	_ = ix.st.UpsertDir(&store.DirEntry{
		Path: dir, Lib: lib.Name, Parent: parent, Name: filepath.Base(dir),
		MTime: time.Now().Unix(), FileCount: counts[0], ImageCount: counts[1],
	})
	return n, nil
}

// IndexFile 索引单个文件并返回记录。
func (ix *Indexer) IndexFile(path string) (*store.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("目标是目录")
	}
	lib, ok := ix.cfg.LibraryByPath(path)
	libName := filepath.Base(filepath.Dir(path))
	if ok {
		libName = lib.Name
	}
	f, tags, err := ix.buildFile(path, info, libName)
	if err != nil {
		return nil, err
	}
	if err := ix.st.UpsertFile(f); err != nil {
		return nil, err
	}
	if rec, err := ix.st.GetFileByPath(path); err == nil {
		for _, t := range tags {
			_ = ix.st.AddFileTag(rec.ID, t, vision.DefaultTagColor(t), true)
		}
		return rec, nil
	}
	return f, nil
}

// Stats 返回索引进度中的文件库概况。
func (ix *Indexer) LibraryStats() []map[string]any {
	out := []map[string]any{}
	for _, lib := range ix.cfg.Libraries {
		info := map[string]any{
			"name":     lib.Name,
			"path":     lib.Path,
			"readonly": lib.ReadOnly,
			"index":    lib.Index,
			"exists":   util.IsDir(lib.Path),
			"files":    0,
			"images":   0,
			"size":     int64(0),
		}
		if files, total, err := ix.st.QueryFiles(store.FileQuery{Lib: lib.Name, Limit: 1, Hidden: nil}); err == nil && len(files) > 0 {
			info["files"] = total
		}
		info["images"] = ix.st.CountKind(lib.Name, "image")
		info["size"] = ix.st.LibrarySize(lib.Name)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return fmt.Sprint(out[i]["name"]) < fmt.Sprint(out[j]["name"]) })
	return out
}
