<script setup lang="ts">
// 行情快照同步（需求 10982，10986 改为按「证券品种」分 Tab，11158 菜单改名去掉「手动」）
// 接口：
//   POST /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run   异步启动（返回 run_id）
//   GET  /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run?runId=  轮询状态 + 逐页日志 + 采集配置
// 采集逻辑：internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go
//
// 期货（需求 11274）：instrument_symbol='future' 时走独立分支，配置读
//   cdeu.finv_quote_cdeu_future_sync_config（按 futuresType 1~8 分行），
//   结果落 cdeu.finv_quote_realtime_future（主键 stock_id），下架由 flag_online 标记。
//   运行状态 / 日志 / 轮询接口与股票完全共用，故本页只多一份 futures_configs 与分类下拉。
//
// Tab 由配置表的 instrument_symbol 去重生成（当前只有 stock），
// 后续在 cdeu.finv_cdeu_secu_realtime_sync_config 里插入 etf/future/forex/option/crypto
// 行即可自动出现对应 Tab，无需改前端。
// 需求 11256：股票之后依次常显 ETF/指数/期货/加密货币/北交所股票/板块/投资组合/黄金 占位 Tab
// （配置表暂无这些品种的行，Tab 先占位、「开始同步」禁用，接入时插配置行即可）。
import { computed, onBeforeUnmount, onMounted, ref, nextTick } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 采集配置（cdeu.finv_cdeu_secu_realtime_sync_config 一行）
interface SyncConfig {
  instrument_symbol: string
  market_type: string
  count_plan: number | null
  count_exists: number
  total_available: number
}

// 期货列表采集配置（cdeu.finv_quote_cdeu_future_sync_config 一行）
interface FuturesConfig {
  market_type: string
  futures_type: string
  futures_type_name: string
  count_plan: number | null
  count_exists: number
  total_available: number
}

// 单个市场的采集结果（期货分支额外带 futures_type / futures_type_name）
interface RunResult {
  instrument_symbol: string
  market_type: string
  futures_type?: string
  futures_type_name?: string
  pages: number
  rows_receive: number
  insert: number
  update: number
  count_exists: number
  total_available: number
  elapsed_ms: number
  error?: string
}

// 一行采集日志
interface RunLog {
  time: string
  level: string
  market: string
  page: number
  message: string
}

// 一次采集运行
interface StockListRun {
  run_id: string
  status: string
  started_at: string
  finished_at?: string
  elapsed_ms: number
  params: Record<string, unknown>
  results: RunResult[]
  logs: RunLog[]
  error?: string
}

const START_API = '/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run'
const STATUS_API = '/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run'

// 品种显示名（新增品种时在此补一行，未登记的直接显示原始符号）
const SYMBOL_LABELS: Record<string, string> = {
  stock: '股票',
  etf: 'ETF',
  cnetf: '沪深京ETF',
  index: '指数',
  future: '期货',
  forex: '外汇',
  option: '期权',
  crypto: '加密货币',
  bsestock: '北交所股票',
  cnindex: '沪深京指数',
  sector: '板块',
  portfolio: '投资组合',
  otc: '中国OTC',
  gold: '黄金',
}
const SYMBOL_ICONS: Record<string, string> = {
  stock: 'mdi-chart-line',
  etf: 'mdi-chart-areaspline',
  cnetf: 'mdi-finance',
  index: 'mdi-chart-timeline-variant-shimmer',
  future: 'mdi-calendar-clock',
  forex: 'mdi-currency-usd',
  option: 'mdi-chart-bell-curve',
  crypto: 'mdi-bitcoin',
  bsestock: 'mdi-office-building-outline',
  cnindex: 'mdi-chart-box-outline',
  sector: 'mdi-view-grid-outline',
  portfolio: 'mdi-briefcase-outline',
  otc: 'mdi-swap-horizontal',
  gold: 'mdi-gold',
}

