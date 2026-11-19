<script setup lang="ts">
// 五日分钟行情采集配置（需求 11344）
//
// 数据源：Supabase `public.finv_quote_secu_ft`（本地库无副本），经服务端 PostgREST 只读代理。
// 本页**只读**：本表是采集链路的配置源，改动在 Supabase 侧进行，服务端不提供写接口。
//
// 接口（前缀 /Meta/Finv/Quote/Collect/FiveDayFt）：
//   GET  /Page           分页查询（默认每页 15 条）
//   GET  /FilterOptions  下拉候选（表内 DISTINCT）
//   GET  /Lookup?usc=    新增对话框的元数据补全（需求 11345：联查本地 4 张表）
//   POST /Create         新增一行采集配置（需求 11345，采集开关默认关闭）
//   POST /Update         编辑一行采集配置（需求 11346，只写改动列）
//
// 列元数据由服务端下发（21 列，含中文表头与类型），前端不再硬编码列名：
// 表结构以 PostgREST 逐列探测为准，列顺序按「标识 → 名称 → 分类 → 富途编码 → 开关/备注 → 时间」。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
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

// ---------- 新增采集证券（需求 11345） ----------
//
// 流程：输入 usc → Lookup（联查 futu.finv_futu_quote_collect / futu.finv_futu_secu_key /
// public.finv_security / cdeu.finv_quote_secu_indicator 补全 21 列）→ 人工核对/手填 → Create。
// 新增的 flag_enable 默认 0（先停采），提交后需人工改成 1 才真正开始采集。

interface LookupResult {
  usc: string
  fields: Record<string, unknown>
  sources: Record<string, string>
  missing: string[]
  notes: string[]
  in_security: boolean
  has_indicator: boolean
  exists_in_supabase: boolean
}

// 采集统计列：新增时不填（由采集链路自己维护），提交也不传
const COLLECT_STAT_COLUMNS = [
  'ts_latest_data',
  'count_last_fetch',
  'count_fail',
  'count_stale',
  'dt_last_check',
  'dt_last_fetch',
]

// 对话框模式：create（新增，含 usc 输入与补全）/ edit（编辑，usc 只读、直接给当前值）
const dialogMode = ref<'create' | 'edit'>('create')
// 编辑时的原始值（用于只提交改动列）
const editOrigin = ref<Record<string, string>>({})
const createOpen = ref(false)
const createUsc = ref('')
const lookupLoading = ref(false)
const lookupResult = ref<LookupResult | null>(null)
const draft = ref<Record<string, string>>({})
const submitting = ref(false)

/** 打开新增对话框（清空上次状态）。 */
function openCreate() {
  dialogMode.value = 'create'
  createUsc.value = ''
  lookupResult.value = null
  draft.value = {}
  editOrigin.value = {}
  createOpen.value = true
}

/** 打开编辑对话框：用当前行数据填充，usc 只读，采集统计列不参与。 */
function openEdit(row: Row) {
  const usc = String(row.usc ?? '').trim()
  if (!usc) return
  dialogMode.value = 'edit'
  createUsc.value = usc
  lookupResult.value = null
  const values: Record<string, string> = {}
  for (const column of editableColumns.value) {
    const raw = row[column.name]
    values[column.name] = raw === null || raw === undefined ? '' : String(raw)
  }
  draft.value = values
  editOrigin.value = { ...values }
  createOpen.value = true
}

/** 编辑模式下草稿与原始值相同的列不提交。 */
function changedFields(): Record<string, unknown> {
  const fields: Record<string, unknown> = {}
  for (const column of editableColumns.value) {
    const raw = (draft.value[column.name] ?? '').trim()
    if (raw === (editOrigin.value[column.name] ?? '').trim()) continue
    if (raw === '') {
      fields[column.name] = null // 清空 → 置 NULL
      continue
    }
    if (column.name === 'flag_enable') {
      fields[column.name] = raw === '1' ? 1 : 0
      continue
    }
    if (column.kind === 'number') {
      const number = Number(raw)
      if (Number.isNaN(number)) {
        throw new Error(`${column.title} 必须是数字`)
      }
      fields[column.name] = number
      continue
    }
    fields[column.name] = raw
  }
  return fields
}

