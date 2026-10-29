<script setup lang="ts">
// 月归档导出总览：归档链路指标、告警、运行参数与 Job 调度（后端 GET /Meta/Finv/Quote/DeDup/Status）
// 数据来源：cdeu.finv_quote_cdeu_export_job / finv_dedup_export_file / finv_dedup_daily_budget 等聚合
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../../api'
import { fmtISO, fmtNum, fmtPct } from '../../../../../../utils'

interface Alert {
  code: string
  level: string
  message: string
}

interface Metrics {
  generated_at: string
  job_status: Record<string, number>
  upload_status: Record<string, number>
  pending_rows: number
  staging_rows: number
  staging_uscs: number
  backlog_days: number
  budget_factor: number
  budget_rows: number
  budget_consumed_rows: number
  budget_usage_ratio: number
  frozen_missing_rows: number
  frozen_expected_rows: number
  frozen_missing_ratio: number
  heartbeat_timeouts: number
  upload_retry_exhausted: number
  patch_last_7_days: number
  high_frequency_patch_groups: number
  local_files_pending_cleanup: number
  mvsv_anomalies: number
  alerts: Alert[]
}

interface StatusOptions {
  enabled: boolean
  worker_id: string
  time_zone: string
  output_dir: string
  upload_mode: string
  compression: string
  window_start: string
  window_end: string
  page_size: number
  maturity_threshold: number
}

interface JobRuntime {
  job_kind: string
  name: string
  summary: string
  flow: string
  details: string
  cautions: string
  daily_at?: string
  interval_seconds?: number
  windowed: boolean
  schedule: string
}

interface StatusData {
  metrics: Metrics
  options: StatusOptions
  jobs: JobRuntime[]
}

// 任务状态 / 上传状态的中文名与颜色（与后端 model.go 常量一致）
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
const JOB_STATUS_ORDER = ['WAITING', 'LOADING', 'LOADED', 'EXPORTING', 'PAUSED', 'EXPORTED', 'FROZEN', 'FAILED']
const UPLOAD_STATUS_NAME: Record<string, string> = {
  PENDING_UPLOAD: '待上传',
  UPLOADING: '上传中',
  UPLOADED: '已上传',
  UPLOAD_FAILED: '上传失败',
  SKIPPED: '已跳过',
}
const UPLOAD_STATUS_COLOR: Record<string, string> = {
  PENDING_UPLOAD: 'grey',
  UPLOADING: 'primary',
  UPLOADED: 'success',
  UPLOAD_FAILED: 'error',
  SKIPPED: 'warning',
}
const UPLOAD_STATUS_ORDER = ['PENDING_UPLOAD', 'UPLOADING', 'UPLOADED', 'UPLOAD_FAILED', 'SKIPPED']
const ALERT_TYPE: Record<string, 'info' | 'warning' | 'error'> = {
  INFO: 'info',
  WARN: 'warning',
  ERROR: 'error',
}

const data = ref<StatusData | null>(null)
const loading = ref(false)
const error = ref('')
const toast = ref(false)
const toastText = ref('')
const autoRefresh = ref(true)
const runningKind = ref('')

let timer: ReturnType<typeof setInterval> | null = null

const metrics = computed<Metrics | null>(() => data.value?.metrics ?? null)
const options = computed<StatusOptions | null>(() => data.value?.options ?? null)
const alerts = computed<Alert[]>(() => metrics.value?.alerts ?? [])

// 指标卡（值统一格式化，避免模板里堆表达式）
const cards = computed(() => {
  const m = metrics.value
  if (!m) return []
  return [
    { title: '待装载行数', value: fmtNum(m.pending_rows, 0), icon: 'mdi-table-clock', hint: `中转表残留 ${fmtNum(m.staging_rows, 0)} 行 / ${fmtNum(m.staging_uscs, 0)} 只证券` },
    { title: '积压天数', value: `${fmtNum(m.backlog_days, 2)} 天`, icon: 'mdi-calendar-clock', hint: 'backlog_days，超过 7 天告警' },
    { title: '预算因子', value: fmtNum(m.budget_factor, 2), icon: 'mdi-speedometer', hint: 'clamp(1.20 ~ 2.00)' },
    { title: '今日预算行数', value: fmtNum(m.budget_rows, 0), icon: 'mdi-database-export', hint: `已消耗 ${fmtNum(m.budget_consumed_rows, 0)} 行` },
    { title: 'FROZEN 缺失行数', value: fmtNum(m.frozen_missing_rows, 0), icon: 'mdi-database-alert', hint: `占比 ${fmtPct(m.frozen_missing_ratio * 100)}（期望 ${fmtNum(m.frozen_expected_rows, 0)} 行）` },
    { title: '近 7 天补丁数', value: fmtNum(m.patch_last_7_days, 0), icon: 'mdi-file-document-edit', hint: `高频补丁证券月 ${fmtNum(m.high_frequency_patch_groups, 0)} 个` },
    { title: '心跳超时任务', value: fmtNum(m.heartbeat_timeouts, 0), icon: 'mdi-heart-pulse', hint: 'RUNNING 但心跳 / 租约超时' },
    { title: '上传重试耗尽', value: fmtNum(m.upload_retry_exhausted, 0), icon: 'mdi-cloud-alert', hint: '需人工介入或重试上传' },
    { title: '待清理本地文件', value: fmtNum(m.local_files_pending_cleanup, 0), icon: 'mdi-delete-clock', hint: '已上传但未清理（需网盘已归档）' },
    { title: 'MVsv 结构异常', value: fmtNum(m.mvsv_anomalies, 0), icon: 'mdi-file-alert', hint: '文件头 / 空行 / Count 抽检异常' },
  ]
})

