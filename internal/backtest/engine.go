package backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------
// 行情 Bar（引擎输入，周期已聚合）
// ---------------------------------------------------------------------

// Bar 回测行情 bar。
type Bar struct {
	TS       int64 // UTC 秒
	Date     int   // yyyymmdd
	Time     int   // hhmmss
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
	Turnover float64
}

// EngineConfig 引擎配置（服务层从策略定义+账户快照+环境快照+任务参数组装）。
type EngineConfig struct {
	Definition      StrategyDefinition
	Account         AccountSnapshot
	Environment     *Environment // 环境快照（可空，自适应交易时段/规则/成本）
	SecuCode        string
	Period          string
	StartTS         int64
	EndTS           int64
	ReportPrecision string
	Options         RunOptions
}

// EngineResult 引擎输出。
type EngineResult struct {
	EquityPoints []EquityPoint
	Trades       []Trade
	Cashflows    []Cashflow
	PositionLogs []PositionLog
	EventTraces  []EventTrace
	Report       *RunReport
	BarCount     int
}

// ProgressFunc 进度回调（processed/total）。
type ProgressFunc func(processed, total int)

// Engine 回测引擎。
type Engine struct {
	cfg    EngineConfig
	onProg ProgressFunc
}

// NewEngine 创建回测引擎。
func NewEngine(cfg EngineConfig, onProg ProgressFunc) *Engine {
	return &Engine{cfg: cfg, onProg: onProg}
}

// 运行时账户状态
type accountState struct {
	cash      float64
	position  float64
	avgCost   float64 // 持仓加权成本
	entryDate int     // 开仓日期
}

// 每日/全局计数
type dayCounters struct {
	trades        int            // 当日总成交笔数
	ruleTrades    map[string]int // 规则当日触发次数
	ruleRunTrades map[string]int // 规则整个回测触发次数
	lastTradeBar  int            // 最近成交 bar 索引
	curDate       int            // 当前日期（用于跨日重置）

	// 滑动窗口计数（滚动 N 自然日，按成交 bar 的 date 判定）
	weekDates   []int            // 近 7 日成交日期序列
	monthDates  []int            // 近 30 日成交日期序列
	ruleWeek    map[string][]int // 分方向：近 7 日触发日期
	ruleMonth   map[string][]int // 分方向：近 30 日触发日期
	feeWindows  []feeWinItem     // 滚动窗口手续费记录（date + 手续费金额）
	ruleFees    map[string][]feeWinItem
	builderBar  int              // 上次加仓 bar 索引（分批建仓间隔控制）
	builderDate int              // 上次加仓日期
	reduceRemain int             // 分批减仓剩余次数（开仓时置为 ReduceTranches，每次减仓递减）
}

// feeWinItem 滚动费用窗口单笔记录。
type feeWinItem struct {
	date int
	fee  float64
}

