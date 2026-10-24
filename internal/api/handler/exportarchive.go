// Package handler 提供 HTTP 处理器。
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/dedup"
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
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"metrics": metrics,
			"options": gin.H{
				"enabled":            options.Enabled,
				"worker_id":          options.WorkerID,
				"time_zone":          options.TimeZone,
				"output_dir":         options.OutputDir,
				"upload_mode":        h.service.Uploader().Mode(),
				"compression":        options.Compression,
				"window_start":       formatClock(options.WindowStart),
				"window_end":         formatClock(options.WindowEnd),
				"page_size":          options.PageSize,
				"maturity_threshold": options.MaturityThreshold,
			},
			"jobs": dedup.JobRuntimes(),
		},
	})
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

// JobRun 手动触发一个归档 Job（POST .../Job/Run，body: {"job_kind": "CALC_DAILY_EXPORT_BUDGET"}）。
//
// 异步执行：返回 202 表示已接受；执行结果通过 .../Status 与 Job/List 观察。
func (h *ExportArchive) JobRun(c *gin.Context) {
	var request struct {
		JobKind string `json:"job_kind"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.JobKind) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误：job_kind 不能为空"})
		return
	}
	kind := strings.ToUpper(strings.TrimSpace(request.JobKind))
	if err := h.service.Trigger(kind); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"code":    202,
		"message": "已接受，任务异步执行",
		"data":    gin.H{"job_kind": kind},
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

// formatClock 分钟数 → HH:MM。
func formatClock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
