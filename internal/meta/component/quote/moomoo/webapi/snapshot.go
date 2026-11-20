package webapi

// 行情快照：POST /api/v1.0/quote/snapshot（一次最多 400 个 code）。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Fetcher 行情快照拉取能力（Client 实现；测试可注入假实现）。
type Fetcher interface {
	GetQuoteSnapshots(ctx context.Context, codes []string) ([]Snapshot, error)
}

// Snapshot 快照响应里本仓库用到的字段（其余字段不解析）。
type Snapshot struct {
	Code string `json:"code"`
	// Name 上游默认名称（多数场景是英文，如 `Bitcoin` / `Hang Seng Index`）。
	//
	// 业务取名称请用 [Snapshot.DisplayName]（中文优先），**不要**直接用本字段；
	// 只有需要中英文各存一列时才单独取本字段作英文名（见包注释「名称取值规范」）。
	Name string `json:"name"`
	// SCName 简体中文名称（上游 `sc_name`，如 `比特币` / `恒生指数`；上游未提供时为空）。
	//
	// 规范口径：展示/落库的默认名称取本字段，为空才回退 [Snapshot.Name]
	// （见 [Snapshot.DisplayName] 与包注释「名称取值规范」）。
	SCName string `json:"sc_name"`
	// TCName 繁体中文名称（上游 `tc_name`，如 `比特幣` / `恒生指數`；暂不使用，保留便于追溯）。
	TCName string `json:"tc_name"`
	// LastPrice 最新价。
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

// DisplayName 默认展示名：优先简体中文（`sc_name`），取不到才回退上游默认名（`name`）。
//
// 实测（2026-11-19，`US..SPX` / `HK.800000` / `CC.BTC`）上游三个名称字段都有值：
// `name=S&P 500 Index`、`sc_name=标普500指数`、`tc_name=標普500指數`；
// 只取 `name` 会让排行榜全显示英文名（需求 11339）。
//
// **这是本仓库取名称的唯一默认口径**：落库、接口响应、日志都用它；
// 只有业务明确要求中英文各存一列时，才在自己的落库结构里分别取 `SCName` / `Name`
// （见包注释「名称取值规范」）。
func (s Snapshot) DisplayName() string {
	if name := strings.TrimSpace(s.SCName); name != "" {
		return name
	}
	return strings.TrimSpace(s.Name)
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
