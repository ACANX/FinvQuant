<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import { showToast } from '../../../../ui/toast'

// finv_security 证券代码行
interface SecurityRow {
  usc: string
  exchange_code: number
  market_code: number
  security_type: string
  security_code: string
  security_name: string
  security_name_cn: string
  security_name_full: string | null
  currency_type: string
  init_date: number
  timezone: string | null
  tz: string | null
  flag_enable: string
  /** 统一证券编号（可空，需求 11327） */
  usn: number | null
  /** 证券分类 / 子类（需求 11327） */
  type_secu: string
  type_secu_sub: string
  /** 备注 / 数据提供方（需求 11328：列配置后可展示） */
  remark: string
  provider: string
  /** 记录创建/更新时间（RFC3339，需求 11328） */
  gmt_create: string
  gmt_update: string
  /** 是否已收藏（quote.finv_quote_secu_fav，需求 11312） */
  is_favorite: boolean
}

// finv_exchange 交易所下拉选项（交易所信息维护字典）
interface ExchangeOption {
  exchange_code: number
  exchange_abbr: string
  exchange_name: string
  exchange_abbr_cn: string
  flag_enable: string
}

// finv_market 市场下拉选项（交易所下设市场字典）
interface MarketOption {
  market_code: number
  market_flag: string
  market_name: string
  flag_enable: string
}

// ---- 列配置（需求 11328）----
// 全部可选列：顺序即表格列顺序；「类型」默认隐藏，其余字段可按需勾选展示。
interface SecurityColumn {
  key: string
  title: string
  width?: number
  align?: 'start' | 'end' | 'center'
  kind?: 'text' | 'time' | 'longtext'
}

const SECURITY_COLUMNS: SecurityColumn[] = [
  { key: 'usn', title: '统一编号', width: 100, align: 'end' },
  { key: 'usc', title: 'usc', width: 110 },
  { key: 'exchange_code', title: '交易所', width: 80 },
  { key: 'market_code', title: '市场', width: 80 },
  { key: 'security_type', title: '类型', width: 100 },
  { key: 'security_code', title: '源代码', width: 110 },
  { key: 'security_name', title: '证券名称' },
  { key: 'security_name_cn', title: '中文名称', width: 140 },
  { key: 'security_name_full', title: '全称', kind: 'longtext' },
  { key: 'type_secu', title: '分类', width: 110 },
  { key: 'type_secu_sub', title: '子类', width: 140 },
  { key: 'currency_type', title: '货币', width: 80 },
  { key: 'init_date', title: '上市日', width: 100 },
  { key: 'timezone', title: '时区', width: 90 },
  { key: 'tz', title: '时区标识', width: 150 },
  { key: 'flag_enable', title: '状态', width: 90 },
  { key: 'provider', title: '提供方', width: 100 },
  { key: 'remark', title: '备注', kind: 'longtext' },
  { key: 'gmt_create', title: '创建时间', width: 160, kind: 'time' },
  { key: 'gmt_update', title: '更新时间', width: 160, kind: 'time' },
]

// 默认可见列：原表格列去掉「类型」（第 5 列，需求 11328），其余需在「选择列」里手动勾选。
const DEFAULT_VISIBLE_KEYS = SECURITY_COLUMNS.map((c) => c.key).filter((k) => k !== 'security_type')
const STORAGE_COLUMNS = 'finv:meta:security:columns'

const visibleKeys = ref<string[]>([...DEFAULT_VISIBLE_KEYS])
const columnDialog = ref(false)
const columnDraft = ref<string[]>([...DEFAULT_VISIBLE_KEYS])

// 列宽（需求 11329）：拖拽表头右边界调整，存 localStorage；
// 未调整的列用 SECURITY_COLUMNS 的默认宽度，单元格 nowrap 保证内容不被压成多行。
const STORAGE_WIDTHS = 'finv:meta:security:widths'
const columnWidths = ref<Record<string, number>>({})

const headers = computed(() =>
  SECURITY_COLUMNS.filter((c) => visibleKeys.value.includes(c.key)).map((c) => ({
    title: c.title,
    key: c.key,
    width: columnWidths.value[c.key] ?? c.width,
    align: c.align,
    sortable: false,
  })),
)

function loadColumnConfig(): void {
  try {
    const raw = localStorage.getItem(STORAGE_COLUMNS) ?? ''
    if (!raw) return
    const known = new Set(SECURITY_COLUMNS.map((c) => c.key))
    const saved = raw.split(',').map((k) => k.trim()).filter((k) => known.has(k))
    if (saved.length > 0) visibleKeys.value = saved
  } catch {
    // localStorage 不可用时用默认列
  }
}

function loadColumnWidths(): void {
  try {
    const raw = localStorage.getItem(STORAGE_WIDTHS) ?? ''
    if (!raw) return
    const parsed = JSON.parse(raw) as Record<string, number>
    const known = new Set(SECURITY_COLUMNS.map((c) => c.key))
    const next: Record<string, number> = {}
    for (const [key, width] of Object.entries(parsed)) {
      if (known.has(key) && Number.isFinite(width) && width >= 48) next[key] = Math.round(width)
    }
    columnWidths.value = next
  } catch {
    columnWidths.value = {}
  }
}

function persistColumnWidths(): void {
  try {
    localStorage.setItem(STORAGE_WIDTHS, JSON.stringify(columnWidths.value))
  } catch {
    // 忽略存储失败（仅本次生效）
  }
}

// 拖拽调列宽：mousedown 记录起点，mousemove 实时更新，mouseup 落盘
const resizing = ref<{ key: string; startX: number; startWidth: number } | null>(null)

function startResize(event: MouseEvent, key: string): void {
  const th = (event.currentTarget as HTMLElement).closest('th')
  const startWidth = th?.getBoundingClientRect().width ?? columnWidths.value[key] ?? 120
  resizing.value = { key, startX: event.clientX, startWidth }
  window.addEventListener('mousemove', onResizeMove)
  window.addEventListener('mouseup', stopResize)
}

