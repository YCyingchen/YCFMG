package store

import (
	"database/sql"
	"strings"
	"time"
)

// Share 是分享记录。
type Share struct {
	ID            int64  `json:"id"`
	Token         string `json:"token"`
	Title         string `json:"title"`
	Descr         string `json:"descr"`
	Kind          string `json:"kind"`
	Items         string `json:"-"`
	PasswordHash  string `json:"-"`
	ExpireAt      int64  `json:"expire_at"`
	MaxDownload   int    `json:"max_download"`
	DownloadCount int    `json:"download_count"`
	ViewCount     int    `json:"view_count"`
	AllowDownload bool   `json:"allow_download"`
	ShowWatermark bool   `json:"show_watermark"`
	CreatedAt     int64  `json:"created_at"`
	Creator       string `json:"creator"`
	Disabled      bool   `json:"disabled"`
	Domains       string `json:"domains"`
	HasPassword   bool   `json:"has_password"`
	Expired       bool   `json:"expired"`
}

const shareCols = "id, token, title, descr, kind, items, password_hash, expire_at, max_download, download_count, view_count, allow_download, show_watermark, created_at, creator, disabled, domains"

func scanShare(sc scanner) (Share, error) {
	var sh Share
	err := sc.Scan(&sh.ID, &sh.Token, &sh.Title, &sh.Descr, &sh.Kind, &sh.Items, &sh.PasswordHash,
		&sh.ExpireAt, &sh.MaxDownload, &sh.DownloadCount, &sh.ViewCount, &sh.AllowDownload,
		&sh.ShowWatermark, &sh.CreatedAt, &sh.Creator, &sh.Disabled, &sh.Domains)
	if err != nil {
		return sh, err
	}
	sh.HasPassword = sh.PasswordHash != ""
	sh.Expired = sh.ExpireAt > 0 && sh.ExpireAt < time.Now().Unix()
	return sh, nil
}

// CreateShare 新建分享。
func (s *Store) CreateShare(sh *Share) error {
	if sh.CreatedAt == 0 {
		sh.CreatedAt = time.Now().Unix()
	}
	res, err := s.db.Exec("INSERT INTO shares (token, title, descr, kind, items, password_hash, expire_at, max_download, allow_download, show_watermark, created_at, creator, disabled, domains) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		sh.Token, sh.Title, sh.Descr, sh.Kind, sh.Items, sh.PasswordHash, sh.ExpireAt, sh.MaxDownload,
		sh.AllowDownload, sh.ShowWatermark, sh.CreatedAt, sh.Creator, sh.Disabled, sh.Domains)
	if err != nil {
		return err
	}
	sh.ID, _ = res.LastInsertId()
	sh.HasPassword = sh.PasswordHash != ""
	return nil
}

// GetShareByToken 按令牌查询分享（是否可用由上层判定）。
func (s *Store) GetShareByToken(token string) (*Share, error) {
	sh, err := scanShare(s.db.QueryRow("SELECT "+shareCols+" FROM shares WHERE token = ?", token))
	if err != nil {
		return nil, err
	}
	return &sh, nil
}

// GetShareByID 按主键查询分享。
func (s *Store) GetShareByID(id int64) (*Share, error) {
	sh, err := scanShare(s.db.QueryRow("SELECT "+shareCols+" FROM shares WHERE id = ?", id))
	if err != nil {
		return nil, err
	}
	return &sh, nil
}

// ListShares 分页列出分享。
func (s *Store) ListShares(q string, limit, offset int) ([]Share, int, error) {
	where := ""
	args := []any{}
	if q != "" {
		where = " WHERE title LIKE ? OR descr LIKE ? OR token = ?"
		args = append(args, "%"+q+"%", "%"+q+"%", q)
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM shares"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	sqlStr := "SELECT " + shareCols + " FROM shares" + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Share{}
	for rows.Next() {
		sh, err := scanShare(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, sh)
	}
	return out, total, rows.Err()
}

// UpdateShare 更新分享的可编辑字段。
func (s *Store) UpdateShare(id int64, fields map[string]any) error {
	allowed := map[string]bool{"title": true, "descr": true, "password_hash": true, "expire_at": true,
		"max_download": true, "allow_download": true, "show_watermark": true, "disabled": true, "items": true, "domains": true}
	sets := []string{}
	args := []any{}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		sets = append(sets, k+" = ?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := s.db.Exec("UPDATE shares SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

// DeleteShare 删除分享及其访问记录。
func (s *Store) DeleteShare(id int64) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM share_visits WHERE share_id = ?", id); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM shares WHERE id = ?", id)
		return err
	})
}

// IncShareView 增加浏览次数。
func (s *Store) IncShareView(id int64) error {
	_, err := s.db.Exec("UPDATE shares SET view_count = view_count + 1 WHERE id = ?", id)
	return err
}

// IncShareDownload 增加下载次数并返回最新值。
func (s *Store) IncShareDownload(id int64) (int, error) {
	var n int
	err := s.db.QueryRow("UPDATE shares SET download_count = download_count + 1 WHERE id = ? RETURNING download_count", id).Scan(&n)
	return n, err
}

// AddVisit 记录一次访问。
func (s *Store) AddVisit(shareID int64, ip, ua, action string) error {
	_, err := s.db.Exec("INSERT INTO share_visits (share_id, ip, ua, action, at) VALUES (?,?,?,?,?)",
		shareID, ip, truncateStr(ua, 300), action, time.Now().Unix())
	return err
}

// VisitStat 是访问统计行。
type VisitStat struct {
	IP     string `json:"ip"`
	UA     string `json:"ua"`
	Action string `json:"action"`
	At     int64  `json:"at"`
}

// ListVisits 列出某个分享的最近访问。
func (s *Store) ListVisits(shareID int64, limit int) []VisitStat {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query("SELECT ip, ua, action, at FROM share_visits WHERE share_id = ? ORDER BY at DESC LIMIT ?", shareID, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []VisitStat{}
	for rows.Next() {
		var v VisitStat
		if err := rows.Scan(&v.IP, &v.UA, &v.Action, &v.At); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

// ShareOverview 汇总分享总体数据。
func (s *Store) ShareOverview() map[string]any {
	out := map[string]any{}
	var total, views, downloads, active int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM shares").Scan(&total)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(view_count),0) FROM shares").Scan(&views)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(download_count),0) FROM shares").Scan(&downloads)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM shares WHERE disabled = 0 AND (expire_at = 0 OR expire_at > ?)", time.Now().Unix()).Scan(&active)
	out["total"] = total
	out["active"] = active
	out["views"] = views
	out["downloads"] = downloads
	return out
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
