package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/indicator"
	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
	"github.com/acanx/finvquant/internal/meta/finv/quote/uscbackfill"
)

// 本文件实现「证券参数同步」三个接口（需求 11042/11043）：
//
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status   只读快照（20013 是否配置 + 源表名 + 目标表行数）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Config   保存源表名（写系统配置 20014）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync     手动触发同步（同步执行，返回逐批日志；脚本/接口用）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run 异步启动（返回 run_id，需求 11046）
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run 运行时快照（进度 + 逐批实时日志，前端轮询）
//
// 同一处理器还承载「FT证券指标数据同步」Tab（需求 11050）：
//
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status      只读快照（三张目标表行数）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run    异步启动（返回 run_id）
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run    运行时快照（进度 + 逐批日志）
//
// 以及「上游主键参数回填」Tab（需求 11054，方向相反：本地 → 上游）：
//
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Status     只读快照（本地源表行数 / 待处理 stock_id）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run   异步启动（返回 run_id）
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run   运行时快照（进度 + 逐批日志）
//
// 数据表：源 = Supabase 的 public.finv_quote_secu_key（表名可配），
// 目标 = 本地 futu.finv_futu_secu_key。
// 约定：JSON 体字段 snake_case、统计字段用动词原形（ApiSpec §1.1/§3）；
// 日志前缀 `[cdeu][secukey]`。API Key 属敏感信息，响应里只出现项目 ID 与脱敏前缀。

// SecuKey 证券参数同步处理器（三个 Tab：基础参数同步 / 指标数据同步 / 上游主键参数回填）。
type SecuKey struct {
	service *secukey.Service
	runner  *secukey.Runner

	// indicator 指标数据同步服务与运行器（Tab 2，需求 11050）。
	indicator       *indicator.Service
	indicatorRunner *indicator.Runner

	// backfill 上游主键参数回填服务与运行器（Tab 3，需求 11054）。
	backfill       *uscbackfill.Service
	backfillRunner *uscbackfill.Runner
}

// NewSecuKey 创建处理器（异步运行器随处理器创建：状态只存内存）。
func NewSecuKey(service *secukey.Service, indicatorService *indicator.Service, backfillService *uscbackfill.Service) *SecuKey {
	return &SecuKey{
		service:         service,
		runner:          secukey.NewRunner(service),
		indicator:       indicatorService,
		indicatorRunner: indicator.NewRunner(indicatorService),
		backfill:        backfillService,
		backfillRunner:  uscbackfill.NewRunner(backfillService),
	}
}

// secuKeyConfigRequest 保存源表名的请求体。
type secuKeyConfigRequest struct {
	SourceTable string `json:"source_table"`
}

// secuKeySyncRequest 手动同步的请求体（两个字段都可省略）。
type secuKeySyncRequest struct {
	SourceTable string `json:"source_table"`
	BatchSize   int    `json:"batch_size"`
}

// uscBackfillRunRequest 「本地 usc 回推上游」的请求体（需求 11163 起支持字段选择）。
//
// Fields 不传 = 两列都写（保持历史行为）；显式传 `{"usc":false,"priority":false}` 会被拒绝。
type uscBackfillRunRequest struct {
	BatchSize int                         `json:"batch_size"`
	Fields    *uscbackfill.FieldSelection `json:"fields"`
}

// Status GET /Meta/Finv/Quote/Cdeu/SecuKey/Status。
//
// 只读本地：读系统配置 20013/20014 + 统计目标表行数，不访问 Supabase，
// 因此页面打开很快；源表实际行数在同步结果里给出（rows_total）。
func (h *SecuKey) Status(c *gin.Context) {
	snapshot, err := h.service.Status(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][secukey] api status failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取同步状态失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api status configured=%v source_table=%s target_rows=%d",
		snapshot.Configured, snapshot.SourceTable, snapshot.TargetRows)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": snapshot})
}

// SaveConfig POST /Meta/Finv/Quote/Cdeu/SecuKey/Config。
//
// 只允许改「源表名」；20013（连接配置）仍在系统配置里维护，不在本页暴露 API Key。
func (h *SecuKey) SaveConfig(c *gin.Context) {
	var req secuKeyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
		return
	}
	table, err := h.service.SetSourceTable(c.Request.Context(), req.SourceTable)
	if err != nil {
		log.Printf("[cdeu][secukey] api save config failed source_table=%q err=%v", req.SourceTable, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "源表名保存失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api save config ok source_table=%s", table)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "源表名已保存", "data": gin.H{"source_table": table}})
}

// Sync POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync。
//
// 同步执行：按 ftmm_symbol 升序分批拉取源表并增量写入目标表，全部批次结束后一次性返回。
// 前端需放宽超时（默认 30s 不够，7 万行量级约 1~3 分钟）。
// 上游失败（网络/鉴权/表不存在）返回 500 + code 2006，message 已是可读原因。
func (h *SecuKey) Sync(c *gin.Context) {
	var req secuKeySyncRequest
	// 允许空请求体：所有字段都有默认值。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
			return
		}
	}
	result, err := h.service.Sync(c.Request.Context(), secukey.SyncRequest{
		SourceTable: req.SourceTable,
		BatchSize:   req.BatchSize,
	})
	if err != nil {
		log.Printf("[cdeu][secukey] api sync failed source_table=%q err=%v", req.SourceTable, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "证券参数同步失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api sync ok source=%s rows_total=%d received=%d insert=%d update=%d unchanged=%d skipped=%d batches=%d elapsed_ms=%d",
		result.SourceTable, result.RowsTotal, result.RowsReceive, result.Insert, result.Update,
		result.Unchanged, result.Skipped, result.Batches, result.ElapsedMs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "证券参数已同步", "data": result})
}

