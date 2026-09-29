// Package thumbs 负责生成与缓存多尺寸缩略图。
package thumbs

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/ycyingchen/ycfmg/internal/logx"
	"github.com/ycyingchen/ycfmg/internal/vision"
)

// Cache 是缩略图缓存。
type Cache struct {
	dir      string
	sizes    []int
	quality  int
	mu       sync.Mutex
	inflight map[string]bool
}

// New 创建缓存并确保目录存在。
func New(dir string, sizes []int, quality int) *Cache {
	if quality <= 0 || quality > 100 {
		quality = 82
	}
	if len(sizes) == 0 {
		sizes = []int{256, 768}
	}
	_ = os.MkdirAll(dir, 0o755)
	return &Cache{dir: dir, sizes: sizes, quality: quality, inflight: map[string]bool{}}
}

// Sizes 返回已配置的尺寸档位。
func (c *Cache) Sizes() []int { return c.sizes }

// Dir 返回缓存根目录。
func (c *Cache) Dir() string { return c.dir }

// key 依据源路径与修改时间生成稳定文件名，源文件变更后自动失效。
func (c *Cache) key(src string, size int, mtime int64) string {
	h := sha1.Sum([]byte(src))
	hexs := hex.EncodeToString(h[:])
	return filepath.Join(hexs[:2], hexs[2:4], fmt.Sprintf("%s_%d_%d.jpg", hexs[4:20], size, mtime))
}

// Path 返回缩略图缓存路径（不保证已存在）。
func (c *Cache) Path(src string, size int, mtime int64) string {
	return filepath.Join(c.dir, c.key(src, size, mtime))
}

// Get 返回缩略图路径，必要时即时生成。
func (c *Cache) Get(src string, size int, mtime int64) (string, error) {
	dst := c.Path(src, size, mtime)
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return dst, nil
	}
	c.mu.Lock()
	if c.inflight[dst] {
		c.mu.Unlock()
		for i := 0; i < 100; i++ {
			if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
				return dst, nil
			}
			timeSleep(50)
		}
		return dst, fmt.Errorf("缩略图生成超时")
	}
	c.inflight[dst] = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.inflight, dst)
		c.mu.Unlock()
	}()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := c.generate(src, dst, size); err != nil {
		if ferr := c.generateExternal(src, dst, size); ferr != nil {
			return "", err
		}
	}
	return dst, nil
}

func (c *Cache) generate(src, dst string, size int) error {
	img, err := vision.Decode(src)
	if err != nil {
		return err
	}
	b := img.Bounds()
	w, h := vision.FitSize(b.Dx(), b.Dy(), size)
	scaled := vision.Scale(img, w, h)
	return writeJPEG(dst, scaled, c.quality)
}

// generateExternal 用系统 ffmpeg 兜底处理 HEIC/RAW 等 Go 标准库不支持的格式。
func (c *Cache) generateExternal(src, dst string, size int) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("未找到 ffmpeg")
	}
	tmp := dst + ".tmp.jpg"
	vf := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", size, size)
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", src,
		"-vf", vf, "-frames:v", "1", "-q:v", "4", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("ffmpeg 失败: %v %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	return nil
}

func writeJPEG(path string, img image.Image, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// Invalidate 删除某个源文件的所有缓存缩略图。
func (c *Cache) Invalidate(src string) {
	h := sha1.Sum([]byte(src))
	hexs := hex.EncodeToString(h[:])
	dir := filepath.Join(c.dir, hexs[:2], hexs[2:4])
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := hexs[4:20] + "_"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// EnsureSizes 为一张图批量生成全部尺寸。
func (c *Cache) EnsureSizes(src string, mtime int64) []string {
	out := make([]string, 0, len(c.sizes))
	for _, s := range c.sizes {
		p, err := c.Get(src, s, mtime)
		if err != nil {
			logx.Debugf("生成缩略图失败 %s (%d): %v", src, s, err)
			continue
		}
		out = append(out, p)
	}
	return out
}

// Cleanup 清空所有缓存缩略图。
func (c *Cache) Cleanup() error {
	if err := os.RemoveAll(c.dir); err != nil {
		return err
	}
	return os.MkdirAll(c.dir, 0o755)
}

// Size 返回缓存占用字节数。
func (c *Cache) Size() int64 {
	var total int64
	_ = filepath.Walk(c.dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// ParseSize 解析尺寸参数。
func ParseSize(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	if n > 4096 {
		n = 4096
	}
	return n
}
