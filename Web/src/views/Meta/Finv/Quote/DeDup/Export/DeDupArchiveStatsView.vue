<script setup lang="ts">
// 归档统计与预算：入口日统计、每日预算（可覆盖）、扫描水位（可重置）、成熟度（可重算）
// 接口：Stats/Page、Budget/Page、Budget/Override、Watermark/List、Watermark、Maturity/Page、Maturity/Refresh
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost, apiPut } from '../../../../../../api'
import { bizMonthOptions, fmtBytes, fmtDate, fmtISO, fmtMonth, fmtNum, fmtPct, isoDate } from '../../../../../../utils'

interface PageData<T> {
  total: number
  list: T[]
}

interface IngestStat {
  stat_date: string
  total_rows: number
  total_bytes: number
  gmt_update: string
}

interface DailyBudget {
  biz_date: string
  base_rows: number
  base_bytes: number
  p75_rows: number | null
  backlog_ratio: number | null
  sla_pressure: number | null
  factor: number
  budget_rows: number
  budget_bytes: number
  hard_cap_rows: number
  consumed_rows: number
  consumed_bytes: number
  status: string
  gmt_create: string
}

interface WatermarkRow {
  watermark_name: string
  last_value: string
  gmt_update: string
}

interface MaturityRow {
  usc: string
  trade_date: number
  biz_month: number
  expected_rows: number
  actual_rows: number
  maturity_status: string
  gmt_first_seen: string | null
  gmt_last_arrival: string | null
  gmt_seale: string | null
  gmt_update: string
}

const MATURITY_NAME: Record<string, string> = {
  IMMATURE: '未成熟',
  PARTIAL: '部分到达',
  COMPLETE: '已完整',
  FROZEN: '已冻结',
}
const MATURITY_COLOR: Record<string, string> = {
  IMMATURE: 'grey',
  PARTIAL: 'warning',
  COMPLETE: 'success',
  FROZEN: 'deep-purple',
}
const maturityStatusItems = Object.keys(MATURITY_NAME).map((value) => ({ title: `${value}（${MATURITY_NAME[value]}）`, value }))

const tab = ref('stats')
const error = ref('')
const toast = ref(false)
const toastText = ref('')

// ---- 入口统计 ----
const stats = ref<PageData<IngestStat>>({ total: 0, list: [] })
const statsPage = ref(1)
const statsPageSize = ref(20)
const statsFrom = ref('')
const statsTo = ref('')
const statsLoading = ref(false)

// ---- 预算 ----
const budgets = ref<PageData<DailyBudget>>({ total: 0, list: [] })
const budgetPage = ref(1)
const budgetPageSize = ref(20)
const budgetLoading = ref(false)
const budgetDialog = ref(false)
const budgetForm = ref({ biz_date: isoDate(new Date()), budget_rows: 0, hard_cap_rows: 0, factor: 1.2, status: 'READY' })
const budgetSaving = ref(false)

// ---- 水位 ----
const watermarks = ref<WatermarkRow[]>([])
const watermarkLoading = ref(false)
const watermarkDialog = ref(false)
const watermarkForm = ref({ watermark_name: '', last_value: '' })
const watermarkSaving = ref(false)

// ---- 成熟度 ----
const maturity = ref<PageData<MaturityRow>>({ total: 0, list: [] })
const maturityPage = ref(1)
const maturityPageSize = ref(20)
const maturityUsc = ref('')
const maturityStatus = ref<string | null>(null)
const maturityMonth = ref('')
/** 业务月份是确定的枚举值：默认最近 10 年（yyyymm），最新在前 */
const bizMonthItems = computed(() => bizMonthOptions())
const maturityLoading = ref(false)
const refreshing = ref(false)

