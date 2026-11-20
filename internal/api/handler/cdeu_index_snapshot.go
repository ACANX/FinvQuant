package handler

// 收藏的美股/港股指数行情快照采集（需求 11333）。
//
// 路径：
//
//	POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Index/Fav/Snapshot/Sync
//
// 流程：quote.finv_quote_secu_fav（收藏的美股/港股指数）→ moomoo OpenAPI quote/snapshot
// → 字段/类型转换 → upsert cdeu.finv_quote_secu_realtime（按 usc，有值才覆盖）。
//
// 凭据复用「加密货币」Tab 保存的 moomoo 配置（sys.sys_config 20016），
// 未配置时返回 503，不落盘、不回显私钥。

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// IndexFavSnapshotSync POST /Meta/Finv/Quote/Cdeu/Realtime/Index/Fav/Snapshot/Sync。
//
// 请求体可选：{"timeout_ms": 60000}；单批 code 数固定 400（moomoo 接口上限）。
func (h *ExportArchive) IndexFavSnapshotSync(c *gin.Context) {
	var req struct {
		TimeoutMs int `json:"timeout_ms"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
			return
		}
	}

	client, err := h.service.Store().MoomooClientFromConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "moomoo OpenAPI 初始化失败: " + err.Error()})
		return
	}
	if client == nil {
		cfg, cfgErr := h.service.Store().LoadMoomooConfig(c.Request.Context())
		message := "未配置 moomoo OpenAPI 凭据（sys_config 20016 或环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）"
		if cfgErr == nil && strings.TrimSpace(cfg.AppKey) != "" {
			message = "moomoo OpenAPI 凭据缺少私钥（sys_config 20016 需为 `AppKey|私钥`；" +
				"也可用环境变量 FINV_MOOMOO_PRIVATE_KEY 兜底）"
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 2006, "message": message})
		return
	}

	opts := cdeu.IndexFavSnapshotCollectOptions{Timeout: 60 * time.Second}
	if req.TimeoutMs > 0 {
		opts.Timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	started := time.Now()
	collector := cdeu.NewIndexFavSnapshotCollector(client, h.service.Store(), nil)
	result, err := collector.Collect(c.Request.Context(), opts)
	if err != nil {
		log.Printf("[cdeu][index] api sync failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "采集失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][index] api sync end usc_total=%d codes=%d rows=%d insert=%d update=%d missing=%d elapsed_ms=%d",
		result.USCTotal, result.CodesRequested, result.RowsReceived,
		result.RowsInsert, result.RowsUpdate, len(result.Missing), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "采集完成",
		"data":    result,
	})
}
