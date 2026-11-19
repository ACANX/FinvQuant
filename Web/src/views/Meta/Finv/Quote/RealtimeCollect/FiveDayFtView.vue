<script setup lang="ts">
// 五日分钟行情采集配置（需求 11344）
//
// 数据源：Supabase `public.finv_quote_secu_ft`（本地库无副本），经服务端 PostgREST 只读代理。
// 本页**只读**：本表是采集链路的配置源，改动在 Supabase 侧进行，服务端不提供写接口。
//
// 接口（前缀 /Meta/Finv/Quote/Collect/FiveDayFt）：
//   GET /Page           分页查询（默认每页 15 条）
//   GET /FilterOptions  下拉候选（表内 DISTINCT）
//
// 列元数据由服务端下发（21 列，含中文表头与类型），前端不再硬编码列名：
// 表结构以 PostgREST 逐列探测为准，列顺序按「标识 → 名称 → 分类 → 富途编码 → 开关/备注 → 时间」。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

const BASE = '/Meta/Finv/Quote/Collect/FiveDayFt'

// 每页条数选项：15 为默认（需求 11344 指定）
const PAGE_SIZE_OPTIONS = [15, 20, 50, 100, 200]

interface ColumnMeta {
  name: string
  title: string
  kind: 'text' | 'number' | 'timestamp'
  sortable: boolean
  note?: string
}

interface Row {
  [key: string]: unknown
}

const columns = ref<ColumnMeta[]>([])
const rows = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const loading = ref(false)
const loadError = ref('')
const elapsedMs = ref<number | null>(null)

// 筛选条件
const keyword = ref('')
const marketLabel = ref('')
const instrument = ref('')
const typeSecu = ref('')
const provider = ref('')
const flagEnable = ref('')

// 排序
const sortBy = ref('usc')
const sortOrder = ref<'asc' | 'desc'>('asc')

const options = ref<Record<string, string[]>>({})

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// 字典口径（与「采集参数」页保持一致）：富途品种 → 中文；采集开关 → 启用/禁用
const INSTRUMENT_LABELS: Record<string, string> = {
  stock: '股票',
  etfs: 'ETF',
  etf: 'ETF',
  index: '指数',
  currency: '外汇',
  crypto: '加密货币',
  futures: '期货',
  sectors: '板块',
}

/** 下拉选项统一写成 `中文[值]`（DictFilterSpec）。 */
function optionEntries(param: string): { title: string; value: string }[] {
  const values = options.value[param] ?? []
  return values.map((value) => {
    if (param === 'instrument') {
      return { title: `${INSTRUMENT_LABELS[value] ?? value}[${value}]`, value }
    }
    return { title: `${value}[${value}]`, value }
  })
}

async function loadOptions() {
  try {
    const data = await apiGet<{ options: Record<string, string[]> }>(`${BASE}/FilterOptions`)
    options.value = data.options ?? {}
  } catch (err) {
    // 下拉候选拿不到不阻断主表查询（表为空时本来就是空数组）
    showToast(`读取下拉候选失败：${(err as Error).message}`, 'warning')
  }
}

