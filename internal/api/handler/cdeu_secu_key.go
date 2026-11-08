package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
)

// 本文件实现「证券参数同步」三个接口（需求 11042/11043）：
//
//	GET  /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Status   只读快照（20013 是否配置 + 源表名 + 目标表行数）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Config   保存源表名（写系统配置 20014）
//	POST /API/V1/Meta/Finv/Quote/Cdeu/SecuKey/Sync     手动触发同步（同步执行，返回逐批日志）
//
// 数据表：源 = Supabase 的 public.finv_quote_secu_key（表名可配），
// 目标 = 本地 futu.finv_futu_secu_key。
// 约定：JSON 体字段 snake_case、统计字段用动词原形（ApiSpec §1.1/§3）；
// 日志前缀 `[cdeu][secukey]`。API Key 属敏感信息，响应里只出现项目 ID 与脱敏前缀。

// SecuKey 证券参数同步处理器。
type SecuKey struct {
	service *secukey.Service
}

// NewSecuKey 创建处理器。
func NewSecuKey(service *secukey.Service) *SecuKey {
	return &SecuKey{service: service}
}

// secuKeyConfigRequest 保存源表名的请求体。
type secuKeyConfigRequest struct {
	SourceTable string `json:"source_table"`
}

// secuKeySyncRequest 手动同步的请求体（两个字段都可省略）。
type secuKeySyncRequest struct {
	SourceTable string `json:"source_table"`
	BatchSize   int    `json:"batch_size"`
}

// Status GET /Meta/Finv/Quote/Cdeu/SecuKey/Status。
//
// 只读本地：读系统配置 20013/20014 + 统计目标表行数，不访问 Supabase，
// 因此页面打开很快；源表实际行数在同步结果里给出（rows_total）。
func (h *SecuKey) Status(c *gin.Context) {
	snapshot, err := h.service.Status(c.Request.Context())
	if err != nil {
		log.Printf("[cdeu][secukey] api status failed err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "读取同步状态失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api status configured=%v source_table=%s target_rows=%d",
		snapshot.Configured, snapshot.SourceTable, snapshot.TargetRows)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": snapshot})
}

// SaveConfig POST /Meta/Finv/Quote/Cdeu/SecuKey/Config。
//
// 只允许改「源表名」；20013（连接配置）仍在系统配置里维护，不在本页暴露 API Key。
func (h *SecuKey) SaveConfig(c *gin.Context) {
	var req secuKeyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
		return
	}
	table, err := h.service.SetSourceTable(c.Request.Context(), req.SourceTable)
	if err != nil {
		log.Printf("[cdeu][secukey] api save config failed source_table=%q err=%v", req.SourceTable, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "源表名保存失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api save config ok source_table=%s", table)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "源表名已保存", "data": gin.H{"source_table": table}})
}

// Sync POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync。
//
// 同步执行：按 ftmm_symbol 升序分批拉取源表并增量写入目标表，全部批次结束后一次性返回。
// 前端需放宽超时（默认 30s 不够，7 万行量级约 1~3 分钟）。
// 上游失败（网络/鉴权/表不存在）返回 500 + code 2006，message 已是可读原因。
func (h *SecuKey) Sync(c *gin.Context) {
	var req secuKeySyncRequest
	// 允许空请求体：所有字段都有默认值。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体非法: " + err.Error()})
			return
		}
	}
	result, err := h.service.Sync(c.Request.Context(), secukey.SyncRequest{
		SourceTable: req.SourceTable,
		BatchSize:   req.BatchSize,
	})
	if err != nil {
		log.Printf("[cdeu][secukey] api sync failed source_table=%q err=%v", req.SourceTable, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "证券参数同步失败: " + err.Error()})
		return
	}
	log.Printf("[cdeu][secukey] api sync ok source=%s rows_total=%d received=%d insert=%d update=%d unchanged=%d skipped=%d batches=%d elapsed_ms=%d",
		result.SourceTable, result.RowsTotal, result.RowsReceive, result.Insert, result.Update,
		result.Unchanged, result.Skipped, result.Batches, result.ElapsedMs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "证券参数已同步", "data": result})
}
