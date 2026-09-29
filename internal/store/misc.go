package store

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"
)

// User 是登录账号。
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
	LastLogin    int64  `json:"last_login"`
}

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("记录不存在")

// CountUsers 返回账号数量。
func (s *Store) CountUsers() int {
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n
}

// CreateUser 新建账号。
func (s *Store) CreateUser(u *User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = time.Now().Unix()
	}
	if u.Role == "" {
		u.Role = "admin"
	}
	res, err := s.db.Exec("INSERT INTO users (username, password_hash, role, created_at, last_login) VALUES (?,?,?,?,?)",
		u.Username, u.PasswordHash, u.Role, u.CreatedAt, u.LastLogin)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

// GetUser 按用户名查询账号。
func (s *Store) GetUser(username string) (*User, error) {
	var u User
	err := s.db.QueryRow("SELECT id, username, password_hash, role, created_at, last_login FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.LastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUserPassword 更新密码哈希。
func (s *Store) UpdateUserPassword(username, hash string) error {
	_, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", hash, username)
	return err
}

// TouchLogin 记录最后一次登录时间。
func (s *Store) TouchLogin(username string) {
	_, _ = s.db.Exec("UPDATE users SET last_login = ? WHERE username = ?", time.Now().Unix(), username)
}

// ListUsers 列出全部账号。
func (s *Store) ListUsers() []User {
	rows, err := s.db.Query("SELECT id, username, password_hash, role, created_at, last_login FROM users ORDER BY id")
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.LastLogin); err != nil {
			continue
		}
		out = append(out, u)
	}
	return out
}

// Session 是登录会话。
type Session struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
	ExpireAt  int64  `json:"expire_at"`
	IP        string `json:"ip"`
	UA        string `json:"ua"`
}

// CreateSession 新建会话。
func (s *Store) CreateSession(sess *Session) error {
	if sess.CreatedAt == 0 {
		sess.CreatedAt = time.Now().Unix()
	}
	_, err := s.db.Exec("INSERT INTO sessions (id, username, created_at, expire_at, ip, ua) VALUES (?,?,?,?,?,?)",
		sess.ID, sess.Username, sess.CreatedAt, sess.ExpireAt, sess.IP, truncateStr(sess.UA, 300))
	return err
}

// GetSession 查询有效会话。
func (s *Store) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow("SELECT id, username, created_at, expire_at, ip, ua FROM sessions WHERE id = ? AND expire_at > ?", id, time.Now().Unix()).
		Scan(&sess.ID, &sess.Username, &sess.CreatedAt, &sess.ExpireAt, &sess.IP, &sess.UA)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteSession 注销会话。
func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

// CleanSessions 清理过期会话。
func (s *Store) CleanSessions() {
	_, _ = s.db.Exec("DELETE FROM sessions WHERE expire_at <= ?", time.Now().Unix())
}

// Tag 是标签。
type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Auto  bool   `json:"auto"`
	Count int    `json:"count"`
}

// EnsureTag 确保标签存在并返回 ID。
func (s *Store) EnsureTag(name, color string, auto bool) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("标签名不能为空")
	}
	autoInt := 0
	if auto {
		autoInt = 1
	}
	if _, err := s.db.Exec("INSERT INTO tags (name, color, auto) VALUES (?,?,?) ON CONFLICT(name) DO NOTHING", name, color, autoInt); err != nil {
		return 0, err
	}
	var id int64
	if err := s.db.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

// TagFile 给文件打标签。
func (s *Store) TagFile(fileID, tagID int64) error {
	_, err := s.db.Exec("INSERT INTO file_tags (file_id, tag_id) VALUES (?,?) ON CONFLICT DO NOTHING", fileID, tagID)
	return err
}

// UntagFile 去掉文件标签。
func (s *Store) UntagFile(fileID, tagID int64) error {
	_, err := s.db.Exec("DELETE FROM file_tags WHERE file_id = ? AND tag_id = ?", fileID, tagID)
	return err
}