// Run 执行回测。ctx 支持取消（任务取消/服务停机）；bars 为已按周期聚合、时间升序的行情。
func (e *Engine) Run(ctx context.Context, bars []Bar) (*EngineResult, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("回测区间内无行情数据")
	}
	def := e.cfg.Definition
	acc := e.cfg.Account

	initialCapital := acc.InitialCapital
	if e.cfg.Options.InitialCapital != nil && *e.cfg.Options.InitialCapital > 0 {
		initialCapital = *e.cfg.Options.InitialCapital
	}

	// 成本覆盖链（环境 > 任务 > 策略 > 账户）：环境（市场/交易所成本基准）优先级最高。
	// 抽取为独立函数，见 resolveCosts 及其单元测试。
	commission, slippage := resolveCosts(e.cfg)

	// 环境交易规则（FR-14 引擎自适应）：T+N 交收 / 涨跌停 / 合约乘数 / 撮合模式 / 币种
	envRules := e.environmentRules() // 见 engine_rules.go

	// 环境交易时段（hhmmss 列表，自适应不同市场交易时段；支持跨午夜时段如 210000-023000）
	envSessions := [][2]string{}
	if e.cfg.Environment != nil {
		for _, s := range e.cfg.Environment.Config.TradingSessions {
			envSessions = append(envSessions, [2]string{s.Start, s.End})
		}
	}
	// 交易时段可读描述（拒绝原因文案用，如 "08:20-13:30"）
	sessionDesc := "未配置"
	if len(envSessions) > 0 {
		parts := []string{}
		for _, se := range envSessions {
			parts = append(parts, se[0][:2]+":"+se[0][2:4]+"-"+se[1][:2]+":"+se[1][2:4])
		}
		sessionDesc = strings.Join(parts, "/")
	}
	envInSession := func(b Bar) bool {
		if len(envSessions) == 0 {
			return true
		}
		timeInt := b.Time
		for _, se := range envSessions {
			start, _ := strconv.Atoi(se[0])
			end, _ := strconv.Atoi(se[1])
			if start <= end {
				if timeInt >= start && timeInt <= end {
					return true
				}
			} else {
				// 跨午夜时段（如夜盘 210000-023000）
				if timeInt >= start || timeInt <= end {
					return true
				}
			}
		}
		return false
	}

	// 涨跌停参考价（前一交易日收盘）与限价预计算：limitPrices[i].up/down=0 表示无限制
	limitPrices := make([]barLimit, len(bars))
	prevDayClose := 0.0
	for i, b := range bars {
		if i > 0 && bars[i-1].Date != b.Date {
			prevDayClose = bars[i-1].Close
		}
		if prevDayClose <= 0 && i > 0 {
			prevDayClose = bars[i-1].Close
		}
		if prevDayClose > 0 {
			if envRules.limitUpPct > 0 {
				limitPrices[i].up = prevDayClose * (1 + envRules.limitUpPct/100)
			}
			if envRules.limitDownPct > 0 {
				limitPrices[i].down = prevDayClose * (1 - envRules.limitDownPct/100)
			}
		}
	}

	maxTradesPerDay := def.Risk.MaxTradesPerDay
	if e.cfg.Options.MaxTradesPerDay != nil {
		maxTradesPerDay = *e.cfg.Options.MaxTradesPerDay
	}
	allowedTimes := map[string]bool{}
	for _, t := range e.cfg.Options.AllowedTimes {
		allowedTimes[strings.TrimSpace(t)] = true
	}
	// 任务级未指定时回退到策略规则的 allowed_times
	if len(allowedTimes) == 0 {
		for _, t := range def.Rules.Buy.AllowedTimes {
			allowedTimes[strings.TrimSpace(t)] = true
		}
		for _, t := range def.Rules.Sell.AllowedTimes {
			allowedTimes[strings.TrimSpace(t)] = true
		}
	}

	// 校验回测开关
	if !e.cfg.Options.EnableBacktest {
		return nil, fmt.Errorf("回测开关未启用（enable_backtest=false），已拒绝启动回测")
	}

	// 编译信号表达式
	buyExpr, err := CompileExpr(def.Signals.Buy)
	if err != nil && strings.TrimSpace(def.Signals.Buy) != "" {
		return nil, err
	}
	sellExpr, err := CompileExpr(def.Signals.Sell)
	if err != nil && strings.TrimSpace(def.Signals.Sell) != "" {
		return nil, err
	}

	// 构建字段序列与指标
	fields := buildFieldSeries(bars)
	indicators := map[string][]float64{}
	for _, indDef := range def.Indicators {
		ind, err := buildIndicator(indDef, fields)
		if err != nil {
			return nil, err
		}
		indicators[ind.ID()] = ind.Values()
	}
	evalCtx := &EvalContext{Fields: fields, Indicators: indicators}

	fillMode := def.Data.FillMode
	// 撮合模式覆盖链：环境 config.fill_mode > 策略 data.fill_mode（缺省 NEXT_BAR_OPEN）
	if envRules.fillMode != "" {
		fillMode = envRules.fillMode
	}
	if fillMode == "" {
		fillMode = FillNextBarOpen
	}

	state := &accountState{cash: initialCapital}
	dc := &dayCounters{
		ruleTrades:    map[string]int{},
		ruleRunTrades: map[string]int{},
		ruleWeek:      map[string][]int{},
		ruleMonth:     map[string][]int{},
		ruleFees:      map[string][]feeWinItem{},
		lastTradeBar:  -1,
		builderBar:    -1,
	}

	result := &EngineResult{}
	trades := []Trade{}
	equityPoints := []EquityPoint{}
	cashflows := []Cashflow{}
	positionLogs := []PositionLog{}
	eventTraces := []EventTrace{}

	// 初始资金注入流水（需求⑨-1）
	if len(bars) > 0 {
		cashflows = append(cashflows, Cashflow{
			Seq: 1, TS: bars[0].TS, Date: bars[0].Date, Time: bars[0].Time,
			FlowType: "INITIAL_DEPOSIT", Amount: initialCapital,
			CashBefore: 0, CashAfter: initialCapital, TradeID: 0, Remark: "初始启动资金注入",
		})
	}

	// 报告桶（按 report_precision 聚合曲线点）
	bucketKeyOf := func(b Bar) string {
		switch e.cfg.ReportPrecision {
		case PeriodMin:
			return fmt.Sprintf("%d", b.TS)
		case PeriodHour:
			return fmt.Sprintf("%d-%02d", b.Date, b.Time/10000)
		default:
			return fmt.Sprintf("%d", b.Date)
		}
	}
	curBucket := ""

	// 投入统计
	var maxInvested, investedSum, investedBars float64
	investedDays := map[int]bool{}

	// 峰值与回撤
	peakEquity := initialCapital
	maxDD := 0.0
	maxDDStart, maxDDEnd := int64(0), int64(0)
	ddStartTS := int64(0)

	// 平仓盈亏统计
	var grossProfit, grossLoss float64
	winCount, lossCount := 0, 0
	signalDetail := map[string]int{}
	totalFee := 0.0
	periodReturns := []float64{}
	prevBucketEquity := initialCapital

	// 挂单（NEXT_BAR_OPEN：信号在当前 bar 收盘确认，下一 bar 开盘价成交）
	var pendingOrder *pendingOrderT

	warmup := def.Data.WarmupBars
	if warmup < 0 {
		warmup = 0
	}

	reportEvery := len(bars) / 100
	if reportEvery < 1 {
		reportEvery = 1
	}

	for i, b := range bars {
		if i < warmup {
			continue
		}
		// 取消检查（每 512 根检查一次，避免热循环频繁访问）
		if i%512 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("回测任务已取消: %w", err)
			}
		}
		evalCtx.At = i

		// 每根 bar 开头做跨日重置：每日计数（成交笔数/规则触发次数）按 bar 日期重置，
		// 而非仅在成交时重置 —— 修复“某日达到每日上限后永久死锁、后续所有信号被拒”的问题。
		if dc.curDate != b.Date {
			dc.curDate = b.Date
			dc.trades = 0
			dc.ruleTrades = map[string]int{}
		}

		// ---------- 1. 执行上一 bar 收盘产生的挂单（当前 bar 开盘价成交，事件追踪更新） ----------
		if pendingOrder != nil {
			execPrice := b.Open * (1 + pendingOrder.slippageSign*slippage/100)
			tradeSeq, executed, rejectReason := e.fill(pendingOrder.action, execPrice, b, i, state, dc, &trades,
				&cashflows, &positionLogs, commission, &totalFee, signalDetail,
				&grossProfit, &grossLoss, &winCount, &lossCount, pendingOrder.signal,
				maxTradesPerDay, limitPrices)
			ev := &eventTraces[pendingOrder.eventIdx]
			ev.ExecTS, ev.ExecDate, ev.ExecTime = b.TS, b.Date, b.Time
			ev.LatencyBars = i - pendingOrder.triggerBar
			ev.LatencySec = b.TS - ev.TriggerTS
			ev.AliveSec = b.TS - ev.TriggerTS
			ev.Price = round6(execPrice)
			if executed {
				dc.lastTradeBar = i
				ev.ExecStatus = "FILLED"
				ev.Qty = trades[len(trades)-1].Qty
				ev.TradeSeq = int(tradeSeq)
			} else {
				ev.ExecStatus = "REJECTED"
				ev.RejectReason = rejectReason
			}
			pendingOrder = nil
		}

		// ---------- 2. 风控（止损/止盈，基于当前 bar 高低价内触达，intrabar 直接成交） ----------
		if state.position > 0 && (def.Risk.StopLossPct > 0 || def.Risk.TakeProfitPct > 0) {
			// T+N 交收限制：当日买入的持仓在 T+N 内不可卖出（含风控平仓）
			if envRules.tPlus > 0 && state.entryDate == b.Date {
				if def.Risk.StopLossPct > 0 && b.Low <= state.avgCost*(1-def.Risk.StopLossPct/100) ||
					def.Risk.TakeProfitPct > 0 && b.High >= state.avgCost*(1+def.Risk.TakeProfitPct/100) {
					e.recordReject(&eventTraces, ActionSell, "止损/止盈", b, fmt.Sprintf("T+%d 交收限制（当日买入不可卖出）", envRules.tPlus))
				}
			} else {
				stopPrice := state.avgCost * (1 - def.Risk.StopLossPct/100)
				tpPrice := state.avgCost * (1 + def.Risk.TakeProfitPct/100)
				exitPrice := 0.0
				reason := ""
				if def.Risk.StopLossPct > 0 && b.Low <= stopPrice {
					exitPrice = stopPrice
					reason = "止损"
				} else if def.Risk.TakeProfitPct > 0 && b.High >= tpPrice {
					exitPrice = tpPrice
					reason = "止盈"
				}
				if exitPrice > 0 {
					tradeSeq, executed, rejectReason := e.fill(ActionSell, exitPrice, b, i, state, dc, &trades,
						&cashflows, &positionLogs, commission, &totalFee, signalDetail,
						&grossProfit, &grossLoss, &winCount, &lossCount, reason,
						maxTradesPerDay, limitPrices)
					// 风控事件：触发即成交（latency=0，intrabar 限价单语义）
					evIdx := e.newEvent(&eventTraces, ActionSell, reason, b)
					ev := &eventTraces[evIdx]
					ev.ExecTS, ev.ExecDate, ev.ExecTime = b.TS, b.Date, b.Time
					ev.AliveSec = 0
					ev.Price = round6(exitPrice)
					if executed {
						dc.lastTradeBar = i
						ev.ExecStatus = "FILLED"
						ev.Qty = trades[len(trades)-1].Qty
						ev.TradeSeq = int(tradeSeq)
					} else {
						ev.ExecStatus = "REJECTED"
						ev.RejectReason = rejectReason
					}
				}
			}
		}

		// ---------- 3. 信号求值（基于当前 bar 收盘，不做限制过滤；限制与拒绝原因在步骤 4 统一判定） ----------
		buyHit, sellHit := false, false
		if buyRuleAllowed(def) && strings.TrimSpace(def.Signals.Buy) != "" {
			v, err := buyExpr(evalCtx)
			if err != nil {
				return nil, err
			}
			buyHit = v
		}
		if sellRuleAllowed(def) && strings.TrimSpace(def.Signals.Sell) != "" {
			v, err := sellExpr(evalCtx)
			if err != nil {
				return nil, err
			}
			sellHit = v
		}
		barTime := fmt.Sprintf("%02d%02d%02d", b.Time/10000, (b.Time/100)%100, b.Time%100)

		// ---------- 4. 下单（限制判定：仓位/时间点/环境时段/频率/次数，事件追踪登记拒绝原因） ----------
		inSession := envInSession(b)
		if buyHit {
			if builderReject := e.buyPositionReject(b, i, state, dc, def.Risk); builderReject != "" {
				e.recordReject(&eventTraces, ActionBuy, "买入信号", b, builderReject)
			} else if len(allowedTimes) > 0 && !allowedTimes[barTime] {
				e.recordReject(&eventTraces, ActionBuy, "买入信号", b, "不在允许交易时间点内")
			} else if !inSession {
				e.recordReject(&eventTraces, ActionBuy, "买入信号", b,
					fmt.Sprintf("当前bar时间 %s 不在环境交易时段（%s）内，非时段行情不产生交易", barTime, sessionDesc))
			} else if reason := dc.ruleRejectReason(def.Rules.Buy, "buy", maxTradesPerDay, i, def.Risk); reason != "" {
				e.recordReject(&eventTraces, ActionBuy, "买入信号", b, reason)
			} else if fillMode == FillCurrentClose {
				// CURRENT_CLOSE 撮合：当前 bar 收盘价立即成交（近似撮合，非默认）
				dc.recordRuleTrigger("buy", b.Date)
				fillPrice := b.Close * (1 + slippage/100)
				tradeSeq, executed, rejectReason := e.fill(ActionBuy, fillPrice, b, i, state, dc, &trades,
					&cashflows, &positionLogs, commission, &totalFee, signalDetail,
					&grossProfit, &grossLoss, &winCount, &lossCount, "买入信号",
					maxTradesPerDay, limitPrices)
				evIdx := e.newEvent(&eventTraces, ActionBuy, "买入信号", b)
				ev := &eventTraces[evIdx]
				ev.ExecTS, ev.ExecDate, ev.ExecTime = b.TS, b.Date, b.Time
				ev.LatencyBars, ev.LatencySec, ev.AliveSec = 0, 0, 0
				ev.Price = round6(fillPrice)
				if executed {
					dc.lastTradeBar = i
					ev.ExecStatus = "FILLED"
					ev.Qty = trades[len(trades)-1].Qty
					ev.TradeSeq = int(tradeSeq)
				} else {
					ev.ExecStatus = "REJECTED"
					ev.RejectReason = rejectReason
				}
			} else {
				dc.recordRuleTrigger("buy", b.Date)
				evIdx := e.newEvent(&eventTraces, ActionBuy, "买入信号", b)
				pendingOrder = &pendingOrderT{action: ActionBuy, slippageSign: 1, signal: "买入信号", eventIdx: evIdx, triggerBar: i}
			}
		}
		if sellHit {
			if state.position <= 0 {
				e.recordReject(&eventTraces, ActionSell, "卖出信号", b, "当前无持仓，卖出信号无法执行（空仓状态）")
			} else if len(allowedTimes) > 0 && !allowedTimes[barTime] {
				e.recordReject(&eventTraces, ActionSell, "卖出信号", b, "不在允许交易时间点内")
			} else if !inSession {
				e.recordReject(&eventTraces, ActionSell, "卖出信号", b,
					fmt.Sprintf("当前bar时间 %s 不在环境交易时段（%s）内，非时段行情不产生交易", barTime, sessionDesc))
			} else if envRules.tPlus > 0 && state.entryDate == b.Date {
				e.recordReject(&eventTraces, ActionSell, "卖出信号", b, fmt.Sprintf("T+%d 交收限制（当日买入不可卖出）", envRules.tPlus))
			} else if reason := dc.ruleRejectReason(def.Rules.Sell, "sell", maxTradesPerDay, i, def.Risk); reason != "" {
				e.recordReject(&eventTraces, ActionSell, "卖出信号", b, reason)
			} else if fillMode == FillCurrentClose {
				// CURRENT_CLOSE 撮合：当前 bar 收盘价立即成交
				dc.recordRuleTrigger("sell", b.Date)
				fillPrice := b.Close * (1 - slippage/100)
				tradeSeq, executed, rejectReason := e.fill(ActionSell, fillPrice, b, i, state, dc, &trades,
					&cashflows, &positionLogs, commission, &totalFee, signalDetail,
					&grossProfit, &grossLoss, &winCount, &lossCount, "卖出信号",
					maxTradesPerDay, limitPrices)
				evIdx := e.newEvent(&eventTraces, ActionSell, "卖出信号", b)
				ev := &eventTraces[evIdx]
				ev.ExecTS, ev.ExecDate, ev.ExecTime = b.TS, b.Date, b.Time
				ev.LatencyBars, ev.LatencySec, ev.AliveSec = 0, 0, 0
				ev.Price = round6(fillPrice)
				if executed {
					dc.lastTradeBar = i
					ev.ExecStatus = "FILLED"
					ev.Qty = trades[len(trades)-1].Qty
					ev.TradeSeq = int(tradeSeq)
				} else {
					ev.ExecStatus = "REJECTED"
					ev.RejectReason = rejectReason
				}
			} else {
				dc.recordRuleTrigger("sell", b.Date)
				evIdx := e.newEvent(&eventTraces, ActionSell, "卖出信号", b)
				pendingOrder = &pendingOrderT{action: ActionSell, slippageSign: -1, signal: "卖出信号", eventIdx: evIdx, triggerBar: i}
			}
		}

		// ---------- 5. 账户市值快照（持仓换算现金，合约乘数自适应） ----------
		positionValue := state.position * b.Close * envRules.multiplier
		equity := state.cash + positionValue
		if state.position > 0 {
			if positionValue > maxInvested {
				maxInvested = positionValue
			}
			investedSum += positionValue
			investedBars++
			investedDays[b.Date] = true
		}

		// 回撤跟踪
		if equity > peakEquity {
			peakEquity = equity
			ddStartTS = b.TS
		} else {
			dd := (peakEquity - equity) / peakEquity * 100
			if dd > maxDD {
				maxDD = dd
				maxDDStart = ddStartTS
				maxDDEnd = b.TS
			}
		}

		// ---------- 6. 报告桶边界：上一桶收尾 ----------
		key := bucketKeyOf(b)
		if curBucket == "" {
			curBucket = key
		} else if key != curBucket {
			last := bars[i-1]
			curEquity := equityAt(last, state, envRules.multiplier)
			dd := 0.0
			if peakEquity > 0 {
				dd = (peakEquity - curEquity) / peakEquity * 100
			}
			equityPoints = append(equityPoints, buildPoint(len(equityPoints)+1, last, state, initialCapital, curEquity, dd, envRules.multiplier))
			periodReturns = append(periodReturns, pctChange(prevBucketEquity, curEquity))
			prevBucketEquity = curEquity
			curBucket = key
		}

		// ---------- 7. 进度 ----------
		if e.onProg != nil && (i%reportEvery == 0 || i == len(bars)-1) {
			e.onProg(i+1, len(bars))
		}
	}

	// 最后一个桶收尾
	if len(bars) > 0 {
		last := bars[len(bars)-1]
		curEquity := equityAt(last, state, envRules.multiplier)
		dd := 0.0
		if peakEquity > 0 {
			dd = (peakEquity - curEquity) / peakEquity * 100
		}
		equityPoints = append(equityPoints, buildPoint(len(equityPoints)+1, last, state, initialCapital, curEquity, dd, envRules.multiplier))
		periodReturns = append(periodReturns, pctChange(prevBucketEquity, curEquity))
	}

	// 回测结束：仍有挂单未执行 → 事件标记 EXPIRED（需求⑨-3 委托结果追踪）
	if pendingOrder != nil {
		ev := &eventTraces[pendingOrder.eventIdx]
		ev.ExecStatus = "EXPIRED"
		ev.RejectReason = "回测结束，委托未执行（后续无 K 线可成交）"
		ev.ExecTS = bars[len(bars)-1].TS
		ev.AliveSec = bars[len(bars)-1].TS - ev.TriggerTS
		pendingOrder = nil
	}

	// 期末总资产（持仓按最后收盘价换算现金）
	finalEquity := equityAt(bars[len(bars)-1], state, envRules.multiplier)

	// Min 精度 + 全量区间的曲线数据量保护：报告点超上限时均匀降采样（汇总指标不受影响）
	equityPoints = downsampleEquity(equityPoints, maxEquityPoints)

	// 汇总报告
	report := buildReport(e.cfg, bars, initialCapital, finalEquity, trades,
		equityPoints, periodReturns, maxDD, maxDDStart, maxDDEnd, maxInvested,
		investedSum, investedBars, investedDays, grossProfit, grossLoss, winCount, lossCount,
		totalFee, signalDetail, eventTraces)

	result.EquityPoints = equityPoints
	result.Trades = trades
	result.Cashflows = cashflows
	result.PositionLogs = positionLogs
	result.EventTraces = eventTraces
	result.Report = report
	result.BarCount = len(bars)
	return result, nil
}

