<script setup lang="ts">
// 加密货币行情排行（cdeu.finv_quote_secu_realtime_crypto，主键 usc，需求 11323）
// 接口：GET /Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page?keyword=&quoteCurrency=&suspension=&orderBy=&order=&page=&pageSize=
// 数据由「行情快照同步 → 加密货币」Tab 的收藏采集写入（provider=MMAPI），本页只做查询。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'

interface CryptoRow {
  usc: string
  code: string
  name: string
  quote_currency: string
  price_nominal: string
  price_open: string
  price_highest: string
  price_lowest: string
  price_last_close: string
  price_avg: string
  change_price: string
  change_ratio: string
  price_amplitude: string
  price_direct: string
  trade_volumn: string
  trade_trunover: string
  volumn_ratio: string
  bid_price: string
  ask_price: string
  bid_vol: string
  ask_vol: string
  buysell_ratio: string
  highest52weeks: string
  lowest52weeks: string
  highest_history: string
  lowest_history: string
  sec_status: string
  suspension: string
  data_date: string
  provider: string
  dt_update: number
}

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page'
const CURRENCIES_API = '/Meta/Finv/Quote/Cdeu/Realtime/Crypto/QuoteCurrencies'

const rows = ref<CryptoRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const quoteCurrency = ref('')
const suspension = ref('')
const orderBy = ref('change_ratio')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const error = ref('')
const currencyOptions = ref<string[]>([])

const ORDER_OPTIONS = [
  { title: '涨跌幅', value: 'change_ratio' },
  { title: '涨跌额', value: 'change_price' },
  { title: '现价', value: 'price_nominal' },
  { title: '振幅', value: 'price_amplitude' },
  { title: '成交量', value: 'trade_volumn' },
  { title: '成交额', value: 'trade_trunover' },
  { title: '量比', value: 'volumn_ratio' },
  { title: '买卖比', value: 'buysell_ratio' },
  { title: '买价', value: 'bid_price' },
  { title: '卖价', value: 'ask_price' },
  { title: '52 周最高', value: 'highest52weeks' },
  { title: '52 周最低', value: 'lowest52weeks' },
  { title: '更新时间', value: 'dt_update' },
  { title: '代码', value: 'code' },
]
const SUSPENSION_OPTIONS = [
  { title: '全部状态', value: '' },
  { title: '未停牌', value: '0' },
  { title: '停牌', value: '1' },
]

// 币种下拉：首项「全部币种」，其余取表内 DISTINCT
const CURRENCY_OPTIONS = computed(() => [
  { title: '全部币种', value: '' },
  ...currencyOptions.value.map((c) => ({ title: c, value: c })),
])

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
    if (quoteCurrency.value) params.set('quoteCurrency', quoteCurrency.value)
    if (suspension.value) params.set('suspension', suspension.value)
    const data = await apiGet<{ total: number; page: number; pageSize: number; list: CryptoRow[] }>(
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

async function loadCurrencies() {
  try {
    const data = await apiGet<{ list: string[] }>(CURRENCIES_API)
    currencyOptions.value = data.list ?? []
  } catch {
    currencyOptions.value = []
  }
}

function search() {
  page.value = 1
  void load()
}

function reset() {
  keyword.value = ''
  quoteCurrency.value = ''
  suspension.value = ''
  orderBy.value = 'change_ratio'
  order.value = 'desc'
  search()
}

// ---- 展示格式化 ----

function trimZero(value: string): string {
  if (!value) return ''
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}

/** 价格类：加密币价格量级跨度大，最多保留 8 位小数再去尾零 */
function price(value: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  const abs = Math.abs(n)
  const digits = abs >= 100 ? 2 : abs >= 1 ? 4 : abs >= 0.01 ? 6 : 8
  return trimZero(n.toFixed(digits))
}

/** 数量类（成交量/成交额）：万 / 亿折算 */
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

function dec(value: string, digits = 3): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  return trimZero(n.toFixed(digits))
}

