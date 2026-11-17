// Package handler 提供 HTTP 处理器。
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/quote"
)

// QuoteQuery 历史行情查询处理器。
type QuoteQuery struct {
	service *quote.Service
}

// NewQuoteQuery 创建查询处理器。
func NewQuoteQuery(service *quote.Service) *QuoteQuery {
	return &QuoteQuery{service: service}
}

// Query 处理 GET /API/V1/Quote/Query：
// 按证券代码 + 交易日（支持 1 日 / 5 日多日回溯）查询分钟级 K 线（周期目前仅支持 Min=1 分钟）。
// 支持 page/page_size 分页。
func (h *QuoteQuery) Query(c *gin.Context) {
	secuCode := c.Query("secu_code")
	secuName := c.Query("secu_name") // 证券名称（可选，前端从 usc 字典选中后回传，便于确认证券）
	dateStr := c.Query("date")
	period := c.Query("period")
	if period == "" {
		period = "Min"
	}

	if secuCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "secu_code 不能为空"})
		return
	}
	date, err := strconv.Atoi(dateStr)
	if err != nil || date <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "date 必须为有效的交易日期（yyyymmdd）"})
		return
	}

	days := parseIntDefault(c.Query("days"), 1)
	page := parseIntDefault(c.Query("page"), 1)
	pageSize := parseIntDefault(c.Query("page_size"), 240)

	ctx, cancel := contextWithTimeout(c, 30*time.Second)
	defer cancel()

	bars, total, err := h.service.QueryBars(ctx, secuCode, date, period, days, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"secu_code": secuCode,
			"secu_name": secuName, // 回显证券名称（可为空）
			"date":      date,
			"period":    period,
			"days":      days,
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"count":     len(bars),
			"bars":      bars,
		},
	})
}

// parseIntDefault 解析正整数查询参数，非法或为空时返回默认值。
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
