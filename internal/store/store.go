// Package store 封装 SQLite 持久层（纯 Go 驱动，无需 cgo）。
package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store 是数据库句柄包装。
type Store struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// Open 打开（并在需要时创建）数据库，执行迁移。
func Open(path string) (*Store, error) {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(8000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层句柄（供高级查询使用）。
func (s *Store) DB() *sql.DB { return s.db }

// Path 返回数据库文件路径。
func (s *Store) Path() string { return s.path }

func ensureDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	return mkdirAll(dir)
}

const schema = "" +
	"CREATE TABLE IF NOT EXISTS files (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" lib TEXT NOT NULL DEFAULT ''," +
	" path TEXT NOT NULL," +
	" parent TEXT NOT NULL DEFAULT ''," +
	" name TEXT NOT NULL DEFAULT ''," +
	" ext TEXT NOT NULL DEFAULT ''," +
	" kind TEXT NOT NULL DEFAULT 'other'," +
	" size INTEGER NOT NULL DEFAULT 0," +
	" mtime INTEGER NOT NULL DEFAULT 0," +
	" width INTEGER NOT NULL DEFAULT 0," +
	" height INTEGER NOT NULL DEFAULT 0," +
	" hash_a INTEGER NOT NULL DEFAULT 0," +
	" hash_d INTEGER NOT NULL DEFAULT 0," +
	" hash_p INTEGER NOT NULL DEFAULT 0," +
	" color TEXT NOT NULL DEFAULT ''," +
	" hist BLOB," +
	" aspect REAL NOT NULL DEFAULT 0," +
	" bright REAL NOT NULL DEFAULT 0," +
	" sharp REAL NOT NULL DEFAULT 0," +
	" camera TEXT NOT NULL DEFAULT ''," +
	" lens TEXT NOT NULL DEFAULT ''," +
	" iso INTEGER NOT NULL DEFAULT 0," +
	" fnum TEXT NOT NULL DEFAULT ''," +
	" exposure TEXT NOT NULL DEFAULT ''," +
	" focal TEXT NOT NULL DEFAULT ''," +
	" taken_at INTEGER NOT NULL DEFAULT 0," +
	" gps_lat REAL NOT NULL DEFAULT 0," +
	" gps_lon REAL NOT NULL DEFAULT 0," +
	" orientation INTEGER NOT NULL DEFAULT 0," +
	" classify TEXT NOT NULL DEFAULT ''," +
	" tags TEXT NOT NULL DEFAULT ''," +
	" quick_hash TEXT NOT NULL DEFAULT ''," +
	" indexed_at INTEGER NOT NULL DEFAULT 0," +
	" missing INTEGER NOT NULL DEFAULT 0," +
	" favorite INTEGER NOT NULL DEFAULT 0," +
	" rating INTEGER NOT NULL DEFAULT 0," +
	" hidden INTEGER NOT NULL DEFAULT 0," +
	" note TEXT NOT NULL DEFAULT ''," +
	" UNIQUE(path))" +
	";CREATE INDEX IF NOT EXISTS idx_files_parent ON files(parent)" +
	";CREATE INDEX IF NOT EXISTS idx_files_kind ON files(kind)" +
	";CREATE INDEX IF NOT EXISTS idx_files_taken ON files(taken_at)" +
	";CREATE INDEX IF NOT EXISTS idx_files_lib ON files(lib)" +
	";CREATE INDEX IF NOT EXISTS idx_files_qhash ON files(quick_hash)" +
	";CREATE INDEX IF NOT EXISTS idx_files_fav ON files(favorite)" +
	";CREATE TABLE IF NOT EXISTS dirs (" +
	" path TEXT PRIMARY KEY," +
	" lib TEXT NOT NULL DEFAULT ''," +
	" parent TEXT NOT NULL DEFAULT ''," +
	" name TEXT NOT NULL DEFAULT ''," +
	" mtime INTEGER NOT NULL DEFAULT 0," +
	" file_count INTEGER NOT NULL DEFAULT 0," +
	" image_count INTEGER NOT NULL DEFAULT 0," +
	" cover TEXT NOT NULL DEFAULT '')" +
	";CREATE INDEX IF NOT EXISTS idx_dirs_parent ON dirs(parent)" +
	";CREATE TABLE IF NOT EXISTS tags (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" name TEXT NOT NULL UNIQUE," +
	" color TEXT NOT NULL DEFAULT ''," +
	" auto INTEGER NOT NULL DEFAULT 0," +
	" count INTEGER NOT NULL DEFAULT 0)" +
	";CREATE TABLE IF NOT EXISTS file_tags (" +
	" file_id INTEGER NOT NULL," +
	" tag_id INTEGER NOT NULL," +
	" PRIMARY KEY(file_id, tag_id))" +
	";CREATE INDEX IF NOT EXISTS idx_file_tags_tag ON file_tags(tag_id)" +
	";CREATE TABLE IF NOT EXISTS shares (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" token TEXT NOT NULL UNIQUE," +
	" title TEXT NOT NULL DEFAULT ''," +
	" descr TEXT NOT NULL DEFAULT ''," +
	" kind TEXT NOT NULL DEFAULT 'files'," +
	" items TEXT NOT NULL DEFAULT '[]'," +
	" password_hash TEXT NOT NULL DEFAULT ''," +
	" expire_at INTEGER NOT NULL DEFAULT 0," +
	" max_download INTEGER NOT NULL DEFAULT 0," +
	" download_count INTEGER NOT NULL DEFAULT 0," +
	" view_count INTEGER NOT NULL DEFAULT 0," +
	" allow_download INTEGER NOT NULL DEFAULT 1," +
	" show_watermark INTEGER NOT NULL DEFAULT 0," +
	" created_at INTEGER NOT NULL DEFAULT 0," +
	" creator TEXT NOT NULL DEFAULT ''," +
	" disabled INTEGER NOT NULL DEFAULT 0," +
	" domains TEXT NOT NULL DEFAULT '')" +
	";CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token)" +
	";CREATE TABLE IF NOT EXISTS share_visits (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" share_id INTEGER NOT NULL," +
	" ip TEXT NOT NULL DEFAULT ''," +
	" ua TEXT NOT NULL DEFAULT ''," +
	" action TEXT NOT NULL DEFAULT 'view'," +
	" at INTEGER NOT NULL DEFAULT 0)" +
	";CREATE INDEX IF NOT EXISTS idx_visits_share ON share_visits(share_id)" +
	";CREATE TABLE IF NOT EXISTS users (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" username TEXT NOT NULL UNIQUE," +
	" password_hash TEXT NOT NULL DEFAULT ''," +
	" role TEXT NOT NULL DEFAULT 'admin'," +
	" created_at INTEGER NOT NULL DEFAULT 0," +
	" last_login INTEGER NOT NULL DEFAULT 0)" +
	";CREATE TABLE IF NOT EXISTS sessions (" +
	" id TEXT PRIMARY KEY," +
	" username TEXT NOT NULL DEFAULT ''," +
	" created_at INTEGER NOT NULL DEFAULT 0," +
	" expire_at INTEGER NOT NULL DEFAULT 0," +
	" ip TEXT NOT NULL DEFAULT ''," +
	" ua TEXT NOT NULL DEFAULT '')" +
	";CREATE TABLE IF NOT EXISTS settings (" +
	" skey TEXT PRIMARY KEY," +
	" svalue TEXT NOT NULL DEFAULT '')" +
	";CREATE TABLE IF NOT EXISTS scan_jobs (" +
	" id INTEGER PRIMARY KEY AUTOINCREMENT," +
	" lib TEXT NOT NULL DEFAULT ''," +
	" root TEXT NOT NULL DEFAULT ''," +
	" started_at INTEGER NOT NULL DEFAULT 0," +
	" finished_at INTEGER NOT NULL DEFAULT 0," +
	" scanned INTEGER NOT NULL DEFAULT 0," +
	" added INTEGER NOT NULL DEFAULT 0," +
	" updated INTEGER NOT NULL DEFAULT 0," +
	" removed INTEGER NOT NULL DEFAULT 0," +
	" failed INTEGER NOT NULL DEFAULT 0," +
	" status TEXT NOT NULL DEFAULT 'running'," +
	" message TEXT NOT NULL DEFAULT '')" +
	";CREATE TABLE IF NOT EXISTS embeddings (" +
	" file_id INTEGER PRIMARY KEY," +
	" model TEXT NOT NULL DEFAULT ''," +
	" dim INTEGER NOT NULL DEFAULT 0," +
	" vec BLOB," +
	" updated_at INTEGER NOT NULL DEFAULT 0)"

func (s *Store) migrate() error {
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("初始化数据库结构失败: %w", err)
	}
	return nil
}

