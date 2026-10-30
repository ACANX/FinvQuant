<script setup lang="ts">
// 文件导出：分页查询 cdeu.finv_quote_cdeu_export_job，支持状态/证券/业务月过滤与人工重试
// 后端接口：GET /Meta/Finv/Quote/DeDup/Job/List、POST /Meta/Finv/Quote/DeDup/Job/Retry
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../../api'
import { bizMonthOptions, fmtISO, fmtMonth, fmtNum } from '../../../../../../utils'

interface JobRow {
  job_id: number
  usc: string
  biz_month: number
  priority_rank: number
  freeze_date: string
  seal_date: string
  status: string
  load_cursor_date: number | null
  load_cursor_ts: number | null
  loaded_rows: number
  base_version: number
  patch_count: number
  oldest_pending_date: number | null
  locked_by: string | null
  locked_until: string | null
  gmt_heartbeat: string | null
  attempt: number
  gmt_next_run: string | null
  last_error: string | null
  gmt_create: string
  gmt_update: string
}

interface PageData {
  total: number
  list: JobRow[]
}

const JOB_STATUS_NAME: Record<string, string> = {
  WAITING: '待执行',
  LOADING: '装载中',
  LOADED: '已装载',
  EXPORTING: '导出中',
  EXPORTED: '已导出',
  PAUSED: '已暂停',
  FAILED: '失败',
  FROZEN: '已冻结',
}
const JOB_STATUS_COLOR: Record<string, string> = {
  WAITING: 'grey',
  LOADING: 'info',
  LOADED: 'info',
  EXPORTING: 'primary',
  EXPORTED: 'success',
  PAUSED: 'warning',
  FAILED: 'error',
  FROZEN: 'deep-purple',
}
const statusItems = Object.keys(JOB_STATUS_NAME).map((value) => ({ title: `${value}（${JOB_STATUS_NAME[value]}）`, value }))

const status = ref<string | null>(null)
const usc = ref('')
const bizMonth = ref('')
/** 业务月份是确定的枚举值：默认最近 10 年（yyyymm），最新在前 */
const bizMonthItems = computed(() => bizMonthOptions())
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<JobRow[]>([])
const loading = ref(false)
const error = ref('')
const toast = ref(false)
const toastText = ref('')
const retrying = ref<number | null>(null)

const headers = [
  { title: '任务 ID', key: 'job_id', width: 90, sortable: false },
  { title: '证券[USC]', key: 'usc', width: 110, sortable: false },
  { title: '业务月份', key: 'biz_month', width: 100, sortable: false },
  { title: '状态', key: 'status', width: 110, sortable: false },
  { title: '已装载行数', key: 'loaded_rows', align: 'end' as const, width: 120, sortable: false },
  { title: 'BASE 版本', key: 'base_version', align: 'end' as const, width: 100, sortable: false },
  { title: '补丁数', key: 'patch_count', align: 'end' as const, width: 90, sortable: false },
  { title: '收口日', key: 'seal_date', width: 110, sortable: false },
  { title: '冻结日', key: 'freeze_date', width: 110, sortable: false },
  { title: '尝试次数', key: 'attempt', align: 'end' as const, width: 90, sortable: false },
  { title: '执行者', key: 'locked_by', width: 120, sortable: false },
  { title: '心跳时间', key: 'gmt_heartbeat', width: 160, sortable: false },
  { title: '下次运行', key: 'gmt_next_run', width: 160, sortable: false },
  { title: '最近错误', key: 'last_error', sortable: false },
  { title: '更新时间', key: 'gmt_update', width: 160, sortable: false },
  { title: '操作', key: 'actions', width: 110, sortable: false },
]

// yyyymm 输入校验：空或 6 位数字
function bizMonthParam(): number | null {
  const text = bizMonth.value.trim()
  if (!text) return null
  return /^\d{6}$/.test(text) ? Number(text) : null
}

