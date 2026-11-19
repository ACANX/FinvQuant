package handler

// 「五日分钟行情采集配置」页的接口（需求 11344）。
//
//	GET  /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/Page          分页查询 Supabase public.finv_quote_secu_ft
//	GET  /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/FilterOptions 下拉候选（表内 DISTINCT）
//	GET  /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/Lookup        新增对话框的元数据补全（需求 11345）
//	POST /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/Create        新增一行采集配置（需求 11345）
//
// 数据不在本地库：经 PostgREST 访问 Supabase（连接取 sys_config 20013）。
// 除「新增」外不提供其它写接口：本表是采集链路的配置源，改/删走 Supabase 侧。
// 新增的采集开关默认 0（先停采），由人工确认后再改成 1。

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secuft"
	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
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

// Lookup GET /Meta/Finv/Quote/Collect/FiveDayFt/Lookup?usc=00700（需求 11345）。
//
// 按 usc 联查本地 4 张表（futu.finv_futu_quote_collect / futu.finv_futu_secu_key /
// public.finv_security / cdeu.finv_quote_secu_indicator）补全 21 列，并顺带查一次
// Supabase 判断该 usc 是否已存在（`exists_in_supabase`，供前端提前拦截）。
func (h *QuoteCollectFiveDayFt) Lookup(c *gin.Context) {
	usc := strings.TrimSpace(c.Query("usc"))
	if usc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "usc 不能为空"})
		return
	}
	started := time.Now()
	result, err := h.store.Lookup(c.Request.Context(), usc)
	if err != nil {
		status, code := http.StatusBadRequest, 400
		var inputErr *secuft.InputError
		if !errors.As(err, &inputErr) {
			status, code = http.StatusInternalServerError, 2006
		}
		log.Printf("[secuft] api lookup failed usc=%s err=%v", usc, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	// Supabase 查重失败不阻断补全（网络抖动时仍让用户看到本地补全结果）。
	exists, existsErr := h.store.Exists(c.Request.Context(), usc)
	if existsErr != nil {
		log.Printf("[secuft] api lookup exists-check failed usc=%s err=%v", usc, existsErr)
		result.Notes = append(result.Notes, "未能确认 Supabase 里是否已存在该 usc（"+existsErr.Error()+"）")
	} else {
		result.ExistsInSupabase = exists
		if exists {
			result.Notes = append(result.Notes, "Supabase 采集配置表里已存在该 usc，请勿重复新增")
		}
	}
	log.Printf("[secuft] api lookup ok usc=%s in_security=%t missing=%d elapsed_ms=%d",
		usc, result.InSecurity, len(result.Missing), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "OK", "data": result})
}

// Create POST /Meta/Finv/Quote/Collect/FiveDayFt/Create（需求 11345）。
//
// 请求体：
//
//	{"usc":"00700","fields":{"symbol":"HK.00700","futu_symbol":"00700", ...}}
//
// fields 里未提供的列用服务端默认值：`flag_enable`=0（先停采）、`provider`="FTMM"、
// `req_section`=0、三列时间列=now()。重复 usc 返回 400（上游 409 被翻成中文提示）。
func (h *QuoteCollectFiveDayFt) Create(c *gin.Context) {
	var req secuft.CreateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	started := time.Now()
	result, err := h.store.Create(c.Request.Context(), req)
	if err != nil {
		status, code := http.StatusBadGateway, 2007
		var inputErr *secuft.InputError
		var permErr *secukey.PermissionError
		switch {
		case errors.As(err, &inputErr):
			status, code = http.StatusBadRequest, 400
		case errors.As(err, &permErr):
			// RLS / GRANT 未开放 insert：属配置问题，回 403 让前端提示去 Supabase 加策略。
			status, code = http.StatusForbidden, 403
		}
		log.Printf("[secuft] api create failed usc=%s err=%v elapsed_ms=%d",
			strings.TrimSpace(req.Usc), err, time.Since(started).Milliseconds())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[secuft] api create ok usc=%s elapsed_ms=%d", result.Usc, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "新增成功（采集开关默认关闭，需手动开启）", "data": result})
}
