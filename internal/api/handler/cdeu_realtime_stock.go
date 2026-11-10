package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// 本文件实现「证券实时行情快照批量 Upsert」接口（需求 10957）。
// 路径：POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert
// 数据表：cdeu.finv_cdeu_secu_realtime_stock（V300115 创建，一行一证券，主键 stock_id）。
// 约定：JSON 体字段 snake_case（ApiSpec §1）。
// 日志前缀 [cdeu][realtimestock]，线上排查用
// `docker logs <容器> 2>&1 | grep '\[cdeu\]\[realtimestock\]'`。

// realtimeStockMaxBodyBytes 单请求体上限（8 MiB）：500 行 × 15 个短字段远小于该值，
// 上限只用于挡住异常大的请求，避免把内存交给调用方决定。
const realtimeStockMaxBodyBytes = 8 << 20

// RealtimeStockListRunStart POST /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run。
//
// 异步启动一次列表采集：立即返回 run_id，采集在后台执行，
// 过程日志与进度通过 GET .../FTStockQuoteSnapshotSync/Run?runId= 轮询获取。
func (h *ExportArchive) RealtimeStockListRunStart(c *gin.Context) {
	var req cdeu.StockListCollectRequest
	if c.Request.ContentLength > 0 {
		body := http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
			return
		}
	}

	run, err := h.service.StartStockListRun(req.Options())
	if err != nil {
		log.Printf("[cdeu][stocklist] api run start failed err=%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][stocklist] api run start run_id=%s", run.RunID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "任务已启动", "data": run})
}

// RealtimeStockListRunStatus GET /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run。
//
// 查询采集运行：runId 为空时返回最近一次运行。响应同时带回采集配置，
// 供前端在一个请求里刷新「配置 + 状态 + 逐页日志」。
func (h *ExportArchive) RealtimeStockListRunStatus(c *gin.Context) {
	runID := c.Query("runId")
	if runID == "" {
		runID = h.service.LatestStockListRun()
	}

	// 配置先查：即使没有任何运行记录（服务刚重启、内存运行表为空）也必须把 configs 带回，
	// 否则前端拿不到「启用配置」列表，会误报「该品种暂无启用的采集配置」（需求 10990）。
	configs, cfgErr := h.service.StockListConfigs(c.Request.Context())
	if cfgErr != nil {
		log.Printf("[cdeu][stocklist] api run status configs failed err=%v", cfgErr)
	}

	var run cdeu.StockListRun
	found := false
	if runID != "" {
		run, found = h.service.StockListRunStatus(runID)
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "暂无运行记录",
			// run_id/status/logs/results 保持原有空值形态；额外带上 configs 与 error，
			// 让前端在没有运行记录时也能渲染品种 Tab 与配置表。
			"data": gin.H{
				"run_id":  "",
				"status":  "IDLE",
				"logs":    []any{},
				"results": []any{},
				"configs": configs,
				"error":   cfgErr,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data": gin.H{
			"run":     run,
			"configs": configs,
			"error":   cfgErr,
		},
	})
}