// 占位品种（需求 11256）：按此顺序排在配置表已有品种之后，暂未接入采集，仅出 Tab。
// 符号与后续配置表的 instrument_symbol 保持一致，接入时插配置行即自动生效。
// 期货已于需求 11274 接入真实采集，故从占位清单移除（Tab 位置由 SYMBOL_ORDER 决定）。
const PLACEHOLDER_SYMBOLS = ['index', 'etf', 'sector', 'portfolio', 'otc', 'gold']
// 期货符号（需求 11274）：配置读 cdeu.finv_quote_cdeu_future_sync_config，结果落
// cdeu.finv_quote_realtime_future；运行状态与日志复用同一套接口。
const FUTURES_SYMBOL = 'future'
// 已接入真实采集、但配置表（cdeu.finv_cdeu_secu_realtime_sync_config）没有采集行的品种：
// bsestock = 板块 GN2309（北交所上市企业）、cnetf = 国证ETF 指数 399380（沪深京 ETF），BDFS 源；
// cnindex = 沪深京指数（中信 XetApp，需求 11294），6 池合并去重，provider=ZXET。全量拉取，无需配置行。
// crypto = 收藏的加密货币（moomoo OpenAPI market-snapshot，需求 11316），来源为 quote.finv_quote_secu_fav，无需配置行。
const FIXED_SYMBOLS = ['bsestock', 'cnindex', 'cnetf', 'crypto']
// 固定来源品种各自的市场取值（服务端 BDFS 分支忽略该参数，仅用于界面展示与请求语义清晰）
const FIXED_MARKET: Record<string, { title: string; value: string }> = {
  bsestock: { title: 'BJ 北交所', value: 'BJ' },
  cnindex: { title: 'CN A 股（沪深京）', value: '4' },
  cnetf: { title: 'CN A 股（沪深京）', value: '4' },
  crypto: { title: '收藏的加密货币', value: '' },
}
// Tab 固定展示顺序（需求 11256/11262/11263）：配置表没出现的品种按此顺序补齐，
// bsestock 固定在「指数 / ETF / 期货 / 加密货币」之后，cnindex（沪深京指数，XetApp 源，需求 11294）与 cnetf（沪深京ETF，BDFS 源）紧随其后。
const SYMBOL_ORDER = ['stock', 'index', 'etf', 'future', 'crypto', 'bsestock', 'cnindex', 'cnetf', 'sector', 'portfolio', 'otc', 'gold']

const MARKET_LABELS: Record<string, string> = { '1': 'HK 港股', '2': 'US 美股', '4': 'CN A 股' }

const run = ref<StockListRun | null>(null)
const configs = ref<SyncConfig[]>([])
// 期货采集配置（cdeu.finv_quote_cdeu_future_sync_config，按 futuresType 1~8 分 8 行）
const futuresConfigs = ref<FuturesConfig[]>([])
// 上一次已提示过的「读取配置失败」原因，避免轮询期间重复弹 Toast
let lastConfigError = ''
const activeSymbol = ref('stock')
const starting = ref(false)
const logBox = ref<HTMLElement | null>(null)

// 采集范围与节奏（对应后端 StockListCollectRequest；市场为空=该品种全部市场）
const marketType = ref('')
// 期货分类（仅期货 Tab 生效；空=全部启用分类）
const futuresType = ref('')
const countPlan = ref<number | null>(null)
// 每页条数默认值：BDFS 板块采集默认 500（接口 rn），其余品种沿用 50
const BDFS_DEFAULT_PAGE_SIZE = 500
const PAGE_SIZE_DEFAULT = 50
// 期货（需求 11274）：每页 50 条（与后端 FutureListDefaultPageSize 一致），
// 每个分类通常只有 1~5 页，页间间隔 2s 即可
const FUTURES_PAGE_SIZE_DEFAULT = 50
const FUTURES_INTERVAL_DEFAULT = 2000
const pageSize = ref(PAGE_SIZE_DEFAULT)
const intervalMs = ref(6000)
const maxRetries = ref(2)
const retryBackoffMs = ref(2000)
const maxPages = ref(0)