func buyRuleAllowed(def StrategyDefinition) bool  { return def.Rules.Buy.Allow }
func sellRuleAllowed(def StrategyDefinition) bool { return def.Rules.Sell.Allow }

// daysFromDate 将 yyyymmdd 转为自 epoch 起的天序数（跨月/年边界也可做日期差比较）。
func daysFromDate(date int) int64 {
	year := date / 10000
	month := (date / 100) % 100
	day := date % 100
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Unix() / 86400
}

// countWithinWindow 统计日期序列中落在近 N 个自然日（含当前日）内的条数。
func countWithinWindow(dates []int, curDate int, days int) int {
	if days <= 0 || len(dates) == 0 {
		return 0
	}
	bound := daysFromDate(curDate) - int64(days-1)
	n := 0
	for _, d := range dates {
		if daysFromDate(d) >= bound {
			n++
		}
	}
	return n
}

// sumFeeWithinWindow 统计滚动窗口内手续费累计。
func sumFeeWithinWindow(items []feeWinItem, curDate int, days int) float64 {
	if days <= 0 || len(items) == 0 {
		return 0
	}
	bound := daysFromDate(curDate) - int64(days-1)
	var sum float64
	for _, it := range items {
		if daysFromDate(it.date) >= bound {
			sum += it.fee
		}
	}
	return sum
}

