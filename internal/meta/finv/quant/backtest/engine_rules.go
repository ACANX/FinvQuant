package backtest

import (
	"math"
)

// ---------------------------------------------------------------------
// 环境规则与成本覆盖（评审意见落实）：
//   - 成本覆盖链统一为「环境 > 任务（options）> 策略（definition.cost）> 账户」，
//     与 Docs/DevSpec/BacktestStrategySpec.md / FR-14 文档一致，抽取为独立函数并配单测；
//   - 环境交易规则（FR-14 引擎自适应）：T+N 交收 / 涨跌停 / 合约乘数 / 撮合模式 /
//     币种，在引擎中真正生效（原实现仅 tick_size 对齐与交易时段过滤生效）。
// ---------------------------------------------------------------------

// barLimit 单根 bar 的涨跌停限价（0=无限制）。
type barLimit struct {
	up   float64 // 涨停价（买入不可高于）
	down float64 // 跌停价（卖出不可低于）
}

// envRules 引擎运行期生效的环境规则快照。
type envRules struct {
	tPlus        int     // T+N 交收（0=T+0）
	limitUpPct   float64 // 涨停幅度 %（0=无限制）
	limitDownPct float64 // 跌停幅度 %（0=无限制）
	multiplier   float64 // 合约乘数（股票/ETF=1，期货如 COMEX 黄金=100）
	fillMode     string  // 撮合模式（NEXT_BAR_OPEN / CURRENT_CLOSE）
	currency     string  // 计价币种
}

// environmentRules 从环境快照提取运行期生效的交易规则（空环境返回默认值）。
func (e *Engine) environmentRules() envRules {
	r := envRules{multiplier: 1, fillMode: ""}
	if e.cfg.Environment == nil || e.cfg.Environment.Config.TradingRules == nil {
		return r
	}
	tr := e.cfg.Environment.Config.TradingRules
	if tr.TPlus > 0 {
		r.tPlus = tr.TPlus
	}
	if tr.LimitUpPct > 0 {
		r.limitUpPct = tr.LimitUpPct
	}
	if tr.LimitDownPct > 0 {
		r.limitDownPct = tr.LimitDownPct
	}
	if tr.ContractMultiplier > 1 {
		r.multiplier = tr.ContractMultiplier
	}
	if fm := e.cfg.Environment.Config.FillMode; fm != "" {
		r.fillMode = fm
	}
	if cur := e.cfg.Environment.Config.Currency; cur != "" {
		r.currency = cur
	}
	return r
}

// resolveCosts 解析最终生效的手续费率与滑点。
//
// 覆盖链（优先级从高到低）：环境 > 任务（options）> 策略（definition.cost）> 账户。
// 环境代表市场/交易所的成本基准（如 COMEX 黄金 0.03%），是最高权威；
// 未配置的高优先级项回退到低优先级项。
func resolveCosts(cfg EngineConfig) (commission, slippage float64) {
	// 基线：账户配置
	commission = cfg.Account.CommissionRate
	slippage = cfg.Account.SlippagePct
	// 策略定义 cost（definition.cost）
	if cfg.Definition.Cost != nil {
		if cfg.Definition.Cost.CommissionRate > 0 {
			commission = cfg.Definition.Cost.CommissionRate
		}
		if cfg.Definition.Cost.SlippagePct > 0 {
			slippage = cfg.Definition.Cost.SlippagePct
		}
	}
	// 任务级覆盖（options）
	if cfg.Options.CommissionRate != nil {
		commission = *cfg.Options.CommissionRate
	}
	if cfg.Options.SlippagePct != nil {
		slippage = *cfg.Options.SlippagePct
	}
	// 环境成本基准（优先级最高）
	if cfg.Environment != nil && cfg.Environment.Config.Cost != nil {
		if cfg.Environment.Config.Cost.CommissionRate > 0 {
			commission = cfg.Environment.Config.Cost.CommissionRate
		}
		if cfg.Environment.Config.Cost.SlippagePct > 0 {
			slippage = cfg.Environment.Config.Cost.SlippagePct
		}
	}
	return commission, slippage
}

// maxEquityPoints 单任务净值曲线最大落库点数（Min 精度全量区间防数据量爆炸，
// 超过时均匀降采样；汇总报告指标不受影响）。
const maxEquityPoints = 20000

// downsampleEquity 当曲线点数超过上限时均匀降采样（保留首尾点）。
func downsampleEquity(points []EquityPoint, max int) []EquityPoint {
	if len(points) <= max {
		return points
	}
	out := make([]EquityPoint, 0, max)
	step := float64(len(points)-1) / float64(max-1)
	for i := 0; i < max; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx >= len(points) {
			idx = len(points) - 1
		}
		out = append(out, points[idx])
	}
	return out
}
