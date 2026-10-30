// Package handler 提供 HTTP 处理器。
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
)

// 本文件实现归档运维台的查询接口与管理員编辑接口（路径前缀 /API/V1/Meta/Finv/Quote/DeDup/）。
// 约定：写操作成功后会追加一条管理审计（finv_dedup_job_run_log_export_file，job_kind=MANUAL_ADMIN_EDIT）。

// adminFilter 从查询参数解析通用过滤条件。
func adminFilter(c *gin.Context) dedup.AdminFilter {
	filter := dedup.AdminFilter{
		Usc:      strings.TrimSpace(c.Query("usc")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Status:   strings.TrimSpace(c.Query("status")),
		Kind:     strings.TrimSpace(c.Query("jobKind")),
		DateFrom: strings.TrimSpace(c.Query("dateFrom")),
		DateTo:   strings.TrimSpace(c.Query("dateTo")),
	}
	if value, ok := queryIntParam(c, "bizMonth"); ok {
		filter.BizMonth = value
	}
	if value, ok := queryInt64Param(c, "fileId"); ok {
		filter.FileID = value
	}
	if value, ok := queryInt64Param(c, "jobId"); ok {
		filter.JobID = value
	}
	filter.Page, _ = queryIntParam(c, "page")
	filter.PageSize, _ = queryIntParam(c, "pageSize")
	return filter
}

// audit 追加一条管理员操作审计（失败只记录日志，不影响主流程返回）。
func (h *ExportArchive) audit(c *gin.Context, detail string) {
	if err := h.service.Store().AppendAdminAudit(c.Request.Context(), detail); err != nil {
		c.Header("X-Audit-Warning", "审计写入失败: "+err.Error())
	}
}

// UscPage 分页查询证券主数据（GET .../Usc/Page）。
func (h *ExportArchive) UscPage(c *gin.Context) {
	page, err := h.service.Store().ListUscs(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询证券主数据失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// UscUpsert 新增或更新证券主数据（PUT .../Usc）。
func (h *ExportArchive) UscUpsert(c *gin.Context) {
	var request struct {
		Usc              string  `json:"usc"`
		Category         *string `json:"category"`
		DayExpectRows    *int    `json:"day_expect_rows"`
		DateList         *string `json:"date_list"`
		DateDelist       *string `json:"date_delist"`
		FlagAutoDiscover *bool   `json:"flag_auto_discover"`
		Region           *string `json:"region"`
		Market           *string `json:"market"`
		Provider         *string `json:"provider"`
		Name             *string `json:"name"`
		Timezone         *string `json:"timezone"`
		Currency         *string `json:"currency"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Usc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：usc 不能为空"})
		return
	}
	if request.DayExpectRows != nil && *request.DayExpectRows < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "day_expect_rows 不能为负数"})
		return
	}
	for _, date := range []*string{request.DateList, request.DateDelist} {
		if date == nil || *date == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", *date); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "日期格式应为 yyyy-MM-dd"})
			return
		}
	}
	input := dedup.UscUpsertInput{
		Usc:              strings.TrimSpace(request.Usc),
		Category:         request.Category,
		DayExpectRows:    request.DayExpectRows,
		DateList:         request.DateList,
		DateDelist:       request.DateDelist,
		FlagAutoDiscover: request.FlagAutoDiscover,
		Region:           request.Region,
		Market:           request.Market,
		Provider:         request.Provider,
		Name:             request.Name,
		Timezone:         request.Timezone,
		Currency:         request.Currency,
	}
	if err := h.service.Store().UpsertUsc(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "更新证券主数据失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Usc usc=%s day_expect_rows=%s", input.Usc, formatIntPtr(input.DayExpectRows)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已更新", "data": gin.H{"usc": input.Usc}})
}

// PriorityList 查询证券优先级（GET .../Priority/List）。
func (h *ExportArchive) PriorityList(c *gin.Context) {
	page, err := h.service.Store().ListPriorities(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询优先级失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// PriorityUpsert 新增或更新证券优先级（PUT .../Priority）。
func (h *ExportArchive) PriorityUpsert(c *gin.Context) {
	var request struct {
		Usc          string `json:"usc"`
		PriorityRank int    `json:"priority_rank"`
		SlaDays      int    `json:"sla_days"`
		MaxWaitDays  int    `json:"max_wait_days"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Usc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：usc 不能为空"})
		return
	}
	if request.PriorityRank <= 0 || request.SlaDays < 0 || request.MaxWaitDays < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "priority_rank 必须为正整数，sla_days / max_wait_days 不能为负"})
		return
	}
	input := dedup.PriorityUpsertInput{
		Usc:          strings.TrimSpace(request.Usc),
		PriorityRank: request.PriorityRank,
		SlaDays:      request.SlaDays,
		MaxWaitDays:  request.MaxWaitDays,
	}
	if err := h.service.Store().SetUscPriority(c.Request.Context(), input.Usc, &input); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, dedup.ErrUscNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"code": status, "message": "更新优先级失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Priority usc=%s rank=%d sla=%d maxwait=%d",
		input.Usc, input.PriorityRank, input.SlaDays, input.MaxWaitDays))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已更新", "data": gin.H{"usc": input.Usc}})
}

