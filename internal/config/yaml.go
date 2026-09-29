package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	lfSeq   = "\n"
	crlfSeq = "\r\n"
)

// yamlLine 表示 YAML 文本中的一行。
type yamlLine struct {
	indent int
	text   string
	num    int
}

// ParseYAML 解析 YAML 的常用子集：缩进映射、缩进序列、行内序列 [a, b]、
// 标量（字符串/数字/布尔/null）与 # 注释。不支持锚点与多文档。
func ParseYAML(data []byte) (any, error) {
	raw := strings.ReplaceAll(string(data), crlfSeq, lfSeq)
	lines := make([]yamlLine, 0, 64)
	for i, ln := range strings.Split(raw, lfSeq) {
		trimmed := stripComment(ln)
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		indent := 0
		for indent < len(trimmed) && trimmed[indent] == 0x20 {
			indent++
		}
		if indent < len(trimmed) && trimmed[indent] == 0x09 {
			indent += 2
		}
		lines = append(lines, yamlLine{indent: indent, text: strings.TrimRight(trimmed[indent:], " \t"), num: i + 1})
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}
	v, next, err := parseBlock(lines, 0, lines[0].indent)
	if err != nil {
		return nil, err
	}
	if next < len(lines) {
		return nil, fmt.Errorf("yaml: 第 %d 行存在无法归属的内容", lines[next].num)
	}
	return v, nil
}

// stripComment 去掉行尾注释，同时尊重引号内的 #。
func stripComment(s string) string {
	inS, inD := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0x27 && !inD:
			inS = !inS
		case c == 0x22 && !inS:
			inD = !inD
		case c == 0x23 && !inS && !inD:
			if i == 0 || s[i-1] == 0x20 || s[i-1] == 0x09 {
				return s[:i]
			}
		}
	}
	return s
}

func parseBlock(lines []yamlLine, i, indent int) (any, int, error) {
	if i >= len(lines) {
		return map[string]any{}, i, nil
	}
	if strings.HasPrefix(lines[i].text, "- ") || lines[i].text == "-" {
		return parseSeq(lines, i, indent)
	}
	return parseMap(lines, i, indent)
}

func parseMap(lines []yamlLine, i, indent int) (any, int, error) {
	out := map[string]any{}
	for i < len(lines) {
		ln := lines[i]
		if ln.indent < indent {
			break
		}
		if ln.indent > indent {
			return nil, i, fmt.Errorf("yaml: 第 %d 行缩进异常", ln.num)
		}
		if strings.HasPrefix(ln.text, "- ") || ln.text == "-" {
			break
		}
		key, rest, ok := splitKey(ln.text)
		if !ok {
			return nil, i, fmt.Errorf("yaml: 第 %d 行缺少键值分隔符", ln.num)
		}
		key = unquote(key)
		i++
		if strings.TrimSpace(rest) != "" {
			out[key] = parseScalar(strings.TrimSpace(rest))
			continue
		}
		if i < len(lines) && lines[i].indent > indent {
			child, next, err := parseBlock(lines, i, lines[i].indent)
			if err != nil {
				return nil, i, err
			}
			out[key] = child
			i = next
			continue
		}
		if i < len(lines) && lines[i].indent == indent && (strings.HasPrefix(lines[i].text, "- ") || lines[i].text == "-") {
			child, next, err := parseSeq(lines, i, indent)
			if err != nil {
				return nil, i, err
			}
			out[key] = child
			i = next
			continue
		}
		out[key] = nil
	}
	return out, i, nil
}

func parseSeq(lines []yamlLine, i, indent int) (any, int, error) {
	out := []any{}
	for i < len(lines) {
		ln := lines[i]
		if ln.indent < indent {
			break
		}
		if !(strings.HasPrefix(ln.text, "- ") || ln.text == "-") {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(ln.text, "-"))
		i++
		if item == "" {
			if i < len(lines) && lines[i].indent > indent {
				child, next, err := parseBlock(lines, i, lines[i].indent)
				if err != nil {
					return nil, i, err
				}
				out = append(out, child)
				i = next
			} else {
				out = append(out, nil)
			}
			continue
		}
		if k, rest, ok := splitKey(item); ok && !strings.HasPrefix(item, "[") {
			m := map[string]any{}
			if strings.TrimSpace(rest) != "" {
				m[unquote(k)] = parseScalar(strings.TrimSpace(rest))
			} else if i < len(lines) && lines[i].indent > indent {
				child, next, err := parseBlock(lines, i, lines[i].indent)
				if err != nil {
					return nil, i, err
				}
				m[unquote(k)] = child
				i = next
			} else {
				m[unquote(k)] = nil
			}
			if i < len(lines) && lines[i].indent > indent {
				child, next, err := parseMap(lines, i, lines[i].indent)
				if err != nil {
					return nil, i, err
				}
				if cm, ok := child.(map[string]any); ok {
					for kk, vv := range cm {
						m[kk] = vv
					}
				}
				i = next
			}
			out = append(out, m)
			continue
		}
		out = append(out, parseScalar(item))
	}
	return out, i, nil
}

func splitKey(s string) (string, string, bool) {
	inS, inD := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0x27 && !inD:
			inS = !inS
		case c == 0x22 && !inS:
			inD = !inD
		case c == 0x3A && !inS && !inD:
			if i+1 >= len(s) || s[i+1] == 0x20 || s[i+1] == 0x09 {
				return s[:i], s[i+1:], true
			}
		}
	}
	return "", "", false
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == 0x22 && s[len(s)-1] == 0x22) || (s[0] == 0x27 && s[len(s)-1] == 0x27) {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseScalar(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		body := strings.TrimSpace(s[1 : len(s)-1])
		if body == "" {
			return []any{}
		}
		parts := splitInline(body)
		arr := make([]any, 0, len(parts))
		for _, p := range parts {
			arr = append(arr, parseScalar(p))
		}
		return arr
	}
	low := strings.ToLower(s)
	switch low {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	case "null", "~", "none":
		return nil
	}
	if !strings.ContainsAny(s, ".eE") {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return unquote(s)
}

func splitInline(s string) []string {
	out := []string{}
	depth := 0
	inS, inD := false, false
	cur := strings.Builder{}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0x27 && !inD:
			inS = !inS
			cur.WriteByte(c)
		case c == 0x22 && !inS:
			inD = !inD
			cur.WriteByte(c)
		case (c == 0x5B || c == 0x7B) && !inS && !inD:
			depth++
			cur.WriteByte(c)
		case (c == 0x5D || c == 0x7D) && !inS && !inD:
			depth--
			cur.WriteByte(c)
		case c == 0x2C && depth == 0 && !inS && !inD:
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}