async function load(silent = false) {
  loading.value = true
  loadError.value = ''
  try {
    const query = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      sortBy: sortBy.value,
      sortOrder: sortOrder.value,
    })
    const filters: [string, string][] = [
      ['keyword', keyword.value],
      ['marketLabel', marketLabel.value],
      ['instrument', instrument.value],
      ['typeSecu', typeSecu.value],
      ['provider', provider.value],
      ['flagEnable', flagEnable.value],
    ]
    for (const [key, value] of filters) {
      if (value) query.set(key, value)
    }
    const data = await apiGet<{
      list: Row[]
      total: number
      page: number
      pageSize: number
      columns: ColumnMeta[]
      elapsed_ms: number
    }>(`${BASE}/Page?${query.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
    columns.value = data.columns ?? []
    elapsedMs.value = data.elapsed_ms ?? null
    // 越界页自动裁剪（总数为 0 或筛选后页数变少）
    if (rows.value.length === 0 && total.value > 0 && page.value > totalPages.value) {
      page.value = totalPages.value
      await load(silent)
    }
  } catch (err) {
    loadError.value = (err as Error).message
    if (!silent) showToast(`查询失败：${(err as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  marketLabel.value = ''
  instrument.value = ''
  typeSecu.value = ''
  provider.value = ''
  flagEnable.value = ''
  sortBy.value = 'usc'
  sortOrder.value = 'asc'
  page.value = 1
  load()
}

function changePageSize(size: number) {
  pageSize.value = size
  page.value = 1
  load()
}

function goPage(target: number) {
  const next = Math.min(Math.max(1, target), totalPages.value)
  if (next === page.value) return
  page.value = next
  load()
}

function toggleSort(column: ColumnMeta) {
  if (!column.sortable) return
  if (sortBy.value === column.name) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = column.name
    sortOrder.value = 'asc'
  }
  page.value = 1
  load()
}

/** 单元格展示：null/空 → `—`；开关/品种按字典口径显示中文。 */
function cellText(column: ColumnMeta, row: Row): string {
  const raw = row[column.name]
  if (raw === null || raw === undefined || raw === '') return '—'
  if (column.name === 'flag_enable') return String(raw) === '1' ? '启用' : '禁用'
  if (column.name === 'instrument') return INSTRUMENT_LABELS[String(raw)] ?? String(raw)
  if (column.kind === 'timestamp') return formatTime(String(raw))
  return String(raw)
}

function cellClass(column: ColumnMeta): string {
  return column.kind === 'number' ? 'text-right text-no-wrap' : 'text-no-wrap'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

/** 启用状态配色（UiSpec）：启用绿色、禁用灰色。 */
function flagColor(row: Row): string | null {
  const raw = row.flag_enable
  if (raw === null || raw === undefined || raw === '') return null
  return String(raw) === '1' ? 'success' : 'grey'
}

onMounted(() => {
  loadOptions()
  load()
})
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center flex-wrap ga-2">
      <v-icon icon="mdi-calendar-clock" class="mr-2" />
      五日分钟行情采集配置
      <v-chip size="small" variant="tonal" prepend-icon="mdi-database-outline">
        共 {{ total }} 条
      </v-chip>
      <v-chip v-if="elapsedMs !== null" size="small" variant="tonal" prepend-icon="mdi-timer-outline">
        上游 {{ elapsedMs }} ms
      </v-chip>
      <v-spacer />
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load()">刷新</v-btn>
    </v-card-title>

    <v-card-text>
      <v-alert type="info" density="compact" variant="tonal" class="mb-4">
        数据源：Supabase <code>public.finv_quote_secu_ft</code>（本地库无副本）。
        本页<strong>只读</strong>，采集配置的改动在 Supabase 侧进行。
      </v-alert>

      <v-alert v-if="loadError" type="error" density="compact" variant="tonal" class="mb-3">
        {{ loadError }}
      </v-alert>

      <!-- 筛选区 -->
      <v-row dense class="mb-2">
        <v-col cols="12" md="3">
          <v-text-field
            v-model="keyword"
            label="关键字"
            density="compact"
            variant="outlined"
            hide-details
            clearable
            prepend-inner-icon="mdi-magnify"
            placeholder="统一编号 / 代码 / 富途代码 / 名称"
            @keyup.enter="search"
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-select
            v-model="marketLabel"
            :items="[{ title: '全部市场', value: '' }, ...optionEntries('marketLabel')]"
            label="行情市场"
            density="compact"
            variant="outlined"
            hide-details
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-select
            v-model="instrument"
            :items="[{ title: '全部品种', value: '' }, ...optionEntries('instrument')]"
            label="品种"
            density="compact"
            variant="outlined"
            hide-details
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-select
            v-model="typeSecu"
            :items="[{ title: '全部分类', value: '' }, ...optionEntries('typeSecu')]"
            label="证券分类"
            density="compact"
            variant="outlined"
            hide-details
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-select
            v-model="provider"
            :items="[{ title: '全部提供方', value: '' }, ...optionEntries('provider')]"
            label="提供方"
            density="compact"
            variant="outlined"
            hide-details
          />
        </v-col>
        <v-col cols="12" md="1">
          <v-select
            v-model="flagEnable"
            :items="[
              { title: '全部', value: '' },
              { title: '启用[1]', value: '1' },
              { title: '禁用[0]', value: '0' },
            ]"
            label="采集开关"
            density="compact"
            variant="outlined"
            hide-details
          />
        </v-col>
      </v-row>
      <div class="d-flex align-center ga-2 mb-3">
        <v-btn color="primary" variant="flat" prepend-icon="mdi-magnify" :loading="loading" @click="search">
          查询
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-backspace-outline" @click="reset">重置</v-btn>
        <v-spacer />
        <span class="text-caption text-medium-emphasis">
          第 {{ page }} / {{ totalPages }} 页 · 每页 {{ pageSize }} 条
        </span>
      </div>

      <!-- 数据表：服务端分页 → hide-default-footer + items-per-page=-1 -->
      <v-data-table
        :headers="columns.map((c) => ({ title: c.title, key: c.name, sortable: false }))"
        :items="rows"
        :loading="loading"
        hide-default-footer
        :items-per-page="-1"
        density="compact"
        class="finv-ft-table"
      >
        <template #[`item.flag_enable`]="{ item }">
          <v-chip v-if="flagColor(item)" size="x-small" variant="tonal" :color="flagColor(item) || undefined">
            {{ cellText({ name: 'flag_enable', title: '', kind: 'number', sortable: true }, item) }}
          </v-chip>
          <span v-else>—</span>
        </template>
        <template
          v-for="column in columns.filter((c) => c.name !== 'flag_enable')"
          #[`item.${column.name}`]="{ item }"
          :key="column.name"
        >
          <span :class="cellClass(column)" :title="cellText(column, item)">
            {{ cellText(column, item) }}
          </span>
        </template>
        <template #no-data>
          <div class="text-medium-emphasis py-6 text-center">
            无数据（该表当前为空，或筛选条件未命中）
          </div>
        </template>
      </v-data-table>

      <!-- 表头排序控件（独立于 v-data-table 内置排序，避免与服务端排序冲突） -->
      <div class="d-flex align-center flex-wrap ga-2 mt-2">
        <span class="text-caption text-medium-emphasis">排序：</span>
        <v-select
          v-model="sortBy"
          :items="columns.filter((c) => c.sortable).map((c) => ({ title: c.title, value: c.name }))"
          density="compact"
          variant="outlined"
          hide-details
          style="max-width: 180px"
          @update:model-value="search"
        />
        <v-btn
          size="small"
          variant="tonal"
          :prepend-icon="sortOrder === 'asc' ? 'mdi-sort-ascending' : 'mdi-sort-descending'"
          @click="toggleSort({ name: sortBy, title: '', kind: 'text', sortable: true })"
        >
          {{ sortOrder === 'asc' ? '升序' : '降序' }}
        </v-btn>
        <span v-if="columns.find((c) => c.name === sortBy)?.note" class="text-caption text-medium-emphasis">
          {{ columns.find((c) => c.name === sortBy)?.note }}
        </span>
      </div>

      <!-- 分页（唯一一处） -->
      <div class="d-flex align-center flex-wrap ga-2 mt-3">
        <span class="text-caption text-medium-emphasis">
          共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页
        </span>
        <v-spacer />
        <v-select
          :model-value="pageSize"
          :items="PAGE_SIZE_OPTIONS.map((n) => ({ title: `${n} 条/页`, value: n }))"
          density="compact"
          variant="outlined"
          hide-details
          style="max-width: 130px"
          @update:model-value="changePageSize"
        />
        <v-btn size="small" variant="text" icon="mdi-page-first" :disabled="page <= 1" @click="goPage(1)" />
        <v-btn size="small" variant="text" icon="mdi-chevron-left" :disabled="page <= 1" @click="goPage(page - 1)" />
        <span class="text-caption">第 {{ page }} 页 / 共 {{ totalPages }} 页</span>
        <v-btn
          size="small"
          variant="text"
          icon="mdi-chevron-right"
          :disabled="page >= totalPages"
          @click="goPage(page + 1)"
        />
        <v-btn
          size="small"
          variant="text"
          icon="mdi-page-last"
          :disabled="page >= totalPages"
          @click="goPage(totalPages)"
        />
        <v-text-field
          :model-value="page"
          type="number"
          density="compact"
          variant="outlined"
          hide-details
          style="max-width: 100px"
          label="跳页"
          @keyup.enter="goPage(Number(($event.target as HTMLInputElement).value))"
        />
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
/* 列宽不压缩 + 横向滚动（与证券字典口径一致） */
.finv-ft-table :deep(table) {
  table-layout: auto;
}

.finv-ft-table :deep(td),
.finv-ft-table :deep(th) {
  white-space: nowrap;
}
</style>
