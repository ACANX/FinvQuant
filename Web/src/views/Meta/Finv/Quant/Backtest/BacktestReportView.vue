<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { apiGet } from '../../../../../api'
import { fmtDate, fmtNum, fmtPct, fmtTime } from '../../../../../utils'

const route = useRoute()
const router = useRouter()

interface Report {
  secu_code: string
  period: string
  report_precision: string
  start_date: number
  end_date: number
  bar_count: number
  initial_capital: number
  final_equity: number
  total_profit: number
  total_return_pct: number
  annual_return_pct: number
  max_drawdown_pct: number
  sharpe_ratio: number
  volatility_pct: number
  trade_count: number
  buy_count: number
  sell_count: number
  win_count: number
  loss_count: number
  win_rate_pct: number
  profit_factor: number
  total_fee: number
  max_invested: number
  avg_invested: number
  invested_days: number
  best_day_pct: number
  worst_day_pct: number
  profit_days: number
  loss_days: number
  trade_signal_detail: Record<string, number>
  event_stats: EventStats | null
  generated_at: string
}

interface EventStats {
  trigger_count: number
  filled_count: number
  rejected_count: number
  expired_count: number
  pending_count: number
  avg_latency_bars: number
  avg_latency_sec: number
  reject_reasons: Record<string, number>
  trigger_reasons: Record<string, number>
}

interface EquityPoint {
  seq: number
  ts: number
  date: number
  time: number
  equity: number
  cash: number
  position_value: number
  position_qty: number
  profit: number
  roi: number
  drawdown: number
}

interface TradeRow {
  trade_id: number
  seq: number
  ts: number
  date: number
  time: number
  action: string
  price: number
  qty: number
  amount: number
  fee: number
  profit: number
  position_after: number
  cash_after: number
  signal: string
  remark?: string
}

interface CashflowRow {
  cashflow_id: number
  seq: number
  date: number
  time: number
  flow_type: string
  amount: number
  cash_before: number
  cash_after: number
  trade_id: number
  remark: string
}

interface PositionLogRow {
  log_id: number
  seq: number
  date: number
  time: number
  action: string
  price: number
  qty: number
  position_before: number
  position_after: number
  avg_cost_before: number
  avg_cost_after: number
  trade_id: number
  remark: string
}

interface EventTraceRow {
  event_id: number
  seq: number
  action: string
  trigger_reason: string
  trigger_date: number
  trigger_time: number
  order_date?: number
  order_time?: number
  exec_status: string
  exec_date: number
  exec_time: number
  latency_bars: number
  latency_sec: number
  alive_sec?: number
  reject_reason: string
  price: number
  qty: number
  trade_id: number
}

// ---------------------------------------------------------------------
// 回测背景信息类型（Run/Get 返回完整 Run 对象；strategy_snapshot 为 StrategyDefinition 序列化）
// ---------------------------------------------------------------------

interface StrategySnapshot {
  version?: string
  strategy_type?: string
  description?: string
  universe?: { securities?: string[] }
  data?: { period?: string; price_field?: string; warmup_bars?: number; fill_mode?: string }
  indicators?: IndicatorDef[]
  signals?: { buy?: string; sell?: string }
  rules?: { buy?: RuleDef; sell?: RuleDef }
  risk?: RiskDef
  cost?: CostDef
}

interface IndicatorDef {
  id?: string
  type?: string
  params?: Record<string, unknown>
}

interface RuleDef {
  action?: string
  quantity_type?: string // ALL_IN/ALL/FIXED/PERCENT/AMOUNT
  quantity?: number
  max_per_day?: number
  allowed_times?: string[] // hhmmss 字符串
  max_per_run?: number
  allow?: boolean
  max_per_week?: number
  max_per_month?: number
  max_fee_per_window?: number
  fee_window_days?: number
}

interface BuilderDef {
  enabled?: boolean
  target_position_pct?: number
  tranches?: number
  tranche_interval_bars?: number
}

interface RiskDef {
  stop_loss_pct?: number
  take_profit_pct?: number
  max_position_pct?: number
  max_positions?: number
  max_trades_per_day?: number
  min_interval_bars?: number
  builder?: BuilderDef | null
  reduce_tranches?: number
  max_trades_per_week?: number
  max_trades_per_month?: number
  max_fee_per_window?: number
  fee_window_days?: number
}

interface CostDef {
  commission_rate?: number // 比值（0.0003 = 0.03%）
  slippage_pct?: number
}

interface AccountSnapshot {
  account_id: string
  account_code: string
  account_name: string
  user_id: string
  group_id?: string | null
  initial_capital: number
  currency_type: string
  commission_rate: number
  slippage_pct: number
  margin_mode: string // FULL/FUTURES
  margin_rate: number
}

interface EnvironmentConfig {
  trading_sessions?: { start: string; end: string }[]
  trading_rules?: {
    t_plus?: number
    tick_size?: number
    contract_multiplier?: number
    limit_up_pct?: number
    limit_down_pct?: number
  }
  cost?: CostDef
  fill_mode?: string
  currency?: string
  preferences?: Record<string, unknown>
}

interface EnvironmentSnapshot {
  env_id: string
  env_code: string
  env_name: string
  env_type: string // BACKTEST/PAPER/SIMULATION/LIVE
  region: string
  market_code: number
  config: EnvironmentConfig
  user_id: string
  is_default: string
  allow_backtest: string
  status: string
  description: string
}

interface RunOptions {
  enable_backtest?: boolean
  report_precision?: string
  initial_capital?: number | null
  commission_rate?: number | null
  slippage_pct?: number | null
  max_trades_per_day?: number | null
  allowed_times?: string[]
}

interface RunDetail {
  run_id: string
  run_no: number
  user_id: string
  strategy_id: string
  strategy_code: string
  strategy_name: string
  strategy_snapshot: StrategySnapshot
  account_id: string
  account_code: string
  account_name: string
  account_snapshot: AccountSnapshot
  env_id: string
  environment_snapshot: EnvironmentSnapshot | null
  secu_code: string
  market_code: number
  period: string
  report_precision: string
  start_ts: number
  end_ts: number
  start_date: number
  end_date: number
  options: RunOptions
  status: string
  progress: number
  error_message: string
  started_at?: string | null
  finished_at?: string | null
  created_by: string
}

interface StrategyDetail {
  strategy_id: string
  strategy_code: string
  strategy_name: string
  strategy_type: string
  description: string
  definition: Record<string, unknown>
  definition_version: number
  data_period: string
  secu_code: string
  template_id?: string | null
  allow_backtest: string
  status: string
  created_by: string
}

const run = ref<RunDetail | null>(null)
const strategyDetail = ref<StrategyDetail | null>(null)
const report = ref<Report | null>(null)
const equity = ref<EquityPoint[]>([])
// 成交记录（服务端分页）
const trades = ref<TradeRow[]>([])
const tradeTotal = ref(0)
const tradePage = ref(1)
const tradePageSize = ref(20)
// 资金流水（服务端分页）
const cashflows = ref<CashflowRow[]>([])
const cashflowTotal = ref(0)
const cashflowPage = ref(1)
const cashflowPageSize = ref(20)
// 持仓变化（服务端分页）
const positionLogs = ref<PositionLogRow[]>([])
const positionLogTotal = ref(0)
const positionLogPage = ref(1)
const positionLogPageSize = ref(20)
// 事件追踪（服务端分页）
const eventTraces = ref<EventTraceRow[]>([])
const eventTraceTotal = ref(0)
const eventTracePage = ref(1)
const eventTracePageSize = ref(20)
const loading = ref(false)
const error = ref('')

const charts: { chart: echarts.ECharts; key: string }[] = []
const chartTitles: Record<string, string> = {
  equity: '总资产曲线',
  cash: '现金曲线',
  positionValue: '持仓市值曲线',
  roi: '投资收益率曲线',
  profit: '累计收益额曲线',
  positionQty: '持仓数量变化曲线',
}
const chartEls: Record<string, HTMLDivElement | null> = {
  equity: null,
  cash: null,
  positionValue: null,
  roi: null,
  profit: null,
  positionQty: null,
}
// 图表 DOM 绑定（Vue 函数 ref：:ref="el => setChartRef('equity', el)"）
// 用 ResizeObserver 监听容器尺寸变化（折叠侧边导航 / 窗口缩放时 canvas 自动跟随宽度）
const chartROs = new Map<string, ResizeObserver>()
function setChartRef(key: string, el: unknown) {
  const dom = el as HTMLDivElement | null
  chartEls[key] = dom
  chartROs.get(key)?.disconnect()
  chartROs.delete(key)
  if (!dom) return
  const ro = new ResizeObserver(() => {
    charts.find((c) => c.key === key)?.chart.resize()
  })
  ro.observe(dom)
  chartROs.set(key, ro)
}
function disposeChartObservers() {
  chartROs.forEach((ro) => ro.disconnect())
  chartROs.clear()
}
// 全屏放大
const fullscreenKey = ref('')
const fullscreenDialog = ref(false)
const fullscreenChartEl = ref<HTMLDivElement | null>(null)

function fmtDateTimeLocal(d: number | undefined, t: number | undefined): string {
  if (!d) return '-'
  return `${fmtDate(d)} ${fmtTime(t)}`
}

/** 指标数值万分位格式化：≥1万 显示 X.XX万；小于 1万 直接显示原始数值（不做千分位） */
function fmtWan(v: number | undefined | null): string {
  if (v === undefined || v === null || Number.isNaN(v)) return '-'
  const n = Number(v)
  if (Math.abs(n) >= 10000) {
    const w = n / 10000
    return `${Number.isInteger(w) ? w.toFixed(0) : w.toFixed(2)}万`
  }
  return String(Math.round(n * 100) / 100)
}

function flowTypeName(t: string): string {
  return {
    INITIAL_DEPOSIT: '初始资金', BUY_PAY: '买入付款', SELL_RECEIVE: '卖出收款',
    FEE: '手续费', MARGIN_HOLD: '保证金占用', MARGIN_RELEASE: '保证金释放',
  }[t] ?? t
}

function flowTypeColor(t: string): string {
  return { INITIAL_DEPOSIT: 'blue', BUY_PAY: 'red', SELL_RECEIVE: 'green', FEE: 'orange', MARGIN_HOLD: 'purple', MARGIN_RELEASE: 'teal' }[t] ?? 'grey'
}

function posActionName(a: string): string {
  return { OPEN: '开仓', ADD: '加仓', REDUCE: '减仓', CLOSE: '平仓' }[a] ?? a
}

function posActionColor(a: string): string {
  return { OPEN: 'green', ADD: 'teal', REDUCE: 'orange', CLOSE: 'red' }[a] ?? 'grey'
}