// recordRuleTrigger 记录规则触发（当日/全回测/滚动窗口），触发即计数（与 MaxPerDay/MaxPerRun 语义一致）。
func (dc *dayCounters) recordRuleTrigger(key string, date int) {
	dc.ruleTrades[key]++
	dc.ruleRunTrades[key]++
	dc.ruleWeek[key] = append(dc.ruleWeek[key], date)
	dc.ruleMonth[key] = append(dc.ruleMonth[key], date)
}

// windowRejectReason 检查风险级滑动窗口限制（成交笔数周/月、手续费窗口），返回拒绝原因（空=允许）。
func (dc *dayCounters) windowRejectReason(risk RiskDef, curDate int) string {
	if risk.MaxTradesPerWeek > 0 && countWithinWindow(dc.weekDates, curDate, 7) >= risk.MaxTradesPerWeek {
		return fmt.Sprintf("近7日成交已达上限（%d 笔）", risk.MaxTradesPerWeek)
	}
	if risk.MaxTradesPerMonth > 0 && countWithinWindow(dc.monthDates, curDate, 30) >= risk.MaxTradesPerMonth {
		return fmt.Sprintf("近30日成交已达上限（%d 笔）", risk.MaxTradesPerMonth)
	}
	feeDays := risk.FeeWindowDays
	if feeDays <= 0 {
		feeDays = 30
	}
	if risk.MaxFeePerWindow > 0 && sumFeeWithinWindow(dc.feeWindows, curDate, feeDays) >= risk.MaxFeePerWindow {
		return fmt.Sprintf("近%d日手续费已达上限（%.2f）", feeDays, risk.MaxFeePerWindow)
	}
	return ""
}

// ruleRejectReason 检查规则限制并返回拒绝原因（空串=允许）。
// 限制维度：规则每日触发次数 / 规则全回测触发次数 / 全局每日成交笔数 / 最小交易间隔 /
// 滑动窗口（规则级周/月触发次数，风险级周/月成交笔数与手续费窗口）。
func (dc *dayCounters) ruleRejectReason(rule RuleDef, key string, maxTradesPerDay, barIdx int, risk RiskDef) string {
	if rule.MaxPerDay > 0 && dc.ruleTrades[key] >= rule.MaxPerDay {
		return fmt.Sprintf("超过规则每日最大触发次数（%d 次）", rule.MaxPerDay)
	}
	if rule.MaxPerRun > 0 && dc.ruleRunTrades[key] >= rule.MaxPerRun {
		return fmt.Sprintf("超过规则回测总触发次数（%d 次）", rule.MaxPerRun)
	}
	if maxTradesPerDay > 0 && dc.trades >= maxTradesPerDay {
		return fmt.Sprintf("今日已达成交笔数上限（%d 笔），可提高 max_trades_per_day 或在任务级覆盖", maxTradesPerDay)
	}
	if risk.MinIntervalBars > 0 && dc.lastTradeBar >= 0 && barIdx-dc.lastTradeBar < risk.MinIntervalBars {
		return fmt.Sprintf("未满足最小交易间隔（%d bar）", risk.MinIntervalBars)
	}
	// 规则级滑动窗口（分方向，覆盖风险级）
	curDate := dc.curDate
	maxW := rule.MaxPerWeek
	if maxW > 0 && countWithinWindow(dc.ruleWeek[key], curDate, 7) >= maxW {
		return fmt.Sprintf("近7日%s触发已达上限（%d 次）", key, maxW)
	}
	maxM := rule.MaxPerMonth
	if maxM > 0 && countWithinWindow(dc.ruleMonth[key], curDate, 30) >= maxM {
		return fmt.Sprintf("近30日%s触发已达上限（%d 次）", key, maxM)
	}
	if rule.MaxFeePerWindow > 0 {
		feeDays := rule.FeeWindowDays
		if feeDays <= 0 {
			feeDays = 30
		}
		if sumFeeWithinWindow(dc.ruleFees[key], curDate, feeDays) >= rule.MaxFeePerWindow {
			return fmt.Sprintf("近%d日%s手续费已达上限（%.2f）", feeDays, key, rule.MaxFeePerWindow)
		}
	}
	return dc.windowRejectReason(risk, curDate)
}

