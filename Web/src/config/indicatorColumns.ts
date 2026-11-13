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
  /**
   * 列的中文短名（只用于表格表头 / 下拉选项 / 列选择器）。
   *
   * 后端 `/Columns` 的 `title` 取自**表列注释**，很多是整句口径说明（如
   * 「最后更新时间（可空；由触发器 finv_quote_secu_indicator_set_dt_update 在 UPDATE 时刷新）」），
   * 直接铺在 132 列的表头和筛选下拉里没法看——所以展示一律用本字段的短名，
   * 完整口径只在表头悬浮、行详情与说明卡片里出现。
   */
  label: string
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
      { name: 'usc', label: '证券编码', kind: 'text', hint: '证券统一编码；上游为空时用 ftmm_symbol 占位' },
      { name: 'ftmm_symbol', label: '富途符号', kind: 'text', hint: '富途符号 {市场}.{代码}，如 US.AAPL' },
      { name: 'name', label: '名称', kind: 'text' },
      { name: 'stock_id', label: '证券ID', kind: 'int' },
      { name: 'lot_size', label: '每手股数', kind: 'int' },
      { name: 'spread_code', label: '点差档位', kind: 'text' },
      { name: 'ex_type', label: '交易所类型', kind: 'int' },
      { name: 'price_accuracy', label: '价格精度', kind: 'int' },
      { name: 'volume_precision', label: '成交量精度', kind: 'int' },
      { name: 'order_volume_precision', label: '委托量精度', kind: 'int' },
    ],
  },
  {
    key: 'price',
    title: '行情价格',
    columns: [
      { name: 'price', label: '最新价', kind: 'num', digits: 4 },
      { name: 'change_price', label: '涨跌额', kind: 'num', digits: 4 },
      { name: 'change_ratio', label: '涨跌幅', kind: 'pct', digits: 2 },
      { name: 'price_direct', label: '涨跌方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'price_open', label: '今开', kind: 'num', digits: 4 },
      { name: 'price_highest', label: '今日最高', kind: 'num', digits: 4 },
      { name: 'price_lowest', label: '今日最低', kind: 'num', digits: 4 },
      { name: 'price_last_close', label: '昨收', kind: 'num', digits: 4 },
      { name: 'price_nominal', label: '名义价', kind: 'num', digits: 4 },
      { name: 'hight_direct', label: '最高方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'open_direct', label: '开盘方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'low_direct', label: '最低方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'price_average', label: '均价', kind: 'num', digits: 4 },
      { name: 'change_speed_price', label: '涨速', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'volume',
    title: '成交与活跃度',
    columns: [
      { name: 'volume', label: '成交量', kind: 'amount' },
      { name: 'turnover', label: '成交额', kind: 'amount' },
      { name: 'ratio_volume', label: '量比', kind: 'num', digits: 3 },
      { name: 'ratio_turnover', label: '换手率', kind: 'pct', digits: 2 },
      { name: 'amplitude_price', label: '振幅', kind: 'pct', digits: 2 },
      { name: 'ratio_bid_ask', label: '委比', kind: 'pct', digits: 2 },
      { name: 'kcb_static_volume', label: '静态成交量', kind: 'amount' },
      { name: 'kcb_static_turnover', label: '静态成交额', kind: 'amount' },
    ],
  },
  {
    key: 'capital',
    title: '股本与市值',
    columns: [
      { name: 'total_shares', label: '总股本', kind: 'amount' },
      { name: 'total_market_cap', label: '总市值', kind: 'amount' },
      { name: 'outstanding_shares', label: '流通股本', kind: 'amount' },
      { name: 'outstanding_market_cap', label: '流通市值', kind: 'amount' },
    ],
  },
  {
    key: 'orderbook',
    title: '盘口与涨跌停',
    columns: [
      { name: 'price_bid', label: '买一价', kind: 'num', digits: 4 },
      { name: 'price_ask', label: '卖一价', kind: 'num', digits: 4 },
      { name: 'volume_bid', label: '买一量', kind: 'amount' },
      { name: 'volume_ask', label: '卖一量', kind: 'amount' },
      { name: 'volume_sold', label: '内盘', kind: 'amount', hint: '内盘' },
      { name: 'volume_bought', label: '外盘', kind: 'amount', hint: '外盘' },
      { name: 'price_limit_up', label: '涨停价', kind: 'num', digits: 4 },
      { name: 'price_limit_down', label: '跌停价', kind: 'num', digits: 4 },
      { name: 'difference', label: '量纲基数', kind: 'num', digits: 6, hint: '上游口径待确认' },
      { name: 'bid_price_lowest', label: '可成交下界', kind: 'num', digits: 4, hint: '上游口径待确认' },
      { name: 'ask_price_highest', label: '可成交上界', kind: 'num', digits: 4, hint: '上游口径待确认' },
    ],
  },
  {
    key: 'valuation',
    title: '估值',
    columns: [
      { name: 'pe_ttm', label: '市盈率TTM', kind: 'num', digits: 4 },
      { name: 'pe_lyr', label: '市盈率LYR', kind: 'num', digits: 4 },
      { name: 'pb_ratio', label: '市净率', kind: 'num', digits: 4 },
      { name: 'eps_ttm', label: '每股收益TTM', kind: 'num', digits: 4 },
      { name: 'eps_lyr', label: '每股收益LYR', kind: 'num', digits: 4 },
      { name: 'ps_ttm', label: '市销率TTM', kind: 'num', digits: 4 },
      { name: 'ps_lyr', label: '市销率LYR', kind: 'num', digits: 4 },
    ],
  },
  {
    key: 'dividend',
    title: '股息',
    columns: [
      { name: 'dividend', label: '每股股息', kind: 'num', digits: 6 },
      { name: 'dividend_ratio', label: '股息率', kind: 'pct', digits: 2 },
      { name: 'dividend_lfy', label: '上年股息', kind: 'num', digits: 6 },
      { name: 'dividend_lfy_ratio', label: '上年股息率', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'range',
    title: '区间与极值',
    columns: [
      { name: 'price_highest_week52', label: '52周最高', kind: 'num', digits: 4 },
      { name: 'price_lowest_week52', label: '52周最低', kind: 'num', digits: 4 },
      { name: 'price_highest_history', label: '历史最高', kind: 'num', digits: 4 },
      { name: 'price_lowest_history', label: '历史最低', kind: 'num', digits: 4 },
      { name: 'change_d005', label: '5日涨跌幅', kind: 'pct', digits: 2, hint: '5 日涨跌幅' },
      { name: 'change_d010', label: '10日涨跌幅', kind: 'pct', digits: 2, hint: '10 日涨跌幅' },
      { name: 'change_d020', label: '20日涨跌幅', kind: 'pct', digits: 2, hint: '20 日涨跌幅' },
      { name: 'change_d060', label: '60日涨跌幅', kind: 'pct', digits: 2, hint: '60 日涨跌幅' },
      { name: 'change_d120', label: '120日涨跌幅', kind: 'pct', digits: 2, hint: '120 日涨跌幅' },
      { name: 'change_d250', label: '250日涨跌幅', kind: 'pct', digits: 2, hint: '250 日涨跌幅' },
      { name: 'change_year', label: '年初至今涨跌幅', kind: 'pct', digits: 2, hint: '年初至今涨跌幅' },
    ],
  },
  {
    key: 'preopen',
    title: '盘前',
    columns: [
      { name: 'before_open_status', label: '盘前状态', kind: 'int' },
      { name: 'before_open_exchange_time', label: '盘前时间', kind: 'epoch' },
      { name: 'before_open_price', label: '盘前价', kind: 'num', digits: 4 },
      { name: 'before_open_change', label: '盘前涨跌额', kind: 'num', digits: 4 },
      { name: 'before_open_change_ratio', label: '盘前涨跌幅', kind: 'pct', digits: 2 },
      { name: 'before_open_price_direct', label: '盘前方向', kind: 'dict', dict: DICT_DIRECT, hint: '库里是数值列，按方向字典翻译' },
    ],
  },
  {
    key: 'fund',
    title: '基金（ETF / 信托）',
    columns: [
      { name: 'trust_dividend_yield', label: '基金股息率', kind: 'pct', digits: 2 },
      { name: 'trust_aum', label: '基金规模', kind: 'amount' },
      { name: 'trust_outstanding_units', label: '基金份额', kind: 'amount' },
      { name: 'trust_net_asset_value', label: '基金净值', kind: 'num', digits: 4 },
      { name: 'trust_premium', label: '溢价率', kind: 'pct', digits: 2 },
      { name: 'trust_asset_class', label: '资产类别', kind: 'int' },
      { name: 'trust_asset_class_sc', label: '资产类别简', kind: 'text' },
      { name: 'trust_asset_class_tc', label: '资产类别繁', kind: 'text' },
      { name: 'trust_asset_class_en', label: '资产类别英', kind: 'text' },
    ],
  },
  {
    key: 'future',
    title: '期货与标的',
    columns: [
      { name: 'future_last_settlement_price', label: '上次结算价', kind: 'num', digits: 4 },
      { name: 'future_hold_vol', label: '持仓量', kind: 'amount' },
      { name: 'future_diff_hold_vol', label: '持仓变化', kind: 'amount' },
      { name: 'future_main_flag', label: '主力合约', kind: 'flag' },
      { name: 'future_distance_due_date', label: '距到期天数', kind: 'int' },
      { name: 'future_price_limit_up', label: '期货涨停价', kind: 'num', digits: 4 },
      { name: 'future_price_limit_down', label: '期货跌停价', kind: 'num', digits: 4 },
      { name: 'underlying_stock_id', label: '标的ID', kind: 'text' },
      { name: 'underlying_stock_code', label: '标的代码', kind: 'text' },
      { name: 'underlying_name', label: '标的名称', kind: 'text' },
      { name: 'underlying_market_label', label: '标的市场', kind: 'text' },
      { name: 'underlying_instrument_type', label: '标的品种', kind: 'dict', dict: DICT_INSTRUMENT },
      { name: 'underlying_price', label: '标的现价', kind: 'num', digits: 4 },
      { name: 'underlying_change', label: '标的涨跌额', kind: 'num', digits: 4 },
      { name: 'underlying_change_ratio', label: '标的涨跌幅', kind: 'pct', digits: 2 },
      { name: 'underlying_price_direct', label: '标的涨跌方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'underlying_price_nominal', label: '标的名义价', kind: 'num', digits: 4 },
      { name: 'pod_text', label: '升贴水文本', kind: 'text' },
      { name: 'pod_value', label: '升贴水', kind: 'num', digits: 6 },
      { name: 'pod_direct', label: '升贴水方向', kind: 'dict', dict: DICT_DIRECT },
      { name: 'hp_multiplier', label: '合约乘数', kind: 'int' },
      { name: 'last_trade_date', label: '最后交易日', kind: 'text' },
    ],
  },
  {
    key: 'index',
    title: '指数与 24h 统计',
    columns: [
      { name: 'index_count_all', label: '成分股数', kind: 'int' },
      { name: 'index_count_raise', label: '上涨家数', kind: 'int' },
      { name: 'index_count_fall', label: '下跌家数', kind: 'int' },
      { name: 'index_count_equal', label: '平盘家数', kind: 'int' },
      { name: 'stat_h24_price_highest', label: '24h最高价', kind: 'num', digits: 4 },
      { name: 'stat_h24_price_lowest', label: '24h最低价', kind: 'num', digits: 4 },
      { name: 'stat_h24_volume', label: '24h成交量', kind: 'amount' },
      { name: 'stat_h24_turnover', label: '24h成交额', kind: 'amount' },
      { name: 'stat_h24_price_change', label: '24h涨跌额', kind: 'num', digits: 4 },
      { name: 'stat_h24_ratio_price_change', label: '24h涨跌幅', kind: 'pct', digits: 2 },
    ],
  },
  {
    key: 'meta',
    title: '时间与来源',
    columns: [
      { name: 'ts', label: '行情时间', kind: 'epoch', hint: '行情时间戳（上游 epoch 毫秒）' },
      { name: 'market_status', label: '交易状态码', kind: 'int' },
      { name: 'market_status_text', label: '交易状态', kind: 'text' },
      { name: 'delay_time', label: '延迟时长', kind: 'int' },
      { name: 'delay_seconds', label: '延迟秒数', kind: 'int' },
      { name: 'ts_exchange_dt', label: '交易所时间', kind: 'epoch' },
      { name: 'ts_server_send_to_client', label: '服务端下发时间', kind: 'epoch' },
      { name: 'ts_server_recv_from_exchange', label: '交易所回包时间', kind: 'epoch' },
      { name: 'timezone', label: '交易所时区', kind: 'text' },
      { name: 'secu_url', label: '来源链接', kind: 'url' },
      { name: 'source_channel', label: '来源通道', kind: 'text' },
      { name: 'dt_collect', label: '采集时间', kind: 'time' },
      { name: 'dt_create', label: '创建时间', kind: 'time' },
      { name: 'dt_update', label: '更新时间', kind: 'time', hint: '由触发器在同步更新时刷新' },
      { name: 'count_update', label: '更新次数', kind: 'int' },
      { name: 'json_unmapped', label: '未映射字段', kind: 'json', hint: '上游未映射字段的原始 JSON' },
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

/**
 * 列的中文短名（表格表头 / 下拉选项 / 列选择器统一用这个）。
 *
 * 未登记的列（后端新加、本文件还没补）回退到后端表列注释：注释往往是整句说明，
 * 这种情况宁可长也不能丢信息，但优先级低于短名。
 */
export function indicatorColumnLabel(column: IndicatorColumn): string {
  return COLUMN_INDEX.get(column.name)?.label ?? column.title ?? column.name
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
