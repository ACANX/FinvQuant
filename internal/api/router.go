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
	"github.com/acanx/finvquant/internal/meta/finv/auth"
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
	// Auth 登录认证服务；为 nil 时不启用登录校验（仅供单元测试构造）。
	// 生产环境由 cmd/server 注入，任何业务请求都必须携带有效令牌。
	Auth *auth.Service
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

	// 公开端点：健康检查、版本、登录（不校验令牌）。
	// 健康检查供容器探针使用，不能被登录拦截。
	publicGroup := router.Group("/API/V1")
	{
		publicGroup.GET("/Health/Live", health.Live)
		publicGroup.GET("/Health/Ready", health.Ready)
		publicGroup.GET("/Version", version.Info)
		if deps.Auth != nil {
			publicGroup.POST("/Auth/Login", handler.NewAuth(deps.Auth).Login)
		}
	}

	// 业务端点：统一要求登录（Authorization: Bearer <token>）。
	apiGroup := router.Group("/API/V1")
	if deps.Auth != nil {
		apiGroup.Use(RequireAuth(deps.Auth))
	}
	{
		if deps.Auth != nil {
			authHandler := handler.NewAuth(deps.Auth)
			apiGroup.POST("/Auth/Logout", authHandler.Logout)
			apiGroup.POST("/Auth/ChangePassword", authHandler.ChangePassword)
			apiGroup.GET("/Auth/Profile", authHandler.Profile)
		}
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
			group.GET("/Settings", archive.Settings)
			group.PUT("/Settings/Window", archive.SettingsWindow)
			group.GET("/Job/Runtime", archive.JobRuntime)
			group.GET("/CDEU/Overview", archive.CDEUOverview)
			group.POST("/CDEU/Workflow/Run", archive.CDEUWorkflowRun)
			group.POST("/CDEU/Workflow/Terminate", archive.CDEUWorkflowTerminate)
			// 兼容旧路径；新前端统一使用 /CDEU/Workflow/Run。
			group.POST("/CDEU/Process/Run", archive.CDEUWorkflowRun)
			group.GET("/Job/List", archive.JobList)
			group.GET("/File/List", archive.FileList)
			group.POST("/Job/Retry", archive.JobRetry)
			group.POST("/File/Retry", archive.FileRetry)
			group.POST("/Job/Run", archive.JobRun)
			group.POST("/Job/Dynamic/Run", archive.DynamicJobRun)

			// 归档运维台：查询（10 张表 + 成熟度）
			group.GET("/Usc/Page", archive.UscPage)
			group.GET("/Priority/List", archive.PriorityList)
			group.GET("/Calendar/Page", archive.CalendarPage)
			group.GET("/Maturity/Page", archive.MaturityPage)
			group.GET("/Stats/Page", archive.StatsPage)
			group.GET("/Budget/Page", archive.BudgetPage)
			group.GET("/Watermark/List", archive.WatermarkList)
			group.GET("/RunLog/Page", archive.RunLogPage)
			group.GET("/RunLog/Detail", archive.RunLogDetail)
			group.GET("/UploadLog/Page", archive.UploadLogPage)
			group.GET("/Patch/Page", archive.PatchPage)

			// 归档运维台：管理员编辑（5 张可编辑表 + 补丁重试 / 任务暂停取消 / 成熟度重算）
			group.PUT("/Usc", archive.UscUpsert)
			group.PUT("/Priority", archive.PriorityUpsert)
			group.DELETE("/Priority", archive.PriorityDelete)
			group.PUT("/Calendar", archive.CalendarUpsert)
			group.DELETE("/Calendar", archive.CalendarDelete)
			group.PUT("/Budget/Override", archive.BudgetOverride)
			group.PUT("/Watermark", archive.WatermarkReset)
			group.POST("/Patch/Retry", archive.PatchRetry)
			group.POST("/Job/Pause", archive.JobPause)
			group.POST("/Job/Cancel", archive.JobCancel)
			group.POST("/Maturity/Refresh", archive.MaturityRefresh)
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
