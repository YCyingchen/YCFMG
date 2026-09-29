package store

import (
	"database/sql"
	"path/filepath"
	"strings"
)

// MovePath 在重命名或移动后同步索引：更新自身记录并让子项索引失效重建。
func (s *Store) MovePath(oldPath, newPath string) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE files SET path = ?, parent = ?, name = ? WHERE path = ?",
			newPath, filepath.Dir(newPath), filepath.Base(newPath), oldPath); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE dirs SET path = ?, parent = ?, name = ? WHERE path = ?",
			newPath, filepath.Dir(newPath), filepath.Base(newPath), oldPath); err != nil {
			return err
		}
		prefix := strings.TrimRight(oldPath, "/") + "/%"
		if _, err := tx.Exec("DELETE FROM files WHERE path LIKE ?", prefix); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM dirs WHERE path LIKE ?", prefix)
		return err
	})
}

// AllPaths 返回全部已索引文件路径（用于一致性巡检）。
func (s *Store) AllPaths() []string {
	rows, err := s.db.Query("SELECT path FROM files WHERE missing = 0")
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