// Tx 在事务中执行 fn。
func (s *Store) Tx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GetSetting 读取配置项。
func (s *Store) GetSetting(key, def string) string {
	var v string
	err := s.db.QueryRow("SELECT svalue FROM settings WHERE skey = ?", key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

// SetSetting 写入配置项。
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec("INSERT INTO settings(skey, svalue) VALUES(?, ?) ON CONFLICT(skey) DO UPDATE SET svalue = excluded.svalue", key, value)
	return err
}

// CountFiles 返回满足条件的文件数量。
func (s *Store) CountFiles(kind string) int {
	q := "SELECT COUNT(*) FROM files WHERE missing = 0"
	args := []any{}
	if kind != "" {
		q += " AND kind = ?"
		args = append(args, kind)
	}
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0
	}
	return n
}

// Stats 汇总统计信息。
func (s *Store) Stats() map[string]any {
	out := map[string]any{}
	rows, err := s.db.Query("SELECT kind, COUNT(*), COALESCE(SUM(size),0) FROM files WHERE missing = 0 GROUP BY kind")
	if err == nil {
		defer rows.Close()
		byKind := map[string]any{}
		total := 0
		var totalSize int64
		for rows.Next() {
			var kind string
			var n int
			var size int64
			if err := rows.Scan(&kind, &n, &size); err != nil {
				continue
			}
			byKind[kind] = map[string]any{"count": n, "size": size}
			total += n
			totalSize += size
		}
		out["by_kind"] = byKind
		out["total"] = total
		out["total_size"] = totalSize
	}
	out["shares"] = s.countRows("SELECT COUNT(*) FROM shares")
	out["tags"] = s.countRows("SELECT COUNT(*) FROM tags")
	return out
}

func (s *Store) countRows(q string, args ...any) int {
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0
	}
	return n
}

// Expr 是 SQL 条件表达式片段。
type Expr struct {
	SQL  string
	Args []any
}

// And 连接多个条件。
func And(parts ...Expr) Expr {
	sqls := []string{}
	args := []any{}
	for _, p := range parts {
		if strings.TrimSpace(p.SQL) == "" {
			continue
		}
		sqls = append(sqls, "("+p.SQL+")")
		args = append(args, p.Args...)
	}
	return Expr{SQL: strings.Join(sqls, " AND "), Args: args}
}
