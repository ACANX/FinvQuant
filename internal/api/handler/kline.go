// Package handler 提供 HTTP 处理器。
package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/kline"
)

// maxKLineBody 请求体上限（32 MiB，远高于单次 5000 行业务上限）。
const maxKLineBody = 32 << 20

// KLine 分钟行情 K 线 API 处理器（融合 FinvQuoteDataDeDup）。
type KLine struct {
	service *kline.Service
	errs    *ErrorResponder
}

// NewKLine 创建 KLine 处理器。
func NewKLine(service *kline.Service, errs *ErrorResponder) *KLine {
	return &KLine{service: service, errs: errs}
}

// readBody 读取请求体（有上限），失败时返回目录错误。
func (h *KLine) readBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxKLineBody+1))
	if err != nil {
		h.errs.Respond(c, kline.Wrap(kline.CodeMalformedJSON, err))
		return nil, false
	}
	if len(body) == 0 {
		h.errs.Respond(c, kline.New(kline.CodeMalformedJSON))
		return nil, false
	}
	if len(body) > maxKLineBody {
		h.errs.Respond(c, kline.New(kline.CodeBatchTooLarge).WithDebug("limit", strconv.Itoa(maxKLineBody)))
		return nil, false
	}
	return body, true
}

// queryString 读取首个非空查询参数（兼容旧下划线名与新小驼峰名）。
func queryString(c *gin.Context, names ...string) *string {
	for _, name := range names {
		if value, ok := c.GetQuery(name); ok && value != "" {
			text := value
			return &text
		}
	}
	return nil
}

// queryInt 读取整数查询参数；空值返回 nil，非法值返回目录错误。
func queryInt(c *gin.Context, field string, names ...string) (*int, *kline.Error) {
	raw := queryString(c, names...)
	if raw == nil {
		return nil, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(*raw))
	if err != nil {
		return nil, kline.New(kline.CodeTypeMismatch).WithDebug("field", field).
			WithDebug("want", "integer").WithDebug("got", *raw)
	}
	return &value, nil
}

// queryInt64 读取 int64 查询参数。
func queryInt64(c *gin.Context, field string, names ...string) (*int64, *kline.Error) {
	raw := queryString(c, names...)
	if raw == nil {
		return nil, nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(*raw), 10, 64)
	if err != nil {
		return nil, kline.New(kline.CodeTypeMismatch).WithDebug("field", field).
			WithDebug("want", "integer").WithDebug("got", *raw)
	}
	return &value, nil
}

// listFilter 解析 /Page 与 /PageUni 的过滤参数。
func listFilter(c *gin.Context) (kline.ListFilter, *kline.Error) {
	var filter kline.ListFilter
	filter.Usc = queryString(c, "usc")
	filter.Provider = queryString(c, "provider")
	var err *kline.Error
	if filter.DateFrom, err = queryInt(c, "dateFrom", "dateFrom", "date_from"); err != nil {
		return filter, err
	}
	if filter.DateTo, err = queryInt(c, "dateTo", "dateTo", "date_to"); err != nil {
		return filter, err
	}
	if filter.TsFrom, err = queryInt64(c, "tsFrom", "tsFrom", "ts_from"); err != nil {
		return filter, err
	}
	if filter.TsTo, err = queryInt64(c, "tsTo", "tsTo", "ts_to"); err != nil {
		return filter, err
	}
	for _, pair := range []struct {
		name  string
		value *int
	}{{"dateFrom", filter.DateFrom}, {"dateTo", filter.DateTo}} {
		if pair.value != nil && (*pair.value < 19500101 || *pair.value > 21001231) {
			return filter, kline.New(kline.CodeOutOfRange).
				WithDebug("field", pair.name).WithDebug("min", "19500101").WithDebug("max", "21001231")
		}
	}
	for _, pair := range []struct {
		name  string
		value *int64
	}{{"tsFrom", filter.TsFrom}, {"tsTo", filter.TsTo}} {
		if pair.value != nil && *pair.value < 0 {
			return filter, kline.New(kline.CodeOutOfRange).WithDebug("field", pair.name).WithDebug("min", "0")
		}
	}
	return filter, nil
}

