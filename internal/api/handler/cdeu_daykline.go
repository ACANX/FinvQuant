package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// 本文件实现「日K线数据同步」接口（需求 10952）。
// 路径前缀 /API/V1/Meta/Finv/Quote/Cdeu/DayKline/：
//   POST /Sync     拉取 moomoo 日K 并 upsert 到 cdeu.finv_quote_secu_kline_day
//   GET  /Stat     查询某 usc 的日K存量
//   GET  /UscList  候选 usc（futu 采集表 flag_enable='1'）
// 约定：URL 查询参数小驼峰，JSON 体字段 snake_case（ApiSpec §3）。
// 日志前缀 `[cdeu][daykline]`，线上排查用 `docker logs <容器> 2>&1 | grep '\[cdeu\]\[daykline\]'`。

// DayKlineSync POST /Meta/Finv/Quote/Cdeu/DayKline/Sync。
//
// 请求体：{"usc":"GCMain","from":0,"to":0}（from/to 为 yyyyMMdd，0 表示不限）。
// 响应 data：cdeu.DayKlineSyncResult（total / in_range / insert / update / date_from / date_to 等）。
func (h *ExportArchive) DayKlineSync(c *gin.Context) {
	var req cdeu.DayKlineSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	started := time.Now()
	log.Printf("[cdeu][daykline] api sync begin usc=%q from=%d to=%d client=%s",
		req.USC, req.From, req.To, c.ClientIP())

	result, err := h.service.SyncDayKlineFromMoomoo(c.Request.Context(), req)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrCollectParamsNotFound) {
			status, code = http.StatusNotFound, 404
		}
		log.Printf("[cdeu][daykline] api sync failed usc=%q err=%v elapsed_ms=%d",
			req.USC, err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][daykline] api sync end usc=%q total=%d in_range=%d inserted=%d updated=%d elapsed_ms=%d",
		req.USC, result.Total, result.InRange, result.Inserted, result.Updated,
		time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "同步完成",
		"data":    result,
	})
}

// DayKlineStat GET /Meta/Finv/Quote/Cdeu/DayKline/Stat?usc=GCMain。
func (h *ExportArchive) DayKlineStat(c *gin.Context) {
	usc := strings.TrimSpace(c.Query("usc"))
	if usc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "usc 不能为空"})
		return
	}
	stat, err := h.service.Store().DayKlineStats(c.Request.Context(), usc)
	if err != nil {
		log.Printf("[cdeu][daykline] api stat failed usc=%q err=%v", usc, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询日K存量失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": stat})
}

// DayKlineUscList GET /Meta/Finv/Quote/Cdeu/DayKline/UscList?keyword=GC&limit=200。
func (h *ExportArchive) DayKlineUscList(c *gin.Context) {
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "limit 必须是整数"})
			return
		}
		limit = value
	}
	items, err := h.service.Store().ListCollectUsc(c.Request.Context(), c.Query("keyword"), limit)
	if err != nil {
		log.Printf("[cdeu][daykline] api usc list failed keyword=%q err=%v", c.Query("keyword"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询候选 usc 失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "查询完成", "data": gin.H{"total": len(items), "list": items}})
}
