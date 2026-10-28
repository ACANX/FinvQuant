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
	"github.com/acanx/finvquant/internal/logging"
	"github.com/acanx/finvquant/internal/meta/finv/auth"
	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	_ "github.com/acanx/finvquant/internal/meta/finv/quote/dedup/job"
	"github.com/acanx/finvquant/internal/meta/finv/sys"
	"github.com/acanx/finvquant/internal/redisclient"
	"github.com/acanx/finvquant/internal/static"
)

// 版本信息（构建时通过 -ldflags 注入）。
var (
	version = "0.1.2"
	commit  = "dev"
)

func main() {
	cfg := config.Load()

	// 日志落盘：Native 方式由 FINV_LOG_DIR 指定；容器方式留空（仅 stdout/stderr）。
	// 初始化失败不阻断启动，降级为仅标准输出。
	if logCloser, logErr := logging.Setup(cfg.LogDir, cfg.LogFile); logErr != nil {
		log.Printf("[finvquant] 日志文件初始化失败（继续输出到 stderr）: %v", logErr)
	} else if logCloser != nil {
		defer logCloser.Close()
		log.Printf("[finvquant] 日志写入 %s", cfg.LogDir)
	}

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
	//
	// 超时给到 10 分钟：迁移里可能包含存量数据回填（如 V100076 恢复 quote 隔离副本需
	// 搬运数百万行，实测 280 万行耗时约 4 分钟；慢盘/低配环境更久），
	// 30 秒会在大表上超时，导致服务启动失败。
	migrateCtx, migrateCancel := context.WithTimeout(ctx, 10*time.Minute)
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

	// 证券行情月归档导出与上传：服务始终创建（运维接口可用），
	// 自动化调度受 FINV_QUOTE_DEDUB_ENABLED 功能开关控制（M-10）。
	archiveService, err := dedup.NewService(pool, dedup.OptionsFromConfig(cfg))
	if err != nil {
		log.Fatalf("[finvquant] 初始化导出归档服务失败: %v", err)
	}
	// 低峰窗口与 CDEU 运行模式不再在启动时加载到内存：两者统一从 sys.sys_config
	// （20006 / 20005）读取，由 syscfg.Store 的 30s TTL 缓存保证在线修改即时生效。
	if cfg.QuoteDeDupEnabled {
		go dedup.NewScheduler(archiveService).Run(ctx)
		log.Printf("[finvquant] 导出归档调度已启用（上传模式 %s）", archiveService.Uploader().Mode())
	} else {
		log.Printf("[finvquant] 导出归档调度未启用（FINV_QUOTE_DEDUB_ENABLED=false），仅提供运维接口")
	}

	// 登录认证：会话存 Redis，有效期取自 sys.sys_config 20012（默认 480 分钟）。
	authService := auth.NewService(pool, rdb, syscfg.NewStore(pool))

	// 16001：API 服务
	apiServer := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           api.NewRouter(&api.Deps{Pool: pool, Redis: rdb, Version: version, Commit: commit, Server: "finvquant", Config: cfg, ExportArchive: archiveService, Auth: authService}),
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
