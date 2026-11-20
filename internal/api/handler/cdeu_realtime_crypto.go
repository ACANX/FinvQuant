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

// RealtimeCryptoPage GET /Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page。
//
// 查询参数（小驼峰，ApiSpec §3）：keyword / quoteCurrency / secStatus / suspension /
// orderBy / order / page / pageSize。
// 数据源 cdeu.finv_quote_secu_realtime_crypto（主键 usc，一行一币种；由加密货币快照采集写入）。
// 响应 data：{total, page, pageSize, list[]}；数值列为文本，精度由前端裁剪。
func (h *ExportArchive) RealtimeCryptoPage(c *gin.Context) {
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

	filter := cdeu.RealtimeCryptoListFilter{
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		QuoteCurrency: strings.TrimSpace(c.Query("quoteCurrency")),
		SecStatus:     strings.TrimSpace(c.Query("secStatus")),
		Suspension:    strings.TrimSpace(c.Query("suspension")),
		OrderBy:       strings.TrimSpace(c.Query("orderBy")),
		Desc:          desc,
		Limit:         pageSize,
		Offset:        (page - 1) * pageSize,
	}

	started := time.Now()
	total, list, err := h.service.Store().ListRealtimeCryptos(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrRealtimeStockInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][realtimecrypto] api page failed keyword=%q orderBy=%q err=%v",
			filter.Keyword, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][realtimecrypto] api page end page=%d pageSize=%d total=%d returned=%d elapsed_ms=%d",
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

// RealtimeCryptoQuoteCurrencies GET /Meta/Finv/Quote/Cdeu/Realtime/Crypto/QuoteCurrencies。
//
// 计价币种下拉项（表内 DISTINCT，按字母序）。
func (h *ExportArchive) RealtimeCryptoQuoteCurrencies(c *gin.Context) {
	values, err := h.service.Store().ListRealtimeCryptoQuoteCurrencies(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][realtimecrypto] api quote currencies failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": gin.H{"list": values}})
}
