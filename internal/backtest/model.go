// Package backtest 提供通用量化回测服务：
//   - 策略定义（结构化 JSON 模型，可持久化、可扩展，覆盖 ETF/股票/场外基金/
//     国内期货/美股期货/黄金/石油等商品期货等任意已导入行情的证券）；
//   - 回测账户（初始资金/手续费/滑点/保证金模式）；
//   - 回测任务（策略+账户+标的+区间快照，异步执行、进度/状态持久化）；
//   - 回测引擎（逐 bar 回放：指标计算 → 信号求值 → 规则限制 → 撮合 → 账户更新）；
//   - 回测报告（汇总指标 + 按报告精度的余额/收益率/收益额/持仓金额曲线数据）。
package backtest

import (
	"time"
)

// ---------------------------------------------------------------------
// 常量
// ---------------------------------------------------------------------

// 状态常量
const (
	StatusDraft    = "DRAFT"
	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"
	FlagOn         = "1"
	FlagOff        = "0"

	RunPending   = "PENDING"
	RunRunning   = "RUNNING"
	RunSucceeded = "SUCCEEDED"
	RunFailed    = "FAILED"
	RunCancelled = "CANCELLED"

	// 回测任务删除任务状态
	DelPending   = "PENDING"
	DelRunning   = "RUNNING"
	DelSucceeded = "SUCCEEDED"
	DelFailed    = "FAILED"

	PeriodMin  = "Min"
	PeriodHour = "Hour"
	PeriodDay  = "Day"

	ActionBuy  = "BUY"
	ActionSell = "SELL"

	StrategyTypeRuleBased = "RULE_BASED"

	// 撮合模式
	FillNextBarOpen  = "NEXT_BAR_OPEN" // 信号在 bar 收盘确认，下一 bar 开盘价成交（默认，无未来函数）
	FillCurrentClose = "CURRENT_CLOSE" // 当前 bar 收盘价成交（近似，可能有轻微未来偏差）
)

// Pager 分页参数（复用元数据模块语义）。
type Pager struct {
	Page     int
	PageSize int
}

// Normalize 归一化分页参数。
func (p *Pager) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 500 {
		p.PageSize = 500
	}
}

// ---------------------------------------------------------------------
// 策略定义（通用可扩展结构化模型，definition JSONB）
// ---------------------------------------------------------------------

// Strategy 回测策略定义表行。
type Strategy struct {
	StrategyID        string             `json:"strategy_id"`
	StrategyCode      string             `json:"strategy_code"`
	StrategyName      string             `json:"strategy_name"`
	StrategyType      string             `json:"strategy_type"`
	Description       string             `json:"description"`
	Definition        StrategyDefinition `json:"definition"`
	DefinitionVersion int                `json:"definition_version"`
	DataPeriod        string             `json:"data_period"`
	SecuCode          string             `json:"secu_code"`
	UserID            string             `json:"user_id"`     // 所属用户（多用户隔离）
	TemplateID        *string            `json:"template_id"` // 来源模板
	AllowBacktest     string             `json:"allow_backtest"`
	Status            string             `json:"status"`
	CreatedBy         string             `json:"created_by"`
	GMTUpdate         time.Time          `json:"gmt_update"`
}

// StrategyDefinition 结构化策略定义（JSON 模型 v1）。
//
// 设计目标：通用性强、可扩展、支持高度自定义 ——
//   - universe：标的池（可多标的，初始引擎单标的回放，多标的留扩展）；
//   - data：数据与撮合配置（周期/价格字段/预热/成交模式）；
//   - indicators：指标管线（id + 类型 + 参数，引擎按声明顺序计算）；
//   - signals：买卖信号（布尔表达式，引用指标 id 与内置函数）；
//   - rules：交易规则（动作 + 数量模式 + 限制：交易时间点/频率/次数/开关）；
//   - risk：风控（止损/止盈/仓位上限/最大持仓数/每日最大笔数/最小间隔）；
//   - cost：成本覆盖（手续费/滑点，未设置时回退账户配置）。
type StrategyDefinition struct {
	Version      string         `json:"version"`       // 模型版本（当前 "1"）
	StrategyType string         `json:"strategy_type"` // 冗余（与表列一致）
	Description  string         `json:"description"`
	Universe     UniverseDef    `json:"universe"`
	Data         DataDef        `json:"data"`
	Indicators   []IndicatorDef `json:"indicators"`
	Signals      SignalsDef     `json:"signals"`
	Rules        RulesDef       `json:"rules"`
	Risk         RiskDef        `json:"risk"`
	Cost         *CostDef       `json:"cost,omitempty"`
}

