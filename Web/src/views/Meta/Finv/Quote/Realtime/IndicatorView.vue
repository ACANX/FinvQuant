<script setup lang="ts">
// 证券指标（需求 11251）
// 数据表：cdeu.finv_quote_secu_indicator（132 列，一行一证券的最新指标快照）
// 接口：GET /Meta/Finv/Quote/Secu/Indicator/Columns（列元数据）
//      GET /Meta/Finv/Quote/Secu/Indicator/Page?keyword=&market=&columns=&orderBy=&order=&page=&pageSize=
//
// 132 列不可能一次铺满屏幕，本页按三层组织：
//   ① 主表 —— 只渲染当前「列方案」选中的列（默认概览 17 列），左侧三列冻结、其余横向滚动；
//   ② 列选择抽屉 —— 按业务分组勾选 / 搜索列 / 预设方案（概览、行情、估值与股息、基金、期货与标的、时间与来源、全部），
//      选择结果存 localStorage；
//   ③ 行详情对话框 —— 点行后按分组展开该证券的全部 132 列（另行请求全列，不受当前列方案影响）。
//
// 数值一律由后端以文本返回（保住 NUMERIC(30,10) 精度），本页只做万/亿折算与小数位裁剪。
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../api'
import TextPopover from '../../../../../components/TextPopover.vue'
import {
  DEFAULT_PRESET_KEY,
  INDICATOR_GROUPS,
  INDICATOR_KEY_COLUMNS,
  INDICATOR_PRESETS,
  formatIndicatorCell,
  indicatorColumnLabel,
  indicatorColumnSpec,
  indicatorGroupOf,
  isIndicatorLongText,
  isIndicatorNumericColumn,
} from '../../../../../config/indicatorColumns'
import type { IndicatorColumn } from '../../../../../config/indicatorColumns'
import { showToast } from '../../../../../ui/toast'
import IndicatorColumnPicker from './IndicatorColumnPicker.vue'

const COLUMNS_API = '/Meta/Finv/Quote/Secu/Indicator/Columns'
const PAGE_API = '/Meta/Finv/Quote/Secu/Indicator/Page'

const STORAGE_COLUMNS = 'finvquant.indicator.columns'
const STORAGE_PRESET = 'finvquant.indicator.preset'
const STORAGE_PAGE_SIZE = 'finvquant.indicator.pageSize'

/** 指标行：键为列名，值为文本（空串 = 库里 NULL） */
type IndicatorRow = Record<string, string>

const columns = ref<IndicatorColumn[]>([])
const selected = ref<string[]>([])
const preset = ref(DEFAULT_PRESET_KEY)

const rows = ref<IndicatorRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const market = ref('')
const orderBy = ref('dt_update')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const columnsLoading = ref(false)
const error = ref('')

const pickerOpen = ref(false)

// 行详情：全列另行请求（不受当前列方案影响）
const detailOpen = ref(false)
const detailLoading = ref(false)
const detailRow = ref<IndicatorRow | null>(null)
const detailColumns = ref<IndicatorColumn[]>([])

const MARKET_OPTIONS = [
  { title: '全部市场', value: '' },
  { title: 'HK', value: 'HK' },
  { title: 'SH', value: 'SH' },
  { title: 'SZ', value: 'SZ' },
  { title: 'BJ', value: 'BJ' },
  { title: 'US', value: 'US' },
  { title: 'CC', value: 'CC' },
  { title: 'FX', value: 'FX' },
]

/** 列名 → 列元数据 */
const columnMap = computed(() => {
  const map = new Map<string, IndicatorColumn>()
  for (const column of columns.value) map.set(column.name, column)
  return map
})

/** 当前生效的列（补身份列、丢掉后端已不存在的列） */
const visibleColumns = computed<IndicatorColumn[]>(() => {
  const result: IndicatorColumn[] = []
  const seen = new Set<string>()
  for (const name of [...INDICATOR_KEY_COLUMNS, ...selected.value]) {
    const column = columnMap.value.get(name)
    if (!column || seen.has(name)) continue
    seen.add(name)
    result.push(column)
  }
  return result
})

