// Package api 组装 Gin 路由。
package api

import (
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/acanx/finvquant/internal/api/handler"
	"github.com/acanx/finvquant/internal/backtest"
	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/kline"
	"github.com/acanx/finvquant/internal/meta"
	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	"github.com/acanx/finvquant/internal/quote"
)

// Deps 路由依赖。
type Deps struct {
	Pool          *pgxpool.Pool
	Redis         *redis.Client
	Version       string
	Commit        string
	Server        string
	Config        *config.Config
	ExportArchive *dedup.Service
}

// NewRouter 构建 Gin 路由（默认监听端口 16001）。
func NewRouter(deps *Deps) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	health := &handler.Health{
		Pool:   deps.Pool,
		Redis:  deps.Redis,
		Server: deps.Server,
	}
	version := &handler.Version{
		Version:   deps.Version,
		GoVersion: runtime.Version(),
		Commit:    deps.Commit,
	}
	quoteImport := handler.NewQuoteImport(quote.NewService(deps.Pool))
	quoteQuery := handler.NewQuoteQuery(quote.NewService(deps.Pool))
	metaHandler := handler.NewMeta(meta.NewService(deps.Pool))
	backtestHandler := handler.NewBacktest(backtest.NewService(deps.Pool))

	// 分钟行情 K 线 API（融合 FinvQuoteDataDeDup；DC-01/DC-05/DC-06）
	cfg := deps.Config
	if cfg == nil {
		cfg = config.Load()
	}
	klineHandler := handler.NewKLine(
		kline.NewService(deps.Pool, kline.Options{
			MaxBatchRows: cfg.KLineMaxBatchRows,
			MaxListSize:  cfg.KLineMaxListSize,
			TSUnit:       cfg.KLineTSUnit,
		}),
		handler.NewErrorResponder(cfg),
	)
	klineOps := handler.NewKLineOps(deps.Pool, deps.Server, deps.Version, kline.APIPrefix)

	apiGroup := router.Group("/API/V1")
	{
		apiGroup.GET("/Health/Live", health.Live)
		apiGroup.GET("/Health/Ready", health.Ready)
		apiGroup.GET("/Version", version.Info)
		// 分钟行情 K 线：新前缀 + 旧前缀过渡别名（同一组 handler，P7 后可移除旧前缀）
		for _, prefix := range []string{
			strings.TrimPrefix(kline.APIPrefix, "/API/V1/"),
			strings.TrimPrefix(kline.LegacyAPIPrefix, "/API/V1/"),
		} {
			group := apiGroup.Group(prefix)
			group.POST("/Add", klineHandler.Add)
			group.POST("/Batch", klineHandler.BatchInsert)
			group.POST("/Upsert", klineHandler.Upsert)
			group.GET("/Page", klineHandler.Page)
			group.GET("/PageUni", klineHandler.PageUni)
			group.POST("/Query", klineHandler.Query)
			group.PATCH("/Batch", klineHandler.BatchPatch)
			group.DELETE("/Batch", klineHandler.BatchDelete)
			group.GET("/Count", klineHandler.Count)
			group.GET("/Stats", klineHandler.Stats)
			group.GET("/Bounds", klineHandler.Bounds)
		}

		// 历史行情：导入 / 查询（统一 /Meta/Finv/Quant/Quote/ 前缀，与前端路由对应）
		apiGroup.POST("/Meta/Finv/Quant/Quote/Import/Upload", quoteImport.Upload)
		apiGroup.GET("/Meta/Finv/Quant/Quote/History/QuoteQuery", quoteQuery.Query)

		// 证券行情月归档导出与上传（运维：状态 / 任务 / 文件 / 重试 / 手动触发）
		if deps.ExportArchive != nil {
			archive := handler.NewExportArchive(deps.ExportArchive)
			group := apiGroup.Group("/Meta/Finv/Quote/DeDup")
			group.GET("/Status", archive.Status)
			group.GET("/Job/List", archive.JobList)
			group.GET("/File/List", archive.FileList)
			group.POST("/Job/Retry", archive.JobRetry)
			group.POST("/File/Retry", archive.FileRetry)
			group.POST("/Job/Run", archive.JobRun)
		}

		// 元数据管理：交易所 / 市场 / 证券字典维护
		apiGroup.GET("/Meta/Finv/Quant/Metadata/Exchange/List", metaHandler.ListExchanges)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Exchange/Save", metaHandler.SaveExchange)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Exchange/Toggle", metaHandler.ToggleExchange)
		apiGroup.GET("/Meta/Finv/Quant/Metadata/Market/List", metaHandler.ListMarkets)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Market/Save", metaHandler.SaveMarket)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Market/Toggle", metaHandler.ToggleMarket)
		apiGroup.GET("/Meta/Finv/Quant/Metadata/Security/List", metaHandler.ListSecurities)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Security/Save", metaHandler.SaveSecurity)
		apiGroup.POST("/Meta/Finv/Quant/Metadata/Security/Toggle", metaHandler.ToggleSecurity)
		apiGroup.GET("/Meta/Finv/Quant/Metadata/Security/Options", metaHandler.SecurityOptions)
		apiGroup.GET("/Meta/Finv/Quant/Metadata/Security/Lookup", metaHandler.LookupSecurity)

		// 通用量化回测：策略 / 账户 / 任务 / 报告 / 链路追踪 / 环境 / 模板
		// （路径前缀规范：/API/V1/Meta/Finv/Quant/Backtest/**）
		bt := apiGroup.Group("/Meta/Finv/Quant/Backtest")
		{
			bt.GET("/Strategy/List", backtestHandler.ListStrategies)
			bt.GET("/Strategy/Get", backtestHandler.GetStrategy)
			bt.POST("/Strategy/Save", backtestHandler.SaveStrategy)
			bt.POST("/Strategy/Toggle", backtestHandler.ToggleStrategy)
			bt.POST("/Strategy/Delete", backtestHandler.DeleteStrategy)
			bt.GET("/Account/List", backtestHandler.ListAccounts)
			bt.GET("/Account/Get", backtestHandler.GetAccount)
			bt.POST("/Account/Save", backtestHandler.SaveAccount)
			bt.POST("/Account/Toggle", backtestHandler.ToggleAccount)
			bt.POST("/Account/Delete", backtestHandler.DeleteAccount)
			bt.POST("/Run/Create", backtestHandler.CreateRun)
			bt.GET("/Run/List", backtestHandler.ListRuns)
			bt.GET("/Run/Get", backtestHandler.GetRun)
			bt.POST("/Run/Cancel", backtestHandler.CancelRun)
			bt.GET("/Run/Report", backtestHandler.GetReport)
			bt.GET("/Run/Equity", backtestHandler.ListEquity)
			bt.GET("/Run/Trades", backtestHandler.ListTrades)
			bt.GET("/Run/Cashflows", backtestHandler.ListCashflows)
			bt.GET("/Run/PositionLogs", backtestHandler.ListPositionLogs)
			bt.GET("/Run/EventTraces", backtestHandler.ListEventTraces)
			bt.POST("/Run/Delete", backtestHandler.DeleteRun)
			bt.GET("/Run/DeleteTask/List", backtestHandler.ListRunDelTasks)
			bt.GET("/Run/DeleteTask/Logs", backtestHandler.ListRunDelLogs)
			bt.POST("/Run/DeleteTask/Retry", backtestHandler.RetryRunDelete)
			bt.GET("/Run/DeleteTask/Archives", backtestHandler.ListRunArchives)
			bt.GET("/Environment/List", backtestHandler.ListEnvironments)
			bt.GET("/Environment/Get", backtestHandler.GetEnvironment)
			bt.POST("/Environment/Save", backtestHandler.SaveEnvironment)
			bt.POST("/Environment/Toggle", backtestHandler.ToggleEnvironment)
			bt.POST("/Environment/Delete", backtestHandler.DeleteEnvironment)
			bt.GET("/Template/List", backtestHandler.ListTemplates)
			bt.GET("/Template/Get", backtestHandler.GetTemplate)
			bt.POST("/Template/Save", backtestHandler.SaveTemplate)
			bt.POST("/Template/Delete", backtestHandler.DeleteTemplate)
		}
	}

	// 运维端点：/Health（DC-02，主路径）+ /healthz（过渡别名）+ / 服务标识
	router.GET("/Health", klineOps.Health)
	router.GET("/healthz", klineOps.Health)
	router.GET("/", klineOps.Root)

	return router
}
