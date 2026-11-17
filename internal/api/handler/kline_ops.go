package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/acanx/finvquant/internal/kline"
)

// KLineOps 分钟行情 API 的运维端点（/Health、/healthz、/）。
type KLineOps struct {
	Pool    *pgxpool.Pool
	Server  string
	Version string
	Prefix  string
}

// NewKLineOps 创建运维端点处理器。
func NewKLineOps(pool *pgxpool.Pool, server, version, prefix string) *KLineOps {
	return &KLineOps{Pool: pool, Server: server, Version: version, Prefix: prefix}
}

// Health 探活：数据库可达返回 200 {"status":"ok"}，否则 503。
// 兼容 Python 服务的 /healthz 语义（只探库），DC-02。
func (h *KLineOps) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if h.Pool == nil || h.Pool.Ping(ctx) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "degraded",
			"code":       kline.CodeDBUnavailable,
			"message":    "数据服务暂时不可用，请稍后重试",
			"detail":     "数据服务暂时不可用，请稍后重试",
			"request_id": RequestID(c),
			"error": gin.H{
				"code":            kline.CodeDBUnavailable,
				"catalog_version": kline.CatalogVersion,
				"retryable":       true,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Root 服务标识（原 Python 服务的 GET /）。
func (h *KLineOps) Root(c *gin.Context) {
	body := gin.H{
		"service": h.Server,
		"prefix":  h.Prefix,
		"docs":    "/Docs",
	}
	if h.Version != "" {
		body["version"] = h.Version
	}
	c.JSON(http.StatusOK, body)
}