function execStatusName(s: string): string {
  return { FILLED: '成交', REJECTED: '拒绝', PENDING: '挂单', EXPIRED: '过期' }[s] ?? s
}

function execStatusColor(s: string): string {
  return { FILLED: 'green', REJECTED: 'red', PENDING: 'blue', EXPIRED: 'orange' }[s] ?? 'grey'
}

// 未成交原因 → 可读说明 / 解决建议（让用户看得懂“为何无法交易”）
function rejectReasonHint(reason: string): string {
  if (reason.includes('不在环境交易时段')) return '该 bar 时间点不在环境配置的交易时段内；可在「环境与模板管理」中调整 trading_sessions（当前 GCMain 已配置为全天时段）'
  if (reason.includes('已达目标持仓上限') || reason.includes('已达最大持仓')) return '当前持仓已达到策略目标仓位上限，买入信号不再追加；如需继续加仓请提高 target_position_pct / max_position_pct'
  if (reason.includes('无持仓')) return '当前处于空仓状态，卖出信号无法执行；需先有买入成交形成持仓'
  if (reason.includes('今日已达成交笔数上限')) return '当日成交笔数已达上限（max_trades_per_day）；可调高该值或改用周/月窗口限制'
  if (reason.includes('近7日') || reason.includes('近30日')) return '滚动窗口内交易频率已达上限；可调高 max_trades_per_week / max_trades_per_month 或手续费上限'
  if (reason.includes('涨停')) return '买入价格触及涨停价，无法成交；通常次日恢复'
  if (reason.includes('跌停')) return '卖出价格触及跌停价，无法成交；通常次日恢复'
  if (reason.includes('T+') && reason.includes('交收')) return 'T+N 交收限制：当日买入的持仓在交收期内不可卖出（环境 trading_rules.t_plus）'
  if (reason.includes('资金不足')) return '可用资金不足以覆盖委托金额（含手续费）；可降低单笔投入或调高初始资金'
  if (reason.includes('最小交易间隔')) return '相邻两次成交间隔未达到策略设置的 min_interval_bars'
  if (reason.includes('允许交易时间点')) return '当前时间点不在策略规则 allowed_times 限定范围内'
  if (reason.includes('回测结束')) return '回测区间结束时仍有未成交委托，后续无 K 线可撮合'
  if (reason.includes('分批建仓最小间隔')) return '相邻两批建仓间隔未达到 builder.tranche_interval_bars 设定值'
  return ''
}

// ---------------------------------------------------------------------
// 回测背景信息：可读化函数与计算属性
// ---------------------------------------------------------------------

interface KV {
  label: string
  value: string
}

/** 数量方式中文名 */
function quantityTypeName(t: string | undefined): string {
  return { ALL_IN: '全部可用资金', ALL: '全部清仓', FIXED: '固定数量', PERCENT: '可用资金百分比', AMOUNT: '固定金额' }[t ?? ''] ?? (t ?? '-')
}

/** 保证金模式中文名 */
function marginModeName(m: string | undefined): string {
  return { FULL: '全额保证金', FUTURES: '期货保证金' }[m ?? ''] ?? (m ?? '-')
}

/** 环境类型中文名 */
function envTypeName(t: string | undefined): string {
  return { BACKTEST: '回测', PAPER: '模拟盘', SIMULATION: '仿真', LIVE: '实盘' }[t ?? ''] ?? (t ?? '-')
}

/** 撮合模式中文名 */
function fillModeName(f: string | undefined): string {
  return { NEXT_BAR_OPEN: '下一Bar开盘价成交（无未来函数）', CURRENT_CLOSE: '当前Bar收盘价成交（近似）' }[f ?? ''] ?? (f ?? '-')
}

/** 数据周期中文名 */
function periodName(p: string | undefined): string {
  return { Min: '分钟', Hour: '小时', Day: '日线' }[p ?? ''] ?? (p ?? '-')
}

/** 策略类型中文名 */
function strategyTypeName(t: string | undefined): string {
  return { RULE_BASED: '规则驱动' }[t ?? ''] ?? (t ?? '-')
}

/** hhmmss 字符串 → HH:mm:ss（如 '093000' → '09:30:00'） */
function hhmmssFmt(s: string | undefined): string {
  if (!s) return '-'
  const t = s.padStart(6, '0')
  return `${t.slice(0, 2)}:${t.slice(2, 4)}:${t.slice(4, 6)}`
}

/** 单段交易时段 → '09:30-15:00' */
function sessionFmt(start: string, end: string): string {
  return `${(start || '').slice(0, 2)}:${(start || '').slice(2, 4)}-${(end || '').slice(0, 2)}:${(end || '').slice(2, 4)}`
}

/** 交易时段数组 → '09:30-15:00 / 21:00-02:30'；空=不限制 */
function sessionsFmt(sessions: { start: string; end: string }[] | undefined): string {
  if (!sessions?.length) return '不限制（全天）'
  return sessions.map((s) => sessionFmt(s.start, s.end)).join(' / ')
}

/** 比值 → 百分比（commission_rate/slippage/margin_rate，0.0003 → '0.03%'） */
function pctRatio(r: number | undefined): string {
  if (r === undefined || r === null || Number.isNaN(r)) return '-'
  return `${(r * 100).toFixed(2)}%`
}

/** 频次限制 → '3 次'；0 或空 → '不限制' */
function limitText(n: number | undefined): string {
  if (n === undefined || n === null || n === 0) return '不限制'
  return `${fmtNum(n, 0)} 次`
}

/** 数量方式完整描述（含数量值） */
function quantityText(r: RuleDef | undefined): string {
  const t = r?.quantity_type
  if (!t) return '-'
  switch (t) {
    case 'FIXED': return `${quantityTypeName(t)} ${fmtNum(r.quantity, 0)} 手`
    case 'PERCENT': return `${quantityTypeName(t)} ${fmtNum(r.quantity)}%`
    case 'AMOUNT': return `${quantityTypeName(t)} ${fmtNum(r.quantity)}`
    default: return quantityTypeName(t)
  }
}

/** 窗口手续费上限 → '55000（30 日窗口）'；0 或空 → '不限制' */
function feeWindowText(v: number | undefined, days: number | undefined): string {
  if (v === undefined || v === null || v === 0) return '不限制'
  return `${fmtNum(v)}（${days || 30} 日窗口）`
}

/** ① 数据配置一行（读者友好）：'K线周期：分钟；信号字段：收盘价；指标预热：30 根K线' */
function dataConfigText(d: StrategySnapshot['data'] | undefined): string {
  if (!d) return '-'
  const parts: string[] = []
  if (d.period) parts.push(`K线周期：${periodName(d.period)}`)
  if (d.price_field) parts.push(`信号字段：${priceFieldName(d.price_field)}`)
  if (d.warmup_bars) parts.push(`指标预热：${d.warmup_bars} 根K线`)
  return parts.join('，') || '-'
}

/** 买卖信号转述为中文（读者友好）：'买入信号：MA5 上穿 MA20；卖出信号：MA5 下穿 MA20' */
function signalExplainText(signals: { buy?: string; sell?: string } | undefined): string {
  if (!signals) return '（未设置）'
  const parts: string[] = []
  if (signals.buy) parts.push(`买入信号：${signalExplain(signals.buy)}`)
  if (signals.sell) parts.push(`卖出信号：${signalExplain(signals.sell)}`)
  return parts.length ? parts.join('；') : '（未设置）'
}

/** 指标短名：'MA5'、'EMA20'（type + window 参数），用于信号转述 */
function indShort(ind: IndicatorDef): string {
  const w = ind.params?.window
  return `${ind.type ?? '?'}${w ? String(w) : ''}`
}

/** 指标 chip 展示：'MA（5 周期）'（不展示价格字段，避免中英文混杂） */
function indChipText(ind: IndicatorDef): string {
  const w = ind.params?.window
  return `${ind.type ?? '?'}${w ? `（${w} 周期）` : ''}`
}

/** 价格字段中文名：close → 收盘价 */
function priceFieldName(f: string | undefined): string {
  return { open: '开盘价', high: '最高价', low: '最低价', close: '收盘价' }[f ?? ''] ?? (f ?? '-')
}

/** 信号表达式转述为中文：cross_up(ma_fast, ma_slow) → 'MA5 上穿 MA20' */
function signalExplain(sig: string): string {
  const inds = indicators.value
  const name = (id: string) => {
    const ind = inds.find((i) => i.id === id)
    return ind ? indShort(ind) : id
  }
  const up = sig.match(/^cross_up\(\s*([\w]+)\s*,\s*([\w]+)\s*\)$/)
  if (up) return `${name(up[1])} 上穿 ${name(up[2])}`
  const down = sig.match(/^cross_down\(\s*([\w]+)\s*,\s*([\w]+)\s*\)$/)
  if (down) return `${name(down[1])} 下穿 ${name(down[2])}`
  const gt = sig.match(/^(.+?)\s*>\s*(.+)$/)
  if (gt) return `${gt[1].trim()} 大于 ${gt[2].trim()}`
  const lt = sig.match(/^(.+?)\s*<\s*(.+)$/)
  if (lt) return `${lt[1].trim()} 小于 ${lt[2].trim()}`
  return sig
}

/** 撮合模式展开说明（给普通用户看） */
function fillModeExplain(f: string | undefined): string {
  if (f === 'NEXT_BAR_OPEN') return '信号在当根K线收盘确认后触发，委托挂入下一根K线并按开盘价成交，全程不引用未来数据，结果贴近实盘可执行性。'
  if (f === 'CURRENT_CLOSE') return '信号触发即按当根K线收盘价近似成交，速度更快，但属于近似撮合，存在轻微前视偏差。'
  return fillModeName(f)
}

// ---- 计算属性（模板薄化） ----

const options = computed<RunOptions>(() => run.value?.options ?? {})
const env = computed<EnvironmentSnapshot | null>(() => run.value?.environment_snapshot ?? null)
const snapRules = computed<{ buy?: RuleDef; sell?: RuleDef } | undefined>(() => run.value?.strategy_snapshot?.rules)
const indicators = computed<IndicatorDef[]>(() => run.value?.strategy_snapshot?.indicators ?? [])
const hasStrategyDef = computed<boolean>(() => !!run.value?.strategy_snapshot && Object.keys(run.value.strategy_snapshot).length > 0)

// 策略描述：快照 description 为主，空则回退当前策略说明
const primaryStrategyDesc = computed<string>(() => {
  const snap = (run.value?.strategy_snapshot?.description ?? '').trim()
  if (snap) return snap
  return (strategyDetail.value?.description ?? '').trim() || '（未填写策略说明）'
})

