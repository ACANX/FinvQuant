// 采集参数页（futu.finv_futu_quote_collect）的共享配置（需求 11254）。
//
// 视图 `CollectParamView.vue` 与编辑对话框 `CollectParamDialog.vue` 共用这里的：
//   - 行结构 `CollectParamRow`（wire 字段 snake_case，含左关联的证券元数据）；
//   - 各下拉选项（市场 / 品种 / 证券分类 / 采集状态 / 排序字段）；
//   - 编码 → 中文的字典与格式化函数。
// 口径说明（各编码含义）写在页面底部说明卡片里，不上表格与下拉。

/** 采集参数列表行：futu.finv_futu_quote_collect + 左关联的 public.finv_security 等 */
export interface CollectParamRow {
  // 采集参数（futu.finv_futu_quote_collect）
  usc: string
  ftmm_symbol: string
  futu_symbol: string
  market_label: string
  market_type: number
  market_code: number
  instrument: string
  instrument_type: number
  sub_instrument_type: number
  req_section: string
  stock_id: number
  flag_enable: string
  remark: string
  gmt_update: string
  // 证券元数据（public.finv_security + 交易所 / 市场字典；主数据缺失时为空）
  security_name: string
  security_type: string
  security_sub_type: string
  currency_type: string
  exchange_code: number
  exchange_name: string
  security_market_code: number
  security_market_name: string
  security_exists: boolean
}

/** 行情市场标识（market_label） */
export const COLLECT_MARKET_OPTIONS = ['HK', 'SH', 'SZ', 'BJ', 'US', 'FX', 'CC']

/** 富途证券类型标识（instrument） */
export const COLLECT_INSTRUMENT_ITEMS = [
  { title: '股票', value: 'stock' },
  { title: 'ETF', value: 'etfs' },
  { title: '指数', value: 'index' },
  { title: '外汇', value: 'currency' },
  { title: '加密', value: 'crypto' },
  { title: '期货', value: 'futures' },
  { title: '板块', value: 'sectors' },
]

/** 富途证券类型 → 中文 */
const INSTRUMENT_LABELS: Record<string, string> = Object.fromEntries(
  COLLECT_INSTRUMENT_ITEMS.map((item) => [item.value, item.title]),
)

/** 富途品种类型（instrument_type）→ 中文 */
const INSTRUMENT_TYPE_LABELS: Record<string, string> = {
  '3': '股票',
  '4': 'ETF',
  '6': '指数',
  '7': '板块',
  '9': '期货',
  '10': '外汇',
  '11': '加密',
}

/** 富途市场类型（market_type）→ 中文 */
const MARKET_TYPE_LABELS: Record<string, string> = {
  '1': '港股',
  '2': '美股',
  '4': 'A股',
  '11': '外汇',
  '17': '加密',
}

/** 证券分类（public.finv_security.security_type）筛选项 */
export const COLLECT_SECURITY_TYPE_OPTIONS = [
  { title: '全部分类', value: '' },
  { title: '股票', value: 'Stock' },
  { title: 'ETF', value: 'ETF' },
  { title: '指数', value: 'StockIndex' },
  { title: '期货', value: 'Futures' },
]

/** 采集状态（flag_enable）筛选项 */
export const COLLECT_FLAG_OPTIONS = [
  { title: '全部状态', value: '' },
  { title: '启用', value: '1' },
  { title: '禁用', value: '0' },
]

/** 排序字段（与后端白名单同源） */
export const COLLECT_ORDER_OPTIONS = [
  { title: '证券编码', value: 'usc' },
  { title: '富途键', value: 'ftmm_symbol' },
  { title: '富途代码', value: 'futu_symbol' },
  { title: '行情市场', value: 'market_label' },
  { title: '品种', value: 'instrument' },
  { title: '更新时间', value: 'gmt_update' },
]

/** 富途证券类型标识 → 中文（未登记值原样） */
export function collectInstrumentText(value: string): string {
  if (!value) return '—'
  return INSTRUMENT_LABELS[value] ?? value
}

/** 编码 → 中文（0 / 空显示 —，未登记值原样） */
function dictText(value: number, dict: Record<string, string>): string {
  if (!value) return '—'
  return dict[String(value)] ?? String(value)
}

/** 富途品种类型 → 中文 */
export function collectInstrumentTypeText(value: number): string {
  return dictText(value, INSTRUMENT_TYPE_LABELS)
}

/** 富途市场类型 → 中文 */
export function collectMarketTypeText(value: number): string {
  return dictText(value, MARKET_TYPE_LABELS)
}

/** 采集状态 → 中文 */
export function collectFlagText(value: string): string {
  return value === '1' ? '启用' : '禁用'
}