function onResizeMove(event: MouseEvent): void {
  const current = resizing.value
  if (!current) return
  const width = Math.max(48, Math.round(current.startWidth + (event.clientX - current.startX)))
  columnWidths.value = { ...columnWidths.value, [current.key]: width }
}

function stopResize(): void {
  if (!resizing.value) return
  resizing.value = null
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
  persistColumnWidths()
}

/** 双击表头右边界：恢复该列默认宽度。 */
function resetColumnWidth(key: string): void {
  const next = { ...columnWidths.value }
  delete next[key]
  columnWidths.value = next
  persistColumnWidths()
}

function openColumnDialog(): void {
  columnDraft.value = [...visibleKeys.value]
  columnDialog.value = true
}

function resetColumns(): void {
  columnDraft.value = [...DEFAULT_VISIBLE_KEYS]
  columnWidths.value = {}
  persistColumnWidths()
}

function saveColumns(): void {
  visibleKeys.value = [...columnDraft.value]
  try {
    localStorage.setItem(STORAGE_COLUMNS, visibleKeys.value.join(','))
  } catch {
    // 忽略存储失败（仅本次生效）
  }
  columnDialog.value = false
}

const rows = ref<SecurityRow[]>([])
const total = ref(0)
const page = ref(1)
// 每页条数（需求 11332：默认 15，可在 15/20/50/100/200 间切换）
const PAGE_SIZE_OPTIONS = [15, 20, 50, 100, 200]
const pageSize = ref(15)
const keyword = ref('')
// 筛选（需求 11330）：交易所 / 市场 / 类型 / 子类 / 货币 / 状态 / 提供方
const exchangeFilter = ref<number | null>(null)
const marketFilter = ref<number | null>(null)
const securityTypeFilter = ref('')
const typeSecuSubFilter = ref('')
const currencyFilter = ref('')
const flagEnableFilter = ref('')
const providerFilter = ref('')

interface SecurityFilterOptions {
  types: string[]
  type_secus: string[]
  type_secu_subs: string[]
  currencies: string[]
  providers: string[]
}
const filterOptions = ref<SecurityFilterOptions>({
  types: [],
  type_secus: [],
  type_secu_subs: [],
  currencies: [],
  providers: [],
})
const typeItems = computed(() => [
  { title: '全部类型', value: '' },
  ...filterOptions.value.types.map((v) => ({ title: v, value: v })),
])
const subTypeItems = computed(() => [
  { title: '全部子类', value: '' },
  ...filterOptions.value.type_secu_subs.map((v) => ({ title: v, value: v })),
])
const currencyItems = computed(() => [
  { title: '全部货币', value: '' },
  ...filterOptions.value.currencies.map((v) => ({ title: v, value: v })),
])
const providerItems = computed(() => [
  { title: '全部提供方', value: '' },
  ...filterOptions.value.providers.map((v) => ({ title: v, value: v })),
])
const exchangeFilterItems = computed(() => [
  { title: '全部交易所', value: null },
  ...exchangeItems.value,
])
const marketFilterItems = computed(() => [
  { title: '全部市场', value: null },
  ...marketItems.value,
])
// 对话框下拉候选（需求 11347）：静态兜底 + 后端 filterOptions 合并，全部允许手动输入其它值。
// 静态值取自 public.finv_security 现网高频值（2026-11-19 实测）：
//   security_type: Stock 13485 / ETF 6933 / StockIndex 1471 / Forex 601 / LOF 347 / Fund 242
//                  / Futures 206 / Crypto 193 / VolatilityIndex 164 / BondIndex 25
//   currency_type: USD 10425 / CNY 9292 / HKD 3532 / JPY 41 / AUD 34 / EUR 32 / GBP 30 / SGD 29 / CAD 27 / CNH 25
const SECURITY_TYPE_CANDIDATES = [
  'Stock',
  'ETF',
  'StockIndex',
  'LOF',
  'Fund',
  'Futures',
  'Forex',
  'Crypto',
  'VolatilityIndex',
  'BondIndex',
  'Warrant',
  'Bond',
]
const CURRENCY_TYPE_CANDIDATES = [
  'CNY',
  'USD',
  'HKD',
  'JPY',
  'EUR',
  'GBP',
  'AUD',
  'CAD',
  'SGD',
  'CNH',
  'TWD',
  'KRW',
  'CHF',
]
// 「时区偏移」只放偏移量（±HH:MM），**不放 IANA 标识**——那是「时区标识」字段的内容。
// 现网 finv_security.timezone 实测：+08:00 12758 / -04:00 10176 / +00:00 173（另有 5 行填了 IANA 名，属历史脏数据）
const TIMEZONE_OFFSET_CANDIDATES = [
  '+08:00',
  '+09:00',
  '+00:00',
  '-04:00',
  '-05:00',
  '-06:00',
  '-07:00',
  '-08:00',
  '+05:30',
  '+07:00',
  '+10:00',
  '+12:00',
  '-03:00',
  '-03:30',
  '+13:00',
  '+14:00',
]
// 「时区标识」放 IANA 名。取自现网 finv_security.tz 实测分布：
//   Asia/Shanghai 12498 / America/New_York 10176 / Asia/Hong_Kong 257 / Etc/UTC 172
//   / Asia/Singapore 3 / Asia/Tokyo 1 / Europe/London 1 / Australia/Sydney 1
const TIMEZONE_ID_CANDIDATES = [
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Taipei',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Bangkok',
  'Asia/Kolkata',
  'Asia/Dubai',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'America/Toronto',
  'Europe/London',
  'Europe/Paris',
  'Europe/Frankfurt',
  'Europe/Zurich',
  'Australia/Sydney',
  'Pacific/Auckland',
  'Etc/UTC',
]

