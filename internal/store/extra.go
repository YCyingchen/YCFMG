package store

// CountKind 统计某个文件库中指定类型的数量。
func (s *Store) CountKind(lib, kind string) int {
	q := "SELECT COUNT(*) FROM files WHERE missing = 0"
	args := []any{}
	if lib != "" {
		q += " AND lib = ?"
		args = append(args, lib)
	}
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

// LibrarySize 统计某个文件库占用的字节数。
func (s *Store) LibrarySize(lib string) int64 {
	var n int64
	if err := s.db.QueryRow("SELECT COALESCE(SUM(size),0) FROM files WHERE missing = 0 AND lib = ?", lib).Scan(&n); err != nil {
		return 0
	}
	return n
}

// SearchTextRows 拉取用于文本打分的候选行。
func (s *Store) SearchTextRows(like []string, lib, kind string, limit int) ([]File, error) {
	if len(like) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 5000 {
		limit = 3000
	}
	conds := []string{}
	args := []any{}
	for _, t := range like {
		conds = append(conds, "(name LIKE ? OR parent LIKE ? OR tags LIKE ? OR classify LIKE ? OR camera LIKE ? OR lens LIKE ? OR ext LIKE ?)")
		p := "%" + t + "%"
		args = append(args, p, p, p, p, p, p, p)
	}
	q := "SELECT " + fileCols + " FROM files WHERE missing = 0 AND (" + joinOr(conds) + ")"
	if lib != "" {
		q += " AND lib = ?"
		args = append(args, lib)
	}
	if kind != "" {
		q += " AND kind = ?"
		args = append(args, kind)
	}
	q += " LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func joinOr(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " OR "
		}
		out += p
	}
	return out
}

// AllTagsWithCount 返回标签及其关联数量。
func (s *Store) AllTagsWithCount() []Tag { return s.ListTags(nil) }
