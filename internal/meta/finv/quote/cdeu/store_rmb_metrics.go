package cdeu

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"
)

// 本文件实现「证券实时快照 → 人民币计价指标」的手动补全（需求 11024）。
//
// 表：cdeu.finv_cdeu_secu_realtime_stock（V300126 增加 5 个 rmb_ 字段，需求 11023）。
// 两步口径（前端一个按钮顺序触发，也可用 step 参数单独执行）：
//  1. rate   —— 按 currency 求「1 单位本币 = N 人民币」，写入 rmb_exchange_rate；
//  2. amount —— 用 rmb_exchange_rate 把 market_value / circulation_market_value /
//     trade_trunover / price_nominal 换算成人民币写入对应 rmb_ 字段。
//
// 汇率来源：public.finv_fxcer 全表（280 行量级，一次读入内存），按
// 「直采 → 反向取倒数 → 经 USD 交叉」三级回退；CNY 自身恒为 1。
// 汇率的乘除用 math/big.Rat（有理数）计算，最终按 10 位小数落库，
// 不经过 float64，避免换算出 1e-16 级噪声。

// RMBMetricsStepAll / RMBMetricsStepRate / RMBMetricsStepAmount 补全步骤取值。
const (
	RMBMetricsStepAll    = "all"
	RMBMetricsStepRate   = "rate"
	RMBMetricsStepAmount = "amount"
	// FxCurrencyUSD 美元代码（人民币换算的交叉中介）。
	FxCurrencyUSD = "USD"
)

// rmbRateScale 汇率落库小数位（与列 NUMERIC(24,10) 一致）。
const rmbRateScale = 10

// RMBMetricsSyncResult 一次补全的结果（接口返回与前端提示用）。
type RMBMetricsSyncResult struct {
	// Step 本次执行的步骤：all / rate / amount。
	Step string `json:"step"`
	// RowsTotal 快照表总行数。
	RowsTotal int64 `json:"rows_total"`
	// RowsCurrencyMissing currency 为空、无法换算的行数。
	RowsCurrencyMissing int64 `json:"rows_currency_missing"`
	// RateRows 已有 rmb_exchange_rate 的行数（补全后统计）。
	RateRows int64 `json:"rate_rows"`
	// RateUpdate 本次写入/变更 rmb_exchange_rate 的行数。
	RateUpdate int64 `json:"rate_update"`
	// AmountRows 已有任一 rmb_ 金额字段的行数（补全后统计）。
	AmountRows int64 `json:"amount_rows"`
	// AmountUpdate 本次写入 rmb_ 金额字段的行数。
	AmountUpdate int64 `json:"amount_update"`
	// Rates 本次使用的汇率（币种 → 十进制文本），便于前端提示「HKD=0.91…」。
	Rates map[string]string `json:"rates"`
	// MissingCurrencies 查不到汇率的币种（不会写入任何 rmb_ 值）。
	MissingCurrencies []string `json:"missing_currencies"`
	// ElapsedMs 耗时（毫秒）。
	ElapsedMs int64 `json:"elapsed_ms"`
}

// NormalizeRMBMetricsStep 归一化并校验步骤参数：空 → all；非法值返回错误（handler 映射 400）。
func NormalizeRMBMetricsStep(step string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(step)) {
	case "", RMBMetricsStepAll:
		return RMBMetricsStepAll, nil
	case RMBMetricsStepRate:
		return RMBMetricsStepRate, nil
	case RMBMetricsStepAmount:
		return RMBMetricsStepAmount, nil
	default:
		return "", fmt.Errorf("%w: step 非法（仅支持 all / rate / amount）: %s", ErrRealtimeStockInvalid, step)
	}
}

// fxRateBook 缺省汇率快照：以「源|目标」为键的汇率表（1 源 = N 目标）。
type fxRateBook map[string]*big.Rat

// fxRateKey 汇率键。
func fxRateKey(src, dest string) string { return src + "|" + dest }

// get 取直采汇率；不存在返回 nil。
func (b fxRateBook) get(src, dest string) *big.Rat { return b[fxRateKey(src, dest)] }

