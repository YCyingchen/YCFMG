package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"
)

// File 是被索引的文件的完整记录。
type File struct {
	ID          int64   `json:"id"`
	Lib         string  `json:"lib"`
	Path        string  `json:"-"`
	Parent      string  `json:"-"`
	Name        string  `json:"name"`
	Ext         string  `json:"ext"`
	Kind        string  `json:"kind"`
	Size        int64   `json:"size"`
	MTime       int64   `json:"mtime"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	HashA       uint64  `json:"-"`
	HashD       uint64  `json:"-"`
	HashP       uint64  `json:"-"`
	Color       string  `json:"color"`
	Hist        []byte  `json:"-"`
	Aspect      float64 `json:"aspect"`
	Bright      float64 `json:"bright"`
	Sharp       float64 `json:"sharp"`
	Camera      string  `json:"camera"`
	Lens        string  `json:"lens"`
	ISO         int     `json:"iso"`
	FNum        string  `json:"fnum"`
	Exposure    string  `json:"exposure"`
	Focal       string  `json:"focal"`
	TakenAt     int64   `json:"taken_at"`
	GPSLat      float64 `json:"gps_lat"`
	GPSLon      float64 `json:"gps_lon"`
	Orientation int     `json:"orientation"`
	Classify    string  `json:"classify"`
	Tags        string  `json:"tags"`
	QuickHash   string  `json:"quick_hash"`
	IndexedAt   int64   `json:"indexed_at"`
	Missing     bool    `json:"missing"`
	Favorite    bool    `json:"favorite"`
	Rating      int     `json:"rating"`
	Hidden      bool    `json:"hidden"`
	Note        string  `json:"note"`
}

// 查询用列（含主键）。
const fileCols = "id, lib, path, parent, name, ext, kind, size, mtime, width, height, hash_a, hash_d, hash_p, color, hist, aspect, bright, sharp, camera, lens, iso, fnum, exposure, focal, taken_at, gps_lat, gps_lon, orientation, classify, tags, quick_hash, indexed_at, missing, favorite, rating, hidden, note"

// 插入用列（不含主键，交由 SQLite 自增）。
const fileInsertCols = "lib, path, parent, name, ext, kind, size, mtime, width, height, hash_a, hash_d, hash_p, color, hist, aspect, bright, sharp, camera, lens, iso, fnum, exposure, focal, taken_at, gps_lat, gps_lon, orientation, classify, tags, quick_hash, indexed_at, missing, favorite, rating, hidden, note"

const filePlaceholders = "?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?"

const fileUpsertSet = "lib=excluded.lib, path=excluded.path, parent=excluded.parent, name=excluded.name, ext=excluded.ext, kind=excluded.kind, size=excluded.size, mtime=excluded.mtime, width=excluded.width, height=excluded.height, hash_a=excluded.hash_a, hash_d=excluded.hash_d, hash_p=excluded.hash_p, color=excluded.color, hist=excluded.hist, aspect=excluded.aspect, bright=excluded.bright, sharp=excluded.sharp, camera=excluded.camera, lens=excluded.lens, iso=excluded.iso, fnum=excluded.fnum, exposure=excluded.exposure, focal=excluded.focal, taken_at=excluded.taken_at, gps_lat=excluded.gps_lat, gps_lon=excluded.gps_lon, orientation=excluded.orientation, classify=excluded.classify, tags=excluded.tags, quick_hash=excluded.quick_hash, indexed_at=excluded.indexed_at, missing=0"

const fileUpsertSQL = "INSERT INTO files (" + fileInsertCols + ") VALUES (" + filePlaceholders + ") ON CONFLICT(path) DO UPDATE SET " + fileUpsertSet

type scanner interface {
	Scan(dest ...any) error
}

func scanFile(sc scanner) (File, error) {
	var f File
	var ha, hd, hp int64
	err := sc.Scan(&f.ID, &f.Lib, &f.Path, &f.Parent, &f.Name, &f.Ext, &f.Kind, &f.Size, &f.MTime,
		&f.Width, &f.Height, &ha, &hd, &hp, &f.Color, &f.Hist, &f.Aspect, &f.Bright, &f.Sharp,
		&f.Camera, &f.Lens, &f.ISO, &f.FNum, &f.Exposure, &f.Focal, &f.TakenAt, &f.GPSLat, &f.GPSLon,
		&f.Orientation, &f.Classify, &f.Tags, &f.QuickHash, &f.IndexedAt, &f.Missing, &f.Favorite,
		&f.Rating, &f.Hidden, &f.Note)
	if err != nil {
		return f, err
	}
	f.HashA = uint64(ha)
	f.HashD = uint64(hd)
	f.HashP = uint64(hp)
	return f, nil
}

func fileArgs(f *File) []any {
	if f.IndexedAt == 0 {
		f.IndexedAt = time.Now().Unix()
	}
	return []any{f.Lib, f.Path, f.Parent, f.Name, f.Ext, f.Kind, f.Size, f.MTime,
		f.Width, f.Height, int64(f.HashA), int64(f.HashD), int64(f.HashP), f.Color, f.Hist,
		f.Aspect, f.Bright, f.Sharp, f.Camera, f.Lens, f.ISO, f.FNum, f.Exposure, f.Focal,
		f.TakenAt, f.GPSLat, f.GPSLon, f.Orientation, f.Classify, f.Tags, f.QuickHash,
		f.IndexedAt, f.Missing, f.Favorite, f.Rating, f.Hidden, f.Note}
}

// UpsertFile 插入或更新一条文件记录（按 path 唯一）。
func (s *Store) UpsertFile(f *File) error {
	_, err := s.db.Exec(fileUpsertSQL, fileArgs(f)...)
	return err
}

// UpsertFiles 批量写入文件记录，单事务提交。
func (s *Store) UpsertFiles(list []*File) error {
	if len(list) == 0 {
		return nil
	}
	return s.Tx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(fileUpsertSQL)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, f := range list {
			if _, err := stmt.Exec(fileArgs(f)...); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetFileByPath 按绝对路径获取记录。
func (s *Store) GetFileByPath(p string) (*File, error) {
	f, err := scanFile(s.db.QueryRow("SELECT "+fileCols+" FROM files WHERE path = ?", p))
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// GetFileByID 按主键获取记录。
func (s *Store) GetFileByID(id int64) (*File, error) {
	f, err := scanFile(s.db.QueryRow("SELECT "+fileCols+" FROM files WHERE id = ?", id))
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteFileByPath 删除记录。
func (s *Store) DeleteFileByPath(p string) error {
	_, err := s.db.Exec("DELETE FROM files WHERE path = ?", p)
	return err
}

// UpdateFileMeta 更新用户的个性化字段（收藏、评分、隐藏、备注、标签）。
func (s *Store) UpdateFileMeta(id int64, fields map[string]any) error {
	allowed := map[string]bool{"favorite": true, "rating": true, "hidden": true, "note": true, "tags": true, "classify": true}
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
	_, err := s.db.Exec("UPDATE files SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
	return err
}

// FileQuery 是文件/图片列表查询条件。
type FileQuery struct {
	Lib        string
	Kind       string
	Parent     string
	PathPrefix string
	NameLike   string
	Tag        string
	Classify   string
	Color      string
	Camera     string
	Favorite   *bool
	Hidden     *bool
	Missing    *bool
	MinRating  int
	TakenFrom  int64
	TakenTo    int64
	SizeMin    int64
	SizeMax    int64
	Sort       string
	Desc       bool
	Limit      int
	Offset     int
}

func (q FileQuery) where() (string, []any) {
	conds := []string{}
	args := []any{}
	add := func(cond string, vals ...any) {
		conds = append(conds, cond)
		args = append(args, vals...)
	}
	if q.Missing != nil {
		if *q.Missing {
			add("missing = 1")
		} else {
			add("missing = 0")
		}
	}
	if q.Lib != "" {
		add("lib = ?", q.Lib)
	}
	if q.Kind != "" {
		if strings.Contains(q.Kind, ",") {
			parts := strings.Split(q.Kind, ",")
			ph := strings.TrimSuffix(strings.Repeat("?,", len(parts)), ",")
			vals := make([]any, 0, len(parts))
			for _, p := range parts {
				vals = append(vals, strings.TrimSpace(p))
			}
			add("kind IN ("+ph+")", vals...)
		} else {
			add("kind = ?", q.Kind)
		}
	}
	if q.Parent != "" {
		add("parent = ?", q.Parent)
	}
	if q.PathPrefix != "" {
		add("path LIKE ?", strings.TrimRight(q.PathPrefix, "/")+"/%")
	}
	if q.NameLike != "" {
		add("(name LIKE ? OR path LIKE ?)", "%"+q.NameLike+"%", "%"+q.NameLike+"%")
	}
	if q.Tag != "" {
		add("id IN (SELECT file_id FROM file_tags ft JOIN tags t ON t.id = ft.tag_id WHERE t.name = ?)", q.Tag)
	}
	if q.Classify != "" {
		add("classify = ?", q.Classify)
	}
	if q.Color != "" {
		add("color = ?", q.Color)
	}
	if q.Camera != "" {
		add("camera = ?", q.Camera)
	}
	if q.Favorite != nil {
		if *q.Favorite {
			add("favorite = 1")
		} else {
			add("favorite = 0")
		}
	}
	if q.Hidden != nil {
		if *q.Hidden {
			add("hidden = 1")
		} else {
			add("hidden = 0")
		}
	}
	if q.MinRating > 0 {
		add("rating >= ?", q.MinRating)
	}
	if q.TakenFrom > 0 {
		add("taken_at >= ?", q.TakenFrom)
	}
	if q.TakenTo > 0 {
		add("taken_at <= ?", q.TakenTo)
	}
	if q.SizeMin > 0 {
		add("size >= ?", q.SizeMin)
	}
	if q.SizeMax > 0 {
		add("size <= ?", q.SizeMax)
	}
	return strings.Join(conds, " AND "), args
}

func (q FileQuery) orderBy() string {
	dir := " ASC"
	if q.Desc {
		dir = " DESC"
	}
	switch q.Sort {
	case "size":
		return "size" + dir
	case "mtime":
		return "mtime" + dir
	case "name":
		return "name COLLATE NOCASE" + dir
	case "rating":
		return "rating" + dir + ", COALESCE(NULLIF(taken_at,0), mtime) DESC"
	case "type":
		return "ext" + dir + ", name COLLATE NOCASE ASC"
	case "random":
		return "RANDOM()"
	case "dimension":
		return "(width * height)" + dir
	default:
		return "COALESCE(NULLIF(taken_at,0), mtime)" + dir
	}
}

// QueryFiles 分页查询文件，返回列表与总数。
func (s *Store) QueryFiles(q FileQuery) ([]File, int, error) {
	where, args := q.where()
	clause := ""
	if where != "" {
		clause = " WHERE " + where
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM files"+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 2000 {
		limit = 2000
	}
	sqlStr := "SELECT " + fileCols + " FROM files" + clause + " ORDER BY " + q.orderBy() + " LIMIT ? OFFSET ?"
	args = append(args, limit, q.Offset)
	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]File, 0, limit)
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}

// ImageFeature 是检索所需的图像特征行。
type ImageFeature struct {
	ID     int64
	Name   string
	Parent string
	Kind   string
	Size   int64
	MTime  int64
	Taken  int64
	Width  int
	Height int
	HashA  uint64
	HashD  uint64
	HashP  uint64
	Color  string
	Hist   []byte
	Class  string
	Tags   string
	Camera string
}

// StreamImageFeatures 流式遍历所有图片/视频特征，避免一次性载入内存。
func (s *Store) StreamImageFeatures(lib string, fn func(ImageFeature) error) error {
	q := "SELECT id, name, parent, kind, size, mtime, taken_at, width, height, hash_a, hash_d, hash_p, color, hist, classify, tags, camera FROM files WHERE missing = 0 AND kind IN ('image','video')"
	args := []any{}
	if lib != "" {
		q += " AND lib = ?"
		args = append(args, lib)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var it ImageFeature
		var ha, hd, hp int64
		if err := rows.Scan(&it.ID, &it.Name, &it.Parent, &it.Kind, &it.Size, &it.MTime, &it.Taken,
			&it.Width, &it.Height, &ha, &hd, &hp, &it.Color, &it.Hist, &it.Class, &it.Tags, &it.Camera); err != nil {
			return err
		}
		it.HashA, it.HashD, it.HashP = uint64(ha), uint64(hd), uint64(hp)
		if err := fn(it); err != nil {
			return err
		}
	}
	return rows.Err()
}

// MarkMissingUnder 把某目录下磁盘上已不存在的记录标记为丢失。
func (s *Store) MarkMissingUnder(prefix string, keep map[string]bool) error {
	rows, err := s.db.Query("SELECT path FROM files WHERE path = ? OR path LIKE ?", prefix, strings.TrimRight(prefix, "/")+"/%")
	if err != nil {
		return err
	}
	paths := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		if !keep[p] {
			paths = append(paths, p)
		}
	}
	rows.Close()
	if len(paths) == 0 {
		return nil
	}
	return s.Tx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare("UPDATE files SET missing = 1 WHERE path = ?")
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range paths {
			if _, err := stmt.Exec(p); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMissingUnder 清理某个目录下已标记丢失的记录。
func (s *Store) DeleteMissingUnder(prefix string) (int64, error) {
	res, err := s.db.Exec("DELETE FROM files WHERE missing = 1 AND (path = ? OR path LIKE ?)", prefix, strings.TrimRight(prefix, "/")+"/%")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Facet 是分组统计项。
type Facet struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	Size  int64  `json:"size"`
}

// Facets 按字段分组统计，用于图库的分类面板。
func (s *Store) Facets(field string, kind string, limit int) []Facet {
	allowed := map[string]string{
		"camera":   "camera",
		"lens":     "lens",
		"classify": "classify",
		"color":    "color",
		"lib":      "lib",
		"ext":      "ext",
		"year":     "strftime('%Y', COALESCE(NULLIF(taken_at,0), mtime), 'unixepoch', 'localtime')",
		"month":    "strftime('%Y-%m', COALESCE(NULLIF(taken_at,0), mtime), 'unixepoch', 'localtime')",
		"day":      "strftime('%Y-%m-%d', COALESCE(NULLIF(taken_at,0), mtime), 'unixepoch', 'localtime')",
		"hour":     "strftime('%H', COALESCE(NULLIF(taken_at,0), mtime), 'unixepoch', 'localtime')",
	}
	col, ok := allowed[field]
	if !ok {
		return nil
	}
	if limit <= 0 {
		limit = 200
	}
	q := "SELECT " + col + " AS k, COUNT(*), COALESCE(SUM(size),0) FROM files WHERE missing = 0 AND k IS NOT NULL AND k <> ''"
	args := []any{}
	if kind != "" {
		q += " AND kind = ?"
		args = append(args, kind)
	}
	q += " GROUP BY k ORDER BY COUNT(*) DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []Facet{}
	for rows.Next() {
		var f Facet
		if err := rows.Scan(&f.Key, &f.Count, &f.Size); err != nil {
			continue
		}
		out = append(out, f)
	}
	return out
}

// DirEntry 是被索引的目录。
type DirEntry struct {
	Path       string `json:"path"`
	Lib        string `json:"lib"`
	Parent     string `json:"parent"`
	Name       string `json:"name"`
	MTime      int64  `json:"mtime"`
	FileCount  int    `json:"file_count"`
	ImageCount int    `json:"image_count"`
	Cover      string `json:"cover"`
}

// UpsertDir 写入目录记录。
func (s *Store) UpsertDir(d *DirEntry) error {
	_, err := s.db.Exec("INSERT INTO dirs(path, lib, parent, name, mtime, file_count, image_count, cover) VALUES(?,?,?,?,?,?,?,?) "+
		"ON CONFLICT(path) DO UPDATE SET lib=excluded.lib, parent=excluded.parent, name=excluded.name, mtime=excluded.mtime, "+
		"file_count=excluded.file_count, image_count=excluded.image_count, cover=excluded.cover",
		d.Path, d.Lib, d.Parent, d.Name, d.MTime, d.FileCount, d.ImageCount, d.Cover)
	return err
}

// SubDirs 列出已索引的子目录。
func (s *Store) SubDirs(parent string) []DirEntry {
	rows, err := s.db.Query("SELECT path, lib, parent, name, mtime, file_count, image_count, cover FROM dirs WHERE parent = ? ORDER BY name COLLATE NOCASE", parent)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []DirEntry{}
	for rows.Next() {
		var d DirEntry
		if err := rows.Scan(&d.Path, &d.Lib, &d.Parent, &d.Name, &d.MTime, &d.FileCount, &d.ImageCount, &d.Cover); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}

// DeleteDirsUnder 删除某个目录下的目录索引。
func (s *Store) DeleteDirsUnder(prefix string) error {
	_, err := s.db.Exec("DELETE FROM dirs WHERE path = ? OR path LIKE ?", prefix, strings.TrimRight(prefix, "/")+"/%")
	return err
}

// ParentOf 返回路径的父目录。
func ParentOf(p string) string { return filepath.Dir(p) }
