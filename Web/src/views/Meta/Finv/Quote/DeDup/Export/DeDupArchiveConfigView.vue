<script setup lang="ts">
// 归档主数据与规则：证券主数据（关键参数 day_expect_rows）、优先级 / SLA、交易日历，均支持管理员编辑
// 接口：Usc/Page + Usc、Priority（PUT/DELETE，V100051 起写入 finv_dedup_usc）、Calendar/Page + Calendar（PUT/DELETE）
import { computed, onMounted, ref, watch } from 'vue'

import { apiDelete, apiGet, apiPut } from '../../../../../../api'
import { bizMonthOptions, fmtISO, fmtNum, isoDate } from '../../../../../../utils'

interface PageData<T> {
  total: number
  list: T[]
}

interface UscRow {
  usc: string
  category: string | null
  day_expect_rows: number | null
  date_list: string | null
  date_delist: string | null
  flag_auto_discover: boolean
  region: string | null
  market: string | null
  provider: string | null
  name: string | null
  timezone: string | null
  currency: string | null
  // 归档优先级覆盖配置（V100051 由 finv_dedup_usc_priority 合并而来）
  priority_rank: number | null
  sla_days: number | null
  max_wait_days: number | null
  priority_gmt_update: string | null
  gmt_update: string
}

interface CalendarRow {
  usc: string
  trade_date: number
  flag_trade_day: boolean
}

const tab = ref('usc')
const error = ref('')
const toast = ref(false)
const toastText = ref('')

// ---- 证券主数据 ----
const uscRows = ref<PageData<UscRow>>({ total: 0, list: [] })
const uscPage = ref(1)
const uscPageSize = ref(20)
const uscKeyword = ref('')
const uscLoading = ref(false)
const uscDialog = ref(false)
const uscSaving = ref(false)
const uscForm = ref({
  usc: '',
  category: '',
  day_expect_rows: null as number | null,
  date_list: '',
  date_delist: '',
  flag_auto_discover: true,
  region: '',
  market: '',
  provider: '',
  name: '',
  timezone: '',
  currency: '',
})

// ---- 优先级 ----
// 优先级覆盖编辑（V100051 起是 finv_dedup_usc 的列，随证券行内联编辑）
const priorityDialog = ref(false)
const prioritySaving = ref(false)
const priorityForm = ref({ usc: '', priority_rank: 1, sla_days: 5, max_wait_days: 7 })
// currentUscRow 当前打开优先级对话框的原始证券行，用于判断是否已有覆盖。
const currentUscRow = ref<UscRow | null>(null)

// ---- 交易日历 ----
const calendarRows = ref<PageData<CalendarRow>>({ total: 0, list: [] })
const calendarPage = ref(1)
const calendarPageSize = ref(20)
const calendarUsc = ref('')
const calendarMonth = ref('')
/** 业务月份是确定的枚举值：默认最近 10 年（yyyymm），最新在前 */
const bizMonthItems = computed(() => bizMonthOptions())
const calendarLoading = ref(false)
const calendarDialog = ref(false)
const calendarSaving = ref(false)
const calendarForm = ref({ usc: '', trade_date: 0, flag_trade_day: true })

const uscHeaders = [
  { title: '证券[USC]', key: 'usc', width: 110, sortable: false },
  { title: '名称', key: 'name', width: 140, sortable: false },
  { title: '分类', key: 'category', width: 100, sortable: false },
  { title: '日校准行数', key: 'day_expect_rows', align: 'end' as const, width: 120, sortable: false },
  { title: '上市日', key: 'date_list', width: 110, sortable: false },
  { title: '退市日', key: 'date_delist', width: 110, sortable: false },
  { title: '自动发现', key: 'flag_auto_discover', width: 100, sortable: false },
  { title: '地区', key: 'region', width: 90, sortable: false },
  { title: '市场', key: 'market', width: 110, sortable: false },
  { title: '数据源', key: 'provider', width: 110, sortable: false },
  { title: '时区', key: 'timezone', width: 130, sortable: false },
  { title: '币种', key: 'currency', width: 80, sortable: false },
  { title: '优先级', key: 'priority_rank', align: 'end' as const, width: 90, sortable: false },
  { title: 'SLA 天数', key: 'sla_days', align: 'end' as const, width: 95, sortable: false },
  { title: '最大等待', key: 'max_wait_days', align: 'end' as const, width: 95, sortable: false },
  { title: '更新时间', key: 'gmt_update', width: 165, sortable: false },
  { title: '操作', key: 'actions', width: 150, sortable: false },
]
const calendarHeaders = [
  { title: '证券[USC]', key: 'usc', sortable: false },
  { title: '交易日期', key: 'trade_date', width: 120, sortable: false },
  { title: '是否交易日', key: 'flag_trade_day', width: 110, sortable: false },
  { title: '操作', key: 'actions', width: 150, sortable: false },
]