let timer: number | null = null
let pollFailures = 0

/** 配置表里出现过的品种 + 固定来源品种 + 占位品种，统一按 SYMBOL_ORDER 排序（未登记在末尾追加） */
const symbols = computed(() => {
  const present: string[] = []
  const push = (symbol: string) => {
    if (symbol && !present.includes(symbol)) present.push(symbol)
  }
  for (const item of configs.value) push(item.instrument_symbol)
  if (!present.length) push('stock')
  for (const symbol of FIXED_SYMBOLS) push(symbol)
  // 期货（需求 11274）：配置在 cdeu.finv_quote_cdeu_future_sync_config，不在股票配置表里，
  // 因此必须常显（否则从占位清单移除后 Tab 会消失）。
  push(FUTURES_SYMBOL)
  for (const symbol of PLACEHOLDER_SYMBOLS) push(symbol)
  const ordered = SYMBOL_ORDER.filter((symbol) => present.includes(symbol))
  const rest = present.filter((symbol) => !SYMBOL_ORDER.includes(symbol))
  return [...ordered, ...rest]
})

/** 当前 Tab 是「固定来源」品种（无配置行但已接入真实采集，如 BDFS 板块成分股） */
const activeIsFixedSource = computed(() => FIXED_SYMBOLS.includes(activeSymbol.value))

/** 当前 Tab 是期货（走 cdeu.finv_quote_cdeu_future_sync_config + 独立结果表） */
const activeIsFutures = computed(() => activeSymbol.value === FUTURES_SYMBOL)

/** 期货分类下拉：首项「全部」，其余来自配置行（无中文名时显示「分类 N」） */
const futuresTypeOptions = computed(() => [
  { title: '全部期货分类', value: '' },
  ...futuresConfigs.value.map((item) => ({
    title: futuresTypeLabel(item),
    value: item.futures_type,
  })),
])

/** 当前品种是否有可启动的采集配置（配置表行 / 固定来源 / 期货配置行） */
const canStart = computed(
  () =>
    symbolConfigs.value.length > 0 ||
    activeIsFixedSource.value ||
    (activeIsFutures.value && futuresConfigs.value.length > 0),
)

/** 当前 Tab 品种的配置行 */
const symbolConfigs = computed(() => configs.value.filter((item) => item.instrument_symbol === activeSymbol.value))

/** 当前 Tab 是占位品种（在占位清单里且配置表尚无该品种的行） */
const activeIsPlaceholder = computed(
  () =>
    PLACEHOLDER_SYMBOLS.includes(activeSymbol.value) &&
    !symbolConfigs.value.length &&
    !(activeIsFutures.value && futuresConfigs.value.length),
)

/** 当前 Tab 品种的市场选项（来自配置行，插新行自动生效） */
const marketOptions = computed(() => {
  // 固定来源品种（BDFS）的市场由来源决定，只给一个选项且不可改
  const fixed = FIXED_MARKET[activeSymbol.value]
  if (fixed) return [fixed]
  return [
    { title: '全部市场', value: '' },
    ...symbolConfigs.value.map((item) => ({ title: marketLabel(item.market_type), value: item.market_type })),
  ]
})

/** 当前 Tab 品种的运行结果 */
const symbolResults = computed(() =>
  (run.value?.results ?? []).filter((item) => item.instrument_symbol === activeSymbol.value),
)

/** 当前 Tab 品种的日志（任务级日志 marketplace 为空，始终保留） */
const symbolLogs = computed(() =>
  (run.value?.logs ?? []).filter((item) => !item.market || item.market.startsWith(`${activeSymbol.value}/`)),
)