// 当前说明与快照不一致时提示
const strategyDescNote = computed<{ label: string; text: string } | null>(() => {
  const snap = (run.value?.strategy_snapshot?.description ?? '').trim()
  const cur = (strategyDetail.value?.description ?? '').trim()
  if (!snap || !cur || snap === cur) return null
  return { label: '策略当前说明与回测快照不同', text: cur }
})

/** 策略概览：把信号/建仓/风控加工成普通用户能读懂的通俗介绍 */
const strategyOverview = computed<string>(() => {
  const snap = run.value?.strategy_snapshot
  if (!snap) return ''
  const parts: string[] = []
  const sig = snap.signals
  if (sig?.buy) parts.push(`买入信号：${signalExplain(sig.buy)}`)
  if (sig?.sell) parts.push(`卖出信号：${signalExplain(sig.sell)}`)
  const rb = snap.rules?.buy
  if (rb?.quantity_type) parts.push(`单次买入：${quantityText(rb)}`)
  const risk = snap.risk
  const b = risk?.builder
  if (b?.enabled) {
    parts.push(`分批建仓：分 ${b.tranches ?? 1} 批建仓，目标仓位 ${fmtPct(b.target_position_pct ?? 0)}，批间隔 ${b.tranche_interval_bars ?? 0} 根K线`)
  }
  if (risk?.reduce_tranches && risk.reduce_tranches > 1) parts.push(`分批减仓：分 ${risk.reduce_tranches} 批卖出`)
  if (risk?.stop_loss_pct) parts.push(`单笔止损 ${fmtPct(risk.stop_loss_pct)}`)
  if (risk?.take_profit_pct) parts.push(`止盈 ${fmtPct(risk.take_profit_pct)}`)
  const maxTrades = options.value.max_trades_per_day != null ? options.value.max_trades_per_day : risk?.max_trades_per_day
  if (maxTrades) parts.push(`每日成交上限 ${maxTrades} 笔`)
  if (risk?.max_trades_per_week) parts.push(`近7日成交上限 ${risk.max_trades_per_week} 笔`)
  if (risk?.max_trades_per_month) parts.push(`近30日成交上限 ${risk.max_trades_per_month} 笔`)
  return parts.length ? parts.join('；') + '。' : ''
})

const accountText = computed<string>(() => {
  const a = run.value?.account_snapshot
  if (!a?.account_name) return '-'
  return `${a.account_name}${a.account_code ? `（${a.account_code}）` : ''}`
})

// 成本生效值（覆盖链：环境 > 任务 options > 策略 definition.cost > 账户）
const effectiveCommission = computed<number | undefined>(() => {
  const envCost = env.value?.config?.cost?.commission_rate
  if (envCost) return envCost
  if (options.value.commission_rate != null) return options.value.commission_rate
  const stratCost = run.value?.strategy_snapshot?.cost?.commission_rate
  if (stratCost) return stratCost
  return run.value?.account_snapshot?.commission_rate || undefined
})
const effectiveSlippage = computed<number | undefined>(() => {
  const envSlip = env.value?.config?.cost?.slippage_pct
  if (envSlip) return envSlip
  if (options.value.slippage_pct != null) return options.value.slippage_pct
  const stratSlip = run.value?.strategy_snapshot?.cost?.slippage_pct
  if (stratSlip) return stratSlip
  return run.value?.account_snapshot?.slippage_pct || undefined
})
const effectiveCapital = computed<number | undefined>(() => {
  if (options.value.initial_capital != null) return options.value.initial_capital
  return run.value?.account_snapshot?.initial_capital || undefined
})
const commissionOverridden = computed<boolean>(() => options.value.commission_rate != null)

// ---- 指标卡片（统一风格，按语义分组） ----
interface MetricCard {
  label: string
  value: string
  color: string // 卡片 tonal 色（正绿负红或组内统一色）
}
interface MetricGroup {
  title: string
  icon: string
  color: string
  items: MetricCard[]
}

const metricGroups = computed<MetricGroup[]>(() => {
  const r = report.value
  if (!r) return []
  const sign = (v: number) => (v > 0 ? 'green' : v < 0 ? 'red' : 'grey')
  return [
    {
      title: '收益指标', icon: 'mdi-trending-up', color: 'green',
      items: [
        { label: '总收益额', value: fmtWan(r.total_profit), color: sign(r.total_profit) },
        { label: '到期收益率', value: fmtPct(r.total_return_pct), color: sign(r.total_return_pct) },
        { label: '年化收益率', value: fmtPct(r.annual_return_pct), color: sign(r.annual_return_pct) },
        { label: '最佳单期收益', value: fmtPct(r.best_day_pct), color: sign(r.best_day_pct) },
        { label: '最差单期收益', value: fmtPct(r.worst_day_pct), color: sign(r.worst_day_pct) },
        { label: '盈利期数', value: `${r.profit_days} 期`, color: 'green' },
        { label: '亏损期数', value: `${r.loss_days} 期`, color: 'red' },
      ],
    },
    {
      title: '风险指标', icon: 'mdi-shield-alert-outline', color: 'orange',
      items: [
        { label: '最大回撤', value: fmtPct(r.max_drawdown_pct), color: 'orange' },
        { label: '夏普比率', value: fmtNum(r.sharpe_ratio), color: 'orange' },
        { label: '年化波动率', value: fmtPct(r.volatility_pct), color: 'orange' },
        { label: '盈亏比', value: fmtNum(r.profit_factor), color: 'orange' },
      ],
    },
    {
      title: '交易指标', icon: 'mdi-swap-horizontal', color: 'blue',
      items: [
        { label: '交易笔数', value: `${fmtNum(r.trade_count, 0)} 笔`, color: 'blue' },
        { label: '买入笔数', value: `${r.buy_count} 笔`, color: 'blue' },
        { label: '卖出笔数', value: `${r.sell_count} 笔`, color: 'blue' },
        { label: '盈利平仓', value: `${r.win_count} 笔`, color: 'blue' },
        { label: '亏损平仓', value: `${r.loss_count} 笔`, color: 'blue' },
        { label: '胜率', value: fmtPct(r.win_rate_pct), color: 'blue' },
        { label: '持仓天数', value: `${fmtNum(r.invested_days, 0)} 天`, color: 'blue' },
      ],
    },
    {
      title: '资金指标', icon: 'mdi-cash-multiple', color: 'cyan',
      items: [
        { label: '初始启动资金', value: fmtWan(r.initial_capital), color: 'cyan' },
        { label: '期末总资产', value: fmtWan(r.final_equity), color: 'cyan' },
        { label: '最大投入', value: fmtWan(r.max_invested), color: 'cyan' },
        { label: '平均投入', value: fmtWan(r.avg_invested), color: 'cyan' },
        { label: '手续费总额', value: fmtWan(r.total_fee), color: 'cyan' },
      ],
    },
  ]
})

/** 风控复盘：从事件触发原因聚合止损/止盈/信号触发次数 */
const riskReview = computed(() => {
  const es = report.value?.event_stats
  const tr = es?.trigger_reasons ?? {}
  const count = (k: string) => tr[k] ?? 0
  const covered = ['止损', '止盈', '买入信号', '卖出信号']
  const other = Object.keys(tr).filter((k) => !covered.includes(k)).reduce((a, k) => a + (tr[k] ?? 0), 0)
  return {
    stopLoss: count('止损'),
    takeProfit: count('止盈'),
    buySignal: count('买入信号'),
    sellSignal: count('卖出信号'),
    other,
    total: es?.trigger_count ?? 0,
  }
})

// 未成交原因分布：按次数从高到低排序（[原因, 次数] 元组数组）
const sortedRejectReasons = computed<[string, number][]>(() => {
  const rr = report.value?.event_stats?.reject_reasons ?? {}
  return Object.entries(rr)
    .map(([reason, cnt]) => [reason, cnt] as [string, number])
    .sort((a, b) => b[1] - a[1])
})

const envRules = computed<KV[]>(() => envRulesKV(env.value?.config?.trading_rules))

// ---- 键值行生成（模板与 CSV 共用） ----

/** 买入/卖出规则块 → KV 行 */
function ruleKV(r: RuleDef | undefined): KV[] {
  const opts = options.value
  const rows: KV[] = []
  rows.push({ label: '方向开关', value: r?.allow === false ? '禁用' : '启用' })
  rows.push({ label: '数量方式', value: quantityText(r) })
  rows.push({ label: '每日最大触发', value: limitText(r?.max_per_day) })
  rows.push({ label: '整轮回测最大触发', value: limitText(r?.max_per_run) })
  // 限定交易时间点：任务级覆盖优先（引擎对 buy/sell 合并生效）
  const allowed = opts.allowed_times?.length ? opts.allowed_times : r?.allowed_times
  const allowedText = allowed?.length ? allowed.map(hhmmssFmt).join('、') : '不限制'
  rows.push({ label: '限定交易时间点', value: allowedText + (opts.allowed_times?.length ? '（任务覆盖）' : '') })
  rows.push({ label: '近7日最大触发', value: limitText(r?.max_per_week) })
  rows.push({ label: '近30日最大触发', value: limitText(r?.max_per_month) })
  rows.push({ label: '窗口手续费上限', value: feeWindowText(r?.max_fee_per_window, r?.fee_window_days) })
  return rows
}

/** 风控块 → KV 行 */
function riskKV(risk: RiskDef | undefined): KV[] {
  const opts = options.value
  const rows: KV[] = []
  rows.push({ label: '止损', value: risk?.stop_loss_pct ? fmtPct(risk.stop_loss_pct) : '未设置' })
  rows.push({ label: '止盈', value: risk?.take_profit_pct ? fmtPct(risk.take_profit_pct) : '未设置' })
  rows.push({ label: '单标的仓位上限', value: risk?.max_position_pct ? fmtPct(risk.max_position_pct) : '100%' })
  rows.push({ label: '最大持仓数', value: risk?.max_positions ? `${fmtNum(risk.max_positions, 0)} 个` : '不限制' })
  const maxTrades = opts.max_trades_per_day != null ? opts.max_trades_per_day : risk?.max_trades_per_day
  rows.push({ label: '每日最大成交笔数', value: limitText(maxTrades) + (opts.max_trades_per_day != null ? '（任务覆盖）' : '') })
  rows.push({ label: '最小交易间隔', value: risk?.min_interval_bars ? `${fmtNum(risk.min_interval_bars, 0)} bar` : '不限制' })
  const b = risk?.builder
  rows.push({
    label: '分批建仓',
    value: b?.enabled
      ? `启用：目标仓位 ${fmtPct(b.target_position_pct ?? 0)}，分 ${b.tranches ?? 1} 批，间隔 ${b.tranche_interval_bars ?? 0} bar`
      : '不启用',
  })
  rows.push({ label: '分批减仓份数', value: risk?.reduce_tranches ? `${fmtNum(risk.reduce_tranches, 0)} 份` : '1 份（一次性清仓）' })
  rows.push({ label: '近7日成交上限', value: limitText(risk?.max_trades_per_week) })
  rows.push({ label: '近30日成交上限', value: limitText(risk?.max_trades_per_month) })
  rows.push({ label: '窗口手续费上限', value: feeWindowText(risk?.max_fee_per_window, risk?.fee_window_days) })
  return rows
}