// buyPositionReject 检查买入方向是否因持仓/建仓进度被拒绝（空串=允许下单）。
//   - 未启用分批建仓：持仓>0 即拒绝（单标的单持仓语义，原逻辑）；
//   - 启用分批建仓：持仓未达目标仓位时允许加仓；已达目标仓位时拒绝。
//   - 相邻批次间隔（tranche_interval_bars）未满足时拒绝。
func (e *Engine) buyPositionReject(b Bar, barIdx int, state *accountState, dc *dayCounters, risk RiskDef) string {
	builder := risk.Builder
	if builder == nil || !builder.Enabled {
		if state.position > 0 {
			return "已达最大持仓（当前版本单标的同时仅允许 1 个持仓）"
		}
		return ""
	}
	if state.position <= 0 {
		// 空仓：允许首笔建仓（间隔限制仅对已有加仓记录生效）
		return ""
	}
	// 相邻批次最小间隔
	if builder.TrancheIntervalBars > 0 && dc.builderBar >= 0 && barIdx-dc.builderBar < builder.TrancheIntervalBars {
		return fmt.Sprintf("未满足分批建仓最小间隔（%d bar）", builder.TrancheIntervalBars)
	}
	// 已达目标仓位：计算目标持仓价值并对比当前持仓价值
	targetPct := builder.TargetPositionPct
	if targetPct <= 0 {
		targetPct = risk.MaxPositionPct
	}
	if targetPct <= 0 {
		targetPct = 100
	}
	multiplier := e.environmentRules().multiplier
	equity := equityAt(b, state, multiplier)
	curVal := state.position * b.Close * multiplier
	targetVal := equity * targetPct / 100
	if curVal >= targetVal {
		return fmt.Sprintf("已达目标持仓上限（当前持仓 %.4f，目标仓位 %.0f%% = %.2f）", state.position, targetPct, targetVal)
	}
	return ""
}

// newEvent 登记一个交易事件（初始 PENDING，含委托下单时间）。返回事件索引。
func (e *Engine) newEvent(eventTraces *[]EventTrace, action, reason string, b Bar) int {
	ev := EventTrace{
		Seq:           len(*eventTraces) + 1,
		Action:        action,
		TriggerReason: reason,
		TriggerTS:     b.TS,
		TriggerDate:   b.Date,
		TriggerTime:   b.Time,
		// ⑤委托下单时间：信号确认后立即下单（NEXT_BAR_OPEN 下挂单至下一 bar 开盘执行）
		OrderTS:     b.TS,
		OrderDate:   b.Date,
		OrderTime:   b.Time,
		ExecStatus:  "PENDING",
		AliveSec:    0,
		RejectReason: "",
	}
	*eventTraces = append(*eventTraces, ev)
	return len(*eventTraces) - 1
}

// recordReject 登记一个被拒绝的交易事件（触发但未成交，登记原因；含下单时间与存活时间）。
func (e *Engine) recordReject(eventTraces *[]EventTrace, action, reason string, b Bar, rejectReason string) {
	ev := EventTrace{
		Seq:           len(*eventTraces) + 1,
		Action:        action,
		TriggerReason: reason,
		TriggerTS:     b.TS,
		TriggerDate:   b.Date,
		TriggerTime:   b.Time,
		OrderTS:       b.TS,
		OrderDate:     b.Date,
		OrderTime:     b.Time,
		ExecStatus:    "REJECTED",
		ExecTS:        b.TS,
		ExecDate:      b.Date,
		ExecTime:      b.Time,
		AliveSec:      0, // 触发即拒绝，存活时间 0
		RejectReason:  rejectReason,
	}
	*eventTraces = append(*eventTraces, ev)
}

// equityAt 计算 bar 收盘口径总资产（合约乘数自适应：持仓市值 = 数量 × 收盘价 × 乘数）。
func equityAt(b Bar, state *accountState, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	return state.cash + state.position*b.Close*multiplier
}

// buildPoint 构造报告点。
func buildPoint(seq int, b Bar, state *accountState, initialCapital float64, equity, dd, multiplier float64) EquityPoint {
	if multiplier <= 0 {
		multiplier = 1
	}
	positionValue := state.position * b.Close * multiplier
	profit := equity - initialCapital
	roi := 0.0
	if initialCapital > 0 {
		roi = profit / initialCapital * 100
	}
	return EquityPoint{
		Seq:           seq,
		TS:            b.TS,
		Date:          b.Date,
		Time:          b.Time,
		Equity:        round6(equity),
		Cash:          round6(state.cash),
		PositionValue: round6(positionValue),
		PositionQty:   round6(state.position),
		Profit:        round6(profit),
		ROI:           round6(roi),
		Drawdown:      round6(dd),
	}
}

// pendingOrderT 挂单（下一 bar 开盘成交）。
type pendingOrderT struct {
	action       string
	slippageSign float64
	signal       string
	eventIdx     int // 关联事件追踪索引（更新 FILLED/REJECTED 状态）
	triggerBar   int // 触发 bar 索引（计算委托耗时）
}