// paging 解析 limit/offset（默认 50/0）。
func paging(c *gin.Context) (int, int, *kline.Error) {
	limit, err := queryInt(c, "limit", "limit", "pageSize", "page_size")
	if err != nil {
		return 0, 0, err
	}
	offset, err := queryInt(c, "offset", "offset")
	if err != nil {
		return 0, 0, err
	}
	pageSize, skip := 50, 0
	if limit != nil {
		pageSize = *limit
	}
	if offset != nil {
		skip = *offset
	}
	return pageSize, skip, nil
}

// Add 处理 POST .../Add：单条插入（201）。
func (h *KLine) Add(c *gin.Context) {
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var item kline.ItemIn
	if err := kline.DecodeJSON(body, &item); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	row, err := h.service.Add(c.Request.Context(), item)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, row)
}

// BatchInsert 处理 POST .../Batch：批量插入（201）。
func (h *KLine) BatchInsert(c *gin.Context) {
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var in kline.BatchIn
	if err := kline.DecodeJSON(body, &in); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	inserted, err := h.service.BatchInsert(c.Request.Context(), in.Items)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"inserted": inserted})
}

// Upsert 处理 POST .../Upsert?onConflict=skip|update。
func (h *KLine) Upsert(c *gin.Context) {
	mode := "update"
	if raw := queryString(c, "onConflict", "onConflict", "on_conflict"); raw != nil {
		mode = strings.ToLower(strings.TrimSpace(*raw))
	}
	if mode != "skip" && mode != "update" {
		h.errs.Respond(c, kline.New(kline.CodeInvalidEnum).
			WithDebug("field", "onConflict").WithDebug("allowed", "skip,update"))
		return
	}
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var in kline.BatchIn
	if err := kline.DecodeJSON(body, &in); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	affected, err := h.service.Upsert(c.Request.Context(), in.Items, mode)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"affected": affected, "mode": mode})
}

// Page 处理 GET .../Page：条件分页（对象数组 + 紧凑字段）。
func (h *KLine) Page(c *gin.Context) {
	filter, perr := listFilter(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	limit, offset, perr := paging(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	cnt, rows, err := h.service.Page(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	list := make([]kline.ListItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, row.ListItem())
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": gin.H{
			"cnt":        cnt,
			"ps":         limit,
			"pn":         kline.PageNumber(offset, limit),
			"list":       list,
			"request_id": RequestID(c),
			"ts":         time.Now().Unix(),
		},
		"message": "OK",
	})
}

// PageUni 处理 GET .../PageUni：条件分页（column/row 行数组）。
func (h *KLine) PageUni(c *gin.Context) {
	filter, perr := listFilter(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	limit, offset, perr := paging(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	cnt, rows, err := h.service.Page(c.Request.Context(), filter, limit, offset)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	uniRows := make([][]*string, 0, len(rows))
	for _, row := range rows {
		uniRows = append(uniRows, row.UniRow())
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": gin.H{
			"cnt":        cnt,
			"ps":         limit,
			"pn":         kline.PageNumber(offset, limit),
			"column":     kline.CompactFields(),
			"row":        uniRows,
			"request_id": RequestID(c),
			"ts":         time.Now().Unix(),
		},
		"message": "OK",
	})
}

// Query 处理 POST .../Query：按 (usc, ts) / uscs / provider 批量查询。
func (h *KLine) Query(c *gin.Context) {
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var in kline.QueryIn
	if err := kline.DecodeJSON(body, &in); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	spec := kline.QuerySpec{
		Uscs:     in.Uscs,
		Provider: in.Provider,
		Limit:    50,
		Offset:   0,
	}
	if in.Limit != nil {
		spec.Limit = *in.Limit
	}
	if in.Offset != nil {
		spec.Offset = *in.Offset
	}
	for _, pair := range in.Keys {
		if pair.Usc == "" {
			h.errs.Respond(c, kline.New(kline.CodeMissingParam).WithDebug("field", "keys.usc"))
			return
		}
		ts, tsErr := h.service.NormalizeTS(pair.Ts)
		if tsErr != nil {
			h.errs.Respond(c, tsErr)
			return
		}
		spec.Keys = append(spec.Keys, kline.Key{Usc: pair.Usc, Ts: ts})
	}
	count, rows, err := h.service.Query(c.Request.Context(), spec)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	if rows == nil {
		rows = []kline.Row{}
	}
	c.JSON(http.StatusOK, gin.H{"count": count, "items": rows})
}

