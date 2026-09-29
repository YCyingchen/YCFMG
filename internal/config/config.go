// Package config 负责 YCFMG 的配置加载、默认值与校验。
// 配置来源优先级：内置默认 < 配置文件 < 环境变量。
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Server 服务端监听与外部访问设置。
type Server struct {
	Host string
	Port int
	// SharePort 分享专用端口；0 表示关闭（分享页与主站共用端口）
	SharePort int
	// Socket 统一网关 Unix Socket 路径（飞牛 fnOS 用）；为空则只监听 TCP
	Socket     string
	BasePath   string
	PublicURLs []string
	TrustProxy bool
	TLSCert    string
	TLSKey     string
	Readonly   bool
}

// Library 是被管理的文件库根目录。
type Library struct {
	Name     string
	Path     string
	ReadOnly bool
	Index    bool
	Cover    string
}

// Auth 账号与会话设置。
type Auth struct {
	Username        string
	Password        string
	SessionSecret   string
	SessionTTLHours int
	AllowGuest      bool
}

// Index 索引器设置。
type Index struct {
	Enabled             bool
	ScanIntervalMinutes int
	ThumbSizes          []int
	Exclude             []string
	FollowSymlinks      bool
	AutoTags            bool
	HashTolerance       int
	MaxFileMB           int
}

// Share 分享设置。
type Share struct {
	DefaultExpireDays int
	MaxItems          int
	AllowDownload     bool
	BrandName         string
	BrandSubtitle     string
}

// Semantic 语义检索（可选 CLIP 端点）设置。
type Semantic struct {
	Enabled    bool
	Endpoint   string
	APIKey     string
	Model      string
	TimeoutSec int
}

// Config 是应用完整配置。
type Config struct {
	Server    Server
	DataDir   string
	Libraries []Library
	Auth      Auth
	Index     Index
	Share     Share
	Semantic  Semantic
	LogLevel  string
	Update    UpdateConfig
	path      string
}

// Default 返回内置默认配置。
func Default() *Config {
	return &Config{
		Server: Server{
			Host:       "0.0.0.0",
			Port:       8686,
			BasePath:   "",
			TrustProxy: true,
		},
		DataDir: defaultDataDir(),
		Auth: Auth{
			Username:        "admin",
			SessionTTLHours: 72 * 24,
			AllowGuest:      true,
		},
		Index: Index{
			Enabled:             true,
			ScanIntervalMinutes: 360,
			ThumbSizes:          []int{256, 768},
			Exclude:             []string{".@__thumb", "#recycle", "@eaDir", ".git", "node_modules", "$RECYCLE.BIN", "System Volume Information", ".thumbnails", ".cache"},
			AutoTags:            true,
			HashTolerance:       6,
			MaxFileMB:           512,
		},
		Share: Share{
			DefaultExpireDays: 7,
			MaxItems:          2000,
			AllowDownload:     true,
			BrandName:         "YCFMG",
			BrandSubtitle:     "文件管理 · 智能图库",
		},
		Semantic: Semantic{
			Enabled:    false,
			Model:      "clip-vit-base-patch32",
			TimeoutSec: 20,
		},
		LogLevel: "info",
	}
}

func defaultDataDir() string {
	if st, err := os.Stat("/config"); err == nil && st.IsDir() {
		if err := os.MkdirAll("/config/ycfmg", 0o755); err == nil {
			return "/config/ycfmg"
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".ycfmg")
	}
	return "./data"
}

// Path 返回配置文件路径。
func (c *Config) Path() string { return c.path }

// Load 读取配置文件（不存在则创建默认文件），随后应用环境变量覆盖。
func Load(path string) (*Config, error) {
	cfg := Default()
	cfg.Update = DefaultUpdate()
	cfg.path = path

	if path == "" {
		path = "config.yaml"
		cfg.path = path
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if werr := os.MkdirAll(filepath.Dir(absOrDot(path)), 0o755); werr == nil {
			_ = os.WriteFile(path, []byte(DefaultYAML()), 0o644)
		}
	} else if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	} else {
		root, perr := ParseYAML(data)
		if perr != nil {
			return nil, fmt.Errorf("解析配置失败: %w", perr)
		}
		if m, ok := root.(map[string]any); ok {
			cfg.apply(m)
		}
	}
	cfg.applyEnv()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func absOrDot(p string) string {
	if p == "" {
		return "."
	}
	return p
}