// StartSyncRun POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run。
//
// 异步启动：立刻返回 run_id，前端随后轮询 SyncRunStatus 取进度与逐批实时日志，
// 不必等整次同步结束（需求 11046）。已有运行在执行时返回 409。
func (h *SecuKey) StartSyncRun(c *gin.Context) {
	var req secuKeySyncRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
			return
		}
	}
	run, err := h.runner.Start(secukey.SyncRequest{
		SourceTable: req.SourceTable,
		BatchSize:   req.BatchSize,
	})
	if err != nil {
		log.Printf("[cdeu][secukey] api run start rejected: %v", err)
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api run started run_id=%s", run.RunID)
	c.JSON(http.StatusAccepted, gin.H{
		"code":    0,
		"message": "同步任务已启动",
		"data":    run,
	})
}

// SyncRunStatus GET /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run?runId=。
//
// 返回运行时快照：status（RUNNING/DONE/FAILED）、progress（0~100）、累计计数与 logs（逐批实时日志）。
// runId 省略时返回最近一次运行；从未运行过返回 404。
func (h *SecuKey) SyncRunStatus(c *gin.Context) {
	runID := strings.TrimSpace(c.Query("runId"))
	run, ok := h.runner.Status(runID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "没有找到同步运行记录（runId=" + runID + "），请先点击「立即同步」",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": run})
}

// IndicatorStatus GET /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status。
//
// 只读本地：读系统配置 20013 并统计三张目标表行数（指标表 / 采集属性表 / 证券键表），
// 不访问 Supabase；源表行数在同步结果的 rows_total 里给出。
func (h *SecuKey) IndicatorStatus(c *gin.Context) {
	snapshot, err := h.indicator.Status(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][indicator] api status failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取指标同步状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": snapshot})
}

// StartIndicatorSyncRun POST /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run。
//
// 异步启动指标同步：立刻返回 run_id，前端轮询 IndicatorSyncRunStatus 取进度与逐批日志。
// 已有运行在执行时返回 409。
func (h *SecuKey) StartIndicatorSyncRun(c *gin.Context) {
	var req secuKeySyncRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
			return
		}
	}
	run, err := h.indicatorRunner.Start(indicator.Request{BatchSize: req.BatchSize})
	if err != nil {
		log.Printf("[cdeu][indicator] api run start rejected: %v", err)
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][indicator] api run started run_id=%s", run.RunID)
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "指标同步任务已启动", "data": run})
}

// IndicatorSyncRunStatus GET /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run?runId=。
func (h *SecuKey) IndicatorSyncRunStatus(c *gin.Context) {
	runID := strings.TrimSpace(c.Query("runId"))
	run, ok := h.indicatorRunner.Status(runID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "没有找到指标同步运行记录（runId=" + runID + "），请先点击「立即同步」",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": run})
}

// UscBackfillStatus GET /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Status。
//
// 只读本地：读系统配置 20013 + 统计本地来源表 futu.finv_futu_secu_key 的行数、
// usc 有值行数（= 一次回推的待处理量）与 usc 为空行数，不访问 Supabase。
func (h *SecuKey) UscBackfillStatus(c *gin.Context) {
	snapshot, err := h.backfill.Status(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][uscbackfill] api status failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取回填状态失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][uscbackfill] api status configured=%v local_rows=%d with_usc=%d empty_usc=%d",
		snapshot.Configured, snapshot.LocalRows, snapshot.LocalWithUSC, snapshot.LocalEmptyUSC)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": snapshot})
}

// StartUscBackfillRun POST /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run。
//
// 异步启动「本地 usc 回推上游」：立刻返回 run_id，前端轮询 UscBackfillRunStatus 取进度与逐批日志。
// 已有运行在执行时返回 409。注意本接口会**写入线上 Supabase**（PATCH public.finv_quote_secu_key.usc，
// 按 ftmm_symbol 关联，只写 usc 一列）。
func (h *SecuKey) StartUscBackfillRun(c *gin.Context) {
	var req uscBackfillRunRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
			return
		}
	}
	// fields（需求 11163）：不传默认两列都写；显式全不勾由 Start 返回入参错误（400，不是 409）
	run, err := h.backfillRunner.Start(uscbackfill.Request{
		BatchSize: req.BatchSize,
		Fields:    req.Fields,
	})
	if err != nil {
		status, code := http.StatusConflict, 409
		if errors.Is(err, uscbackfill.ErrInvalidRequest) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][uscbackfill] api run start rejected: %v", err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][uscbackfill] api run started run_id=%s fields=%s",
		run.RunID, run.Fields.Text())
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "本地 usc 回推上游任务已启动", "data": run})
}

// UscBackfillRunStatus GET /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run?runId=。
func (h *SecuKey) UscBackfillRunStatus(c *gin.Context) {
	runID := strings.TrimSpace(c.Query("runId"))
	run, ok := h.backfillRunner.Status(runID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "没有找到 usc 回推运行记录（runId=" + runID + "），请先点击「立即回推」",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": run})
}