// 新增证券按交易所预填「计价货币 + 时区偏移」（需求 11347）。
// 前段依据 finv_security 现网分布（按 exchange_code 分组实测）：
//   11/12/13 → CNY +08:00（4706 / 4122 / 358 行）
//   21       → HKD +08:00（3497 行）
//   30/31/32/35 → USD -04:00（9197 / 885 / 61 行）
//   10001    → USD +00:00（172 行）
// 其余交易所现网无数据或不足 5 行，按各市场常规交易时区取值。
const EXCHANGE_DEFAULTS: Record<number, { currency: string; timezone: string; tz: string }> = {
  10: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  11: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  12: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  13: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  14: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  15: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  19: { currency: 'CNY', timezone: '+08:00', tz: 'Asia/Shanghai' },
  21: { currency: 'HKD', timezone: '+08:00', tz: 'Asia/Hong_Kong' },
  22: { currency: 'HKD', timezone: '+08:00', tz: 'Asia/Hong_Kong' },
  29: { currency: 'TWD', timezone: '+08:00', tz: 'Asia/Taipei' },
  30: { currency: 'USD', timezone: '-04:00', tz: 'America/New_York' },
  31: { currency: 'USD', timezone: '-04:00', tz: 'America/New_York' },
  32: { currency: 'USD', timezone: '-04:00', tz: 'America/New_York' },
  33: { currency: 'USD', timezone: '-05:00', tz: 'America/Chicago' },
  34: { currency: 'USD', timezone: '-05:00', tz: 'America/Chicago' },
  35: { currency: 'USD', timezone: '-04:00', tz: 'America/New_York' },
  36: { currency: 'USD', timezone: '-05:00', tz: 'America/Chicago' },
  37: { currency: 'USD', timezone: '-05:00', tz: 'America/Chicago' },
  38: { currency: 'USD', timezone: '-05:00', tz: 'America/Chicago' },
  40: { currency: 'CAD', timezone: '-05:00', tz: 'America/Toronto' },
  51: { currency: 'JPY', timezone: '+09:00', tz: 'Asia/Tokyo' },
  52: { currency: 'JPY', timezone: '+09:00', tz: 'Asia/Tokyo' },
  53: { currency: 'SGD', timezone: '+08:00', tz: 'Asia/Singapore' },
  10001: { currency: 'USD', timezone: '+00:00', tz: 'Etc/UTC' },
}

// 新增证券的类型缺省值：Stock（现网最高频，13485 行）
const DEFAULT_SECURITY_TYPE = 'Stock'
// finv_security.init_date 的缺省占位值（DB 默认值，现网 23736 行）
const DEFAULT_INIT_DATE = 20000000
// 对齐 DB CHECK 约束 finv_security_init_date_check / finv_security_exchange_code_check
// 上市日期下限已放宽到 1000-01-01（V300233，需求 11348）：美股有上市超过 200 年的公司
// （如 Bank of New York 1784 年、纽交所 1792 年），1950 年的下限会把它们挡住
const INIT_DATE_MIN = 10000101
const INIT_DATE_MAX = 21001231
const EXCHANGE_CODE_MIN = 1
const EXCHANGE_CODE_MAX = 999999

/**
 * 取 combobox 的文本值。
 *
 * v-combobox 的 model 值可能是字符串、{title,value} 对象、或数组（项目内
 * HistoryQuoteQueryView 已有同样结论）。直接 Number()/trim() 会得到
 * [object Object] → NaN，导致「明明选了大于 0 的值却提示不合法」。
 */
function comboboxText(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (Array.isArray(value)) return value.length ? comboboxText(value[0]) : ''
  const record = value as { value?: unknown; title?: unknown }
  if (record.value !== null && record.value !== undefined) return comboboxText(record.value)
  if (record.title !== null && record.title !== undefined) return comboboxText(record.title)
  return String(value).trim()
}

/** 合并去重（保持顺序）。 */
function uniq(values: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (!text || seen.has(text)) continue
    seen.add(text)
    out.push(text)
  }
  return out
}

/** yyyymmdd 整数 → yyyy-mm-dd（占位值 20000000 等无效日期返回 null）。 */
function yyyymmddToIso(value: number | string | null | undefined): string | null {
  const text = String(value ?? '').trim()
  if (!/^\d{8}$/.test(text)) return null
  const year = text.slice(0, 4)
  const month = text.slice(4, 6)
  const day = text.slice(6, 8)
  if (month === '00' || day === '00') return null // 20000000 = 未知/未上市占位
  return `${year}-${month}-${day}`
}

/** yyyy-mm-dd → yyyymmdd 整数。 */
function isoToYyyymmdd(iso: string): number {
  return Number(iso.replace(/-/g, ''))
}

const FLAG_FILTER_ITEMS = [
  { title: '全部状态', value: '' },
  { title: '启用', value: '1' },
  { title: '禁用', value: '0' },
]

const loading = ref(false)
const error = ref('')

// 下拉选项：交易所（finv_exchange）/ 市场（finv_market），仅启用记录
const exchangeOptions = ref<ExchangeOption[]>([])
const marketOptions = ref<MarketOption[]>([])
const exchangeItems = computed(() =>
  exchangeOptions.value.map((e) => ({
    title: `${e.exchange_code} ${e.exchange_abbr}（${e.exchange_name}）`,
    value: e.exchange_code,
  })),
)
const marketItems = computed(() =>
  marketOptions.value.map((m) => ({
    title: `${m.market_code} ${m.market_flag}（${m.market_name}）`,
    value: m.market_code,
  })),
)