const statsHeaders = [
  { title: '统计日', key: 'stat_date', width: 120, sortable: false },
  { title: '入口行数', key: 'total_rows', align: 'end' as const, sortable: false },
  { title: '入口字节', key: 'total_bytes', align: 'end' as const, sortable: false },
  { title: '更新时间', key: 'gmt_update', width: 170, sortable: false },
]
const budgetHeaders = [
  { title: '预算日', key: 'biz_date', width: 110, sortable: false },
  { title: '基线行数', key: 'base_rows', align: 'end' as const, sortable: false },
  { title: 'P75 行数', key: 'p75_rows', align: 'end' as const, sortable: false },
  { title: '积压系数', key: 'backlog_ratio', align: 'end' as const, sortable: false },
  { title: '因子', key: 'factor', align: 'end' as const, width: 80, sortable: false },
  { title: '预算行数', key: 'budget_rows', align: 'end' as const, sortable: false },
  { title: '硬上限', key: 'hard_cap_rows', align: 'end' as const, sortable: false },
  { title: '已消耗', key: 'consumed_rows', align: 'end' as const, sortable: false },
  { title: '使用率', key: 'usage', align: 'end' as const, width: 100, sortable: false },
  { title: '状态', key: 'status', width: 100, sortable: false },
  { title: '操作', key: 'actions', width: 100, sortable: false },
]
const watermarkHeaders = [
  { title: '水位名称', key: 'watermark_name', sortable: false },
  { title: '当前值', key: 'last_value', width: 200, sortable: false },
  { title: '更新时间', key: 'gmt_update', width: 170, sortable: false },
  { title: '操作', key: 'actions', width: 100, sortable: false },
]
const maturityHeaders = [
  { title: '证券[USC]', key: 'usc', width: 110, sortable: false },
  { title: '交易日期', key: 'trade_date', width: 110, sortable: false },
  { title: '业务月份', key: 'biz_month', width: 100, sortable: false },
  { title: '期望行数', key: 'expected_rows', align: 'end' as const, sortable: false },
  { title: '实际行数', key: 'actual_rows', align: 'end' as const, sortable: false },
  { title: '成熟度', key: 'maturity_status', width: 120, sortable: false },
  { title: '首次入库', key: 'gmt_first_seen', width: 170, sortable: false },
  { title: '最近到达', key: 'gmt_last_arrival', width: 170, sortable: false },
  { title: '收口时间', key: 'gmt_seale', width: 170, sortable: false },
]

function bizMonthParam(value: string): number | null {
  const text = value.trim()
  if (!text) return null
  return /^\d{6}$/.test(text) ? Number(text) : null
}

async function loadStats() {
  statsLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(statsPage.value), pageSize: String(statsPageSize.value) })
  if (statsFrom.value) params.set('dateFrom', statsFrom.value)
  if (statsTo.value) params.set('dateTo', statsTo.value)
  try {
    stats.value = await apiGet<PageData<IngestStat>>(`/Meta/Finv/Quote/DeDup/Stats/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    statsLoading.value = false
  }
}

async function loadBudgets() {
  budgetLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(budgetPage.value), pageSize: String(budgetPageSize.value) })
  try {
    budgets.value = await apiGet<PageData<DailyBudget>>(`/Meta/Finv/Quote/DeDup/Budget/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    budgetLoading.value = false
  }
}

async function loadWatermarks() {
  watermarkLoading.value = true
  error.value = ''
  try {
    const data = await apiGet<PageData<WatermarkRow>>('/Meta/Finv/Quote/DeDup/Watermark/List?pageSize=200')
    watermarks.value = data.list ?? []
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    watermarkLoading.value = false
  }
}