// RealtimeStockListCollect POST /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync。
//
// 手动触发「证券列表采集」：读 cdeu.finv_cdeu_secu_realtime_sync_config 的启用配置，
// 按市场分页拉取 moomoo get-stock-list，逐页 upsert 到 cdeu.finv_cdeu_secu_realtime_stock，
// 并把 count_exists / total_available 回写到配置表。
//
// 请求体全部可选（缺省用配置表 + 默认控制参数）；主要字段见 cdeu.StockListCollectRequest：
// instrument_symbol / market_type 过滤采集范围，count_plan 覆盖计划数量，
// interval_ms / max_retries / retry_backoff_ms / request_timeout_ms / max_pages 控制采集节奏。
func (h *ExportArchive) RealtimeStockListCollect(c *gin.Context) {
	started := time.Now()

	var req cdeu.StockListCollectRequest
	if c.Request.ContentLength > 0 {
		body := http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		decoder := json.NewDecoder(body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
			return
		}
	}

	opts := req.Options()
	log.Printf("[cdeu][stocklist] api collect begin symbol=%q market=%q plan=%v page_size=%d interval_ms=%d",
		opts.FilterInstrumentSymbol, opts.FilterMarketType, opts.OverrideCountPlan,
		opts.PageSize, opts.Interval.Milliseconds())

	results, err := h.service.CollectStockList(c.Request.Context(), opts)
	if err != nil && len(results) == 0 {
		log.Printf("[cdeu][stocklist] api collect failed err=%v elapsed_ms=%d",
			err, time.Since(started).Milliseconds())
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}

	var rowsReceived, rowsInsert, rowsUpdate int64
	for _, result := range results {
		rowsReceived += result.RowsReceived
		rowsInsert += result.RowsInsert
		rowsUpdate += result.RowsUpdate
	}
	log.Printf("[cdeu][stocklist] api collect end markets=%d received=%d insert=%d update=%d elapsed_ms=%d err=%v",
		len(results), rowsReceived, rowsInsert, rowsUpdate, time.Since(started).Milliseconds(), err)

	payload := gin.H{
		"results":      results,
		"rows_receive": rowsReceived,
		"insert":       rowsInsert,
		"update":       rowsUpdate,
		"elapsed_ms":   time.Since(started).Milliseconds(),
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "采集完成", "data": payload})
}

// RealtimeStockUpsert POST /Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert。
//
// 请求体：{"items":[{"stock_id":1001,"usc":"SH600000",...,"dt_update":1767225600000}]}
// 响应 data：cdeu.RealtimeStockUpsertStat（total / insert / update）。
//
// 严格解码：出现未定义字段、items 为空、stock_id 缺失/越界/批内重复、
// dt_update 为负，统一返回 HTTP 400（业务码 400），不做静默忽略。
func (h *ExportArchive) RealtimeStockUpsert(c *gin.Context) {
	started := time.Now()

	body := http.MaxBytesReader(c.Writer, c.Request.Body, realtimeStockMaxBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var req cdeu.RealtimeStockUpsertRequest
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体只能包含一个 JSON 对象"})
		return
	}
	if err := cdeu.ValidateRealtimeStockItems(req.Items); err != nil {
		log.Printf("[cdeu][realtimestock] api upsert invalid items=%d err=%v client=%s",
			len(req.Items), err, c.ClientIP())
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	log.Printf("[cdeu][realtimestock] api upsert begin items=%d client=%s", len(req.Items), c.ClientIP())
	stat, err := h.service.Store().UpsertRealtimeStocks(c.Request.Context(), req.Items)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimestock] api upsert failed items=%d err=%v elapsed_ms=%d",
			len(req.Items), err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api upsert end items=%d inserted=%d updated=%d elapsed_ms=%d",
		stat.Total, stat.Inserted, stat.Updated, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "写入完成",
		"data":    stat,
	})
}

// RealtimeStockRmbSync POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/RmbSync?step=all|rate|amount。
//
// 手动补全快照表的人民币计价指标（需求 11024）：先把各行的 rmb_exchange_rate 按
// public.finv_fxcer 补齐（currency → CNY，CNY 自身为 1），再用该汇率换算
// rmb_market_value / rmb_circulation_market_value / rmb_trade_trunover / rmb_price_nominal。
//
// step 缺省 all（先 rate 后 amount，前端一个按钮的默认行为）；rate/amount 可单独执行。
// 幂等：值未变化的行不计入 update 统计，可反复点击。
func (h *ExportArchive) RealtimeStockRmbSync(c *gin.Context) {
	step, err := cdeu.NormalizeRMBMetricsStep(c.Query("step"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	started := time.Now()
	log.Printf("[cdeu][realtimestock] api rmb sync begin step=%s client=%s", step, c.ClientIP())
	result, err := h.service.Store().SyncRMBMetrics(c.Request.Context(), step)
	if err != nil {
		log.Printf("[cdeu][realtimestock] api rmb sync failed step=%s err=%v elapsed_ms=%d",
			step, err, time.Since(started).Milliseconds())
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "人民币计价指标更新失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api rmb sync end step=%s rows=%d rate_update=%d amount_update=%d "+
		"rate_rows=%d amount_rows=%d missing=%d elapsed_ms=%d",
		result.Step, result.RowsTotal, result.RateUpdate, result.AmountUpdate,
		result.RateRows, result.AmountRows, len(result.MissingCurrencies), result.ElapsedMs)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "人民币计价指标已更新",
		"data":    result,
	})
}

