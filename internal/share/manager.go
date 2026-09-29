// Package share 实现分享链接、专属分享页与多域名绑定。
package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ycyingchen/ycfmg/internal/auth"
	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
	"github.com/ycyingchen/ycfmg/internal/util"
)

// Manager 管理分享生命周期。
type Manager struct {
	cfg *config.Config
	st  *store.Store
	th  *thumbs.Cache
}

// New 创建分享管理器。
func New(cfg *config.Config, st *store.Store, th *thumbs.Cache) *Manager {
	return &Manager{cfg: cfg, st: st, th: th}
}

// Item 是分享中的一个条目。
type Item struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Kind   string `json:"kind"`
	IsDir  bool   `json:"is_dir"`
	MTime  int64  `json:"mtime"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	ID     int64  `json:"id"`
}

// CreateRequest 是创建分享的入参。
type CreateRequest struct {
	Title         string
	Descr         string
	Paths         []string
	Kind          string
	Password      string
	ExpireDays    int
	MaxDownload   int
	AllowDownload bool
	ShowWatermark bool
	Creator       string
}

// Create 创建分享记录。
func (m *Manager) Create(req CreateRequest, r *http.Request) (*store.Share, error) {
	clean := make([]Item, 0, len(req.Paths))
	for _, p := range req.Paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err == nil {
			p = abs
		}
		if !m.cfg.Writable(p) {
			if _, ok := m.cfg.LibraryByPath(p); !ok {
				return nil, fmt.Errorf("路径不在受管文件库内: %s", p)
			}
		}
		st, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("文件不存在: %s", p)
		}
		it := Item{
			Path:  p,
			Name:  st.Name(),
			Size:  st.Size(),
			IsDir: st.IsDir(),
			MTime: st.ModTime().Unix(),
			Kind:  util.KindOf(st.Name(), st.IsDir()),
		}
		if !st.IsDir() {
			if rec, err := m.st.GetFileByPath(p); err == nil {
				it.ID = rec.ID
				it.Width, it.Height = rec.Width, rec.Height
			}
		}
		clean = append(clean, it)
		if len(clean) >= m.cfg.Share.MaxItems {
			break
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("没有可分享的内容")
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	kind := req.Kind
	if kind == "" {
		kind = "files"
		if len(clean) == 1 && clean[0].IsDir {
			kind = "folder"
		}
	}
	expireAt := int64(0)
	days := req.ExpireDays
	if days == 0 {
		days = m.cfg.Share.DefaultExpireDays
	}
	if days > 0 {
		expireAt = time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
	}
	pwHash := ""
	if strings.TrimSpace(req.Password) != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		pwHash = string(h)
	}
	sh := &store.Share{
		Token:         util.Token(16),
		Title:         strings.TrimSpace(req.Title),
		Descr:         strings.TrimSpace(req.Descr),
		Kind:          kind,
		Items:         string(raw),
		PasswordHash:  pwHash,
		ExpireAt:      expireAt,
		MaxDownload:   req.MaxDownload,
		AllowDownload: req.AllowDownload,
		ShowWatermark: req.ShowWatermark,
		CreatedAt:     time.Now().Unix(),
		Creator:       req.Creator,
		Domains:       strings.Join(m.domainList(r), ","),
	}
	if sh.Title == "" {
		if len(clean) == 1 {
			sh.Title = clean[0].Name
			if clean[0].IsDir {
				sh.Title = clean[0].Name + " 文件夹"
			}
		} else {
			sh.Title = fmt.Sprintf("%d 个文件", len(clean))
		}
	}
	if err := m.st.CreateShare(sh); err != nil {
		return nil, err
	}
	return sh, nil
}

// ParseItems 解析分享条目。
func (m *Manager) ParseItems(sh *store.Share) []Item {
	items := []Item{}
	if sh == nil || sh.Items == "" {
		return items
	}
	if err := json.Unmarshal([]byte(sh.Items), &items); err != nil {
		return []Item{}
	}
	for i := range items {
		if st, err := os.Stat(items[i].Path); err == nil {
			items[i].Size = st.Size()
			items[i].MTime = st.ModTime().Unix()
		} else {
			items[i].Size = -1
		}
	}
	return items
}

// Check 校验分享可用性与密码。
func (m *Manager) Check(sh *store.Share, password string) error {
	if sh == nil {
		return errors.New("分享不存在")
	}
	if sh.Disabled {
		return errors.New("该分享已被停用")
	}
	if sh.ExpireAt > 0 && sh.ExpireAt < time.Now().Unix() {
		return errors.New("该分享已过期")
	}
	if sh.MaxDownload > 0 && sh.DownloadCount >= sh.MaxDownload {
		return errors.New("该分享已达到下载次数上限")
	}
	if sh.PasswordHash != "" {
		if password == "" {
			return ErrNeedPassword
		}
		if bcrypt.CompareHashAndPassword([]byte(sh.PasswordHash), []byte(password)) != nil {
			return errors.New("提取密码错误")
		}
	}
	return nil
}

// ErrNeedPassword 表示需要提取密码。
var ErrNeedPassword = errors.New("需要提取密码")

// Link 是一条可用的分享链接（对应一个域名）。
type Link struct {
	URL     string `json:"url"`
	Host    string `json:"host"`
	Primary bool   `json:"primary"`
	Current bool   `json:"current"`
}

// RequestHost 返回当前请求的 Host（考虑反代）。
func (m *Manager) RequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := r.Host
	if m.cfg.Server.TrustProxy {
		if v := r.Header.Get("X-Forwarded-Host"); v != "" {
			host = strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	return host
}

// RequestScheme 返回当前请求的协议。
func (m *Manager) RequestScheme(r *http.Request) string {
	if r != nil && auth.IsHTTPS(r, m.cfg.Server.TrustProxy) {
		return "https"
	}
	return "http"
}

func (m *Manager) domainList(r *http.Request) []string {
	out := []string{}
	seen := map[string]bool{}
	host := m.RequestHost(r)
	if host != "" {
		scheme := m.RequestScheme(r)
		u := scheme + "://" + host + m.cfg.Server.BasePath
		out = append(out, u)
		seen[u] = true
	}
	for _, u := range m.cfg.Server.PublicURLs {
		u = strings.TrimRight(strings.TrimSpace(u), "/")
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// BaseURL 返回用于生成链接的基地址（优先当前访问域名）。
func (m *Manager) BaseURL(r *http.Request) string {
	list := m.domainList(r)
	if len(list) > 0 {
		return list[0]
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", m.cfg.Server.Port, m.cfg.Server.BasePath)
}

// Links 返回某个分享在全部绑定域名下的链接。
func (m *Manager) Links(r *http.Request, token string) []Link {
	host := m.RequestHost(r)
	list := m.domainList(r)
	out := make([]Link, 0, len(list))
	for i, base := range list {
		out = append(out, Link{
			URL:     base + "/s/" + token,
			Host:    hostOf(base),
			Primary: i == 0,
			Current: host != "" && strings.Contains(base, host),
		})
	}
	return out
}

func hostOf(u string) string {
	s := u
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[i+2:]
	}
	if i := strings.IndexAny(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}

// Thumbs 暴露缩略图缓存。
func (m *Manager) Thumbs() *thumbs.Cache { return m.th }

// Store 暴露数据层。
func (m *Manager) Store() *store.Store { return m.st }

// Config 暴露配置。
func (m *Manager) Config() *config.Config { return m.cfg }