// UniverseDef 标的池定义。
type UniverseDef struct {
	Securities []string `json:"securities"` // 证券代码列表（如 ["GCMain"]）
}

// DataDef 数据与撮合配置。
type DataDef struct {
	Period     string `json:"period"`      // Min/Hour/Day
	PriceField string `json:"price_field"` // 指标默认取值字段（close/open/high/low）
	WarmupBars int    `json:"warmup_bars"` // 预热 bar 数（指标未就绪前不产生信号）
	FillMode   string `json:"fill_mode"`   // NEXT_BAR_OPEN / CURRENT_CLOSE
}

// IndicatorDef 指标定义。
type IndicatorDef struct {
	ID     string         `json:"id"`     // 指标标识（信号表达式引用名）
	Type   string         `json:"type"`   // MA/EMA/RSI/MACD/BOLL/ATR/STDDEV/HHV/LLV...
	Params map[string]any `json:"params"` // 指标参数（window/field 等）
}

// SignalsDef 信号定义（布尔表达式）。
type SignalsDef struct {
	Buy  string `json:"buy"`  // 买入信号表达式（如 cross_up(ma_fast, ma_slow) AND rsi14 < 70）
	Sell string `json:"sell"` // 卖出信号表达式
}

// RulesDef 交易规则定义。
type RulesDef struct {
	Buy  RuleDef `json:"buy"`  // 买入规则
	Sell RuleDef `json:"sell"` // 卖出规则
}

// RuleDef 单方向规则：动作 + 数量模式 + 交易限制。
type RuleDef struct {
	Action       string   `json:"action"`        // BUY/SELL（与方向一致）
	QuantityType string   `json:"quantity_type"` // ALL_IN（全部可用资金）/ ALL（清仓）/ FIXED（固定数量）/ PERCENT（可用资金百分比）/ AMOUNT（固定金额）
	Quantity     float64  `json:"quantity"`      // FIXED 数量 / PERCENT 百分比 / AMOUNT 金额
	MaxPerDay    int      `json:"max_per_day"`   // 每日最大触发次数（0=不限制）
	AllowedTimes []string `json:"allowed_times"` // 限定交易时间点（hhmmss 字符串，空=不限制）
	MaxPerRun    int      `json:"max_per_run"`   // 整个回测最大触发次数（0=不限制）
	Allow        bool     `json:"allow"`         // 规则开关（false=禁用该方向交易）
	// 滑动窗口限制（分方向，滚动 N 自然日；覆盖 risk 全局对应项）
	MaxPerWeek     int     `json:"max_per_week"`      // 滚动 7 自然日最大触发次数（0=不限制，缺省跟随 risk）
	MaxPerMonth    int     `json:"max_per_month"`     // 滚动 30 自然日最大触发次数（0=不限制，缺省跟随 risk）
	MaxFeePerWindow float64 `json:"max_fee_per_window"` // 滚动窗口内手续费上限（0=不限制，缺省跟随 risk）
	FeeWindowDays  int     `json:"fee_window_days"`   // 费用窗口天数（0=缺省 30）
}

// BuilderDef 分批建仓（动态建仓策略）：买入信号未达目标仓位时分批加仓，而非满仓或拒绝。
type BuilderDef struct {
	Enabled             bool    `json:"enabled"`               // 是否启用分批建仓
	TargetPositionPct   float64 `json:"target_position_pct"`   // 目标仓位占净资产 %（0=缺省 MaxPositionPct）
	Tranches            int     `json:"tranches"`              // 分几批建仓（>=1，每批 = 目标仓位/份数）
	TrancheIntervalBars int     `json:"tranche_interval_bars"` // 相邻批次最小间隔 bar 数（0=不限制）
}

