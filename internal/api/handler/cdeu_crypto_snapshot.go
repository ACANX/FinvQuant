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
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// CryptoFavSnapshotSync POST /Meta/Finv/Quote/Crypto/Fav/Snapshot/Sync。
//
// 请求体可选：{"timeout_ms": 60000}；单批 code 数固定 400（moomoo 接口上限）。
func (h *ExportArchive) CryptoFavSnapshotSync(c *gin.Context) {
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
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    2006,
			"message": "未配置 moomoo OpenAPI 凭据（sys_config 20016/20017 或环境变量 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY）",
		})
		return
	}

	opts := cdeu.CryptoFavSnapshotCollectOptions{}
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

// CryptoMoomooConfigGet GET /Meta/Finv/Quote/Crypto/Moomoo/Config。
//
// 回显 moomoo OpenAPI 凭据的配置状态（需求 11318）：**只返回 AppKey 与私钥掩码**，
// 不下发私钥明文；`source` 说明当前生效来源（sys_config / env / none）。
func (h *ExportArchive) CryptoMoomooConfigGet(c *gin.Context) {
	cfg, err := h.service.Store().LoadMoomooConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}
	source := "none"
	switch {
	case cfg.Configured():
		source = "sys_config"
	case strings.TrimSpace(os.Getenv("FINV_MOOMOO_APP_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("FINV_MOOMOO_PRIVATE_KEY")) != "":
		source = "env"
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"app_key":                cfg.AppKey,
			"private_key_configured": strings.TrimSpace(cfg.PrivateKeyPEM) != "",
			"private_key_masked":     cfg.MaskedPrivateKey(),
			"source":                 source,
		},
	})
}

// CryptoMoomooConfigSave POST /Meta/Finv/Quote/Crypto/Moomoo/Config。
//
// 请求体：{"app_key": "...", "private_key": "..."}。
// `private_key` 留空 = 保留原值（页面不回显明文）；传 `__CLEAR__` 清空私钥。
// 私钥会先做 Ed25519 PKCS#8 解析校验，非法 PEM 直接 400，不写系统配置。
func (h *ExportArchive) CryptoMoomooConfigSave(c *gin.Context) {
	var req struct {
		AppKey     string `json:"app_key"`
		PrivateKey string `json:"private_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := h.service.Store().SaveMoomooConfig(c.Request.Context(), req.AppKey, req.PrivateKey); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "AppKey") || strings.Contains(err.Error(), "私钥") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"code": 400, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][crypto] moomoo 凭据已更新 app_key=%.6s***", req.AppKey)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存"})
}
