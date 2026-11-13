package handler

// 采集参数（futu.finv_futu_quote_collect）查询与单行维护（需求 11254）。
//
// 路径（与证券指标同域，都不带 Cdeu 之外的跑批前缀）：
//
//	GET  /API/V1/Meta/Finv/Quote/Secu/Collect/Page    分页查询（左关联 public.finv_security 等）
//	POST /API/V1/Meta/Finv/Quote/Secu/Collect/Update  单行更新（字段白名单）
//
// 页面：行情 → 收剔导传ETL → CDEU自动化 → 采集参数。
//
// 日志前缀 [cdeu][secuftmmcollect]，线上排查用
// `docker logs <容器> 2>&1 | grep '\[cdeu\]\[secuftmmcollect\]'`。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/acanx/finvquant/internal/meta/finv/quote/cdeu"
)

// secuFtmmCollectMaxBodyBytes 单行更新的请求体上限：11 个字段，1 MB 足够。
const secuFtmmCollectMaxBodyBytes = 1 << 20

// SecuFtmmCollectStore 采集参数查询 / 更新能力（由 cdeu.Store 实现；接口化便于 Handler 单测）。
type SecuFtmmCollectStore interface {
	ListSecuFtmmCollect(ctx context.Context, f cdeu.SecuFtmmCollectFilter) (int64, []cdeu.SecuFtmmCollectRow, error)
	UpdateSecuFtmmCollect(ctx context.Context, patch cdeu.SecuFtmmCollectPatch) (bool, error)
}

// SecuFtmmCollect 采集参数查询与维护处理器。
type SecuFtmmCollect struct {
	store SecuFtmmCollectStore
}

// NewSecuFtmmCollect 创建处理器。
func NewSecuFtmmCollect(store SecuFtmmCollectStore) *SecuFtmmCollect {
	return &SecuFtmmCollect{store: store}
}

// Page GET /Meta/Finv/Quote/Secu/Collect/Page。
//
// 查询参数（小驼峰，ApiSpec §3）：keyword / marketLabel / instrument / securityType /
// flagEnable / onlyMissing / orderBy / order / page / pageSize。
// 响应 data：{total, page, pageSize, list[]}；list 行 = 采集参数字段 + 左关联的证券元数据
// （security_name / security_type / security_sub_type / currency_type / exchange_name /
// security_market_name），主数据里没有该证券时 security_exists=false 且元数据为空串。
func (h *SecuFtmmCollect) Page(c *gin.Context) {
	page, err := realtimeStockQueryInt(c, "page", 1, 1, 1_000_000)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	pageSize, err := realtimeStockQueryInt(c, "pageSize", 50, 1, 500)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	desc := false
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "", "asc":
		desc = false
	case "desc":
		desc = true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "order 只能为 asc 或 desc"})
		return
	}

	filter := cdeu.SecuFtmmCollectFilter{
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		MarketLabel:  strings.TrimSpace(c.Query("marketLabel")),
		Instrument:   strings.TrimSpace(c.Query("instrument")),
		SecurityType: strings.TrimSpace(c.Query("securityType")),
		FlagEnable:   strings.TrimSpace(c.Query("flagEnable")),
		USCMissing:   isTruthyQuery(c.Query("onlyMissing")),
		OrderBy:      strings.TrimSpace(c.Query("orderBy")),
		Desc:         desc,
		Limit:        pageSize,
		Offset:       (page - 1) * pageSize,
	}

	started := time.Now()
	total, list, err := h.store.ListSecuFtmmCollect(c.Request.Context(), filter)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		if errors.Is(err, cdeu.ErrSecuFtmmCollectInvalid) {
			status, code = http.StatusBadRequest, 400
		}
		log.Printf("[cdeu][secuftmmcollect] api page failed keyword=%q market=%q orderBy=%q err=%v",
			filter.Keyword, filter.MarketLabel, filter.OrderBy, err)
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}
	log.Printf("[cdeu][secuftmmcollect] api page end page=%d pageSize=%d total=%d returned=%d elapsed_ms=%d",
		page, pageSize, total, len(list), time.Since(started).Milliseconds())
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