/** 环境交易规则 → KV 行 */
function envRulesKV(rules: EnvironmentConfig['trading_rules'] | undefined): KV[] {
  if (!rules) return []
  const rows: KV[] = []
  rows.push({ label: 'T+N 交收', value: rules.t_plus ? `T+${rules.t_plus}` : 'T+0' })
  rows.push({ label: '最小变动价位', value: rules.tick_size ? fmtNum(rules.tick_size) : '-' })
  rows.push({ label: '合约乘数', value: rules.contract_multiplier ? fmtNum(rules.contract_multiplier, 0) : '-' })
  rows.push({ label: '涨停幅度', value: rules.limit_up_pct ? fmtPct(rules.limit_up_pct) : '无限制' })
  rows.push({ label: '跌停幅度', value: rules.limit_down_pct ? fmtPct(rules.limit_down_pct) : '无限制' })
  return rows
}

/** 汇总全部背景信息为 KV 行（供 CSV 导出） */
function backgroundKV(): KV[] {
  const rows: KV[] = []
  const snap = run.value?.strategy_snapshot
  const acc = run.value?.account_snapshot
  const envSnap = env.value
  // ① 策略
  rows.push({ label: '策略', value: `${run.value?.strategy_name ?? ''}${run.value?.strategy_code ? `（${run.value.strategy_code}）` : ''}` })
  rows.push({ label: '策略类型', value: strategyTypeName(snap?.strategy_type) })
  rows.push({ label: '策略描述', value: primaryStrategyDesc.value })
  if (strategyDescNote.value?.text) rows.push({ label: '当前策略说明', value: strategyDescNote.value.text })
  rows.push({ label: '数据配置', value: dataConfigText(snap?.data) })
  rows.push({ label: '指标', value: snap?.indicators?.length ? snap.indicators.map(indChipText).join(' | ') : '未配置指标' })
  rows.push({ label: '买卖信号', value: signalExplainText(snap?.signals) })
  // ② 标的与市场
  rows.push({ label: '标的代码', value: run.value?.secu_code ?? '-' })
  rows.push({ label: '市场代码', value: run.value?.market_code ? fmtNum(run.value.market_code, 0) : '-' })
  rows.push({ label: '数据周期', value: periodName(run.value?.period) })
  rows.push({ label: '报告精度', value: report.value?.report_precision ?? '-' })
  rows.push({ label: '回测区间', value: `${fmtDate(run.value?.start_date)} ~ ${fmtDate(run.value?.end_date)}` })
  rows.push({ label: 'K线数量', value: report.value?.bar_count ? `${fmtNum(report.value.bar_count, 0)} 根` : '-' })
  // ③ 账户
  rows.push({ label: '账户', value: accountText.value })
  rows.push({ label: '初始资金', value: fmtWan(effectiveCapital.value) + (options.value.initial_capital != null ? '（任务覆盖：本次回测在任务上单独指定了初始资金，覆盖了策略/账户的默认值）' : '') })
  rows.push({ label: '手续费率', value: pctRatio(effectiveCommission.value) + (commissionOverridden.value ? '（任务覆盖）' : '') })
  rows.push({ label: '滑点', value: pctRatio(effectiveSlippage.value) })
  rows.push({ label: '保证金模式', value: marginModeName(acc?.margin_mode) + (acc?.margin_rate ? `（${pctRatio(acc.margin_rate)}）` : '') })
  rows.push({ label: '计价币种', value: acc?.currency_type ?? '-' })
  // ④ 环境
  if (envSnap) {
    rows.push({ label: '环境', value: `${envSnap.env_name}${envSnap.env_code ? `（${envSnap.env_code}）` : ''}` })
    rows.push({ label: '环境类型', value: envTypeName(envSnap.env_type) })
    rows.push({ label: '地区', value: envSnap.region || '-' })
    rows.push({ label: '交易时段', value: sessionsFmt(envSnap.config?.trading_sessions) })
    rows.push({ label: '撮合模式', value: fillModeExplain(envSnap.config?.fill_mode ?? snap?.data?.fill_mode) })
    rows.push({ label: '环境币种', value: envSnap.config?.currency ?? '-' })
    if (envSnap.description) rows.push({ label: '环境说明', value: envSnap.description })
  } else {
    rows.push({ label: '环境', value: '未启用环境（按策略/账户配置运行）' })
  }
  // ⑤ 交易规则与限制
  for (const dir of ['buy', 'sell'] as const) {
    const dn = dir === 'buy' ? '买入' : '卖出'
    for (const kv of ruleKV(snap?.rules?.[dir])) rows.push({ label: `${dn}·${kv.label}`, value: kv.value })
  }
  for (const kv of riskKV(snap?.risk)) rows.push({ label: `风控·${kv.label}`, value: kv.value })
  for (const kv of envRulesKV(envSnap?.config?.trading_rules)) rows.push({ label: `环境·${kv.label}`, value: kv.value })
  return rows
}

/** 拉取全量净值点（Equity 接口单页上限 5000，循环翻页直至 total，上限保护 20000 点） */
async function loadAllEquity(runId: string): Promise<EquityPoint[]> {
  const all: EquityPoint[] = []
  let page = 1
  let total = Infinity
  while (all.length < total && page <= 5) {
    const r = await apiGet<{ total: number; list: EquityPoint[] }>(
      `/Meta/Finv/Quant/Backtest/Run/Equity?runId=${runId}&page=${page}&pageSize=5000`,
    )
    all.push(...(r.list ?? []))
    total = r.total ?? all.length
    page++
  }
  return all
}

