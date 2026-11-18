package handler

// 收藏的加密货币行情快照采集（需求 11314）。
//
// 路径：
//
//	POST /API/V1/Meta/Finv/Quote/Crypto/Fav/Snapshot/Sync
//
// 流程：quote.finv_quote_secu_fav（收藏的加密货币）→ moomoo OpenAPI market-snapshot →
// upsert cdeu.finv_quote_secu_realtime_crypto。
//
// 凭据只从环境变量读取（FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）；
// 未配置时返回 503，提示先配置，不落盘、不回显私钥。

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// CryptoFavSnapshotSync POST /Meta/Finv/Quote/Crypto/Fav/Snapshot/Sync。
//
// 请求体可选：{"timeout_ms": 60000, "batch_size": 400}。
func (h *ExportArchive) CryptoFavSnapshotSync(c *gin.Context) {
	var req struct {
		TimeoutMs int `json:"timeout_ms"`
		BatchSize int `json:"batch_size"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
			return
		}
	}

	client, err := cdeu.NewMoomooOpenAPIClientFromEnv()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "moomoo OpenAPI 初始化失败: " + err.Error()})
		return
	}
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    2006,
			"message": "未配置 moomoo OpenAPI 凭据（FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）",
		})
		return
	}

	opts := cdeu.CryptoFavSnapshotCollectOptions{BatchSize: req.BatchSize}
	if req.TimeoutMs > 0 {
		opts.Timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	started := time.Now()
	collector := cdeu.NewCryptoFavSnapshotCollector(client, h.service.Store(), nil)
	result, err := collector.Collect(c.Request.Context(), opts)
	if err != nil {
		log.Printf("[cdeu][crypto] api sync failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "采集失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][crypto] api sync end usc_total=%d codes=%d rows=%d insert=%d update=%d elapsed_ms=%d",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "采集完成",
		"data":    result,
	})
}
