package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/update"
)

// hUpdateCheck 检查在线更新（下载站 / GitHub / Docker Hub）。
func (s *Server) hUpdateCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	res := update.Check(ctx, s.cfg)
	okData(w, res)
}

// hUpdateConfig 返回更新检查配置。
func (s *Server) hUpdateConfig(w http.ResponseWriter, r *http.Request) {
	okData(w, map[string]any{
		"enabled":              s.cfg.Update.Enabled,
		"check_interval_hours": s.cfg.Update.CheckIntervalHours,
		"download_page":        s.cfg.Update.DownloadPage,
		"proxy":                s.cfg.Update.Proxy,
		"github_repo":          s.cfg.Update.GitHubRepo,
		"docker_repo":          s.cfg.Update.DockerRepo,
		"sources":              s.cfg.Update.Sources,
		"current":              s.Version(),
	})
}

// hUpdateConfigSave 保存更新检查配置（写入 settings，重启后仍生效）。
func (s *Server) hUpdateConfigSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled      *bool   `json:"enabled"`
		Proxy        *string `json:"proxy"`
		DownloadPage *string `json:"download_page"`
		GitHubRepo   *string `json:"github_repo"`
		DockerRepo   *string `json:"docker_repo"`
	}
	if err := decodeBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Enabled != nil {
		v := "0"
		if *req.Enabled {
			v = "1"
		}
		_ = s.st.SetSetting("update_enabled", v)
		s.cfg.Update.Enabled = *req.Enabled
	}
	if req.Proxy != nil {
		p := strings.TrimSpace(*req.Proxy)
		_ = s.st.SetSetting("update_proxy", p)
		s.cfg.Update.Proxy = p
	}
	if req.DownloadPage != nil {
		p := strings.TrimRight(strings.TrimSpace(*req.DownloadPage), "/")
		_ = s.st.SetSetting("update_page", p)
		s.cfg.Update.DownloadPage = p
	}
	if req.GitHubRepo != nil {
		_ = s.st.SetSetting("update_github", strings.TrimSpace(*req.GitHubRepo))
		s.cfg.Update.GitHubRepo = strings.TrimSpace(*req.GitHubRepo)
	}
	if req.DockerRepo != nil {
		_ = s.st.SetSetting("update_docker", strings.TrimSpace(*req.DockerRepo))
		s.cfg.Update.DockerRepo = strings.TrimSpace(*req.DockerRepo)
	}
	// 重新生成源列表，使改动立即生效
	s.refreshUpdateSources()
	okData(w, map[string]any{"saved": true, "sources": s.cfg.Update.Sources})
}

// refreshUpdateSources 依据当前配置重建源列表。
func (s *Server) refreshUpdateSources() {
	cfg := s.cfg
	list := []struct {
		name, typ, url, repo string
	}{}
	if cfg.Update.DownloadPage != "" {
		list = append(list, struct{ name, typ, url, repo string }{"官方下载站", "manifest", cfg.Update.DownloadPage + "/version.json", ""})
	}
	if cfg.Update.GitHubRepo != "" {
		list = append(list, struct{ name, typ, url, repo string }{"GitHub", "github", "", cfg.Update.GitHubRepo})
	}
	if cfg.Update.DockerRepo != "" {
		list = append(list, struct{ name, typ, url, repo string }{"Docker Hub", "dockerhub", "", cfg.Update.DockerRepo})
	}
	out := make([]config.UpdateSource, 0, len(list))
	for _, it := range list {
		out = append(out, config.UpdateSource{Name: it.name, Type: it.typ, URL: it.url, Repo: it.repo, Proxy: cfg.Update.Proxy})
	}
	cfg.Update.Sources = out
}
