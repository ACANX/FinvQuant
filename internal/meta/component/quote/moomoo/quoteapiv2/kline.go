package quoteapiv2

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// GetKLineRequest K 线请求参数（对应 get-kline 接口）。
//
// 五个定位参数（StockID / MarketType / MarketCode / InstrumentType /
// SubInstrumentType）原样透传，不做语义换算：同一编号在不同接口含义不同，
// 换算会引入难以排查的错位。
type GetKLineRequest struct {
	// StockID 标的编号，必填。
	StockID string
	// MarketType 市场类型。
	MarketType string
	// KLineType 周期类型，为空取 KLineTypeDay。
	KLineType string
	// MarketCode 市场代码。
	MarketCode string
	// InstrumentType 标的类型。
	InstrumentType string
	// SubInstrumentType 标的子类型。
	SubInstrumentType string
	// TimestampMillis 毫秒时间戳（查询串 _ 参数），为空自动取当前时间。
	TimestampMillis string
}

// KLineBar 一根 K 线，保持接口原始口径：时间为秒级 UNIX 时间戳，价格保留字符串
// 以免浮点精度损失，数值仅在明确是数字时使用 float64 承载。由 GetKLine 返回。
type KLineBar struct {
	// Time 秒级 UNIX 时间戳。
	Time int64
	// Open 开盘价原始字符串，可能为空。
	Open string
	// High 最高价原始字符串，可能为空。
	High string
	// Low 最低价原始字符串，可能为空。
	Low string
	// Close 收盘价原始字符串，可能为空。
	Close string
	// Volume 成交量。
	Volume float64
	// Turnover 成交额。
	Turnover float64
	// ChangeRatio 涨跌幅，接口可能返回 null，故用指针区分「无值」与「0」。
	ChangeRatio *float64
	// LastClose 昨收价。
	LastClose float64
	// ChangePrice 涨跌额原始字符串，含符号。
	ChangePrice string
}

// GetKLine 拉取全量 K 线序列（对应 get-kline 接口）。
//
// 该接口无分页与时间范围参数，一次返回全部历史数据；返回值按时间升序（最早在前），
// 与接口顺序一致，区间过滤由调用方完成。
func (c *Client) GetKLine(ctx context.Context, req GetKLineRequest) ([]KLineBar, error) {
	if strings.TrimSpace(req.StockID) == "" {
		return nil, errors.New("moomoo GetKLine：stockId 不能为空")
	}
	klineType := strings.TrimSpace(req.KLineType)
	if klineType == "" {
		klineType = KLineTypeDay
	}
	timestamp := strings.TrimSpace(req.TimestampMillis)
	if timestamp == "" {
		timestamp = strconv.FormatInt(c.cfg.Now().UnixMilli(), 10)
	}
	// 参数顺序即签名键序，不可随意调整。
	params := []Param{
		{Name: "stockId", Value: strings.TrimSpace(req.StockID)},
		{Name: "marketType", Value: strings.TrimSpace(req.MarketType)},
		{Name: "type", Value: klineType},
		{Name: "marketCode", Value: strings.TrimSpace(req.MarketCode)},
		{Name: "instrumentType", Value: strings.TrimSpace(req.InstrumentType)},
		{Name: "subInstrumentType", Value: strings.TrimSpace(req.SubInstrumentType)},
		{Name: "_", Value: timestamp},
	}

	path := c.cfg.KLinePath
	body, err := c.get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var payload klinePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &APIError{
			Path: path, URL: c.url(path, params), Body: truncate(string(body), 500),
			Message: "响应解析失败: " + err.Error(),
		}
	}
	if err := decode(body, path, c.url(path, params), payload.Code, payload.Message); err != nil {
		return nil, err
	}
	return payload.Data.Bars(), nil
}

// klinePayload K 线响应外层包裹。
type klinePayload struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    klineData `json:"data"`
}

// klineItem 接口原始字段：开高低收与涨跌额为字符串，k/v/t/lc 为数值，
// 类型混用是接口行为，这里统一用 FlexNum 兼容两种写法。
type klineItem struct {
	K  int64    `json:"k"`
	O  FlexNum  `json:"o"`
	H  FlexNum  `json:"h"`
	L  FlexNum  `json:"l"`
	C  FlexNum  `json:"c"`
	V  FlexNum  `json:"v"`
	T  FlexNum  `json:"t"`
	R  *FlexNum `json:"r"`
	LC FlexNum  `json:"lc"`
	CP FlexNum  `json:"cp"`
}

// klineData 响应 data 字段。
type klineData struct {
	List []klineItem `json:"list"`
}

// Bars 把原始条目转换为 KLineBar 切片。
func (p klineData) Bars() []KLineBar {
	bars := make([]KLineBar, 0, len(p.List))
	for _, item := range p.List {
		bar := KLineBar{
			Time:        item.K,
			Open:        item.O.String(),
			High:        item.H.String(),
			Low:         item.L.String(),
			Close:       item.C.String(),
			Volume:      item.V.Float(),
			Turnover:    item.T.Float(),
			LastClose:   item.LC.Float(),
			ChangePrice: item.CP.String(),
		}
		if item.R != nil {
			ratio := item.R.Float()
			bar.ChangeRatio = &ratio
		}
		bars = append(bars, bar)
	}
	return bars
}