func (c *Config) apply(m map[string]any) {
	if s, ok := asMap(m["server"]); ok {
		c.Server.Host = str(s["host"], c.Server.Host)
		c.Server.Port = num(s["port"], c.Server.Port)
		c.Server.SharePort = num(s["share_port"], c.Server.SharePort)
		c.Server.Socket = str(s["socket"], c.Server.Socket)
		c.Server.BasePath = str(s["base_path"], c.Server.BasePath)
		c.Server.TrustProxy = boolean(s["trust_proxy"], c.Server.TrustProxy)
		c.Server.TLSCert = str(s["tls_cert"], c.Server.TLSCert)
		c.Server.TLSKey = str(s["tls_key"], c.Server.TLSKey)
		c.Server.Readonly = boolean(s["readonly"], c.Server.Readonly)
		if v, ok := list(s["public_urls"]); ok {
			c.Server.PublicURLs = v
		}
	}
	if v, ok := strv(m["data_dir"]); ok {
		c.DataDir = v
	}
	if arr, ok := slice(m["libraries"]); ok {
		libs := make([]Library, 0, len(arr))
		for _, item := range arr {
			im, ok := asMap(item)
			if !ok {
				if p, ok := strv(item); ok {
					libs = append(libs, Library{Name: filepath.Base(p), Path: p, Index: true})
				}
				continue
			}
			p := str(im["path"], "")
			if p == "" {
				continue
			}
			libs = append(libs, Library{
				Name:     str(im["name"], filepath.Base(p)),
				Path:     p,
				ReadOnly: boolean(im["readonly"], false),
				Index:    boolean(im["index"], true),
				Cover:    str(im["cover"], ""),
			})
		}
		c.Libraries = libs
	}
	if s, ok := asMap(m["auth"]); ok {
		c.Auth.Username = str(s["username"], c.Auth.Username)
		c.Auth.Password = str(s["password"], c.Auth.Password)
		c.Auth.SessionSecret = str(s["session_secret"], c.Auth.SessionSecret)
		c.Auth.SessionTTLHours = num(s["session_ttl_hours"], c.Auth.SessionTTLHours)
		c.Auth.AllowGuest = boolean(s["allow_guest"], c.Auth.AllowGuest)
	}
	if s, ok := asMap(m["index"]); ok {
		c.Index.Enabled = boolean(s["enabled"], c.Index.Enabled)
		c.Index.ScanIntervalMinutes = num(s["scan_interval_minutes"], c.Index.ScanIntervalMinutes)
		c.Index.AutoTags = boolean(s["auto_tags"], c.Index.AutoTags)
		c.Index.FollowSymlinks = boolean(s["follow_symlinks"], c.Index.FollowSymlinks)
		c.Index.HashTolerance = num(s["hash_tolerance"], c.Index.HashTolerance)
		c.Index.MaxFileMB = num(s["max_file_mb"], c.Index.MaxFileMB)
		if v, ok := intlist(s["thumb_sizes"]); ok {
			c.Index.ThumbSizes = v
		}
		if v, ok := list(s["exclude"]); ok {
			c.Index.Exclude = v
		}
	}
	if s, ok := asMap(m["share"]); ok {
		c.Share.DefaultExpireDays = num(s["default_expire_days"], c.Share.DefaultExpireDays)
		c.Share.MaxItems = num(s["max_items"], c.Share.MaxItems)
		c.Share.AllowDownload = boolean(s["allow_download"], c.Share.AllowDownload)
		c.Share.BrandName = str(s["brand_name"], c.Share.BrandName)
		c.Share.BrandSubtitle = str(s["brand_subtitle"], c.Share.BrandSubtitle)
	}
	if s, ok := asMap(m["semantic"]); ok {
		c.Semantic.Enabled = boolean(s["enabled"], c.Semantic.Enabled)
		c.Semantic.Endpoint = str(s["endpoint"], c.Semantic.Endpoint)
		c.Semantic.APIKey = str(s["api_key"], c.Semantic.APIKey)
		c.Semantic.Model = str(s["model"], c.Semantic.Model)
		c.Semantic.TimeoutSec = num(s["timeout_sec"], c.Semantic.TimeoutSec)
	}
	c.LogLevel = str(m["log_level"], c.LogLevel)
	c.parseUpdate(m)
}

