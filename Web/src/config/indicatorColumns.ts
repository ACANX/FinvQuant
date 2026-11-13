// 证券指标表（cdeu.finv_quote_secu_indicator，132 列）的**前端静态列配置**（需求 11251）。
//
// 分工（改列时只动这一个文件）：
//   - 后端 `GET /API/V1/Meta/Finv/Quote/Secu/Indicator/Columns` 给出「有哪些列」+ 中文名 + 类型；
//   - 本文件给出「列属于哪个分组、用什么格式化口径、预设列方案」，不改后端。
// 表里新增了本文件没登记的列：自动落到「其它」分组、按后端 dataType 走默认格式化，页面不会崩。

/** 后端 `/Columns` 返回的列元数据 */
export interface IndicatorColumn {
  /** 列名（snake_case，与响应行里的键一致） */
  name: string
  /** 列中文名（取自表列注释） */
  title: string
  /** 简化类型：numeric / integer / text / timestamp / boolean / other */
  dataType: string
  /** 是否允许作为排序字段 */
  sortable: boolean
}

/** 单元格渲染口径 */
export type IndicatorCellKind =
  | 'text' // 原样文本
  | 'int' // 整数
  | 'num' // 普通数值（按 digits 位小数、去尾零）
  | 'pct' // 百分点数值 → 加 %
  | 'amount' // 大额数值 → 万 / 亿 / 万亿
  | 'epoch' // Unix 毫秒 → 本地时间
  | 'time' // timestamptz 文本（PG ::text）→ 规整为本地时间
  | 'json' // 长 JSON → ⓘ 浮层
  | 'url' // 长 URL → ⓘ 浮层
  | 'dict' // 字典值 → 中文
  | 'flag' // 0/1 标记 → 否 / 是

/** 列分组（顺序 = 列选择器里的分组顺序） */
export type IndicatorGroupKey =
  | 'identity'
  | 'price'
  | 'volume'
  | 'capital'
  | 'orderbook'
  | 'valuation'
  | 'dividend'
  | 'range'
  | 'preopen'
  | 'fund'
  | 'future'
  | 'index'
  | 'meta'
  | 'other'

/** 字典口径（表格列只显示中文，见 DictColumnSpec） */
const DICT_DIRECT: Record<string, string> = {
  '+': '涨',
  '-': '跌',
  '0': '平',
  up: '涨',
  down: '跌',
  flat: '平',
}

/** 品种类型：与「待采证券录入」页同口径（需求 11151） */
const DICT_INSTRUMENT: Record<string, string> = {
  '3': '股票',
  '4': 'ETF',
  '6': '指数',
  '7': '板块',
  '9': '期货',
  '11': '加密货币',
  '12': '股票',
}

/** 单列的静态配置 */
export interface IndicatorColumnSpec {
  /** 列名（与接口返回的行键一致） */
  name: string
  /** 渲染口径 */
  kind: IndicatorCellKind
  /** 小数位（kind = num / pct 时生效） */
  digits?: number
  /** 字典表（kind = dict 时必填） */
  dict?: Record<string, string>
  /** 表头悬浮说明（口径补充，可选） */
  hint?: string
}

interface IndicatorGroupSpec {
  key: IndicatorGroupKey
  title: string
  columns: IndicatorColumnSpec[]
}