async function submitUpdate() {
  const usc = createUsc.value.trim()
  if (!usc) return
  submitting.value = true
  try {
    const fields = changedFields()
    if (!Object.keys(fields).length) {
      showToast('没有修改任何字段', 'info')
      submitting.value = false
      return
    }
    await apiPost(`${BASE}/Update`, { usc, fields })
    showToast(`已保存 ${usc}（${Object.keys(fields).length} 个字段）`, 'success')
    createOpen.value = false
    await load()
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    submitting.value = false
  }
}

async function runLookup() {
  const usc = createUsc.value.trim()
  if (!usc) {
    showToast('请先输入 usc', 'warning')
    return
  }
  lookupLoading.value = true
  lookupResult.value = null
  try {
    const data = await apiGet<LookupResult>(`${BASE}/Lookup?usc=${encodeURIComponent(usc)}`)
    lookupResult.value = data
    const next: Record<string, string> = {}
    for (const column of columns.value) {
      if (column.kind === 'timestamp' || COLLECT_STAT_COLUMNS.includes(column.name)) continue
      const value = data.fields[column.name]
      next[column.name] = value === null || value === undefined ? '' : String(value)
    }
    // 采集开关固定按「先停采」预置（0），允许手动改
    next.flag_enable = String(data.fields.flag_enable ?? 0)
    draft.value = next
    if (!data.in_security) {
      showToast('该 usc 不在证券主数据里，不能新增', 'error')
    } else if (data.exists_in_supabase) {
      showToast('Supabase 里已存在该 usc，请勿重复新增', 'warning')
    }
  } catch (err) {
    showToast(`补全失败：${(err as Error).message}`, 'error')
  } finally {
    lookupLoading.value = false
  }
}

function isMissing(column: string): boolean {
  return lookupResult.value?.missing.includes(column) ?? false
}

function sourceText(column: string): string {
  return lookupResult.value?.sources[column] ?? ''
}

async function submitCreate() {
  const result = lookupResult.value
  if (!result || !result.in_security) return
  submitting.value = true
  try {
    const fields: Record<string, unknown> = {}
    for (const column of columns.value) {
      if (column.name === 'usc' || column.kind === 'timestamp') continue
      if (COLLECT_STAT_COLUMNS.includes(column.name)) continue
      const raw = (draft.value[column.name] ?? '').trim()
      if (raw === '') continue
      if (column.kind === 'number') {
        const number = Number(raw)
        if (Number.isNaN(number)) {
          showToast(`${column.title} 必须是数字`, 'error')
          submitting.value = false
          return
        }
        fields[column.name] = number
      } else if (column.name === 'flag_enable') {
        fields[column.name] = raw === '1' ? 1 : 0
      } else {
        fields[column.name] = raw
      }
    }
    await apiPost(`${BASE}/Create`, { usc: result.usc, fields })
    showToast(`已新增 ${result.usc}（采集开关为关闭，需手动开启）`, 'success')
    createOpen.value = false
    await loadOptions()
    await load()
  } catch (err) {
    showToast(`新增失败：${(err as Error).message}`, 'error')
  } finally {
    submitting.value = false
  }
}

const createDisabled = computed(() => {
  if (dialogMode.value === 'edit') return !createUsc.value.trim()
  const result = lookupResult.value
  return !result || !result.in_security || result.exists_in_supabase
})

