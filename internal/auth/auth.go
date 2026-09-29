// Package auth 提供账号、会话与访问控制。
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/logx"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/util"
)

// CookieName 是会话 Cookie 名。
const CookieName = "ycfmg_session"

// Manager 管理登录与会话。
type Manager struct {
	cfg *config.Config
	st  *store.Store
}

// New 创建管理器。
func New(cfg *config.Config, st *store.Store) *Manager {
	return &Manager{cfg: cfg, st: st}
}

// EnsureAdmin 确保存在管理员账号；首次运行会生成随机密码并写回配置提示。
func (m *Manager) EnsureAdmin() (string, error) {
	username := m.cfg.Auth.Username
	if username == "" {
		username = "admin"
	}
	if m.st.CountUsers() > 0 {
		return "", nil
	}
	pwd := strings.TrimSpace(m.cfg.Auth.Password)
	generated := false
	if pwd == "" {
		pwd = util.Token(12)
		generated = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	if err := m.st.CreateUser(&store.User{Username: username, PasswordHash: string(hash), Role: "admin"}); err != nil {
		return "", err
	}
	if generated {
		logx.Infof("已创建管理员账号 %s，初始密码: %s（请登录后立即修改）", username, pwd)
		return pwd, nil
	}
	logx.Infof("已创建管理员账号 %s（使用配置中的密码）", username)
	return "", nil
}

// Login 校验账号密码并创建会话。
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, username, password string) (*store.Session, error) {
	u, err := m.st.GetUser(username)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("账号或密码错误")
	}
	ttl := m.cfg.Auth.SessionTTLHours
	if ttl <= 0 {
		ttl = 72
	}
	sess := &store.Session{
		ID:        util.Token(40),
		Username:  u.Username,
		CreatedAt: time.Now().Unix(),
		ExpireAt:  time.Now().Add(time.Duration(ttl) * time.Hour).Unix(),
		IP:        ClientIP(r, m.cfg.Server.TrustProxy),
		UA:        r.UserAgent(),
	}
	if err := m.st.CreateSession(sess); err != nil {
		return nil, err
	}
	m.st.TouchLogin(u.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sess.ID,
		Path:     m.cookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   IsHTTPS(r, m.cfg.Server.TrustProxy),
		Expires:  time.Unix(sess.ExpireAt, 0),
	})
	return sess, nil
}

func (m *Manager) cookiePath() string {
	p := m.cfg.Server.BasePath
	if p == "" {
		return "/"
	}
	return p
}

// Logout 注销当前会话。
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		_ = m.st.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   CookieName,
		Value:  "",
		Path:   m.cookiePath(),
		MaxAge: -1,
	})
}

// Current 返回当前登录会话，未登录返回 nil。
func (m *Manager) Current(r *http.Request) *store.Session {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	sess, err := m.st.GetSession(c.Value)
	if err != nil {
		return nil
	}
	return sess
}

// ChangePassword 修改当前账号密码。
func (m *Manager) ChangePassword(username, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return errors.New("新密码至少 6 位")
	}
	u, err := m.st.GetUser(username)
	if err != nil {
		return errors.New("账号不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)) != nil {
		return errors.New("原密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return m.st.UpdateUserPassword(username, string(hash))
}

// ClientIP 解析真实客户端 IP。
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if v := r.Header.Get("X-Real-IP"); v != "" {
			return strings.TrimSpace(v)
		}
		if v := r.Header.Get("X-Forwarded-For"); v != "" {
			parts := strings.Split(v, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

// IsHTTPS 判断当前请求是否为 HTTPS。
func IsHTTPS(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if trustProxy {
		if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			return true
		}
	}
	return false
}
