// Package server 提供 FinvQuant 量化策略交易平台服务端入口（All-in-One）。
//
// 技术栈：Go 1.25.3 + Gin v1.12 + PostgreSQL 18（pgx/v5）+ Redis 8（go-redis/v9）。
// 单进程双端口：
//   - 16001：API 服务（/API/V1/*，Gin）
//   - 16002：前端静态资源（内嵌 Web/dist，SPA fallback）
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/api"
	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/database"
	"github.com/acanx/finvquant/internal/redisclient"
	"github.com/acanx/finvquant/internal/static"
)

// 版本信息（构建时通过 -ldflags 注入）。
var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	cfg := config.Load()
	if cfg.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// PG 18 连接池
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("[finvquant] %v", err)
	}
	defer pool.Close()

	// 应用数据库迁移（启动时幂等；quote.finv_quote_secu_kline_min 等域表建表）
	migrateCtx, migrateCancel := context.WithTimeout(ctx, 30*time.Second)
	if err := database.Migrate(migrateCtx, pool, cfg.MigrationsDir()); err != nil {
		migrateCancel()
		log.Fatalf("[finvquant] 数据库迁移失败: %v", err)
	}
	migrateCancel()

	// Redis 8 客户端
	rdb, err := redisclient.NewClient(ctx, cfg)
	if err != nil {
		log.Fatalf("[finvquant] %v", err)
	}
	defer rdb.Close()

	// 16001：API 服务
	apiServer := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           api.NewRouter(&api.Deps{Pool: pool, Redis: rdb, Version: version, Commit: commit, Server: "finvquant"}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 16002：前端静态资源（内嵌）+ /API 反向代理到 16001
	apiBase := "http://" + cfg.ListenAddr()
	webServer := &http.Server{
		Addr:              cfg.WebListenAddr(),
		Handler:           static.Handler(apiBase),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[finvquant] API 服务启动，监听 %s（Go %s）", cfg.ListenAddr(), runtime.Version())
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[finvquant] API 服务异常退出: %v", err)
		}
	}()
	go func() {
		log.Printf("[finvquant] 前端服务启动，监听 %s", cfg.WebListenAddr())
		if err := webServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[finvquant] 前端服务异常退出: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[finvquant] 收到退出信号，优雅关闭...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = apiServer.Shutdown(shutdownCtx)
	_ = webServer.Shutdown(shutdownCtx)
}