// PriorityDelete 删除证券优先级（DELETE .../Priority）。
func (h *ExportArchive) PriorityDelete(c *gin.Context) {
	usc := strings.TrimSpace(c.Query("usc"))
	if usc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "usc 不能为空"})
		return
	}
	// V200051 起「删除优先级」等价于「清空覆盖」，证券行本身保留。
	if err := h.service.Store().SetUscPriority(c.Request.Context(), usc, nil); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, dedup.ErrUscNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"code": status, "message": "清除优先级失败: " + err.Error()})
		return
	}
	h.audit(c, "DELETE /Priority usc="+usc)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已清除优先级覆盖", "data": gin.H{"usc": usc}})
}

// CalendarPage 分页查询交易日历（GET .../Calendar/Page）。
func (h *ExportArchive) CalendarPage(c *gin.Context) {
	page, err := h.service.Store().ListCalendar(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询交易日历失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// CalendarUpsert 新增或更新交易日历标记（PUT .../Calendar）。
func (h *ExportArchive) CalendarUpsert(c *gin.Context) {
	var request struct {
		Usc          string `json:"usc"`
		TradeDate    int    `json:"trade_date"`
		FlagTradeDay bool   `json:"flag_trade_day"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Usc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：usc 不能为空"})
		return
	}
	if request.TradeDate < 19000101 || request.TradeDate > 29991231 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "trade_date 应为 yyyymmdd 整数"})
		return
	}
	input := dedup.CalendarUpsertInput{
		Usc:          strings.TrimSpace(request.Usc),
		TradeDate:    request.TradeDate,
		FlagTradeDay: request.FlagTradeDay,
	}
	if err := h.service.Store().UpsertCalendar(c.Request.Context(), input); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "更新交易日历失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Calendar usc=%s trade_date=%d flag=%v", input.Usc, input.TradeDate, input.FlagTradeDay))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已更新", "data": gin.H{"usc": input.Usc, "trade_date": input.TradeDate}})
}

// CalendarDelete 删除交易日历（DELETE .../Calendar）。
func (h *ExportArchive) CalendarDelete(c *gin.Context) {
	usc := strings.TrimSpace(c.Query("usc"))
	tradeDate, ok := queryIntParam(c, "tradeDate")
	if usc == "" || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "usc 与 tradeDate 不能为空"})
		return
	}
	if err := h.service.Store().DeleteCalendar(c.Request.Context(), usc, tradeDate); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "删除交易日历失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("DELETE /Calendar usc=%s trade_date=%d", usc, tradeDate))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已删除", "data": gin.H{"usc": usc, "trade_date": tradeDate}})
}

// MaturityPage 分页查询成熟度（GET .../Maturity/Page）。
func (h *ExportArchive) MaturityPage(c *gin.Context) {
	page, err := h.service.Store().ListMaturity(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询成熟度失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// MaturityRefresh 触发成熟度重算（POST .../Maturity/Refresh）。
func (h *ExportArchive) MaturityRefresh(c *gin.Context) {
	h.runJobAsync(c, dedup.JobRefreshQuoteMaturity, "REFRESH_QUOTE_MATURITY")
}

// StatsPage 分页查询每日入口统计（GET .../Stats/Page）。
func (h *ExportArchive) StatsPage(c *gin.Context) {
	page, err := h.service.Store().ListIngestStats(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询入口统计失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// BudgetPage 分页查询每日预算（GET .../Budget/Page）。
func (h *ExportArchive) BudgetPage(c *gin.Context) {
	page, err := h.service.Store().ListBudgets(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询预算失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// BudgetOverride 覆盖当日预算（PUT .../Budget/Override）。
func (h *ExportArchive) BudgetOverride(c *gin.Context) {
	var request struct {
		BizDate     string  `json:"biz_date"`
		BudgetRows  int64   `json:"budget_rows"`
		HardCapRows int64   `json:"hard_cap_rows"`
		Factor      float64 `json:"factor"`
		Status      string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误"})
		return
	}
	if request.BudgetRows <= 0 || request.HardCapRows < request.BudgetRows {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "budget_rows 必须为正，且 hard_cap_rows 不得小于 budget_rows"})
		return
	}
	if request.Factor < 1.0 || request.Factor > 3.0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "factor 应在 1.00 ~ 3.00 之间"})
		return
	}
	if request.Status == "" {
		request.Status = "READY"
	}
	bizDate := h.service.Options().Today()
	if request.BizDate != "" {
		parsed, err := time.Parse("2006-01-02", request.BizDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "biz_date 格式应为 yyyy-MM-dd"})
			return
		}
		bizDate = parsed
	}
	input := dedup.BudgetOverrideInput{
		BudgetRows:  request.BudgetRows,
		HardCapRows: request.HardCapRows,
		Factor:      request.Factor,
		Status:      request.Status,
	}
	if err := h.service.Store().OverrideBudget(c.Request.Context(), input, bizDate); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "覆盖预算失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Budget/Override date=%s budget_rows=%d factor=%.2f",
		bizDate.Format("2006-01-02"), input.BudgetRows, input.Factor))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已覆盖", "data": gin.H{"biz_date": bizDate.Format("2006-01-02")}})
}

// WatermarkList 查询扫描水位（GET .../Watermark/List）。
func (h *ExportArchive) WatermarkList(c *gin.Context) {
	page, err := h.service.Store().ListWatermarks(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询水位失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// WatermarkReset 重置扫描水位（PUT .../Watermark）。
func (h *ExportArchive) WatermarkReset(c *gin.Context) {
	var request struct {
		WatermarkName string `json:"watermark_name"`
		LastValue     string `json:"last_value"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.WatermarkName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：watermark_name 不能为空"})
		return
	}
	value, err := time.Parse(time.RFC3339, request.LastValue)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "last_value 应为 RFC3339 时间，如 2026-10-01T00:00:00Z"})
		return
	}
	if err := h.service.Store().ResetWatermark(c.Request.Context(), request.WatermarkName, value); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "重置水位失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Watermark name=%s value=%s", request.WatermarkName, value.Format(time.RFC3339)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已重置", "data": gin.H{"watermark_name": request.WatermarkName}})
}