const running = computed(() => run.value?.status === 'RUNNING')
const anyRunning = computed(() => running.value)
const statusText = computed(() => {
  const status = run.value?.status
  if (!status) return '未运行'
  return { RUNNING: '运行中', SUCCESS: '成功', PARTIAL: '部分成功', FAILED: '失败' }[status] ?? status
})
const statusColor = computed(() => {
  const status = run.value?.status
  if (status === 'SUCCESS') return 'success'
  if (status === 'RUNNING') return 'info'
  if (status === 'PARTIAL') return 'warning'
  if (status === 'FAILED') return 'error'
  return 'grey'
})
const totals = computed(() =>
  symbolResults.value.reduce(
    (acc, item) => {
      acc.receive += item.rows_receive || 0
      acc.insert += item.insert || 0
      acc.update += item.update || 0
      return acc
    },
    { receive: 0, insert: 0, update: 0 },
  ),
)

function symbolLabel(value: string) {
  return SYMBOL_LABELS[value] ?? value
}

function symbolIcon(value: string) {
  return SYMBOL_ICONS[value] ?? 'mdi-shape-outline'
}

function marketLabel(value: string) {
  return MARKET_LABELS[value] ?? value
}

/** 期货分类显示名：有中文名用中文名，否则降级「分类 N」（配置表 futures_type_name 待补） */
function futuresTypeLabel(item: FuturesConfig) {
  const name = (item.futures_type_name ?? '').trim()
  return name || `分类 ${item.futures_type}`
}

/** 结果行里的分类显示（期货 Tab 专用） */
function resultFuturesLabel(item: RunResult) {
  const name = (item.futures_type_name ?? '').trim()
  if (name) return name
  return item.futures_type ? `分类 ${item.futures_type}` : '—'
}

function logColor(level: string) {
  if (level === 'ERROR') return 'text-error'
  if (level === 'WARN') return 'text-warning'
  return ''
}

async function scrollLogs() {
  await nextTick()
  if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight
}

/** 拉取运行状态（runId 为空时取最近一次） */
async function loadStatus(runId = '') {
  try {
    const query = runId ? `?runId=${encodeURIComponent(runId)}` : ''
    const payload = await apiGet<{
      run: StockListRun
      configs: SyncConfig[]
      futures_configs?: FuturesConfig[]
      error?: string
      futures_error?: string
    }>(`${STATUS_API}${query}`)
    // 后端读取配置失败时（配置表不存在/无权限等）会把原因放在 data.error；
    // 不提示的话页面只会显示「暂无启用的采集配置」，无法定位（需求 10990）。
    const cfgError = payload?.error ? String(payload.error) : ''
    if (cfgError && cfgError !== lastConfigError) {
      lastConfigError = cfgError
      showToast(`读取采集配置失败：${cfgError}`, 'error')
    }
    // 期货配置来自另一张表（cdeu.finv_quote_cdeu_future_sync_config），单独的键返回
    const futuresError = payload?.futures_error ? String(payload.futures_error) : ''
    if (futuresError && futuresError !== lastConfigError) {
      lastConfigError = futuresError
      showToast(`读取期货采集配置失败：${futuresError}`, 'error')
    }
    if (payload?.futures_configs) futuresConfigs.value = payload.futures_configs
    if (payload?.configs) {
      configs.value = payload.configs
      // 首次加载后确保当前 Tab 有效：占位品种也是有效 Tab（需求 11256），
      // 不能因为配置表里没有该品种就把用户切走。
      if (!symbols.value.includes(activeSymbol.value)) {
        activeSymbol.value = symbols.value[0] ?? 'stock'
      }
    }
    if (payload?.run?.run_id) {
      const logsChanged = payload.run.logs?.length !== (run.value?.logs?.length ?? 0)
      run.value = payload.run
      if (logsChanged) await scrollLogs()
    }
    pollFailures = 0
  } catch (error) {
    pollFailures += 1
    if (pollFailures >= 3) {
      stopPolling()
      showToast(`读取采集状态失败：${(error as Error).message}`, 'error')
    }
  }
}

function stopPolling() {
  if (timer !== null) {
    window.clearTimeout(timer)
    timer = null
  }
}

function schedulePoll() {
  stopPolling()
  timer = window.setTimeout(async () => {
    await loadStatus(run.value?.run_id ?? '')
    if (running.value) schedulePoll()
  }, 1500)
}