const editableColumns = computed(() =>
  columns.value.filter((c) => c.kind !== 'timestamp' && !COLLECT_STAT_COLUMNS.includes(c.name)),
)

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
        <v-btn
          color="primary"
          variant="tonal"
          prepend-icon="mdi-plus"
          data-testid="fiveday-ft-create"
          @click="openCreate"
        >
          新增
        </v-btn>
        <v-spacer />
        <span class="text-caption text-medium-emphasis">
          第 {{ page }} / {{ totalPages }} 页 · 每页 {{ pageSize }} 条
        </span>
      </div>

      <!-- 数据表：服务端分页 → hide-default-footer + items-per-page=-1 -->
      <v-data-table
        :headers="[
          ...columns.map((c) => ({ title: c.title, key: c.name, sortable: false })),
          { title: '操作', key: 'actions', sortable: false, width: 76 },
        ]"
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
        <template #[`item.actions`]="{ item }">
          <v-btn
            size="small"
            variant="text"
            color="primary"
            prepend-icon="mdi-pencil-outline"
            density="comfortable"
            @click.stop="openEdit(item)"
          >
            编辑
          </v-btn>
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

  <!-- 新增采集证券对话框（需求 11345） -->
  <v-dialog v-model="createOpen" fullscreen scrollable persistent>
    <v-card>
      <v-card-title class="text-subtitle-1 d-flex align-center">
        <v-icon :icon="dialogMode === 'edit' ? 'mdi-pencil-outline' : 'mdi-plus-box-outline'" class="mr-2" />
        {{ dialogMode === 'edit' ? `编辑采集配置 · ${createUsc}` : '新增采集证券' }}
        <v-spacer />
        <v-btn variant="text" icon="mdi-close" :disabled="submitting" @click="createOpen = false" />
      </v-card-title>
      <v-divider />
      <v-card-text>
        <v-alert v-if="dialogMode === 'edit'" type="info" density="compact" variant="tonal" class="mb-3">
          只提交改动过的列；<code>usc</code>、时间列、采集统计列不可修改。
          采集开关改成「启用[1]」后该证券即进入采集范围，请确认。
        </v-alert>
        <v-alert v-else type="info" density="compact" variant="tonal" class="mb-3">
          输入 <code>usc</code> 后自动联查 <code>futu.finv_futu_quote_collect</code> /
          <code>futu.finv_futu_secu_key</code> / <code>public.finv_security</code> /
          <code>cdeu.finv_quote_secu_indicator</code> 补全元数据；补不出来的列标黄，需手工填写。
          <strong>新增后采集开关默认关闭</strong>，确认无误后再手动改成「启用」。
          采集统计列（最新数据时间戳、累计采集次数等）不在此处填写，由采集链路维护。
        </v-alert>

        <v-row v-if="dialogMode === 'create'" dense class="mb-3">
          <v-col cols="12" md="4">
            <v-text-field
              v-model="createUsc"
              label="usc（统一编号）*"
              density="compact"
              variant="outlined"
              hide-details
              placeholder="如 00700 / 301293 / BKX"
              :disabled="lookupLoading || submitting"
              @keyup.enter="runLookup"
            />
          </v-col>
          <v-col cols="12" md="8" class="d-flex align-center">
            <v-btn
              color="primary"
              variant="flat"
              prepend-icon="mdi-database-search"
              :loading="lookupLoading"
              :disabled="submitting"
              @click="runLookup"
            >
              补全元数据
            </v-btn>
          </v-col>
        </v-row>

        <template v-if="dialogMode === 'edit'">
          <v-table density="compact" class="finv-ft-table finv-create-table">
            <colgroup>
              <col style="width: 190px" />
              <col />
              <col style="width: 120px" />
            </colgroup>
            <thead>
              <tr>
                <th>列</th>
                <th>值</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="column in editableColumns.filter((c) => c.name !== 'usc')" :key="column.name">
                <td class="text-no-wrap">
                  <div>{{ column.title }}</div>
                  <div class="text-caption text-medium-emphasis">{{ column.name }}</div>
                </td>
                <td>
                  <v-select
                    v-if="column.name === 'flag_enable'"
                    v-model="draft[column.name]"
                    :items="[
                      { title: '停用[0]', value: '0' },
                      { title: '启用[1]', value: '1' },
                    ]"
                    density="compact"
                    variant="outlined"
                    hide-details
                  />
                  <v-text-field
                    v-else
                    v-model="draft[column.name]"
                    :type="column.kind === 'number' ? 'number' : 'text'"
                    :bg-color="
                      (draft[column.name] ?? '') !== (editOrigin[column.name] ?? '')
                        ? 'amber-lighten-5'
                        : undefined
                    "
                    density="compact"
                    variant="outlined"
                    hide-details
                  />
                </td>
                <td class="text-caption text-no-wrap">
                  <v-chip
                    v-if="(draft[column.name] ?? '') !== (editOrigin[column.name] ?? '')"
                    size="x-small"
                    variant="tonal"
                    color="warning"
                  >
                    已改
                  </v-chip>
                  <span v-else class="text-medium-emphasis">未改</span>
                </td>
              </tr>
              <tr>
                <td class="text-no-wrap">
                  只读列
                  <div class="text-caption text-medium-emphasis">usc / 时间列 / 采集统计列</div>
                </td>
                <td colspan="2" class="text-caption text-medium-emphasis">
                  usc 为主键不可改；时间列由服务端维护；采集统计列由采集链路维护
                </td>
              </tr>
            </tbody>
          </v-table>
        </template>

        <template v-if="dialogMode === 'create' && lookupResult">
          <v-alert
            v-for="note in lookupResult.notes"
            :key="note"
            :type="lookupResult.exists_in_supabase ? 'warning' : 'info'"
            density="compact"
            variant="tonal"
            class="mb-2"
          >
            {{ note }}
          </v-alert>
          <v-alert
            v-if="!lookupResult.in_security"
            type="error"
            density="compact"
            variant="tonal"
            class="mb-2"
          >
            该 usc 不在本地证券主数据里，无法新增。请先在「元数据管理 → 证券字典」登记。
          </v-alert>
          <v-alert
            v-if="lookupResult.missing.length"
            type="warning"
            density="compact"
            variant="tonal"
            class="mb-2"
          >
            有 {{ lookupResult.missing.length }} 列未能自动补全（标黄），请核对后手填：
            {{ lookupResult.missing.map((n) => columns.find((c) => c.name === n)?.title ?? n).join('、') }}
          </v-alert>

          <v-table density="compact" class="finv-ft-table finv-create-table">
            <colgroup>
              <col style="width: 190px" />
              <col />
              <col style="width: 120px" />
            </colgroup>
            <thead>
              <tr>
                <th>列</th>
                <th>值</th>
                <th>来源</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="column in editableColumns" :key="column.name">
                <td class="text-no-wrap">
                  <div>{{ column.title }}</div>
                  <div class="text-caption text-medium-emphasis">{{ column.name }}</div>
                </td>
                <td>
                  <v-select
                    v-if="column.name === 'flag_enable'"
                    v-model="draft.flag_enable"
                    :items="[
                      { title: '停用[0]（默认，先停采）', value: '0' },
                      { title: '启用[1]', value: '1' },
                    ]"
                    density="compact"
                    variant="outlined"
                    hide-details
                  />
                  <v-text-field
                    v-else
                    v-model="draft[column.name]"
                    :type="column.kind === 'number' ? 'number' : 'text'"
                    :bg-color="isMissing(column.name) ? 'amber-lighten-5' : undefined"
                    :placeholder="isMissing(column.name) ? '补全不到，请手填' : '—'"
                    density="compact"
                    variant="outlined"
                    hide-details
                  />
                </td>
                <td class="text-caption text-medium-emphasis text-no-wrap">
                  <span>{{ sourceText(column.name) || '—' }}</span>
                  <v-chip
                    v-if="isMissing(column.name)"
                    size="x-small"
                    variant="tonal"
                    color="warning"
                    class="ml-1"
                  >
                    缺失
                  </v-chip>
                </td>
              </tr>
              <tr>
                <td class="text-no-wrap">
                  时间列
                  <div class="text-caption text-medium-emphasis">gmt_create / gmt_update / dt_update</div>
                </td>
                <td colspan="2" class="text-caption text-medium-emphasis">
                  由服务端写入当前时间（now()），不在此处填写
                </td>
              </tr>
            </tbody>
          </v-table>
        </template>
      </v-card-text>
      <v-divider />
      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">
          {{ dialogMode === 'edit' ? '只提交改动过的列' : '新增后采集开关为「停用」，需人工改成「启用」' }}
        </span>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="createOpen = false">取消</v-btn>
        <v-btn
          color="primary"
          variant="flat"
          prepend-icon="mdi-content-save-plus-outline"
          :loading="submitting"
          :disabled="createDisabled"
          @click="dialogMode === 'edit' ? submitUpdate() : submitCreate()"
        >
          {{ dialogMode === 'edit' ? '保存' : '新增' }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
/* 新增对话框：值列撑满，来源列固定窄宽 */
.finv-create-table :deep(table) {
  table-layout: fixed;
  width: 100%;
}

.finv-create-table :deep(td),
.finv-create-table :deep(th) {
  padding: 4px 8px;
  vertical-align: middle;
}

.finv-create-table :deep(.v-input) {
  width: 100%;
}

.finv-create-table :deep(.v-field__input) {
  min-height: 36px;
}

/* 列宽不压缩 + 横向滚动（与证券字典口径一致） */
.finv-ft-table :deep(table) {
  table-layout: auto;
}

.finv-ft-table :deep(td),
.finv-ft-table :deep(th) {
  white-space: nowrap;
}
</style>
