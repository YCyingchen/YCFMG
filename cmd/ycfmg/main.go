// Command ycfmg 是 YCFMG（文件管理 + 智能图库）的服务端入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ycyingchen/ycfmg/internal/auth"
	"github.com/ycyingchen/ycfmg/internal/config"
	"github.com/ycyingchen/ycfmg/internal/fsapi"
	"github.com/ycyingchen/ycfmg/internal/indexer"
	"github.com/ycyingchen/ycfmg/internal/logx"
	"github.com/ycyingchen/ycfmg/internal/search"
	"github.com/ycyingchen/ycfmg/internal/server"
	"github.com/ycyingchen/ycfmg/internal/share"
	"github.com/ycyingchen/ycfmg/internal/store"
	"github.com/ycyingchen/ycfmg/internal/thumbs"
	"github.com/ycyingchen/ycfmg/internal/update"
	"github.com/ycyingchen/ycfmg/internal/version"
	webassets "github.com/ycyingchen/ycfmg/web"
)

func main() {
	var (
		configPath    string
		portFlag      int
		sharePortFlag int
		socketFlag    string
		dataFlag      string
		showVer       bool
		scanNow       bool
	)
	flag.StringVar(&configPath, "config", "config.yaml", "配置文件路径")
	flag.IntVar(&portFlag, "port", 0, "监听端口（覆盖配置文件）")
	flag.IntVar(&sharePortFlag, "share-port", 0, "分享专用端口（0 表示与主端口共用）")
	flag.StringVar(&socketFlag, "socket", "", "统一网关 Unix Socket 路径（飞牛 fnOS 用）")
	flag.StringVar(&dataFlag, "data", "", "数据目录（覆盖配置文件）")
	flag.BoolVar(&showVer, "version", false, "打印版本号后退出")
	flag.BoolVar(&scanNow, "scan", false, "启动后立即执行一次全量索引")
	flag.Parse()

	if showVer {
		fmt.Printf("YCFMG %s (%s, %s, %s)\n", version.Version, version.Commit, runtime.GOOS+"/"+runtime.GOARCH, version.BuildTime)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败:", err)
		os.Exit(1)
	}
	if portFlag > 0 {
		cfg.Server.Port = portFlag
	}
	if sharePortFlag > 0 {
		cfg.Server.SharePort = sharePortFlag
	}
	if socketFlag != "" {
		cfg.Server.Socket = socketFlag
	}
	if dataFlag != "" {
		cfg.DataDir = dataFlag
	}
	update.SetCurrent(version.Version)
	logx.SetLevel(logx.ParseLevel(cfg.LogLevel))
	logx.Infof("YCFMG %s 启动中 ...", version.Version)

	st, err := store.Open(cfg.DataFile("ycfmg.db"))
	if err != nil {
		logx.Errorf("数据库初始化失败: %v", err)
		os.Exit(1)
	}
	defer st.Close()

	applyRuntimeSettings(cfg, st)

	th := thumbs.New(cfg.ThumbDir(), cfg.Index.ThumbSizes, 82)
	au := auth.New(cfg, st)
	if pwd, err := au.EnsureAdmin(); err != nil {
		logx.Errorf("管理员初始化失败: %v", err)
	} else if pwd != "" {
		logx.Infof("首次启动，请使用 admin / %s 登录并尽快修改密码", pwd)
	}
	st.CleanSessions()

	ix := indexer.New(cfg, st, th)
	eng := search.New(cfg, st)
	shm := share.New(cfg, st, th)
	fsys := fsapi.New(cfg, st)

	srv := server.New(server.Options{
		Config: cfg, Store: st, Thumbs: th, Index: ix, Engine: eng,
		Share: shm, Auth: au, FS: fsys, WebFS: webassets.Assets(),
	})

	ix.Loop()
	if cfg.Index.Enabled || scanNow {
		go func() {
			if ix.Progress().Running {
				return
			}
			ix.ScanAll(false)
		}()
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 20 * time.Second,
	}

	for _, l := range cfg.Libraries {
		logx.Infof("文件库: %s -> %s (只读=%v)", l.Name, l.Path, l.ReadOnly)
	}
	if len(cfg.Server.PublicURLs) > 0 {
		logx.Infof("已绑定域名: %v", cfg.Server.PublicURLs)
	}
	logx.Infof("访问地址: http://%s:%d%s", displayHost(cfg.Server.Host), cfg.Server.Port, cfg.Server.BasePath)

	// 飞牛统一网关：监听 Unix Socket，由网关按 /app/<name> 前缀同源转发，
	// 桌面点图标因此无需任何端口，也不经过浏览器跳转。
	if cfg.Server.Socket != "" {
		_ = os.Remove(cfg.Server.Socket)
		ln, lerr := net.Listen("unix", cfg.Server.Socket)
		if lerr != nil {
			logx.Errorf("统一网关 Socket 监听失败: %v", lerr)
			os.Exit(1)
		}
		_ = os.Chmod(cfg.Server.Socket, 0o666)
		// 网关转发会保留 /app/ycfmg 前缀，这里剥掉后再进主路由；
		// TCP 端口不剥前缀，分享页链接保持 http://host:port/s/xxx 的简洁形式。
		const gatewayPrefix = "/app/ycfmg"
		inner := srv.Handler()
		sockSrv := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 告诉应用它被挂在前缀下，前端据此拼接口地址
				if strings.HasPrefix(r.URL.Path, gatewayPrefix) {
					r.Header.Set("X-YCFMG-Gateway-Prefix", gatewayPrefix)
				}
				http.StripPrefix(gatewayPrefix, inner).ServeHTTP(w, r)
			}),
			ReadHeaderTimeout: 20 * time.Second,
		}
		logx.Infof("统一网关入口: %s（对外路径 %s）", cfg.Server.Socket, gatewayPrefix)
		go func() {
			if serr := sockSrv.Serve(ln); serr != nil && serr != http.ErrServerClosed {
				logx.Errorf("网关服务异常退出: %v", serr)
			}
		}()
		defer func() { _ = sockSrv.Close() }()
	}
	// 分享专用端口：只放行分享页面与前端静态资源，其余一律 404，
	// 这样对外只需暴露分享端口，文件管理器本身不必暴露。
	if cfg.Server.SharePort > 0 && cfg.Server.SharePort != cfg.Server.Port {
		shareAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.SharePort)
		inner := srv.Handler()
		shareSrv := &http.Server{
			Addr: shareAddr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := r.URL.Path
				if strings.HasPrefix(p, "/s/") || strings.HasPrefix(p, "/css/") ||
					strings.HasPrefix(p, "/js/") || strings.HasPrefix(p, "/img/") ||
					p == "/favicon.ico" || p == "/favicon.svg" || p == "/api/public" {
					inner.ServeHTTP(w, r)
					return
				}
				if p == "/" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					fmt.Fprint(w, "<!doctype html><meta charset=\"utf-8\"><title>YCFMG 分享</title>"+
						"<body style=\"font-family:system-ui;padding:40px;color:#333\"><h2>YCFMG 分享服务</h2>"+
						"<p>请使用完整的分享链接访问，例如 <code>/s/xxxxxxxx</code></p></body>")
					return
				}
				http.NotFound(w, r)
			}),
			ReadHeaderTimeout: 20 * time.Second,
		}
		go func() {
			logx.Infof("分享专用端口: http://%s", shareAddr)
			if err := shareSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logx.Errorf("分享端口异常退出: %v", err)
			}
		}()
		defer func() { _ = shareSrv.Close() }()
	}

	go func() {
		var err error
		if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
			logx.Infof("启用 HTTPS")
			err = httpSrv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey)
		} else {
			err = httpSrv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			if cfg.Server.Socket != "" {
				// 有统一网关兜底时，端口被占用不致命
				logx.Errorf("HTTP 端口不可用（统一网关仍可访问）: %v", err)
				return
			}
			logx.Errorf("HTTP 服务异常退出: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logx.Infof("正在关闭 ...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	ix.Stop()
	logx.Infof("已退出")
}

func displayHost(h string) string {
	if h == "0.0.0.0" || h == "" || h == "::" {
		return "127.0.0.1"
	}
	return h
}

// applyRuntimeSettings 读取数据库中保存的覆盖项（如多域名），使其无需重启即可生效。
func applyRuntimeSettings(cfg *config.Config, st *store.Store) {
	if v := st.GetSetting("public_urls", ""); v != "" {
		parts := []string{}
		for _, p := range splitList(v) {
			if p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			cfg.Server.PublicURLs = parts
		}
	}
	if v := st.GetSetting("brand_name", ""); v != "" {
		cfg.Share.BrandName = v
	}
	if v := st.GetSetting("brand_subtitle", ""); v != "" {
		cfg.Share.BrandSubtitle = v
	}
	if v := st.GetSetting("auto_tags", ""); v == "0" {
		cfg.Index.AutoTags = false
	}
	if v := st.GetSetting("update_proxy", ""); v != "" {
		cfg.Update.Proxy = v
	}
	if v := st.GetSetting("update_page", ""); v != "" {
		cfg.Update.DownloadPage = v
	}
	if v := st.GetSetting("update_github", ""); v != "" {
		cfg.Update.GitHubRepo = v
	}
	if v := st.GetSetting("update_docker", ""); v != "" {
		cfg.Update.DockerRepo = v
	}
	if st.GetSetting("update_enabled", "") == "0" {
		cfg.Update.Enabled = false
	}

	if v := st.GetSetting("hash_tolerance", ""); v != "" {
		n := 0
		for _, c := range v {
			if c < 0x30 || c > 0x39 {
				n = 0
				break
			}
			n = n*10 + int(c-0x30)
		}
		if n > 0 {
			cfg.Index.HashTolerance = n
		}
	}
}

func splitList(s string) []string {
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == 0x2C || c == 0x3B || c == 0x0A {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	out = append(out, cur)
	for i := range out {
		out[i] = trimSpace(out[i])
	}
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == 0x20 || s[start] == 0x09) {
		start++
	}
	for end > start && (s[end-1] == 0x20 || s[end-1] == 0x09) {
		end--
	}
	return s[start:end]
}