/**
 * 排序候选（与后端白名单同源：后端只认表里真实存在的列）。
 *
 * 选项文字只用中文短名：表列注释是整句口径说明、列名是 snake_case，
 * 两个都不该出现在下拉选项里（列名与完整口径看表头悬浮 / 「选择列」 / 行详情 / 说明卡片）。
 */
const orderOptions = computed(() =>
  columns.value
    .filter((column) => column.sortable)
    .map((column) => ({ title: indicatorColumnLabel(column), value: column.name })),
)

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

/** 当前列方案名（用户手改后显示「自定义」） */
const presetTitle = computed(() => {
  const hit = INDICATOR_PRESETS.find((item) => item.key === preset.value)
  return hit ? hit.title : '自定义'
})

// ---------- 列元数据 ----------

async function loadColumns(): Promise<void> {
  columnsLoading.value = true
  try {
    const data = await apiGet<{ table: string; total: number; columns: IndicatorColumn[] }>(COLUMNS_API)
    columns.value = data.columns ?? []
    restoreSelection()
  } catch (err) {
    error.value = `列元数据加载失败：${(err as Error).message}`
    columns.value = []
  } finally {
    columnsLoading.value = false
  }
}

/** 恢复上次的列方案；没有记录或已失效则用默认「概览」 */
function restoreSelection(): void {
  const known = new Set(columns.value.map((column) => column.name))
  const storedPreset = localStorage.getItem(STORAGE_PRESET) ?? ''
  const storedColumns = (localStorage.getItem(STORAGE_COLUMNS) ?? '')
    .split(',')
    .map((name) => name.trim())
    .filter((name) => name && known.has(name))

  if (storedColumns.length > 0) {
    selected.value = dedupeWithKeys(storedColumns)
    preset.value = INDICATOR_PRESETS.some((item) => item.key === storedPreset) ? storedPreset : ''
    return
  }
  applyPreset(INDICATOR_PRESETS.some((item) => item.key === storedPreset) ? storedPreset : DEFAULT_PRESET_KEY)
}

function dedupeWithKeys(names: string[]): string[] {
  const known = new Set(columns.value.map((column) => column.name))
  const seen = new Set<string>()
  const result: string[] = []
  for (const name of [...INDICATOR_KEY_COLUMNS, ...names]) {
    if (!known.has(name) || seen.has(name)) continue
    seen.add(name)
    result.push(name)
  }
  return result
}

function applyPreset(key: string): void {
  const target = INDICATOR_PRESETS.find((item) => item.key === key) ?? INDICATOR_PRESETS[0]
  preset.value = target.key
  selected.value = dedupeWithKeys(target.columns)
}

function onSelectedChange(next: string[]): void {
  selected.value = next
  localStorage.setItem(STORAGE_COLUMNS, next.join(','))
  localStorage.setItem(STORAGE_PRESET, preset.value)
  void load()
}

function onPresetChange(next: string): void {
  preset.value = next
  localStorage.setItem(STORAGE_PRESET, next)
}

// ---------- 列表 ----------