// fill 执行一笔成交（买入/卖出），更新账户、成交记录、资金流水与持仓明细。
// 返回引擎内成交序号（tradeSeq，明细表 trade_id 关联用）、是否成交与未成交原因（供事件追踪登记）。
func (e *Engine) fill(action string, price float64, b Bar, barIdx int, state *accountState, dc *dayCounters,
	trades *[]Trade, cashflows *[]Cashflow, positionLogs *[]PositionLog,
	commission float64, totalFee *float64, signalDetail map[string]int,
	grossProfit, grossLoss *float64, winCount, lossCount *int, signal string,
	maxTradesPerDay int, limitPrices []barLimit) (int64, bool, string) {

	def := e.cfg.Definition
	acc := e.cfg.Account

	// 环境 tick_size 价格对齐（最小变动单位，自适应不同产品）
	if e.cfg.Environment != nil && e.cfg.Environment.Config.TradingRules != nil {
		if ts := e.cfg.Environment.Config.TradingRules.TickSize; ts > 0 {
			price = math.Round(price/ts) * ts
		}
	}

	// 涨跌停限制（基于前一交易日收盘价，环境 trading_rules.limit_up_pct/limit_down_pct）
	if barIdx >= 0 && barIdx < len(limitPrices) {
		if action == ActionBuy && limitPrices[barIdx].up > 0 && price >= limitPrices[barIdx].up {
			return 0, false, "涨停限制（价格触及涨停价，无法买入）"
		}
		if action == ActionSell && limitPrices[barIdx].down > 0 && price <= limitPrices[barIdx].down {
			return 0, false, "跌停限制（价格触及跌停价，无法卖出）"
		}
	}

	// 跨日重置每日计数
	if dc.curDate != b.Date {
		dc.curDate = b.Date
		dc.trades = 0
		dc.ruleTrades = map[string]int{}
	}
	// 全局每日笔数限制
	if maxTradesPerDay > 0 && dc.trades >= maxTradesPerDay {
		return 0, false, fmt.Sprintf("超过每日最大成交笔数（%d 笔）", maxTradesPerDay)
	}
	// 相邻交易最小间隔
	if def.Risk.MinIntervalBars > 0 && dc.lastTradeBar >= 0 && barIdx-dc.lastTradeBar < def.Risk.MinIntervalBars {
		return 0, false, fmt.Sprintf("未满足最小交易间隔（%d bar）", def.Risk.MinIntervalBars)
	}

	marginRate := acc.MarginRate
	if marginRate <= 0 {
		marginRate = 1
	}
	// 合约乘数（环境 trading_rules.contract_multiplier，股票/ETF 默认 1；期货如 COMEX 黄金=100）
	multiplier := e.environmentRules().multiplier

	qty := 0.0
	profit := 0.0
	fee := 0.0
	amount := 0.0
	avgCostBefore := state.avgCost
	posBefore := state.position
	seq := int64(len(*trades) + 1) // 引擎内成交序号（明细表 trade_id 关联用）

	switch action {
	case ActionBuy:
		// 分批建仓：每批按「目标仓位/份数」固定档位推进（未达目标时分批加仓）；
		// 未启用则保持原数量模式（满仓/百分比等）。
		if bd, ok := e.builderDef(); ok && bd.Enabled {
			qty = e.builderAddQty(bd, state, price, def.Risk, commission, marginRate, multiplier)
			if qty <= 0 {
				return 0, false, "已达目标持仓上限（分批建仓目标仓位已达成）"
			}
		} else {
			qty = e.buyQuantity(b, state, price, def.Rules.Buy, commission, marginRate, multiplier)
		}
		if qty <= 0 {
			return 0, false, "资金不足（可用资金无法覆盖委托金额）"
		}
		amount = qty * price * multiplier
		fee = amount * commission
		// 资金流水：扣款 + 手续费（FUTURES 模式为保证金占用）
		cashBeforePay := state.cash
		if acc.MarginMode == "FUTURES" {
			hold := amount * marginRate
			state.cash -= hold + fee
			appendCashflow(cashflows, b, "MARGIN_HOLD", -hold, cashBeforePay, state.cash+fee, seq, signal)
			appendCashflow(cashflows, b, "FEE", -fee, state.cash+fee, state.cash, seq, signal)
		} else {
			state.cash -= amount + fee
			appendCashflow(cashflows, b, "BUY_PAY", -amount, cashBeforePay, state.cash+fee, seq, signal)
			appendCashflow(cashflows, b, "FEE", -fee, state.cash+fee, state.cash, seq, signal)
		}
		newPos := state.position + qty
		state.avgCost = (state.position*state.avgCost + qty*price) / newPos
		state.position = newPos
		state.entryDate = b.Date
		dc.builderBar = barIdx
		dc.builderDate = b.Date
		// 空仓开新仓时重置分批减仓次数（配合 ReduceTranches）
		if posBefore <= 0 && def.Risk.ReduceTranches > 1 {
			dc.reduceRemain = def.Risk.ReduceTranches
		}
	case ActionSell:
		// 分批减仓：每批卖出 持仓/剩余次数，最后一批清仓；未启用则按原数量模式（清仓/百分比等）
		if def.Risk.ReduceTranches > 1 {
			if dc.reduceRemain <= 0 {
				dc.reduceRemain = def.Risk.ReduceTranches
			}
			qty = e.reduceQty(state, price, dc.reduceRemain, multiplier)
			dc.reduceRemain--
		} else {
			qty = e.sellQuantity(b, state, price, def.Rules.Sell, multiplier)
		}
		if qty <= 0 {
			return 0, false, "当前无持仓，卖出信号无法执行（空仓状态）"
		}
		amount = qty * price * multiplier
		fee = amount * commission
		cashBeforeRecv := state.cash
		if acc.MarginMode == "FUTURES" {
			release := amount * marginRate
			state.cash += release - fee
			appendCashflow(cashflows, b, "MARGIN_RELEASE", release, cashBeforeRecv, state.cash+fee, seq, signal)
			appendCashflow(cashflows, b, "FEE", -fee, state.cash+fee, state.cash, seq, signal)
		} else {
			state.cash += amount - fee
			appendCashflow(cashflows, b, "SELL_RECEIVE", amount, cashBeforeRecv, state.cash+fee, seq, signal)
			appendCashflow(cashflows, b, "FEE", -fee, state.cash+fee, state.cash, seq, signal)
		}
		profit = (price - state.avgCost) * qty * multiplier
		if profit > 0 {
			*grossProfit += profit
			*winCount++
		} else if profit < 0 {
			*grossLoss += -profit
			*lossCount++
		}
		if signalDetail != nil {
			signalDetail[signal]++
		}
		state.position -= qty
		if state.position < 1e-9 {
			state.position = 0
			state.avgCost = 0
		}
	}

	*totalFee += fee
	// 滑动窗口记录（成交级）：风险级周/月成交日期、手续费窗口（按自然日滚动）
	dc.weekDates = append(dc.weekDates, b.Date)
	dc.monthDates = append(dc.monthDates, b.Date)
	dc.feeWindows = append(dc.feeWindows, feeWinItem{date: b.Date, fee: fee})
	// 规则级手续费窗口（分方向）：规则触发已由 recordRuleTrigger 记入 ruleWeek/ruleMonth，
	// 但手续费只在成交时产生，故在此按方向补充记录（供 rules.max_fee_per_window 生效）
	key := ""
	if action == ActionBuy {
		key = "buy"
	} else {
		key = "sell"
	}
	dc.ruleFees[key] = append(dc.ruleFees[key], feeWinItem{date: b.Date, fee: fee})
	trade := Trade{
		TS:            b.TS,
		Date:          b.Date,
		Time:          b.Time,
		Action:        action,
		Price:         round6(price),
		Qty:           round6(qty),
		Amount:        round6(amount),
		Fee:           round6(fee),
		Profit:        round6(profit),
		PositionAfter: round6(state.position),
		CashAfter:     round6(state.cash),
		Signal:        signal,
		Remark:        signal, // 备注列与触发原因一致（评审：V26 remark 列需有值）
		Seq:           int(seq),
	}
	*trades = append(*trades, trade)
	dc.trades++

	// 持仓变化明细（需求⑨-2）：开仓/加仓/减仓/平仓
	posAction := "OPEN"
	if action == ActionBuy {
		if posBefore > 0 {
			posAction = "ADD"
		}
	} else {
		if state.position > 0 {
			posAction = "REDUCE"
		} else {
			posAction = "CLOSE"
		}
	}
	*positionLogs = append(*positionLogs, PositionLog{
		Seq:            len(*positionLogs) + 1,
		TS:             b.TS,
		Date:           b.Date,
		Time:           b.Time,
		Action:         posAction,
		Price:          round6(price),
		Qty:            round6(qty),
		PositionBefore: round6(posBefore),
		PositionAfter:  round6(state.position),
		AvgCostBefore:  round6(avgCostBefore),
		AvgCostAfter:   round6(state.avgCost),
		TradeSeq:       int(seq),
		Remark:         signal,
	})
	return seq, true, ""
}