// RiskDef 风控定义。
type RiskDef struct {
	StopLossPct     float64 `json:"stop_loss_pct"`      // 止损（相对持仓成本，%）
	TakeProfitPct   float64 `json:"take_profit_pct"`    // 止盈（相对持仓成本，%）
	MaxPositionPct  float64 `json:"max_position_pct"`   // 单标的仓位上限（占净资产 %，默认 100）
	MaxPositions    int     `json:"max_positions"`      // 最大持仓数（单标的回测恒为 0/1）
	MaxTradesPerDay int     `json:"max_trades_per_day"` // 每日最大成交笔数（0=不限制）
	MinIntervalBars int     `json:"min_interval_bars"`  // 相邻交易最小间隔 bar 数（0=不限制）
	// 分批建仓 / 分批减仓（动态建仓策略）
	Builder        *BuilderDef `json:"builder,omitempty"` // 分批建仓配置（nil=不启用，保持满仓/清仓语义）
	ReduceTranches int         `json:"reduce_tranches"`   // 卖出分批减仓份数（>=1；每批卖出 持仓/份数，最后一批清仓）
	// 滑动窗口交易限制（全局，滚动 N 自然日；分方向可在 rules 覆盖）
	MaxTradesPerWeek  int     `json:"max_trades_per_week"`   // 滚动 7 自然日成交笔数上限（0=不限制）
	MaxTradesPerMonth int     `json:"max_trades_per_month"`  // 滚动 30 自然日成交笔数上限（0=不限制）
	MaxFeePerWindow   float64 `json:"max_fee_per_window"`    // 滚动窗口内手续费上限（0=不限制）
	FeeWindowDays     int     `json:"fee_window_days"`       // 费用窗口天数（0=缺省 30）
}

// CostDef 成本覆盖（任务级优先于账户）。
type CostDef struct {
	CommissionRate float64 `json:"commission_rate"` // 手续费率（按成交金额比例）
	SlippagePct    float64 `json:"slippage_pct"`    // 滑点（按成交价比例）
}

// ---------------------------------------------------------------------
// 回测账户
// ---------------------------------------------------------------------

// Account 回测账户表行。
type Account struct {
	AccountID      string    `json:"account_id"`
	AccountCode    string    `json:"account_code"`
	AccountName    string    `json:"account_name"`
	UserID         string    `json:"user_id"`  // 所属用户（多用户隔离）
	GroupID        *string   `json:"group_id"` // 子账户分组/主账户归属（单用户多子账户）
	EnvID          *string   `json:"env_id"`   // 默认关联环境
	InitialCapital float64   `json:"initial_capital"`
	CurrencyType   string    `json:"currency_type"`
	CommissionRate float64   `json:"commission_rate"`
	SlippagePct    float64   `json:"slippage_pct"`
	MarginMode     string    `json:"margin_mode"`
	MarginRate     float64   `json:"margin_rate"`
	AllowBacktest  string    `json:"allow_backtest"`
	Status         string    `json:"status"`
	Remark         string    `json:"remark"`
	CreatedBy      string    `json:"created_by"`
	GMTUpdate      time.Time `json:"gmt_update"`
}

// AccountSnapshot 账户快照（写入回测任务）。
type AccountSnapshot struct {
	AccountID      string  `json:"account_id"`
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	UserID         string  `json:"user_id"`
	GroupID        *string `json:"group_id"`
	InitialCapital float64 `json:"initial_capital"`
	CurrencyType   string  `json:"currency_type"`
	CommissionRate float64 `json:"commission_rate"`
	SlippagePct    float64 `json:"slippage_pct"`
	MarginMode     string  `json:"margin_mode"`
	MarginRate     float64 `json:"margin_rate"`
}

// ---------------------------------------------------------------------
// 回测任务
// ---------------------------------------------------------------------

// Run 回测任务表行。
type Run struct {
	RunID               string          `json:"run_id"`
	RunNo               int64           `json:"run_no"`
	UserID              string          `json:"user_id"` // 所属用户（多用户隔离）
	StrategyID          string          `json:"strategy_id"`
	StrategyCode        string          `json:"strategy_code"`
	StrategyName        string          `json:"strategy_name"`
	StrategySnapshot    map[string]any  `json:"strategy_snapshot"`
	AccountID           string          `json:"account_id"`
	AccountCode         string          `json:"account_code"`
	AccountName         string          `json:"account_name"`
	AccountSnapshot     AccountSnapshot `json:"account_snapshot"`
	EnvID               string          `json:"env_id"`                         // 关联环境
	EnvironmentSnapshot *Environment    `json:"environment_snapshot,omitempty"` // 环境配置快照（保证任务可复现）
	SecuCode            string          `json:"secu_code"`
	MarketCode          int             `json:"market_code"`
	Period              string          `json:"period"`
	ReportPrecision     string          `json:"report_precision"`
	StartTS             int64           `json:"start_ts"`
	EndTS               int64           `json:"end_ts"`
	StartDate           int             `json:"start_date"`
	EndDate             int             `json:"end_date"`
	Options             map[string]any  `json:"options"`
	Status              string          `json:"status"`
	Progress            int             `json:"progress"`
	ErrorMessage        string          `json:"error_message"`
	Report              *RunReport      `json:"report,omitempty"`
	StartedAt           *time.Time      `json:"started_at"`
	FinishedAt          *time.Time      `json:"finished_at"`
	CreatedBy           string          `json:"created_by"`
	GMTUpdate           time.Time       `json:"gmt_update"`
}

