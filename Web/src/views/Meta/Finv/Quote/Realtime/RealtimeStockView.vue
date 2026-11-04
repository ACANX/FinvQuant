<script setup lang="ts">
// 证券行情排行（cdeu.finv_cdeu_secu_realtime_stock，需求 10962，更名 10964）
// 接口：GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Page?keyword=&marketLabel=&currency=&orderBy=&order=&page=&pageSize=
// 数值列由后端以文本返回（保住 NUMERIC(24,6) 精度），本页负责万/亿折算与小数位裁剪。
// 涨跌用整行颜色表达（红涨绿跌，row-rise / row-fall），不占单独的方向列。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'

// 列表行（wire 字段 snake_case，数值列为文本）
interface RealtimeStockRow {
  stock_id: number
  usc: string
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
  total_shares: string
  circulation_total_shares: string
  trade_trunover: string
  trade_volumn: string
  circulation_market_value: string
  trust_aum: string
  trade_change_ratio: string
  price_amplitude: string
  volumn_ratio: string
  buysell_ratio: string
  leverage: string
  dt_update: number
}

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page'

const rows = ref<RealtimeStockRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const marketLabel = ref('')
const currency = ref('')
const orderBy = ref('market_value')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const error = ref('')

const ORDER_OPTIONS = [
  { title: '总市值', value: 'market_value' },
  { title: '流通市值', value: 'circulation_market_value' },
  { title: '成交额', value: 'trade_trunover' },
  { title: '成交量', value: 'trade_volumn' },
  { title: '涨跌幅', value: 'change_ratio' },
  { title: '涨跌额', value: 'change_price' },
  { title: 'AUM', value: 'trust_aum' },
  { title: '量比', value: 'volumn_ratio' },
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
const CURRENCY_OPTIONS = [
  { title: '全部币种', value: '' },
  { title: 'CNY', value: 'CNY' },
  { title: 'HKD', value: 'HKD' },
  { title: 'USD', value: 'USD' },
]

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

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
    if (currency.value) params.set('currency', currency.value)
    const data = await apiGet<{ total: number; page: number; pageSize: number; list: RealtimeStockRow[] }>(
      `${API}?${params.toString()}`,
    )
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
  currency.value = ''
  orderBy.value = 'market_value'
  order.value = 'desc'
  search()
}

// ---- 展示格式化 ----

/** 去掉多余尾零：'1.230000' -> '1.23'、'2.000000' -> '2' */
function trimZero(value: string): string {
  if (!value) return ''
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}

/** 普通数值：最多 digits 位小数，去尾零；空值显示 — */
function dec(value: string, digits = 4): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  return trimZero(n.toFixed(digits))
}

/** 大额数值：万 / 亿 / 万亿 折算（市值、股本、成交额等） */
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

/** 百分点数值 -> '1.03%' */
function pct(value: string): string {
  if (!value) return '—'
  return `${dec(value, 2)}%`
}

/** 百万分之一量级的小数原样展示（量比、杠杆） */
function plain(value: string): string {
  return dec(value, 3)
}