// 对话框下拉候选：后端 filterOptions（现网 distinct 值）优先，静态候选兜底，全部允许手动输入
const securityTypeFormItems = computed(() =>
  uniq([...filterOptions.value.types, ...SECURITY_TYPE_CANDIDATES]),
)
const currencyTypeFormItems = computed(() =>
  uniq([...filterOptions.value.currencies, ...CURRENCY_TYPE_CANDIDATES]),
)
// 证券分类（type_secu，子类的父类）候选：现网 distinct 值 + 静态兜底
// 现网实测：Stock 13485 / ETF 6933 / StockIndex 1471 / Forex 601 / LOF 347 / Fund 242
//           / Futures 206 / Crypto 193 / VolatilityIndex 164 / BondIndex 25 / Sector 18 / OTC 15
const TYPE_SECU_CANDIDATES = [
  'Stock',
  'ETF',
  'LOF',
  'Fund',
  'StockIndex',
  'VolatilityIndex',
  'BondIndex',
  'Sector',
  'Forex',
  'Futures',
  'Crypto',
  'OTC',
]
const typeSecuFormItems = computed(() => uniq([...filterOptions.value.type_secus, ...TYPE_SECU_CANDIDATES]))
// 证券子类（type_secu_sub）候选：现网实测 Stock.US 4818 / ETF.US 4650 / Stock.SZ 2935
// / Stock.HK 2791 / Stock.SH 2344 / ETF.SH 1122 / ETF.SZ 829 / CER 572 / StockIndex.SH 413 等
const typeSecuSubFormItems = computed(() => filterOptions.value.type_secu_subs)
// 数据提供方候选：现网 provider 实测 FTMM 23218 行、空串 524 行；其余为已知外部来源，作兜底
const PROVIDER_CANDIDATES = ['FTMM', 'XETAPP', 'SUPABASE', 'MANUAL']
const providerFormItems = computed(() => uniq([...filterOptions.value.providers, ...PROVIDER_CANDIDATES]))
const timezoneOffsetFormItems = TIMEZONE_OFFSET_CANDIDATES
const timezoneIdFormItems = TIMEZONE_ID_CANDIDATES
// 对话框里的交易所 / 市场候选：value 统一为字符串，便于与手填内容比较
const exchangeFormItems = computed(() =>
  exchangeOptions.value.map((e) => ({
    title: `${e.exchange_code} ${e.exchange_abbr}（${e.exchange_name}）`,
    value: String(e.exchange_code),
  })),
)
const marketFormItems = computed(() =>
  marketOptions.value.map((m) => ({
    title: `${m.market_code} ${m.market_flag}（${m.market_name}）`,
    value: String(m.market_code),
  })),
)

