// Package handler 提供 HTTP 处理器。
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
	"github.com/acanx/finvquant/internal/syscfg"
)

// ExportArchive 证券行情月归档导出 / 上传运维接口处理器。
//
// 路径前缀：/API/V1/Meta/Finv/Quote/DeDup/**
type ExportArchive struct {
	service *dedup.Service
}

// NewExportArchive 创建处理器。
func NewExportArchive(service *dedup.Service) *ExportArchive {
	return &ExportArchive{service: service}
}

// Status 查询归档链路状态与指标（GET .../Status）。
func (h *ExportArchive) Status(c *gin.Context) {
	metrics, err := h.service.Metrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询归档状态失败: " + err.Error()})
		return
	}
	options := h.service.Options()
	windowStart, windowEnd, _, _ := h.service.Window(c.Request.Context())
	cdeuMode, cdeuModeErr := h.service.CDEUMode(c.Request.Context())
	optionsData := gin.H{
		"enabled":            options.Enabled,
		"worker_id":          options.WorkerID,
		"time_zone":          options.TimeZone,
		"output_dir":         options.OutputDir,
		"upload_mode":        h.service.Uploader().Mode(),
		"compression":        options.Compression,
		"window_start":       formatClock(windowStart),
		"window_end":         formatClock(windowEnd),
		"page_size":          options.PageSize,
		"maturity_threshold": options.MaturityThreshold,
		// 20005 CDEU 运行模式：Manual（默认，仅手动触发）/ Auto（按定时任务自动跑）
		"cdeu_mode":             cdeuMode,
		"cdeu_schedule_enabled": cdeuMode == "Auto",
		// 20008 历史运行动态 JOB 实例保留天数（默认 30）
		"dynamic_job_retention_days": h.service.DynamicJobRetentionDays(c.Request.Context()),
	}
	if cdeuModeErr != nil {
		optionsData["cdeu_mode_error"] = cdeuModeErr.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"metrics": metrics,
			"options": optionsData,
			"jobs":    dedup.JobRuntimes(),
		},
	})
}

// Settings 查询运行时设置（GET .../Settings），当前包括导出低峰窗口。
func (h *ExportArchive) Settings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": h.settingsData()})
}