// RunLogPage 分页查询 Job 运行日志（GET .../RunLog/Page）。
func (h *ExportArchive) RunLogPage(c *gin.Context) {
	page, err := h.service.Store().ListRunLogs(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询运行日志失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// UploadLogPage 分页查询上传审计（GET .../UploadLog/Page）。
func (h *ExportArchive) UploadLogPage(c *gin.Context) {
	page, err := h.service.Store().ListUploadLogs(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询上传审计失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// PatchPage 分页查询补丁任务（GET .../Patch/Page）。
func (h *ExportArchive) PatchPage(c *gin.Context) {
	page, err := h.service.Store().ListPatches(c.Request.Context(), adminFilter(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询补丁任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// PatchRetry 重试补丁任务（POST .../Patch/Retry）。
func (h *ExportArchive) PatchRetry(c *gin.Context) {
	var request struct {
		PatchID int64 `json:"patch_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.PatchID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：patch_id 必须为正整数"})
		return
	}
	if err := h.service.Store().ResetPatchForRetry(c.Request.Context(), request.PatchID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "重试补丁失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("POST /Patch/Retry patch_id=%d", request.PatchID))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已重置为 PENDING", "data": gin.H{"patch_id": request.PatchID}})
}

// JobPause 暂停导出任务（POST .../Job/Pause）。
func (h *ExportArchive) JobPause(c *gin.Context) {
	h.setJobStatus(c, dedup.JobStatusPaused, "管理员暂停")
}

// JobCancel 取消导出任务（POST .../Job/Cancel，置为 FAILED 并记录原因）。
func (h *ExportArchive) JobCancel(c *gin.Context) {
	h.setJobStatus(c, dedup.JobStatusFailed, "管理员取消")
}

// setJobStatus 解析 job_id 并变更任务状态。
func (h *ExportArchive) setJobStatus(c *gin.Context, status, reason string) {
	var request struct {
		JobID int64 `json:"job_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.JobID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：job_id 必须为正整数"})
		return
	}
	if err := h.service.Store().SetJobStatus(c.Request.Context(), request.JobID, status, reason); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "变更任务状态失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("POST /Job/Status job_id=%d status=%s reason=%s", request.JobID, status, reason))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已更新", "data": gin.H{"job_id": request.JobID, "status": status}})
}

// runJobAsync 异步触发一个 Job（供成熟度重算等复用）。
func (h *ExportArchive) runJobAsync(c *gin.Context, kind, label string) {
	runLogID, err := h.service.TriggerWithOptions(c.Request.Context(), kind, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.audit(c, "POST /Trigger "+label)
	c.JSON(http.StatusAccepted, gin.H{"code": 202, "message": "已接受，任务异步执行", "data": gin.H{"job_kind": kind, "run_log_id": runLogID}})
}

// queryInt64Param 读取小驼峰 int64 查询参数；缺省或非法时返回 (0, false)。
func queryInt64Param(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, false
	}
	var value int64
	if _, err := fmt.Sscan(raw, &value); err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

// formatIntPtr 指针整数格式化（nil → "-"），用于审计详情。
func formatIntPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

// StockPage 查询指定业务月份的存量清单（GET .../Export/Stock/Page，需求 10835）。
//
// 数据源为 cdeu.finv_quote_secu_kline_min（主表）+ cdeu.finv_quote_cdeu_quote_maturity_mon，
// 用于在跑导出流程前确认「该月会导出哪些证券、各多少行、文件大致多大」。
func (h *ExportArchive) StockPage(c *gin.Context) {
	bizMonth, _ := queryIntParam(c, "bizMonth")
	pageNo, ok := queryIntParam(c, "page")
	if !ok || pageNo <= 0 {
		pageNo = 1
	}
	pageSize, ok := queryIntParam(c, "pageSize")
	if !ok || pageSize <= 0 {
		pageSize = 100
	}
	filter := dedup.StockFilter{
		BizMonth: bizMonth,
		Usc:      strings.TrimSpace(c.Query("usc")),
		Limit:    pageSize,
		Offset:   (pageNo - 1) * pageSize,
	}
	page, err := h.service.StockPage(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询月份存量清单失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}
