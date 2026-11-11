// Package api 组装 Gin 路由。
package api

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/acanx/finvquant/internal/api/handler"
	"github.com/acanx/finvquant/internal/api/openapi"
	"github.com/acanx/finvquant/internal/config"
	"github.com/acanx/finvquant/internal/meta"
	"github.com/acanx/finvquant/internal/meta/finv/auth"
	"github.com/acanx/finvquant/internal/meta/finv/brs"
	openapiSvc "github.com/acanx/finvquant/internal/meta/finv/openapi"
	"github.com/acanx/finvquant/internal/meta/finv/quant/backtest"
	"github.com/acanx/finvquant/internal/meta/finv/quote"
	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu/kline"
	"github.com/acanx/finvquant/internal/meta/finv/quote/indicator"
	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	"github.com/acanx/finvquant/internal/meta/finv/quote/uscbackfill"
	"github.com/acanx/finvquant/internal/meta/finv/quote/uscrule"
)

// Deps 路由依赖。
type Deps struct {
	Pool          *pgxpool.Pool
	Redis         *redis.Client
	Version       string
	Commit        string
	Server        string
	Config        *config.Config
	ExportArchive *cdeu.Service
	// Auth 登录认证服务；为 nil 时不启用登录校验（仅供单元测试构造）。
	// 生产环境由 cmd/server 注入，任何业务请求都必须携带有效令牌。
	Auth *auth.Service
	// BRSRunner 小表全量备份运行器（手动触发 + 运行状态）；为 nil 时不注册相关路由（仅供单元测试构造）。
	BRSRunner *brs.Runner
	// BRSSliceRunner 大表分片备份运行器（手动触发 + 运行状态）；为 nil 时不注册相关路由。
	BRSSliceRunner *brs.SliceRunner
	// USCSync USC 映射规则服务（需求 11072）；为 nil 时不注册相关路由（仅供单元测试构造）。
	USCSync *uscrule.Service
	// USCRunner USC 映射同步运行器（手动触发 + 每日 cron 共用）；为 nil 时不注册相关路由。
	USCRunner *uscrule.Runner
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
			// 认证接口统一挂在 /Meta/Finv/Auth 下，与 /Meta/Finv/Quote、
			// /Meta/Finv/Quant 等业务域保持同一层级结构
			publicGroup.POST("/Meta/Finv/Auth/Login", handler.NewAuth(deps.Auth).Login)
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
			authGroup := apiGroup.Group("/Meta/Finv/Auth")
			authGroup.POST("/Logout", authHandler.Logout)
			authGroup.POST("/ChangePassword", authHandler.ChangePassword)
			authGroup.GET("/Profile", authHandler.Profile)
		}
		// 分钟行情 K 线：统一 /Meta/Finv/Quote/Cdeu/Min 前缀（需求 10782）。
		// 旧 /DeDup/Secu/KLine/Min 与 legacy /Secu/KLine/Min 均不再注册。
		group := apiGroup.Group(strings.TrimPrefix(kline.APIPrefix, "/API/V1/"))
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

		// 外部 OpenAPI：独立前缀 /OpenAPI/V1，业务逻辑复用内部 handler。
		// 需求 11168：鉴权中间件 openapi.Middleware，支持静态 API Key 与 AK/SK 签名，
		// 由 sys_config 19001 控制（0 关闭 / 1 启用 / 2 宽松），失败返回 190xx 错误码；
		// 兼容期环境变量 FINV_OPENAPI_KEY 与 sys_config 19006 作为兜底静态密钥。
		openGroup := router.Group("/OpenAPI/V1")
		// Redis 故障策略（需求 11188）：FINV_OPENAPI_REDIS_FAIL_MODE=close（默认）时拒绝请求，
		// 避免 Redis 不可用期间防重放/限流静默失效；=open 时退回旧行为（放行 + 日志告警）。
		redisFailClose := !strings.EqualFold(strings.TrimSpace(cfg.OpenAPIRedisFailMode), "open")
		openGroup.Use(openapi.NewMiddleware(openapi.Options{
			Pool:           deps.Pool,
			Redis:          deps.Redis,
			FallbackKey:    cfg.OpenAPIKey,
			RedisFailClose: redisFailClose,
		}).Handler())
		openGroup.Use(WrapOpenAPIResponse())
		openGroup.POST("/Meta/Finv/Quote/Cdeu/Min/Upsert", klineHandler.Upsert)
		openGroup.GET("/Meta/Finv/Quote/Cdeu/Min/PageUni", klineHandler.PageUni)
		// STS 换证（需求 11191）：用长期 AK/SK 换取 2 小时有效的 AK/SK + SecurityToken。
		// 路径由用户定稿：Meta/Auth/Credential/STS/Issue（缩写段全大写，其余 PascalCase）。
		stsHandler := openapi.NewStsIssueHandler(
			openapi.NewSettingsStore(deps.Pool), openapi.NewStsStore(deps.Pool))
		openGroup.POST("/Meta/Auth/Credential/STS/Issue", stsHandler.Issue)

		// DELETE 显式拒绝（需求 11180）：对外 OpenAPI 不提供删除能力。
		// 注册成真实路由（而不是留给 404）有两个目的：
		//   ① 删除类请求也会先过鉴权——无凭证照旧 19001，不会因为「方法不支持」而绕过鉴权被探测；
		//   ② 调用方拿到明确的 405 + 说明，而不是含糊的 404。
		openAPIRejectDelete := func(c *gin.Context) {
			c.JSON(http.StatusMethodNotAllowed, gin.H{
				"code":    405,
				"message": "OpenAPI 不提供删除能力；如需清理数据请走内部管理接口",
			})
		}
		for _, path := range []string{
			"/Meta/Finv/Quote/Cdeu/Min/PageUni",
			"/Meta/Finv/Quote/Cdeu/Min/Upsert",
			"/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert",
		} {
			openGroup.DELETE(path, openAPIRejectDelete)
		}

		// 历史行情：导入 / 查询（统一 /Meta/Finv/Quant/Quote/ 前缀，与前端路由对应）
		apiGroup.POST("/Meta/Finv/Quant/Quote/Import/Upload", quoteImport.Upload)
		apiGroup.GET("/Meta/Finv/Quant/Quote/History/QuoteQuery", quoteQuery.Query)

		// 证券行情月归档导出与上传（运维：状态 / 任务 / 文件 / 重试 / 手动触发）
		if deps.ExportArchive != nil {
			archive := handler.NewExportArchive(deps.ExportArchive)
			secuKeyHandler := handler.NewSecuKey(
				secukey.NewService(deps.Pool),
				indicator.NewService(deps.Pool),
				uscbackfill.NewService(deps.Pool),
			)
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

			// OpenAPI 调用方管理（需求 11169）：调用方 / 凭证 / 调用审计 + 鉴权开关。
			// 只操作 open schema 与 sys_config 19xxx，不碰行情业务表。
			openAPIService := openapiSvc.NewService(openapiSvc.NewStore(deps.Pool))
			// 凭证变更后让鉴权侧的 Redis 缓存立即失效（需求 11212：吊销/停用不用等 TTL）
			openAPIService.SetCacheInvalidator(openapi.NewCacheInvalidator(deps.Redis))
			openAPIAdmin := handler.NewOpenAPIAdmin(openAPIService)
			openAPIGroup := apiGroup.Group("/Meta/Finv/OpenAPI")
			openAPIGroup.GET("/Secret/Status", openAPIAdmin.SecretStatus)
			openAPIGroup.POST("/Secret/Save", openAPIAdmin.SaveSecret)
			openAPIGroup.GET("/Settings", openAPIAdmin.Settings)
			openAPIGroup.PUT("/Settings", openAPIAdmin.SaveSettings)
			openAPIGroup.GET("/Client/Page", openAPIAdmin.ClientPage)
			openAPIGroup.POST("/Client/Save", openAPIAdmin.SaveClient)
			openAPIGroup.POST("/Client/Enable", openAPIAdmin.SetClientEnable)
			openAPIGroup.GET("/Credential/Page", openAPIAdmin.CredentialPage)
			openAPIGroup.POST("/Credential/Create", openAPIAdmin.CreateCredential)
			openAPIGroup.POST("/Credential/Enable", openAPIAdmin.SetCredentialEnable)
			openAPIGroup.POST("/Credential/Revoke", openAPIAdmin.RevokeCredential)
			openAPIGroup.POST("/Credential/Rotate", openAPIAdmin.RotateCredential)
			openAPIGroup.GET("/InvokeLog/Page", openAPIAdmin.InvokeLogPage)

			// USC 补丁规则（需求 10931）：新接口前缀统一用 /Quote/Cdeu，不再使用 /Quote/DeDup。
			patch := apiGroup.Group("/Meta/Finv/Quote/Cdeu/Usc/Patch")
			patch.GET("/List", archive.UscPatchRuleList)
			patch.POST("/Save", archive.UscPatchRuleSave)
			patch.POST("/Enable", archive.UscPatchRuleEnable)
			patch.POST("/Delete", archive.UscPatchRuleDelete)
			patch.GET("/Preview", archive.UscPatchPreview)
			patch.POST("/Run", archive.UscPatchRun)
			patch.GET("/Status", archive.UscPatchStatus)

			// 日K线数据同步（需求 10952）：拉取 moomoo 日K 并 upsert 到 cdeu.finv_quote_secu_kline_day。
			dayKline := apiGroup.Group("/Meta/Finv/Quote/Cdeu/DayKline")
			dayKline.POST("/Sync", archive.DayKlineSync)
			dayKline.GET("/Stat", archive.DayKlineStat)
			dayKline.GET("/UscList", archive.DayKlineUscList)

			// 证券实时行情快照批量 Upsert（需求 10957）：写入 cdeu.finv_cdeu_secu_realtime_stock。
			// 内部路径需登录；对外 OpenAPI 路径复用同一 handler，响应经 WrapOpenAPIResponse
			// 包成 {code,data,message}（当前 OpenAPI 组公开访问、暂不鉴权，同 Min/Upsert 先例）。
			realtime := apiGroup.Group("/Meta/Finv/Quote/Cdeu/Realtime")
			realtime.POST("/FTStockUpsert", archive.RealtimeStockUpsert)
			// 证券列表采集（需求 10976/10977）：读配置表分页拉取列表并落库。
			realtime.POST("/FTStockQuoteSnapshotSync", archive.RealtimeStockListCollect)
			// 网页「行情快照手动同步」：异步启动 + 轮询进度/逐页日志（需求 10982）。
			realtime.POST("/FTStockQuoteSnapshotSync/Run", archive.RealtimeStockListRunStart)
			realtime.GET("/FTStockQuoteSnapshotSync/Run", archive.RealtimeStockListRunStatus)
			realtime.GET("/Stock/Page", archive.RealtimeStockPage)
			// 人民币计价指标手动补全（需求 11024）：先补 rmb_exchange_rate，再补 rmb_ 金额字段。
			realtime.POST("/Stock/RmbSync", archive.RealtimeStockRmbSync)
			// 需求 10995：待录入证券（usc 为空，作为补录主数据的依据），先只提供查询。
			realtime.GET("/Stock/Pending", archive.RealtimeStockPendingPage)
			// 手动补录 usc：按派生键关联键表补空值（需求 11146）
			realtime.POST("/Stock/USCBackfill", archive.RealtimeStockUSCBackfill)
			// 待录入证券补录：预填（关联自动填）+ 录入（写两张主数据表并回写 usc，需求 11147）
			realtime.GET("/Stock/Pending/Prefill", archive.RealtimeStockPendingPrefill)
			realtime.POST("/Stock/Pending/Enroll", archive.RealtimeStockPendingEnroll)
			// 待录入证券「入队」：探测（是否存在）→ 人工确认 → 入队 / 更新键（需求 11153–11156）
			realtime.POST("/Stock/Pending/Enqueue/Probe", archive.RealtimeStockPendingEnqueueProbe)
			realtime.POST("/Stock/Pending/Enqueue", archive.RealtimeStockPendingEnqueue)
			realtime.POST("/Stock/Pending/Enqueue/Update", archive.RealtimeStockPendingEnqueueUpdate)
			// 需求 11161：「证券参数补全」——按目标表（security / collect）分别列缺口、预填、写入
			realtime.GET("/Secu/Param/Complete/Pending", archive.RealtimeSecuParamPendingPage)
			realtime.GET("/Secu/Param/Complete/Prefill", archive.RealtimeSecuParamPrefill)
			realtime.POST("/Secu/Param/Complete", archive.RealtimeSecuParamComplete)
			openGroup.POST("/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert", archive.RealtimeStockUpsert)

			// 缺省汇率（需求 10999/11002）：public.finv_fxcer，手动触发 get-fx-overview / get-fx-rate-list 全量更新 + 整表查询。
			fx := apiGroup.Group("/Meta/Finv/Quote/Cdeu/Fx")
			fx.POST("/CerSync", archive.FxCERSync)
			fx.GET("/Cer", archive.FxCERList)

			// 证券参数同步（需求 11042/11043）：Supabase 证券键表 → 本地 futu.finv_futu_secu_key，
			// 只由前端按钮手动触发，分批增量 upsert；源表名可配（系统配置 20014，默认 finv_quote_secu_key）。
			secuKeyGroup := apiGroup.Group("/Meta/Finv/Quote/Cdeu/SecuKey")
			secuKeyGroup.GET("/Status", secuKeyHandler.Status)
			secuKeyGroup.POST("/Config", secuKeyHandler.SaveConfig)
			secuKeyGroup.POST("/Sync", secuKeyHandler.Sync)
			// 异步运行（需求 11046）：启动后立刻返回 run_id，前端轮询状态拿进度与逐批实时日志。
			secuKeyGroup.POST("/Sync/Run", secuKeyHandler.StartSyncRun)
			secuKeyGroup.GET("/Sync/Run", secuKeyHandler.SyncRunStatus)
			// FT证券指标数据同步（需求 11050）：上游 public.finv_quote_secu_meta → 指标表 + 采集属性表 + 键表。
			secuKeyGroup.GET("/Indicator/Status", secuKeyHandler.IndicatorStatus)
			secuKeyGroup.POST("/Indicator/Sync/Run", secuKeyHandler.StartIndicatorSyncRun)
			secuKeyGroup.GET("/Indicator/Sync/Run", secuKeyHandler.IndicatorSyncRunStatus)

			// Tab 3「上游主键参数回填」（需求 11054）：本地 futu.finv_futu_quote_collect → 上游 public.finv_quote_secu_meta
			secuKeyGroup.GET("/UscBackfill/Status", secuKeyHandler.UscBackfillStatus)
			secuKeyGroup.POST("/UscBackfill/Sync/Run", secuKeyHandler.StartUscBackfillRun)
			secuKeyGroup.GET("/UscBackfill/Sync/Run", secuKeyHandler.UscBackfillRunStatus)
			group.POST("/Job/Run", archive.JobRun)
			group.POST("/Job/Dynamic/Run", archive.DynamicJobRun)

			// 归档运维台：查询（10 张表 + 成熟度）
			group.GET("/Usc/Page", archive.UscPage)
			group.GET("/Priority/List", archive.PriorityList)
			group.GET("/Calendar/Page", archive.CalendarPage)
			group.GET("/Maturity/Page", archive.MaturityPage)
			group.GET("/Stats/Page", archive.StatsPage)
			group.GET("/Export/Stock/Page", archive.StockPage)
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

		// USC 映射规则维护（需求 11072）：规则/条目维护 + 预演 diff + 异步应用 + 审计日志。
		// 前缀按用户指定为 /Meta/Finv/Quote/UscMap（不带 Cdeu 段）。
		if deps.USCSync != nil && deps.USCRunner != nil {
			uscMap := handler.NewUscMap(deps.USCSync, deps.USCRunner)
			uscGroup := apiGroup.Group("/Meta/Finv/Quote/UscMap")
			uscGroup.GET("/Status", uscMap.Status)
			uscGroup.GET("/Rule/List", uscMap.RuleList)
			uscGroup.POST("/Rule/Save", uscMap.RuleSave)
			uscGroup.GET("/Item/List", uscMap.ItemList)
			uscGroup.POST("/Item/Save", uscMap.ItemSave)
			uscGroup.POST("/Item/Retire", uscMap.ItemRetire)
			uscGroup.GET("/Diff/Preview", uscMap.DiffPreview)
			uscGroup.GET("/Diff/Page", uscMap.DiffPage)
			uscGroup.GET("/Map/Page", uscMap.MapPage)
			uscGroup.POST("/Sync/Run", uscMap.StartSyncRun)
			uscGroup.GET("/Sync/Run", uscMap.SyncRunStatus)
			uscGroup.GET("/Log/List", uscMap.LogList)
			uscGroup.GET("/Schedule", uscMap.ScheduleGet)
			uscGroup.POST("/Schedule", uscMap.ScheduleSave)
		}

		// BRS 备份登记维护（brs.brs_bak_rdb_table_full）：查询 / 新增编辑 / 启用禁用。
		// 前端路由 /Meta/BRS/Backup/Finv/Table 与接口前缀保持一致。
		brsBackupTable := handler.NewBrsBackupTable(brs.NewRegistry(deps.Pool))
		brsTable := apiGroup.Group("/Meta/BRS/Backup/Finv/Table")
		brsTable.GET("/List", brsBackupTable.List)
		brsTable.POST("/Save", brsBackupTable.Save)
		brsTable.POST("/Toggle", brsBackupTable.Toggle)

		// BRS 对象存储配置（sys.sys_config 39002 的 JSON 可视化配置）
		brsOss := apiGroup.Group("/Meta/BRS/Backup/Finv/OSSConfig")
		brsOss.GET("/Get", brsBackupTable.GetOSSConfig)
		brsOss.POST("/Save", brsBackupTable.SaveOSSConfig)

		// BRS 备份执行：调度配置（39001）+ 手动触发 + 实时运行状态与日志。
		// Runner 缺失时不注册（单元测试构造的 Deps 可省略）。
		if deps.BRSRunner != nil {
			brsRun := handler.NewBrsRun(deps.BRSRunner)
			schedule := apiGroup.Group("/Meta/BRS/Backup/Finv/Schedule")
			schedule.GET("/Get", brsRun.GetSchedule)
			schedule.POST("/Save", brsRun.SaveSchedule)
			run := apiGroup.Group("/Meta/BRS/Backup/Finv/Run")
			run.POST("/Manual", brsRun.TriggerManual)
			run.GET("/Status", brsRun.Status)
		}

		// BRS 大表增量（分片）备份：登记维护 + 分片计划 + 手动触发 + 运行状态。
		// 与小表全量备份完全独立（各自一套登记、执行链与运行状态）。
		if deps.BRSSliceRunner != nil {
			brsSlice := handler.NewBrsSlice(deps.BRSSliceRunner)
			sliceRule := apiGroup.Group("/Meta/BRS/Backup/Finv/Slice/Rule")
			sliceRule.GET("/List", brsSlice.ListRules)
			sliceRule.POST("/Save", brsSlice.SaveRule)
			sliceRule.POST("/Toggle", brsSlice.ToggleRule)
			apiGroup.GET("/Meta/BRS/Backup/Finv/Slice/Plan", brsSlice.Plan)
			sliceRun := apiGroup.Group("/Meta/BRS/Backup/Finv/Slice/Run")
			sliceRun.POST("/Manual", brsSlice.TriggerManual)
			sliceRun.GET("/Status", brsSlice.Status)
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
