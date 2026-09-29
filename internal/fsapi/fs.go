// Package fsapi 实现真实文件系统操作：浏览、新建、重命名、移动、复制、删除、回收站、上传。
package fsapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/util"
)

// Entry 是目录中的一个条目（合并了磁盘信息与索引信息）。
type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size"`
	MTime     int64  `json:"mtime"`
	Kind      string `json:"kind"`
	Ext       string `json:"ext"`
	Mode      string `json:"mode"`
	Writable  bool   `json:"writable"`
	FileCount int    `json:"file_count"`
	ImgCount  int    `json:"image_count"`
	Indexed   bool   `json:"indexed"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Color     string `json:"color"`
	Classify  string `json:"classify"`
	Tags      string `json:"tags"`
	Favorite  bool   `json:"favorite"`
	Rating    int    `json:"rating"`
	HasGPS    bool   `json:"has_gps"`
	Camera    string `json:"camera"`
	TakenAt   int64  `json:"taken_at"`
}

// FS 是文件系统门面。
type FS struct {
	cfg *config.Config
	st  *store.Store
}

// New 创建门面。
func New(cfg *config.Config, st *store.Store) *FS { return &FS{cfg: cfg, st: st} }

// Config 暴露配置。
func (f *FS) Config() *config.Config { return f.cfg }

// Guard 校验路径是否可访问，返回绝对路径。
func (f *FS) Guard(p string, needWrite bool) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("路径不能为空")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if _, ok := f.cfg.LibraryByPath(abs); !ok {
		return "", errors.New("路径不在受管文件库内")
	}
	if needWrite && !f.cfg.Writable(abs) {
		return "", errors.New("该位置为只读，无法修改")
	}
	return abs, nil
}

// List 列出目录内容（含索引增强信息）。
func (f *FS) List(dir string, showHidden bool) ([]Entry, error) {
	abs, err := f.Guard(dir, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("目录不存在: %s", abs)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, ierr := e.Info()
		if ierr != nil {
			continue
		}
		ep := filepath.Join(abs, e.Name())
		it := Entry{
			Name:     e.Name(),
			Path:     ep,
			IsDir:    e.IsDir(),
			Size:     fi.Size(),
			MTime:    fi.ModTime().Unix(),
			Kind:     util.KindOf(e.Name(), e.IsDir()),
			Ext:      util.Ext(e.Name()),
			Mode:     fi.Mode().String(),
			Writable: f.cfg.Writable(ep),
		}
		out = append(out, it)
	}
	f.enrich(abs, out)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (f *FS) enrich(dir string, entries []Entry) {
	if f.st == nil {
		return
	}
	want := map[string]*Entry{}
	for i := range entries {
		if entries[i].IsDir {
			continue
		}
		want[entries[i].Path] = &entries[i]
	}
	for i := range entries {
		if !entries[i].IsDir {
			continue
		}
		sub := entries[i].Path
		files, images := 0, 0
		kids, err := os.ReadDir(sub)
		if err == nil {
			for _, k := range kids {
				if k.IsDir() {
					continue
				}
				files++
				if util.IsImage(k.Name()) {
					images++
				}
				if len(want) == 0 {
					break
				}
			}
		}
		entries[i].FileCount = files
		entries[i].ImgCount = images
	}
	paths := make([]string, 0, len(want))
	for p := range want {
		paths = append(paths, p)
	}
	for _, p := range paths {
		rec, err := f.st.GetFileByPath(p)
		if err != nil {
			continue
		}
		e := want[p]
		e.Indexed = true
		e.Width, e.Height = rec.Width, rec.Height
		e.Color = rec.Color
		e.Classify = rec.Classify
		e.Tags = rec.Tags
		e.Favorite = rec.Favorite
		e.Rating = rec.Rating
		e.Camera = rec.Camera
		e.TakenAt = rec.TakenAt
		e.HasGPS = rec.GPSLat != 0 || rec.GPSLon != 0
	}
}

// Mkdir 新建目录。
func (f *FS) Mkdir(parent, name string) (string, error) {
	abs, err := f.Guard(parent, true)
	if err != nil {
		return "", err
	}
	name = util.CleanName(name)
	target := util.UniquePath(filepath.Join(abs, name))
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	return target, nil
}

// Rename 重命名（仅改同目录下的名字）。
func (f *FS) Rename(path, newName string) (string, error) {
	abs, err := f.Guard(path, true)
	if err != nil {
		return "", err
	}
	newName = util.CleanName(newName)
	if newName == filepath.Base(abs) {
		return abs, nil
	}
	target := filepath.Join(filepath.Dir(abs), newName)
	if util.Exists(target) {
		return "", errors.New("同名文件已存在")
	}
	if err := os.Rename(abs, target); err != nil {
		return "", err
	}
	f.reindex(abs, target)
	return target, nil
}

// Move 移动多个条目到目标目录。
func (f *FS) Move(paths []string, dstDir string) (int, error) {
	dst, err := f.Guard(dstDir, true)
	if err != nil {
		return 0, err
	}
	if !util.IsDir(dst) {
		return 0, errors.New("目标不是目录")
	}
	n := 0
	for _, p := range paths {
		src, err := f.Guard(p, true)
		if err != nil {
			continue
		}
		if src == dst || strings.HasPrefix(dst, src+string(os.PathSeparator)) {
			continue
		}
		target := util.UniquePath(filepath.Join(dst, filepath.Base(src)))
		if err := os.Rename(src, target); err != nil {
			if err2 := copyRecursive(src, target); err2 != nil {
				continue
			}
			_ = os.RemoveAll(src)
		}
		f.reindex(src, target)
		n++
	}
	return n, nil
}

// Copy 复制多个条目到目标目录。
func (f *FS) Copy(paths []string, dstDir string) (int, error) {
	dst, err := f.Guard(dstDir, true)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range paths {
		src, err := f.Guard(p, false)
		if err != nil {
			continue
		}
		target := util.UniquePath(filepath.Join(dst, filepath.Base(src)))
		if src == target {
			continue
		}
		if err := copyRecursive(src, target); err != nil {
			continue
		}
		n++
	}
	return n, nil
}

func copyRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return util.CopyFile(src, dst)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyRecursive(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// TrashItem 是回收站条目。
type TrashItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	OrigPath string `json:"orig_path"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Deleted  int64  `json:"deleted"`
}

