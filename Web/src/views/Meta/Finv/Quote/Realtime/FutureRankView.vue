<script setup lang="ts">
// 期货行情排行（cdeu.finv_quote_realtime_future，主键 stock_id，需求 11322）
// 接口：GET /Meta/Finv/Quote/Cdeu/Realtime/Future/Page?keyword=&futuresType=&marketLabel=&onlyOnline=&orderBy=&order=&page=&pageSize=
// 分类中文名来自 cdeu.finv_quote_cdeu_future_sync_config（接口 /Future/Types）。
// 表里只有代码/名称/现价/涨跌幅是结构化列，其余字段留档在 fields_json，
// 字段定义在 field_list_json —— 明细弹窗按 field_list_json 把编号翻成字段名，不臆造含义。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'

interface FutureRow {
  stock_id: string
  usc: string
  futures_type: string
  futures_type_name: string
  market_type: string
  market_label: string
  contract_code: string
  name: string
  price_nominal: string
  change_ratio: string
  price_direct: string
  flag_online: string
  provider: string
  remark: string
  dt_update: number
  fields_json: string
  field_list_json: string
}

interface FutureTypeOption {
  value: string
  label: string
}

/** field_list_json 元素（富途字段字典留档）。 */
interface FutureFieldDef {
  fieldId: number
  fieldName: string
}

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Future/Page'
const TYPES_API = '/Meta/Finv/Quote/Cdeu/Realtime/Future/Types'

const rows = ref<FutureRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const futuresType = ref('')
const marketLabel = ref('')
const onlyOnline = ref<'1' | ''>('1')
const orderBy = ref('change_ratio')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const error = ref('')

const typeOptions = ref<FutureTypeOption[]>([])

// 明细弹窗
const detailRow = ref<FutureRow | null>(null)
const detailOpen = ref(false)

const ORDER_OPTIONS = [
  { title: '涨跌幅', value: 'change_ratio' },
  { title: '现价', value: 'price_nominal' },
  { title: '更新时间', value: 'dt_update' },
  { title: '合约代码', value: 'contract_code' },
  { title: '名称', value: 'name' },
  { title: '分类', value: 'futures_type' },
  { title: '市场', value: 'market_label' },
  { title: '编号', value: 'stock_id' },
]
const MARKET_OPTIONS = [
  { title: '全部市场', value: '' },
  { title: 'HK', value: 'HK' },
  { title: 'US', value: 'US' },
  { title: 'SG', value: 'SG' },
  { title: 'JP', value: 'JP' },
]
const ONLINE_OPTIONS = [
  { title: '仅在线', value: '1' },
  { title: '全部合约', value: '' },
]