function bizMonthParam(value: string): number | null {
  const text = value.trim()
  if (!text) return null
  return /^\d{6}$/.test(text) ? Number(text) : null
}

async function loadUsc() {
  uscLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(uscPage.value), pageSize: String(uscPageSize.value) })
  if (uscKeyword.value.trim()) params.set('keyword', uscKeyword.value.trim())
  try {
    uscRows.value = await apiGet<PageData<UscRow>>(`/Meta/Finv/Quote/DeDup/Usc/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    uscLoading.value = false
  }
}

async function loadCalendar() {
  calendarLoading.value = true
  error.value = ''
  const month = bizMonthParam(calendarMonth.value)
  if (calendarMonth.value.trim() && month === null) {
    error.value = '业务月份格式应为 yyyymm（6 位数字）'
    calendarLoading.value = false
    return
  }
  const params = new URLSearchParams({ page: String(calendarPage.value), pageSize: String(calendarPageSize.value) })
  if (calendarUsc.value.trim()) params.set('usc', calendarUsc.value.trim())
  if (month !== null) params.set('bizMonth', String(month))
  try {
    calendarRows.value = await apiGet<PageData<CalendarRow>>(`/Meta/Finv/Quote/DeDup/Calendar/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    calendarLoading.value = false
  }
}

function openUsc(row?: UscRow) {
  if (row) {
    uscForm.value = {
      usc: row.usc,
      category: row.category ?? '',
      day_expect_rows: row.day_expect_rows,
      date_list: row.date_list ? row.date_list.slice(0, 10) : '',
      date_delist: row.date_delist ? row.date_delist.slice(0, 10) : '',
      flag_auto_discover: row.flag_auto_discover,
      region: row.region ?? '',
      market: row.market ?? '',
      provider: row.provider ?? '',
      name: row.name ?? '',
      timezone: row.timezone ?? '',
      currency: row.currency ?? '',
    }
  } else {
    uscForm.value = {
      usc: '',
      category: '',
      day_expect_rows: null,
      date_list: '',
      date_delist: '',
      flag_auto_discover: true,
      region: '',
      market: '',
      provider: '',
      name: '',
      timezone: '',
      currency: '',
    }
  }
  uscDialog.value = true
}

async function saveUsc() {
  if (!uscForm.value.usc.trim()) {
    error.value = 'usc 不能为空'
    return
  }
  uscSaving.value = true
  error.value = ''
  const form = uscForm.value
  const payload: Record<string, unknown> = {
    usc: form.usc.trim(),
    flag_auto_discover: form.flag_auto_discover,
    category: form.category || null,
    day_expect_rows: form.day_expect_rows,
    date_list: form.date_list || null,
    date_delist: form.date_delist || null,
    region: form.region || null,
    market: form.market || null,
    provider: form.provider || null,
    name: form.name || null,
    timezone: form.timezone || null,
    currency: form.currency || null,
  }
  try {
    await apiPut<{ usc: string }>('/Meta/Finv/Quote/DeDup/Usc', payload)
    toastText.value = `证券 ${form.usc} 已保存`
    toast.value = true
    uscDialog.value = false
    await loadUsc()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    uscSaving.value = false
  }
}

// openPriority 从证券行打开优先级覆盖编辑：未配置时给默认值（100 / 5 / 7）作起点。
function openPriority(row: UscRow) {
  currentUscRow.value = row
  priorityForm.value = {
    usc: row.usc,
    priority_rank: row.priority_rank ?? 100,
    sla_days: row.sla_days ?? 5,
    max_wait_days: row.max_wait_days ?? 7,
  }
  priorityDialog.value = true
}

