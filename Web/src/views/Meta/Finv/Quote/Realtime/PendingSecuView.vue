<script setup lang="ts">
// 待录入证券（需求 10995）
// 接口：GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending?keyword=&marketLabel=&orderBy=&order=&page=&pageSize=
//
// 数据语义：cdeu.finv_cdeu_secu_realtime_stock 中 usc 为空的快照行 —— 采集会把所有在售证券
// 都写进快照表，usc 为空表示该证券尚未录入证券主数据，是后续补录以下三张表的依据：
//   public.finv_security（证券主数据）
//   futu.finv_futu_quote_collect（采集配置）
// 需求 11147 起 `cdeu.finv_quote_cdeu_usc` 不再录入（数据由运行时采集链路动态生成）。
// 手动补录 usc（需求 11146）：POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill
//   —— 按派生键关联 futu.finv_futu_secu_key，把 usc 为空的行就地写回快照表（幂等）。
// 本页其余能力仍只做查询；「一键录入」为规划功能（需求登记在案，尚未实现）。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'
import PendingSecuEnrollDialog from './PendingSecuEnrollDialog.vue'

// 列表行（wire 字段 snake_case，数值列为文本，保住 NUMERIC(24,6) 精度）
interface PendingSecuRow {
  stock_id: number
  usc: string
  // 派生键 upper(市场标签.代码)：后端用它关联键表补 usc（需求 11145）
  ftmm_symbol: string
  name: string
  stock_code: string
  market_type: string
  market_label: string
  instrument_type: string
  currency: string
  price_nominal: string
  change_price: string
  change_ratio: string
  price_direct: string
  market_value: string
  trade_trunover: string
  trade_volumn: string
  page: number | null
  dt_update: number
}

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending'

const rows = ref<PendingSecuRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const marketLabel = ref('')
const orderBy = ref('market_value')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const error = ref('')
// 录入对话框（需求 10996）：当前只做字段登记与必填校验，写入接口待接入
const enrollOpen = ref(false)
const enrollRow = ref<PendingSecuRow | null>(null)

const ORDER_OPTIONS = [
  { title: '总市值', value: 'market_value' },
  { title: '成交额', value: 'trade_trunover' },
  { title: '涨跌幅', value: 'change_ratio' },
  { title: '来源页码', value: 'page' },
  { title: '更新时间', value: 'dt_update' },
  { title: '证券编号', value: 'stock_id' },
]
const MARKET_OPTIONS = [
  { title: '全部市场', value: '' },
  { title: 'HK', value: 'HK' },
  { title: 'SH', value: 'SH' },
  { title: 'SZ', value: 'SZ' },
  { title: 'BJ', value: 'BJ' },
  { title: 'US', value: 'US' },
]

// 富途 instrument_type 字典（口径取自 futu.finv_futu_quote_collect 实际数据：
// 3=stock / 4=etf / 6=index / 7=sectors / 9=futures / 11=crypto / 12=stock）。
// 表格列只显示中文（DictColumnSpec），未登记的值原样显示，空值显示 —。
const INSTRUMENT_TYPE_LABELS: Record<string, string> = {
  '3': '股票',
  '4': 'ETF',
  '6': '指数',
  '7': '板块',
  '9': '期货',
  '11': '加密货币',
  '12': '股票',
}

function instrumentTypeText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return INSTRUMENT_TYPE_LABELS[key] ?? key
}

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// ---- 手动补录 usc（需求 11146）----

interface USCBackfillResult {
  null_before: number
  updated: number
  unresolved: number
  elapsed_ms: number
}

const backfilling = ref(false)

/**
 * 手动补录 usc：按派生键 `upper(市场标签.代码)` 关联本地键表，
 * 把 usc 为空（NULL / 空白）的快照行就地补上；键表里没有的保持不动。
 * 幂等，可反复点击；补完刷新列表。
 */