// appendCashflow 追加资金流水（需求⑨-1）。
func appendCashflow(cashflows *[]Cashflow, b Bar, flowType string, amount, cashBefore, cashAfter float64, tradeSeq int64, remark string) {
	*cashflows = append(*cashflows, Cashflow{
		Seq:        len(*cashflows) + 1,
		TS:         b.TS,
		Date:       b.Date,
		Time:       b.Time,
		FlowType:   flowType,
		Amount:     round6(amount),
		CashBefore: round6(cashBefore),
		CashAfter:  round6(cashAfter),
		TradeSeq:   int(tradeSeq),
		Remark:     remark,
	})
}

// buyQuantity 计算买入数量（合约乘数自适应：可用资金 / (价格 × 乘数 × 保证金率)）。
func (e *Engine) buyQuantity(b Bar, state *accountState, price float64, rule RuleDef, commission, marginRate, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	ruleType := rule.QuantityType
	if ruleType == "" || ruleType == "ALL" {
		// ALL 为清仓语义（仅卖出方向）；买入侧缺省/误用均按全额买入处理
		ruleType = "ALL_IN"
	}
	available := state.cash
	if commission > 0 {
		available = state.cash / (1 + commission)
	}
	switch ruleType {
	case "PERCENT":
		pct := rule.Quantity
		if pct <= 0 || pct > 100 {
			pct = 100
		}
		return floorQty(available * pct / 100 / (price * multiplier * marginRate))
	case "AMOUNT":
		amt := rule.Quantity
		if amt <= 0 {
			amt = available
		}
		if amt > available {
			amt = available
		}
		return floorQty(amt / (price * multiplier * marginRate))
	case "FIXED":
		qty := rule.Quantity
		if qty <= 0 {
			return 0
		}
		if qty*price*multiplier*marginRate > state.cash {
			qty = floorQty(available / (price * multiplier * marginRate))
		}
		return floorQty(qty)
	default: // ALL_IN
		return floorQty(available / (price * multiplier * marginRate))
	}
}

// sellQuantity 计算卖出数量（FR-4 数量模式在卖出方向同样生效，缺省清仓 ALL）。
func (e *Engine) sellQuantity(b Bar, state *accountState, price float64, rule RuleDef, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	ruleType := rule.QuantityType
	if ruleType == "" || ruleType == "ALL_IN" {
		// ALL_IN 为全额买入语义；卖出侧缺省/误用均按清仓处理
		ruleType = "ALL"
	}
	pos := state.position
	if pos <= 0 {
		return 0
	}
	switch ruleType {
	case "FIXED":
		qty := rule.Quantity
		if qty <= 0 {
			return 0
		}
		if qty > pos {
			qty = pos
		}
		return floorQty(qty)
	case "PERCENT":
		pct := rule.Quantity
		if pct <= 0 || pct > 100 {
			pct = 100
		}
		return floorQty(pos * pct / 100)
	case "AMOUNT":
		amt := rule.Quantity
		if amt <= 0 {
			amt = pos * price * multiplier
		}
		qty := amt / (price * multiplier)
		if qty > pos {
			qty = pos
		}
		if qty <= 0 {
			return 0
		}
		return floorQty(qty)
	default: // ALL 清仓
		return pos
	}
}

// builderDef 返回启用的分批建仓配置（nil 表示未启用）。
func (e *Engine) builderDef() (*BuilderDef, bool) {
	b := e.cfg.Definition.Risk.Builder
	if b == nil || !b.Enabled {
		return nil, false
	}
	return b, true
}

// builderAddQty 计算分批建仓数量：以「目标仓位价值 / 份数」作为每档固定买入额，
// 买入后当前持仓价值不得超过目标仓位。返回本次建仓数量。
func (e *Engine) builderAddQty(b *BuilderDef, state *accountState, price float64, risk RiskDef,
	commission, marginRate, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	targetPct := b.TargetPositionPct
	if targetPct <= 0 {
		targetPct = risk.MaxPositionPct
	}
	if targetPct <= 0 {
		targetPct = 100
	}
	tranches := b.Tranches
	if tranches < 1 {
		tranches = 1
	}
	// 当前净资产（现金 + 当前持仓市值）；买价近似现价，直接用 price 计算
	curVal := state.position * price * multiplier
	equity := state.cash + curVal
	targetVal := equity * targetPct / 100
	// 每档固定金额
	trancheVal := targetVal / float64(tranches)
	// 若当前持仓已接近目标，仅买入差额（避免超配）
	want := trancheVal
	if targetVal-curVal < want {
		want = targetVal - curVal
	}
	if want <= 0 {
		return 0
	}
	// 可用资金封顶（含手续费预留）
	available := state.cash
	if commission > 0 {
		available = state.cash / (1 + commission)
	}
	qty := want / (price * multiplier * marginRate)
	maxQty := floorQty(available / (price * multiplier * marginRate))
	if qty > maxQty {
		qty = maxQty
	}
	return floorQty(qty)
}

// reduceQty 计算分批减仓数量：每批卖出 持仓/剩余次数（最后一笔清仓，余数由最后一次全出）。返回本次卖出数量。
func (e *Engine) reduceQty(state *accountState, price float64, remain int, multiplier float64) float64 {
	pos := state.position
	if pos <= 0 || remain <= 1 {
		return pos
	}
	per := floorQty(pos / float64(remain))
	if per <= 0 {
		return pos
	}
	return per
}

// floorQty 数量向下取整（保留 6 位小数，兼容股票整数股/期货手数）。
func floorQty(q float64) float64 {
	return math.Floor(q*1e6) / 1e6
}

// profitFactorInfSentinel 全盈利无亏损时盈亏比的大数哨兵（round6 会把 +Inf 归零，前端失真）。
const profitFactorInfSentinel = 999999

// round6 保留 6 位小数。
func round6(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*1e6) / 1e6
}

// pctChange 计算两个净值间的收益率 %。
func pctChange(prev, cur float64) float64 {
	if prev == 0 {
		return 0
	}
	return (cur - prev) / prev * 100
}

// buildFieldSeries 构建字段序列。
func buildFieldSeries(bars []Bar) map[string][]float64 {
	n := len(bars)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	closeV := make([]float64, n)
	volume := make([]float64, n)
	turnover := make([]float64, n)
	for i, b := range bars {
		open[i], high[i], low[i], closeV[i] = b.Open, b.High, b.Low, b.Close
		volume[i], turnover[i] = b.Volume, b.Turnover
	}
	return map[string][]float64{
		"open": open, "high": high, "low": low, "close": closeV,
		"volume": volume, "turnover": turnover,
	}
}