function pct(value: string): string {
  if (!value) return '—'
  return `${dec(value, 2)}%`
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

function rowClass(direct: string): string {
  if (direct === '+') return 'row-rise'
  if (direct === '-') return 'row-fall'
  return ''
}

function suspensionText(value: string): string {
  if (value === 'true') return '停牌'
  if (value === 'false') return '正常'
  return '—'
}

onMounted(async () => {
  await loadCurrencies()
  await load()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-currency-btc" class="mr-2" />
      加密货币行情排行
    </v-card-title>
    <v-card-subtitle>
      cdeu.finv_quote_secu_realtime_crypto：收藏币种最新一张行情快照（主键 usc，provider=MMAPI）
    </v-card-subtitle>

    <v-card-text>
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（usc / 代码 / 名称）"
          placeholder="如: BTC / CC.BTCUSD / Bitcoin"
          density="compact"
          hide-details
          clearable
          style="min-width: 240px; max-width: 320px"
          @keyup.enter="search"
        />
        <v-select
          v-model="quoteCurrency"
          :items="CURRENCY_OPTIONS"
          item-title="title"
          item-value="value"
          label="计价币种"
          density="compact"
          hide-details
          style="max-width: 150px"
          @update:model-value="search"
        />
        <v-select
          v-model="suspension"
          :items="SUSPENSION_OPTIONS"
          item-title="title"
          item-value="value"
          label="状态"
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
            <th class="text-no-wrap">代码</th>
            <th class="text-no-wrap">名称</th>
            <th class="text-no-wrap">计价币种</th>
            <th class="text-no-wrap text-right">现价</th>
            <th class="text-no-wrap text-right">开盘</th>
            <th class="text-no-wrap text-right">最高</th>
            <th class="text-no-wrap text-right">最低</th>
            <th class="text-no-wrap text-right">昨收</th>
            <th class="text-no-wrap text-right">涨跌额</th>
            <th class="text-no-wrap text-right">涨跌幅</th>
            <th class="text-no-wrap text-right">振幅</th>
            <th class="text-no-wrap text-right">成交量</th>
            <th class="text-no-wrap text-right">成交额</th>
            <th class="text-no-wrap text-right">量比</th>
            <th class="text-no-wrap text-right">买卖比</th>
            <th class="text-no-wrap text-right">买价</th>
            <th class="text-no-wrap text-right">卖价</th>
            <th class="text-no-wrap text-right">52周最高</th>
            <th class="text-no-wrap text-right">52周最低</th>
            <th class="text-no-wrap">状态</th>
            <th class="text-no-wrap">数据日期</th>
            <th class="text-no-wrap">更新时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.usc" :class="rowClass(row.price_direct)">
            <td class="text-no-wrap">{{ row.usc || '—' }}</td>
            <td class="text-no-wrap">{{ row.code || '—' }}</td>
            <td class="text-no-wrap">{{ row.name || '—' }}</td>
            <td class="text-no-wrap">{{ row.quote_currency || '—' }}</td>
            <td class="text-right num">{{ price(row.price_nominal) }}</td>
            <td class="text-right num">{{ price(row.price_open) }}</td>
            <td class="text-right num">{{ price(row.price_highest) }}</td>
            <td class="text-right num">{{ price(row.price_lowest) }}</td>
            <td class="text-right num">{{ price(row.price_last_close) }}</td>
            <td class="text-right num">{{ price(row.change_price) }}</td>
            <td class="text-right num">{{ pct(row.change_ratio) }}</td>
            <td class="text-right num">{{ pct(row.price_amplitude) }}</td>
            <td class="text-right num">{{ compact(row.trade_volumn) }}</td>
            <td class="text-right num">{{ compact(row.trade_trunover) }}</td>
            <td class="text-right num">{{ dec(row.volumn_ratio) }}</td>
            <td class="text-right num">{{ pct(row.buysell_ratio) }}</td>
            <td class="text-right num">{{ price(row.bid_price) }}</td>
            <td class="text-right num">{{ price(row.ask_price) }}</td>
            <td class="text-right num">{{ price(row.highest52weeks) }}</td>
            <td class="text-right num">{{ price(row.lowest52weeks) }}</td>
            <td class="text-no-wrap">{{ suspensionText(row.suspension) }}</td>
            <td class="text-no-wrap">{{ row.data_date || '—' }}</td>
            <td class="text-no-wrap">{{ timeText(row.dt_update) }}</td>
          </tr>
          <tr v-if="!rows.length && !loading">
            <td colspan="23" class="text-center text-medium-emphasis py-6">暂无数据</td>
          </tr>
        </tbody>
      </v-table>

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
.finv-rt-table :deep(th),
.finv-rt-table :deep(td) {
  white-space: nowrap;
  padding-inline: 12px;
}

.finv-rt-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

.finv-rt-table :deep(td.num) {
  min-width: 96px;
}

/* 红涨绿跌（与证券行情排行一致） */
.finv-rt-table :deep(tr.row-rise) {
  color: #ff0000;
}

.finv-rt-table :deep(tr.row-fall) {
  color: #00e676;
}
</style>