/** 列分组与列配置（132 列，与建表列序一致） */
const GROUPS: IndicatorGroupSpec[] = [
  {
    key: 'identity',
    title: '身份与精度',
    columns: [
      { name: 'usc', kind: 'text', hint: '证券统一编码；上游为空时用 ftmm_symbol 占位' },
      { name: 'ftmm_symbol', kind: 'text', hint: '富途符号 {市场}.{代码}，如 US.AAPL' },
      { name: 'name', kind: 'text' },
      { name: 'stock_id', kind: 'int' },
      { name: 'lot_size', kind: 'int' },
      { name: 'spread_code', kind: 'text' },
      { name: 'ex_type', kind: 'int' },
      { name: 'price_accuracy', kind: 'int' },
      { name: 'volume_precision', kind: 'int' },
      { name: 'order_volume_precision', kind: 'int' },
    ],
  },
  {
    key: 'price',
    title: '行情价格',
    columns: [
      { name: 'price', kind: 'num', digits: 4 },
      { name: 'change_price', kind: 'num', digits: 4 },
      { name: 'change_ratio', kind: 'pct', digits: 2 },
      { name: 'price_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'price_open', kind: 'num', digits: 4 },
      { name: 'price_highest', kind: 'num', digits: 4 },
      { name: 'price_lowest', kind: 'num', digits: 4 },
      { name: 'price_last_close', kind: 'num', digits: 4 },
      { name: 'price_nominal', kind: 'num', digits: 4 },
      { name: 'hight_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'open_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'low_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'price_average', kind: 'num', digits: 4 },
      { name: 'change_speed_price', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'volume',
    title: '成交与活跃度',
    columns: [
      { name: 'volume', kind: 'amount' },
      { name: 'turnover', kind: 'amount' },
      { name: 'ratio_volume', kind: 'num', digits: 3 },
      { name: 'ratio_turnover', kind: 'pct', digits: 2 },
      { name: 'amplitude_price', kind: 'pct', digits: 2 },
      { name: 'ratio_bid_ask', kind: 'pct', digits: 2 },
      { name: 'kcb_static_volume', kind: 'amount' },
      { name: 'kcb_static_turnover', kind: 'amount' },
    ],
  },
  {
    key: 'capital',
    title: '股本与市值',
    columns: [
      { name: 'total_shares', kind: 'amount' },
      { name: 'total_market_cap', kind: 'amount' },
      { name: 'outstanding_shares', kind: 'amount' },
      { name: 'outstanding_market_cap', kind: 'amount' },
    ],
  },
  {
    key: 'orderbook',
    title: '盘口与涨跌停',
    columns: [
      { name: 'price_bid', kind: 'num', digits: 4 },
      { name: 'price_ask', kind: 'num', digits: 4 },
      { name: 'volume_bid', kind: 'amount' },
      { name: 'volume_ask', kind: 'amount' },
      { name: 'volume_sold', kind: 'amount', hint: '内盘' },
      { name: 'volume_bought', kind: 'amount', hint: '外盘' },
      { name: 'price_limit_up', kind: 'num', digits: 4 },
      { name: 'price_limit_down', kind: 'num', digits: 4 },
      { name: 'difference', kind: 'num', digits: 6, hint: '上游口径待确认' },
      { name: 'bid_price_lowest', kind: 'num', digits: 4, hint: '上游口径待确认' },
      { name: 'ask_price_highest', kind: 'num', digits: 4, hint: '上游口径待确认' },
    ],
  },
  {
    key: 'valuation',
    title: '估值',
    columns: [
      { name: 'pe_ttm', kind: 'num', digits: 4 },
      { name: 'pe_lyr', kind: 'num', digits: 4 },
      { name: 'pb_ratio', kind: 'num', digits: 4 },
      { name: 'eps_ttm', kind: 'num', digits: 4 },
      { name: 'eps_lyr', kind: 'num', digits: 4 },
      { name: 'ps_ttm', kind: 'num', digits: 4 },
      { name: 'ps_lyr', kind: 'num', digits: 4 },
    ],
  },
  {
    key: 'dividend',
    title: '股息',
    columns: [
      { name: 'dividend', kind: 'num', digits: 6 },
      { name: 'dividend_ratio', kind: 'pct', digits: 2 },
      { name: 'dividend_lfy', kind: 'num', digits: 6 },
      { name: 'dividend_lfy_ratio', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'range',
    title: '区间与极值',
    columns: [
      { name: 'price_highest_week52', kind: 'num', digits: 4 },
      { name: 'price_lowest_week52', kind: 'num', digits: 4 },
      { name: 'price_highest_history', kind: 'num', digits: 4 },
      { name: 'price_lowest_history', kind: 'num', digits: 4 },
      { name: 'change_d005', kind: 'pct', digits: 2, hint: '5 日涨跌幅' },
      { name: 'change_d010', kind: 'pct', digits: 2, hint: '10 日涨跌幅' },
      { name: 'change_d020', kind: 'pct', digits: 2, hint: '20 日涨跌幅' },
      { name: 'change_d060', kind: 'pct', digits: 2, hint: '60 日涨跌幅' },
      { name: 'change_d120', kind: 'pct', digits: 2, hint: '120 日涨跌幅' },
      { name: 'change_d250', kind: 'pct', digits: 2, hint: '250 日涨跌幅' },
      { name: 'change_year', kind: 'pct', digits: 2, hint: '年初至今涨跌幅' },
    ],
  },
  {
    key: 'preopen',
    title: '盘前',
    columns: [
      { name: 'before_open_status', kind: 'int' },
      { name: 'before_open_exchange_time', kind: 'epoch' },
      { name: 'before_open_price', kind: 'num', digits: 4 },
      { name: 'before_open_change', kind: 'num', digits: 4 },
      { name: 'before_open_change_ratio', kind: 'pct', digits: 2 },
      { name: 'before_open_price_direct', kind: 'dict', dict: DICT_DIRECT, hint: '库里是数值列，按方向字典翻译' },
    ],
  },
  {
    key: 'fund',
    title: '基金（ETF / 信托）',
    columns: [
      { name: 'trust_dividend_yield', kind: 'pct', digits: 2 },
      { name: 'trust_aum', kind: 'amount' },
      { name: 'trust_outstanding_units', kind: 'amount' },
      { name: 'trust_net_asset_value', kind: 'num', digits: 4 },
      { name: 'trust_premium', kind: 'pct', digits: 2 },
      { name: 'trust_asset_class', kind: 'int' },
      { name: 'trust_asset_class_sc', kind: 'text' },
      { name: 'trust_asset_class_tc', kind: 'text' },
      { name: 'trust_asset_class_en', kind: 'text' },
    ],
  },
  {
    key: 'future',
    title: '期货与标的',
    columns: [
      { name: 'future_last_settlement_price', kind: 'num', digits: 4 },
      { name: 'future_hold_vol', kind: 'amount' },
      { name: 'future_diff_hold_vol', kind: 'amount' },
      { name: 'future_main_flag', kind: 'flag' },
      { name: 'future_distance_due_date', kind: 'int' },
      { name: 'future_price_limit_up', kind: 'num', digits: 4 },
      { name: 'future_price_limit_down', kind: 'num', digits: 4 },
      { name: 'underlying_stock_id', kind: 'text' },
      { name: 'underlying_stock_code', kind: 'text' },
      { name: 'underlying_name', kind: 'text' },
      { name: 'underlying_market_label', kind: 'text' },
      { name: 'underlying_instrument_type', kind: 'dict', dict: DICT_INSTRUMENT },
      { name: 'underlying_price', kind: 'num', digits: 4 },
      { name: 'underlying_change', kind: 'num', digits: 4 },
      { name: 'underlying_change_ratio', kind: 'pct', digits: 2 },
      { name: 'underlying_price_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'underlying_price_nominal', kind: 'num', digits: 4 },
      { name: 'pod_text', kind: 'text' },
      { name: 'pod_value', kind: 'num', digits: 6 },
      { name: 'pod_direct', kind: 'dict', dict: DICT_DIRECT },
      { name: 'hp_multiplier', kind: 'int' },
      { name: 'last_trade_date', kind: 'text' },
    ],
  },
  {
    key: 'index',
    title: '指数与 24h 统计',
    columns: [
      { name: 'index_count_all', kind: 'int' },
      { name: 'index_count_raise', kind: 'int' },
      { name: 'index_count_fall', kind: 'int' },
      { name: 'index_count_equal', kind: 'int' },
      { name: 'stat_h24_price_highest', kind: 'num', digits: 4 },
      { name: 'stat_h24_price_lowest', kind: 'num', digits: 4 },
      { name: 'stat_h24_volume', kind: 'amount' },
      { name: 'stat_h24_turnover', kind: 'amount' },
      { name: 'stat_h24_price_change', kind: 'num', digits: 4 },
      { name: 'stat_h24_ratio_price_change', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'meta',
    title: '时间与来源',
    columns: [
      { name: 'ts', kind: 'epoch', hint: '行情时间戳（上游 epoch 毫秒）' },
      { name: 'market_status', kind: 'int' },
      { name: 'market_status_text', kind: 'text' },
      { name: 'delay_time', kind: 'int' },
      { name: 'delay_seconds', kind: 'int' },
      { name: 'ts_exchange_dt', kind: 'epoch' },
      { name: 'ts_server_send_to_client', kind: 'epoch' },
      { name: 'ts_server_recv_from_exchange', kind: 'epoch' },
      { name: 'timezone', kind: 'text' },
      { name: 'secu_url', kind: 'url' },
      { name: 'source_channel', kind: 'text' },
      { name: 'dt_collect', kind: 'time' },
      { name: 'dt_create', kind: 'time' },
      { name: 'dt_update', kind: 'time', hint: '由触发器在同步更新时刷新' },
      { name: 'count_update', kind: 'int' },
      { name: 'json_unmapped', kind: 'json', hint: '上游未映射字段的原始 JSON' },
    ],
  },
]

/** 分组（含「其它」兜底） */
export const INDICATOR_GROUPS: { key: IndicatorGroupKey; title: string }[] = [
  ...GROUPS.map((group) => ({ key: group.key, title: group.title })),
  { key: 'other', title: '其它（未登记）' },
]

/** 列名 → 静态配置（扁平索引） */
const COLUMN_INDEX = new Map<string, IndicatorColumnSpec & { group: IndicatorGroupKey }>()
for (const group of GROUPS) {
  for (const column of group.columns) {
    COLUMN_INDEX.set(column.name, { ...column, group: group.key })
  }
}

/** 列名 → 分组内的全部列名（按配置顺序） */
export const GROUP_COLUMNS: Record<IndicatorGroupKey, string[]> = (() => {
  const result = {} as Record<IndicatorGroupKey, string[]>
  for (const group of INDICATOR_GROUPS) result[group.key] = []
  for (const group of GROUPS) result[group.key] = group.columns.map((column) => column.name)
  return result
})()

/** 行身份列：任何列方案都必须包含（后端也会强制补齐） */
export const INDICATOR_KEY_COLUMNS = ['usc', 'ftmm_symbol']

/** 列方案（预设） */
export interface IndicatorPreset {
  key: string
  title: string
  /** 列名数组（按此顺序渲染） */
  columns: string[]
}

function groupColumns(...keys: IndicatorGroupKey[]): string[] {
  return keys.flatMap((key) => GROUP_COLUMNS[key] ?? [])
}

export const INDICATOR_PRESETS: IndicatorPreset[] = [
  {
    key: 'overview',
    title: '概览',
    columns: [
      ...groupColumns('identity'),
      'price',
      'change_price',
      'change_ratio',
      'price_direct',
      'volume',
      'turnover',
      'total_market_cap',
      'pe_ttm',
      'pb_ratio',
      'market_status_text',
      'ts',
      'dt_update',
    ],
  },
  { key: 'quote', title: '行情', columns: groupColumns('identity', 'price', 'volume', 'capital', 'orderbook') },
  { key: 'valuation', title: '估值与股息', columns: groupColumns('identity', 'valuation', 'dividend', 'capital', 'range') },
  { key: 'fund', title: '基金', columns: groupColumns('identity', 'price', 'valuation', 'fund') },
  { key: 'future', title: '期货与标的', columns: groupColumns('identity', 'price', 'future') },
  { key: 'meta', title: '时间与来源', columns: groupColumns('identity', 'meta') },
  { key: 'all', title: '全部', columns: groupColumns(...INDICATOR_GROUPS.map((group) => group.key)) },
]

/** 默认列方案 key */
export const DEFAULT_PRESET_KEY = 'overview'

/** 取列静态配置；未登记的列返回 undefined（调用方按后端 dataType 兜底） */
export function indicatorColumnSpec(name: string): (IndicatorColumnSpec & { group: IndicatorGroupKey }) | undefined {
  return COLUMN_INDEX.get(name)
}

/** 列所属分组（未登记 → other） */
export function indicatorGroupOf(name: string): IndicatorGroupKey {
  return COLUMN_INDEX.get(name)?.group ?? 'other'
}

// ---------- 格式化 ----------

/** 去掉多余尾零：'1.230000' -> '1.23'、'2.000000' -> '2' */
function trimZero(text: string): string {
  return text.includes('.') ? text.replace(/0+$/, '').replace(/\.$/, '') : text
}

/** 普通数值：按 digits 位小数四舍五入后去尾零；空值 → — */
function dec(value: string, digits = 4): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  return trimZero(n.toFixed(digits))
}

/** 大额数值：万 / 亿 / 万亿 折算 */
function compact(value: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  const abs = Math.abs(n)
  if (abs >= 1e12) return `${trimZero((n / 1e12).toFixed(2))} 万亿`
  if (abs >= 1e8) return `${trimZero((n / 1e8).toFixed(2))} 亿`
  if (abs >= 1e4) return `${trimZero((n / 1e4).toFixed(2))} 万`
  return trimZero(n.toFixed(2))
}

/** Unix 毫秒 → 本地 yyyy-MM-dd HH:mm:ss（0 / 非法 → —） */
function epochText(value: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return value
  return dateTimeText(new Date(n))
}

/** timestamptz 文本（PG ::text）→ 本地时间；解析不了就原样显示 */
function timeText(value: string): string {
  if (!value) return '—'
  const date = new Date(value.replace(' ', 'T'))
  if (Number.isNaN(date.getTime())) return value
  return dateTimeText(date)
}

function dateTimeText(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  )
}

/** 按列口径格式化单元格；未登记的列按后端 dataType 兜底 */
export function formatIndicatorCell(column: IndicatorColumn, value: string): string {
  const spec = indicatorColumnSpec(column.name)
  if (!spec) {
    // 未登记：数值列按 4 位小数，时间列按本地时间，其余原样
    if (column.dataType === 'numeric') return dec(value, 4)
    if (column.dataType === 'timestamp') return timeText(value)
    return value || '—'
  }
  switch (spec.kind) {
    case 'int':
      return value === '' ? '—' : dec(value, 0)
    case 'num':
      return dec(value, spec.digits ?? 4)
    case 'pct':
      return value ? `${dec(value, spec.digits ?? 2)}%` : '—'
    case 'amount':
      return compact(value)
    case 'epoch':
      return epochText(value)
    case 'time':
      return timeText(value)
    case 'flag':
      if (value === '') return '—'
      return value === '1' ? '是' : value === '0' ? '否' : value
    case 'dict':
      if (value === '') return '—'
      return spec.dict?.[value] ?? value
    case 'json':
    case 'url':
    case 'text':
    default:
      return value || '—'
  }
}

/** 该列是否走 ⓘ 浮层（长文本：JSON / URL） */
export function isIndicatorLongText(name: string): boolean {
  const kind = COLUMN_INDEX.get(name)?.kind
  return kind === 'json' || kind === 'url'
}

/** 该列是否右对齐（数值 / 时间） */
export function isIndicatorNumericColumn(name: string): boolean {
  const kind = COLUMN_INDEX.get(name)?.kind
  return (
    kind === 'int' ||
    kind === 'num' ||
    kind === 'pct' ||
    kind === 'amount' ||
    kind === 'epoch' ||
    kind === 'time' ||
    kind === 'flag'
  )
}
