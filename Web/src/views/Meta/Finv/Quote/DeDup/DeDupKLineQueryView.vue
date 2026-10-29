<script setup lang="ts">
// 去重分钟行情查询：按 usc + 交易日期范围分页查询 cdeu.finv_quote_secu_kline_min
import { onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../api'
import { fmtDate, fmtTime, shiftMonthISO, todayISO } from '../../../../../utils'

// 证券下拉选项（来自 finv_security 字典）
interface SecurityOption {
  usc: string
  security_name_cn: string
}

// /Page 紧凑字段行
interface KLineRow {
  usc: string
  ts: number
  date: number | null
  time: number | null
  o: number | null
  h: number | null
  l: number | null
  c: number | null
  v: number | null
  t: number | null
  cr: string | null
  cp: number | null
  pa: number | null
  pv: string | null
  ext: string | null
}

interface PageData {
  cnt: number
  ps: number
  pn: number
  list: KLineRow[]
}

interface KLineRowWithKey extends KLineRow {
  _key?: string
}

// 默认证券与默认日期范围（最近一个月，按库内最新交易日期回推）
const DEFAULT_USC = 'GCMain'
const usc = ref<string | null>(DEFAULT_USC)
const uscName = ref('')
const dateFrom = ref<string | null>(shiftMonthISO(todayISO(), -1))
const dateTo = ref<string | null>(todayISO())
const defaultRange = ref<{ from: string; to: string } | null>(null)
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const rows = ref<KLineRowWithKey[]>([])
const loading = ref(false)
const error = ref('')

const securityOptions = ref<SecurityOption[]>([])
const secuItems = ref<{ title: string; value: string }[]>([])

const headers = [
  { title: '证券', key: 'usc', width: 110, sortable: false },
  { title: '日期', key: 'date', width: 120, sortable: false },
  { title: '时间', key: 'time', width: 100, sortable: false },
  { title: '开盘', key: 'o', align: 'end' as const, sortable: false },
  { title: '最高', key: 'h', align: 'end' as const, sortable: false },
  { title: '最低', key: 'l', align: 'end' as const, sortable: false },
  { title: '收盘', key: 'c', align: 'end' as const, sortable: false },
  { title: '成交量', key: 'v', align: 'end' as const, sortable: false },
  { title: '成交额', key: 't', align: 'end' as const, sortable: false },
  { title: '涨跌额', key: 'cp', align: 'end' as const, sortable: false },
  { title: '涨跌幅(%)', key: 'cr', align: 'end' as const, sortable: false },
  { title: '来源', key: 'pv', width: 100, sortable: false },
  { title: '扩展字段', key: 'ext', sortable: false },
]

// 8 位日期 → yyyymmdd 数字；空值返回 null
function dateParam(value: string | null): string | null {
  if (!value) return null
  const compact = value.replace(/-/g, '')
  return /^\d{8}$/.test(compact) ? compact : null
}

// 数值展示（默认不使用千分位；空值显示 —）
function fmtValue(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '—'
  return String(Number(value))
}

// 涨跌幅（后端已按两位小数输出文本）
function fmtRatio(value: string | null): string {
  return value === null || value === undefined ? '—' : `${value}%`
}

function onUscInput(value: unknown) {
  const text = typeof value === 'string' ? value : ((value as { value?: string } | null)?.value ?? '')
  usc.value = text
  const hit = securityOptions.value.find((item) => item.usc === text)
  uscName.value = hit ? hit.security_name_cn : ''
}

async function load() {
  loading.value = true
  error.value = ''
  const from = dateParam(dateFrom.value)
  const to = dateParam(dateTo.value)
  if (from && to && from > to) {
    error.value = '开始日期不能晚于结束日期'
    loading.value = false
    return
  }
  const params = new URLSearchParams({
    limit: String(pageSize.value),
    offset: String((page.value - 1) * pageSize.value),
  })
  const uscText = (usc.value ?? '').trim()
  if (uscText) params.set('usc', uscText)
  if (from) params.set('dateFrom', from)
  if (to) params.set('dateTo', to)
  try {
    const data = await apiGet<PageData>(`/Meta/Finv/Quote/Cdeu/Min/Page?${params.toString()}`)
    rows.value = (data.list ?? []).map((row) => ({ ...row, _key: `${row.usc}-${row.ts}` }))
    total.value = data.cnt ?? 0
  } catch (e) {
    rows.value = []
    total.value = 0
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// 条件查询：页码回到 1；已在第 1 页时直接重新请求
function search() {
  if (page.value !== 1) {
    page.value = 1
  } else {
    load()
  }
}

// 默认日期范围：以库内最新交易日期为结束日、往前 1 个月；查询失败时回退到本机今天
async function resolveDefaultRange(): Promise<{ from: string; to: string }> {
  try {
    const bounds = await apiGet<{ date_to: number | null }>(
      `/Meta/Finv/Quote/Cdeu/Min/Bounds?usc=${DEFAULT_USC}`,
    )
    const end = bounds.date_to ? fmtDate(bounds.date_to) : todayISO()
    return { from: shiftMonthISO(end, -1), to: end }
  } catch {
    const end = todayISO()
    return { from: shiftMonthISO(end, -1), to: end }
  }
}

function reset() {
  usc.value = DEFAULT_USC
  onUscInput(DEFAULT_USC)
  const range = defaultRange.value ?? { from: shiftMonthISO(todayISO(), -1), to: todayISO() }
  dateFrom.value = range.from
  dateTo.value = range.to
  search()
}

// 分页变化自动请求（v-data-table-server 的 page / items-per-page 双向绑定）
watch([page, pageSize], () => load())

onMounted(async () => {
  try {
    const data = await apiGet<{ list: SecurityOption[] }>('/Meta/Finv/Quant/Metadata/Security/Options')
    securityOptions.value = data.list ?? []
    secuItems.value = securityOptions.value.map((item) => ({
      title: `${item.usc}:${item.security_name_cn}`,
      value: item.usc,
    }))
  } catch {
    // 字典加载失败不阻塞查询（仍可手动输入 usc）
    securityOptions.value = []
    secuItems.value = []
  }
  onUscInput(DEFAULT_USC)
  const range = await resolveDefaultRange()
  defaultRange.value = range
  dateFrom.value = range.from
  dateTo.value = range.to
  await load()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-table-search" class="mr-2" />
      去重分钟行情查询
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-3">
        <v-col cols="12" sm="6" md="3">
          <v-combobox
            v-model="usc"
            :items="secuItems"
            item-title="title"
            item-value="value"
            label="证券代码[USC]"
            placeholder="选择或输入，如 GCMain"
            density="compact"
            hide-details
            clearable
            @update:model-value="onUscInput"
          />
        </v-col>
        <v-col cols="6" sm="3" md="2">
          <v-text-field
            v-model="dateFrom"
            type="date"
            label="开始日期"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="6" sm="3" md="2">
          <v-text-field
            v-model="dateTo"
            type="date"
            label="结束日期"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" :loading="loading" @click="search">查询</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
        </v-col>
        <v-col cols="12" md="auto" class="ml-md-auto text-medium-emphasis text-caption">
          <span v-if="uscName">证券：{{ uscName }} · </span>
          共 <strong>{{ total }}</strong> 条
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-data-table-server
        v-model:page="page"
        v-model:items-per-page="pageSize"
        :headers="headers"
        :items="rows"
        :items-length="total"
        :loading="loading"
        item-value="_key"
        density="compact"
        class="text-no-wrap"
      >
        <template #item.usc="{ item }">{{ item.usc }}</template>
        <template #item.date="{ item }">{{ fmtDate(item.date) }}</template>
        <template #item.time="{ item }">{{ fmtTime(item.time) }}</template>
        <template #item.o="{ item }">{{ fmtValue(item.o) }}</template>
        <template #item.h="{ item }">{{ fmtValue(item.h) }}</template>
        <template #item.l="{ item }">{{ fmtValue(item.l) }}</template>
        <template #item.c="{ item }">{{ fmtValue(item.c) }}</template>
        <template #item.v="{ item }">{{ fmtValue(item.v) }}</template>
        <template #item.t="{ item }">{{ fmtValue(item.t) }}</template>
        <template #item.cp="{ item }">{{ fmtValue(item.cp) }}</template>
        <template #item.cr="{ item }">{{ fmtRatio(item.cr) }}</template>
        <template #item.pv="{ item }">{{ item.pv ?? '—' }}</template>
        <template #item.ext="{ item }">{{ item.ext ?? '—' }}</template>
        <template #no-data>
          <div class="py-6 text-medium-emphasis">当前条件下没有去重行情数据</div>
        </template>
      </v-data-table-server>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>查询 cdeu.finv_quote_secu_kline_min 去重后的分钟行情；支持按证券代码[USC]与交易日期范围过滤。</div>
          <div class="mt-1">数据来源：cdeu.finv_quote_secu_kline_min（按 usc + ts 去重后的分钟 K 线；列表不含 remark / last_close）。</div>
        </v-card-text>
      </v-card>
    </v-card-text>
  </v-card>
</template>