async function savePriority() {
  prioritySaving.value = true
  error.value = ''
  try {
    await apiPut<{ usc: string }>('/Meta/Finv/Quote/DeDup/Priority', priorityForm.value)
    toastText.value = `${priorityForm.value.usc} 优先级已保存`
    toast.value = true
    priorityDialog.value = false
    await loadUsc()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    prioritySaving.value = false
  }
}

// clearPriority 撤销该证券的自定义优先级，退回默认 100 / 5 / 7（证券行保留）。
async function clearPriority(usc: string) {
  if (!usc) return
  if (!window.confirm(`确认清除 ${usc} 的优先级覆盖？将退回默认 100 / 5 / 7。`)) return
  error.value = ''
  try {
    await apiDelete<{ usc: string }>(`/Meta/Finv/Quote/DeDup/Priority?usc=${encodeURIComponent(usc)}`)
    toastText.value = `${usc} 优先级覆盖已清除`
    toast.value = true
    priorityDialog.value = false
    await loadUsc()
  } catch (e) {
    error.value = (e as Error).message
  }
}

// priorityConfigured 当前编辑项是否已有「人工设置」的覆盖值（决定「清除覆盖」按钮是否可用）。
// 自动补全写入的默认值（priority_gmt_update 为 NULL）不算人工配置。
const priorityConfigured = computed(() => currentUscRow.value?.priority_rank !== null
  && currentUscRow.value?.priority_rank !== undefined
  && currentUscRow.value?.priority_gmt_update !== null)

function openCalendar(row?: CalendarRow) {
  calendarForm.value = row
    ? { usc: row.usc, trade_date: row.trade_date, flag_trade_day: row.flag_trade_day }
    : { usc: '', trade_date: Number(isoDate(new Date()).replace(/-/g, '')), flag_trade_day: true }
  calendarDialog.value = true
}

async function saveCalendar() {
  calendarSaving.value = true
  error.value = ''
  try {
    await apiPut<{ usc: string }>('/Meta/Finv/Quote/DeDup/Calendar', calendarForm.value)
    toastText.value = '交易日历已保存'
    toast.value = true
    calendarDialog.value = false
    await loadCalendar()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    calendarSaving.value = false
  }
}

async function removeCalendar(row: CalendarRow) {
  if (!window.confirm(`确认删除 ${row.usc} / ${row.trade_date} 的日历记录？`)) return
  error.value = ''
  try {
    await apiDelete<{ usc: string }>(
      `/Meta/Finv/Quote/DeDup/Calendar?usc=${encodeURIComponent(row.usc)}&tradeDate=${row.trade_date}`,
    )
    toastText.value = '日历记录已删除'
    toast.value = true
    await loadCalendar()
  } catch (e) {
    error.value = (e as Error).message
  }
}

watch([uscPage, uscPageSize], loadUsc)
watch([calendarPage, calendarPageSize], () => {
  if (tab.value === 'calendar') loadCalendar()
})