// TrashDir 返回回收站目录。
func (f *FS) TrashDir() string { return filepath.Join(f.cfg.DataDir, "trash") }

// MetaDir 返回回收站元数据目录。
func (f *FS) MetaDir() string { return filepath.Join(f.cfg.DataDir, "trashmeta") }

// Delete 删除条目；toTrash 为真时移入回收站。
func (f *FS) Delete(paths []string, toTrash bool) (int, error) {
	n := 0
	for _, p := range paths {
		abs, err := f.Guard(p, true)
		if err != nil {
			continue
		}
		if abs == "/" || abs == filepath.VolumeName(abs)+string(os.PathSeparator) {
			continue
		}
		info, serr := os.Stat(abs)
		if serr != nil {
			continue
		}
		if toTrash && info.IsDir() {
			// 目录整体移入回收站
		}
		if toTrash {
			if err := f.moveToTrash(abs, info); err != nil {
				if err2 := os.RemoveAll(abs); err2 != nil {
					continue
				}
			}
		} else if err := os.RemoveAll(abs); err != nil {
			continue
		}
		if f.st != nil {
			_ = f.st.DeleteFileByPath(abs)
			_ = f.st.DeleteDirsUnder(abs)
		}
		n++
	}
	return n, nil
}

func (f *FS) moveToTrash(abs string, info os.FileInfo) error {
	if err := os.MkdirAll(f.TrashDir(), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(f.MetaDir(), 0o755); err != nil {
		return err
	}
	id := fmt.Sprintf("%d_%s", time.Now().UnixNano(), util.RandHex(4))
	ext := filepath.Ext(abs)
	dst := filepath.Join(f.TrashDir(), id+ext)
	if err := os.Rename(abs, dst); err != nil {
		return err
	}
	meta := TrashItem{
		ID:       id,
		Name:     filepath.Base(abs),
		OrigPath: abs,
		Size:     info.Size(),
		IsDir:    info.IsDir(),
		Deleted:  time.Now().Unix(),
	}
	raw, _ := json.Marshal(meta)
	return os.WriteFile(filepath.Join(f.MetaDir(), id+".json"), raw, 0o644)
}

// TrashList 列出回收站。
func (f *FS) TrashList() []TrashItem {
	out := []TrashItem{}
	entries, err := os.ReadDir(f.MetaDir())
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(f.MetaDir(), e.Name()))
		if err != nil {
			continue
		}
		var it TrashItem
		if err := json.Unmarshal(raw, &it); err != nil {
			continue
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Deleted > out[j].Deleted })
	return out
}