func (c *Config) applyEnv() {
	if v := os.Getenv("YCFMG_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("YCFMG_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.Port = n
		}
	}
	if v := os.Getenv("YCFMG_SHARE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.SharePort = n
		}
	}
	if v := os.Getenv("YCFMG_SOCKET"); v != "" {
		c.Server.Socket = v
	}
	if v := os.Getenv("YCFMG_BASE_PATH"); v != "" {
		c.Server.BasePath = v
	}
	if v := os.Getenv("YCFMG_DATA_DIR"); v != "" {
		c.DataDir = v
	} else if v := os.Getenv("YCFMG_DATA"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("YCFMG_PUBLIC_URLS"); v != "" {
		c.Server.PublicURLs = splitCSV(v)
	}
	if v := os.Getenv("YCFMG_PUBLIC_URLS_B64"); v != "" {
		if raw, derr := base64.StdEncoding.DecodeString(v); derr == nil {
			c.Server.PublicURLs = splitCSV(string(raw))
		}
	}
	if v := os.Getenv("YCFMG_ADMIN_USER"); v != "" {
		c.Auth.Username = v
	}
	if v := os.Getenv("YCFMG_ADMIN_PASSWORD"); v != "" {
		c.Auth.Password = v
	}
	if v := os.Getenv("YCFMG_ADMIN_PASSWORD_B64"); v != "" {
		if raw, derr := base64.StdEncoding.DecodeString(v); derr == nil {
			c.Auth.Password = string(raw)
		}
	}
	if v := os.Getenv("YCFMG_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("YCFMG_LIBRARIES"); v != "" {
		libs := []Library{}
		for _, item := range splitCSV(v) {
			ro := false
			p := item
			if strings.HasSuffix(p, ":ro") {
				ro = true
				p = strings.TrimSuffix(p, ":ro")
			} else if strings.HasSuffix(p, ":rw") {
				p = strings.TrimSuffix(p, ":rw")
			}
			if p == "" {
				continue
			}
			libs = append(libs, Library{Name: filepath.Base(p), Path: p, ReadOnly: ro, Index: true})
		}
		if len(libs) > 0 {
			c.Libraries = libs
		}
	}
	if v := os.Getenv("YCFMG_SEMANTIC_ENDPOINT"); v != "" {
		c.Semantic.Endpoint = v
		c.Semantic.Enabled = true
	}
	if v := os.Getenv("YCFMG_SEMANTIC_KEY"); v != "" {
		c.Semantic.APIKey = v
	}
	if v := os.Getenv("YCFMG_READONLY"); v == "1" || strings.EqualFold(v, "true") {
		c.Server.Readonly = true
	}
}

func (c *Config) validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("端口非法: %d", c.Server.Port)
	}
	c.Server.BasePath = normalizeBasePath(c.Server.BasePath)
	if c.DataDir == "" {
		c.DataDir = defaultDataDir()
	}
	if abs, err := filepath.Abs(c.DataDir); err == nil {
		c.DataDir = abs
	}
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return fmt.Errorf("无法创建数据目录 %s: %w", c.DataDir, err)
	}
	if err := os.MkdirAll(filepath.Join(c.DataDir, "thumbs"), 0o755); err != nil {
		return fmt.Errorf("无法创建缩略图目录: %w", err)
	}
	if c.Share.MaxItems <= 0 {
		c.Share.MaxItems = 2000
	}
	if len(c.Index.ThumbSizes) == 0 {
		c.Index.ThumbSizes = []int{256, 768}
	}
	if c.Index.ScanIntervalMinutes <= 0 {
		c.Index.ScanIntervalMinutes = 360
	}
	seen := map[string]bool{}
	libs := make([]Library, 0, len(c.Libraries))
	for _, l := range c.Libraries {
		if l.Path == "" {
			continue
		}
		abs, err := filepath.Abs(l.Path)
		if err == nil {
			l.Path = abs
		}
		l.Path = strings.TrimRight(l.Path, "/")
		if seen[l.Path] {
			continue
		}
		seen[l.Path] = true
		if l.Name == "" {
			l.Name = filepath.Base(l.Path)
		}
		libs = append(libs, l)
	}
	c.Libraries = libs
	return nil
}

func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimRight(p, "/")
}

// DataFile 返回数据目录下的文件路径。
func (c *Config) DataFile(name string) string { return filepath.Join(c.DataDir, name) }

// ThumbDir 返回缩略图缓存目录。
func (c *Config) ThumbDir() string { return filepath.Join(c.DataDir, "thumbs") }

// LibraryByPath 按路径前缀查找所属文件库。
func (c *Config) LibraryByPath(p string) (Library, bool) {
	for _, l := range c.Libraries {
		if p == l.Path || strings.HasPrefix(p, l.Path+"/") {
			return l, true
		}
	}
	return Library{}, false
}

// Writable 判断某路径是否允许写操作。
func (c *Config) Writable(p string) bool {
	if c.Server.Readonly {
		return false
	}
	lib, ok := c.LibraryByPath(p)
	if !ok {
		return false
	}
	return !lib.ReadOnly
}

// --- 取值辅助 ---

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func slice(v any) ([]any, bool) {
	a, ok := v.([]any)
	return a, ok
}

func str(v any, def string) string {
	if s, ok := strv(v); ok {
		return s
	}
	return def
}

func strv(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case nil:
		return "", false
	default:
		return fmt.Sprintf("%v", t), true
	}
}

func num(v any, def int) int {
	switch t := v.(type) {
	case int64:
		return int(t)
	case int:
		return t
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}

func boolean(v any, def bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "yes", "on", "1":
			return true
		case "false", "no", "off", "0":
			return false
		}
	}
	return def
}

func list(v any) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok {
		if s, ok2 := strv(v); ok2 && s != "" {
			return splitCSV(s), true
		}
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := strv(item); ok && s != "" {
			out = append(out, s)
		}
	}
	return out, true
}

func intlist(v any) ([]int, bool) {
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]int, 0, len(arr))
	for _, item := range arr {
		n := num(item, 0)
		if n > 0 {
			out = append(out, n)
		}
	}
	return out, true
}

func splitCSV(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