// 分类下拉：首项「全部分类」，其余按配置表 1~8（缺名称时降级为「分类 N」）
const FUTURES_TYPE_OPTIONS = computed(() => [
  { title: '全部分类', value: '' },
  ...typeOptions.value.map((o) => ({
    title: o.label ? `${o.label}[${o.value}]` : `分类 ${o.value}[${o.value}]`,
    value: o.value,
  })),
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
    if (futuresType.value) params.set('futuresType', futuresType.value)
    if (marketLabel.value) params.set('marketLabel', marketLabel.value)
    if (onlyOnline.value) params.set('onlyOnline', '1')
    const data = await apiGet<{ total: number; page: number; pageSize: number; list: FutureRow[] }>(
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

async function loadTypes() {
  try {
    const data = await apiGet<{ list: FutureTypeOption[] }>(TYPES_API)
    typeOptions.value = data.list ?? []
  } catch {
    // 分类字典取不到不影响列表；下拉只有「全部分类」
    typeOptions.value = []
  }
}

function search() {
  page.value = 1
  void load()
}

function reset() {
  keyword.value = ''
  futuresType.value = ''
  marketLabel.value = ''
  onlyOnline.value = '1'
  orderBy.value = 'change_ratio'
  order.value = 'desc'
  search()
}

/** 分类展示名：优先配置表中文名，缺失时降级「分类 N」。 */
function typeText(row: FutureRow): string {
  if (row.futures_type_name) return row.futures_type_name
  return row.futures_type ? `分类 ${row.futures_type}` : '—'
}

// ---- 展示格式化 ----

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

// ---- 明细弹窗：fields_json（编号→原文）+ field_list_json（编号→字段名） ----

const detailFields = computed<{ name: string; value: string }[]>(() => {
  const row = detailRow.value
  if (!row) return []
  let defs: FutureFieldDef[] = []
  let values: Record<string, string> = {}
  try {
    defs = JSON.parse(row.field_list_json || '[]') as FutureFieldDef[]
  } catch {
    defs = []
  }
  try {
    values = JSON.parse(row.fields_json || '{}') as Record<string, string>
  } catch {
    values = {}
  }
  if (defs.length > 0) {
    return defs.map((d) => ({
      name: d.fieldName || `字段 ${d.fieldId}`,
      value: values[String(d.fieldId)] ?? '—',
    }))
  }
  // 没有字段定义时按编号直出
  return Object.entries(values).map(([k, v]) => ({ name: `字段 ${k}`, value: String(v) }))
})

function openDetail(row: FutureRow) {
  detailRow.value = row
  detailOpen.value = true
}

onMounted(async () => {
  await loadTypes()
  await load()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-finance" class="mr-2" />
      期货行情排行
    </v-card-title>
    <v-card-subtitle>
      cdeu.finv_quote_realtime_future：期货合约最新一张行情快照（主键 stock_id，整行覆盖）
    </v-card-subtitle>

    <v-card-text>
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（代码 / 名称 / usc）"
          placeholder="如: XAMD0main / 微型辉瑞"
          density="compact"
          hide-details
          clearable
          style="min-width: 240px; max-width: 320px"
          @keyup.enter="search"
        />
        <v-select
          v-model="futuresType"
          :items="FUTURES_TYPE_OPTIONS"
          item-title="title"
          item-value="value"
          label="分类"
          density="compact"
          hide-details
          style="max-width: 180px"
          @update:model-value="search"
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
          v-model="onlyOnline"
          :items="ONLINE_OPTIONS"
          item-title="title"
          item-value="value"
          label="状态"
          density="compact"
          hide-details
          style="max-width: 140px"
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
            <th class="text-no-wrap">编号</th>
            <th class="text-no-wrap">合约代码</th>
            <th class="text-no-wrap">名称</th>
            <th class="text-no-wrap">分类</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap text-right">现价</th>
            <th class="text-no-wrap text-right">涨跌幅</th>
            <th class="text-no-wrap">usc</th>
            <th class="text-no-wrap">在线</th>
            <th class="text-no-wrap">提供方</th>
            <th class="text-no-wrap">更新时间</th>
            <th class="text-no-wrap">明细</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.stock_id" :class="rowClass(row.price_direct)">
            <td class="text-no-wrap">{{ row.stock_id }}</td>
            <td class="text-no-wrap">{{ row.contract_code || '—' }}</td>
            <td class="text-no-wrap">{{ row.name || '—' }}</td>
            <td class="text-no-wrap">{{ typeText(row) }}</td>
            <td class="text-no-wrap">{{ row.market_label || '—' }}</td>
            <td class="text-right num">{{ dec(row.price_nominal, 4) }}</td>
            <td class="text-right num">{{ pct(row.change_ratio) }}</td>
            <td class="text-no-wrap">{{ row.usc || '—' }}</td>
            <td class="text-no-wrap">{{ row.flag_online === '1' ? '在线' : '离线' }}</td>
            <td class="text-no-wrap">{{ row.provider || '—' }}</td>
            <td class="text-no-wrap">{{ timeText(row.dt_update) }}</td>
            <td class="text-no-wrap">
              <v-btn
                size="x-small"
                variant="text"
                prepend-icon="mdi-text-box-search-outline"
                @click="openDetail(row)"
              >
                明细
              </v-btn>
            </td>
          </tr>
          <tr v-if="!rows.length && !loading">
            <td colspan="12" class="text-center text-medium-emphasis py-6">暂无数据</td>
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

    <!-- 明细：按 field_list_json 把 fields_json 的编号翻成字段名 -->
    <v-dialog v-model="detailOpen" max-width="720">
      <v-card>
        <v-card-title class="text-body-1">
          {{ detailRow?.name || detailRow?.contract_code }}
          <span class="text-caption text-medium-emphasis ml-2">
            {{ detailRow?.contract_code }} / 编号 {{ detailRow?.stock_id }}
          </span>
        </v-card-title>
        <v-card-text>
          <v-table density="compact" class="text-caption">
            <thead>
              <tr>
                <th class="text-no-wrap">字段</th>
                <th class="text-no-wrap">值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in detailFields" :key="f.name">
                <td class="text-no-wrap">{{ f.name }}</td>
                <td>{{ f.value }}</td>
              </tr>
              <tr v-if="!detailFields.length">
                <td colspan="2" class="text-center text-medium-emphasis py-4">该合约没有字段留档</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="detailOpen = false">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
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
