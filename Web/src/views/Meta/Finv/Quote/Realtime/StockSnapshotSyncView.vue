<script setup lang="ts">
// 行情快照手动同步（需求 10982，10986 改为按「证券品种」分 Tab）
// 接口：
//   POST /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run   异步启动（返回 run_id）
//   GET  /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run?runId=  轮询状态 + 逐页日志 + 采集配置
// 采集逻辑：internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go
//
// Tab 由配置表的 instrument_symbol 去重生成（当前只有 stock），
// 后续在 cdeu.finv_cdeu_secu_realtime_sync_config 里插入 etf/future/forex/option/crypto
// 行即可自动出现对应 Tab，无需改前端。
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

// 单个市场的采集结果
interface RunResult {
  instrument_symbol: string
  market_type: string
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
  future: '期货',
  forex: '外汇',
  option: '期权',
  crypto: '加密货币',
}
const SYMBOL_ICONS: Record<string, string> = {
  stock: 'mdi-chart-line',
  etf: 'mdi-chart-areaspline',
  future: 'mdi-calendar-clock',
  forex: 'mdi-currency-usd',
  option: 'mdi-chart-bell-curve',
  crypto: 'mdi-bitcoin',
}

const MARKET_LABELS: Record<string, string> = { '1': 'HK 港股', '2': 'US 美股', '4': 'CN A 股' }

const run = ref<StockListRun | null>(null)
const configs = ref<SyncConfig[]>([])
// 上一次已提示过的「读取配置失败」原因，避免轮询期间重复弹 Toast
let lastConfigError = ''
const activeSymbol = ref('stock')
const starting = ref(false)
const logBox = ref<HTMLElement | null>(null)

// 采集范围与节奏（对应后端 StockListCollectRequest；市场为空=该品种全部市场）
const marketType = ref('')
const countPlan = ref<number | null>(null)
const pageSize = ref(50)
const intervalMs = ref(6000)
const maxRetries = ref(2)
const retryBackoffMs = ref(2000)
const maxPages = ref(0)

let timer: number | null = null
let pollFailures = 0

/** 配置表里出现过的品种（按配置顺序去重），至少给一个 stock 兜底 */
const symbols = computed(() => {
  const seen: string[] = []
  for (const item of configs.value) {
    if (item.instrument_symbol && !seen.includes(item.instrument_symbol)) {
      seen.push(item.instrument_symbol)
    }
  }
  return seen.length ? seen : ['stock']
})

/** 当前 Tab 品种的配置行 */
const symbolConfigs = computed(() => configs.value.filter((item) => item.instrument_symbol === activeSymbol.value))

/** 当前 Tab 品种的市场选项（来自配置行，插新行自动生效） */
const marketOptions = computed(() => [
  { title: '全部市场', value: '' },
  ...symbolConfigs.value.map((item) => ({ title: marketLabel(item.market_type), value: item.market_type })),
])

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
    const payload = await apiGet<{ run: StockListRun; configs: SyncConfig[]; error?: string }>(
      `${STATUS_API}${query}`,
    )
    // 后端读取配置失败时（配置表不存在/无权限等）会把原因放在 data.error；
    // 不提示的话页面只会显示「暂无启用的采集配置」，无法定位（需求 10990）。
    const cfgError = payload?.error ? String(payload.error) : ''
    if (cfgError && cfgError !== lastConfigError) {
      lastConfigError = cfgError
      showToast(`读取采集配置失败：${cfgError}`, 'error')
    }
    if (payload?.configs) {
      configs.value = payload.configs
      // 首次加载后确保当前 Tab 有效
      if (!payload.configs.some((item) => item.instrument_symbol === activeSymbol.value)) {
        const first = payload.configs[0]?.instrument_symbol
        if (first) activeSymbol.value = first
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
    if (marketType.value) body.market_type = marketType.value
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
  marketType.value = ''
  countPlan.value = null
  pageSize.value = 50
  intervalMs.value = 6000
  maxRetries.value = 2
  retryBackoffMs.value = 2000
  maxPages.value = 0
}

/** 切换 Tab：清空当前品种专属的筛选值，避免把上个品种的市场带过去 */
function onSymbolChange() {
  marketType.value = ''
  countPlan.value = null
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
      行情快照手动同步
      <v-spacer />
      <v-chip :color="statusColor" variant="flat" size="small">
        {{ statusText }}
        <template v-if="run?.run_id"> · {{ run.run_id }}</template>
      </v-chip>
    </v-card-title>
    <v-card-subtitle>
      读 cdeu.finv_cdeu_secu_realtime_sync_config 的启用配置，按品种分 Tab、按市场分页拉取 moomoo get-stock-list，
      逐页写入 cdeu.finv_cdeu_secu_realtime_stock，并回写 count_exists / total_available
    </v-card-subtitle>

    <!-- 证券品种 Tab：由配置表去重生成，新增 etf/future/forex/option/crypto 行即自动出现 -->
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
          <tr>
            <th class="text-no-wrap">品种</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap text-right">计划数量</th>
            <th class="text-no-wrap text-right">市场证券总数</th>
            <th class="text-no-wrap text-right">本轮可采集</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in symbolConfigs" :key="`${item.instrument_symbol}-${item.market_type}`">
            <td>{{ symbolLabel(item.instrument_symbol) }}</td>
            <td class="text-no-wrap">{{ marketLabel(item.market_type) }}</td>
            <td class="text-right">{{ item.count_plan ?? '全量' }}</td>
            <td class="text-right">{{ item.count_exists || '—' }}</td>
            <td class="text-right">{{ item.total_available || '—' }}</td>
          </tr>
          <tr v-if="!symbolConfigs.length">
            <td colspan="5" class="text-center text-medium-emphasis py-4">
              该品种暂无启用的采集配置（可在 cdeu.finv_cdeu_secu_realtime_sync_config 插入一行）
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 采集范围与节奏 -->
      <div class="text-subtitle-2 mb-1">本次采集参数（{{ symbolLabel(activeSymbol) }}）</div>
      <div class="d-flex flex-wrap align-center ga-2 mb-4">
        <v-select
          v-model="marketType"
          :items="marketOptions"
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
        <v-btn color="primary" prepend-icon="mdi-play" :loading="starting" :disabled="anyRunning" @click="startSync">
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
          <tr v-for="item in symbolResults" :key="`${item.instrument_symbol}-${item.market_type}`">
            <td class="text-no-wrap">{{ symbolLabel(item.instrument_symbol) }} / {{ marketLabel(item.market_type) }}</td>
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
</template>

<style scoped>
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
