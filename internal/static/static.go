// Package static 提供内嵌前端静态资源服务（SPA fallback + /API 反向代理）。
package static

import (
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/acanx/finvquant/internal/webui"
)

// Handler 返回前端静态文件处理器；未知路径回退 index.html（SPA 路由）。
// 生产模式（All-in-One 单进程双端口）下，/API/* 请求反向代理到服务端 16001。
func Handler(apiBaseURL string) http.Handler {
	sub, err := fs.Sub(webui.Dist, "dist")
	if err != nil {
		panic("内嵌前端资源缺失: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	apiProxy := newReverseProxy(apiBaseURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /API/* 转发到服务端（16001）
		if strings.HasPrefix(r.URL.Path, "/API/") {
			apiProxy.ServeHTTP(w, r)
			return
		}
		// 其余按 SPA 静态资源处理
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// 入口 HTML 禁止缓存：保证前端重建后浏览器立即加载新的 hash 资源，
		// 避免继续使用旧的 index.html（引用已不存在的旧 JS）。
		if path == "index.html" {
			noCache(w)
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// SPA 回退：直接返回 index.html（避免 FileServer 目录重定向）
			content, readErr := fs.ReadFile(sub, "index.html")
			if readErr != nil {
				http.NotFound(w, r)
				return
			}
			noCache(w)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

// noCache 设置入口 HTML 的禁止缓存响应头（前端重建后立即生效）。
func noCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func newReverseProxy(target string) *httputil.ReverseProxy {
	parsed, err := url.Parse(target)
	if err != nil {
		panic("API 地址非法: " + err.Error())
	}
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	return proxy
}
