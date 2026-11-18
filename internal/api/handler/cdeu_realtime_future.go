package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// RealtimeFuturePage GET /Meta/Finv/Quote/Cdeu/Realtime/Future/Page。
//
// 查询参数（小驼峰，ApiSpec §3）：keyword / futuresType / marketLabel / onlyOnline /
// orderBy / order / page / pageSize。
// 数据源 cdeu.finv_quote_realtime_future（主键 stock_id，一行一合约）。
// 响应 data：{total, page, pageSize, list[]}；数值列为文本，精度由前端裁剪。
func (h *ExportArchive) RealtimeFuturePage(c *gin.Context) {
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

	filter := cdeu.RealtimeFutureListFilter{
		Keyword:     strings.TrimSpace(c.Query("keyword")),
		FuturesType: strings.TrimSpace(c.Query("futuresType")),
		MarketLabel: strings.TrimSpace(c.Query("marketLabel")),
		OnlyOnline: strings.EqualFold(strings.TrimSpace(c.Query("onlyOnline")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.Query("onlyOnline")), "true"),
		OrderBy: strings.TrimSpace(c.Query("orderBy")),
		Desc:    desc,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	}

	started := time.Now()
	total, list, err := h.service.Store().ListRealtimeFutures(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimefuture] api page failed keyword=%q orderBy=%q err=%v",
			filter.Keyword, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimefuture] api page end page=%d pageSize=%d total=%d returned=%d elapsed_ms=%d",
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

// RealtimeFutureTypes GET /Meta/Finv/Quote/Cdeu/Realtime/Future/Types。
//
// 期货分类下拉项：cdeu.finv_quote_cdeu_future_sync_config 的 futures_type / futures_type_name。
func (h *ExportArchive) RealtimeFutureTypes(c *gin.Context) {
	options, err := h.service.Store().ListRealtimeFutureTypes(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][realtimefuture] api types failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": gin.H{"list": options}})
}