// BatchPatch 处理 PATCH .../Batch：批量增量更新（请求体为数组）。
func (h *KLine) BatchPatch(c *gin.Context) {
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var items []kline.ItemIn
	if err := kline.DecodeJSON(body, &items); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	updated, err := h.service.PatchBatch(c.Request.Context(), items)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

// BatchDelete 处理 DELETE .../Batch：按 keys / uscs 批量删除（OR 语义）。
func (h *KLine) BatchDelete(c *gin.Context) {
	body, ok := h.readBody(c)
	if !ok {
		return
	}
	var in kline.DeleteIn
	if err := kline.DecodeJSON(body, &in); err != nil {
		h.errs.Respond(c, kline.MapDecodeError(err))
		return
	}
	keys := make([]kline.Key, 0, len(in.Keys))
	for _, pair := range in.Keys {
		if pair.Usc == "" {
			h.errs.Respond(c, kline.New(kline.CodeMissingParam).WithDebug("field", "keys.usc"))
			return
		}
		keys = append(keys, kline.Key{Usc: pair.Usc, Ts: pair.Ts})
	}
	deleted, err := h.service.DeleteBatch(c.Request.Context(), keys, in.Uscs)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// Bounds 处理 GET .../Bounds：返回过滤范围内的交易日期边界（轻量，供前端默认日期范围使用）。
func (h *KLine) Bounds(c *gin.Context) {
	filter, perr := listFilter(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	bounds, err := h.service.Bounds(c.Request.Context(), filter)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": gin.H{
			"date_from":  bounds.DateFrom,
			"date_to":    bounds.DateTo,
			"request_id": RequestID(c),
			"ts":         time.Now().Unix(),
		},
		"message": "OK",
	})
}

// Stats 处理 GET .../Stats：按 usc/日期范围做汇总统计与可视化数据。
func (h *KLine) Stats(c *gin.Context) {
	filter, perr := listFilter(c)
	if perr != nil {
		h.errs.Respond(c, perr)
		return
	}
	result, err := h.service.Stats(c.Request.Context(), filter)
	if err != nil {
		h.errs.Respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 1,
		"data": gin.H{
			"summary":    result.Summary,
			"daily":      result.Daily,
			"top_usc":    result.TopUsc,
			"request_id": RequestID(c),
			"ts":         time.Now().Unix(),
		},
		"message": "OK",
	})
}

// Count 处理 GET .../Count：统计总记录数。
func (h *KLine) Count(c *gin.Context) {
	var filter kline.CountFilter
	filter.Usc = queryString(c, "usc")
	filter.Provider = queryString(c, "provider")
	var err *kline.Error
	if filter.DateFrom, err = queryInt(c, "dateFrom", "dateFrom", "date_from"); err != nil {
		h.errs.Respond(c, err)
		return
	}
	if filter.DateTo, err = queryInt(c, "dateTo", "dateTo", "date_to"); err != nil {
		h.errs.Respond(c, err)
		return
	}
	for _, pair := range []struct {
		name  string
		value *int
	}{{"dateFrom", filter.DateFrom}, {"dateTo", filter.DateTo}} {
		if pair.value != nil && (*pair.value < 19500101 || *pair.value > 21001231) {
			h.errs.Respond(c, kline.New(kline.CodeOutOfRange).
				WithDebug("field", pair.name).WithDebug("min", "19500101").WithDebug("max", "21001231"))
			return
		}
	}
	total, svcErr := h.service.Count(c.Request.Context(), filter)
	if svcErr != nil {
		h.errs.Respond(c, svcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total})
}
