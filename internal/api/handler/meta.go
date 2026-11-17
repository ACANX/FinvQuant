// Package handler 提供 HTTP 处理器。
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta"
)

// Meta 元数据管理处理器（finv_exchange / finv_market / finv_security 字典维护）。
type Meta struct {
	service *meta.Service
}

// NewMeta 创建元数据管理处理器。
func NewMeta(service *meta.Service) *Meta {
	return &Meta{service: service}
}

// ---------------------------------------------------------------------
// finv_exchange：GET /API/V1/Meta/FinvQuant/Metadata/Exchange/List
// ---------------------------------------------------------------------

// ListExchanges 分页查询交易所字典。
func (h *Meta) ListExchanges(c *gin.Context) {
	keyword := c.Query("keyword")
	flagEnable := c.Query("flag_enable")
	list, total, err := h.service.ListExchanges(c.Request.Context(), parsePager(c), keyword, flagEnable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询交易所字典失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "list": list},
	})
}

// SaveExchange 新增或更新交易所（POST /API/V1/Meta/FinvQuant/Metadata/Exchange/Save）。
func (h *Meta) SaveExchange(c *gin.Context) {
	var e meta.Exchange
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	code, err := h.service.SaveExchange(c.Request.Context(), e)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存交易所失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"exchange_code": code}})
}

// ToggleExchange 切换交易所启用状态（POST /API/V1/Meta/FinvQuant/Metadata/Exchange/Toggle）。
func (h *Meta) ToggleExchange(c *gin.Context) {
	var req struct {
		ExchangeCode int    `json:"exchange_code"`
		FlagEnable   string `json:"flag_enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	if err := h.service.ToggleExchange(c.Request.Context(), req.ExchangeCode, req.FlagEnable); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "切换交易所状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "切换成功", "data": gin.H{"exchange_code": req.ExchangeCode, "flag_enable": req.FlagEnable}})
}

// ---------------------------------------------------------------------
// finv_market：GET /API/V1/Meta/FinvQuant/Metadata/Market/List
// ---------------------------------------------------------------------

// ListMarkets 分页查询交易市场。
func (h *Meta) ListMarkets(c *gin.Context) {
	keyword := c.Query("keyword")
	flagEnable := c.Query("flag_enable")
	list, total, err := h.service.ListMarkets(c.Request.Context(), parsePager(c), keyword, flagEnable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询交易市场失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "list": list},
	})
}

// SaveMarket 新增或更新交易市场（POST /API/V1/Meta/FinvQuant/Metadata/Market/Save）。
func (h *Meta) SaveMarket(c *gin.Context) {
	var m meta.Market
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	code, err := h.service.SaveMarket(c.Request.Context(), m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存市场失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"market_code": code}})
}

// ToggleMarket 切换市场启用状态（POST /API/V1/Meta/FinvQuant/Metadata/Market/Toggle）。
func (h *Meta) ToggleMarket(c *gin.Context) {
	var req struct {
		MarketCode int    `json:"market_code"`
		FlagEnable string `json:"flag_enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	if err := h.service.ToggleMarket(c.Request.Context(), req.MarketCode, req.FlagEnable); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "切换市场状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "切换成功", "data": gin.H{"market_code": req.MarketCode, "flag_enable": req.FlagEnable}})
}

// ---------------------------------------------------------------------
// finv_security：GET /API/V1/Meta/FinvQuant/Metadata/Security/List
// ---------------------------------------------------------------------

// ListSecurities 分页查询证券代码。
func (h *Meta) ListSecurities(c *gin.Context) {
	keyword := c.Query("keyword")
	flagEnable := c.Query("flag_enable")
	list, total, err := h.service.ListSecurities(c.Request.Context(), parsePager(c), keyword, flagEnable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询证券代码失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": total, "list": list},
	})
}

// SaveSecurity 新增或更新证券代码（POST /API/V1/Meta/FinvQuant/Metadata/Security/Save）。
func (h *Meta) SaveSecurity(c *gin.Context) {
	var sec meta.Security
	if err := c.ShouldBindJSON(&sec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	usc, err := h.service.SaveSecurity(c.Request.Context(), sec)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "保存证券失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "保存成功", "data": gin.H{"usc": usc}})
}

// ToggleSecurity 切换证券启用状态（POST /API/V1/Meta/FinvQuant/Metadata/Security/Toggle）。
func (h *Meta) ToggleSecurity(c *gin.Context) {
	var req struct {
		USC        string `json:"usc"`
		FlagEnable string `json:"flag_enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 4001, "message": "请求体格式错误: " + err.Error()})
		return
	}
	if err := h.service.ToggleSecurity(c.Request.Context(), req.USC, req.FlagEnable); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "切换证券状态失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "切换成功", "data": gin.H{"usc": req.USC, "flag_enable": req.FlagEnable}})
}

// SecurityOptions 返回证券下拉选项（GET /API/V1/Meta/FinvQuant/Metadata/Security/Options）。
func (h *Meta) SecurityOptions(c *gin.Context) {
	list, err := h.service.SecurityOptions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询证券选项失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"total": len(list), "list": list},
	})
}

// LookupSecurity 按代码查询证券详情（GET /API/V1/Meta/FinvQuant/Metadata/Security/Lookup?code=xxx）。
// 支持 usc 或源证券代码（security_code）精确匹配；供历史行情导入双策略使用。
func (h *Meta) LookupSecurity(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		code = c.Query("usc") // 兼容旧参数名
	}
	sec, err := h.service.LookupSecurity(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": "查询证券详情失败: " + err.Error()})
		return
	}
	if sec == nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "证券不存在", "data": gin.H{"found": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data":    gin.H{"found": true, "security": sec},
	})
}

// parsePager 从 Query 参数解析分页参数（page / page_size）。
func parsePager(c *gin.Context) meta.Pager {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return meta.Pager{Page: page, PageSize: pageSize}
}
