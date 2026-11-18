package webapi

// 行情快照：POST /api/v1.0/quote/snapshot（一次最多 400 个 code）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Fetcher 行情快照拉取能力（Client 实现；测试可注入假实现）。
type Fetcher interface {
	GetQuoteSnapshots(ctx context.Context, codes []string) ([]Snapshot, error)
}

// Snapshot 快照响应里本仓库用到的字段（其余字段不解析）。
type Snapshot struct {
	Code                string  `json:"code"`
	Name                string  `json:"name"`
	LastPrice           float64 `json:"last_price"`
	OpenPrice           float64 `json:"open_price"`
	HighPrice           float64 `json:"high_price"`
	LowPrice            float64 `json:"low_price"`
	PrevClosePrice      float64 `json:"prev_close_price"`
	AvgPrice            float64 `json:"avg_price"`
	Amplitude           float64 `json:"amplitude"`
	Volume              float64 `json:"volume"`
	Turnover            float64 `json:"turnover"`
	VolumeRatio         float64 `json:"volume_ratio"`
	BidPrice            float64 `json:"bid_price"`
	AskPrice            float64 `json:"ask_price"`
	BidVol              float64 `json:"bid_vol"`
	AskVol              float64 `json:"ask_vol"`
	BidAskRatio         float64 `json:"bid_ask_ratio"`
	Highest52WeeksPrice float64 `json:"highest52weeks_price"`
	Lowest52WeeksPrice  float64 `json:"lowest52weeks_price"`
	HighestHistoryPrice float64 `json:"highest_history_price"`
	LowestHistoryPrice  float64 `json:"lowest_history_price"`
	SecStatus           string  `json:"sec_status"`
	Suspension          bool    `json:"suspension"`
	DataDate            string  `json:"data_date"`
	UpdateTime          int64   `json:"update_time"`
}

// snapshotEnvelope 响应信封。
type snapshotEnvelope struct {
	RetCode int    `json:"ret_code"`
	RetMsg  string `json:"ret_msg"`
	Data    *struct {
		SnapshotList []Snapshot `json:"snapshot_list"`
	} `json:"data"`
}

// GetQuoteSnapshots 批量拉取行情快照（一次最多 MaxCodes 个 code）。
func (c *Client) GetQuoteSnapshots(ctx context.Context, codes []string) ([]Snapshot, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	if len(codes) > MaxCodes {
		return nil, fmt.Errorf("moomoo OpenAPI：单次最多 %d 个 code，实际 %d", MaxCodes, len(codes))
	}
	payload, err := json.Marshal(map[string]any{"code_list": codes})
	if err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：序列化请求体失败: %w", err)
	}

	raw, status, err := c.do(ctx, http.MethodPost, SnapshotPath, "", payload)
	if err != nil {
		return nil, err
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("moomoo OpenAPI：解析快照响应失败（HTTP %d）: %w", status, err)
	}
	if status != http.StatusOK || envelope.RetCode != 0 {
		return nil, fmt.Errorf("moomoo OpenAPI：HTTP %d ret_code=%d ret_msg=%s",
			status, envelope.RetCode, envelope.RetMsg)
	}
	if envelope.Data == nil {
		return nil, errors.New("moomoo OpenAPI：响应缺少 data")
	}
	return envelope.Data.SnapshotList, nil
}

// 编译期断言：Client 实现 Fetcher。
var _ Fetcher = (*Client)(nil)