async function loadMaturity() {
  maturityLoading.value = true
  error.value = ''
  const month = bizMonthParam(maturityMonth.value)
  if (maturityMonth.value.trim() && month === null) {
    error.value = '业务月份格式应为 yyyymm（6 位数字），如 202608'
    maturityLoading.value = false
    return
  }
  const params = new URLSearchParams({ page: String(maturityPage.value), pageSize: String(maturityPageSize.value) })
  if (maturityUsc.value.trim()) params.set('usc', maturityUsc.value.trim())
  if (maturityStatus.value) params.set('status', maturityStatus.value)
  if (month !== null) params.set('bizMonth', String(month))
  try {
    maturity.value = await apiGet<PageData<MaturityRow>>(`/Meta/Finv/Quote/DeDup/Maturity/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    maturityLoading.value = false
  }
}

// 打开预算覆盖对话框（默认带入今日/首行数据）
function openBudget(row?: DailyBudget) {
  if (row) {
    budgetForm.value = {
      biz_date: fmtDateText(row.biz_date),
      budget_rows: row.budget_rows,
      hard_cap_rows: row.hard_cap_rows,
      factor: row.factor,
      status: row.status,
    }
  } else {
    budgetForm.value = {
      biz_date: isoDate(new Date()),
      budget_rows: budgets.value.list[0]?.budget_rows ?? 0,
      hard_cap_rows: budgets.value.list[0]?.hard_cap_rows ?? 0,
      factor: budgets.value.list[0]?.factor ?? 1.2,
      status: 'READY',
    }
  }
  budgetDialog.value = true
}

async function saveBudget() {
  budgetSaving.value = true
  error.value = ''
  try {
    await apiPut<{ biz_date: string }>('/Meta/Finv/Quote/DeDup/Budget/Override', budgetForm.value)
    toastText.value = '预算已覆盖，下一个调度周期生效'
    toast.value = true
    budgetDialog.value = false
    await loadBudgets()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    budgetSaving.value = false
  }
}

function openWatermark(row: WatermarkRow) {
  watermarkForm.value = { watermark_name: row.watermark_name, last_value: row.last_value }
  watermarkDialog.value = true
}

async function saveWatermark() {
  watermarkSaving.value = true
  error.value = ''
  try {
    await apiPut<{ watermark_name: string }>('/Meta/Finv/Quote/DeDup/Watermark', watermarkForm.value)
    toastText.value = '水位已重置'
    toast.value = true
    watermarkDialog.value = false
    await loadWatermarks()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    watermarkSaving.value = false
  }
}

async function refreshMaturity() {
  refreshing.value = true
  error.value = ''
  try {
    await apiPost<{ job_kind: string }>('/Meta/Finv/Quote/DeDup/Maturity/Refresh', {})
    toastText.value = '已触发成熟度重算，稍后刷新查看'
    toast.value = true
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    refreshing.value = false
  }
}

function fmtDateText(value: string): string {
  return value ? value.slice(0, 10) : ''
}

function usage(row: DailyBudget): string {
  if (!row.budget_rows) return '-'
  return fmtPct((row.consumed_rows / row.budget_rows) * 100)
}

watch([statsPage, statsPageSize], loadStats)
watch([budgetPage, budgetPageSize], loadBudgets)
watch([maturityPage, maturityPageSize], () => {
  if (tab.value === 'maturity') loadMaturity()
})

onMounted(async () => {
  await Promise.all([loadStats(), loadBudgets(), loadWatermarks(), loadMaturity()])
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-chart-box-multiple-outline" class="mr-2" />
      归档统计与预算
    </v-card-title>

    <v-card-text>
      <v-tabs v-model="tab" density="compact" class="mb-3">
        <v-tab value="stats">入口统计</v-tab>
        <v-tab value="budget">每日预算</v-tab>
        <v-tab value="watermark">扫描水位</v-tab>
        <v-tab value="maturity">成熟度</v-tab>
      </v-tabs>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-window v-model="tab">
        <!-- 入口统计 -->
        <v-window-item value="stats">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="statsFrom" type="date" label="起始日期" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="statsTo" type="date" label="结束日期" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="statsLoading" @click="loadStats">查询</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ stats.total }}</strong> 天
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="statsPage"
            v-model:items-per-page="statsPageSize"
            :headers="statsHeaders"
            :items="stats.list"
            :items-length="stats.total"
            :loading="statsLoading"
            item-value="stat_date"
            density="compact"
          >
            <template #item.stat_date="{ item }">{{ fmtDateText(item.stat_date) }}</template>
            <template #item.total_rows="{ item }">{{ fmtNum(item.total_rows, 0) }}</template>
            <template #item.total_bytes="{ item }">{{ fmtBytes(item.total_bytes) }}</template>
            <template #item.gmt_update="{ item }">{{ fmtISO(item.gmt_update) }}</template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无入口统计数据（由 CALC_DAILY_INGEST_STATS 每日生成）</div></template>
          </v-data-table-server>
        </v-window-item>

        <!-- 每日预算 -->
        <v-window-item value="budget">
          <v-row dense align="center" class="mb-3">
            <v-col cols="auto">
              <v-btn color="primary" variant="tonal" prepend-icon="mdi-tune" @click="openBudget()">覆盖当日预算</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ budgets.total }}</strong> 天
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="budgetPage"
            v-model:items-per-page="budgetPageSize"
            :headers="budgetHeaders"
            :items="budgets.list"
            :items-length="budgets.total"
            :loading="budgetLoading"
            item-value="biz_date"
            density="compact"
          >
            <template #item.biz_date="{ item }">{{ fmtDateText(item.biz_date) }}</template>
            <template #item.base_rows="{ item }">{{ fmtNum(item.base_rows, 0) }}</template>
            <template #item.p75_rows="{ item }">{{ item.p75_rows === null ? '—' : fmtNum(item.p75_rows, 0) }}</template>
            <template #item.backlog_ratio="{ item }">{{ item.backlog_ratio === null ? '—' : fmtNum(item.backlog_ratio, 2) }}</template>
            <template #item.factor="{ item }">{{ fmtNum(item.factor, 2) }}</template>
            <template #item.budget_rows="{ item }">{{ fmtNum(item.budget_rows, 0) }}</template>
            <template #item.hard_cap_rows="{ item }">{{ fmtNum(item.hard_cap_rows, 0) }}</template>
            <template #item.consumed_rows="{ item }">{{ fmtNum(item.consumed_rows, 0) }}</template>
            <template #item.usage="{ item }">{{ usage(item) }}</template>
            <template #item.status="{ item }">
              <v-chip size="small" variant="tonal" :color="item.status === 'READY' ? 'success' : 'grey'">{{ item.status }}</v-chip>
            </template>
            <template #item.actions="{ item }">
              <v-btn size="small" variant="text" color="primary" prepend-icon="mdi-pencil" @click="openBudget(item)">覆盖</v-btn>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无预算数据（由 CALC_DAILY_EXPORT_BUDGET 每日生成）</div></template>
          </v-data-table-server>
        </v-window-item>

        <!-- 扫描水位 -->
        <v-window-item value="watermark">
          <v-data-table
            :headers="watermarkHeaders"
            :items="watermarks"
            :loading="watermarkLoading"
            item-value="watermark_name"
            density="compact"
          >
            <template #item.last_value="{ item }">{{ fmtISO(item.last_value) }}</template>
            <template #item.gmt_update="{ item }">{{ fmtISO(item.gmt_update) }}</template>
            <template #item.actions="{ item }">
              <v-btn size="small" variant="text" color="warning" prepend-icon="mdi-restore" @click="openWatermark(item)">重置</v-btn>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无水位记录</div></template>
          </v-data-table>
        </v-window-item>

        <!-- 成熟度 -->
        <v-window-item value="maturity">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="maturityUsc" label="证券[USC]" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-select v-model="maturityStatus" :items="maturityStatusItems" label="成熟度" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-select v-model="maturityMonth" :items="bizMonthItems"
                        label="业务月份" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="maturityLoading" @click="loadMaturity">查询</v-btn>
            </v-col>
            <v-col cols="auto">
              <v-btn variant="tonal" color="warning" prepend-icon="mdi-refresh" :loading="refreshing" @click="refreshMaturity">立即重算</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ maturity.total }}</strong> 条
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="maturityPage"
            v-model:items-per-page="maturityPageSize"
            :headers="maturityHeaders"
            :items="maturity.list"
            :items-length="maturity.total"
            :loading="maturityLoading"
            item-value="trade_date"
            density="compact"
          >
            <template #item.trade_date="{ item }">{{ fmtDate(item.trade_date) }}</template>
            <template #item.biz_month="{ item }">{{ fmtMonth(item.biz_month) }}</template>
            <template #item.expected_rows="{ item }">{{ fmtNum(item.expected_rows, 0) }}</template>
            <template #item.actual_rows="{ item }">{{ fmtNum(item.actual_rows, 0) }}</template>
            <template #item.maturity_status="{ item }">
              <v-chip size="small" variant="tonal" :color="MATURITY_COLOR[item.maturity_status] ?? 'grey'">
                {{ MATURITY_NAME[item.maturity_status] ?? item.maturity_status }}
              </v-chip>
            </template>
            <template #item.gmt_first_seen="{ item }">{{ fmtISO(item.gmt_first_seen) }}</template>
            <template #item.gmt_last_arrival="{ item }">{{ fmtISO(item.gmt_last_arrival) }}</template>
            <template #item.gmt_seale="{ item }">{{ fmtISO(item.gmt_seale) }}</template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无成熟度数据（由 REFRESH_QUOTE_MATURITY 每日刷新）</div></template>
          </v-data-table-server>
        </v-window-item>
      </v-window>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>数据来源：finv_dedup_ingest_stats、finv_dedup_daily_budget、finv_dedup_scan_watermark、finv_dedup_quote_maturity_mon。</div>
          <div class="mt-1">「覆盖当日预算」「重置水位」为管理员写操作，会记录审计（job_kind=MANUAL_ADMIN_EDIT）；「立即重算」异步触发 REFRESH_QUOTE_MATURITY。</div>
        </v-card-text>
      </v-card>
    </v-card-text>

    <!-- 预算覆盖对话框 -->
    <v-dialog v-model="budgetDialog" max-width="520">
      <v-card>
        <v-card-title>覆盖预算</v-card-title>
        <v-card-text>
          <v-text-field v-model="budgetForm.biz_date" type="date" label="预算日" density="compact" class="mb-2" />
          <v-text-field v-model.number="budgetForm.budget_rows" type="number" label="预算行数" density="compact" class="mb-2" />
          <v-text-field v-model.number="budgetForm.hard_cap_rows" type="number" label="硬上限行数（不得小于预算）" density="compact" class="mb-2" />
          <v-text-field v-model.number="budgetForm.factor" type="number" step="0.05" label="预算因子（1.00 ~ 3.00）" density="compact" class="mb-2" />
          <v-text-field v-model="budgetForm.status" label="状态（READY / PAUSED）" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="budgetDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="budgetSaving" @click="saveBudget">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 水位重置对话框 -->
    <v-dialog v-model="watermarkDialog" max-width="520">
      <v-card>
        <v-card-title>重置水位（高危）</v-card-title>
        <v-card-text>
          <v-alert type="warning" density="compact" variant="tonal" class="mb-3">
            重置后 {{ watermarkForm.watermark_name }} 会重新扫描该时间之后的数据，请确认影响范围。
          </v-alert>
          <v-text-field v-model="watermarkForm.last_value" label="新的水位值（RFC3339）" density="compact" placeholder="2026-10-01T00:00:00Z" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="watermarkDialog = false">取消</v-btn>
          <v-btn color="warning" :loading="watermarkSaving" @click="saveWatermark">确认重置</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="toast" :timeout="3000" color="success">{{ toastText }}</v-snackbar>
  </v-card>
</template>