// toCNY 求「1 单位 currency 折合多少人民币」。
//
// 回退顺序（与 Docs/DataDictMapping/FinvFxcer.md §4 一致）：
//  1. CNY 自身 → 1；
//  2. 直采 (currency, CNY)；
//  3. 反向 (CNY, currency) 取倒数；
//  4. 经 USD 交叉：currency→USD 与 USD→CNY 两腿相乘（每腿各自允许反向取倒数）。
//
// 任一环节缺失返回 nil（调用方记为「缺汇率币种」，不写任何值）。
func (b fxRateBook) toCNY(currency string) *big.Rat {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return nil
	}
	if currency == FxCurrencyCNY {
		return big.NewRat(1, 1)
	}
	if rate := b.get(currency, FxCurrencyCNY); rate != nil {
		return new(big.Rat).Set(rate)
	}
	if rate := b.get(FxCurrencyCNY, currency); rate != nil && rate.Sign() != 0 {
		return new(big.Rat).Inv(rate)
	}

	legA := b.legToUSD(currency)
	if legA == nil {
		return nil
	}
	// USD → CNY：直采，或 CNY→USD 取倒数
	legB := b.get(FxCurrencyUSD, FxCurrencyCNY)
	if legB == nil {
		if rev := b.get(FxCurrencyCNY, FxCurrencyUSD); rev != nil && rev.Sign() != 0 {
			legB = new(big.Rat).Inv(rev)
		}
	}
	if legB == nil {
		return nil
	}
	return new(big.Rat).Mul(legA, legB)
}

// legToUSD 求「1 单位 currency 折合多少 USD」：直采优先，缺失时用 (USD, currency) 取倒数。
func (b fxRateBook) legToUSD(currency string) *big.Rat {
	if currency == FxCurrencyUSD {
		return big.NewRat(1, 1)
	}
	if rate := b.get(currency, FxCurrencyUSD); rate != nil {
		return new(big.Rat).Set(rate)
	}
	if rev := b.get(FxCurrencyUSD, currency); rev != nil && rev.Sign() != 0 {
		return new(big.Rat).Inv(rev)
	}
	return nil
}