// RunOptions 回测运行配置（options JSONB）。
type RunOptions struct {
	EnableBacktest  bool     `json:"enable_backtest"`              // 回测开关（false 拒绝启动）
	ReportPrecision string   `json:"report_precision"`             // 报告时间精度（冗余）
	InitialCapital  *float64 `json:"initial_capital,omitempty"`    // 初始资金覆盖（可选）
	CommissionRate  *float64 `json:"commission_rate,omitempty"`    // 手续费覆盖（可选）
	SlippagePct     *float64 `json:"slippage_pct,omitempty"`       // 滑点覆盖（可选）
	MaxTradesPerDay *int     `json:"max_trades_per_day,omitempty"` // 每日最大笔数覆盖
	AllowedTimes    []string `json:"allowed_times,omitempty"`      // 限定交易时间点覆盖
}

// CreateRunRequest 创建回测任务请求体。
type CreateRunRequest struct {
	StrategyID      string     `json:"strategy_id"`
	AccountID       string     `json:"account_id"`
	EnvID           string     `json:"env_id"`  // 环境（可选，缺省取账户 env_id / 默认环境）
	UserID          string     `json:"user_id"` // 所属用户（可选，缺省 default）
	SecuCode        string     `json:"secu_code"`
	StartDate       int        `json:"start_date"`       // yyyymmdd（可选，缺省用行情最早）
	EndDate         int        `json:"end_date"`         // yyyymmdd
	Period          string     `json:"period"`           // Min/Hour/Day（缺省取策略 data.period）
	ReportPrecision string     `json:"report_precision"` // Min/Hour/Day（缺省 Day）
	Options         RunOptions `json:"options"`
	CreatedBy       string     `json:"created_by"` // 创建人（可选，缺省 console，接入认证后为登录账号）
}

// Environment 回测环境表行（回测/模拟盘/仿真/实盘环境 × 地区/市场）。
type Environment struct {
	EnvID         string            `json:"env_id"`
	EnvCode       string            `json:"env_code"`
	EnvName       string            `json:"env_name"`
	EnvType       string            `json:"env_type"`    // BACKTEST/PAPER/SIMULATION/LIVE
	Region        string            `json:"region"`      // 地区（CN/US/HK...）
	MarketCode    int               `json:"market_code"` // 关联市场
	Config        EnvironmentConfig `json:"config"`      // 环境配置（交易时段/规则/成本/撮合/偏好）
	UserID        string            `json:"user_id"`
	IsDefault     string            `json:"is_default"`
	AllowBacktest string            `json:"allow_backtest"`
	Status        string            `json:"status"`
	Description   string            `json:"description"`
	CreatedBy     string            `json:"created_by"`
	GMTUpdate     time.Time         `json:"gmt_update"`
}

// EnvironmentConfig 环境配置（config JSONB，自适应不同市场/环境）。
type EnvironmentConfig struct {
	TradingSessions []SessionDef     `json:"trading_sessions,omitempty"` // 交易时段（hhmmss，空=不限制）
	TradingRules    *TradingRulesDef `json:"trading_rules,omitempty"`    // 交易规则（T+N/涨跌停/合约乘数/tick_size）
	Cost            *CostDef         `json:"cost,omitempty"`             // 成本基准（覆盖链：环境>任务>策略>账户）
	FillMode        string           `json:"fill_mode,omitempty"`        // 撮合模式
	Currency        string           `json:"currency,omitempty"`         // 计价币种
	Preferences     map[string]any   `json:"preferences,omitempty"`      // 地区习惯偏好（自定义扩展）
}