/** 启动当前 Tab 品种的同步任务 */
async function startSync() {
  starting.value = true
  try {
    const body: Record<string, unknown> = {
      instrument_symbol: activeSymbol.value,
      page_size: pageSize.value,
      interval_ms: intervalMs.value,
      max_retries: maxRetries.value,
      retry_backoff_ms: retryBackoffMs.value,
      max_pages: maxPages.value,
    }
    const fixedMarket = FIXED_MARKET[activeSymbol.value]
    if (activeIsFutures.value) {
      // 期货：分类可选，市场由配置行决定（默认 0=聚合口径），前端不传 market_type
      if (futuresType.value) body.futures_type = futuresType.value
    } else if (fixedMarket) body.market_type = fixedMarket.value
    else if (marketType.value) body.market_type = marketType.value
    if (countPlan.value !== null && countPlan.value > 0) body.count_plan = countPlan.value

    const payload = await apiPost<StockListRun>(START_API, body)
    run.value = payload
    showToast(`任务已启动（${symbolLabel(activeSymbol.value)}）：${payload?.run_id ?? ''}`, 'success')
    schedulePoll()
  } catch (error) {
    showToast(`启动失败：${(error as Error).message}`, 'error')
  } finally {
    starting.value = false
  }
}

/** 只刷新一次状态 */
async function refreshOnce() {
  await loadStatus(run.value?.run_id ?? '')
  if (running.value) schedulePoll()
}

function resetOptions() {
  marketType.value = FIXED_MARKET[activeSymbol.value]?.value ?? ''
  futuresType.value = ''
  countPlan.value = null
  pageSize.value = activeIsFutures.value
    ? FUTURES_PAGE_SIZE_DEFAULT
    : activeIsFixedSource.value
      ? BDFS_DEFAULT_PAGE_SIZE
      : PAGE_SIZE_DEFAULT
  intervalMs.value = activeIsFutures.value ? FUTURES_INTERVAL_DEFAULT : 6000
  maxRetries.value = 2
  retryBackoffMs.value = 2000
  maxPages.value = 0
}

/** 切换 Tab：清空当前品种专属的筛选值，避免把上个品种的市场带过去 */
function onSymbolChange() {
  marketType.value = FIXED_MARKET[activeSymbol.value]?.value ?? ''
  futuresType.value = ''
  countPlan.value = null
  pageSize.value = activeIsFutures.value
    ? FUTURES_PAGE_SIZE_DEFAULT
    : activeIsFixedSource.value
      ? BDFS_DEFAULT_PAGE_SIZE
      : PAGE_SIZE_DEFAULT
  intervalMs.value = activeIsFutures.value ? FUTURES_INTERVAL_DEFAULT : 6000
  scrollLogs()
}

onMounted(async () => {
  await loadStatus()
  if (running.value) schedulePoll()
})