// SettingsWindow 在线修改低峰窗口（PUT .../Settings/Window）。
//
// 请求体：{"window_start":"01:00:00","window_end":"06:00:00"}（**必须 HH:MM:SS**，精确到秒）；
// 保存到 sys.sys_config 20006 后立即生效，无需重启。
func (h *ExportArchive) SettingsWindow(c *gin.Context) {
	var request struct {
		WindowStart string `json:"window_start"`
		WindowEnd   string `json:"window_end"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：window_start / window_end 必填"})
		return
	}
	// 窗口精确到秒：只接受 HH:MM:SS，拒绝 HH:MM（避免不同调用方对秒的默认值理解不一致）。
	start, err := syscfg.ParseClockSeconds(request.WindowStart)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "window_start 非法（需 HH:MM:SS）: " + err.Error()})
		return
	}
	end, err := syscfg.ParseClockSeconds(request.WindowEnd)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "window_end 非法（需 HH:MM:SS）: " + err.Error()})
		return
	}
	if err := h.service.SetWindow(c.Request.Context(), start, end); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存低峰窗口失败: " + err.Error()})
		return
	}
	h.audit(c, fmt.Sprintf("PUT /Settings/Window window=%s-%s", formatClock(start), formatClock(end)))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存，立即生效", "data": h.settingsData()})
}

// settingsData 组装运行时设置响应。
func (h *ExportArchive) settingsData() gin.H {
	ctx := context.Background()
	start, end, source, updated := h.service.Window(ctx)
	options := h.service.Options()
	return gin.H{
		"window_start": formatClock(start),
		"window_end":   formatClock(end),
		"source":       source,
		"updated_at":   updated,
		"time_zone":    options.TimeZone,
		"in_window":    h.service.InWindow(ctx, time.Now()),
		// 20008 历史运行动态 JOB 实例保留天数
		"dynamic_job_retention_days": h.service.DynamicJobRetentionDays(ctx),
	}
}

// JobRuntime 查询 10 个 Job 的运行快照（GET .../Job/Runtime）。
//
// 返回每个 Job 是否正在执行、当前阶段 / 进度 / 心跳、最近一次结果与 24 小时成功/失败统计，
// 供总览页展示「正在跑哪一阶段、是否卡住、是否已完成」。
func (h *ExportArchive) JobRuntime(c *gin.Context) {
	jobs, progress, err := h.service.JobRuntime(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询 Job 运行快照失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"generated_at":        time.Now(),
			"stale_after_seconds": int64(h.service.StaleAfter().Seconds()),
			"job_progress":        progress,
			"jobs":                jobs,
		},
	})
}

// CDEUWorkflowRun 手动启动一次完整 CDEU 工作流（POST .../CDEU/Workflow/Run）。
//
// 请求体可选：{"operator":"web"}。返回 202 与新建的 run_id，后台按阶段顺序执行。
func (h *ExportArchive) CDEUWorkflowRun(c *gin.Context) {
	var request struct {
		Operator string `json:"operator"`
	}
	_ = c.ShouldBindJSON(&request)
	operator := strings.TrimSpace(request.Operator)
	if operator == "" {
		operator = "web"
	}
	runID, err := h.service.StartCDEUWorkflow(c.Request.Context(), operator)
	if err != nil {
		if errors.Is(err, dedup.ErrCDEUWorkflowActive) {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "启动 CDEU 工作流失败: " + err.Error()})
		return
	}
	_ = h.service.Store().AppendAdminAudit(c.Request.Context(),
		fmt.Sprintf("POST /CDEU/Workflow/Run run_id=%d operator=%s", runID, operator))
	c.JSON(http.StatusAccepted, gin.H{
		"code":    202,
		"message": "已启动，后台按阶段顺序执行",
		"data":    gin.H{"run_id": runID},
	})
}

// CDEUWorkflowTerminate 强制终止运行中的 CDEU 工作流（POST .../CDEU/Workflow/Terminate）。
//
// 请求体：{"run_id": 60, "operator": "web"}；run_id 缺省或 0 时取最近一次未终态运行。
// 把该运行下所有 PENDING/READY/RUNNING 的阶段、流程、JOB、步骤统一置为 CANCELLED，
// 并释放 finv_dedup_export_job 上遗留的任务锁。
func (h *ExportArchive) CDEUWorkflowTerminate(c *gin.Context) {
	var request struct {
		RunID    int64  `json:"run_id"`
		Operator string `json:"operator"`
	}
	_ = c.ShouldBindJSON(&request)
	operator := strings.TrimSpace(request.Operator)
	if operator == "" {
		operator = "web"
	}

	result, err := h.service.TerminateCDEUWorkflow(c.Request.Context(), request.RunID)
	if err != nil {
		switch {
		case errors.Is(err, dedup.ErrCDEURunTerminal):
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		case errors.Is(err, dedup.ErrCDEUWorkflowActive):
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "没有运行中的 CDEU 工作流可终止"})
		case errors.Is(err, dedup.ErrCDEURunNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "CDEU 工作流运行实例不存在"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "终止 CDEU 工作流失败: " + err.Error()})
		}
		return
	}
	_ = h.service.Store().AppendAdminAudit(c.Request.Context(),
		fmt.Sprintf("POST /CDEU/Workflow/Terminate run_id=%d operator=%s phases=%d processes=%d jobs=%d dynamic=%d released=%d",
			result.RunID, operator, result.PhaseRuns, result.ProcessRuns, result.JobRuns,
			result.DynamicJobs, result.ReleasedJobs))
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "已强制终止",
		"data":    result,
	})
}

// CDEUOverview 查询 CDEU 工作流定义树、指定运行状态与历史运行（GET .../CDEU/Overview）。
//
// 查询参数：runId（可选，缺省取最近一次运行）、historyLimit（默认 20，最大 100）。
func (h *ExportArchive) CDEUOverview(c *gin.Context) {
	runID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("runId")), 10, 64)
	historyLimit, _ := queryIntParam(c, "historyLimit")
	overview, err := h.service.CDEUOverview(c.Request.Context(), runID, historyLimit)
	if err != nil {
		if errors.Is(err, dedup.ErrCDEURunNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "CDEU 工作流运行实例不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询 CDEU 工作流失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": overview})
}

// RunLogDetail 查询单次 Job 运行的汇总与详细日志行（GET .../RunLog/Detail?runId=）。
func (h *ExportArchive) RunLogDetail(c *gin.Context) {
	runIDValue, ok := queryIntParam(c, "runId")
	if !ok || runIDValue <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "runId 必填且必须为正整数"})
		return
	}
	detail, err := h.service.RunLogDetail(c.Request.Context(), int64(runIDValue))
	if err != nil {
		if errors.Is(err, dedup.ErrRunLogNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Job 运行记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询 Job 运行日志失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": detail})
}

// JobList 分页查询证券月导出任务（GET .../Job/List）。
func (h *ExportArchive) JobList(c *gin.Context) {
	filter := dedup.JobFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Usc:    strings.TrimSpace(c.Query("usc")),
	}
	if value, ok := queryIntParam(c, "bizMonth"); ok {
		filter.BizMonth = value
	}
	filter.Page, _ = queryIntParam(c, "page")
	filter.PageSize, _ = queryIntParam(c, "pageSize")

	page, err := h.service.Store().ListJobs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询导出任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// FileList 分页查询导出文件登记与上传状态（GET .../File/List）。
func (h *ExportArchive) FileList(c *gin.Context) {
	filter := dedup.FileFilter{
		UploadStatus: strings.TrimSpace(c.Query("uploadStatus")),
		FileType:     strings.TrimSpace(c.Query("fileType")),
		Usc:          strings.TrimSpace(c.Query("usc")),
	}
	if value, ok := queryIntParam(c, "bizMonth"); ok {
		filter.BizMonth = value
	}
	filter.Page, _ = queryIntParam(c, "page")
	filter.PageSize, _ = queryIntParam(c, "pageSize")

	page, err := h.service.Store().ListFiles(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询导出文件失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": page})
}

// JobRetry 人工重试导出任务（POST .../Job/Retry，body: {"job_id": 1}）。
func (h *ExportArchive) JobRetry(c *gin.Context) {
	var request struct {
		JobID int64 `json:"job_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.JobID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：job_id 必须为正整数"})
		return
	}
	if err := h.service.Store().ResetJobForRetry(c.Request.Context(), request.JobID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "重试导出任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已重置为 WAITING", "data": gin.H{"job_id": request.JobID}})
}

