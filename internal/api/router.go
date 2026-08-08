// Package api 组装 Gin 路由。
package api

import (
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/acanx/finvquant/internal/api/handler"
	"github.com/acanx/finvquant/internal/backtest"
	"github.com/acanx/finvquant/internal/meta"
	"github.com/acanx/finvquant/internal/quote"
)

// Deps 路由依赖。
type Deps struct {
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Version string
	Commit  string
	Server  string
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

	apiGroup := router.Group("/API/V1")
	{
		apiGroup.GET("/Health/Live", health.Live)
		apiGroup.GET("/Health/Ready", health.Ready)
		apiGroup.GET("/Version", version.Info)
		// 历史行情：导入 / 查询（统一 /Meta/Finv/Quant/Quote/ 前缀，与前端路由对应）
		apiGroup.POST("/Meta/Finv/Quant/Quote/Import/Upload", quoteImport.Upload)
		apiGroup.GET("/Meta/Finv/Quant/Quote/History/QuoteQuery", quoteQuery.Query)

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

	return router
}
