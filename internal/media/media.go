// Package media 负责文件流式传输、缩略图输出与打包下载。
package media

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/util"
)

// ContentDisposition 生成兼容中文文件名的下载头。
func ContentDisposition(name string, inline bool) string {
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	ascii := make([]rune, 0, len(name))
	for _, r := range name {
		if r < 128 && r != 0x22 && r != 0x5C {
			ascii = append(ascii, r)
		} else {
			ascii = append(ascii, 0x5F)
		}
	}
	return fmt.Sprintf("%s; filename=%q; filename*=UTF-8%s%s", disp, string(ascii), "%27%27", url.PathEscape(name))
}

// Serve 以支持 HTTP Range 的方式输出文件内容。
func Serve(w http.ResponseWriter, r *http.Request, path, name string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "文件不存在或无法访问", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "文件不可读", http.StatusInternalServerError)
		return
	}
	if st.IsDir() {
		http.Error(w, "目标是目录", http.StatusBadRequest)
		return
	}
	if name == "" {
		name = filepath.Base(path)
	}
	ctype := util.MimeOf(name)
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	q := r.URL.Query()
	if q.Get("download") == "1" || q.Get("dl") == "1" {
		w.Header().Set("Content-Disposition", ContentDisposition(name, false))
		w.Header().Set("Cache-Control", "no-store")
	} else if util.IsImage(name) || util.IsVideo(name) || util.IsAudio(name) {
		w.Header().Set("Content-Disposition", ContentDisposition(name, true))
	}
	http.ServeContent(w, r, name, st.ModTime(), f)
}

// ServeThumb 输出缩略图文件。
func ServeThumb(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "缩略图不存在", http.StatusNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "缩略图不可读", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	http.ServeContent(w, r, "thumb.jpg", st.ModTime(), f)
}

// Zip 把若干文件或目录打包为 zip 直接写入响应。
func Zip(w http.ResponseWriter, r *http.Request, paths []string, rootName string) error {
	if rootName == "" {
		rootName = "ycfmg-share"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", ContentDisposition(rootName+".zip", false))
	w.Header().Set("Cache-Control", "no-store")
	zw := zip.NewWriter(w)
	defer zw.Close()
	used := map[string]bool{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			base := filepath.Base(p)
			err = filepath.Walk(p, func(sub string, info os.FileInfo, werr error) error {
				if werr != nil || info == nil {
					return nil
				}
				select {
				case <-r.Context().Done():
					return r.Context().Err()
				default:
				}
				if info.IsDir() {
					return nil
				}
				rel, rerr := filepath.Rel(filepath.Dir(p), sub)
				if rerr != nil {
					rel = info.Name()
				}
				_ = base
				return addToZip(zw, sub, rel, info, used)
			})
			if err != nil {
				return err
			}
			continue
		}
		if err := addToZip(zw, p, filepath.Base(p), st, used); err != nil {
			return err
		}
	}
	return nil
}

func addToZip(zw *zip.Writer, src, name string, info os.FileInfo, used map[string]bool) error {
	name = strings.ReplaceAll(name, string(os.PathSeparator), "/")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = info.Name()
	}
	orig := name
	i := 1
	for used[name] {
		ext := filepath.Ext(orig)
		stem := strings.TrimSuffix(orig, ext)
		name = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		i++
	}
	used[name] = true
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	hdr.Modified = info.ModTime()
	wr, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return nil
	}
	defer f.Close()
	_, err = io.Copy(wr, f)
	return err
}

// ZipWriter 暴露一个可复用的打包入口（供分享页使用）。
func ZipToFile(dst string, paths []string) error {
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	used := map[string]bool{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			_ = filepath.Walk(p, func(sub string, info os.FileInfo, werr error) error {
				if werr != nil || info == nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(filepath.Dir(p), sub)
				return addToZip(zw, sub, rel, info, used)
			})
			continue
		}
		if err := addToZip(zw, p, filepath.Base(p), st, used); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return nil
}

// ETag 生成基于路径与时间的弱校验值。
func ETag(path string, mtime time.Time, size int64) string {
	return fmt.Sprintf("W/\"%x-%x-%s\"", mtime.Unix(), size, filepath.Base(path))
}