// FileRetry 人工重试上传（POST .../File/Retry，body: {"file_id": 1}）。
func (h *ExportArchive) FileRetry(c *gin.Context) {
	var request struct {
		FileID int64 `json:"file_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.FileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：file_id 必须为正整数"})
		return
	}
	if err := h.service.Store().ResetFileForRetry(c.Request.Context(), request.FileID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "重试上传失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已重置为 PENDING_UPLOAD", "data": gin.H{"file_id": request.FileID}})
}

// DynamicJobRun 手动执行一个单证券动态导出 JOB（POST .../Job/Dynamic/Run，
// body: {"dynamic_job_id": 774, "auto_continue": false}）。
//
// 异步执行并立即返回 run_log_id；auto_continue=true 时继续执行导出阶段之后的 JOB。
func (h *ExportArchive) DynamicJobRun(c *gin.Context) {
	var request struct {
		DynamicJobID int64 `json:"dynamic_job_id"`
		AutoContinue bool  `json:"auto_continue"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.DynamicJobID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：dynamic_job_id 必须为正整数"})
		return
	}
	runLogID, err := h.service.RunDynamicExportJob(c.Request.Context(), request.DynamicJobID, request.AutoContinue)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "执行动态 JOB 失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"code":    202,
		"message": "动态 JOB 已开始执行",
		"data":    gin.H{"dynamic_job_id": request.DynamicJobID, "run_log_id": runLogID, "auto_continue": request.AutoContinue},
	})
}

// JobRun 手动触发一个归档 Job（POST .../Job/Run，
// body: {"job_kind": "CALC_DAILY_EXPORT_BUDGET", "auto_continue": false}）。
//
// 异步执行并立即返回 run_log_id；auto_continue=true 时按工作流顺序续跑后续 JOB。
func (h *ExportArchive) JobRun(c *gin.Context) {
	var request struct {
		JobKind      string `json:"job_kind"`
		AutoContinue bool   `json:"auto_continue"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.JobKind) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：job_kind 不能为空"})
		return
	}
	kind := strings.ToUpper(strings.TrimSpace(request.JobKind))
	runLogID, err := h.service.TriggerWithOptions(c.Request.Context(), kind, request.AutoContinue)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"code":    202,
		"message": "已接受，任务异步执行",
		"data":    gin.H{"job_kind": kind, "run_log_id": runLogID, "auto_continue": request.AutoContinue},
	})
}

// queryIntParam 读取小驼峰整数查询参数；缺省或非法时返回 (0, false)。
func queryIntParam(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

// formatClock 自 0 点起的秒数 → HH:MM:SS（窗口配置精确到秒）。
func formatClock(seconds int) string {
	return syscfg.FormatClockSeconds(seconds)
}