async function load(): Promise<void> {
  if (!columns.value.length) return
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      // clearable 下拉被清空时 v-model 可能是 null，回退默认排序
      orderBy: orderBy.value || 'dt_update',
      order: order.value === 'asc' ? 'asc' : 'desc',
      columns: visibleColumns.value.map((column) => column.name).join(','),
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (market.value) params.set('market', market.value)
    const data = await apiGet<{ total: number; page: number; pageSize: number; list: IndicatorRow[] }>(
      `${PAGE_API}?${params.toString()}`,
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

function search(): void {
  page.value = 1
  void load()
}

function reset(): void {
  keyword.value = ''
  market.value = ''
  orderBy.value = 'dt_update'
  order.value = 'desc'
  page.value = 1
  pageSize.value = 50
  localStorage.setItem(STORAGE_PAGE_SIZE, '50')
  applyPreset(DEFAULT_PRESET_KEY)
  localStorage.setItem(STORAGE_COLUMNS, selected.value.join(','))
  localStorage.setItem(STORAGE_PRESET, preset.value)
  void load()
}

function onPageSizeChange(): void {
  localStorage.setItem(STORAGE_PAGE_SIZE, String(pageSize.value))
  search()
}

// ---------- 行详情 ----------

async function openDetail(row: IndicatorRow): Promise<void> {
  detailRow.value = row
  detailOpen.value = true
  detailLoading.value = true
  detailColumns.value = columns.value
  try {
    // 全列请求：按 usc 精确回找当前行（keyword 是模糊匹配，取 usc 相同的第一条）
    const params = new URLSearchParams({
      page: '1',
      pageSize: '20',
      orderBy: 'usc',
      order: 'asc',
      columns: columns.value.map((column) => column.name).join(','),
    })
    if (row.usc) params.set('keyword', row.usc)
    const data = await apiGet<{ list: IndicatorRow[] }>(`${PAGE_API}?${params.toString()}`)
    const exact = (data.list ?? []).find((item) => item.usc === row.usc)
    detailRow.value = exact ?? row
  } catch (err) {
    showToast(`详情加载失败，已展示当前列：${(err as Error).message}`, 'warning')
  } finally {
    detailLoading.value = false
  }
}

/** 详情分组：按静态分组的顺序把全列切开（未登记的列归「其它」） */
const detailGroups = computed(() => {
  const known = detailColumns.value
  const buckets = new Map<string, IndicatorColumn[]>()
  for (const column of known) {
    const key = indicatorGroupOf(column.name)
    const list = buckets.get(key) ?? []
    list.push(column)
    buckets.set(key, list)
  }
  return INDICATOR_GROUPS.filter((group) => (buckets.get(group.key)?.length ?? 0) > 0).map(
    (group) => ({ key: group.key, title: group.title, columns: buckets.get(group.key) ?? [] }),
  )
})

async function copyDetailJson(): Promise<void> {
  if (!detailRow.value) return
  try {
    await navigator.clipboard.writeText(JSON.stringify(detailRow.value, null, 2))
    showToast('已复制整行 JSON', 'success')
  } catch {
    showToast('复制失败：浏览器不允许，请手动选中复制', 'warning')
  }
}

// ---------- 渲染辅助 ----------

function cellText(column: IndicatorColumn, row: IndicatorRow): string {
  return formatIndicatorCell(column, row[column.name] ?? '')
}

/** 涨跌着色：只给涨跌幅 / 涨跌额上色（132 列整行着色会读不清） */
function cellClass(column: IndicatorColumn): string {
  const spec = indicatorColumnSpec(column.name)
  const classes: string[] = []
  if (isIndicatorNumericColumn(column.name)) classes.push('text-right', 'num')
  if (spec && (spec.name === 'change_ratio' || spec.name === 'change_price')) classes.push('finv-indicator-move')
  return classes.join(' ')
}

function moveClass(row: IndicatorRow): string {
  switch (row.price_direct) {
    case '+':
    case 'up':
      return 'finv-indicator-rise'
    case '-':
    case 'down':
      return 'finv-indicator-fall'
    default:
      return ''
  }
}

/** 列标题悬浮说明：短名 + 完整口径（表列注释）+ 列名 */
function columnHint(column: IndicatorColumn): string {
  const spec = indicatorColumnSpec(column.name)
  const parts = [column.title, `列名：${column.name}`]
  if (spec?.hint) parts.push(spec.hint)
  return parts.join('｜')
}

onMounted(async () => {
  const storedPageSize = Number(localStorage.getItem(STORAGE_PAGE_SIZE))
  if (Number.isFinite(storedPageSize) && storedPageSize > 0) pageSize.value = storedPageSize
  await loadColumns()
  await load()
})

watch(detailOpen, (open) => {
  if (!open) detailRow.value = null
})
</script>

<template>
  <div>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-table-large" class="mr-2" />
        证券指标
        <v-chip class="ml-3" size="small" variant="tonal">
          {{ visibleColumns.length }} / {{ columns.length }} 列
        </v-chip>
        <v-chip class="ml-2" size="small" variant="outlined">{{ presetTitle }}</v-chip>
      </v-card-title>

      <v-card-text>
        <!-- 筛选与操作同处一行，窄屏自动换行 -->
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            v-model="keyword"
            label="关键字（usc / 富途符号 / 名称）"
            placeholder="如: 00700 / HK.00700 / 腾讯"
            density="compact"
            hide-details
            clearable
            style="min-width: 260px; max-width: 340px"
            @keyup.enter="search"
          />
          <v-select
            v-model="market"
            :items="MARKET_OPTIONS"
            item-title="title"
            item-value="value"
            label="市场"
            density="compact"
            hide-details
            style="max-width: 170px"
            @update:model-value="search"
          />
          <v-autocomplete
            v-model="orderBy"
            :items="orderOptions"
            item-title="title"
            item-value="value"
            label="排序字段"
            density="compact"
            hide-details
            clearable
            style="min-width: 240px; max-width: 320px"
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
          <v-btn
            color="secondary"
            variant="tonal"
            prepend-icon="mdi-view-column-outline"
            :loading="columnsLoading"
            @click="pickerOpen = true"
          >
            选择列
          </v-btn>
        </div>

        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
          {{ error }}
        </v-alert>

        <v-alert
          v-if="!columnsLoading && !columns.length && !error"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-3"
        >
          指标表暂无列信息（cdeu.finv_quote_secu_indicator 不存在？），请先执行「证券参数同步 → FT证券指标数据同步」。
        </v-alert>

        <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />

        <div class="finv-indicator-table-wrap">
          <v-table density="compact" class="text-caption finv-indicator-table">
            <thead>
              <tr>
                <th
                  v-for="column in visibleColumns"
                  :key="column.name"
                  class="text-no-wrap"
                  :class="{ 'text-right': isIndicatorNumericColumn(column.name) }"
                  :title="columnHint(column)"
                >
                  {{ indicatorColumnLabel(column) }}
                </th>
                <th class="text-no-wrap text-center">详情</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(row, index) in rows"
                :key="`${row.usc}-${index}`"
                class="finv-indicator-row"
                @click="openDetail(row)"
              >
                <td
                  v-for="column in visibleColumns"
                  :key="column.name"
                  class="text-no-wrap"
                  :class="cellClass(column)"
                >
                  <template v-if="isIndicatorLongText(column.name)">
                    <span v-if="row[column.name]" class="d-inline-flex align-center">
                      <TextPopover :title="indicatorColumnLabel(column)" :text="row[column.name]" />
                      <span class="text-caption text-medium-emphasis">查看</span>
                    </span>
                    <span v-else>—</span>
                  </template>
                  <template v-else>
                    <span :class="cellClass(column).includes('finv-indicator-move') ? moveClass(row) : ''">
                      {{ cellText(column, row) }}
                    </span>
                  </template>
                </td>
                <td class="text-center">
                  <v-btn
                    size="x-small"
                    variant="text"
                    icon="mdi-open-in-new"
                    title="查看该证券全部 132 列"
                    @click.stop="openDetail(row)"
                  />
                </td>
              </tr>
              <tr v-if="!rows.length && !loading">
                <td :colspan="visibleColumns.length + 1" class="text-center text-medium-emphasis py-6">
                  暂无数据
                </td>
              </tr>
            </tbody>
          </v-table>
        </div>

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
              @update:model-value="onPageSizeChange"
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

    <!-- 列选择抽屉 -->
    <IndicatorColumnPicker
      v-model="pickerOpen"
      :columns="columns"
      :selected="selected"
      :preset="preset"
      @update:selected="onSelectedChange"
      @update:preset="onPresetChange"
    />

    <!-- 行详情：全部 132 列按分组展开 -->
    <v-dialog v-model="detailOpen" max-width="1100" scrollable>
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon icon="mdi-card-text-outline" />
          <span>{{ detailRow?.name || '—' }}</span>
          <code class="text-caption text-medium-emphasis">{{ detailRow?.usc || '—' }}</code>
          <code class="text-caption text-medium-emphasis">{{ detailRow?.ftmm_symbol || '—' }}</code>
          <v-spacer />
          <v-btn size="small" variant="tonal" prepend-icon="mdi-content-copy" @click="copyDetailJson">
            复制 JSON
          </v-btn>
          <v-btn size="small" variant="text" icon="mdi-close" @click="detailOpen = false" />
        </v-card-title>
        <v-divider />
        <v-progress-linear v-if="detailLoading" indeterminate color="primary" />
        <v-card-text>
          <div v-for="group in detailGroups" :key="group.key" class="mb-4">
            <div class="finv-indicator-note-title">{{ group.title }}</div>
            <v-table density="compact" class="text-caption finv-indicator-detail">
              <tbody>
                <tr v-for="column in group.columns" :key="column.name">
                  <td class="text-no-wrap finv-indicator-detail-key">
                    {{ indicatorColumnLabel(column) }}
                  </td>
                  <td class="finv-indicator-detail-desc">
                    <code class="text-caption">{{ column.name }}</code>
                    <TextPopover
                      v-if="column.title !== indicatorColumnLabel(column)"
                      :title="`${indicatorColumnLabel(column)} 的口径`"
                      :text="column.title"
                    />
                  </td>
                  <td class="text-no-wrap">
                    <template v-if="isIndicatorLongText(column.name)">
                      <span v-if="detailRow?.[column.name]" class="d-inline-flex align-center">
                        <TextPopover :title="indicatorColumnLabel(column)" :text="detailRow[column.name]" />
                        <span class="text-caption text-medium-emphasis">查看</span>
                      </span>
                      <span v-else>—</span>
                    </template>
                    <span v-else>{{ cellText(column, detailRow ?? {}) }}</span>
                  </td>
                </tr>
              </tbody>
            </v-table>
          </div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- 说明卡片：页面最底部、唯一（ExplainCardSpec） -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" />
        说明
      </v-card-title>
      <v-card-text>
        <div class="finv-indicator-note">
          <div class="finv-indicator-note-title">表口径</div>
          <div class="text-body-2 finv-indicator-note-body">
            <code>cdeu.finv_quote_secu_indicator</code>：一行 = 一个证券的最新指标快照（主键 <code>usc</code>，
            全表 132 列），只保最新、不存历史。数据由「行情 → 实时行情 → 证券参数同步」的
            「FT证券指标数据同步」从上游 <code>public.finv_quote_secu_meta</code> 分批增量同步，
            采集端也会通过 OpenAPI <code>FTSecuIndicatorUpsert</code> 回传单只证券。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-indicator-note">
          <div class="finv-indicator-note-title">列的读法</div>
          <div class="text-body-2 finv-indicator-note-body">
            132 列按业务分成 {{ INDICATOR_GROUPS.length - 1 }} 组（身份与精度、行情价格、成交与活跃度、股本与市值、盘口与涨跌停、
            估值、股息、区间与极值、盘前、基金、期货与标的、指数与 24h 统计、时间与来源）。主表只渲染「选择列」里勾中的列，
            点行或在「详情」列点击可看该证券全部列。数值以文本返回、由页面按列口径折算：
            金额类按 <strong>万 / 亿 / 万亿</strong> 缩写，比率类直接加 <code>%</code>（库里是百分点数值），
            时间类按浏览器本地时区显示；空值统一显示 <code>—</code>，不显示 0。
            表头、排序下拉与「选择列」里**只显示中文短名**：列名与完整口径看表头悬浮、「选择列」里的列名小字，或行详情。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-indicator-note">
          <div class="finv-indicator-note-title">与其它页面的关系</div>
          <div class="text-body-2 finv-indicator-note-body">
            本页只读。写入有两个来源，都不在本页操作：① 「证券参数同步」页的
            「FT证券指标数据同步」批量同步上游；② 采集端 OpenAPI 回传。
            指标表里 <code>usc</code> 为空的上游行会用 <code>ftmm_symbol</code> 占位（约 3,673 行），
            因此本页会出现 <code>usc = ftmm_symbol</code> 的行，属正常口径而非重复数据。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-indicator-note">
          <div class="finv-indicator-note-title">注意事项</div>
          <div class="text-body-2 finv-indicator-note-body">
            <code>json_unmapped</code> 是上游未映射字段的原始 JSON，点 <code>ⓘ</code> 查看全文并可复制；
            <code>difference</code> / <code>bid_price_lowest</code> / <code>ask_price_highest</code> 的上游口径仍待确认，
            取值仅供参考。排序默认按 <code>dt_update</code> 降序（最近同步的行在前），空值一律排最后。
            市场筛选取 <code>ftmm_symbol</code> 的第一段：<code>CC</code> 为加密货币、<code>FX</code> 为外汇/货币指数，
            其余为交易所市场代码（HK / SH / SZ / BJ / US）。
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
/* 宽表：单元格一律不换行，列宽随内容撑开；左侧三列冻结（横向滚动时固定身份与名称）。
   横向滚动交给外层容器，纵向按内容展开、随整页滚动。 */
.finv-indicator-table-wrap {
  overflow-x: auto;
}

.finv-indicator-table :deep(th),
.finv-indicator-table :deep(td) {
  white-space: nowrap;
  padding-inline: 12px;
}

.finv-indicator-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

.finv-indicator-table :deep(td.num) {
  min-width: 96px;
}

/* 冻结前三列：列宽固定，left 偏移按累计宽度给出 */
.finv-indicator-table :deep(th:nth-child(1)),
.finv-indicator-table :deep(td:nth-child(1)) {
  position: sticky;
  left: 0;
  z-index: 2;
  width: 120px;
  min-width: 120px;
  background: rgb(var(--v-theme-surface));
}

.finv-indicator-table :deep(th:nth-child(2)),
.finv-indicator-table :deep(td:nth-child(2)) {
  position: sticky;
  left: 120px;
  z-index: 2;
  width: 150px;
  min-width: 150px;
  background: rgb(var(--v-theme-surface));
}

.finv-indicator-table :deep(th:nth-child(3)),
.finv-indicator-table :deep(td:nth-child(3)) {
  position: sticky;
  left: 270px;
  z-index: 2;
  width: 160px;
  min-width: 160px;
  background: rgb(var(--v-theme-surface));
}

.finv-indicator-row {
  cursor: pointer;
}

.finv-indicator-row:hover :deep(td) {
  background: rgba(var(--v-theme-primary), 0.06);
}

/* 涨跌着色：只作用于涨跌幅 / 涨跌额两列（红涨绿跌，同「证券行情排行」口径） */
.finv-indicator-rise {
  color: #ff0000;
}

.finv-indicator-fall {
  color: #00e676;
}

.finv-indicator-detail :deep(td) {
  padding-block: 2px;
}

.finv-indicator-detail-key {
  width: 160px;
}

/* 详情第二列放「列名 + 口径 ⓘ」，给个宽度上限，避免把值列挤到屏幕外 */
.finv-indicator-detail-desc {
  max-width: 260px;
  white-space: normal;
  word-break: break-all;
}

/* 说明卡片分节（ExplainCardSpec §4 统一 token） */
.finv-indicator-note {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}

.finv-indicator-note-title {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgb(var(--v-theme-primary));
  margin-bottom: 2px;
}

.finv-indicator-note-body {
  opacity: 0.85;
  line-height: 1.6;
}
</style>