// RealtimeStockPendingPrefill GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Prefill?stockId=。
//
// 待录入证券的补录预填（需求 11147）：把「能关联到的一律自动填」的字段一次性算好返回，
// 并标注每个字段的来源（快照 / 字典 / 参考同市场 / 缺省 / 需人工），前端只需核对即可提交。
// `cdeu.finv_quote_cdeu_usc` 不在补录范围内（该表数据由运行时采集链路动态生成）。
func (h *ExportArchive) RealtimeStockPendingPrefill(c *gin.Context) {
	// stockId 是 bigint 主键（实测值已到 7.9e13），必须按 int64 解析；
	// 也不能套 realtimeStockQueryInt 的「min..max 闭区间」——那支函数在 max 缺省传 0 时会把所有合法值判为越界。
	stockID, err := realtimeStockQueryInt64(c, "stockId")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	prefill, err := h.service.Store().PendingSecuPrefill(c.Request.Context(), stockID)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimestock] api pending prefill failed stock_id=%d err=%v", stockID, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "预填完成", "data": prefill})
}

// RealtimeStockPendingEnroll POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enroll。
//
// 录入待录入证券（需求 10996 / 11147）：单事务写入 `public.finv_security` +
// `futu.finv_futu_quote_collect`，并把 `usc` 回写快照表，让该行从待录入清单消失。
// `usc` / `stock_id` / `ftmm_symbol` 由服务端派生；两张表都按主键 `ON CONFLICT DO NOTHING`（幂等、不覆盖人工数据）。
// `remark` 写 `[AUTO-FILL]` 标记，便于后续追踪是补录程序写入的。
func (h *ExportArchive) RealtimeStockPendingEnroll(c *gin.Context) {
	var req cdeu.PendingSecuEnrollRequest
	body := http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}

	started := time.Now()
	log.Printf("[cdeu][realtimestock] api pending enroll begin stock_id=%d client=%s", req.StockID, c.ClientIP())
	result, err := h.service.Store().EnrollPendingSecu(c.Request.Context(), req)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimestock] api pending enroll failed stock_id=%d err=%v elapsed_ms=%d",
			req.StockID, err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api pending enroll end stock_id=%d usc=%s security_created=%t "+
		"collect_created=%t snapshot_updated=%t elapsed_ms=%d",
		req.StockID, result.USC, result.SecurityCreated, result.CollectCreated,
		result.SnapshotUpdated, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "补录完成", "data": result})
}

// RealtimeStockUSCBackfill POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill。
//
// 手动补录 usc（需求 11146）：按派生键 `ftmm_symbol = upper(market_label || '.' || stock_code)`
// 关联本地键表 `futu.finv_futu_secu_key`，把 `usc` 为空的快照行就地补上。
// 键表里没有（或 usc 为空）的行保持不动，仍是待录入——补完刷新列表即可看到清单变短。
//
// 幂等：只写空值行，重复点击 `updated` 为 0。
func (h *ExportArchive) RealtimeStockUSCBackfill(c *gin.Context) {
	started := time.Now()
	log.Printf("[cdeu][realtimestock] api usc backfill begin client=%s", c.ClientIP())
	result, err := h.service.Store().BackfillRealtimeStockUSC(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][realtimestock] api usc backfill failed err=%v elapsed_ms=%d",
			err, time.Since(started).Milliseconds())
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "补录 usc 失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api usc backfill end null_before=%d updated=%d unresolved=%d elapsed_ms=%d",
		result.NullBefore, result.Updated, result.Unresolved, result.ElapsedMs)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "usc 补录完成",
		"data":    result,
	})
}