// SessionDef 交易时段定义。
type SessionDef struct {
	Start string `json:"start"` // hhmmss
	End   string `json:"end"`   // hhmmss
}

// TradingRulesDef 交易规则定义。
type TradingRulesDef struct {
	TPlus              int     `json:"t_plus,omitempty"`              // T+N 交收规则（0=T+0）
	TickSize           float64 `json:"tick_size,omitempty"`           // 最小变动单位（价格精度）
	ContractMultiplier float64 `json:"contract_multiplier,omitempty"` // 合约乘数
	LimitUpPct         float64 `json:"limit_up_pct,omitempty"`        // 涨停幅度 %（0=无限制）
	LimitDownPct       float64 `json:"limit_down_pct,omitempty"`      // 跌停幅度 %（0=无限制）
}

// Template 模板表行（策略/账户/环境三类模板，相同部分复用、差异自定义）。
type Template struct {
	TemplateID   string         `json:"template_id"`
	TemplateCode string         `json:"template_code"`
	TemplateName string         `json:"template_name"`
	TemplateType string         `json:"template_type"` // STRATEGY/ACCOUNT/ENVIRONMENT
	Content      map[string]any `json:"content"`       // 模板内容
	UserID       string         `json:"user_id"`
	IsBuiltin    string         `json:"is_builtin"`
	Status       string         `json:"status"`
	Description  string         `json:"description"`
	CreatedBy    string         `json:"created_by"`
	GMTUpdate    time.Time      `json:"gmt_update"`
}

// EquityPoint 净值曲线点。
type EquityPoint struct {
	Seq           int     `json:"seq"`
	TS            int64   `json:"ts"`
	Date          int     `json:"date"`
	Time          int     `json:"time"`
	Equity        float64 `json:"equity"`
	Cash          float64 `json:"cash"`
	PositionValue float64 `json:"position_value"`
	PositionQty   float64 `json:"position_qty"`
	Profit        float64 `json:"profit"`
	ROI           float64 `json:"roi"`
	Drawdown      float64 `json:"drawdown"`
}

// Trade 成交记录行。
type Trade struct {
	TradeID       int64   `json:"trade_id"`
	RunID         string  `json:"run_id"`
	Seq           int     `json:"seq"` // 引擎内成交顺序号（明细关联用）
	TS            int64   `json:"ts"`
	Date          int     `json:"date"`
	Time          int     `json:"time"`
	Action        string  `json:"action"`
	Price         float64 `json:"price"`
	Qty           float64 `json:"qty"`
	Amount        float64 `json:"amount"`
	Fee           float64 `json:"fee"`
	Profit        float64 `json:"profit"`
	PositionAfter float64 `json:"position_after"`
	CashAfter     float64 `json:"cash_after"`
	Signal        string  `json:"signal"`
	Remark        string  `json:"remark"` // 备注（与触发原因一致，V26 remark 列落值）
}

// Cashflow 资金流水明细（需求⑨-1）。
type Cashflow struct {
	CashflowID int64   `json:"cashflow_id"`
	RunID      string  `json:"run_id"`
	Seq        int     `json:"seq"`
	TS         int64   `json:"ts"`
	Date       int     `json:"date"`
	Time       int     `json:"time"`
	FlowType   string  `json:"flow_type"`
	Amount     float64 `json:"amount"`
	CashBefore float64 `json:"cash_before"`
	CashAfter  float64 `json:"cash_after"`
	TradeID    int64   `json:"trade_id"`
	TradeSeq   int     `json:"trade_seq"` // 引擎内成交序号（落库时映射为 trade_id）
	Remark     string  `json:"remark"`
}

// PositionLog 持仓变化明细（需求⑨-2）。
type PositionLog struct {
	LogID          int64   `json:"log_id"`
	RunID          string  `json:"run_id"`
	Seq            int     `json:"seq"`
	TS             int64   `json:"ts"`
	Date           int     `json:"date"`
	Time           int     `json:"time"`
	Action         string  `json:"action"`
	Price          float64 `json:"price"`
	Qty            float64 `json:"qty"`
	PositionBefore float64 `json:"position_before"`
	PositionAfter  float64 `json:"position_after"`
	AvgCostBefore  float64 `json:"avg_cost_before"`
	AvgCostAfter   float64 `json:"avg_cost_after"`
	TradeID        int64   `json:"trade_id"`
	TradeSeq       int     `json:"trade_seq"` // 引擎内成交序号（落库时映射为 trade_id）
	Remark         string  `json:"remark"`
}