/** Unix 毫秒 -> 本地 yyyy-MM-dd HH:mm:ss */
function timeText(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  if (Number.isNaN(d.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes(),
  )}:${pad(d.getSeconds())}`
}

/** 整行着色（红涨绿跌）：price_direct 为 + 显示红色、- 显示绿色，其余默认 */
function rowClass(direct: string): string {
  if (direct === '+') return 'row-rise'
  if (direct === '-') return 'row-fall'
  return ''
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-access-point" class="mr-2" />
      证券行情排行
    </v-card-title>
    <v-card-subtitle>
      cdeu.finv_cdeu_secu_realtime_stock：证券最新一张行情快照（一行一证券，按 stock_id 覆盖更新）
    </v-card-subtitle>

    <v-card-text>
      <!-- 筛选控件与操作按钮同处一行：按钮紧跟筛选控件之后，窄屏才自动换行 -->
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（usc / 代码 / 名称）"
          placeholder="如: 00002 / 中电控股"
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
          v-model="currency"
          :items="CURRENCY_OPTIONS"
          item-title="title"
          item-value="value"
          label="币种"
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
        <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
      </div>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />

      <v-table density="compact" class="text-caption finv-rt-table">
        <thead>
          <tr>
            <th class="text-no-wrap">usc</th>
            <th class="text-no-wrap">名称</th>
            <th class="text-no-wrap">代码</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap">币种</th>
            <th class="text-no-wrap text-right">现价</th>
            <th class="text-no-wrap text-right">涨跌额</th>
            <th class="text-no-wrap text-right">涨跌幅</th>
            <th class="text-no-wrap text-right">总市值</th>
            <th class="text-no-wrap text-right">成交额</th>
            <th class="text-no-wrap text-right">成交量</th>
            <th class="text-no-wrap text-right">总股本</th>
            <th class="text-no-wrap text-right">流通股本</th>
            <th class="text-no-wrap text-right">流通市值</th>
            <th class="text-no-wrap text-right">AUM</th>
            <th class="text-no-wrap text-right">成交额变化率</th>
            <th class="text-no-wrap text-right">振幅</th>
            <th class="text-no-wrap text-right">量比</th>
            <th class="text-no-wrap text-right">买卖比</th>
            <th class="text-no-wrap text-right">杠杆</th>
            <th class="text-no-wrap">更新时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.stock_id" :class="rowClass(row.price_direct)">
            <td class="text-no-wrap">{{ row.usc || '—' }}</td>
            <td class="text-no-wrap">{{ row.name || '—' }}</td>
            <td class="text-no-wrap">{{ row.stock_code || '—' }}</td>
            <td class="text-no-wrap">{{ row.market_label || '—' }}</td>
            <td class="text-no-wrap">{{ row.currency || '—' }}</td>
            <td class="text-right num">{{ plain(row.price_nominal) }}</td>
            <td class="text-right num">{{ dec(row.change_price) }}</td>
            <td class="text-right num">{{ pct(row.change_ratio) }}</td>
            <td class="text-right num">{{ compact(row.market_value) }}</td>
            <td class="text-right num">{{ compact(row.trade_trunover) }}</td>
            <td class="text-right num">{{ compact(row.trade_volumn) }}</td>
            <td class="text-right num">{{ compact(row.total_shares) }}</td>
            <td class="text-right num">{{ compact(row.circulation_total_shares) }}</td>
            <td class="text-right num">{{ compact(row.circulation_market_value) }}</td>
            <td class="text-right num">{{ compact(row.trust_aum) }}</td>
            <td class="text-right num">{{ pct(row.trade_change_ratio) }}</td>
            <td class="text-right num">{{ pct(row.price_amplitude) }}</td>
            <td class="text-right num">{{ plain(row.volumn_ratio) }}</td>
            <td class="text-right num">{{ pct(row.buysell_ratio) }}</td>
            <td class="text-right num">{{ plain(row.leverage) }}</td>
            <td class="text-no-wrap">{{ timeText(row.dt_update) }}</td>
          </tr>
          <tr v-if="!rows.length && !loading">
            <td colspan="21" class="text-center text-medium-emphasis py-6">暂无数据</td>
          </tr>
        </tbody>
      </v-table>

      <div class="d-flex flex-wrap align-center justify-space-between ga-2 mt-3">
        <span class="text-caption text-medium-emphasis">
          共 {{ total }} 条{{ total ? `，第 ${page} / ${pageCount} 页` : '' }}
        </span>
        <!-- 每页条数固定在页码控件左侧；「每页 / 条」用独立文本，
             不依赖 v-select 的浮动 label（窄宽度下会被省略成「每…」） -->
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
/* 列较多：所有单元格一律不换行，列宽随内容撑开（列多时整体横向滚动）；
   纵向不做内部滚动，表格按内容展开、整页滚动。 */
.finv-rt-table :deep(th),
.finv-rt-table :deep(td) {
  white-space: nowrap;
}

.finv-rt-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

/* compact 密度默认左右内边距偏窄，数值与单位（如「2.51 万亿」）容易顶到相邻列，适当放宽 */
.finv-rt-table :deep(th),
.finv-rt-table :deep(td) {
  padding-inline: 12px;
}

/* 指标列（数值/百分比/金额）给一个最小宽度，避免窄屏被压缩后视觉拥挤 */
.finv-rt-table :deep(td.num) {
  min-width: 96px;
}

/* 红涨绿跌：整行文字着色，不依赖单独的方向列。
   涨用正红 #FF0000；跌用亮绿 #00E676（Vuetify 的 error/success 主题色都偏暗，不够醒目）。
   不取纯绿 #00FF00：白底上对比度过低，长串数字会读不清。 */
.finv-rt-table :deep(tr.row-rise) {
  color: #ff0000;
}

.finv-rt-table :deep(tr.row-fall) {
  color: #00e676;
}
</style>