// AggregateBars 将分钟 bar 聚合为指定周期（Min 原样返回）。
// period: Min/Hour/Day。Hour 按 (date, hour) 聚合，Day 按 date 聚合。
func AggregateBars(bars []Bar, period string) ([]Bar, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("无行情数据")
	}
	switch period {
	case "", PeriodMin:
		return bars, nil
	case PeriodHour, PeriodDay:
		sorted := make([]Bar, len(bars))
		copy(sorted, bars)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].TS < sorted[j].TS })
		var out []Bar
		for _, b := range sorted {
			key := aggKeyOf(b, period)
			if len(out) > 0 && aggKeyOf(out[len(out)-1], period) == key {
				last := &out[len(out)-1]
				if b.High > last.High {
					last.High = b.High
				}
				if b.Low < last.Low {
					last.Low = b.Low
				}
				last.Close = b.Close
				last.TS = b.TS // 时间轴取周期结束（净值曲线时间点）
				// last.Time 保持周期首根 bar 时间：交易时段判定用周期起点，
				// 避免 Hour/Day 聚合 bar 因取最后一根分钟 bar 时间而误判非交易时段
				last.Volume += b.Volume
				last.Turnover += b.Turnover
			} else {
				out = append(out, b)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("不支持的周期 %q", period)
	}
}

func aggKeyOf(b Bar, period string) int {
	if period == PeriodHour {
		return b.Date*100 + b.Time/10000
	}
	return b.Date
}

// ---------------------------------------------------------------------
// 报告构建
// ---------------------------------------------------------------------

func buildReport(cfg EngineConfig, bars []Bar, initialCapital, finalEquity float64,
	trades []Trade, equityPoints []EquityPoint, periodReturns []float64,
	maxDD float64, maxDDStart, maxDDEnd int64, maxInvested, investedSum float64, investedBars float64,
	investedDays map[int]bool, grossProfit, grossLoss float64, winCount, lossCount int,
	totalFee float64, signalDetail map[string]int, eventTraces []EventTrace) *RunReport {

	totalProfit := finalEquity - initialCapital
	totalReturnPct := 0.0
	if initialCapital > 0 {
		totalReturnPct = totalProfit / initialCapital * 100
	}

	// 年化（按自然日）
	annualReturnPct := 0.0
	if len(bars) >= 2 {
		days := (bars[len(bars)-1].TS - bars[0].TS) / 86400
		if days < 1 {
			days = 1
		}
		if initialCapital > 0 && finalEquity > 0 {
			annualReturnPct = (math.Pow(finalEquity/initialCapital, 365.0/float64(days)) - 1) * 100
		}
	}

	// 夏普 / 波动率
	sharpe, volPct := computeSharpeVol(periodReturns, cfg.ReportPrecision)

	// 交易统计
	tradeCount := len(trades)
	buyCount, sellCount := 0, 0
	for _, t := range trades {
		if t.Action == ActionBuy {
			buyCount++
		} else {
			sellCount++
		}
	}
	winRatePct := 0.0
	closed := winCount + lossCount
	if closed > 0 {
		winRatePct = float64(winCount) / float64(closed) * 100
	}
	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		// 全盈利无亏损：盈亏比 = +Inf。round6 会将 +Inf 归零导致前端显示失真，
		// 改用大数哨兵（999999）表示“无穷”，前端可显示为 ∞。
		profitFactor = profitFactorInfSentinel
	}

	avgInvested := 0.0
	if investedBars > 0 {
		avgInvested = investedSum / investedBars
	}

	bestDay, worstDay := 0.0, 0.0
	profitDays, lossDays := 0, 0
	for _, r := range periodReturns {
		if r > bestDay {
			bestDay = r
		}
		if r < worstDay {
			worstDay = r
		}
		if r > 0 {
			profitDays++
		} else if r < 0 {
			lossDays++
		}
	}

	// 事件追踪统计（需求⑨-3）
	eventStats := buildEventStats(eventTraces)

	return &RunReport{
		SecuCode:           cfg.SecuCode,
		Period:             cfg.Period,
		ReportPrecision:    cfg.ReportPrecision,
		StartDate:          firstDate(bars),
		EndDate:            lastDate(bars),
		BarCount:           len(bars),
		InitialCapital:     round6(initialCapital),
		FinalEquity:        round6(finalEquity),
		TotalProfit:        round6(totalProfit),
		TotalReturnPct:     round6(totalReturnPct),
		AnnualReturnPct:    round6(annualReturnPct),
		MaxDrawdownPct:     round6(maxDD),
		MaxDrawdownStartTS: maxDDStart,
		MaxDrawdownEndTS:   maxDDEnd,
		SharpeRatio:        round6(sharpe),
		VolatilityPct:      round6(volPct),
		TradeCount:         tradeCount,
		BuyCount:           buyCount,
		SellCount:          sellCount,
		WinCount:           winCount,
		LossCount:          lossCount,
		WinRatePct:         round6(winRatePct),
		ProfitFactor:       round6(profitFactor),
		TotalFee:           round6(totalFee),
		MaxInvested:        round6(maxInvested),
		AvgInvested:        round6(avgInvested),
		InvestedDays:       len(investedDays),
		BestDayPct:         round6(bestDay),
		WorstDayPct:        round6(worstDay),
		ProfitDays:         profitDays,
		LossDays:           lossDays,
		TradeSignalDetail:  signalDetail,
		EventStats:         eventStats,
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
	}
}

// buildEventStats 汇总交易事件统计（需求⑨-3）。
func buildEventStats(eventTraces []EventTrace) *EventStats {
	if len(eventTraces) == 0 {
		return nil
	}
	es := &EventStats{
		RejectReasons:  map[string]int{},
		TriggerReasons: map[string]int{},
	}
	var latencyBarsSum, latencySecSum int
	filled := 0
	for _, ev := range eventTraces {
		es.TriggerReasons[ev.TriggerReason]++
		es.TriggerCount++
		switch ev.ExecStatus {
		case "FILLED":
			es.FilledCount++
			// 平均委托耗时只统计信号驱动订单（买入/卖出信号）：止损/止盈为 intrabar
			// 限价单语义（latency=0），计入会拉低均值造成失真（评审意见）。
			if ev.TriggerReason == "买入信号" || ev.TriggerReason == "卖出信号" {
				filled++
				latencyBarsSum += ev.LatencyBars
				latencySecSum += int(ev.LatencySec)
			}
		case "REJECTED":
			es.RejectedCount++
			es.RejectReasons[ev.RejectReason]++
		case "EXPIRED":
			es.ExpiredCount++
		case "PENDING":
			es.PendingCount++
		}
	}
	if filled > 0 {
		es.AvgLatencyBars = round6(float64(latencyBarsSum) / float64(filled))
		es.AvgLatencySec = round6(float64(latencySecSum) / float64(filled))
	}
	return es
}

func computeSharpeVol(periodReturns []float64, precision string) (float64, float64) {
	if len(periodReturns) < 2 {
		return 0, 0
	}
	var sum float64
	for _, r := range periodReturns {
		sum += r
	}
	mean := sum / float64(len(periodReturns))
	var variance float64
	for _, r := range periodReturns {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(periodReturns) - 1)
	std := math.Sqrt(variance)
	if std == 0 {
		return 0, 0
	}
	periodsPerYear := 252.0
	switch precision {
	case PeriodMin:
		periodsPerYear = 252 * 240
	case PeriodHour:
		periodsPerYear = 252 * 6
	}
	sharpe := mean / std * math.Sqrt(periodsPerYear)
	volPct := std * math.Sqrt(periodsPerYear)
	return sharpe, volPct
}

func firstDate(bars []Bar) int {
	if len(bars) == 0 {
		return 0
	}
	return bars[0].Date
}

func lastDate(bars []Bar) int {
	if len(bars) == 0 {
		return 0
	}
	return bars[len(bars)-1].Date
}
