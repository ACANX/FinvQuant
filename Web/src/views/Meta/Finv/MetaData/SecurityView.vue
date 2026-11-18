<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

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
const pageSize = ref(20)
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
  type_secu_subs: string[]
  currencies: string[]
  providers: string[]
}
const filterOptions = ref<SecurityFilterOptions>({
  types: [],
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

// 新增/编辑对话框
const dialog = ref(false)
const editing = ref(false)
const form = ref<SecurityRow>({
  usc: '',
  exchange_code: 0,
  market_code: 0,
  security_type: '',
  security_code: '',
  security_name: '',
  security_name_cn: '',
  security_name_full: null,
  currency_type: '',
  init_date: 20000000,
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
  form.value = {
    usc: '',
    exchange_code: 0,
    market_code: 0,
    security_type: '',
    security_code: '',
    security_name: '',
    security_name_cn: '',
    security_name_full: null,
    currency_type: '',
    init_date: 20000000,
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
  dialog.value = true
}

async function save() {
  if (!form.value.usc.trim()) {
    showToast('统一证券代码（usc）不能为空', 'error')
    return
  }
  if (!form.value.exchange_code || form.value.exchange_code <= 0) {
    showToast('交易所代码必须为正整数', 'error')
    return
  }
  if (!form.value.security_code.trim() || !form.value.security_name_cn.trim()) {
    showToast('源证券代码与中文名称不能为空', 'error')
    return
  }
  try {
    await apiPost('/Meta/Finv/Quant/Metadata/Security/Save', {
      usc: form.value.usc.trim(),
      exchange_code: form.value.exchange_code,
      market_code: form.value.market_code ?? 0,
      security_type: form.value.security_type.trim(),
      security_code: form.value.security_code.trim(),
      security_name: form.value.security_name.trim(),
      security_name_cn: form.value.security_name_cn.trim(),
      security_name_full: form.value.security_name_full?.trim() || null,
      currency_type: form.value.currency_type.trim(),
      init_date: form.value.init_date,
      timezone: form.value.timezone?.trim() || null,
      tz: form.value.tz?.trim() || null,
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

function search() {
  page.value = 1
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
      type_secu_subs: data.type_secu_subs ?? [],
      currencies: data.currencies ?? [],
      providers: data.providers ?? [],
    }
  } catch {
    filterOptions.value = { types: [], type_secu_subs: [], currencies: [], providers: [] }
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
        :items-per-page="pageSize"
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

      <v-pagination
        v-model="page"
        :length="Math.max(1, Math.ceil(total / pageSize))"
        :total-visible="7"
        density="compact"
        class="mt-3"
        @update:model-value="load"
      />
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
              <v-select
                v-model="form.exchange_code"
                label="交易所代码（基于 finv_exchange 下拉）"
                :items="exchangeItems"
                clearable
                density="compact"
                hint="仅展示启用状态的交易所"
              />
            </v-col>
            <v-col cols="6">
              <v-select
                v-model="form.market_code"
                label="市场代码（基于 finv_market 下拉，缺省 0）"
                :items="marketItems"
                clearable
                density="compact"
                hint="仅展示启用状态的市场"
              />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_type" label="证券类型（如 Stock / ETF / Futures）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.currency_type" label="计价货币（对齐 finv_currency）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model.number="form.init_date" label="上市日期（yyyymmdd）" type="number" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.timezone" label="时区偏移（如 -04:00）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.tz" label="时区标识（如 America/New_York）" density="compact" />
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
