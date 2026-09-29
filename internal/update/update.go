// Package update 提供在线更新检查：支持下载站清单、GitHub Release、Docker Hub 三类源，
// 每个源可单独配置 HTTP 代理（GitHub 与 Docker Hub 在境内通常需要代理）。
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/config"
)

// SourceResult 是单个源的检查结果。
type SourceResult struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
	URL     string `json:"url"`
	OK      bool   `json:"ok"`
	Proxy   string `json:"proxy,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Result 是整体检查结果。
type Result struct {
	Current     string         `json:"current"`
	Latest      string         `json:"latest"`
	HasUpdate   bool           `json:"has_update"`
	CheckedAt   int64          `json:"checked_at"`
	PublishedAt string         `json:"published_at"`
	Notes       []string       `json:"notes"`
	Download    string         `json:"download"`
	Sources     []SourceResult `json:"sources"`
	Enabled     bool           `json:"enabled"`
}

var digitRe = regexp.MustCompile("[0-9]+")

// ParseVersion 把版本号解析为数字片段，用于比较。
// 支持 fmg2609.001、v1.2.3、1.2.3-beta 等写法。
func ParseVersion(v string) []int {
	parts := digitRe.FindAllString(v, -1)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out
}

// Compare 比较版本：a > b 返回 1，a < b 返回 -1，相等返回 0。
func Compare(a, b string) int {
	x, y := ParseVersion(a), ParseVersion(b)
	n := len(x)
	if len(y) > n {
		n = len(y)
	}
	for i := 0; i < n; i++ {
		var xi, yi int
		if i < len(x) {
			xi = x[i]
		}
		if i < len(y) {
			yi = y[i]
		}
		if xi > yi {
			return 1
		}
		if xi < yi {
			return -1
		}
	}
	return 0
}

func clientFor(proxy string, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}
	if p := strings.TrimSpace(proxy); p != "" {
		if u, err := url.Parse(p); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

func getJSON(ctx context.Context, proxy, rawURL string, out any) error {
	cli := clientFor(proxy, 45*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "YCFMG-Updater")
	req.Header.Set("Accept", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// manifestPayload 是下载站 version.json 的结构。
type manifestPayload struct {
	Version     string   `json:"version"`
	PublishedAt string   `json:"published_at"`
	Notes       []string `json:"notes"`
	Download    string   `json:"download"`
}

func checkManifest(ctx context.Context, src config.UpdateSource, current string) (SourceResult, manifestPayload) {
	res := SourceResult{Name: src.Name, Type: src.Type, URL: src.URL, Proxy: src.Proxy}
	var m manifestPayload
	if err := getJSON(ctx, src.Proxy, src.URL, &m); err != nil {
		res.Error = err.Error()
		return res, m
	}
	res.Version = m.Version
	res.OK = m.Version != ""
	return res, m
}

// githubRelease 是 GitHub Release API 的片段。
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
}

func checkGitHub(ctx context.Context, src config.UpdateSource) SourceResult {
	repo := strings.TrimSpace(src.Repo)
	res := SourceResult{Name: src.Name, Type: src.Type, Proxy: src.Proxy}
	if repo == "" {
		res.Error = "未配置仓库"
		return res
	}
	res.URL = "https://github.com/" + repo
	// 优先使用 Atom feed：无需认证、不受 API 限流影响
	if tag, _, err := atomLatest(ctx, src.Proxy, "https://github.com/"+repo+"/releases.atom"); err == nil && tag != "" {
		res.Version = strings.TrimPrefix(tag, "v")
		res.OK = res.Version != ""
		return res
	}
	api := "https://api.github.com/repos/" + repo + "/releases/latest"
	var rel githubRelease
	if err := getJSON(ctx, src.Proxy, api, &rel); err != nil {
		res.Error = err.Error()
		return res
	}
	res.Version = strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if res.Version == "" {
		res.Version = strings.TrimPrefix(strings.TrimSpace(rel.Name), "v")
	}
	res.OK = res.Version != ""
	return res
}

var atomEntryRe = regexp.MustCompile("(?s)<entry>.*?<id>[^<]*/([^/<]+)</id>.*?<updated>([^<]+)</updated>")

func atomLatest(ctx context.Context, proxy, rawURL string) (string, string, error) {
	body, err := getText(ctx, proxy, rawURL)
	if err != nil {
		return "", "", err
	}
	m := atomEntryRe.FindStringSubmatch(body)
	if len(m) < 3 {
		return "", "", fmt.Errorf("未解析到版本条目")
	}
	return strings.TrimSpace(m[1]), strings.TrimSpace(m[2]), nil
}

func getText(ctx context.Context, proxy, rawURL string) (string, error) {
	cli := clientFor(proxy, 45*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "YCFMG-Updater")
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b), err
}

// dockerTags 是 Docker Hub tags API 的片段。
type dockerTags struct {
	Results []struct {
		Name        string `json:"name"`
		LastUpdated string `json:"last_updated"`
	} `json:"results"`
}

func checkDockerHub(ctx context.Context, src config.UpdateSource) SourceResult {
	repo := strings.TrimSpace(src.Repo)
	res := SourceResult{Name: src.Name, Type: src.Type, Proxy: src.Proxy}
	if repo == "" {
		res.Error = "未配置仓库"
		return res
	}
	api := "https://hub.docker.com/v2/repositories/" + repo + "/tags?page_size=25&ordering=last_updated"
	res.URL = "https://hub.docker.com/r/" + repo + "/tags"
	var tags dockerTags
	if err := getJSON(ctx, src.Proxy, api, &tags); err != nil {
		res.Error = err.Error()
		return res
	}
	best := ""
	for _, t := range tags.Results {
		name := strings.TrimSpace(t.Name)
		if name == "" || name == "latest" {
			continue
		}
		if best == "" || Compare(name, best) > 0 {
			best = name
		}
	}
	res.Version = best
	res.OK = best != ""
	if best == "" {
		res.Error = "未找到版本标签"
	}
	return res
}

// Check 依次检查全部启用的更新源，汇总出最新版本。
func Check(ctx context.Context, cfg *config.Config) Result {
	out := Result{
		Current:   versionCurrent(),
		CheckedAt: time.Now().Unix(),
		Sources:   []SourceResult{},
		Enabled:   cfg.Update.Enabled,
	}
	if !cfg.Update.Enabled {
		return out
	}
	best := ""
	for _, src := range cfg.Update.Sources {
		var res SourceResult
		switch strings.ToLower(src.Type) {
		case "github":
			res = checkGitHub(ctx, src)
		case "dockerhub", "docker":
			res = checkDockerHub(ctx, src)
		default:
			var m manifestPayload
			res, m = checkManifest(ctx, src, out.Current)
			if res.OK {
				out.Notes = m.Notes
				out.PublishedAt = m.PublishedAt
				if m.Download != "" {
					out.Download = m.Download
				}
			}
		}
		if !res.OK {
			time.Sleep(400 * time.Millisecond)
			switch strings.ToLower(src.Type) {
			case "github":
				if r2 := checkGitHub(ctx, src); r2.OK {
					res = r2
				}
			case "dockerhub", "docker":
				if r2 := checkDockerHub(ctx, src); r2.OK {
					res = r2
				}
			}
		}
		out.Sources = append(out.Sources, res)
		if res.OK && (best == "" || Compare(res.Version, best) > 0) {
			best = res.Version
		}
	}
	out.Latest = best
	if best != "" && Compare(best, out.Current) > 0 {
		out.HasUpdate = true
	}
	if out.Download == "" {
		out.Download = cfg.Update.DownloadPage
	}
	return out
}

// versionCurrent 由外部注入，避免循环依赖。
var currentVersion = "dev"

// SetCurrent 设置当前版本（启动时调用）。
func SetCurrent(v string) { currentVersion = v }

func versionCurrent() string { return currentVersion }