onMounted(async () => {
  await Promise.all([loadUsc(), loadCalendar()])
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-tune-variant" class="mr-2" />
      归档主数据与规则
    </v-card-title>

    <v-card-text>
      <v-tabs v-model="tab" density="compact" class="mb-3">
        <v-tab value="usc">证券主数据</v-tab>
        <v-tab value="calendar">交易日历</v-tab>
      </v-tabs>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-window v-model="tab">
        <!-- 证券主数据 -->
        <v-window-item value="usc">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="6" md="3">
              <v-text-field v-model="uscKeyword" label="证券[USC] / 名称" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="uscLoading" @click="loadUsc">查询</v-btn>
            </v-col>
            <v-col cols="auto">
              <v-btn variant="tonal" color="success" prepend-icon="mdi-plus" @click="openUsc()">新增/编辑</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ uscRows.total }}</strong> 只证券
            </v-col>
          </v-row>
          <v-alert type="info" density="compact" variant="tonal" class="mb-3">
            <strong>day_expect_rows</strong> 为空时该证券的「日校准」无法判定，成熟度会长期停留在 PARTIAL，导致永不导出——请优先补全。
          </v-alert>
          <v-alert type="info" density="compact" variant="tonal" class="mb-3">
            归档优先级 / SLA 已并入证券行（V100051）：未配置时按 <strong>100 / 5 / 7</strong> 兜底，
            点行尾「优先级」可设置覆盖，清除覆盖不影响证券本身。
          </v-alert>
          <v-data-table-server
            v-model:page="uscPage"
            v-model:items-per-page="uscPageSize"
            :headers="uscHeaders"
            :items="uscRows.list"
            :items-length="uscRows.total"
            :loading="uscLoading"
            item-value="usc"
            density="compact"
            class="text-no-wrap"
          >
            <template #item.day_expect_rows="{ item }">
              <span :class="item.day_expect_rows === null ? 'text-error' : ''">
                {{ item.day_expect_rows === null ? '未校准' : fmtNum(item.day_expect_rows, 0) }}
              </span>
            </template>
            <template #item.date_list="{ item }">{{ item.date_list ? item.date_list.slice(0, 10) : '—' }}</template>
            <template #item.date_delist="{ item }">{{ item.date_delist ? item.date_delist.slice(0, 10) : '—' }}</template>
            <template #item.flag_auto_discover="{ item }">
              <v-chip size="small" variant="tonal" :color="item.flag_auto_discover ? 'success' : 'grey'">
                {{ item.flag_auto_discover ? '是' : '否' }}
              </v-chip>
            </template>
            <template #item.category="{ item }">{{ item.category ?? '—' }}</template>
            <template #item.name="{ item }">{{ item.name ?? '—' }}</template>
            <template #item.region="{ item }">{{ item.region ?? '—' }}</template>
            <template #item.market="{ item }">{{ item.market ?? '—' }}</template>
            <template #item.provider="{ item }">{{ item.provider ?? '—' }}</template>
            <template #item.timezone="{ item }">{{ item.timezone ?? '—' }}</template>
            <template #item.currency="{ item }">{{ item.currency ?? '—' }}</template>
            <!-- 优先级覆盖：自动补全写入的默认值（priority_gmt_update 为 NULL）置灰并标注「默认」，人工设置过的高亮 -->
            <template #item.priority_rank="{ item }">
              <v-chip v-if="item.priority_rank !== null"
                      size="small"
                      variant="tonal"
                      :color="item.priority_gmt_update === null ? 'grey' : 'primary'">
                {{ item.priority_rank }}<span v-if="item.priority_gmt_update === null">&nbsp;默认</span>
              </v-chip>
              <span v-else class="text-medium-emphasis text-caption">默认 100</span>
            </template>
            <template #item.sla_days="{ item }">
              <span :class="item.priority_gmt_update === null ? 'text-medium-emphasis' : ''">
                {{ item.sla_days ?? '未配置(5)' }}<span v-if="item.sla_days !== null && item.priority_gmt_update === null">（默认）</span>
              </span>
            </template>
            <template #item.max_wait_days="{ item }">
              <span :class="item.priority_gmt_update === null ? 'text-medium-emphasis' : ''">
                {{ item.max_wait_days ?? '未配置(7)' }}<span v-if="item.max_wait_days !== null && item.priority_gmt_update === null">（默认）</span>
              </span>
            </template>
            <template #item.gmt_update="{ item }">{{ fmtISO(item.gmt_update) }}</template>
            <template #item.actions="{ item }">
              <v-btn size="small" variant="text" color="primary" prepend-icon="mdi-pencil" @click="openUsc(item)">编辑</v-btn>
              <v-btn size="small" variant="text" color="info" prepend-icon="mdi-tune" @click="openPriority(item)">优先级</v-btn>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无证券主数据</div></template>
          </v-data-table-server>
        </v-window-item>

        <!-- 交易日历 -->
        <v-window-item value="calendar">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="calendarUsc" label="证券[USC]" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-select v-model="calendarMonth" :items="bizMonthItems"
                        label="业务月份" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="calendarLoading" @click="loadCalendar">查询</v-btn>
            </v-col>
            <v-col cols="auto">
              <v-btn variant="tonal" color="success" prepend-icon="mdi-plus" @click="openCalendar()">新增/编辑</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ calendarRows.total }}</strong> 条
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="calendarPage"
            v-model:items-per-page="calendarPageSize"
            :headers="calendarHeaders"
            :items="calendarRows.list"
            :items-length="calendarRows.total"
            :loading="calendarLoading"
            item-value="trade_date"
            density="compact"
          >
            <template #item.flag_trade_day="{ item }">
              <v-chip size="small" variant="tonal" :color="item.flag_trade_day ? 'success' : 'grey'">
                {{ item.flag_trade_day ? '交易日' : '非交易日' }}
              </v-chip>
            </template>
            <template #item.actions="{ item }">
              <v-btn size="small" variant="text" color="primary" prepend-icon="mdi-pencil" @click="openCalendar(item)">编辑</v-btn>
              <v-btn size="small" variant="text" color="error" prepend-icon="mdi-delete" @click="removeCalendar(item)">删除</v-btn>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无交易日历</div></template>
          </v-data-table-server>
        </v-window-item>
      </v-window>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>可编辑表：finv_dedup_usc（证券主数据 + 归档优先级 / SLA）、finv_dedup_trade_calendar（交易日历）。</div>
          <div class="mt-1">所有保存 / 删除操作都会写一条管理员审计（job_kind=MANUAL_ADMIN_EDIT），可在「归档日志与补丁 → 运行日志」中追溯。</div>
        </v-card-text>
      </v-card>
    </v-card-text>

    <!-- 证券主数据对话框 -->
    <v-dialog v-model="uscDialog" max-width="720">
      <v-card>
        <v-card-title>证券主数据</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.usc" label="证券[USC]（主键，不可改）" density="compact" :readonly="!!uscForm.usc && uscRows.list.some((r) => r.usc === uscForm.usc)" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.name" label="名称" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.category" label="分类" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model.number="uscForm.day_expect_rows" type="number" label="日校准行数（day_expect_rows）" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.date_list" type="date" label="上市日" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.date_delist" type="date" label="退市日" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.region" label="地区" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.market" label="市场" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.provider" label="数据源" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.timezone" label="时区" density="compact" />
            </v-col>
            <v-col cols="12" sm="6">
              <v-text-field v-model="uscForm.currency" label="币种" density="compact" />
            </v-col>
            <v-col cols="12" sm="6" class="d-flex align-center">
              <v-switch v-model="uscForm.flag_auto_discover" label="允许自动发现" density="compact" hide-details color="primary" />
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="uscDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="uscSaving" @click="saveUsc">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 优先级对话框 -->
    <v-dialog v-model="priorityDialog" max-width="460">
      <v-card>
        <v-card-title>优先级 / SLA</v-card-title>
        <v-card-text>
          <v-text-field v-model="priorityForm.usc" label="证券[USC]" density="compact" readonly class="mb-2" />
          <v-text-field v-model.number="priorityForm.priority_rank" type="number" label="优先级（正整数，越小越先导出）" density="compact" class="mb-2" />
          <v-text-field v-model.number="priorityForm.sla_days" type="number" label="SLA 天数" density="compact" class="mb-2" />
          <v-text-field v-model.number="priorityForm.max_wait_days" type="number" label="最大等待天数" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-btn v-if="priorityConfigured" variant="text" color="error" :disabled="prioritySaving"
                 @click="clearPriority(priorityForm.usc)">清除覆盖</v-btn>
          <v-spacer />
          <v-btn variant="text" @click="priorityDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="prioritySaving" @click="savePriority">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 交易日历对话框 -->
    <v-dialog v-model="calendarDialog" max-width="460">
      <v-card>
        <v-card-title>交易日历</v-card-title>
        <v-card-text>
          <v-text-field v-model="calendarForm.usc" label="证券[USC]" density="compact" class="mb-2" />
          <v-text-field v-model.number="calendarForm.trade_date" type="number" label="交易日期（yyyymmdd）" density="compact" class="mb-2" />
          <v-switch v-model="calendarForm.flag_trade_day" label="是否交易日" density="compact" hide-details color="primary" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="calendarDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="calendarSaving" @click="saveCalendar">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="toast" :timeout="3000" color="success">{{ toastText }}</v-snackbar>
  </v-card>
</template>