// RealtimeStockPage GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Page。
//
// 查询参数（小驼峰，ApiSpec §3）：keyword / marketLabel / currency /
// orderBy / order / page / pageSize。
// 响应 data：{total, page, pageSize, list[]}；list 内数值列为文本，保住 NUMERIC 精度，
// 万/亿折算与小数位裁剪由前端负责。
func (h *ExportArchive) RealtimeStockPage(c *gin.Context) {
	page, err := realtimeStockQueryInt(c, "page", 1, 1, 1_000_000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	pageSize, err := realtimeStockQueryInt(c, "pageSize", 50, 1, 500)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	desc := true
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "", "desc":
		desc = true
	case "asc":
		desc = false
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "order 只能为 asc 或 desc"})
		return
	}

	filter := cdeu.RealtimeStockListFilter{
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		MarketLabel: strings.TrimSpace(c.Query("marketLabel")),
		Currency:    strings.TrimSpace(c.Query("currency")),
		OrderBy:     strings.TrimSpace(c.Query("orderBy")),
		Desc:        desc,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}

	started := time.Now()
	total, list, err := h.service.Store().ListRealtimeStocks(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimestock] api page failed keyword=%q orderBy=%q err=%v",
			filter.Keyword, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api page end page=%d pageSize=%d total=%d returned=%d elapsed_ms=%d",
		page, pageSize, total, len(list), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"list":     list,
		},
	})
}

// RealtimeStockPendingPage GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending。
//
// 待录入证券：`cdeu.finv_cdeu_secu_realtime_stock` 里 **补全后仍为空** 的 `usc` 行。
//
// 需求 11145 起先做一次补全：按 `ftmm_symbol = upper(market_label || '.' || stock_code)`
// 关联 `futu.finv_futu_secu_key` 取 `usc`；键表里已有的证券不算待录入，不进这份清单。
// 采集会把所有在售证券写入快照表，`usc` 为空即该证券尚未录入
// `public.finv_security` + `futu.finv_futu_quote_collect` + `cdeu.finv_quote_cdeu_usc`，
// 本接口只做查询，后续再挂「一键录入」（需求 10995）。
func (h *ExportArchive) RealtimeStockPendingPage(c *gin.Context) {
	page, err := realtimeStockQueryInt(c, "page", 1, 1, 1_000_000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	pageSize, err := realtimeStockQueryInt(c, "pageSize", 50, 1, 500)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	desc := true
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "", "desc":
		desc = true
	case "asc":
		desc = false
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "order 只能为 asc 或 desc"})
		return
	}

	filter := cdeu.RealtimeStockListFilter{
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		MarketLabel: strings.TrimSpace(c.Query("marketLabel")),
		OrderBy:     strings.TrimSpace(c.Query("orderBy")),
		Desc:        desc,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
		USCMissing:  true,
		// 需求 11145：先用派生 ftmm_symbol 关联 futu.finv_futu_secu_key 补 usc，
		// 补完仍为空才算是待录入证券
		FillUSCFromKey: true,
	}

	started := time.Now()
	total, list, err := h.service.Store().ListRealtimeStocks(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimestock] api pending failed keyword=%q orderBy=%q err=%v",
			filter.Keyword, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimestock] api pending end page=%d pageSize=%d total=%d returned=%d elapsed_ms=%d",
		page, pageSize, total, len(list), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
			"list":     list,
		},
	})
}

// realtimeStockQueryInt 解析整数型查询参数：缺省用 def，越界或非整数返回错误。
// realtimeStockQueryInt64 解析必填的正整数查询参数（bigint 主键用，如 stockId）。
//
// 只校验下界，不设上界：主键值本身没有业务上界，写死 max 只会挡住合法值。
func realtimeStockQueryInt64(c *gin.Context, name string) (int64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, fmt.Errorf("%s 不能为空", name)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%s 必须是正整数（收到 %q）", name, raw)
	}
	return value, nil
}

// realtimeStockQueryInt 解析带默认值与闭区间 [min, max] 的整数查询参数。
//
// 注意 min 必须 <= max，否则任何值都会被判越界（历史事故：max 传 0 导致 stockId 全部被挡）。
func realtimeStockQueryInt(c *gin.Context, name string, def, min, max int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return def, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, fmt.Errorf("%s 必须是 %d..%d 的整数", name, min, max)
	}
	return value, nil
}