// SyncRMBMetrics 手动补全快照表的人民币计价指标（需求 11024）。
//
// 步骤按 step 参数执行；all 表示先 rate 后 amount（前端一个按钮的默认行为）。
// 两个步骤都是幂等 UPDATE（值未变化不计入 update 行数），可反复点击。
func (s *Store) SyncRMBMetrics(ctx context.Context, step string) (RMBMetricsSyncResult, error) {
	started := time.Now()
	result := RMBMetricsSyncResult{Step: step, Rates: map[string]string{}, MissingCurrencies: []string{}}

	// 1) 读缺省汇率表（280 行量级，一次读入）
	book, err := s.loadFxRateBook(ctx)
	if err != nil {
		return result, err
	}

	// 2) 快照表出现过的币种 → 汇率（文本，10 位小数）
	currencies, err := s.realtimeStockCurrencies(ctx)
	if err != nil {
		return result, err
	}
	rates := make(map[string]string, len(currencies))
	for _, currency := range currencies {
		rate := book.toCNY(currency)
		if rate == nil {
			result.MissingCurrencies = append(result.MissingCurrencies, currency)
			continue
		}
		rates[currency] = rate.FloatString(rmbRateScale)
	}
	sort.Strings(result.MissingCurrencies)
	result.Rates = rates

	// 3) 第一步：写 rmb_exchange_rate（按币种逐条 UPDATE，币种数量在几十个量级）
	if step == RMBMetricsStepAll || step == RMBMetricsStepRate {
		for _, currency := range sortedKeys(rates) {
			tag, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_cdeu_secu_realtime_stock t
   SET rmb_exchange_rate = $1::numeric
 WHERE btrim(upper(t.currency)) = $2
   AND t.rmb_exchange_rate IS DISTINCT FROM $1::numeric`, rates[currency], currency)
			if err != nil {
				return result, fmt.Errorf("SyncRMBMetrics：写入 %s 的 rmb_exchange_rate 失败: %w", currency, err)
			}
			result.RateUpdate += tag.RowsAffected()
		}
	}

	// 4) 第二步：按汇率换算金额/价格（只在已知汇率、且对应本币值存在时写）
	if step == RMBMetricsStepAll || step == RMBMetricsStepAmount {
		tag, err := s.pool.Exec(ctx, `
UPDATE cdeu.finv_cdeu_secu_realtime_stock t
   SET rmb_market_value             = CASE WHEN t.market_value IS NULL THEN NULL
                                           ELSE round(t.market_value * t.rmb_exchange_rate, 6) END,
       rmb_circulation_market_value = CASE WHEN t.circulation_market_value IS NULL THEN NULL
                                           ELSE round(t.circulation_market_value * t.rmb_exchange_rate, 6) END,
       rmb_trade_trunover           = CASE WHEN t.trade_trunover IS NULL THEN NULL
                                           ELSE round(t.trade_trunover * t.rmb_exchange_rate, 6) END,
       rmb_price_nominal            = CASE WHEN t.price_nominal IS NULL THEN NULL
                                           ELSE round(t.price_nominal * t.rmb_exchange_rate, 6) END
 WHERE t.rmb_exchange_rate IS NOT NULL
   AND (t.market_value IS NOT NULL OR t.circulation_market_value IS NOT NULL
        OR t.trade_trunover IS NOT NULL OR t.price_nominal IS NOT NULL)
   AND (t.rmb_market_value IS DISTINCT FROM round(t.market_value * t.rmb_exchange_rate, 6)
        OR t.rmb_circulation_market_value IS DISTINCT FROM round(t.circulation_market_value * t.rmb_exchange_rate, 6)
        OR t.rmb_trade_trunover IS DISTINCT FROM round(t.trade_trunover * t.rmb_exchange_rate, 6)
        OR t.rmb_price_nominal IS DISTINCT FROM round(t.price_nominal * t.rmb_exchange_rate, 6))`)
		if err != nil {
			return result, fmt.Errorf("SyncRMBMetrics：写入 rmb_ 金额字段失败: %w", err)
		}
		result.AmountUpdate = tag.RowsAffected()
	}

	// 5) 汇总
	if err := s.pool.QueryRow(ctx, `
SELECT count(*),
       count(*) FILTER (WHERE currency IS NULL OR btrim(currency) = ''),
       count(*) FILTER (WHERE rmb_exchange_rate IS NOT NULL),
       count(*) FILTER (WHERE rmb_market_value IS NOT NULL OR rmb_circulation_market_value IS NOT NULL
                           OR rmb_trade_trunover IS NOT NULL OR rmb_price_nominal IS NOT NULL)
  FROM cdeu.finv_cdeu_secu_realtime_stock`).
		Scan(&result.RowsTotal, &result.RowsCurrencyMissing, &result.RateRows, &result.AmountRows); err != nil {
		return result, fmt.Errorf("SyncRMBMetrics：统计结果失败: %w", err)
	}

	result.ElapsedMs = time.Since(started).Milliseconds()
	return result, nil
}

// loadFxRateBook 读取 public.finv_fxcer 全表为有理数汇率表。
func (s *Store) loadFxRateBook(ctx context.Context) (fxRateBook, error) {
	rows, err := s.pool.Query(ctx, `
SELECT currency_src, currency_dest, rate::text FROM public.finv_fxcer`)
	if err != nil {
		return nil, fmt.Errorf("SyncRMBMetrics：读取缺省汇率失败: %w", err)
	}
	defer rows.Close()

	book := make(fxRateBook, 320)
	for rows.Next() {
		var src, dest, rateText string
		if err := rows.Scan(&src, &dest, &rateText); err != nil {
			return nil, fmt.Errorf("SyncRMBMetrics：读取汇率行失败: %w", err)
		}
		rate, ok := new(big.Rat).SetString(strings.TrimSpace(rateText))
		if !ok || rate.Sign() <= 0 {
			continue // 非法汇率跳过（表侧已有 CHECK，这里只做防御）
		}
		book[fxRateKey(src, dest)] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SyncRMBMetrics：遍历汇率失败: %w", err)
	}
	return book, nil
}

// realtimeStockCurrencies 取快照表里出现过的币种（去空白、去空值、统一大写）。
func (s *Store) realtimeStockCurrencies(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
SELECT DISTINCT upper(btrim(currency)) AS currency
  FROM cdeu.finv_cdeu_secu_realtime_stock
 WHERE currency IS NOT NULL AND btrim(currency) <> ''
 ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("SyncRMBMetrics：读取币种失败: %w", err)
	}
	defer rows.Close()

	list := make([]string, 0, 16)
	for rows.Next() {
		var currency string
		if err := rows.Scan(&currency); err != nil {
			return nil, fmt.Errorf("SyncRMBMetrics：读取币种行失败: %w", err)
		}
		list = append(list, currency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SyncRMBMetrics：遍历币种失败: %w", err)
	}
	return list, nil
}

// sortedKeys 返回 map 的有序键（输出稳定，便于日志与前端提示顺序一致）。
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