onBeforeUnmount(stopPolling)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-cloud-sync-outline" class="mr-2" />
      行情快照同步
      <v-spacer />
      <v-chip :color="statusColor" variant="flat" size="small">
        {{ statusText }}
        <template v-if="run?.run_id"> · {{ run.run_id }}</template>
      </v-chip>
    </v-card-title>

    <!-- 证券品种 Tab：配置表已有品种 + 占位品种（需求 11256），配置行新增即自动生效 -->
    <v-tabs
      v-model="activeSymbol"
      density="compact"
      color="primary"
      show-arrows
      @update:model-value="onSymbolChange"
    >
      <v-tab v-for="symbol in symbols" :key="symbol" :value="symbol">
        <v-icon :icon="symbolIcon(symbol)" start size="small" />
        {{ symbolLabel(symbol) }}
      </v-tab>
    </v-tabs>
    <v-divider />

    <v-card-text>
      <!-- 当前品种的采集配置 -->
      <div class="text-subtitle-2 mb-1">{{ symbolLabel(activeSymbol) }}采集配置（数据库）</div>
      <v-table density="compact" class="text-caption mb-4">
        <thead>
          <tr v-if="activeIsFutures">
            <th class="text-no-wrap">分类</th>
            <th class="text-no-wrap">分类名称</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap text-right">计划数量</th>
            <th class="text-no-wrap text-right">分类合约总数</th>
            <th class="text-no-wrap text-right">本轮可采集</th>
          </tr>
          <tr v-else>
            <th class="text-no-wrap">品种</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap text-right">计划数量</th>
            <th class="text-no-wrap text-right">市场证券总数</th>
            <th class="text-no-wrap text-right">本轮可采集</th>
          </tr>
        </thead>
        <tbody>
          <template v-if="activeIsFutures">
            <tr v-for="item in futuresConfigs" :key="`future-${item.market_type}-${item.futures_type}`">
              <td class="text-no-wrap">{{ futuresTypeLabel(item) }}</td>
              <td class="text-no-wrap">{{ item.futures_type_name || '待确认' }}</td>
              <td class="text-no-wrap">{{ item.market_type }}</td>
              <td class="text-right">{{ item.count_plan ?? '全量' }}</td>
              <td class="text-right">{{ item.count_exists || '—' }}</td>
              <td class="text-right">{{ item.total_available || '—' }}</td>
            </tr>
          </template>
          <template v-else>
            <tr v-for="item in symbolConfigs" :key="`${item.instrument_symbol}-${item.market_type}`">
              <td>{{ symbolLabel(item.instrument_symbol) }}</td>
              <td class="text-no-wrap">{{ marketLabel(item.market_type) }}</td>
              <td class="text-right">{{ item.count_plan ?? '全量' }}</td>
              <td class="text-right">{{ item.count_exists || '—' }}</td>
              <td class="text-right">{{ item.total_available || '—' }}</td>
            </tr>
          </template>
          <tr v-if="activeIsFutures ? !futuresConfigs.length : !symbolConfigs.length">
            <td :colspan="activeIsFutures ? 6 : 5" class="text-center text-medium-emphasis py-4">
              <template v-if="activeIsFutures">
                期货采集配置表暂无启用行（可在 cdeu.finv_quote_cdeu_future_sync_config 插入 futures_type 1~8）
              </template>
              <template v-else-if="activeIsPlaceholder">
                该品种为占位 Tab：配置表暂无该品种的采集行，暂不参与采集
              </template>
              <template v-else-if="activeSymbol === 'bsestock'">
                该品种为 BDFS 板块采集（板块 GN2309 北交所上市企业），全量拉取成分股行情，无需配置表行；
                结果写入 cdeu.finv_quote_secu_realtime（provider=BDFS）
              </template>
              <template v-else-if="activeSymbol === 'cnindex'">
                该品种为沪深京指数采集（中信 XetApp，上游 4 池 + 深市 1200 + 北交所 1400 合并去重），
                全量拉取指数快照，无需配置表行；结果写入 cdeu.finv_quote_secu_realtime（provider=ZXET，instrument_type=6）
              </template>
              <template v-else-if="activeSymbol === 'crypto'">
                该品种为「收藏的加密货币」采集（moomoo OpenAPI market-snapshot）：
                来源为 quote.finv_quote_secu_fav 里收藏的加密货币（CC.&lt;usc&gt;，一次最多 400 个），
                结果写入 cdeu.finv_quote_secu_realtime_crypto（provider=MMAPI）；
                需服务端配置 FINV_MOOMOO_APP_KEY / FINV_MOOMOO_PRIVATE_KEY
              </template>
              <template v-else-if="activeSymbol === 'cnetf'">
                该品种为 BDFS ETF 采集（国证ETF 指数 399380，覆盖沪深京 ETF），全量拉取成分股行情，无需配置表行；
                结果写入 cdeu.finv_quote_secu_realtime（provider=BDFS，instrument_type=etf）
              </template>
              <template v-else>该品种暂无启用的采集配置（可在 cdeu.finv_cdeu_secu_realtime_sync_config 插入一行）</template>
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 采集范围与节奏 -->
      <div class="text-subtitle-2 mb-1">本次采集参数（{{ symbolLabel(activeSymbol) }}）</div>
      <div class="d-flex flex-wrap align-center ga-2 mb-4">
        <v-select
          v-if="activeIsFutures"
          v-model="futuresType"
          :items="futuresTypeOptions"
          item-title="title"
          item-value="value"
          label="期货分类"
          density="compact"
          hide-details
          style="max-width: 180px"
        />
        <v-select
          v-else
          v-model="marketType"
          :items="marketOptions"
          :disabled="activeIsFixedSource"
          :title="activeIsFixedSource ? 'BDFS 采集的市场由来源固定，不可更改' : ''"
          item-title="title"
          item-value="value"
          label="市场"
          density="compact"
          hide-details
          style="max-width: 150px"
        />
        <v-text-field
          v-model.number="countPlan"
          type="number"
          label="计划数量覆盖"
          placeholder="留空用配置值"
          density="compact"
          hide-details
          clearable
          style="max-width: 160px"
        />
        <v-text-field v-model.number="pageSize" type="number" label="每页条数" density="compact" hide-details style="max-width: 120px" />
        <v-text-field v-model.number="intervalMs" type="number" label="页间隔(ms)" density="compact" hide-details style="max-width: 130px" />
        <v-text-field v-model.number="maxRetries" type="number" label="重试次数" density="compact" hide-details style="max-width: 120px" />
        <v-text-field v-model.number="retryBackoffMs" type="number" label="退避(ms)" density="compact" hide-details style="max-width: 120px" />
        <v-text-field v-model.number="maxPages" type="number" label="页数上限" density="compact" hide-details style="max-width: 120px" />
        <v-btn
          color="primary"
          prepend-icon="mdi-play"
          :loading="starting"
          :disabled="anyRunning || !canStart"
          :title="canStart ? '' : '该品种暂无采集配置，无法启动同步'"
          @click="startSync"
        >
          开始同步
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" @click="refreshOnce">刷新状态</v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="resetOptions">重置参数</v-btn>
      </div>

      <!-- 汇总（仅当前品种） -->
      <div class="d-flex flex-wrap align-center ga-4 mb-2 text-caption">
        <span>状态：<strong>{{ statusText }}</strong></span>
        <span>耗时：{{ run?.elapsed_ms ?? 0 }} ms</span>
        <span>收到：{{ totals.receive }}</span>
        <span>新增：{{ totals.insert }}</span>
        <span>覆盖：{{ totals.update }}</span>
        <span v-if="run?.error" class="text-error">错误：{{ run.error }}</span>
      </div>

      <!-- 当前品种各市场结果 -->
      <v-table v-if="symbolResults.length" density="compact" class="text-caption mb-4">
        <thead>
          <tr>
            <th>品种/市场</th>
            <th class="text-right">页数</th>
            <th class="text-right">收到</th>
            <th class="text-right">新增</th>
            <th class="text-right">覆盖</th>
            <th class="text-right">市场总数</th>
            <th class="text-right">本轮可采</th>
            <th>结果</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in symbolResults"
            :key="`${item.instrument_symbol}-${item.market_type}-${item.futures_type ?? ''}`"
          >
            <td class="text-no-wrap">
              <template v-if="item.futures_type">{{ resultFuturesLabel(item) }}</template>
              <template v-else>{{ symbolLabel(item.instrument_symbol) }} / {{ marketLabel(item.market_type) }}</template>
            </td>
            <td class="text-right">{{ item.pages }}</td>
            <td class="text-right">{{ item.rows_receive }}</td>
            <td class="text-right">{{ item.insert }}</td>
            <td class="text-right">{{ item.update }}</td>
            <td class="text-right">{{ item.count_exists }}</td>
            <td class="text-right">{{ item.total_available }}</td>
            <td class="text-no-wrap">
              <span v-if="item.error" class="text-error">{{ item.error }}</span>
              <span v-else class="text-success">成功</span>
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 当前品种的逐页日志 -->
      <div class="text-subtitle-2 mb-1">采集执行日志（{{ symbolLabel(activeSymbol) }}）</div>
      <div ref="logBox" class="sync-log text-caption">
        <div v-for="(item, index) in symbolLogs" :key="index" :class="logColor(item.level)">
          <span class="text-medium-emphasis">[{{ item.time }}]</span>
          <span class="mx-1">{{ item.level }}</span>
          <span v-if="item.market" class="mx-1">({{ item.market }})</span>
          <span>{{ item.message }}</span>
        </div>
        <div v-if="!symbolLogs.length" class="text-medium-emphasis">
          暂无日志（点击「开始同步」后逐页输出）
        </div>
      </div>
    </v-card-text>
  </v-card>

  <!-- 说明卡片：页面最底部、唯一、标题固定为「说明」（Docs/DevSpec/ExplainCardSpec.md） -->
  <v-card class="mt-4">
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-information-outline" class="mr-2" />
      说明
    </v-card-title>
    <v-card-text>
      <div class="finv-note">
        <div class="finv-note-title">表口径</div>
        <div class="text-body-2 finv-note-body">
          股票 / ETF 快照写 <code>cdeu.finv_quote_secu_realtime</code>（主键 <code>usc</code>，一行一证券，
          <code>provider</code> 为 <code>FTMM</code> / <code>BDFS</code> / <code>ZXET</code>，按快照整行覆盖、不保留历史）；
          期货快照写 <code>cdeu.finv_quote_realtime_future</code>（主键 <code>stock_id</code>，一行一合约，
          <code>futures_type</code> 由最后采集的分类覆盖）。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">数据源</div>
        <div class="text-body-2 finv-note-body">
          股票走 moomoo <code>get-stock-list</code>（按 <code>marketType</code> 分市场）；ETF 走
          <code>get-etf-list</code>（市场由 <code>plateId</code> 决定：<code>900075</code> 港股、<code>10002799</code> 美股）；
          期货走 <code>get-futures-list</code>（<code>marketType=0</code> 聚合口径，按 <code>futuresType</code> 1~8 分 8 类）。
          固定来源品种例外：<code>bsestock</code> / <code>cnetf</code> 走 BDFS 成分股，不读配置表。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">配置与回写</div>
        <div class="text-body-2 finv-note-body">
          Tab 与市场下拉由配置行生成：股票各品种读 <code>cdeu.finv_cdeu_secu_realtime_sync_config</code>（一行 = 一个
          「市场 + 品种」），期货读 <code>cdeu.finv_quote_cdeu_future_sync_config</code>（一行 = 一个「市场 + 分类」）。
          每页回写进度，结束时把接口 <code>pagination.total</code> 写入 <code>count_exists</code>、本轮实际数量写入
          <code>total_available</code>；新增品种或分类只需插配置行，无需改前端。注意 <code>etf</code>（港美 ETF，
          moomoo）与 <code>cnetf</code>（沪深京 ETF，BDFS）是刻意分开的两个品种。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">读法与注意事项</div>
        <div class="text-body-2 finv-note-body">
          逐页日志里的 <code>[I]</code> = 新增、<code>[U]</code> = 覆盖，只列代码便于对数；
          <code>usc</code> 解析不到的行不进快照表，会落到「待采证券录入」等待补录。
          期货的下架标记（<code>flag_online='0'</code>）只在<strong>本轮全部分类都成功</strong>时按整轮水位统一置位，
          单分类运行或任一分类失败一律跳过——避免把其它分类仍在售的合约误标下架。
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
/* 说明卡片分节（Docs/DevSpec/ExplainCardSpec.md §4 统一 token） */
.finv-note {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}

.finv-note-title {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgba(var(--v-theme-primary), 1);
  margin-bottom: 2px;
}

.finv-note-body {
  opacity: 0.85;
  line-height: 1.6;
}

.sync-log {
  max-height: 320px;
  overflow-y: auto;
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(var(--v-theme-surface-variant), 0.35);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