async function loadReport(runId: string) {
  loading.value = true
  error.value = ''
  disposeCharts()
  report.value = null
  strategyDetail.value = null
  equity.value = []
  trades.value = []
  cashflows.value = []
  positionLogs.value = []
  eventTraces.value = []
  try {
    // 先拿任务完整信息（含策略/账户/环境快照），再并行加载报告与明细
    const r = await apiGet<RunDetail>(
      `/Meta/Finv/Quant/Backtest/Run/Get?runId=${encodeURIComponent(runId)}`,
    )
    if (r.status !== 'SUCCEEDED') {
      error.value = `任务当前状态为 ${r.status ?? '未知'}，仅 SUCCEEDED 任务可查看报告`
      return
    }
    run.value = r
    const [rep, eq, tr, cf, pl, ev, st] = await Promise.all([
      apiGet<Report>(`/Meta/Finv/Quant/Backtest/Run/Report?runId=${runId}`),
      loadAllEquity(runId),
      apiGet<{ total: number; list: TradeRow[] }>(`/Meta/Finv/Quant/Backtest/Run/Trades?runId=${runId}&page=1&pageSize=${tradePageSize.value}`),
      apiGet<{ total: number; list: CashflowRow[] }>(`/Meta/Finv/Quant/Backtest/Run/Cashflows?runId=${runId}&page=1&pageSize=${cashflowPageSize.value}`),
      apiGet<{ total: number; list: PositionLogRow[] }>(`/Meta/Finv/Quant/Backtest/Run/PositionLogs?runId=${runId}&page=1&pageSize=${positionLogPageSize.value}`),
      apiGet<{ total: number; list: EventTraceRow[] }>(`/Meta/Finv/Quant/Backtest/Run/EventTraces?runId=${runId}&page=1&pageSize=${eventTracePageSize.value}`),
      // 策略当前说明（Strategy/Get，失败不阻塞报告）
      r.strategy_id
        ? apiGet<StrategyDetail>(`/Meta/Finv/Quant/Backtest/Strategy/Get?strategyId=${encodeURIComponent(r.strategy_id)}`).catch(() => null)
        : Promise.resolve(null),
    ])
    report.value = rep
    equity.value = eq
    trades.value = tr.list ?? []
    tradeTotal.value = tr.total ?? 0
    cashflows.value = cf.list ?? []
    cashflowTotal.value = cf.total ?? 0
    positionLogs.value = pl.list ?? []
    positionLogTotal.value = pl.total ?? 0
    eventTraces.value = ev.list ?? []
    eventTraceTotal.value = ev.total ?? 0
    strategyDetail.value = st
    await nextTick()
    renderCharts()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function xLabels(): string[] {
  const precision = report.value?.report_precision ?? 'Day'
  return equity.value.map((p) => {
    if (precision === 'Min') {
      return `${fmtDate(p.date)} ${fmtTime(p.time)}`
    }
    if (precision === 'Hour') {
      const h = Math.floor((p.time || 0) / 10000)
      return `${fmtDate(p.date)} ${String(h).padStart(2, '0')}时`
    }
    return fmtDate(p.date)
  })
}

function mkChart(key: string, el: HTMLDivElement | null): echarts.ECharts | null {
  if (!el) return null
  const chart = echarts.init(el)
  charts.push({ chart, key })
  return chart
}

function renderCharts() {
  if (!equity.value.length) return
  const labels = xLabels()
  // 刻度自适应：按容器宽度算显示密度，按回测跨度选刻度精度，文字不重叠
  const hostW = chartEls.equity?.clientWidth ?? 800
  const maxTicks = Math.max(4, Math.floor(hostW / 76))
  const tickStep = Math.max(1, Math.ceil(labels.length / maxTicks))
  const ts0 = equity.value[0]?.ts ?? 0
  const ts1 = equity.value[equity.value.length - 1]?.ts ?? 0
  const spanDays = Math.max(1, (ts1 - ts0) / 86400)
  // 每年首个数据点的索引：跨年时在该刻度强制标注年份（不依赖 1 月数据是否存在）
  const yearStartIdx = new Map<number, number>()
  labels.forEach((l, i) => {
    const y = Number(l.slice(0, 4))
    if (!Number.isNaN(y) && !yearStartIdx.has(y)) yearStartIdx.set(y, i)
  })
  const yearStartIndexes = new Set(yearStartIdx.values())
  // 候选刻度 = tickStep 均匀跳显 + 每年首个刻度（升序合并去重）
  const stepIdx: number[] = []
  for (let i = 0; i < labels.length; i += tickStep) stepIdx.push(i)
  const candidates = [...new Set([...stepIdx, ...yearStartIndexes])].sort((a, b) => a - b)
  // 两两间隔不小于 tickStep；年份刻度与前一均匀刻度间距不足时，替换掉前一个刻度以保证年份一定显示
  const shownSet = new Set<number>()
  let lastShown = -Infinity
  let lastWasYearStart = false
  for (const i of candidates) {
    const isYearStart = yearStartIndexes.has(i)
    if (i - lastShown >= tickStep) {
      shownSet.add(i)
      lastShown = i
      lastWasYearStart = isYearStart
    } else if (isYearStart && !lastWasYearStart) {
      shownSet.delete(lastShown)
      shownSet.add(i)
      lastShown = i
      lastWasYearStart = true
    }
  }
  // 年份/月份/日期分开标注：跨年大跨度单行年份；跨月“年份\n月份”两行；短跨度“月-日\n时间”两行
  const fmtTick = (v: string, index: number): string => {
    const d = v.slice(0, 10)
    const t = v.slice(10).trim()
    if (spanDays >= 1000) return d.slice(0, 4) // 大跨度：单行年份
    if (spanDays >= 300) {
      // 中跨度：年份只在新一年的首个刻度标注一次，其余刻度只显示月份；两行标注“月份在上、年份在下”
      const m = d.slice(5, 7)
      return yearStartIndexes.has(index) ? `${m}\n${d.slice(0, 4)}` : m
    }
    return `${d.slice(5, 7)}-${d.slice(8, 10)}${t ? '\n' + t : ''}` // 短跨度：月-日/时间两行
  }
  // 网格线统一为细淡色
  const gridLine = { color: 'rgba(128,128,128,0.18)', width: 0.5 }
  const base = (unit: string) => ({
    // 图表内部不再绘制标题（卡片标题已展示，避免两处重复）
    tooltip: { trigger: 'axis' },
    grid: { left: 70, right: 20, top: 40, bottom: 64 },
    xAxis: {
      type: 'category',
      data: labels,
      // 竖直网格线（x 轴刻度对应的竖线）
      splitLine: { show: true, lineStyle: gridLine },
      axisTick: { show: true, lineStyle: { color: 'rgba(128,128,128,0.4)' } },
      axisLabel: {
        fontSize: 10,
        lineHeight: 13,
        // 每年首个刻度强制显示年份，其余按 tickStep 跳显；间隔已保证不重叠，无需 hideOverlap 二次过滤
        interval: (index: number) => shownSet.has(index),
        formatter: (v: string, index: number) => fmtTick(v, index),
      },
    },
    yAxis: {
      type: 'value', scale: true, name: unit,
      // 水平网格线：细淡色
      splitLine: { lineStyle: gridLine },
      // 刻度值不做千分位/缩写，直接展示原始数值
      axisLabel: { formatter: (v: number) => String(v) },
    },
    dataZoom: [
      { type: 'inside', xAxisIndex: 0 },
      {
        type: 'slider', xAxisIndex: 0, height: 16, bottom: 6, start: 0, end: 100,
        // 滑块调暗，避免过亮
        borderColor: 'rgba(128,128,128,0.25)',
        backgroundColor: 'rgba(128,128,128,0.05)',
        fillerColor: 'rgba(110,130,180,0.15)',
        selectedDataBackground: { lineStyle: { color: 'rgba(110,130,180,0.35)' }, areaStyle: { color: 'rgba(110,130,180,0.06)' } },
        dataBackground: { lineStyle: { color: 'rgba(128,128,128,0.3)' }, areaStyle: { color: 'rgba(128,128,128,0.04)' } },
        handleStyle: { borderColor: 'rgba(128,128,128,0.4)' },
        moveHandleStyle: { color: 'rgba(128,128,128,0.22)' },
        textStyle: { color: 'rgba(128,128,128,0.7)', fontSize: 10 },
      },
    ],
  })
  const mk = (key: string, title: string, unit: string, color: string, area: boolean, field: keyof EquityPoint) => {
    const c = mkChart(key, chartEls[key])
    c?.setOption({
      ...base(unit),
      series: [{
        name: title,
        type: 'line',
        data: equity.value.map((p) => (p[field] == null ? null : Number(p[field]))),
        showSymbol: false,
        // 线宽减小到原来的 70%（2 → 1.4）
        lineStyle: { width: 1.4 },
        itemStyle: { color },
        areaStyle: area ? { opacity: 0.06 } : undefined,
      }],
    })
  }
  // 账户余额拆分为 3 张独立曲线 + 收益/持仓数量（标题无序号）
  mk('equity', '总资产曲线', '', '#1565c0', false, 'equity')
  mk('cash', '现金曲线', '', '#90a4ae', false, 'cash')
  mk('positionValue', '持仓市值曲线', '', '#6a1b9a', true, 'position_value')
  mk('roi', '投资收益率曲线', '%', '#2e7d32', true, 'roi')
  mk('profit', '累计收益额曲线', '', '#ef6c00', true, 'profit')
  mk('positionQty', '持仓数量变化曲线', '份', '#00838f', false, 'position_qty')
}

// 全屏放大：复用主图表配置渲染到 dialog 内
function openFullscreen(key: string) {
  fullscreenKey.value = key
  fullscreenDialog.value = true
  nextTick(() => {
    const el = fullscreenChartEl.value
    if (!el) return
    const src = charts.find((c) => c.key === key)?.chart
    const f = echarts.init(el)
    if (src) {
      f.setOption(JSON.parse(JSON.stringify(src.getOption())))
    }
    f.resize()
    // 记录到独立变量，关闭时 dispose
    fullscreenChart = f
  })
}

let fullscreenChart: echarts.ECharts | null = null

function closeFullscreen() {
  if (fullscreenChart) {
    fullscreenChart.dispose()
    fullscreenChart = null
  }
  fullscreenDialog.value = false
  fullscreenKey.value = ''
  // 重新触发主图表 resize（布局恢复后）
  setTimeout(() => charts.forEach((c) => c.chart.resize()), 50)
}

function disposeCharts() {
  charts.forEach((c) => c.chart.dispose())
  charts.length = 0
}

function resizeCharts() {
  charts.forEach((c) => c.chart.resize())
  fullscreenChart?.resize()
}

async function loadTradesPage() {
  if (!run.value) return
  try {
    const tr = await apiGet<{ total: number; list: TradeRow[] }>(
      `/Meta/Finv/Quant/Backtest/Run/Trades?runId=${run.value.run_id}&page=${tradePage.value}&pageSize=${tradePageSize.value}`,
    )
    trades.value = tr.list ?? []
    tradeTotal.value = tr.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadCashflowPage() {
  if (!run.value) return
  try {
    const cf = await apiGet<{ total: number; list: CashflowRow[] }>(
      `/Meta/Finv/Quant/Backtest/Run/Cashflows?runId=${run.value.run_id}&page=${cashflowPage.value}&pageSize=${cashflowPageSize.value}`,
    )
    cashflows.value = cf.list ?? []
    cashflowTotal.value = cf.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadPositionLogPage() {
  if (!run.value) return
  try {
    const pl = await apiGet<{ total: number; list: PositionLogRow[] }>(
      `/Meta/Finv/Quant/Backtest/Run/PositionLogs?runId=${run.value.run_id}&page=${positionLogPage.value}&pageSize=${positionLogPageSize.value}`,
    )
    positionLogs.value = pl.list ?? []
    positionLogTotal.value = pl.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadEventTracePage() {
  if (!run.value) return
  try {
    const ev = await apiGet<{ total: number; list: EventTraceRow[] }>(
      `/Meta/Finv/Quant/Backtest/Run/EventTraces?runId=${run.value.run_id}&page=${eventTracePage.value}&pageSize=${eventTracePageSize.value}`,
    )
    eventTraces.value = ev.list ?? []
    eventTraceTotal.value = ev.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  }
}

// ---- 明细表头（4 张独立表格，v-data-table-server 服务端分页） ----
const cashflowHeaders = [
  { title: '序号', key: 'seq', width: 60 },
  { title: '时间', key: 'timeText', width: 158 },
  { title: '类型', key: 'flow_type', width: 104 },
  { title: '金额', key: 'amount', width: 148 },
  { title: '变动前现金', key: 'cash_before', width: 148 },
  { title: '变动后现金', key: 'cash_after', width: 148 },
  { title: '关联成交', key: 'trade_id', width: 96 },
  // 备注列限宽，避免在 fixed 布局下独占剩余宽度把其他列挤窄
  { title: '备注', key: 'remark', width: 220 },
]
const positionLogHeaders = [
  { title: '序号', key: 'seq', width: 60 },
  { title: '时间', key: 'timeText', width: 158 },
  { title: '动作', key: 'action', width: 80 },
  { title: '价格', key: 'price', width: 120 },
  { title: '变动数量', key: 'qty', width: 120 },
  { title: '持仓前', key: 'position_before', width: 120 },
  { title: '持仓后', key: 'position_after', width: 120 },
  { title: '成本前', key: 'avg_cost_before', width: 120 },
  { title: '成本后', key: 'avg_cost_after', width: 120 },
  { title: '关联成交', key: 'trade_id', width: 96 },
  // 备注列限宽，避免在 fixed 布局下独占剩余宽度把其他列挤窄
  { title: '备注', key: 'remark', width: 200 },
]
const tradeHeaders = [
  { title: '序号', key: 'seq', width: 70 },
  { title: '时间', key: 'timeText', width: 170 },
  { title: '方向', key: 'action', width: 80 },
  { title: '价格', key: 'price', width: 100 },
  { title: '数量', key: 'qty', width: 100 },
  { title: '金额', key: 'amount', width: 120 },
  { title: '手续费', key: 'fee', width: 100 },
  { title: '盈亏', key: 'profit', width: 110 },
  { title: '持仓后', key: 'position_after', width: 100 },
  { title: '信号', key: 'signal' },
  { title: '备注', key: 'remark', width: 110 },
]
const eventTraceHeaders = [
  { title: '序号', key: 'seq', width: 70 },
  { title: '方向', key: 'action', width: 80 },
  { title: '触发原因', key: 'trigger_reason' },
  { title: '触发时间', key: 'trigger_date', width: 160 },
  { title: '委托下单时间', key: 'order_date', width: 160 },
  { title: '结果', key: 'exec_status', width: 90 },
  { title: '成交时间', key: 'exec_date', width: 160 },
  { title: '委托耗时', key: 'latency', width: 110 },
  { title: '存活时间', key: 'alive', width: 90 },
  { title: '未成交原因', key: 'reject_reason' },
  { title: '关联成交', key: 'trade_id', width: 90 },
]

// ---------------------------------------------------------------------
// 导出（需求：报告底部结构化数据导出 JSON / CSV）
// ---------------------------------------------------------------------
function downloadBlob(filename: string, content: string, mime: string) {
  const blob = new Blob(['﻿' + content], { type: mime + ';charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

/** 拉取全量明细（4 张表，循环翻页直至 total），供导出使用（当前页 ref 只含一页） */
async function fetchAllDetails(): Promise<{
  trades: TradeRow[]
  cashflows: CashflowRow[]
  positionLogs: PositionLogRow[]
  eventTraces: EventTraceRow[]
}> {
  const runId = run.value?.run_id
  if (!runId) return { trades: [], cashflows: [], positionLogs: [], eventTraces: [] }
  const pageOf = async <T>(path: string): Promise<T[]> => {
    const all: T[] = []
    let page = 1
    let total = Infinity
    while (all.length < total && page <= 20) {
      const r = await apiGet<{ total: number; list: T[] }>(
        `/Meta/Finv/Quant/Backtest/Run/${path}?runId=${runId}&page=${page}&pageSize=5000`,
      )
      all.push(...(r.list ?? []))
      total = r.total ?? all.length
      page++
    }
    return all
  }
  const [trades, cashflows, positionLogs, eventTraces] = await Promise.all([
    pageOf<TradeRow>('Trades'),
    pageOf<CashflowRow>('Cashflows'),
    pageOf<PositionLogRow>('PositionLogs'),
    pageOf<EventTraceRow>('EventTraces'),
  ])
  return { trades, cashflows, positionLogs, eventTraces }
}

async function exportJSON() {
  const all = await fetchAllDetails()
  const payload = {
    run: run.value,
    strategy_detail: strategyDetail.value,
    report: report.value,
    equity_points: equity.value,
    trades: all.trades,
    cashflows: all.cashflows,
    position_logs: all.positionLogs,
    event_traces: all.eventTraces,
  }
  downloadBlob(`finvquant_report_${run.value?.run_no ?? 'run'}_${Date.now()}.json`, JSON.stringify(payload, null, 2), 'application/json')
}

function csvEscape(v: unknown): string {
  const s = String(v ?? '')
  if (s.includes(',') || s.includes('"') || s.includes('\n')) {
    return '"' + s.replaceAll('"', '""') + '"'
  }
  return s
}

async function exportCSV() {
  const all = await fetchAllDetails()
  const lines: string[] = []
  const hr = () => lines.push('')
  // 1) 任务与报告主指标（key,value）
  lines.push('== 任务与汇总指标 ==')
  lines.push('字段,值')
  if (run.value) {
    lines.push(csvEscape('任务号'), csvEscape(run.value.run_no))
    lines.push(csvEscape('任务ID'), csvEscape(run.value.run_id))
    lines.push(csvEscape('策略'), csvEscape(run.value.strategy_name))
  }
  if (report.value) {
    for (const [k, v] of Object.entries(report.value)) {
      if (k === 'event_stats' || k === 'trade_signal_detail') continue
      lines.push(`${csvEscape(k)},${csvEscape(v)}`)
    }
  }
  // 2) 回测背景信息（字段,值）
  hr()
  lines.push('== 回测背景 ==')
  lines.push('字段,值')
  for (const kv of backgroundKV()) {
    lines.push(`${csvEscape(kv.label)},${csvEscape(kv.value)}`)
  }
  // 3) 未成交原因分布
  if (report.value?.event_stats?.reject_reasons) {
    hr()
    lines.push('== 未成交原因分布 ==')
    lines.push('原因,计数,说明')
    for (const [reason, cnt] of Object.entries(report.value.event_stats.reject_reasons)) {
      lines.push(`${csvEscape(reason)},${cnt},${csvEscape(rejectReasonHint(reason))}`)
    }
  }
  // 4) 资金流水明细
  hr()
  lines.push('== 资金流水明细 ==')
  lines.push('序号,时间,类型,金额,变动前现金,变动后现金,关联成交,备注')
  for (const cf of all.cashflows) {
    lines.push([cf.seq, `${fmtDate(cf.date)} ${fmtTime(cf.time)}`, flowTypeName(cf.flow_type), cf.amount, cf.cash_before, cf.cash_after, cf.trade_id ? '#' + cf.trade_id : '', cf.remark].map(csvEscape).join(','))
  }
  // 5) 持仓变化明细
  hr()
  lines.push('== 持仓变化明细 ==')
  lines.push('序号,时间,动作,价格,数量,持仓前,持仓后,成本前,成本后,备注')
  for (const p of all.positionLogs) {
    lines.push([p.seq, `${fmtDate(p.date)} ${fmtTime(p.time)}`, p.action, p.price, p.qty, p.position_before, p.position_after, p.avg_cost_before, p.avg_cost_after, p.remark].map(csvEscape).join(','))
  }
  // 6) 成交记录
  hr()
  lines.push('== 成交记录 ==')
  lines.push('序号,时间,方向,价格,数量,金额,手续费,盈亏,持仓后,信号')
  for (const t of all.trades) {
    lines.push([t.seq, `${fmtDate(t.date)} ${fmtTime(t.time)}`, t.action, t.price, t.qty, t.amount, t.fee, t.profit, t.position_after, t.signal].map(csvEscape).join(','))
  }
  // 7) 事件追踪
  hr()
  lines.push('== 事件追踪 ==')
  lines.push('序号,方向,触发原因,触发时间,结果,未成交原因')
  for (const ev of all.eventTraces) {
    lines.push([ev.seq, ev.action, ev.trigger_reason, `${fmtDate(ev.trigger_date)} ${fmtTime(ev.trigger_time)}`, ev.exec_status, ev.reject_reason || ''].map(csvEscape).join(','))
  }
  downloadBlob(`finvquant_report_${run.value?.run_no ?? 'run'}_${Date.now()}.csv`, lines.join('\n'), 'text/csv')
}

onMounted(async () => {
  window.addEventListener('resize', resizeCharts)
  const runId = route.query.runId as string | undefined
  if (runId) {
    await loadReport(runId)
  } else {
    error.value = '缺少 runId 参数，请从「回测分析」任务列表选择任务查看报告'
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeCharts)
  disposeChartObservers()
  disposeCharts()
  if (fullscreenChart) {
    fullscreenChart.dispose()
    fullscreenChart = null
  }
})
</script>

<template>
  <v-container fluid>
    <v-alert v-if="error" type="error" dismissible class="mb-3">
      {{ error }}
      <template v-if="route.query.runId">
        <v-btn size="small" variant="text" color="error" class="ml-2" @click="router.push('/Meta/Finv/Quant/Backtest/Analysis')">
          返回任务列表
        </v-btn>
      </template>
    </v-alert>

    <v-card :loading="loading" v-if="!report">
      <v-card-text class="text-center py-12 text-medium-emphasis">
        <v-icon icon="mdi-chart-line" size="56" class="mb-3" />
        <p class="text-h6">正在加载投资策略回测收益分析报告…</p>
      </v-card-text>
    </v-card>

    <template v-if="report">
      <!-- 返回按钮：独立贴边展示，不与标题同行 -->
      <v-btn size="small" icon="mdi-arrow-left" variant="tonal" color="primary" class="mb-2"
        title="返回任务列表" @click="router.push('/Meta/Finv/Quant/Backtest/Analysis')" />

      <!-- 报告头：大标题独立一行（标的/周期/区间等字段在下方「回测背景信息」中展示，顶部不重复） -->
      <v-card class="mb-3">
        <v-card-title class="text-h5 font-weight-bold d-flex align-center flex-wrap">
          <v-icon icon="mdi-file-chart" class="mr-2" color="primary" />
          投资策略回测收益分析报告
          <span v-if="run?.run_no || run?.run_id" class="text-body-1 text-medium-emphasis ml-2">
            （{{ run.run_no || run.run_id }}）
          </span>
        </v-card-title>
      </v-card>

      <!-- 回测背景信息（平铺展开，5 小节） -->
      <v-card class="mb-3">
        <v-card-title class="pb-0 d-flex align-center flex-wrap">
          <v-icon icon="mdi-clipboard-text-clock-outline" class="mr-2" color="primary" />
          回测背景信息
          <v-chip v-if="strategyDescNote" size="small" color="warning" variant="tonal" class="ml-2"
            prepend-icon="mdi-alert">{{ strategyDescNote.label }}</v-chip>
        </v-card-title>
        <v-card-text>
          <v-row class="mt-2">
            <!-- ② 标的与市场 -->
            <v-col cols="12">
              <div class="text-subtitle-2 font-weight-bold mb-1">
                <v-icon icon="mdi-chart-box-outline" size="small" class="mr-1" />标的与市场
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">标的代码</span>
                <span class="text-body-2">{{ run?.secu_code || '-' }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">市场代码</span>
                <span class="text-body-2">{{ run?.market_code ? fmtNum(run?.market_code, 0) : '-' }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">数据周期</span>
                <span class="text-body-2">{{ periodName(run?.period) }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">报告精度</span>
                <span class="text-body-2">{{ report.report_precision || '-' }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">回测区间</span>
                <span class="text-body-2">{{ fmtDate(run?.start_date) }} ~ {{ fmtDate(run?.end_date) }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">K线数量</span>
                <span class="text-body-2">{{ fmtNum(report.bar_count, 0) }} 根</span>
              </div>
            </v-col>

            <!-- ③ 账户配置 -->
            <v-col cols="12">
              <div class="text-subtitle-2 font-weight-bold mb-1">
                <v-icon icon="mdi-bank-outline" size="small" class="mr-1" />账户配置
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">账户</span>
                <span class="text-body-2">{{ accountText }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">初始资金</span>
                <span class="text-body-2 d-flex align-center">
                  {{ fmtWan(effectiveCapital) }}
                  <v-chip v-if="options.initial_capital != null" size="x-small" color="primary" variant="tonal" class="ml-1">任务覆盖</v-chip>
                </span>
              </div>
              <div v-if="options.initial_capital != null" class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">说明</span>
                <span class="text-body-2 text-medium-emphasis">任务配置中单独指定了初始资金，优先于策略/账户默认值参与本轮回测。</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">手续费率</span>
                <span class="text-body-2 d-flex align-center">
                  {{ pctRatio(effectiveCommission) }}
                  <v-chip v-if="commissionOverridden" size="x-small" color="primary" variant="tonal" class="ml-1">任务覆盖</v-chip>
                </span>
              </div>
              <div v-if="commissionOverridden" class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">说明</span>
                <span class="text-body-2 text-medium-emphasis">任务配置中单独指定了手续费率，优先于策略/账户默认值参与本轮回测。</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">滑点</span>
                <span class="text-body-2">{{ pctRatio(effectiveSlippage) }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">保证金模式</span>
                <span class="text-body-2">{{ marginModeName(run?.account_snapshot?.margin_mode) }}{{ run?.account_snapshot?.margin_rate ? `（${pctRatio(run?.account_snapshot.margin_rate)}）` : '' }}</span>
              </div>
              <div class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">计价币种</span>
                <span class="text-body-2">{{ run?.account_snapshot?.currency_type || '-' }}</span>
              </div>
            </v-col>
          </v-row>

          <v-row>
            <!-- ④ 环境配置 -->
            <v-col cols="12">
              <div class="text-subtitle-2 font-weight-bold mb-1">
                <v-icon icon="mdi-server-outline" size="small" class="mr-1" />环境配置
              </div>
              <template v-if="env">
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">环境</span>
                  <span class="text-body-2">{{ env.env_name }}{{ env.env_code ? `（${env.env_code}）` : '' }}</span>
                </div>
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">环境类型</span>
                  <span class="text-body-2">{{ envTypeName(env.env_type) }}</span>
                </div>
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">地区</span>
                  <span class="text-body-2">{{ env.region || '-' }}</span>
                </div>
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">交易时段</span>
                  <span class="text-body-2">{{ sessionsFmt(env.config?.trading_sessions) }}</span>
                </div>
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">撮合模式</span>
                  <span class="text-body-2">{{ fillModeExplain(env.config?.fill_mode ?? run?.strategy_snapshot?.data?.fill_mode) }}</span>
                </div>
                <div class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">币种</span>
                  <span class="text-body-2">{{ env.config?.currency || '-' }}</span>
                </div>
                <div v-if="env.description" class="d-flex align-center py-1">
                  <span class="text-caption text-medium-emphasis kv-label">环境说明</span>
                  <span class="text-body-2">{{ env.description }}</span>
                </div>
              </template>
              <v-alert v-else density="compact" type="info" variant="tonal">
                本次回测未记录环境快照，按策略/账户配置运行。
              </v-alert>
            </v-col>
          </v-row>

          <!-- ⑤ 交易规则与限制 -->
          <div class="text-subtitle-2 font-weight-bold mt-1 mb-1">
            <v-icon icon="mdi-list-status" size="small" class="mr-1" />交易规则与限制
          </div>
          <template v-if="hasStrategyDef">
            <div class="text-caption font-weight-bold mb-1">买入规则</div>
            <div v-for="(kv, i) in ruleKV(snapRules?.buy)" :key="'buy-' + i" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">{{ kv.label }}</span>
              <span class="text-body-2">{{ kv.value }}</span>
            </div>
            <div class="text-caption font-weight-bold mb-1 mt-2">卖出规则</div>
            <div v-for="(kv, i) in ruleKV(snapRules?.sell)" :key="'sell-' + i" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">{{ kv.label }}</span>
              <span class="text-body-2">{{ kv.value }}</span>
            </div>
            <div class="text-caption font-weight-bold mb-1 mt-2">风控与频率限制</div>
            <div v-for="(kv, i) in riskKV(run?.strategy_snapshot?.risk)" :key="'risk-' + i" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">{{ kv.label }}</span>
              <span class="text-body-2">{{ kv.value }}</span>
            </div>
            <div v-if="envRules.length" class="mt-2">
              <div class="text-caption font-weight-bold mb-1">环境交易规则（T+N / 涨跌停 / 精度）</div>
              <div v-for="(kv, i) in envRules" :key="'envrule-' + i" class="d-flex align-center py-1">
                <span class="text-caption text-medium-emphasis kv-label">{{ kv.label }}</span>
                <span class="text-body-2">{{ kv.value }}</span>
              </div>
            </div>
          </template>
          <v-alert v-else density="compact" type="info" variant="tonal">
            无策略定义快照，无法展示交易规则与限制。
          </v-alert>

          <!-- 策略信息（放背景信息最后展示） -->
          <div class="text-subtitle-2 font-weight-bold mt-3 mb-1">
            <v-icon icon="mdi-account-cog-outline" size="small" class="mr-1" />策略信息
          </div>
          <template v-if="hasStrategyDef">
            <div class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">策略</span>
              <span class="text-body-2">{{ run?.strategy_name }}{{ run?.strategy_code ? `（${run?.strategy_code}）` : '' }}</span>
            </div>
            <div class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">策略类型</span>
              <span class="text-body-2">{{ strategyTypeName(run?.strategy_snapshot?.strategy_type) }}</span>
            </div>
            <div v-if="strategyOverview" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">策略概览</span>
              <span class="text-body-2">{{ strategyOverview }}</span>
            </div>
            <div v-if="primaryStrategyDesc && primaryStrategyDesc !== strategyOverview" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">策略原始说明</span>
              <span class="text-body-2 text-medium-emphasis">{{ primaryStrategyDesc }}</span>
            </div>
            <div v-if="strategyDescNote" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">当前策略说明</span>
              <span class="text-body-2">{{ strategyDescNote.text }}</span>
            </div>
            <div class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">数据配置</span>
              <span class="text-body-2">{{ dataConfigText(run?.strategy_snapshot?.data) }}</span>
            </div>
            <div v-if="indicators.length" class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">指标</span>
              <span class="text-body-2 d-flex flex-wrap ga-1">
                <v-chip v-for="ind in indicators" :key="ind.id ?? ind.type ?? indChipText(ind)" size="x-small" variant="outlined">
                  {{ indChipText(ind) }}
                </v-chip>
              </span>
            </div>
            <div class="d-flex align-center py-1">
              <span class="text-caption text-medium-emphasis kv-label">买卖信号</span>
              <span class="text-body-2">{{ signalExplainText(run?.strategy_snapshot?.signals) }}</span>
            </div>
          </template>
          <v-alert v-else density="compact" type="info" variant="tonal">
            本次回测未记录策略定义快照（策略可能已删除或定义为空），仅展示任务与报告信息。
          </v-alert>
        </v-card-text>
      </v-card>

      <!-- 曲线图（全宽 + 放大/全屏；标题无序号） -->
      <v-row>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">总资产曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('equity')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('equity', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">现金曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('cash')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('cash', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">持仓市值曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('positionValue')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('positionValue', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">投资收益率曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('roi')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('roi', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">累计收益额曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('profit')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('profit', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="12">
          <v-card class="mb-3">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              <span class="flex-grow-1">持仓数量变化曲线</span>
              <v-btn size="small" variant="text" icon="mdi-fullscreen" title="放大/全屏" @click="openFullscreen('positionQty')" />
            </v-card-title>
            <v-card-text class="pt-0">
              <div :ref="el => setChartRef('positionQty', el)" style="height: 320px" />
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <!-- 链路追踪明细：4 个独立表格按顺序排列（服务端分页） -->
      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-cash-multiple" class="mr-2" color="primary" />资金流水明细（{{ cashflowTotal }}）
        </v-card-title>
        <v-data-table-server v-model:page="cashflowPage" v-model:items-per-page="cashflowPageSize"
          :headers="cashflowHeaders" :items="cashflows" :items-length="cashflowTotal" item-value="cashflow_id"
          @update:options="loadCashflowPage" density="compact">
          <template #item.timeText="{ item }">
            {{ fmtDate(item.date) }} {{ String(item.time).padStart(6, '0').slice(0, 2) }}:{{ String(item.time).padStart(6, '0').slice(2, 4) }}
          </template>
          <template #item.flow_type="{ item }">
            <v-chip size="x-small" :color="flowTypeColor(item.flow_type)">{{ flowTypeName(item.flow_type) }}</v-chip>
          </template>
          <template #item.amount="{ item }">
            <span :class="item.amount < 0 ? 'text-error' : 'text-success'">{{ fmtWan(item.amount) }}</span>
          </template>
          <template #item.cash_before="{ item }">
            <span>{{ fmtWan(item.cash_before) }}</span>
          </template>
          <template #item.cash_after="{ item }">
            <span>{{ fmtWan(item.cash_after) }}</span>
          </template>
          <template #item.trade_id="{ item }">
            <span>{{ item.trade_id ? '#' + item.trade_id : '-' }}</span>
          </template>
        </v-data-table-server>
      </v-card>

      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-briefcase-variant-outline" class="mr-2" color="primary" />持仓变化明细（{{ positionLogTotal }}）
        </v-card-title>
        <v-data-table-server v-model:page="positionLogPage" v-model:items-per-page="positionLogPageSize"
          :headers="positionLogHeaders" :items="positionLogs" :items-length="positionLogTotal" item-value="log_id"
          @update:options="loadPositionLogPage" density="compact">
          <template #item.timeText="{ item }">
            {{ fmtDate(item.date) }} {{ String(item.time).padStart(6, '0').slice(0, 2) }}:{{ String(item.time).padStart(6, '0').slice(2, 4) }}
          </template>
          <template #item.action="{ item }">
            <v-chip size="x-small" :color="posActionColor(item.action)">{{ posActionName(item.action) }}</v-chip>
          </template>
          <template #item.qty="{ item }">
            <span :class="item.qty < 0 ? 'text-error' : 'text-success'">{{ fmtNum(item.qty, 4) }}</span>
          </template>
          <template #item.trade_id="{ item }">
            <span>{{ item.trade_id ? '#' + item.trade_id : '-' }}</span>
          </template>
        </v-data-table-server>
      </v-card>

      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-swap-horizontal" class="mr-2" color="primary" />成交记录（{{ tradeTotal }}）
        </v-card-title>
        <v-data-table-server v-model:page="tradePage" v-model:items-per-page="tradePageSize"
          :headers="tradeHeaders" :items="trades" :items-length="tradeTotal" item-value="trade_id"
          @update:options="loadTradesPage" density="compact">
          <template #item.timeText="{ item }">
            {{ fmtDate(item.date) }} {{ String(item.time).padStart(6, '0').slice(0, 2) }}:{{ String(item.time).padStart(6, '0').slice(2, 4) }}
          </template>
          <template #item.action="{ item }">
            <v-chip size="small" :color="item.action === 'BUY' ? 'red' : 'green'">
              {{ item.action === 'BUY' ? '买入' : '卖出' }}
            </v-chip>
          </template>
          <template #item.amount="{ item }">
            <span>{{ fmtWan(item.amount) }}</span>
          </template>
          <template #item.fee="{ item }">
            <span>{{ fmtWan(item.fee) }}</span>
          </template>
          <template #item.profit="{ item }">
            <span :class="item.profit > 0 ? 'text-success' : item.profit < 0 ? 'text-error' : ''">
              {{ fmtWan(item.profit) }}
            </span>
          </template>
          <template #item.remark="{ item }">
            <span class="text-caption">{{ item.remark || '-' }}</span>
          </template>
        </v-data-table-server>
      </v-card>

      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-routes" class="mr-2" color="primary" />回测期间交易事件回放（{{ eventTraceTotal }}）
        </v-card-title>
        <v-data-table-server v-model:page="eventTracePage" v-model:items-per-page="eventTracePageSize"
          :headers="eventTraceHeaders" :items="eventTraces" :items-length="eventTraceTotal" item-value="event_id"
          @update:options="loadEventTracePage" density="compact">
          <template #item.action="{ item }">
            <v-chip size="x-small" :color="item.action === 'BUY' ? 'red' : 'green'">{{ item.action === 'BUY' ? '买入' : '卖出' }}</v-chip>
          </template>
          <template #item.trigger_date="{ item }">
            {{ fmtDateTimeLocal(item.trigger_date, item.trigger_time) }}
          </template>
          <template #item.order_date="{ item }">
            {{ fmtDateTimeLocal(item.order_date, item.order_time) }}
          </template>
          <template #item.exec_status="{ item }">
            <v-chip size="x-small" :color="execStatusColor(item.exec_status)">{{ execStatusName(item.exec_status) }}</v-chip>
          </template>
          <template #item.exec_date="{ item }">
            {{ item.exec_status === 'FILLED' ? fmtDateTimeLocal(item.exec_date, item.exec_time) : '-' }}
          </template>
          <template #item.latency="{ item }">
            {{ item.exec_status === 'FILLED' ? item.latency_bars + ' bar / ' + item.latency_sec + 's' : '-' }}
          </template>
          <template #item.alive="{ item }">
            {{ item.exec_status !== 'PENDING' ? item.alive_sec + 's' : '-' }}
          </template>
          <template #item.reject_reason="{ item }">
            <span class="text-error text-caption">{{ item.reject_reason || '-' }}</span>
          </template>
          <template #item.trade_id="{ item }">
            <span>{{ item.trade_id ? '#' + item.trade_id : '-' }}</span>
          </template>
        </v-data-table-server>
      </v-card>

      <!-- 回测结果指标：按语义分组（收益 / 风险 / 交易 / 资金） -->
      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-chart-donut" class="mr-2" color="primary" />回测结果评估指标
        </v-card-title>
        <v-card-text>
          <template v-for="g in metricGroups" :key="g.title">
            <div class="text-subtitle-2 font-weight-bold mb-1 mt-2 d-flex align-center">
              <v-icon :icon="g.icon" size="small" class="mr-1" :color="g.color" />{{ g.title }}
            </div>
            <v-row>
              <v-col v-for="m in g.items" :key="g.title + m.label" cols="6" sm="4" md="3" lg="2">
                <v-card variant="tonal" :color="m.color">
                  <v-card-text class="pa-2 text-center">
                    <div class="text-caption text-truncate" :title="m.label">{{ m.label }}</div>
                    <div class="text-h6 font-weight-bold text-truncate" :title="m.value">{{ m.value }}</div>
                  </v-card-text>
                </v-card>
              </v-col>
            </v-row>
          </template>
        </v-card-text>
      </v-card>

      <!-- 回测复盘分析：信号归因 + 链路追踪统计 -->
      <v-card class="mb-3">
        <v-card-title class="text-subtitle-1">
          <v-icon icon="mdi-file-chart-check-outline" class="mr-2" color="primary" />回测复盘分析
        </v-card-title>
        <v-card-text>
          <!-- 信号归因 -->
          <div v-if="report.trade_signal_detail && Object.keys(report.trade_signal_detail).length" class="mb-2">
            <div class="text-subtitle-2 text-medium-emphasis mb-1">信号归因（各信号触发的实际成交笔数）：</div>
            <v-chip v-for="(cnt, sig) in report.trade_signal_detail" :key="sig" class="mr-2 mb-1" label variant="tonal">
              {{ signalExplain(sig) }}：{{ cnt }} 笔
            </v-chip>
          </div>

          <!-- 链路追踪统计（事件触发 → 委托 → 成交/拒绝） -->
          <template v-if="report.event_stats">
            <div class="text-subtitle-2 text-medium-emphasis mb-1 mt-2">链路追踪统计（事件触发 → 委托 → 成交/拒绝）：</div>
            <v-row>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="blue"><v-card-text class="pa-2 text-center"><div class="text-caption">事件触发总数</div><div class="text-h6">{{ report.event_stats.trigger_count }}</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="green"><v-card-text class="pa-2 text-center"><div class="text-caption">成交事件</div><div class="text-h6">{{ report.event_stats.filled_count }}</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="red"><v-card-text class="pa-2 text-center"><div class="text-caption">拒绝事件</div><div class="text-h6">{{ report.event_stats.rejected_count }}</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="orange"><v-card-text class="pa-2 text-center"><div class="text-caption">过期事件</div><div class="text-h6">{{ report.event_stats.expired_count }}</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="teal"><v-card-text class="pa-2 text-center"><div class="text-caption">平均委托耗时</div><div class="text-h6">{{ fmtNum(report.event_stats.avg_latency_bars, 2) }} bar</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="teal"><v-card-text class="pa-2 text-center"><div class="text-caption">平均耗时（秒）</div><div class="text-h6">{{ fmtNum(report.event_stats.avg_latency_sec, 2) }} s</div></v-card-text></v-card></v-col>
            </v-row>

            <!-- 未成交原因分布：可读化（原因 + 计数 + 说明/建议） -->
            <v-row v-if="report.event_stats.reject_reasons && Object.keys(report.event_stats.reject_reasons).length" class="mt-2">
              <v-col cols="12">
                <div class="text-subtitle-2 text-medium-emphasis mb-1">未成交原因分布（含解决建议）：</div>
                <v-table density="compact">
                  <thead>
                    <tr><th>未成交原因</th><th class="text-right" style="width: 110px">次数</th><th>说明 / 解决建议</th></tr>
                  </thead>
                  <tbody>
                    <tr v-for="[reason, cnt] in sortedRejectReasons" :key="reason">
                      <td class="text-error text-body-2">{{ reason }}</td>
                      <td class="text-right text-body-2 font-weight-bold">{{ cnt }}</td>
                      <td class="text-caption text-medium-emphasis">{{ rejectReasonHint(reason) || '-' }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </v-col>
            </v-row>
          </template>

          <!-- 风控复盘分析 -->
          <template v-if="report.event_stats">
            <div class="text-subtitle-2 text-medium-emphasis mb-1 mt-3">风控复盘分析（止损/止盈等风控条件的触发与执行情况）：</div>
            <v-row>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="red"><v-card-text class="pa-2 text-center"><div class="text-caption">止损触发</div><div class="text-h6">{{ riskReview.stopLoss }} 次</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="teal"><v-card-text class="pa-2 text-center"><div class="text-caption">止盈触发</div><div class="text-h6">{{ riskReview.takeProfit }} 次</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="blue"><v-card-text class="pa-2 text-center"><div class="text-caption">买入信号触发</div><div class="text-h6">{{ riskReview.buySignal }} 次</div></v-card-text></v-card></v-col>
              <v-col cols="6" sm="3" lg="2"><v-card variant="tonal" color="blue"><v-card-text class="pa-2 text-center"><div class="text-caption">卖出信号触发</div><div class="text-h6">{{ riskReview.sellSignal }} 次</div></v-card-text></v-card></v-col>
              <v-col v-if="riskReview.other" cols="6" sm="3" lg="2"><v-card variant="tonal" color="grey"><v-card-text class="pa-2 text-center"><div class="text-caption">其他触发</div><div class="text-h6">{{ riskReview.other }} 次</div></v-card-text></v-card></v-col>
            </v-row>
            <div class="text-caption text-medium-emphasis mt-1">
              本轮回测事件共触发 {{ riskReview.total }} 次：止损 {{ riskReview.stopLoss }} 次、止盈 {{ riskReview.takeProfit }} 次、买入信号 {{ riskReview.buySignal }} 次、卖出信号 {{ riskReview.sellSignal }} 次。
              <template v-if="riskReview.stopLoss > 0">止损单在价格触及持仓成本下方止损位时按止损价离场，本轮回测共承担 {{ riskReview.stopLoss }} 次止损对应的价格回落。</template>
              <template v-else>本轮回测未触发止损。</template>
            </div>
          </template>
        </v-card-text>
      </v-card>

      <!-- 报告生成时间 + 导出按钮（报告最底部，居中展示） -->
      <div class="text-center text-caption text-medium-emphasis mb-1">
        报告生成时间：{{ (report.generated_at ?? '').replace('T', ' ').slice(0, 19) }}
      </div>
      <div class="d-flex justify-center ga-2 mb-4">
        <v-btn color="primary" variant="tonal" prepend-icon="mdi-export-variant" @click="exportJSON">导出 JSON</v-btn>
        <v-btn color="teal" variant="tonal" prepend-icon="mdi-file-delimited" @click="exportCSV">导出 CSV</v-btn>
      </div>
    </template>

    <!-- 图表全屏放大 Dialog -->
    <v-dialog v-model="fullscreenDialog" fullscreen transition="fade-transition">
      <v-card>
        <v-toolbar color="primary" density="comfortable">
          <v-toolbar-title>图表放大：{{ chartTitles[fullscreenKey] ?? fullscreenKey }}</v-toolbar-title>
          <v-spacer />
          <v-btn icon="mdi-close" @click="closeFullscreen" title="关闭" />
        </v-toolbar>
        <v-card-text class="pa-2">
          <div ref="fullscreenChartEl" style="height: calc(100vh - 72px); width: 100%" />
        </v-card-text>
      </v-card>
    </v-dialog>
  </v-container>
</template>

<style scoped>
/* 回测背景信息卡片：label 固定等宽，保证各行的第二列（描述/值）统一左对齐起点 */
.kv-label {
  width: 128px;
  flex-shrink: 0;
  padding-right: 8px;
}

/* 明细表格：强制 table-layout fixed，使列按比例撑满整表宽度，
   导航折叠后容器变宽时不再右侧留白（无宽度列的备注列吸收剩余空间） */
:deep(.v-data-table .v-table__wrapper > table) {
  width: 100%;
  table-layout: fixed;
}

/* 明细表格分页器（每页条数 / 页码）整体居中展示 */
:deep(.v-data-table .v-data-table-footer) {
  justify-content: center;
}
</style>