async function load() {
  loading.value = true
  error.value = ''
  const month = bizMonthParam()
  if (bizMonth.value.trim() && month === null) {
    error.value = '业务月份格式应为 yyyymm（6 位数字），如 202610'
    loading.value = false
    return
  }
  const params = new URLSearchParams({
    page: String(page.value),
    pageSize: String(pageSize.value),
  })
  const statusText = (status.value ?? '').trim()
  const uscText = usc.value.trim()
  if (statusText) params.set('status', statusText)
  if (uscText) params.set('usc', uscText)
  if (month !== null) params.set('bizMonth', String(month))
  try {
    const data = await apiGet<PageData>(`/Meta/Finv/Quote/DeDup/Job/List?${params.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    rows.value = []
    total.value = 0
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function search() {
  if (page.value !== 1) {
    page.value = 1
  } else {
    load()
  }
}

function reset() {
  status.value = null
  usc.value = ''
  bizMonth.value = ''
  search()
}

// 人工重试：把任务重置为 WAITING（后端校验状态，失败时返回 409）
async function retry(row: JobRow) {
  retrying.value = row.job_id
  try {
    await apiPost<{ job_id: number }>('/Meta/Finv/Quote/DeDup/Job/Retry', { job_id: row.job_id })
    toastText.value = `任务 ${row.job_id} 已重置为 WAITING`
    toast.value = true
    await load()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    retrying.value = null
  }
}

watch([page, pageSize], () => load())

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-clipboard-list-outline" class="mr-2" />
      文件导出
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-3">
        <v-col cols="12" sm="6" md="2">
          <v-select
            v-model="status"
            :items="statusItems"
            label="任务状态"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="12" sm="6" md="2">
          <v-text-field v-model="usc" label="证券[USC]" placeholder="如 GCMain" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="12" sm="6" md="2">
          <v-select v-model="bizMonth" :items="bizMonthItems"
                    label="业务月份" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" :loading="loading" @click="search">查询</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
        </v-col>
        <v-col cols="12" md="auto" class="ml-md-auto text-medium-emphasis text-caption">
          共 <strong>{{ total }}</strong> 个任务
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
        item-value="job_id"
        density="compact"
        class="text-no-wrap"
      >
        <template #item.biz_month="{ item }">{{ fmtMonth(item.biz_month) }}</template>
        <template #item.status="{ item }">
          <v-chip size="small" :color="JOB_STATUS_COLOR[item.status] ?? 'grey'" variant="tonal">
            {{ JOB_STATUS_NAME[item.status] ?? item.status }}
          </v-chip>
        </template>
        <template #item.loaded_rows="{ item }">{{ fmtNum(item.loaded_rows, 0) }}</template>
        <template #item.seal_date="{ item }">{{ fmtISO(item.seal_date) }}</template>
        <template #item.freeze_date="{ item }">{{ fmtISO(item.freeze_date) }}</template>
        <template #item.locked_by="{ item }">{{ item.locked_by ?? '—' }}</template>
        <template #item.gmt_heartbeat="{ item }">{{ fmtISO(item.gmt_heartbeat) }}</template>
        <template #item.gmt_next_run="{ item }">{{ fmtISO(item.gmt_next_run) }}</template>
        <template #item.last_error="{ item }">
          <v-tooltip v-if="item.last_error" :text="item.last_error" location="top">
            <template #activator="{ props }">
              <span v-bind="props" class="text-error text-truncate d-inline-block" style="max-width: 260px">
                {{ item.last_error }}
              </span>
            </template>
          </v-tooltip>
          <span v-else class="text-medium-emphasis">—</span>
        </template>
        <template #item.gmt_update="{ item }">{{ fmtISO(item.gmt_update) }}</template>
        <template #item.actions="{ item }">
          <v-btn
            size="small"
            variant="text"
            color="primary"
            prepend-icon="mdi-refresh"
            :loading="retrying === item.job_id"
            @click="retry(item)"
          >
            重试
          </v-btn>
        </template>
        <template #no-data>
          <div class="py-6 text-medium-emphasis">当前条件下没有导出任务</div>
        </template>
      </v-data-table-server>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>数据来源：cdeu.finv_quote_cdeu_export_job；接口 GET /API/V1/Meta/Finv/Quote/DeDup/Job/List。</div>
          <div class="mt-1">「重试」调用 POST /Job/Retry，将任务重置为 WAITING；仅失败 / 暂停等可重试状态生效，否则返回冲突提示。</div>
        </v-card-text>
      </v-card>
    </v-card-text>

    <v-snackbar v-model="toast" :timeout="3000" color="success">{{ toastText }}</v-snackbar>
  </v-card>
</template>
