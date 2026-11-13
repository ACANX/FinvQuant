package handler

// 证券指标表（cdeu.finv_quote_secu_indicator，132 列）只读查询（需求 11249/11250）。
//
// 路径（需求 11249 定稿：不再挂在 /Quote/Cdeu/SecuKey 下，指标数据不属于 CDEU 跑批域）：
//
//	GET /API/V1/Meta/Finv/Quote/Secu/Indicator/Columns   列元数据（列名 / 中文名 / 类型）
//	GET /API/V1/Meta/Finv/Quote/Secu/Indicator/Page      分页查询（可变列集）
//
// 与同步接口（/Quote/Cdeu/SecuKey/Indicator/Status|Sync/Run）的关系：
// 同步接口写数据、本文件只读数据，两者共用同一张表，互不影响。
//
// 日志前缀 [indicator][query]，线上排查用
// `docker logs <容器> 2>&1 | grep '\[indicator\]\[query\]'`。

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/indicator"
)

// IndicatorReader 指标表只读查询能力（由 indicator.Reader 实现；接口化便于 Handler 单测）。
type IndicatorReader interface {
	Columns(ctx context.Context) ([]indicator.IndicatorColumn, error)
	Page(ctx context.Context, f indicator.IndicatorPageFilter) (int64, []map[string]string, error)
}

// SecuIndicator 证券指标只读查询处理器。
type SecuIndicator struct {
	reader IndicatorReader
}

// NewSecuIndicator 创建处理器。
func NewSecuIndicator(reader IndicatorReader) *SecuIndicator {
	return &SecuIndicator{reader: reader}
}

// Columns GET /Meta/Finv/Quote/Secu/Indicator/Columns。
//
// 返回指标表的全部列（按建表列序）：列名 + 中文名（表列注释）+ 简化类型。
// 前端据此生成列选择器与列标题，不硬编码 132 列。
func (h *SecuIndicator) Columns(c *gin.Context) {
	started := time.Now()
	columns, err := h.reader.Columns(c.Request.Context())
	if err != nil {
		log.Printf("[indicator][query] api columns failed err=%v client=%s", err, c.ClientIP())
		c.JSON(http.StatusInternalServerError, gin.H{"code": 2006, "message": err.Error()})
		return
	}
	log.Printf("[indicator][query] api columns end columns=%d elapsed_ms=%d",
		len(columns), time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "查询完成",
		"data": gin.H{
			"table":   indicator.IndicatorTable,
			"total":   len(columns),
			"columns": columns,
		},
	})
}

// Page GET /Meta/Finv/Quote/Secu/Indicator/Page。
//
// 查询参数（小驼峰，ApiSpec §3）：keyword / market / columns / orderBy / order / page / pageSize。
// `columns` 为逗号分隔的列名（如 `usc,name,price,pe_ttm`），空则用默认概览列集；
// 不在表内的列名返回 400，不会拼进 SQL。
// 响应 data：{total, page, pageSize, columns[], list[]}，list 内所有值为文本（保住 NUMERIC 精度）。
func (h *SecuIndicator) Page(c *gin.Context) {
	page, err := realtimeStockQueryInt(c, "page", 1, 1, 1_000_000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	pageSize, err := realtimeStockQueryInt(c, "pageSize", indicator.DefaultPageSize, 1, indicator.MaxPageSize)
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

	filter := indicator.IndicatorPageFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Market:  strings.TrimSpace(c.Query("market")),
		Columns: splitIndicatorColumns(c.Query("columns")),
		OrderBy: strings.TrimSpace(c.Query("orderBy")),
		Desc:    desc,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	}

	started := time.Now()
	total, list, err := h.reader.Page(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, indicator.ErrQueryInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[indicator][query] api page failed keyword=%q market=%q orderBy=%q err=%v",
			filter.Keyword, filter.Market, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}

	// 返回列数（含服务端强制补齐的 usc / ftmm_symbol）一并打日志，便于排查「列没回来」的现场。
	returnedColumns := 0
	if len(list) > 0 {
		returnedColumns = len(list[0])
	}
	log.Printf("[indicator][query] api page end page=%d pageSize=%d total=%d returned=%d "+
		"columns=%d orderBy=%q elapsed_ms=%d",
		page, pageSize, total, len(list), returnedColumns, filter.OrderBy,
		time.Since(started).Milliseconds())
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

// splitIndicatorColumns 拆分 `columns=a,b,c`：去空、去重（保留请求顺序）。
func splitIndicatorColumns(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	columns := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		columns = append(columns, name)
	}
	return columns
}