// EventTrace 交易事件追踪（需求⑨-3 / FR-10）：触发原因/成交结果/委托下单时间/委托耗时/事件存活时间/未成交原因。
// 八项登记信息：①触发原因（trigger_reason）②触发时间（trigger_ts）③成交结果与否（exec_status）
// ④结束时间（exec_ts）⑤委托下单时间（order_ts）⑥成交耗时（latency_bars/latency_sec）
// ⑦事件存活时间（alive_sec=结束时间-触发时间）⑧未能成交的原因（reject_reason）。
type EventTrace struct {
	EventID       int64   `json:"event_id"`
	RunID         string  `json:"run_id"`
	Seq           int     `json:"seq"`
	Action        string  `json:"action"`
	TriggerReason string  `json:"trigger_reason"`
	TriggerTS     int64   `json:"trigger_ts"`
	TriggerDate   int     `json:"trigger_date"`
	TriggerTime   int     `json:"trigger_time"`
	OrderTS       int64   `json:"order_ts"` // ⑤委托下单时间（信号确认后立即下单，= 触发时点）
	OrderDate     int     `json:"order_date"`
	OrderTime     int     `json:"order_time"`
	ExecStatus    string  `json:"exec_status"`
	ExecTS        int64   `json:"exec_ts"`
	ExecDate      int     `json:"exec_date"`
	ExecTime      int     `json:"exec_time"`
	LatencyBars   int     `json:"latency_bars"`
	LatencySec    int64   `json:"latency_sec"`
	AliveSec      int64   `json:"alive_sec"` // ⑦事件存活时间（结束时间-触发时间，秒）
	RejectReason  string  `json:"reject_reason"`
	Price         float64 `json:"price"`
	Qty           float64 `json:"qty"`
	TradeID       int64   `json:"trade_id"`
	TradeSeq      int     `json:"trade_seq"` // 引擎内成交序号（落库时映射为 trade_id）
}

// ---------------------------------------------------------------------
// 回测报告（run.report JSONB）
// ---------------------------------------------------------------------

// RunReport 汇总回测报告（需求报告项 ④⑤⑥⑧ 等汇总指标）。
type RunReport struct {
	// 基础
	SecuCode        string `json:"secu_code"`
	Period          string `json:"period"`
	ReportPrecision string `json:"report_precision"`
	StartDate       int    `json:"start_date"`
	EndDate         int    `json:"end_date"`
	BarCount        int    `json:"bar_count"` // 参与回放 bar 总数
	// 资金与收益（①账户余额 / ②收益率 / ③收益额）
	InitialCapital  float64 `json:"initial_capital"`   // 初始启动资金
	FinalEquity     float64 `json:"final_equity"`      // 期末总资产（持仓换算现金）
	TotalProfit     float64 `json:"total_profit"`      // 总收益额
	TotalReturnPct  float64 `json:"total_return_pct"`  // 总收益率 %
	AnnualReturnPct float64 `json:"annual_return_pct"` // 年化收益率 %（按自然日）
	// 风险（⑧技术指标）
	MaxDrawdownPct     float64 `json:"max_drawdown_pct"` // 最大回撤 %
	MaxDrawdownStartTS int64   `json:"max_drawdown_start_ts"`
	MaxDrawdownEndTS   int64   `json:"max_drawdown_end_ts"`
	SharpeRatio        float64 `json:"sharpe_ratio"`   // 夏普比率（rf=0，按报告精度收益率序列）
	VolatilityPct      float64 `json:"volatility_pct"` // 收益率年化波动率 %
	// 交易统计
	TradeCount   int     `json:"trade_count"` // 成交笔数
	BuyCount     int     `json:"buy_count"`
	SellCount    int     `json:"sell_count"`
	WinCount     int     `json:"win_count"` // 盈利平仓笔数
	LossCount    int     `json:"loss_count"`
	WinRatePct   float64 `json:"win_rate_pct"`  // 胜率 %
	ProfitFactor float64 `json:"profit_factor"` // 盈亏比（总盈利/总亏损）
	TotalFee     float64 `json:"total_fee"`     // 手续费总额
	// 投入资金（④⑤）
	MaxInvested  float64 `json:"max_invested"`  // 期间投入的最大金额（持仓市值峰值）
	AvgInvested  float64 `json:"avg_invested"`  // 期间平均投入金额（按持仓期时间加权）
	InvestedDays int     `json:"invested_days"` // 持仓天数（有持仓的交易日数）
	// 周期收益（⑧）
	BestDayPct  float64 `json:"best_day_pct"`  // 报告精度下最佳单期收益率 %
	WorstDayPct float64 `json:"worst_day_pct"` // 报告精度下最差单期收益率 %
	ProfitDays  int     `json:"profit_days"`   // 盈利期数
	LossDays    int     `json:"loss_days"`     // 亏损期数
	// 其他
	TradeSignalDetail map[string]int `json:"trade_signal_detail"` // 信号归因统计（信号名 → 笔数）
	GeneratedAt       string         `json:"generated_at"`        // 报告生成时间（RFC3339）

	// 链路追踪统计（需求⑨）：事件触发/成交/拒绝/耗时分布
	EventStats *EventStats `json:"event_stats,omitempty"` // 事件追踪汇总（nil=无事件）
}

