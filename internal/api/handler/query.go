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
// 按证券代码 + ts 时间范围（startTs/endTs，UTC 秒）查询分钟级 K 线（周期目前仅支持 Min=1 分钟）。
// 返回时间范围内全部记录（不分页，适配 A 股/港股/美股/24h 电子盘单日数据量差异）。
// URL 查询参数遵循小驼峰规范（secuCode/secuName/startTs/endTs）；兼容旧 snake_case 参数。
func (h *QuoteQuery) Query(c *gin.Context) {
	secuCode := c.Query("secuCode")
	if secuCode == "" {
		secuCode = c.Query("secu_code") // 兼容旧参数
	}
	secuName := c.Query("secuName") // 证券名称（可选，前端从 usc 字典选中后回传，便于确认证券）
	if secuName == "" {
		secuName = c.Query("secu_name")
	}
	period := c.Query("period")
	if period == "" {
		period = "Min"
	}

	if secuCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "secuCode 不能为空"})
		return
	}

	// ts 范围：优先 startTs/endTs（前端已把日期+N日换算为 UTC 秒）；
	// 兼容旧参数 date（yyyymmdd）+ days（回溯 N 个自然日）→ 转 ts 范围。
	var startTS, endTS int64
	st := c.Query("startTs")
	if st == "" {
		st = c.Query("start_ts") // 兼容旧参数
	}
	if st != "" {
		var err error
		startTS, err = strconv.ParseInt(st, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "startTs 必须为整数"})
			return
		}
		et := c.Query("endTs")
		if et == "" {
			et = c.Query("end_ts")
		}
		if et == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "缺少 endTs"})
			return
		}
		endTS, err = strconv.ParseInt(et, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "endTs 必须为整数"})
			return
		}
	} else {
		dateStr := c.Query("date")
		date, err := strconv.Atoi(dateStr)
		if err != nil || date <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "date 必须为有效的交易日期（yyyymmdd）"})
			return
		}
		days := parseIntDefault(c.Query("days"), 1)
		if days < 1 {
			days = 1
		}
		if days > 30 {
			days = 30
		}
		startTS, endTS = quote.DateRangeToTS(date, days)
	}
	if startTS <= 0 || endTS < startTS {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "时间范围非法"})
		return
	}

	ctx, cancel := contextWithTimeout(c, 30*time.Second)
	defer cancel()

	bars, total, err := h.service.QueryBars(ctx, secuCode, period, startTS, endTS)
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
			"start_ts":  startTS,
			"end_ts":    endTS,
			"period":    period,
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