const budgetPercent = computed(() => {
  const m = metrics.value
  if (!m) return 0
  return Math.min(100, Math.max(0, m.budget_usage_ratio * 100))
})

const jobStatusRows = computed(() => {
  const map = metrics.value?.job_status ?? {}
  return JOB_STATUS_ORDER.map((status) => ({
    status,
    name: JOB_STATUS_NAME[status] ?? status,
    color: JOB_STATUS_COLOR[status] ?? 'grey',
    count: map[status] ?? 0,
  }))
})

const uploadStatusRows = computed(() => {
  const map = metrics.value?.upload_status ?? {}
  return UPLOAD_STATUS_ORDER.map((status) => ({
    status,
    name: UPLOAD_STATUS_NAME[status] ?? status,
    color: UPLOAD_STATUS_COLOR[status] ?? 'grey',
    count: map[status] ?? 0,
  }))
})

const jobHeaders = [
  { title: 'Job 标识', key: 'job_kind', sortable: false },
  { title: '调度', key: 'schedule', sortable: false },
  { title: '窗口内', key: 'windowed', width: 90, sortable: false },
  { title: '操作', key: 'actions', width: 110, sortable: false },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await apiGet<StatusData>('/Meta/Finv/Quote/DeDup/Status')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// 手动触发一个 Job（异步执行，结果刷新指标与任务页观察）
async function runJob(kind: string) {
  runningKind.value = kind
  try {
    await apiPost<{ job_kind: string }>('/Meta/Finv/Quote/DeDup/Job/Run', { job_kind: kind })
    toastText.value = `已触发 ${kind}，异步执行中`
    toast.value = true
    await load()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    runningKind.value = ''
  }
}

function setupTimer() {
  clearTimer()
  if (autoRefresh.value) {
    timer = setInterval(load, 30000)
  }
}

function clearTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(async () => {
  await load()
  setupTimer()
})

onBeforeUnmount(clearTimer)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-monitor-dashboard" class="mr-2" />
      月归档导出总览
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-3">
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-switch
            v-model="autoRefresh"
            label="自动刷新（30 秒）"
            density="compact"
            hide-details
            color="primary"
            @update:model-value="setupTimer"
          />
        </v-col>
        <v-col cols="12" md="auto" class="ml-md-auto text-medium-emphasis text-caption">
          <span v-if="metrics">指标生成时间：{{ fmtISO(metrics.generated_at) }}</span>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-row v-if="options" dense class="mb-2">
        <v-col cols="12">
          <v-chip
            class="mr-2 mb-2"
            :color="options.enabled ? 'success' : 'grey'"
            variant="tonal"
            prepend-icon="mdi-power"
          >
            调度{{ options.enabled ? '已启用' : '已停用' }}
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-account-cog">
            worker：{{ options.worker_id }}
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-cloud-upload">
            上传方式：{{ options.upload_mode }}
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-clock-outline">
            低峰窗口：{{ options.window_start }} ~ {{ options.window_end }}（{{ options.time_zone }}）
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-folder-outline">
            输出目录：{{ options.output_dir }}
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-zip-box">
            压缩：{{ options.compression }}
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-view-sequential">
            分页：{{ options.page_size }} 行/页
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-check-decagram">
            成熟度阈值：{{ options.maturity_threshold }}
          </v-chip>
        </v-col>
      </v-row>

      <v-alert
        v-for="alert in alerts"
        :key="alert.code + alert.message"
        :type="ALERT_TYPE[alert.level] ?? 'info'"
        density="compact"
        variant="tonal"
        class="mb-2"
      >
        <strong>{{ alert.code }}</strong>：{{ alert.message }}
      </v-alert>
      <v-alert v-if="metrics && alerts.length === 0" type="success" density="compact" variant="tonal" class="mb-2">
        当前无告警
      </v-alert>

      <v-row v-if="metrics" dense>
        <v-col v-for="card in cards" :key="card.title" cols="12" sm="6" md="4" lg="3">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="d-flex align-center text-medium-emphasis text-caption mb-1">
              <v-icon :icon="card.icon" size="16" class="mr-1" />
              {{ card.title }}
            </div>
            <div class="text-h6">{{ card.value }}</div>
            <div class="text-caption text-medium-emphasis">{{ card.hint }}</div>
          </v-sheet>
        </v-col>
      </v-row>

      <v-row v-if="metrics" dense class="mt-2">
        <v-col cols="12" md="6">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">预算使用率</div>
            <v-progress-linear
              :model-value="budgetPercent"
              :color="budgetPercent >= 90 ? 'error' : budgetPercent >= 70 ? 'warning' : 'primary'"
              height="16"
              rounded
            >
              <template #default>
                <span class="text-caption">{{ fmtPct(budgetPercent) }}</span>
              </template>
            </v-progress-linear>
            <div class="text-caption text-medium-emphasis mt-2">
              预算 {{ fmtNum(metrics.budget_rows, 0) }} 行 / 已消耗 {{ fmtNum(metrics.budget_consumed_rows, 0) }} 行
            </div>
          </v-sheet>
        </v-col>
        <v-col cols="12" md="6">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">FROZEN 完整性</div>
            <v-progress-linear
              :model-value="Math.min(100, Math.max(0, 100 - metrics.frozen_missing_ratio * 100))"
              :color="metrics.frozen_missing_ratio > 0.05 ? 'error' : 'success'"
              height="16"
              rounded
            >
              <template #default>
                <span class="text-caption">缺失 {{ fmtPct(metrics.frozen_missing_ratio * 100) }}</span>
              </template>
            </v-progress-linear>
            <div class="text-caption text-medium-emphasis mt-2">
              期望 {{ fmtNum(metrics.frozen_expected_rows, 0) }} 行 / 缺失 {{ fmtNum(metrics.frozen_missing_rows, 0) }} 行
            </div>
          </v-sheet>
        </v-col>
      </v-row>

      <v-row v-if="metrics" dense class="mt-2">
        <v-col cols="12" md="6">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">任务状态分布</div>
            <div class="d-flex flex-wrap">
              <v-chip
                v-for="row in jobStatusRows"
                :key="row.status"
                :color="row.color"
                variant="tonal"
                class="mr-2 mb-2"
              >
                {{ row.name }}：{{ row.count }}
              </v-chip>
            </div>
          </v-sheet>
        </v-col>
        <v-col cols="12" md="6">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">文件上传状态分布</div>
            <div class="d-flex flex-wrap">
              <v-chip
                v-for="row in uploadStatusRows"
                :key="row.status"
                :color="row.color"
                variant="tonal"
                class="mr-2 mb-2"
              >
                {{ row.name }}：{{ row.count }}
              </v-chip>
            </div>
          </v-sheet>
        </v-col>
      </v-row>

      <v-sheet v-if="data" border rounded class="pa-3 mt-4">
        <div class="text-subtitle-2 mb-2">Job 调度表（可手动触发）</div>
        <v-data-table
          :headers="jobHeaders"
          :items="data.jobs"
          item-value="job_kind"
          density="compact"
          hide-default-footer
        >
          <template #item.job_kind="{ item }">
            <v-tooltip location="top" max-width="480" open-delay="150">
              <template #activator="{ props }">
                <div v-bind="props">
                  <div class="text-body-2">{{ item.name || item.job_kind }}</div>
                  <div class="text-caption text-medium-emphasis">{{ item.job_kind }}</div>
                </div>
              </template>
              <div class="tooltip-doc text-caption">
                <div class="font-weight-bold mb-1">{{ item.name }}（{{ item.job_kind }}）</div>
                <div class="mb-1">调度：{{ item.schedule }}</div>
                <div class="mb-1">功能：{{ item.summary }}</div>
                <div class="mb-1">流程：{{ item.flow }}</div>
                <div class="mb-1">工作细节：{{ item.details }}</div>
                <div>注意事项：{{ item.cautions }}</div>
              </div>
            </v-tooltip>
          </template>
          <template #item.schedule="{ item }">{{ item.schedule }}</template>
          <template #item.windowed="{ item }">
            <v-chip size="small" :color="item.windowed ? 'primary' : 'grey'" variant="tonal">
              {{ item.windowed ? '是' : '否' }}
            </v-chip>
          </template>
          <template #item.actions="{ item }">
            <v-btn
              size="small"
              variant="text"
              color="primary"
              prepend-icon="mdi-play"
              :loading="runningKind === item.job_kind"
              @click="runJob(item.job_kind)"
            >
              运行
            </v-btn>
          </template>
        </v-data-table>
      </v-sheet>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>数据来源：GET /API/V1/Meta/Finv/Quote/DeDup/Status（指标、运行参数与 Job 调度表）。</div>
          <div class="mt-1">
            指标来自 finv_dedup_export_job / finv_dedup_export_file / finv_dedup_daily_budget 等表聚合；
            「运行」按钮调用 POST /Job/Run 异步触发，结果请到「导出任务」「文件与上传」页观察。
          </div>
        </v-card-text>
      </v-card>
    </v-card-text>

    <v-snackbar v-model="toast" :timeout="3000" color="success">{{ toastText }}</v-snackbar>
  </v-card>
</template>

<style scoped>
.tooltip-doc {
  white-space: normal;
  text-align: left;
  line-height: 1.5;
}
</style>