async function backfillUSC() {
  backfilling.value = true
  try {
    // 全表一次 UPDATE，给 120s 超时；服务端无请求体，传空对象
    const data = await apiPost<USCBackfillResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/USCBackfill',
      {},
      120000,
    )
    const parts = [
      `本次补录 ${data.updated} 行`,
      `补录前为空 ${data.null_before} 行`,
      data.unresolved > 0 ? `仍无法补全 ${data.unresolved} 行（键表里没有对应证券）` : '已全部补全',
      `${data.elapsed_ms} ms`,
    ]
    showToast(`usc 补录完成：${parts.join('、')}`, data.unresolved > 0 ? 'warning' : 'success')
    await load()
  } catch (err) {
    showToast(`usc 补录失败：${(err as Error).message}`, 'error')
  } finally {
    backfilling.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      orderBy: orderBy.value,
      order: order.value,
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (marketLabel.value) params.set('marketLabel', marketLabel.value)
    const data = await apiGet<{
      total: number
      page: number
      pageSize: number
      list: PendingSecuRow[]
    }>(`${API}?${params.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = (err as Error).message
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function reset() {
  keyword.value = ''
  marketLabel.value = ''
  orderBy.value = 'market_value'
  order.value = 'desc'
  search()
}

// ---- 展示格式化（与「证券行情排行」保持一致）----

function trimZero(value: string): string {
  if (!value) return ''
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}

function dec(value: string, digits = 4): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  return trimZero(n.toFixed(digits))
}

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

function pct(value: string): string {
  if (!value) return '—'
  return `${dec(value, 2)}%`
}

function timeText(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  if (Number.isNaN(d.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes(),
  )}:${pad(d.getSeconds())}`
}

/** 整行着色（红涨绿跌），与「证券行情排行」一致 */
function rowClass(direct: string): string {
  if (direct === '+') return 'row-rise'
  if (direct === '-') return 'row-fall'
  return ''
}

function openEnroll(row: PendingSecuRow) {
  enrollRow.value = row
  enrollOpen.value = true
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-database-plus-outline" class="mr-2" />
      待录入证券
      <v-spacer />
      <v-tooltip text="规划功能：批量录入（当前请用列表行内的「录入」按钮逐只登记）" location="bottom">
        <template #activator="{ props }">
          <span v-bind="props">
            <v-btn variant="tonal" prepend-icon="mdi-playlist-plus" disabled>一键录入（规划中）</v-btn>
          </span>
        </template>
      </v-tooltip>
    </v-card-title>
    <v-card-subtitle class="pb-2">
      cdeu.finv_cdeu_secu_realtime_stock 中 <strong>补全后仍为空</strong>的快照：该证券尚未录入证券主数据
      （新上市 / 未纳入采集），是后续补录
      <code>public.finv_security</code> + <code>futu.finv_futu_quote_collect</code> +
      <code>cdeu.finv_quote_cdeu_usc</code> 的依据
    </v-card-subtitle>

    <v-card-text>
      <v-alert type="info" density="compact" variant="tonal" class="mb-3">
        列表口径：先用<strong>派生键</strong> <code>ftmm_symbol = upper(市场标签 || '.' || 代码)</code>
        关联 <code>futu.finv_futu_secu_key</code> 补一次 <code>usc</code>（本页「唯一标识」列即该值），
        <strong>补完仍为空</strong>的才列在这里 —— 键表里已登记的证券不会被当成待录入。
        上面这一步只是「查询时判定」，不写库；要把 <code>usc</code> 真正写回快照表，点<strong>「手动补录USC」</strong>。
        注意 usc 为空是<strong>刻意保留的观测信号</strong>，不要用 stock_code 直接回填：两者并非等价，
        直接挪用会把不同市场的同码证券映射错（如 <code>000001</code> 在 SZ 与 SH 都存在）。
      </v-alert>

      <!-- 筛选与操作同处一行，窄屏自动换行 -->
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（代码 / 名称 / 派生键）"
          placeholder="如: 06288 / FAST RETAIL"
          density="compact"
          hide-details
          clearable
          style="min-width: 240px; max-width: 320px"
          @keyup.enter="search"
        />
        <v-select
          v-model="marketLabel"
          :items="MARKET_OPTIONS"
          item-title="title"
          item-value="value"
          label="市场"
          density="compact"
          hide-details
          style="max-width: 130px"
          @update:model-value="search"
        />
        <v-select
          v-model="orderBy"
          :items="ORDER_OPTIONS"
          item-title="title"
          item-value="value"
          label="排序字段"
          density="compact"
          hide-details
          style="max-width: 150px"
          @update:model-value="search"
        />
        <v-select
          v-model="order"
          :items="[
            { title: '降序', value: 'desc' },
            { title: '升序', value: 'asc' },
          ]"
          item-title="title"
          item-value="value"
          label="方向"
          density="compact"
          hide-details
          style="max-width: 110px"
          @update:model-value="search"
        />
        <v-btn color="primary" prepend-icon="mdi-magnify" @click="search">查询</v-btn>
        <v-tooltip
          text="按唯一标识 ftmm_symbol（市场标签.代码，大写）关联 futu.finv_futu_secu_key，把 usc 为空（NULL / 空白）的快照行就地补上；键表里没有对应证券的行保持不动，仍是待录入。可反复点击（幂等）"
          location="bottom"
          max-width="420"
        >
          <template #activator="{ props }">
            <v-btn
              v-bind="props"
              color="secondary"
              variant="tonal"
              prepend-icon="mdi-link-variant-plus"
              :loading="backfilling"
              @click="backfillUSC"
            >
              手动补录USC
            </v-btn>
          </template>
        </v-tooltip>
        <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
      </div>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />

      <v-table density="compact" class="text-caption finv-pending-table">
        <thead>
          <tr>
            <th class="text-no-wrap">usc</th>
            <th class="text-no-wrap">证券编号</th>
            <th class="text-no-wrap">代码</th>
            <th class="text-no-wrap">名称</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap">
              <v-tooltip text="唯一标识 = upper(市场标签.代码)，与 futu.finv_futu_secu_key.ftmm_symbol 同口径；本页 usc 尚未补全，可据此核对补录后将写入的 usc" location="bottom" max-width="360">
                <template #activator="{ props: tipProps }">
                  <span v-bind="tipProps" class="d-inline-flex align-center">
                    唯一标识
                    <v-icon icon="mdi-information-outline" size="14" class="ml-1 text-medium-emphasis" />
                  </span>
                </template>
              </v-tooltip>
            </th>
            <th class="text-no-wrap">品种类型</th>
            <th class="text-no-wrap">币种</th>
            <th class="text-no-wrap text-right">现价</th>
            <th class="text-no-wrap text-right">涨跌额</th>
            <th class="text-no-wrap text-right">涨跌幅</th>
            <th class="text-no-wrap text-right">总市值</th>
            <th class="text-no-wrap text-right">成交额</th>
            <th class="text-no-wrap text-right">来源页码</th>
            <th class="text-no-wrap">快照时间</th>
            <th class="text-no-wrap">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.stock_id" :class="rowClass(row.price_direct)">
            <td class="text-no-wrap">
              <template v-if="row.usc">{{ row.usc }}</template>
              <v-tooltip v-else text="快照 usc 仍为空（本页只列补全后仍为空的证券）；补录后将写入的值见「唯一标识」列" location="bottom" max-width="360">
                <template #activator="{ props: tipProps }">
                  <span v-bind="tipProps" class="text-medium-emphasis">—</span>
                </template>
              </v-tooltip>
            </td>
            <td class="text-no-wrap">{{ row.stock_id }}</td>
            <td class="text-no-wrap">{{ row.stock_code || '—' }}</td>
            <td class="text-no-wrap">{{ row.name || '—' }}</td>
            <td class="text-no-wrap">{{ row.market_label || '—' }}</td>
            <td class="text-no-wrap"><code>{{ row.ftmm_symbol || '—' }}</code></td>
            <td class="text-no-wrap">{{ instrumentTypeText(row.instrument_type) }}</td>
            <td class="text-no-wrap">{{ row.currency || '—' }}</td>
            <td class="text-right num">{{ dec(row.price_nominal) }}</td>
            <td class="text-right num">{{ dec(row.change_price) }}</td>
            <td class="text-right num">{{ pct(row.change_ratio) }}</td>
            <td class="text-right num">{{ compact(row.market_value) }}</td>
            <td class="text-right num">{{ compact(row.trade_trunover) }}</td>
            <td class="text-right num">{{ row.page ?? '—' }}</td>
            <td class="text-no-wrap">{{ timeText(row.dt_update) }}</td>
            <td class="text-no-wrap">
              <v-btn size="x-small" variant="tonal" color="primary" prepend-icon="mdi-database-plus-outline" @click="openEnroll(row)">
                录入
              </v-btn>
            </td>
          </tr>
          <tr v-if="!rows.length && !loading">
            <td colspan="16" class="text-center text-medium-emphasis py-6">
              没有待录入证券（快照行的 usc 都能关联到证券主数据，或已由键表补全）
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 录入对话框：三张表字段登记 + 必填校验（需求 10996） -->
      <PendingSecuEnrollDialog v-model="enrollOpen" :row="enrollRow" @done="load" />

      <div class="d-flex flex-wrap align-center justify-space-between ga-2 mt-3">
        <span class="text-caption text-medium-emphasis">
          共 {{ total }} 条{{ total ? `，第 ${page} / ${pageCount} 页` : '' }}
        </span>
        <div class="d-flex align-center ga-2">
          <span class="text-caption text-medium-emphasis">每页</span>
          <v-select
            v-model="pageSize"
            :items="[20, 50, 100, 200]"
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
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.finv-pending-table :deep(th),
.finv-pending-table :deep(td) {
  white-space: nowrap;
  padding-inline: 12px;
}

.finv-pending-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

.finv-pending-table :deep(td.num) {
  min-width: 96px;
}

/* 红涨绿跌与「证券行情排行」保持同一套配色 */
.finv-pending-table :deep(tr.row-rise) {
  color: #ff0000;
}

.finv-pending-table :deep(tr.row-fall) {
  color: #00e676;
}
</style>
