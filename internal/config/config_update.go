package config

import (
	"os"
	"strings"
)

// UpdateSource 是一个更新检查源。
type UpdateSource struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	Repo  string `json:"repo"`
	Proxy string `json:"proxy"`
}

// UpdateConfig 是在线更新检查配置。
type UpdateConfig struct {
	Enabled            bool           `json:"enabled"`
	CheckIntervalHours int            `json:"check_interval_hours"`
	DownloadPage       string         `json:"download_page"`
	Proxy              string         `json:"proxy"`
	GitHubRepo         string         `json:"github_repo"`
	DockerRepo         string         `json:"docker_repo"`
	Sources            []UpdateSource `json:"sources"`
}

// DefaultUpdate 返回默认更新配置。
func DefaultUpdate() UpdateConfig {
	return UpdateConfig{
		Enabled:            true,
		CheckIntervalHours: 12,
		DownloadPage:       "https://ycfmg.202693.xyz",
		GitHubRepo:         "YCyingchen/YCFMG",
		DockerRepo:         "ycyingchen/ycfmg",
	}
}

func (c *Config) parseUpdate(m map[string]any) {
	c.Update = DefaultUpdate()
	if s, ok := asMap(m["update"]); ok {
		c.Update.Enabled = boolean(s["enabled"], c.Update.Enabled)
		c.Update.CheckIntervalHours = num(s["check_interval_hours"], c.Update.CheckIntervalHours)
		c.Update.DownloadPage = str(s["download_page"], c.Update.DownloadPage)
		c.Update.Proxy = str(s["proxy"], c.Update.Proxy)
		c.Update.GitHubRepo = str(s["github_repo"], c.Update.GitHubRepo)
		c.Update.DockerRepo = str(s["docker_repo"], c.Update.DockerRepo)
		if arr, ok := slice(s["sources"]); ok {
			list := make([]UpdateSource, 0, len(arr))
			for _, it := range arr {
				im, ok := asMap(it)
				if !ok {
					continue
				}
				list = append(list, UpdateSource{
					Name:  str(im["name"], ""),
					Type:  str(im["type"], "manifest"),
					URL:   str(im["url"], ""),
					Repo:  str(im["repo"], ""),
					Proxy: str(im["proxy"], c.Update.Proxy),
				})
			}
			if len(list) > 0 {
				c.Update.Sources = list
			}
		}
	}
	c.finishUpdate()
	c.applyUpdateEnv()
}

// finishUpdate 在未显式声明 sources 时，按下载站 / GitHub / Docker Hub 自动生成。
func (c *Config) finishUpdate() {
	if len(c.Update.Sources) > 0 {
		for i := range c.Update.Sources {
			if c.Update.Sources[i].Proxy == "" {
				c.Update.Sources[i].Proxy = c.Update.Proxy
			}
		}
		return
	}
	list := []UpdateSource{}
	if p := strings.TrimRight(strings.TrimSpace(c.Update.DownloadPage), "/"); p != "" {
		list = append(list, UpdateSource{Name: "官方下载站", Type: "manifest", URL: p + "/version.json", Proxy: c.Update.Proxy})
	}
	if r := strings.TrimSpace(c.Update.GitHubRepo); r != "" {
		list = append(list, UpdateSource{Name: "GitHub", Type: "github", Repo: r, Proxy: c.Update.Proxy})
	}
	if r := strings.TrimSpace(c.Update.DockerRepo); r != "" {
		list = append(list, UpdateSource{Name: "Docker Hub", Type: "dockerhub", Repo: r, Proxy: c.Update.Proxy})
	}
	c.Update.Sources = list
}

func (c *Config) applyUpdateEnv() {
	if v := os.Getenv("YCFMG_UPDATE_PROXY"); v != "" {
		c.Update.Proxy = v
		for i := range c.Update.Sources {
			if c.Update.Sources[i].Proxy == "" {
				c.Update.Sources[i].Proxy = v
			}
		}
	}
	if v := os.Getenv("YCFMG_UPDATE_PAGE"); v != "" {
		c.Update.DownloadPage = v
	}
	if v := os.Getenv("YCFMG_UPDATE_DISABLED"); v == "1" || strings.EqualFold(v, "true") {
		c.Update.Enabled = false
	}
}

// RefreshUpdateSources 依据当前配置重建更新源列表（供运行时修改后立即生效）。
func (c *Config) RefreshUpdateSources() {
	c.Update.Sources = nil
	c.finishUpdate()
}