// FileTags 返回文件的标签列表。
func (s *Store) FileTags(fileID int64) []Tag {
	rows, err := s.db.Query("SELECT t.id, t.name, t.color, t.auto, t.count FROM tags t JOIN file_tags ft ON ft.tag_id = t.id WHERE ft.file_id = ? ORDER BY t.name", fileID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Auto, &t.Count); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// ListTags 列出标签（按关联数量倒序）。
func (s *Store) ListTags(autoOnly *bool) []Tag {
	q := "SELECT id, name, color, auto, count FROM tags"
	if autoOnly != nil {
		if *autoOnly {
			q += " WHERE auto = 1"
		} else {
			q += " WHERE auto = 0"
		}
	}
	q += " ORDER BY count DESC, name ASC"
	rows, err := s.db.Query(q)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Auto, &t.Count); err == nil {
			out = append(out, t)
		}
	}
	return out
}

// RebuildTagCounts 重算标签关联数量。
func (s *Store) RebuildTagCounts() error {
	_, err := s.db.Exec("UPDATE tags SET count = (SELECT COUNT(*) FROM file_tags ft WHERE ft.tag_id = tags.id)")
	return err
}

// AddFileTag 按标签名给文件打标签（自动建标签）。
func (s *Store) AddFileTag(fileID int64, name, color string, auto bool) error {
	id, err := s.EnsureTag(name, color, auto)
	if err != nil {
		return err
	}
	return s.TagFile(fileID, id)
}

// DeleteTag 删除标签及其关联。
func (s *Store) DeleteTag(id int64) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM file_tags WHERE tag_id = ?", id); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM tags WHERE id = ?", id)
		return err
	})
}

// ScanJob 是一次索引任务。
type ScanJob struct {
	ID         int64  `json:"id"`
	Lib        string `json:"lib"`
	Root       string `json:"root"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	Scanned    int    `json:"scanned"`
	Added      int    `json:"added"`
	Updated    int    `json:"updated"`
	Removed    int    `json:"removed"`
	Failed     int    `json:"failed"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

// StartJob 创建索引任务记录。
func (s *Store) StartJob(lib, root string) (int64, error) {
	res, err := s.db.Exec("INSERT INTO scan_jobs (lib, root, started_at, status) VALUES (?,?,?,?)", lib, root, time.Now().Unix(), "running")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishJob 结束索引任务。
func (s *Store) FinishJob(id int64, status string, scanned, added, updated, removed, failed int, message string) error {
	_, err := s.db.Exec("UPDATE scan_jobs SET finished_at = ?, status = ?, scanned = ?, added = ?, updated = ?, removed = ?, failed = ?, message = ? WHERE id = ?",
		time.Now().Unix(), status, scanned, added, updated, removed, failed, truncateStr(message, 500), id)
	return err
}

// RecentJobs 最近索引任务。
func (s *Store) RecentJobs(limit int) []ScanJob {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query("SELECT id, lib, root, started_at, finished_at, scanned, added, updated, removed, failed, status, message FROM scan_jobs ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []ScanJob{}
	for rows.Next() {
		var j ScanJob
		if err := rows.Scan(&j.ID, &j.Lib, &j.Root, &j.StartedAt, &j.FinishedAt, &j.Scanned, &j.Added,
			&j.Updated, &j.Removed, &j.Failed, &j.Status, &j.Message); err == nil {
			out = append(out, j)
		}
	}
	return out
}

// SaveEmbedding 保存图像向量（语义检索用）。
func (s *Store) SaveEmbedding(fileID int64, model string, vec []byte, dim int) error {
	_, err := s.db.Exec("INSERT INTO embeddings (file_id, model, dim, vec, updated_at) VALUES (?,?,?,?,?) "+
		"ON CONFLICT(file_id) DO UPDATE SET model = excluded.model, dim = excluded.dim, vec = excluded.vec, updated_at = excluded.updated_at",
		fileID, model, dim, vec, time.Now().Unix())
	return err
}

// LoadEmbeddings 载入全部向量。
func (s *Store) LoadEmbeddings(model string) map[int64][]float32 {
	rows, err := s.db.Query("SELECT file_id, dim, vec FROM embeddings WHERE model = ?", model)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[int64][]float32{}
	for rows.Next() {
		var id int64
		var dim int
		var raw []byte
		if err := rows.Scan(&id, &dim, &raw); err != nil {
			continue
		}
		out[id] = BytesToFloats(raw)
	}
	return out
}

// CountEmbeddings 统计已生成向量的文件数。
func (s *Store) CountEmbeddings(model string) int {
	var n int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM embeddings WHERE model = ?", model).Scan(&n)
	return n
}

// BytesToFloats 把 little-endian float32 字节流还原为切片。
func BytesToFloats(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		out[i] = math.Float32frombits(bits)
	}
	return out
}
