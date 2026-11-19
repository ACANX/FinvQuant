package handler

// 「五日分钟行情采集配置」页的接口（需求 11344）。
//
//	GET /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/Page          分页查询 Supabase public.finv_quote_secu_ft
//	GET /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/FilterOptions 下拉候选（表内 DISTINCT）
//
// 数据不在本地库：经 PostgREST 只读访问 Supabase（连接取 sys_config 20013）。
// 页面只读——本表是采集链路的配置源，改动走 Supabase 侧，本服务不提供写接口。

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secuft"
)

// QuoteCollectFiveDayFt 五日分钟行情采集配置查询接口。
type QuoteCollectFiveDayFt struct {
	store *secuft.Store
}

// NewQuoteCollectFiveDayFt 创建接口处理器。
func NewQuoteCollectFiveDayFt(store *secuft.Store) *QuoteCollectFiveDayFt {
	return &QuoteCollectFiveDayFt{store: store}
}

// Page GET /Meta/Finv/Quote/Collect/FiveDayFt/Page。
//
// 查询参数：
//
//	page        页码（默认 1）
//	pageSize    每页条数（默认 15，1–200）
//	keyword     模糊匹配 usc / symbol / futu_symbol / name
//	marketLabel 行情市场精确匹配（空或 all 不限）
//	instrument  品种精确匹配
//	typeSecu    证券分类精确匹配
//	provider    提供方精确匹配
//	flagEnable  采集开关 1 / 0
//	sortBy      排序列（白名单内，默认 usc）
//	sortOrder   asc / desc（默认 asc）
func (h *QuoteCollectFiveDayFt) Page(c *gin.Context) {
	query := secuft.Query{
		Page:        parseIntDefault(c.Query("page"), 1),
		PageSize:    parseIntDefault(c.Query("pageSize"), secuft.DefaultPageSize),
		Keyword:     c.Query("keyword"),
		FlagEnable:  c.Query("flagEnable"),
		SortBy:      c.Query("sortBy"),
		SortDesc:    strings.EqualFold(strings.TrimSpace(c.Query("sortOrder")), "desc"),
		TextFilters: map[string]string{},
	}
	for _, param := range []string{"marketLabel", "instrument", "typeSecu", "provider"} {
		query.TextFilters[param] = c.Query(param)
	}
	started := time.Now()
	result, err := h.store.Page(c.Request.Context(), query)
	query.Normalize()
	secuft.LogQuery(query, result, err)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    2007,
			"message": "查询 Supabase 源表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"list":       result.Rows,
			"total":      result.Total,
			"page":       query.Page,
			"pageSize":   query.PageSize,
			"columns":    secuft.Columns(),
			"table":      secuft.TableName,
			"elapsed_ms": time.Since(started).Milliseconds(),
		},
	})
}

// FilterOptions GET /Meta/Finv/Quote/Collect/FiveDayFt/FilterOptions。
//
// 返回 `{"options": {"marketLabel": [...], "instrument": [...], ...}}`；
// 表为空时各维度为空数组（页面下拉只有「全部」）。
func (h *QuoteCollectFiveDayFt) FilterOptions(c *gin.Context) {
	started := time.Now()
	options, err := h.store.FilterOptions(c.Request.Context())
	if err != nil {
		secuft.LogQuery(secuft.Query{}, nil, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    2007,
			"message": "读取 Supabase 源表下拉候选失败: " + err.Error(),
		})
		return
	}
	secuft.LogQuery(secuft.Query{}, &secuft.Result{Total: int64(len(options))}, nil)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data": gin.H{
			"options":    options,
			"elapsed_ms": time.Since(started).Milliseconds(),
		},
	})
}