// EventStats 交易事件追踪汇总统计。
type EventStats struct {
	TriggerCount   int            `json:"trigger_count"`    // 事件触发总数
	FilledCount    int            `json:"filled_count"`     // 成交事件数
	RejectedCount  int            `json:"rejected_count"`   // 拒绝事件数
	ExpiredCount   int            `json:"expired_count"`    // 过期事件数
	PendingCount   int            `json:"pending_count"`    // 挂单事件数
	AvgLatencyBars float64        `json:"avg_latency_bars"` // 平均委托耗时（bar）
	AvgLatencySec  float64        `json:"avg_latency_sec"`  // 平均委托耗时（秒）
	RejectReasons  map[string]int `json:"reject_reasons"`   // 未成交原因分布
	TriggerReasons map[string]int `json:"trigger_reasons"`  // 触发原因分布
}

// RunListQuery 任务列表查询参数。
type RunListQuery struct {
	Pager
	Status     string
	SecuCode   string
	StrategyID string
	Keyword    string
}

// ---------------------------------------------------------------------
// 回测任务删除（异步批处理删除任务 + 审计日志）
// ---------------------------------------------------------------------

// RunDelTask 回测任务删除任务表行（按 run_id 删除任务及其关联明细）。
type RunDelTask struct {
	DelTaskID     string         `json:"del_task_id"`
	RunID         string         `json:"run_id"`
	Status        string         `json:"status"`         // PENDING/RUNNING/SUCCEEDED/FAILED
	Progress      int            `json:"progress"`       // 进度 0-100
	ErrorMessage  string         `json:"error_message"`  // 失败原因
	DeletedCounts map[string]int `json:"deleted_counts"` // 各表删除行数 {表名: 行数}
	CreatedBy     string         `json:"created_by"`
	GMTUpdate     time.Time      `json:"gmt_update"`
}

// RunDelLog 删除任务审计日志行（只追加，不可改）。
type RunDelLog struct {
	LogID     int64     `json:"log_id"`
	DelTaskID string    `json:"del_task_id"`
	RunID     string    `json:"run_id"`
	Seq       int       `json:"seq"`
	Action    string    `json:"action"` // TASK_CREATED/DELETING_TABLE/TASK_SUCCEEDED/TASK_FAILED
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// RunArchive 已删除任务归档（"曾经存在的证明"）：删除任务时归档 run 元信息留痕。
type RunArchive struct {
	ArchiveID    string    `json:"archive_id"`
	RunID        string    `json:"run_id"`
	RunNo        int64     `json:"run_no"`
	StrategyID   string    `json:"strategy_id"`
	StrategyName string    `json:"strategy_name"`
	AccountID    string    `json:"account_id"`
	AccountName  string    `json:"account_name"`
	SecuCode     string    `json:"secu_code"`
	Period       string    `json:"period"`
	StartDate    int       `json:"start_date"`
	EndDate      int       `json:"end_date"`
	Status       string    `json:"status"` // 删除前任务状态
	ErrorMessage string    `json:"error_message"`
	ReportJSON   string    `json:"report_json"`
	DeletedAt    time.Time `json:"deleted_at"`
	DeletedBy    string    `json:"deleted_by"`
	DelTaskID    string    `json:"del_task_id"`
}

// DelTaskListQuery 删除任务列表查询参数。
type DelTaskListQuery struct {
	Pager
	Status string
	RunID  string
}