// secuFtmmCollectUpdateFields 可更新的字段（指针为 nil 表示不改该列）。
//
// 白名单与 cdeu.SecuFtmmCollectPatch 同源：usc 是定位键、ftmm_symbol 是生成列、
// gmt_create / gmt_update 由触发器维护，都不在这里出现。
type secuFtmmCollectUpdateFields struct {
	StockID           *int64  `json:"stock_id"`
	MarketType        *int    `json:"market_type"`
	MarketCode        *int    `json:"market_code"`
	InstrumentType    *int    `json:"instrument_type"`
	SubInstrumentType *int    `json:"sub_instrument_type"`
	MarketLabel       *string `json:"market_label"`
	Instrument        *string `json:"instrument"`
	FutuSymbol        *string `json:"futu_symbol"`
	ReqSection        *string `json:"req_section"`
	FlagEnable        *string `json:"flag_enable"`
	Remark            *string `json:"remark"`
}

// secuFtmmCollectUpdateRequest 单行更新请求体。
type secuFtmmCollectUpdateRequest struct {
	USC    string                      `json:"usc"`
	Fields secuFtmmCollectUpdateFields `json:"fields"`
}

// Update POST /Meta/Finv/Quote/Secu/Collect/Update。
//
// 请求体：`{"usc":"00700","fields":{...}}`，fields 里只出现要改的字段（未出现 = 不改）。
// 成功 code=0；入参非法 400；`usc` 不存在 404；`(market_code, futu_symbol)` 撞到别的 usc 409。
func (h *SecuFtmmCollect) Update(c *gin.Context) {
	started := time.Now()

	body := http.MaxBytesReader(c.Writer, c.Request.Body, secuFtmmCollectMaxBodyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var req secuFtmmCollectUpdateRequest
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败: " + err.Error()})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体只能包含一个 JSON 对象"})
		return
	}

	patch := cdeu.SecuFtmmCollectPatch{
		USC:               strings.TrimSpace(req.USC),
		StockID:           req.Fields.StockID,
		MarketType:        req.Fields.MarketType,
		MarketCode:        req.Fields.MarketCode,
		InstrumentType:    req.Fields.InstrumentType,
		SubInstrumentType: req.Fields.SubInstrumentType,
		MarketLabel:       req.Fields.MarketLabel,
		Instrument:        req.Fields.Instrument,
		FutuSymbol:        req.Fields.FutuSymbol,
		ReqSection:        req.Fields.ReqSection,
		FlagEnable:        req.Fields.FlagEnable,
		Remark:            req.Fields.Remark,
	}

	updated, err := h.store.UpdateSecuFtmmCollect(c.Request.Context(), patch)
	if err != nil {
		status, code := http.StatusInternalServerError, 2006
		switch {
		case errors.Is(err, cdeu.ErrSecuFtmmCollectInvalid):
			status, code = http.StatusBadRequest, 400
		case errors.Is(err, cdeu.ErrSecuFtmmCollectNotFound):
			status, code = http.StatusNotFound, 404
		case errors.Is(err, cdeu.ErrSecuFtmmCollectConflict):
			status, code = http.StatusConflict, 409
		}
		log.Printf("[cdeu][secuftmmcollect] api update failed usc=%q status=%d err=%v client=%s",
			patch.USC, status, err, c.ClientIP())
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}

	log.Printf("[cdeu][secuftmmcollect] api update end usc=%q updated=%t elapsed_ms=%d",
		patch.USC, updated, time.Since(started).Milliseconds())
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "保存完成",
		"data":    gin.H{"usc": patch.USC, "updated": updated},
	})
}

// isTruthyQuery 解析布尔型查询参数：1 / true / yes / on 视为真。
func isTruthyQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
