// Package util 提供跨模块复用的通用工具函数。
package util

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Token 生成指定位数的 base62 随机串，用于分享令牌、会话 ID。
func Token(n int) string {
	if n <= 0 {
		n = 16
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下退化为时间戳，保证不 panic
		for i := range buf {
			buf[i] = byte(time.Now().UnixNano() >> (i % 8))
		}
	}
	sb := make([]byte, n)
	for i, b := range buf {
		sb[i] = base62[int(b)%len(base62)]
	}
	return string(sb)
}

// RandHex 生成 n 字节的十六进制随机串。
func RandHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// HumanSize 把字节数转成人类可读体积。
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := -1
	for v >= unit && i < len(units)-1 {
		v /= unit
		i++
	}
	if v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// Ext 返回小写扩展名（含点）。
func Ext(name string) string {
	return strings.ToLower(filepath.Ext(name))
}

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
	".webp": true, ".tif": true, ".tiff": true, ".heic": true, ".heif": true,
	".avif": true, ".jfif": true, ".ico": true, ".svg": true, ".raw": true,
	".cr2": true, ".cr3": true, ".nef": true, ".arw": true, ".dng": true,
	".orf": true, ".rw2": true, ".pef": true, ".srw": true,
}

var videoExts = map[string]bool{
	".mp4": true, ".mkv": true, ".mov": true, ".avi": true, ".wmv": true,
	".flv": true, ".webm": true, ".m4v": true, ".mpg": true, ".mpeg": true,
	".ts": true, ".m2ts": true, ".3gp": true, ".rmvb": true, ".vob": true,
}

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true,
	".m4a": true, ".wma": true, ".ape": true, ".opus": true, ".aiff": true,
}

var archiveExts = map[string]bool{
	".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
	".bz2": true, ".xz": true, ".tgz": true, ".zst": true, ".iso": true,
}

var docExts = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true, ".md": true, ".csv": true,
	".epub": true, ".mobi": true, ".rtf": true, ".json": true, ".xml": true,
	".yaml": true, ".yml": true, ".log": true, ".ini": true, ".conf": true,
}

// IsImage 判断是否为可索引图片。
func IsImage(name string) bool { return imageExts[Ext(name)] }

// IsVideo 判断是否为视频。
func IsVideo(name string) bool { return videoExts[Ext(name)] }

// IsAudio 判断是否为音频。
func IsAudio(name string) bool { return audioExts[Ext(name)] }

// IsArchive 判断是否为压缩包。
func IsArchive(name string) bool { return archiveExts[Ext(name)] }

// IsDoc 判断是否为文档。
func IsDoc(name string) bool { return docExts[Ext(name)] }

// KindOf 返回文件大类：image/video/audio/archive/doc/other。
func KindOf(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	switch {
	case IsImage(name):
		return "image"
	case IsVideo(name):
		return "video"
	case IsAudio(name):
		return "audio"
	case IsArchive(name):
		return "archive"
	case IsDoc(name):
		return "doc"
	}
	return "other"
}

// MimeOf 依据扩展名返回 MIME。
func MimeOf(name string) string {
	switch Ext(name) {
	case ".jpg", ".jpeg", ".jfif":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".ico":
		return "image/x-icon"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".txt", ".log", ".md", ".ini", ".conf":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	}
	return "application/octet-stream"
}

// WithinBase 判断 target 是否位于 base 之内，防止路径穿越。
func WithinBase(base, target string) bool {
	absBase, err1 := filepath.Abs(base)
	absTarget, err2 := filepath.Abs(target)
	if err1 != nil || err2 != nil {
		return false
	}
	absBase = filepath.Clean(absBase)
	absTarget = filepath.Clean(absTarget)
	if absTarget == absBase {
		return true
	}
	return strings.HasPrefix(absTarget, absBase+string(os.PathSeparator))
}

// CleanName 清洗用户提供的文件名，去掉路径分隔与危险字符。
func CleanName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")
	if name == "" {
		name = "unnamed"
	}
	return name
}

// Exists 判断路径是否存在。
func Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// IsDir 判断是否为目录。
func IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// FileMD5 计算文件 MD5（用于去重）。
func FileMD5(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileSHA1 计算文件 SHA1（用于区分同名文件）。
func FileSHA1(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// QuickHash 计算文件「头 64KB + 尾 64KB + 大小」的 SHA256，用于快速判重。
func QuickHash(p string, size int64) string {
	const chunk = 64 * 1024
	h := sha256.New()
	fmt.Fprintf(h, "%d:", size)
	f, err := os.Open(p)
	if err != nil {
		return hex.EncodeToString(h.Sum(nil))
	}
	defer f.Close()
	head := make([]byte, chunk)
	n, _ := io.ReadFull(f, head)
	h.Write(head[:n])
	if size > chunk*2 {
		if _, err := f.Seek(size-chunk, io.SeekStart); err == nil {
			tail := make([]byte, chunk)
			m, _ := io.ReadFull(f, tail)
			h.Write(tail[:m])
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CopyFile 复制文件并保留权限。
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// UniquePath 若目标已存在则自动追加 (1)、(2) 后缀。
func UniquePath(p string) string {
	if !Exists(p) {
		return p
	}
	dir := filepath.Dir(p)
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i < 10000; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if !Exists(cand) {
			return cand
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, RandHex(4), ext))
}

// FormatTime 输出 ISO 风格本地时间。
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