// TrashRestore 从回收站恢复。
func (f *FS) TrashRestore(id string) error {
	raw, err := os.ReadFile(filepath.Join(f.MetaDir(), id+".json"))
	if err != nil {
		return errors.New("回收站条目不存在")
	}
	var it TrashItem
	if err := json.Unmarshal(raw, &it); err != nil {
		return err
	}
	src := ""
	entries, _ := os.ReadDir(f.TrashDir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), id) {
			src = filepath.Join(f.TrashDir(), e.Name())
			break
		}
	}
	if src == "" {
		return errors.New("回收站文件已不存在")
	}
	target := util.UniquePath(it.OrigPath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, target); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(f.MetaDir(), id+".json"))
	return nil
}

// TrashPurge 清空回收站（传空则全部清空）。
func (f *FS) TrashPurge(ids []string) error {
	if len(ids) == 0 {
		_ = os.RemoveAll(f.TrashDir())
		_ = os.RemoveAll(f.MetaDir())
		return os.MkdirAll(f.TrashDir(), 0o755)
	}
	for _, id := range ids {
		entries, _ := os.ReadDir(f.TrashDir())
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), id) {
				_ = os.RemoveAll(filepath.Join(f.TrashDir(), e.Name()))
			}
		}
		_ = os.Remove(filepath.Join(f.MetaDir(), id+".json"))
	}
	return nil
}

// Save 保存上传的文件流。
func (f *FS) Save(dir, name string, r io.Reader) (string, error) {
	abs, err := f.Guard(dir, true)
	if err != nil {
		return "", err
	}
	name = util.CleanName(name)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	target := util.UniquePath(filepath.Join(abs, name))
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, r); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	return target, nil
}

// SaveChunk 支持分片上传：先写临时文件，最后合并。
func (f *FS) SaveChunk(dir, name, uploadID string, index int, total int, r io.Reader) (string, bool, error) {
	tmpDir := filepath.Join(f.cfg.DataDir, "uploads", uploadID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", false, err
	}
	part := filepath.Join(tmpDir, fmt.Sprintf("%06d.part", index))
	out, err := os.Create(part)
	if err != nil {
		return "", false, err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return "", false, err
	}
	out.Close()
	parts, _ := os.ReadDir(tmpDir)
	if len(parts) < total {
		return "", false, nil
	}
	abs, err := f.Guard(dir, true)
	if err != nil {
		return "", false, err
	}
	target := util.UniquePath(filepath.Join(abs, util.CleanName(name)))
	dst, err := os.Create(target)
	if err != nil {
		return "", false, err
	}
	defer dst.Close()
	for i := 0; i < total; i++ {
		p := filepath.Join(tmpDir, fmt.Sprintf("%06d.part", i))
		pf, err := os.Open(p)
		if err != nil {
			continue
		}
		_, _ = io.Copy(dst, pf)
		pf.Close()
	}
	_ = os.RemoveAll(tmpDir)
	return target, true, nil
}

func (f *FS) reindex(oldPath, newPath string) {
	if f.st == nil {
		return
	}
	_ = f.st.MovePath(oldPath, newPath)
}

// Dirs 返回所有文件库根目录。
func (f *FS) Dirs() []map[string]string {
	out := []map[string]string{}
	for _, l := range f.cfg.Libraries {
		out = append(out, map[string]string{"name": l.Name, "path": l.Path})
	}
	return out
}