// 新增/编辑对话框
const dialog = ref(false)
const editing = ref(false)
// 交易所代码 / 市场代码在对话框里用字符串承载（需求 11347：支持手填下拉之外的任意值）
const formExchangeCode = ref('')
const formMarketCode = ref('')
// 上市日期日历弹层
const dateMenu = ref(false)
// 记录上一次「按交易所自动预填」的值：当前值仍等于它说明用户没手改过，可以继续联动覆盖
let autoCurrency = ''
let autoTimezone = ''
let autoTzId = ''
const form = ref<SecurityRow>({
  usc: '',
  exchange_code: 0,
  market_code: 0,
  security_type: DEFAULT_SECURITY_TYPE,
  security_code: '',
  security_name: '',
  security_name_cn: '',
  security_name_full: null,
  currency_type: '',
  init_date: DEFAULT_INIT_DATE,
  timezone: null,
  tz: null,
  flag_enable: '1',
  usn: null,
  type_secu: '',
  type_secu_sub: '',
  remark: '',
  provider: '',
  gmt_create: '',
  gmt_update: '',
  is_favorite: false,
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (exchangeFilter.value) params.set('exchangeCode', String(exchangeFilter.value))
    if (marketFilter.value) params.set('marketCode', String(marketFilter.value))
    if (securityTypeFilter.value) params.set('securityType', securityTypeFilter.value)
    if (typeSecuSubFilter.value) params.set('typeSecuSub', typeSecuSubFilter.value)
    if (currencyFilter.value) params.set('currencyType', currencyFilter.value)
    if (flagEnableFilter.value) params.set('flagEnable', flagEnableFilter.value)
    if (providerFilter.value) params.set('provider', providerFilter.value)
    const data = await apiGet<{ total: number; list: SecurityRow[] }>(
      `/Meta/Finv/Quant/Metadata/Security/List?${params.toString()}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = false
  formExchangeCode.value = ''
  formMarketCode.value = ''
  autoCurrency = ''
  autoTimezone = ''
  autoTzId = ''
  dateMenu.value = false
  form.value = {
    usc: '',
    exchange_code: 0,
    market_code: 0,
    security_type: DEFAULT_SECURITY_TYPE,
    security_code: '',
    security_name: '',
    security_name_cn: '',
    security_name_full: null,
    currency_type: '',
    init_date: DEFAULT_INIT_DATE,
    timezone: null,
    tz: null,
    flag_enable: '1',
    usn: null,
    type_secu: '',
    type_secu_sub: '',
    remark: '',
    provider: '',
    gmt_create: '',
    gmt_update: '',
    is_favorite: false,
  }
  dialog.value = true
}

function openEdit(row: SecurityRow) {
  editing.value = true
  form.value = { ...row }
  // 交易所 / 市场代码转成字符串承载，允许用户改成下拉之外的任意值
  formExchangeCode.value = row.exchange_code ? String(row.exchange_code) : ''
  formMarketCode.value = row.market_code === null || row.market_code === undefined ? '' : String(row.market_code)
  // 编辑模式不联动预填，避免覆盖已有数据
  autoCurrency = form.value.currency_type ?? ''
  autoTimezone = form.value.timezone ?? ''
  autoTzId = form.value.tz ?? ''
  dateMenu.value = false
  dialog.value = true
}

/** 交易所代码变化时，按交易所预填计价货币与时区偏移（需求 11347）。 */
watch(formExchangeCode, (code) => {
  if (editing.value) return
  // v-combobox 可能回传 {title,value} 对象，必须先归一化再查表，否则永远取不到交易所缺省值
  const defaults = EXCHANGE_DEFAULTS[Number(comboboxText(code))]
  if (!defaults) return
  const currency = (form.value.currency_type ?? '').trim()
  if (!currency || currency === autoCurrency) {
    form.value.currency_type = defaults.currency
    autoCurrency = defaults.currency
  }
  const timezone = (form.value.timezone ?? '').trim()
  if (!timezone || timezone === autoTimezone) {
    form.value.timezone = defaults.timezone
    autoTimezone = defaults.timezone
  }
  const tzId = (form.value.tz ?? '').trim()
  if (!tzId || tzId === autoTzId) {
    form.value.tz = defaults.tz
    autoTzId = defaults.tz
  }
})

/**
 * Vuetify 4 的 v-date-picker 选中后回传值可能是 Date 对象（项目内 HistoryQuoteQueryView
 * 已有同样结论），也可能是 yyyy-mm-dd 字符串。这里统一规范化，非日期返回 null。
 */
function normalizeIsoDate(value: unknown): string | null {
  if (value === null || value === undefined || value === '') return null
  if (typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value)) return value
  const parsed = value instanceof Date ? value : new Date(value as string)
  if (Number.isNaN(parsed.getTime())) return null
  const month = String(parsed.getMonth() + 1).padStart(2, '0')
  const day = String(parsed.getDate()).padStart(2, '0')
  return `${parsed.getFullYear()}-${month}-${day}`
}

/** 日历当前选中值（只读，供 v-date-picker 回显）。 */
const initDateIso = computed<string | null>(() => yyyymmddToIso(form.value.init_date))

/**
 * 日历选中日期后写回 init_date（yyyymmdd 整数）。
 *
 * 显式处理 update:model-value，不用 computed setter：Vuetify 4 传的是 Date 对象，
 * computed setter 里做字符串操作会直接抛错，导致字段一直不更新。
 */
function onInitDatePicked(value: unknown) {
  const iso = normalizeIsoDate(value)
  if (!iso) return
  form.value.init_date = isoToYyyymmdd(iso)
  dateMenu.value = false
}

async function save() {
  if (!form.value.usc.trim()) {
    showToast('统一证券代码（usc）不能为空', 'error')
    return
  }
  // 交易所 / 市场代码支持手填下拉之外的任意值，但仍需是合法整数（对齐 DB CHECK 约束）
  // 注意用 comboboxText 归一化：v-combobox 选中下拉项后可能回传 {title,value} 对象，
  // 直接 String()/Number() 会变成 [object Object] → NaN，误报「必须是不小于 0 的整数」
  const exchangeText = comboboxText(formExchangeCode.value)
  if (!exchangeText) {
    showToast('交易所代码不能为空（可从下拉选择，也可直接输入任意值）', 'error')
    return
  }
  const exchangeCode = Number(exchangeText)
  if (!Number.isInteger(exchangeCode) || exchangeCode < EXCHANGE_CODE_MIN || exchangeCode > EXCHANGE_CODE_MAX) {
    showToast(`交易所代码必须是 ${EXCHANGE_CODE_MIN}–${EXCHANGE_CODE_MAX} 的整数`, 'error')
    return
  }
  const marketText = comboboxText(formMarketCode.value)
  const marketCode = marketText === '' ? 0 : Number(marketText)
  if (!Number.isInteger(marketCode) || marketCode < 0 || marketCode > EXCHANGE_CODE_MAX) {
    showToast(`市场代码必须是不小于 0 的整数（留空按 0 处理）`, 'error')
    return
  }
  if (!form.value.security_code.trim() || !form.value.security_name_cn.trim()) {
    showToast('源证券代码与中文名称不能为空', 'error')
    return
  }
  const initDate = Number(form.value.init_date)
  if (!Number.isInteger(initDate) || initDate < INIT_DATE_MIN || initDate > INIT_DATE_MAX) {
    showToast(`上市日期必须是 ${INIT_DATE_MIN}–${INIT_DATE_MAX} 的 yyyymmdd 整数（缺省 20000000 表示未知）`, 'error')
    return
  }
  try {
    await apiPost('/Meta/Finv/Quant/Metadata/Security/Save', {
      usc: form.value.usc.trim(),
      exchange_code: exchangeCode,
      market_code: marketCode,
      security_type: comboboxText(form.value.security_type),
      type_secu: comboboxText(form.value.type_secu),
      type_secu_sub: comboboxText(form.value.type_secu_sub),
      security_code: form.value.security_code.trim(),
      security_name: form.value.security_name.trim(),
      security_name_cn: form.value.security_name_cn.trim(),
      security_name_full: form.value.security_name_full?.trim() || null,
      currency_type: comboboxText(form.value.currency_type),
      init_date: form.value.init_date,
      timezone: comboboxText(form.value.timezone) || null,
      tz: comboboxText(form.value.tz) || null,
      provider: comboboxText(form.value.provider),
      // 备注允许清空（导入批次类备注本身会过期），只做首尾空白裁剪
      remark: (form.value.remark ?? '').trim(),
    })
    showToast('保存成功')
    dialog.value = false
    await load()
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

/** 收藏 / 取消收藏（需求 11312）：成功后只改本地标记，不整页刷新。 */
async function toggleFavorite(row: SecurityRow) {
  const next = !row.is_favorite
  try {
    await apiPost('/Meta/Finv/Quant/Metadata/Security/Favorite', {
      usc: row.usc,
      favorite: next,
    })
    row.is_favorite = next
    showToast(next ? `已收藏证券 ${row.usc}` : `已取消收藏证券 ${row.usc}`)
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

async function toggle(row: SecurityRow) {
  const next = row.flag_enable === '1' ? '0' : '1'
  try {
    await apiPost('/Meta/Finv/Quant/Metadata/Security/Toggle', { usc: row.usc, flag_enable: next })
    row.flag_enable = next
    showToast(`已${next === '1' ? '启用' : '禁用'}证券 ${row.usc}`)
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

async function loadOptions() {
  try {
    const [exData, mkData] = await Promise.all([
      apiGet<{ list: ExchangeOption[] }>('/Meta/Finv/Quant/Metadata/Exchange/List?page=1&pageSize=500'),
      apiGet<{ list: MarketOption[] }>('/Meta/Finv/Quant/Metadata/Market/List?page=1&pageSize=500'),
    ])
    // 仅启用（flag_enable='1'）的记录入下拉；保持后端排序（启用优先）
    exchangeOptions.value = (exData.list ?? []).filter((e) => e.flag_enable === '1')
    marketOptions.value = (mkData.list ?? []).filter((m) => m.flag_enable === '1')
  } catch {
    exchangeOptions.value = []
    marketOptions.value = []
  }
}

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const pageJump = ref<number | null>(null)

function search() {
  page.value = 1
  load()
}

/** 跳页：越界自动裁剪到 [1, pageCount]。 */
function jumpToPage(): void {
  const target = Math.round(Number(pageJump.value ?? 0))
  if (!Number.isFinite(target) || target <= 0) return
  page.value = Math.min(pageCount.value, Math.max(1, target))
  pageJump.value = null
  load()
}

/** 重置筛选条件（不影响列配置与列宽）。 */
function resetFilters(): void {
  keyword.value = ''
  exchangeFilter.value = null
  marketFilter.value = null
  securityTypeFilter.value = ''
  typeSecuSubFilter.value = ''
  currencyFilter.value = ''
  flagEnableFilter.value = ''
  providerFilter.value = ''
  search()
}

async function loadFilterOptions(): Promise<void> {
  try {
    const data = await apiGet<SecurityFilterOptions>(
      '/Meta/Finv/Quant/Metadata/Security/FilterOptions',
    )
    filterOptions.value = {
      types: data.types ?? [],
      type_secus: data.type_secus ?? [],
      type_secu_subs: data.type_secu_subs ?? [],
      currencies: data.currencies ?? [],
      providers: data.providers ?? [],
    }
  } catch {
    filterOptions.value = { types: [], type_secus: [], type_secu_subs: [], currencies: [], providers: [] }
  }
}

onMounted(() => {
  loadColumnConfig()
  loadColumnWidths()
  loadFilterOptions()
  load()
  loadOptions()
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onResizeMove)
  window.removeEventListener('mouseup', stopResize)
})

/** RFC3339 -> 本地 yyyy-MM-dd HH:mm:ss（空值 —）。 */
function timeText(value: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes(),
  )}:${pad(d.getSeconds())}`
}
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-tag-multiple" class="mr-2" />
      规范证券信息维护
    </v-card-title>
    <v-card-subtitle>
      finv_security 证券代码字典维护：查询展示、新增、修改、禁用/启用（usc 为统一证券代码）
    </v-card-subtitle>

    <v-card-text>
      <!-- 筛选（需求 11330）：关键字 + 交易所/市场/类型/子类/货币/状态/提供方 -->
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（usc / 证券代码 / 证券名称）"
          placeholder="如: GCMain / NVDA / 黄金"
          density="compact"
          hide-details
          clearable
          style="min-width: 240px; max-width: 320px"
          @keyup.enter="search"
        />
        <v-select
          v-model="exchangeFilter"
          :items="exchangeFilterItems"
          item-title="title"
          item-value="value"
          label="交易所"
          density="compact"
          hide-details
          clearable
          style="max-width: 210px"
          @update:model-value="search"
        />
        <v-select
          v-model="marketFilter"
          :items="marketFilterItems"
          item-title="title"
          item-value="value"
          label="市场"
          density="compact"
          hide-details
          clearable
          style="max-width: 200px"
          @update:model-value="search"
        />
        <v-select
          v-model="securityTypeFilter"
          :items="typeItems"
          item-title="title"
          item-value="value"
          label="类型"
          density="compact"
          hide-details
          style="max-width: 150px"
          @update:model-value="search"
        />
        <v-autocomplete
          v-model="typeSecuSubFilter"
          :items="subTypeItems"
          item-title="title"
          item-value="value"
          label="子类"
          density="compact"
          hide-details
          clearable
          style="min-width: 200px; max-width: 260px"
          @update:model-value="search"
        />
        <v-select
          v-model="currencyFilter"
          :items="currencyItems"
          item-title="title"
          item-value="value"
          label="货币"
          density="compact"
          hide-details
          style="max-width: 130px"
          @update:model-value="search"
        />
        <v-select
          v-model="flagEnableFilter"
          :items="FLAG_FILTER_ITEMS"
          item-title="title"
          item-value="value"
          label="状态"
          density="compact"
          hide-details
          style="max-width: 120px"
          @update:model-value="search"
        />
        <v-select
          v-model="providerFilter"
          :items="providerItems"
          item-title="title"
          item-value="value"
          label="提供方"
          density="compact"
          hide-details
          style="max-width: 150px"
          @update:model-value="search"
        />
        <v-btn color="primary" prepend-icon="mdi-magnify" @click="search">查询</v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="resetFilters">重置</v-btn>
        <v-spacer />
        <v-btn
          color="secondary"
          variant="tonal"
          prepend-icon="mdi-view-column-outline"
          @click="openColumnDialog"
        >
          选择列（{{ visibleKeys.length }} / {{ SECURITY_COLUMNS.length }}）
        </v-btn>
        <v-btn color="success" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
      </div>

      <v-alert v-if="error" type="error" class="mb-3" density="compact">{{ error }}</v-alert>

      <v-data-table
        :headers="[...headers, { title: '操作', key: 'actions', width: 210, sortable: false }]"
        :items="rows"
        :loading="loading"
        :items-per-page="-1"
        hide-default-footer
        item-value="usc"
        density="compact"
        class="finv-sec-table"
      >
        <!-- 表头右边界可拖拽调列宽（需求 11329）；双击恢复该列默认宽度 -->
        <template v-for="column in headers" :key="`header-${column.key}`" #[`header.${column.key}`]>
          <div class="th-cell">
            <span class="th-title">{{ column.title }}</span>
            <span
              class="col-resizer"
              :title="`拖动调整「${column.title}」列宽；双击恢复默认`"
              @mousedown.stop.prevent="startResize($event, column.key)"
              @dblclick.stop="resetColumnWidth(column.key)"
            />
          </div>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.security_type="{ item }">
          <span :class="item.security_type ? '' : 'text-medium-emphasis'">
            {{ item.security_type || '—' }}
          </span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.security_name_full="{ item }">
          <span v-if="!item.security_name_full" class="text-medium-emphasis">—</span>
          <v-tooltip v-else :text="item.security_name_full" location="top" max-width="520">
            <template #activator="{ props }">
              <v-icon v-bind="props" icon="mdi-information-outline" size="small" />
            </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
          </v-tooltip>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.remark="{ item }">
          <span v-if="!item.remark" class="text-medium-emphasis">—</span>
          <v-tooltip v-else :text="item.remark" location="top" max-width="520">
            <template #activator="{ props }">
              <v-icon v-bind="props" icon="mdi-information-outline" size="small" />
            </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
          </v-tooltip>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.timezone="{ item }">
          <span :class="item.timezone ? '' : 'text-medium-emphasis'">{{ item.timezone || '—' }}</span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.tz="{ item }">
          <span :class="item.tz ? '' : 'text-medium-emphasis'">{{ item.tz || '—' }}</span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.provider="{ item }">
          <span :class="item.provider ? '' : 'text-medium-emphasis'">{{ item.provider || '—' }}</span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.gmt_create="{ item }">
          <span class="text-no-wrap">{{ timeText(item.gmt_create) }}</span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.gmt_update="{ item }">
          <span class="text-no-wrap">{{ timeText(item.gmt_update) }}</span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.usn="{ item }">
          <span :class="item.usn ? 'num' : 'text-medium-emphasis'">
            {{ item.usn ?? '—' }}
          </span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.type_secu="{ item }">
          <span :class="item.type_secu ? '' : 'text-medium-emphasis'">
            {{ item.type_secu || '—' }}
          </span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.type_secu_sub="{ item }">
          <span :class="item.type_secu_sub ? '' : 'text-medium-emphasis'">
            {{ item.type_secu_sub || '—' }}
          </span>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.flag_enable="{ item }">
          <v-chip :color="item.flag_enable === '1' ? 'success' : 'grey'" size="small">
            {{ item.flag_enable === '1' ? '启用' : '禁用' }}
          </v-chip>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
        <template #item.actions="{ item }">
          <v-btn
            size="x-small"
            variant="text"
            :icon="item.is_favorite ? 'mdi-heart' : 'mdi-heart-outline'"
            :color="item.is_favorite ? 'red-darken-2' : undefined"
            :title="item.is_favorite ? '取消收藏' : '收藏'"
            :aria-label="item.is_favorite ? '取消收藏' : '收藏'"
            @click="toggleFavorite(item)"
          />
          <v-btn size="x-small" variant="text" prepend-icon="mdi-pencil" @click="openEdit(item)">修改</v-btn>
          <v-btn
            size="x-small"
            variant="text"
            :prepend-icon="item.flag_enable === '1' ? 'mdi-toggle-switch-off' : 'mdi-toggle-switch'"
            @click="toggle(item)"
          >
            {{ item.flag_enable === '1' ? '禁用' : '启用' }}
          </v-btn>
        </template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
      </v-data-table>

      <!-- 列配置（需求 11328）：勾选要展示的列，保存到 localStorage -->
      <v-dialog v-model="columnDialog" max-width="640" scrollable>
        <v-card>
          <v-card-title class="text-body-1 d-flex align-center">
            <v-icon icon="mdi-view-column-outline" class="mr-2" />
            选择展示列
            <v-spacer />
            <span class="text-caption text-medium-emphasis">
              {{ columnDraft.length }} / {{ SECURITY_COLUMNS.length }}
            </span>
          </v-card-title>
          <v-card-text style="max-height: 60vh">
            <v-checkbox
              v-for="column in SECURITY_COLUMNS"
              :key="column.key"
              v-model="columnDraft"
              :value="column.key"
              :label="column.title"
              density="compact"
              hide-details
            />
          </v-card-text>
          <v-card-actions>
            <v-btn variant="text" prepend-icon="mdi-restore" @click="resetColumns">
              恢复默认（含列宽）
            </v-btn>
            <v-spacer />
            <v-btn variant="text" @click="columnDialog = false">取消</v-btn>
            <v-btn color="primary" @click="saveColumns">保存</v-btn>
          </v-card-actions>
        </v-card>
      </v-dialog>

      <!-- 服务端分页：唯一分页控件（表格自带 footer 已关闭，需求 11331） -->
      <div class="d-flex flex-wrap align-center justify-space-between ga-2 mt-3">
        <span class="text-caption text-medium-emphasis">
          共 {{ total }} 条{{ total ? `，第 ${page} / ${pageCount} 页` : '' }}
        </span>
        <div class="d-flex align-center ga-2">
          <span class="text-caption text-medium-emphasis">每页</span>
          <v-select
            v-model="pageSize"
            :items="PAGE_SIZE_OPTIONS"
            density="compact"
            hide-details
            style="width: 88px"
            @update:model-value="search"
          />
          <span class="text-caption text-medium-emphasis">条</span>
          <v-pagination
            v-model="page"
            :length="pageCount"
            :total-visible="7"
            density="compact"
            @update:model-value="load"
          />
          <v-text-field
            v-model.number="pageJump"
            type="number"
            label="跳至"
            density="compact"
            hide-details
            style="width: 96px"
            @keyup.enter="jumpToPage"
          />
          <v-btn variant="text" size="small" :disabled="!pageJump" @click="jumpToPage">跳转</v-btn>
        </div>
      </div>
    </v-card-text>

    <!-- 新增 / 编辑对话框 -->
    <v-dialog v-model="dialog" max-width="680">
      <v-card>
        <v-card-title>{{ editing ? '修改证券' : '新增证券' }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model="form.usc" label="usc（统一证券代码，全局唯一）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_code" label="源证券代码（交易所原始代码）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_name_cn" label="证券名称（中文）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.security_name" label="源证券名称" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.security_name_full" label="证券名称（全称，可选）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="formExchangeCode"
                @update:model-value="(v: unknown) => (formExchangeCode = comboboxText(v))"
                label="交易所代码"
                :items="exchangeFormItems"
                density="compact"
                clearable
                hint="可从下拉选择，也可直接输入其它整数（1–999999）"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="formMarketCode"
                @update:model-value="(v: unknown) => (formMarketCode = comboboxText(v))"
                label="市场代码"
                :items="marketFormItems"
                density="compact"
                clearable
                hint="可从下拉选择，也可直接输入其它整数（留空按 0）"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.security_type"
                @update:model-value="(v: unknown) => (form.security_type = comboboxText(v))"
                label="证券类型"
                :items="securityTypeFormItems"
                density="compact"
                hint="缺省 Stock；可从下拉切换，也可直接输入"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.type_secu"
                @update:model-value="(v: unknown) => (form.type_secu = comboboxText(v))"
                label="证券分类"
                :items="typeSecuFormItems"
                density="compact"
                clearable
                hint="type_secu，证券子类的父类（如 Stock / ETF）；可从下拉切换或直接输入"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.type_secu_sub"
                @update:model-value="(v: unknown) => (form.type_secu_sub = comboboxText(v))"
                label="证券子类"
                :items="typeSecuSubFormItems"
                density="compact"
                clearable
                hint="type_secu_sub（如 Stock.US / ETF.SH）；可从下拉切换或直接输入"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.currency_type"
                @update:model-value="(v: unknown) => (form.currency_type = comboboxText(v))"
                label="计价货币"
                :items="currencyTypeFormItems"
                density="compact"
                hint="按交易所自动预填（如 11→CNY），可改或直接输入"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-menu v-model="dateMenu" :close-on-content-click="false">
                <template #activator="{ props: menuProps }">
                  <v-text-field
                    v-bind="menuProps"
                    v-model.number="form.init_date"
                    label="上市日期（yyyymmdd）"
                    type="number"
                    density="compact"
                    append-inner-icon="mdi-calendar"
                    hint="缺省 20000000 表示未知；可手输 8 位数字或点右侧日历选择（支持 1950 年前的老公司）"
                    persistent-hint
                  />
                </template>
                <v-date-picker
                  :model-value="initDateIso"
                  :min="'1000-01-01'"
                  :max="'2100-12-31'"
                  show-adjacent-months
                  @update:model-value="onInitDatePicked"
                />
              </v-menu>
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.timezone"
                @update:model-value="(v: unknown) => (form.timezone = comboboxText(v) || null)"
                label="时区偏移"
                :items="timezoneOffsetFormItems"
                density="compact"
                hint="只填偏移量（如 +08:00 / -04:00），按交易所自动预填"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.tz"
                @update:model-value="(v: unknown) => (form.tz = comboboxText(v) || null)"
                label="时区标识"
                :items="timezoneIdFormItems"
                density="compact"
                clearable
                hint="IANA 标识（如 America/New_York / Asia/Shanghai），按交易所自动预填，可改或直接输入"
                persistent-hint
              />
            </v-col>
            <v-col cols="6">
              <v-combobox
                :model-value="form.provider"
                @update:model-value="(v: unknown) => (form.provider = comboboxText(v))"
                label="提供方"
                :items="providerFormItems"
                density="compact"
                clearable
                hint="数据提供方（如 FTMM）；可从下拉切换或直接输入，留空不改原值"
                persistent-hint
              />
            </v-col>
            <v-col cols="12">
              <v-text-field
                :model-value="form.remark"
                @update:model-value="(v: unknown) => (form.remark = String(v ?? ''))"
                label="备注"
                density="compact"
                maxlength="200"
                counter="200"
                hint="自由文本（现网多为导入批次说明，如 20261030-SupabasePG 批量导入）；可清空"
                persistent-hint
              />
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>
</template>

<style scoped>
/* 不压缩列（需求 11329）：单元格不换行，列宽不足时表格整体横向滚动，
   而不是把长文本挤成多行。 */
.finv-sec-table :deep(table) {
  /* auto：列宽按设定宽度 + 内容最小宽，容器不够时横向滚动而不是压缩换行 */
  table-layout: auto;
}

.finv-sec-table :deep(th),
.finv-sec-table :deep(td) {
  white-space: nowrap;
}

.finv-sec-table :deep(th) {
  position: relative;
  user-select: none;
}

/* 表头单元格：标题 + 右侧拖拽手柄 */
.finv-sec-table .th-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.finv-sec-table .th-title {
  flex: 1 1 auto;
}

.finv-sec-table .col-resizer {
  flex: 0 0 auto;
  width: 8px;
  height: 18px;
  cursor: col-resize;
  border-right: 2px solid rgba(var(--v-border-color), var(--v-border-opacity));
  border-radius: 1px;
}

.finv-sec-table .col-resizer:hover {
  border-right-color: rgb(var(--v-theme-primary));
}
</style>
