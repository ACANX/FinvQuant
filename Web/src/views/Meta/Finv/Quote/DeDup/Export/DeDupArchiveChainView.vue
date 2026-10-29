<script setup lang="ts">
// 端到端链路：以数据流向图展示「主表 → 成熟度/预算 → 任务领取 → LOAD → EXPORT → 登记 → CLEANUP → 上传 → 网盘 → 清理」全链路
// 支持总体视角（步骤、卡点、数据流）与任务视角（10 个 Job 的说明、状态与所在环节）
// 数据来源：GET /Status（业务指标）、GET /Job/Runtime（Job 阶段/进度/心跳）、GET /Settings（低峰窗口）
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPut } from '../../../../../../api'
import { fmtISO, fmtPct } from '../../../../../../utils'

interface Alert {
  code: string
  level: string
  message: string
}

interface JobProgress {
  total: number
  success: number
  failed: number
  running: number
  waiting: number
  paused: number
  frozen: number
  loading: number
  loaded: number
  exporting: number
  patch_total: number
  patch_success: number
  patch_failed: number
  patch_running: number
  patch_pending: number
}

interface Metrics {
  generated_at: string
  job_status: Record<string, number>
  job_progress: JobProgress
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
  upload_mode: string
  window_start: string
  window_end: string
  compression: string
  maturity_threshold: number
}

interface StatusData {
  metrics: Metrics
  options: StatusOptions
}

interface RunLogRow {
  run_id: number
  job_kind: string
  gmt_start: string
  gmt_finish: string | null
  status: string | null
  processed_rows: number | null
  processed_bytes: number | null
  error: string | null
  stage: string | null
  progress_percent: number | null
  progress_detail: string | null
  gmt_heartbeat: string | null
}

interface JobRuntimeState {
  job_kind: string
  name: string
  summary: string
  flow: string
  details: string
  cautions: string
  schedule: string
  windowed: boolean
  in_process: boolean
  stale: boolean
  elapsed_seconds: number
  heartbeat_age_seconds: number | null
  running?: RunLogRow
  last?: RunLogRow
  success_24h: number
  failed_24h: number
}

interface RuntimeData {
  generated_at: string
  stale_after_seconds: number
  job_progress: JobProgress
  jobs: JobRuntimeState[]
}

interface SettingsData {
  window_start: string
  window_end: string
  source: string
  updated_at: string | null
  time_zone: string
  in_window: boolean
}

type FlowState = 'running' | 'waiting' | 'stale' | 'error' | 'warning' | 'ok' | 'idle'

interface FlowNumber {
  label: string
  value: string
}

interface ChainStep {
  key: string
  no: string
  kind: 'main' | 'branch'
  title: string
  icon: string
  hint: string
  dataFlow: string
  jobs: string[]
  state: FlowState
  stateText: string
  progress: number
  progressText: string
  numbers: FlowNumber[]
  error?: string
  last?: RunLogRow
  activeStage?: string
  elapsedSeconds?: number
  heartbeatAgeSeconds?: number
}

interface JobSummary {
  state: FlowState
  last?: RunLogRow
  progress: number
  activeStage?: string
  elapsedSeconds?: number
  heartbeatAgeSeconds?: number
}

const STATE_META: Record<FlowState, { text: string; color: string }> = {
  running: { text: '执行中', color: 'primary' },
  waiting: { text: '等待中', color: 'info' },
  stale: { text: '心跳超时', color: 'warning' },
  error: { text: '异常', color: 'error' },
  warning: { text: '需关注', color: 'warning' },
  ok: { text: '正常', color: 'success' },
  idle: { text: '未运行', color: 'grey' },
}

const ALERT_TYPE: Record<string, 'info' | 'warning' | 'error'> = {
  INFO: 'info',
  WARN: 'warning',
  ERROR: 'error',
}

const STAGE_NAME: Record<string, string> = {
  RUNNING: '执行中',
  STATS: '入口统计',
  BUDGET: '预算计算',
  MATURITY: '成熟度扫描',
  SEAL: '收口冻结',
  LOAD: '装载',
  EXPORT: '导出',
  CLEANUP: '清理',
  UPLOAD: '上传',
  PATCH: '补丁',
  RECOVER: '超时回收',
  HEALTH: '健康检查',
  DONE: '已完成',
  FAILED: '失败',
  PAUSED: '已暂停',
}

const data = ref<StatusData | null>(null)
const runtime = ref<RuntimeData | null>(null)
const settings = ref<SettingsData | null>(null)
const statusError = ref('')
const runtimeError = ref('')
const loading = ref(false)
const autoRefresh = ref(true)
const viewMode = ref<'overall' | 'job'>('overall')
const selectedStep = ref('')
const selectedJob = ref('')

const settingsOpen = ref(false)
const windowStart = ref('00:30:00')
const windowEnd = ref('06:30:00')
const savingWindow = ref(false)
const settingsError = ref('')
const toast = ref(false)
const toastText = ref('')
const toastColor = ref('success')

let statusTimer: ReturnType<typeof setInterval> | null = null
let runtimeTimer: ReturnType<typeof setInterval> | null = null

const metrics = computed<Metrics | null>(() => data.value?.metrics ?? null)
const options = computed<StatusOptions | null>(() => data.value?.options ?? null)
const alerts = computed<Alert[]>(() => metrics.value?.alerts ?? [])
const runtimeByKind = computed(() => {
  const map = new Map<string, JobRuntimeState>()
  for (const job of runtime.value?.jobs ?? []) {
    map.set(job.job_kind, job)
  }
  return map
})

function jobOf(kind: string): JobRuntimeState | undefined {
  return runtimeByKind.value.get(kind)
}

function jobDoc(kind: string): JobRuntimeState | undefined {
  return runtimeByKind.value.get(kind)
}

function jobName(kind: string): string {
  return jobDoc(kind)?.name || kind
}

function percentOf(row: RunLogRow | undefined): number {
  const value = row?.progress_percent
  if (value === null || value === undefined) return -1
  return Math.min(100, Math.max(-1, value))
}

function timeOf(row: RunLogRow | undefined): number {
  if (!row) return 0
  const value = new Date(row.gmt_start).getTime()
  return Number.isNaN(value) ? 0 : value
}

// 合并一个或多个 Job 的运行快照：执行中 > 心跳超时 > 失败 > 暂停 > 正常 > 未运行
function summarizeJobs(kinds: string[]): JobSummary {
  const items = kinds.map(jobOf).filter((job): job is JobRuntimeState => Boolean(job))
  if (!items.length) return { state: 'idle', progress: -1 }

  const running = items.find((job) => job.in_process)
  if (running) {
    return {
      state: 'running',
      last: running.last,
      progress: percentOf(running.running),
      activeStage: running.running?.stage ?? undefined,
      elapsedSeconds: running.elapsed_seconds,
      heartbeatAgeSeconds: running.heartbeat_age_seconds ?? undefined,
    }
  }
  const stale = items.find((job) => job.stale)
  if (stale) {
    return { state: 'stale', last: stale.last, progress: -1, activeStage: stale.running?.stage ?? undefined }
  }
  const failed = items.find((job) => job.last?.status === 'FAILED')
  if (failed) return { state: 'error', last: failed.last, progress: -1 }
  const paused = items.find((job) => job.last?.status === 'PAUSED')
  if (paused) return { state: 'warning', last: paused.last, progress: -1 }
  const latest = items.filter((job) => job.last).sort((a, b) => timeOf(b.last) - timeOf(a.last))[0]
  if (latest) return { state: 'ok', last: latest.last, progress: -1 }
  return { state: 'idle', progress: -1 }
}

// 同一 Job 内部按阶段（LOAD / EXPORT / CLEANUP）细化状态：更早阶段视为已完成，更晚阶段视为未开始。
function stageNode(kind: string, stage: string): JobSummary {
  const job = jobOf(kind)
  if (!job) return { state: 'idle', progress: -1 }
  const stageRank = (value: string) => ['LOAD', 'EXPORT', 'CLEANUP'].indexOf(value)
  if (job.stale) {
    const current = job.running?.stage ?? ''
    const currentRank = stageRank(current)
    const base = { last: job.last, progress: -1, activeStage: current || undefined }
    if (currentRank < 0) return { ...base, state: stage === 'LOAD' ? 'stale' : 'idle' }
    if (currentRank === stageRank(stage)) return { ...base, state: 'stale' }
    if (currentRank > stageRank(stage)) return { ...base, state: 'ok' }
    return { ...base, state: 'idle' }
  }
  if (job.in_process) {
    const current = job.running?.stage ?? ''
    const base = {
      last: job.last,
      elapsedSeconds: job.elapsed_seconds,
      heartbeatAgeSeconds: job.heartbeat_age_seconds ?? undefined,
    }
    if (current === stage) {
      return { ...base, state: 'running', progress: percentOf(job.running), activeStage: current }
    }
    const currentRank = stageRank(current)
    if (currentRank >= 0 && currentRank > stageRank(stage)) {
      return { ...base, state: 'ok', progress: -1, activeStage: current }
    }
    return { ...base, state: 'waiting', progress: -1, activeStage: current || undefined }
  }
  if (job.last?.status === 'FAILED') {
    const failedStage = job.last.stage ?? ''
    const failedRank = stageRank(failedStage)
    if (failedRank < 0) return { state: stage === 'LOAD' ? 'error' : 'idle', last: job.last, progress: -1 }
    if (failedRank === stageRank(stage)) return { state: 'error', last: job.last, progress: -1 }
    if (failedRank > stageRank(stage)) return { state: 'ok', last: job.last, progress: -1 }
    return { state: 'idle', progress: -1 }
  }
  if (job.last?.status === 'PAUSED') {
    const pausedStage = job.last.stage ?? ''
    const pausedRank = stageRank(pausedStage)
    if (pausedRank < 0) return { state: stage === 'LOAD' ? 'warning' : 'idle', last: job.last, progress: -1 }
    if (pausedRank === stageRank(stage)) return { state: 'warning', last: job.last, progress: -1 }
    if (pausedRank > stageRank(stage)) return { state: 'ok', last: job.last, progress: -1 }
    return { state: 'idle', progress: -1 }
  }
  if (job.last) return { state: 'ok', last: job.last, progress: -1 }
  return { state: 'idle', progress: -1 }
}

function summaryText(summary: JobSummary): string {
  const stageText = summary.activeStage ? `阶段：${STAGE_NAME[summary.activeStage] ?? summary.activeStage}` : ''
  const percentText = summary.progress >= 0 && summary.state === 'running' ? ` · ${fmtPct(summary.progress)}` : ''
  return `${stageText}${percentText}`
}

function errorOf(summary: JobSummary, fallback = ''): string {
  if (summary.state === 'stale') return '心跳超时，任务可能卡住'
  if (summary.state === 'error') return summary.last?.error || '上次执行失败'
  return fallback
}

function buildStep(
  base: Pick<ChainStep, 'key' | 'no' | 'kind' | 'title' | 'icon' | 'hint' | 'dataFlow' | 'jobs' | 'numbers'>,
  summary: JobSummary,
  override: Partial<ChainStep> = {},
): ChainStep {
  const state = override.state ?? summary.state
  return {
    ...base,
    state,
    stateText: override.stateText ?? STATE_META[state].text,
    progress: override.progress ?? summary.progress,
    progressText: override.progressText ?? summaryText(summary),
    error: override.error ?? errorOf(summary),
    last: summary.last,
    activeStage: summary.activeStage,
    elapsedSeconds: summary.elapsedSeconds,
    heartbeatAgeSeconds: summary.heartbeatAgeSeconds,
  }
}

const steps = computed<ChainStep[]>(() => {
  const m = metrics.value
  if (!m) return []
  const uploadStatus = m.upload_status ?? {}
  const count = (map: Record<string, number>, key: string) => map[key] ?? 0
  const batch: JobProgress = runtime.value?.job_progress ?? m.job_progress

  const uploadTotal =
    count(uploadStatus, 'PENDING_UPLOAD') +
    count(uploadStatus, 'UPLOADING') +
    count(uploadStatus, 'UPLOADED') +
    count(uploadStatus, 'UPLOAD_FAILED')
  const uploadedPercent = uploadTotal > 0 ? (count(uploadStatus, 'UPLOADED') / uploadTotal) * 100 : -1
  const loadDone = batch.loaded + batch.exporting + batch.success
  const loadTotal = loadDone + batch.loading
  const loadPercent = loadTotal > 0 ? (loadDone / loadTotal) * 100 : -1
  const exportDone = batch.success
  const exportTotal = exportDone + batch.exporting
  const exportPercent = exportTotal > 0 ? (exportDone / exportTotal) * 100 : -1

  const maturity = summarizeJobs(['REFRESH_QUOTE_MATURITY'])
  const maturityState: FlowState = m.backlog_days > 7 ? 'warning' : maturity.state
  const budget = summarizeJobs(['CALC_DAILY_INGEST_STATS', 'CALC_DAILY_EXPORT_BUDGET'])
  const budgetState: FlowState = m.budget_usage_ratio >= 0.9 ? 'warning' : budget.state
  const exportJob = summarizeJobs(['RUN_DUE_EXPORT_JOBS'])
  const load = stageNode('RUN_DUE_EXPORT_JOBS', 'LOAD')
  const exportStage = stageNode('RUN_DUE_EXPORT_JOBS', 'EXPORT')
  const cleanup = stageNode('RUN_DUE_EXPORT_JOBS', 'CLEANUP')
  const upload = summarizeJobs(['UPLOAD_PENDING_EXPORT_FILES'])
  const uploadState: FlowState =
    m.upload_retry_exhausted > 0
      ? 'error'
      : count(uploadStatus, 'UPLOAD_FAILED') > 0
        ? 'warning'
        : upload.state
  const archive: JobSummary = {
    state: m.local_files_pending_cleanup > 0 ? 'waiting' : 'ok',
    progress: -1,
  }
  const artifact = summarizeJobs(['CLEAN_EXPIRED_EXPORT_ARTIFACTS'])
  const artifactState: FlowState = m.mvsv_anomalies > 0 ? 'warning' : artifact.state
  const patch = summarizeJobs(['SCAN_POST_SEAL_PATCHES'])
  const patchState: FlowState = m.high_frequency_patch_groups > 0 ? 'warning' : patch.state
  const recover = summarizeJobs(['RECOVER_TIMEOUT_EXPORT_JOBS'])
  const recoverState: FlowState = m.heartbeat_timeouts > 0 ? 'warning' : recover.state
  const health = summarizeJobs(['CHECK_EXPORT_JOB_HEALTH'])
  const healthState: FlowState = alerts.value.length > 0 ? 'warning' : health.state

  return [
    buildStep(
      {
        key: 'source', no: '①', kind: 'main', title: '去重主表扫描',
        icon: 'mdi-database-search', hint: '扫描 gmt_create / gmt_update 水位，刷新成熟度并自动发现新 usc',
        dataFlow: 'cdeu.finv_quote_secu_kline_min → 成熟度 / 新 usc',
        jobs: ['REFRESH_QUOTE_MATURITY'],
        numbers: [
          { label: '待装载行数', value: fmtCount(m.pending_rows, 0) },
          { label: '积压天数', value: `${fmtCount(m.backlog_days, 2)} 天` },
        ],
      },
      maturity,
      {
        state: maturityState,
        error: maturityState === 'warning' ? `积压 ${fmtCount(m.backlog_days, 2)} 天（超过 7 天阈值）` : undefined,
      },
    ),
    buildStep(
      {
        key: 'usc', no: '②', kind: 'main', title: '证券主数据补全',
        icon: 'mdi-account-details', hint: '自动发现新 usc，补全 region / market / provider / name 等文件头元数据',
        dataFlow: 'cdeu.finv_quote_cdeu_usc（新 usc 与文件头元数据）',
        jobs: ['REFRESH_QUOTE_MATURITY'],
        numbers: [
          { label: '最近处理行数', value: fmtCount(maturity.last?.processed_rows, 0) },
          { label: '成熟度阈值', value: fmtPct((options.value?.maturity_threshold ?? 0) * 100) },
        ],
      },
      maturity,
    ),
    buildStep(
      {
        key: 'budget', no: '③', kind: 'main', title: '入口统计与预算',
        icon: 'mdi-calculator-variant', hint: '聚合近 30 天入口流量，计算当日预算与硬上限',
        dataFlow: 'cdeu.finv_quote_cdeu_ingest_stats / cdeu.finv_quote_cdeu_daily_budget',
        jobs: ['CALC_DAILY_INGEST_STATS', 'CALC_DAILY_EXPORT_BUDGET'],
        numbers: [
          { label: '预算因子', value: fmtCount(m.budget_factor, 2) },
          { label: '预算 / 已消耗', value: `${fmtCount(m.budget_rows, 0)} / ${fmtCount(m.budget_consumed_rows, 0)}` },
        ],
      },
      budget,
      {
        state: budgetState,
        progress: budget.state === 'running' ? budget.progress : m.budget_usage_ratio * 100,
        progressText: `预算使用率 ${fmtPct(m.budget_usage_ratio * 100)}`,
        error: budgetState === 'warning' ? '预算使用率超过 90%，导出可能随时暂停' : undefined,
      },
    ),
    buildStep(
      {
        key: 'dispatch', no: '④', kind: 'main', title: '导出任务领取',
        icon: 'mdi-format-list-checks', hint: '按优先级领取到期证券月任务，受预算与低峰窗口约束',
        dataFlow: 'cdeu.finv_quote_cdeu_export_job（按优先级领取 job_id）',
        jobs: ['RUN_DUE_EXPORT_JOBS'],
        numbers: [
          { label: '待执行任务', value: fmtCount(batch.waiting, 0) },
          { label: '装载中任务', value: fmtCount(batch.loading, 0) },
        ],
      },
      exportJob,
      {
        progress:
          batch.total > 0 ? ((batch.success + batch.failed) / batch.total) * 100 : exportJob.progress,
        progressText: exportJob.state === 'running' ? `阶段：${STAGE_NAME[exportJob.activeStage ?? ''] ?? '执行中'}` : '',
      },
    ),
    buildStep(
      {
        key: 'load', no: '⑤', kind: 'main', title: 'LOAD 装载',
        icon: 'mdi-database-import', hint: '主表按日 + ts keyset 分页装入中转表（短事务、可暂停续跑）',
        dataFlow: 'cdeu.finv_quote_secu_kline_min → export.finv_quote_secu_kline_min',
        jobs: ['RUN_DUE_EXPORT_JOBS'],
        numbers: [
          { label: '装载中', value: fmtCount(batch.loading, 0) },
          { label: '已装载', value: fmtCount(batch.loaded, 0) },
        ],
      },
      load,
      {
        progress: loadPercent >= 0 ? loadPercent : load.progress,
        progressText:
          load.state === 'running'
            ? summaryText(load)
            : load.state === 'waiting'
              ? `当前阶段：${stageName(load.activeStage)}`
              : '',
      },
    ),
    buildStep(
      {
        key: 'export', no: '⑥', kind: 'main', title: 'EXPORT 导出',
        icon: 'mdi-file-export', hint: '中转表流式写出 .mvsv（16 行头 + 空行 + 数据行），校验 Count 与 SHA256',
        dataFlow: 'export.finv_quote_secu_kline_min → 本地 .mvsv 文件',
        jobs: ['RUN_DUE_EXPORT_JOBS'],
        numbers: [
          { label: '导出中', value: fmtCount(batch.exporting, 0) },
          { label: '已导出', value: fmtCount(batch.success, 0) },
        ],
      },
      exportStage,
      {
        progress: exportPercent >= 0 ? exportPercent : exportStage.progress,
        progressText:
          exportStage.state === 'running'
            ? summaryText(exportStage)
            : exportStage.state === 'waiting'
              ? `当前阶段：${stageName(exportStage.activeStage)}`
              : '',
      },
    ),
    buildStep(
      {
        key: 'register', no: '⑦', kind: 'main', title: '导出文件登记',
        icon: 'mdi-file-document-check', hint: '登记 BASE / MANIFEST 到导出文件表，作为上传任务唯一来源',
        dataFlow: 'cdeu.finv_quote_cdeu_export_file（BASE / MANIFEST）',
        jobs: ['RUN_DUE_EXPORT_JOBS'],
        numbers: [
          { label: '已登记任务', value: fmtCount(batch.success, 0) },
          { label: '待上传文件', value: fmtCount(count(uploadStatus, 'PENDING_UPLOAD'), 0) },
        ],
      },
      exportStage,
      {
        stateText: exportStage.state === 'running' ? '导出中登记' : undefined,
        progress: exportPercent >= 0 ? exportPercent : exportStage.progress,
        progressText:
          exportStage.state === 'running'
            ? '随 EXPORT 阶段写入登记记录'
            : exportStage.state === 'waiting'
              ? `当前阶段：${stageName(exportStage.activeStage)}`
              : '',
      },
    ),
    buildStep(
      {
        key: 'cleanup', no: '⑧', kind: 'main', title: 'CLEANUP 中转清理',
        icon: 'mdi-delete-sweep', hint: '按 usc + 日期范围删除中转数据；空表执行 TRUNCATE',
        dataFlow: 'export.finv_quote_secu_kline_min（删除已导出数据）',
        jobs: ['RUN_DUE_EXPORT_JOBS'],
        numbers: [
          { label: '中转残留行', value: fmtCount(m.staging_rows, 0) },
          { label: '残留证券', value: `${fmtCount(m.staging_uscs, 0)} 只` },
        ],
      },
      cleanup,
      {
        progress: m.staging_rows === 0 ? 100 : -1,
        progressText:
          cleanup.state === 'running'
            ? summaryText(cleanup)
            : cleanup.state === 'waiting'
              ? `当前阶段：${stageName(cleanup.activeStage)}`
              : '',
      },
    ),
    buildStep(
      {
        key: 'upload', no: '⑨', kind: 'main', title: '文件上传',
        icon: 'mdi-cloud-upload', hint: '扫描待上传记录，上传独享存储库并校验远端 checksum',
        dataFlow: 'cdeu.finv_quote_cdeu_export_file → 独享存储库（.mvsv）',
        jobs: ['UPLOAD_PENDING_EXPORT_FILES'],
        numbers: [
          { label: '待上传', value: fmtCount(count(uploadStatus, 'PENDING_UPLOAD'), 0) },
          { label: '上传失败', value: fmtCount(count(uploadStatus, 'UPLOAD_FAILED'), 0) },
        ],
      },
      upload,
      {
        state: uploadState,
        progress: upload.state === 'running' ? upload.progress : uploadedPercent,
        progressText:
          uploadTotal > 0
            ? `已上传 ${fmtCount(count(uploadStatus, 'UPLOADED'), 0)} / ${fmtCount(uploadTotal, 0)}`
            : '暂无待上传文件',
        error:
          m.upload_retry_exhausted > 0
            ? `上传重试耗尽 ${m.upload_retry_exhausted} 个文件`
            : count(uploadStatus, 'UPLOAD_FAILED') > 0
              ? `上传失败 ${count(uploadStatus, 'UPLOAD_FAILED')} 个文件`
              : undefined,
      },
    ),
    buildStep(
      {
        key: 'archive', no: '⑩', kind: 'main', title: '网盘归档',
        icon: 'mdi-folder-network', hint: '外部归档流程；完成后回写 network_status=ARCHIVED',
        dataFlow: '独享存储库 → 网盘（network_status=ARCHIVED）',
        jobs: [],
        numbers: [
          { label: '待清理本地文件', value: fmtCount(m.local_files_pending_cleanup, 0) },
          { label: '已上传文件', value: fmtCount(count(uploadStatus, 'UPLOADED'), 0) },
        ],
      },
      archive,
      {
        state: archive.state,
        stateText: archive.state === 'waiting' ? '等待网盘归档' : '正常',
        progressText: m.local_files_pending_cleanup > 0 ? '存在已上传但未完成网盘归档的文件' : '本地无待清理文件',
        error: m.local_files_pending_cleanup > 0 ? `待清理本地文件 ${m.local_files_pending_cleanup} 个（需确认网盘归档）` : undefined,
      },
    ),
    buildStep(
      {
        key: 'artifact', no: '⑪', kind: 'main', title: '生命周期清理',
        icon: 'mdi-delete-clock', hint: '清理过期本地文件与主表旧数据（需全链路成功确认）',
        dataFlow: '本地过期文件 / 主表旧月份 → 删除',
        jobs: ['CLEAN_EXPIRED_EXPORT_ARTIFACTS'],
        numbers: [
          { label: '待清理本地文件', value: fmtCount(m.local_files_pending_cleanup, 0) },
          { label: '上传重试耗尽', value: fmtCount(m.upload_retry_exhausted, 0) },
        ],
      },
      artifact,
      {
        state: artifactState,
        error: artifactState === 'warning' ? `MVsv 结构异常 ${m.mvsv_anomalies} 个` : undefined,
      },
    ),
    // 旁路：补丁 / 超时回收 / 健康检查
    buildStep(
      {
        key: 'patch', no: '⑫', kind: 'branch', title: '收口后补丁',
        icon: 'mdi-file-document-edit', hint: '增量/整月数据先分页装入 ⑤ 中转表，再经 ⑥ 导出、⑦ 登记、⑧ 清理后进入 ⑨ 上传（属于本月导出批次的一部分）',
        dataFlow: '收口数据 → cdeu.finv_quote_cdeu_export_file_patch → 补丁文件',
        jobs: ['SCAN_POST_SEAL_PATCHES'],
        numbers: [
          { label: '近 7 天补丁', value: fmtCount(m.patch_last_7_days, 0) },
          { label: '高频补丁证券月', value: fmtCount(m.high_frequency_patch_groups, 0) },
        ],
      },
      patch,
      {
        state: patchState,
        error: patchState === 'warning' ? `高频补丁证券月 ${m.high_frequency_patch_groups} 个` : undefined,
      },
    ),
    buildStep(
      {
        key: 'recover', no: '⑬', kind: 'branch', title: '超时任务回收',
        icon: 'mdi-backup-restore', hint: '回收心跳 / 租约超时的 RUNNING 任务与长时间上传',
        dataFlow: '超时任务 → 重新排队 / 失败重排',
        jobs: ['RECOVER_TIMEOUT_EXPORT_JOBS'],
        numbers: [
          { label: '心跳超时任务', value: fmtCount(m.heartbeat_timeouts, 0) },
          { label: '上传重试耗尽', value: fmtCount(m.upload_retry_exhausted, 0) },
        ],
      },
      recover,
      {
        state: recoverState,
        error: recoverState === 'warning' ? `存在心跳超时任务 ${m.heartbeat_timeouts} 个` : undefined,
      },
    ),
    buildStep(
      {
        key: 'health', no: '⑭', kind: 'branch', title: '健康检查与告警',
        icon: 'mdi-shield-check', hint: '汇总指标、抽检 MVsv 结构并输出告警',
        dataFlow: '指标 / 文件抽检 → 告警日志',
        jobs: ['CHECK_EXPORT_JOB_HEALTH'],
        numbers: [
          { label: '当前告警', value: fmtCount(alerts.value.length, 0) },
          { label: 'MVsv 异常', value: fmtCount(m.mvsv_anomalies, 0) },
        ],
      },
      health,
      {
        state: healthState,
        error: healthState === 'warning' ? `当前存在 ${alerts.value.length} 条告警` : undefined,
      },
    ),
  ]
})

const mainSteps = computed(() => steps.value.filter((step) => step.kind === 'main'))
const branchSteps = computed(() => steps.value.filter((step) => step.kind === 'branch'))
const problemSteps = computed(() =>
  steps.value.filter((step) => step.state === 'stale' || step.state === 'error' || step.state === 'warning'),
)
const runningSteps = computed(() => steps.value.filter((step) => step.state === 'running'))
const currentStep = computed<ChainStep | null>(() => {
  return steps.value.find((step) => step.key === selectedStep.value) ?? problemSteps.value[0] ?? mainSteps.value[0] ?? null
})
const jobList = computed<JobRuntimeState[]>(() => runtime.value?.jobs ?? [])
const currentJob = computed<JobRuntimeState | null>(() => {
  return jobOf(selectedJob.value) ?? jobList.value[0] ?? null
})

// ---- SVG 数据流向图 ----
const NODE_HEIGHT = 84
const NODE_GAP = 106
const GROUP_SHIFT = 64
const AFTER_GROUP_SHIFT = 24
const MAIN_X = 56
const MAIN_WIDTH = 340
const BRANCH_X = 470
const BRANCH_WIDTH = 300
const CHART_WIDTH = 850

// ⑤–⑧ 属于同一个「导出任务批次」大节点：在其上方预留批次标题与进度条空间。
function mainY(index: number): number {
  return 24 + index * NODE_GAP + (index >= 4 ? GROUP_SHIFT : 0) + (index >= 8 ? AFTER_GROUP_SHIFT : 0)
}

const STATE_CHART: Record<FlowState, { border: string; fill: string }> = {
  running: { border: '#1976D2', fill: 'rgba(25,118,210,0.18)' },
  waiting: { border: '#0288D1', fill: 'rgba(2,136,209,0.14)' },
  stale: { border: '#F9A825', fill: 'rgba(249,168,37,0.24)' },
  error: { border: '#D32F2F', fill: 'rgba(211,47,47,0.22)' },
  warning: { border: '#F9A825', fill: 'rgba(249,168,37,0.20)' },
  ok: { border: '#2E7D32', fill: 'rgba(46,125,50,0.16)' },
  idle: { border: '#9E9E9E', fill: 'rgba(158,158,158,0.14)' },
}

const chartHeight = computed(() => Math.max(mainY(mainSteps.value.length - 1) + NODE_HEIGHT + 60, 400))
const branchAnchor: Record<string, number> = { patch: 6, recover: 3, health: 10 }
const LOOP_STEPS = new Set(['dispatch', 'upload', 'recover', 'health'])

interface ChartNode {
  key: string
  x: number
  y: number
  width: number
  fill: string
  border: string
  selected: boolean
  warn: boolean
  no: string
  title: string
  stateText: string
  metricLine: string
  progress: number
  schedule: string
  tooltip: string
}

interface ChartEdge {
  id: string
  path: string
  color: string
  marker: string
  dashed: boolean
  label?: string
  labelX?: number
  labelY?: number
}

function truncateText(value: string, max: number): string {
  return value.length > max ? `${value.slice(0, max)}…` : value
}

function makeChartNode(step: ChainStep, x: number, y: number, width: number): ChartNode {
  const color = STATE_CHART[step.state]
  const selected = currentStep.value?.key === step.key
  const warn = step.state === 'error' || step.state === 'stale'
  const numbers = step.numbers.map((number) => `${number.label}：${number.value}`).join('；')
  const metrics = step.numbers.map((number) => `${number.label} ${number.value}`).join(' · ')
  const job = step.jobs.map(jobOf).find((item): item is JobRuntimeState => Boolean(item))
  const schedule = truncateText(job?.schedule ?? '', 16)
  const loop = LOOP_STEPS.has(step.key)
  const tooltip = `${step.no} ${step.title}\n状态：${step.stateText}${schedule ? ` · ${job?.schedule}` : ''}\n${step.hint}\n数据：${step.dataFlow}\n${numbers}${step.error ? `\n${step.error}` : ''}`
  return {
    key: step.key,
    x,
    y,
    width,
    fill: color.fill,
    border: color.border,
    selected,
    warn,
    no: step.no,
    title: `${loop ? '↻ ' : ''}${truncateText(step.title, 20)}`,
    stateText: step.stateText,
    metricLine: truncateText(metrics, width > 320 ? 30 : 22),
    progress: step.progress,
    schedule,
    tooltip,
  }
}

const svgNodes = computed<ChartNode[]>(() => {
  const nodes = mainSteps.value.map((step, index) => makeChartNode(step, MAIN_X, mainY(index), MAIN_WIDTH))
  for (const step of branchSteps.value) {
    nodes.push(makeChartNode(step, BRANCH_X, mainY(branchAnchor[step.key] ?? 0), BRANCH_WIDTH))
  }
  return nodes
})

// 导出任务批次大节点：包裹 ⑤ LOAD / ⑥ EXPORT / ⑦ 登记 / ⑧ CLEANUP，展示成功 / 失败 / 总数与分段进度条。
const exportGroup = computed(() => {
  const first = svgNodes.value.find((node) => node.key === 'load')
  const last = svgNodes.value.find((node) => node.key === 'cleanup')
  const patch = svgNodes.value.find((node) => node.key === 'patch')
  if (!first || !last) return null
  const progress = runtime.value?.job_progress ?? metrics.value?.job_progress
  // 批次进度 = 导出任务 + ⑫ 补丁任务（补丁同属本月导出批次）
  const total = (progress?.total ?? 0) + (progress?.patch_total ?? 0)
  const success = (progress?.success ?? 0) + (progress?.patch_success ?? 0)
  const failed = (progress?.failed ?? 0) + (progress?.patch_failed ?? 0)
  const running = (progress?.running ?? 0) + (progress?.patch_running ?? 0)
  const x = MAIN_X - 14
  const y = first.y - 58
  // ⑫ 补丁同属本月导出批次：批次框向右扩展到补丁节点。
  const right = Math.max(last.x + last.width, patch ? patch.x + patch.width : 0) + 14
  const width = right - x
  const height = last.y + NODE_HEIGHT + 16 - y
  const headerX = MAIN_X + MAIN_WIDTH / 2 + 18
  const barWidth = Math.max(160, BRANCH_X - headerX - 20)
  return {
    x,
    y,
    width,
    height,
    headerX,
    barY: y + 42,
    barWidth,
    total,
    success,
    failed,
    running,
    successW: total > 0 ? (barWidth * success) / total : 0,
    failedW: total > 0 ? (barWidth * failed) / total : 0,
    runningW: total > 0 ? (barWidth * running) / total : 0,
  }
})

function markerColor(name: string): string {
  return { grey: '#9CA3AF', blue: '#1976D2', orange: '#F9A825', red: '#D32F2F', loop: '#7C3AED' }[name] ?? '#9CA3AF'
}

function edgeMarker(state: FlowState): { color: string; marker: string } {
  if (state === 'error') return { color: '#D32F2F', marker: 'arrow-red' }
  if (state === 'stale' || state === 'warning') return { color: '#F9A825', marker: 'arrow-orange' }
  if (state === 'running') return { color: '#1976D2', marker: 'arrow-blue' }
  return { color: '#9CA3AF', marker: 'arrow-grey' }
}

const svgEdges = computed<ChartEdge[]>(() => {
  const edges: ChartEdge[] = []
  const main = mainSteps.value
  for (let index = 0; index < main.length - 1; index += 1) {
    const from = svgNodes.value.find((node) => node.key === main[index].key)
    const to = svgNodes.value.find((node) => node.key === main[index + 1].key)
    if (!from || !to) continue
    const x = from.x + from.width / 2
    const marker = edgeMarker(main[index + 1].state)
    edges.push({
      id: `${from.key}-${to.key}`,
      path: `M ${x} ${from.y + NODE_HEIGHT} L ${x} ${to.y - 4}`,
      color: marker.color,
      marker: marker.marker,
      dashed: false,
    })
  }
  for (const step of branchSteps.value) {
    const anchorStep = main[branchAnchor[step.key] ?? 0]
    if (!anchorStep) continue
    const from = svgNodes.value.find((node) => node.key === anchorStep.key)
    const to = svgNodes.value.find((node) => node.key === step.key)
    if (!from || !to) continue
    const x1 = from.x + from.width
    const y1 = from.y + NODE_HEIGHT / 2 - 10
    const x2 = to.x - 6
    const y2 = to.y + NODE_HEIGHT / 2 - 10
    const marker = edgeMarker(step.state)
    edges.push({
      id: `${from.key}-${to.key}`,
      path: `M ${x1} ${y1} L ${x2} ${y2}`,
      color: marker.color,
      marker: marker.marker,
      dashed: true,
    })
  }

  // 旁路任务的下游：补丁 → 上传；超时回收 → 重新排队；健康检查 → 告警反馈到调度
  const dispatchNode = svgNodes.value.find((node) => node.key === 'dispatch')
  const patchNode = svgNodes.value.find((node) => node.key === 'patch')
  const recoverNode = svgNodes.value.find((node) => node.key === 'recover')
  const healthNode = svgNodes.value.find((node) => node.key === 'health')

  const loadNode = svgNodes.value.find((node) => node.key === 'load')
  if (patchNode && loadNode) {
    const y1 = patchNode.y + 14
    const y2 = loadNode.y + NODE_HEIGHT / 2
    const laneX = patchNode.x - 40
    const x1 = patchNode.x - 4
    const x2 = loadNode.x + loadNode.width + 4
    edges.push({
      id: 'patch-load',
      path: `M ${x1} ${y1} L ${laneX} ${y1} L ${laneX} ${y2} L ${x2} ${y2}`,
      color: '#6B7280',
      marker: 'arrow-grey',
      dashed: true,
      label: '补丁数据入中转表',
      labelX: laneX + 8,
      labelY: (y1 + y2) / 2,
    })
  }
  if (recoverNode && dispatchNode) {
    const y = dispatchNode.y + NODE_HEIGHT / 2 + 12
    edges.push({
      id: 'recover-dispatch',
      path: `M ${recoverNode.x - 4} ${y} L ${dispatchNode.x + dispatchNode.width + 4} ${y}`,
      color: '#6B7280',
      marker: 'arrow-grey',
      dashed: true,
    })
  }
  if (healthNode && dispatchNode) {
    const x = CHART_WIDTH - 18
    const startX = healthNode.x + healthNode.width / 2
    const startY = healthNode.y + NODE_HEIGHT
    const bottomY = dispatchNode.y + NODE_HEIGHT + 16
    const entryY = dispatchNode.y + NODE_HEIGHT - 12
    const entryX = dispatchNode.x + dispatchNode.width + 4
    edges.push({
      id: 'health-dispatch',
      path: `M ${startX} ${startY} L ${startX} ${startY + 20} L ${x} ${startY + 20} L ${x} ${bottomY} L ${entryX} ${bottomY} L ${entryX} ${entryY} L ${dispatchNode.x + dispatchNode.width} ${entryY}`,
      color: '#6B7280',
      marker: 'arrow-grey',
      dashed: true,
    })
  }

  // 定时循环任务：从节点出口绕回入口（主链路节点画在左侧，旁路节点画在右侧）
  const loopNodes = new Set(['dispatch', 'upload', 'recover', 'health'])
  for (const node of svgNodes.value) {
    if (!loopNodes.has(node.key)) continue
    const isBranch = branchSteps.value.some((step) => step.key === node.key)
    // 自环规范：从节点底部中点（出口）出发，绕到节点外侧，再回到顶部中点（入口）。
    const centerX = node.x + node.width / 2
    const bottom = node.y + NODE_HEIGHT
    const top = node.y
    const offset = 8
    const laneX = isBranch ? node.x + node.width + 30 : node.x - 30
    edges.push({
      id: `loop-${node.key}`,
      path: `M ${centerX} ${bottom} L ${centerX} ${bottom + offset} L ${laneX} ${bottom + offset} L ${laneX} ${top - offset} L ${centerX} ${top - offset} L ${centerX} ${top}`,
      color: '#7C3AED',
      marker: 'arrow-loop',
      dashed: false,
    })
  }
  return edges
})

function stepsOfJob(kind: string): ChainStep[] {
  return steps.value.filter((step) => step.jobs.includes(kind))
}

function selectStep(key: string) {
  selectedStep.value = key
  viewMode.value = 'overall'
}

function selectJob(kind: string) {
  selectedJob.value = kind
  viewMode.value = 'job'
}

function stateMeta(state: FlowState) {
  return STATE_META[state]
}

function stageName(stage: string | null | undefined): string {
  if (!stage) return '-'
  return STAGE_NAME[stage] ?? stage
}

function fmtDuration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined || Number.isNaN(seconds)) return '-'
  const total = Math.max(0, Math.round(seconds))
  if (total < 60) return `${total} 秒`
  const minutes = Math.floor(total / 60)
  if (minutes < 60) return `${minutes} 分 ${total % 60} 秒`
  const hours = Math.floor(minutes / 60)
  return `${hours} 小时 ${minutes % 60} 分`
}

function fmtCount(value: number | null | undefined, digits = 0): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return Number(value).toLocaleString('en-US', { maximumFractionDigits: digits })
}

function runStatusName(status: string | null | undefined): string {
  return { SUCCESS: '成功', FAILED: '失败', PAUSED: '暂停' }[status ?? ''] ?? (status || '-')
}

function runStatusColor(status: string | null | undefined): string {
  return { SUCCESS: 'success', FAILED: 'error', PAUSED: 'warning' }[status ?? ''] ?? 'grey'
}

function jobStateText(job: JobRuntimeState): string {
  if (job.stale) return '心跳超时'
  if (job.in_process) return '执行中'
  const status = job.last?.status
  if (status === 'FAILED') return '上次失败'
  if (status === 'PAUSED') return '上次暂停'
  if (status === 'SUCCESS') return '上次成功'
  return '未执行'
}

function jobStateColor(job: JobRuntimeState): string {
  if (job.stale) return 'warning'
  if (job.in_process) return 'primary'
  const status = job.last?.status
  if (status === 'FAILED') return 'error'
  if (status === 'PAUSED') return 'warning'
  if (status === 'SUCCESS') return 'success'
  return 'grey'
}

function stepLastText(step: ChainStep): string {
  if (!step.last) return '暂无运行记录'
  return `${runStatusName(step.last.status)} · ${fmtISO(step.last.gmt_finish)}`
}

async function loadStatus() {
  loading.value = true
  statusError.value = ''
  try {
    data.value = await apiGet<StatusData>('/Meta/Finv/Quote/DeDup/Status')
  } catch (e) {
    statusError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function loadRuntime() {
  runtimeError.value = ''
  try {
    runtime.value = await apiGet<RuntimeData>('/Meta/Finv/Quote/DeDup/Job/Runtime')
  } catch (e) {
    runtimeError.value = (e as Error).message
  }
}

async function loadSettings() {
  try {
    settings.value = await apiGet<SettingsData>('/Meta/Finv/Quote/DeDup/Settings')
  } catch {
    // 设置读取失败不阻断链路图
  }
}

async function refreshAll() {
  await Promise.all([loadStatus(), loadRuntime(), loadSettings()])
}

function openWindowDialog() {
  settingsError.value = ''
  windowStart.value = settings.value?.window_start ?? options.value?.window_start ?? '00:30:00'
  windowEnd.value = settings.value?.window_end ?? options.value?.window_end ?? '06:30:00'
  settingsOpen.value = true
}

async function saveWindow() {
  settingsError.value = ''
  savingWindow.value = true
  try {
    const updated = await apiPut<SettingsData>('/Meta/Finv/Quote/DeDup/Settings/Window', {
      window_start: windowStart.value,
      window_end: windowEnd.value,
    })
    settings.value = updated
    settingsOpen.value = false
    showToast(`低峰窗口已更新为 ${updated.window_start} ~ ${updated.window_end}，立即生效`, 'success')
    await refreshAll()
  } catch (e) {
    settingsError.value = (e as Error).message
  } finally {
    savingWindow.value = false
  }
}

function showToast(text: string, color = 'success') {
  toastText.value = text
  toastColor.value = color
  toast.value = true
}

function setupTimers() {
  clearTimers()
  if (!autoRefresh.value) return
  runtimeTimer = setInterval(loadRuntime, 5000)
  statusTimer = setInterval(() => {
    void loadStatus()
    void loadSettings()
  }, 20000)
}

function clearTimers() {
  if (runtimeTimer) {
    clearInterval(runtimeTimer)
    runtimeTimer = null
  }
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
}

onMounted(async () => {
  await refreshAll()
  setupTimers()
})

onBeforeUnmount(clearTimers)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center flex-wrap">
      <v-icon icon="mdi-transit-connection-variant" class="mr-2" />
      端到端链路
      <v-spacer />
      <v-btn-group density="compact" variant="tonal" class="mr-2">
        <v-btn :color="viewMode === 'overall' ? 'primary' : undefined" @click="viewMode = 'overall'">总体视角</v-btn>
        <v-btn :color="viewMode === 'job' ? 'primary' : undefined" @click="viewMode = 'job'">任务视角</v-btn>
      </v-btn-group>
      <v-btn color="primary" prepend-icon="mdi-refresh" :loading="loading" @click="refreshAll">刷新</v-btn>
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-2">
        <v-col cols="auto">
          <v-switch
            v-model="autoRefresh"
            label="自动刷新（运行快照 5 秒 / 业务指标 20 秒）"
            density="compact"
            hide-details
            color="primary"
            @update:model-value="setupTimers"
          />
        </v-col>
        <v-col cols="12" md="auto" class="ml-md-auto text-medium-emphasis text-caption">
          <div v-if="metrics">指标时间：{{ fmtISO(metrics.generated_at) }}</div>
          <div v-if="runtime">运行快照：{{ fmtISO(runtime.generated_at) }}</div>
        </v-col>
      </v-row>

      <v-alert v-if="statusError" type="error" density="compact" class="mb-2" closable @click:close="statusError = ''">
        {{ statusError }}
      </v-alert>
      <v-alert v-if="runtimeError" type="warning" density="compact" class="mb-2" closable @click:close="runtimeError = ''">
        运行快照查询失败：{{ runtimeError }}
      </v-alert>

      <v-row v-if="options || metrics" dense class="mb-1">
        <v-col cols="12">
          <v-chip
            v-if="options"
            class="mr-2 mb-2"
            :color="options.enabled ? 'success' : 'grey'"
            variant="tonal"
            prepend-icon="mdi-power"
          >
            调度{{ options.enabled ? '已启用' : '已停用' }}
          </v-chip>
          <v-chip v-if="options" class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-account-cog">
            worker：{{ options.worker_id }}
          </v-chip>
          <v-chip v-if="options" class="mr-1 mb-2" variant="tonal" prepend-icon="mdi-clock-outline">
            低峰窗口：{{ options.window_start }} ~ {{ options.window_end }}（{{ options.time_zone }}）
          </v-chip>
          <v-btn
            v-if="options"
            class="mr-2 mb-2"
            size="small"
            variant="text"
            prepend-icon="mdi-pencil"
            @click="openWindowDialog"
          >
            修改
          </v-btn>
          <v-chip
            v-if="settings"
            class="mr-2 mb-2"
            :color="settings.in_window ? 'success' : 'grey'"
            variant="tonal"
          >
            {{ settings.in_window ? '当前在窗口内' : '当前在窗口外' }} ·
            {{ settings.source === 'db' ? '在线设置' : '环境变量默认' }}
          </v-chip>
          <v-chip
            class="mr-2 mb-2"
            :color="problemSteps.length > 0 ? 'warning' : 'success'"
            variant="tonal"
            prepend-icon="mdi-alert-decagram"
          >
            链路卡点：{{ problemSteps.length }} 个环节
          </v-chip>
          <v-chip class="mr-2 mb-2" variant="tonal" prepend-icon="mdi-progress-clock">
            执行中：{{ runningSteps.length }} 个环节
          </v-chip>
          <v-chip
            class="mr-2 mb-2"
            :color="alerts.length > 0 ? 'warning' : 'success'"
            variant="tonal"
            prepend-icon="mdi-bell-alert"
          >
            告警：{{ alerts.length }} 条
          </v-chip>
        </v-col>
      </v-row>

      <!-- 总体视角：数据流向 + 卡点 + 步骤详情 + 旁路 -->
      <v-row v-show="viewMode === 'overall'" dense>
        <v-col cols="12" lg="7">
          <v-sheet border rounded class="pa-3">
            <div class="d-flex align-center flex-wrap mb-2">
              <div class="text-subtitle-2">总体视角 · 数据流向图</div>
              <v-spacer />
              <span class="text-caption text-medium-emphasis">滚轮缩放 / 拖拽平移 / 点击节点查看详情</span>
            </div>
            <div class="d-flex flex-wrap mb-2">
              <v-chip
                v-for="(meta, state) in STATE_META"
                :key="state"
                size="x-small"
                :color="meta.color"
                variant="tonal"
                class="mr-1 mb-1"
              >
                {{ meta.text }}
              </v-chip>
              <span class="text-caption text-medium-emphasis ml-2">
                红 / 黄边框为卡点环节 · 实线=主流程 · 虚线=旁路（⑫⑬⑭） · 紫色=定时循环
              </span>
            </div>
            <div class="chain-svg-wrap">
              <svg
                :viewBox="`0 0 ${CHART_WIDTH} ${chartHeight}`"
                class="chain-svg"
                role="img"
                aria-label="端到端数据流向图"
              >
                <defs>
                  <marker
                    v-for="name in ['grey', 'blue', 'orange', 'red', 'loop']"
                    :id="`arrow-${name}`"
                    :key="name"
                    viewBox="0 0 10 10"
                    refX="9"
                    refY="5"
                    markerWidth="7"
                    markerHeight="7"
                    orient="auto"
                  >
                    <path d="M 0 0 L 10 5 L 0 10 z" :fill="markerColor(name)" />
                  </marker>
                  <filter id="nodeShadow" x="-10%" y="-10%" width="120%" height="120%">
                    <feDropShadow dx="0" dy="2" stdDeviation="4" flood-color="#0F172A" flood-opacity="0.25" />
                  </filter>
                </defs>
                <g v-if="exportGroup" class="chain-group">
                  <rect
                    :x="exportGroup.x"
                    :y="exportGroup.y"
                    :width="exportGroup.width"
                    :height="exportGroup.height"
                    rx="14"
                  />
                  <text :x="exportGroup.headerX" :y="exportGroup.y + 18" class="chain-group-title">
                    本月导出批次（含⑫补丁）
                  </text>
                  <text :x="exportGroup.headerX" :y="exportGroup.y + 34" class="chain-group-meta">
                    成功 {{ fmtCount(exportGroup.success, 0) }} · 失败
                    {{ fmtCount(exportGroup.failed, 0) }} · 总 {{ fmtCount(exportGroup.total, 0) }}
                  </text>
                  <rect
                    :x="exportGroup.headerX"
                    :y="exportGroup.barY"
                    :width="exportGroup.barWidth"
                    height="7"
                    rx="3.5"
                    class="chain-group-track"
                  />
                  <rect
                    :x="exportGroup.headerX"
                    :y="exportGroup.barY"
                    :width="exportGroup.successW"
                    height="7"
                    rx="3.5"
                    fill="#2E7D32"
                  />
                  <rect
                    :x="exportGroup.headerX + exportGroup.successW"
                    :y="exportGroup.barY"
                    :width="exportGroup.failedW"
                    height="7"
                    rx="3.5"
                    fill="#D32F2F"
                  />
                  <rect
                    :x="exportGroup.headerX + exportGroup.successW + exportGroup.failedW"
                    :y="exportGroup.barY"
                    :width="exportGroup.runningW"
                    height="7"
                    rx="3.5"
                    fill="#1976D2"
                  />
                </g>
                <g v-for="edge in svgEdges" :key="edge.id">
                  <path
                    :d="edge.path"
                    fill="none"
                    :stroke="edge.color"
                    stroke-width="1.8"
                    :stroke-dasharray="edge.dashed ? '6 4' : undefined"
                    :marker-end="`url(#${edge.marker})`"
                    stroke-linejoin="round"
                  />
                  <text v-if="edge.label" :x="edge.labelX" :y="edge.labelY" class="chain-edge-label">
                    {{ edge.label }}
                  </text>
                </g>
                <g
                  v-for="node in svgNodes"
                  :key="node.key"
                  class="chain-node"
                  @click="selectedStep = node.key"
                >
                  <title>{{ node.tooltip }}</title>
                  <rect
                    :x="node.x"
                    :y="node.y"
                    :width="node.width"
                    :height="NODE_HEIGHT"
                    rx="10"
                    :fill="node.fill"
                    :stroke="node.border"
                    :stroke-width="node.selected ? 3 : node.warn ? 2.5 : 1.5"
                    :filter="node.selected ? 'url(#nodeShadow)' : undefined"
                  />
                  <text
                    :x="node.x + node.width - 10"
                    :y="node.y + 14"
                    text-anchor="end"
                    class="chain-node-schedule"
                  >
                    {{ node.schedule }}
                  </text>
                  <text :x="node.x + 14" :y="node.y + 30" class="chain-node-title">
                    {{ node.no }} {{ node.title }}
                  </text>
                  <text :x="node.x + 14" :y="node.y + 47" class="chain-node-status" :fill="node.border">
                    {{ node.stateText }}
                  </text>
                  <text :x="node.x + 14" :y="node.y + 63" class="chain-node-metric">
                    {{ node.metricLine }}
                  </text>
                  <rect
                    v-if="node.progress >= 0"
                    :x="node.x + 14"
                    :y="node.y + NODE_HEIGHT - 13"
                    :width="node.width - 28"
                    height="5"
                    rx="2.5"
                    class="chain-node-track"
                  />
                  <rect
                    v-if="node.progress >= 0"
                    :x="node.x + 14"
                    :y="node.y + NODE_HEIGHT - 13"
                    :width="((node.width - 28) * Math.min(100, Math.max(0, node.progress))) / 100"
                    height="5"
                    rx="2.5"
                    :fill="node.border"
                  />
                </g>
              </svg>
            </div>
          </v-sheet>
        </v-col>

        <v-col cols="12" lg="5">
          <v-sheet border rounded class="pa-3 mb-3">
            <div class="text-subtitle-2 mb-2">链路卡点</div>
            <v-alert v-if="problemSteps.length === 0" type="success" density="compact" variant="tonal">
              全链路无卡点
            </v-alert>
            <v-alert
              v-for="step in problemSteps"
              :key="step.key"
              :type="step.state === 'error' ? 'error' : 'warning'"
              density="compact"
              variant="tonal"
              class="mb-2 chain-problem"
              @click="selectStep(step.key)"
            >
              <div class="font-weight-medium">{{ step.no }} {{ step.title }}</div>
              <div class="text-caption">{{ step.error || step.stateText }}</div>
            </v-alert>
            <div v-if="alerts.length" class="mt-2">
              <div class="text-caption text-medium-emphasis mb-1">指标告警</div>
              <v-alert
                v-for="alert in alerts"
                :key="alert.code + alert.message"
                :type="ALERT_TYPE[alert.level] ?? 'info'"
                density="compact"
                variant="tonal"
                class="mb-1"
              >
                <strong>{{ alert.code }}</strong>：{{ alert.message }}
              </v-alert>
            </div>
          </v-sheet>

          <v-sheet v-if="currentStep" border rounded class="pa-3 mb-3">
            <div class="d-flex align-center mb-2">
              <v-icon :icon="currentStep.icon" size="20" class="mr-1" />
              <span class="text-subtitle-2">{{ currentStep.no }} {{ currentStep.title }}</span>
              <v-spacer />
              <v-chip size="x-small" :color="stateMeta(currentStep.state).color" variant="tonal">
                {{ currentStep.stateText }}
              </v-chip>
            </div>
            <div class="text-caption text-medium-emphasis mb-1">{{ currentStep.hint }}</div>
            <div class="text-caption mb-1">
              <v-icon icon="mdi-swap-horizontal" size="14" class="mr-1" />{{ currentStep.dataFlow }}
            </div>
            <div
              v-if="currentStep.error"
              class="text-caption mb-1"
              :class="currentStep.state === 'error' ? 'text-error' : 'text-warning'"
            >
              {{ currentStep.error }}
            </div>
            <div class="text-caption text-medium-emphasis mb-1">
              最近运行：{{ stepLastText(currentStep) }}
              <template v-if="currentStep.last?.processed_rows !== null && currentStep.last?.processed_rows !== undefined">
                · 处理 {{ fmtCount(currentStep.last.processed_rows, 0) }} 行
              </template>
            </div>
            <div v-if="currentStep.last?.error" class="text-caption text-error mb-1">
              {{ currentStep.last.error }}
            </div>
            <div class="chain-numbers mt-2">
              <div v-for="number in currentStep.numbers" :key="number.label">
                <div class="text-caption text-medium-emphasis">{{ number.label }}</div>
                <div class="text-body-2">{{ number.value }}</div>
              </div>
            </div>
            <div v-if="currentStep.jobs.length" class="mt-2 text-caption text-medium-emphasis">
              涉及任务：
              <v-chip
                v-for="kind in currentStep.jobs"
                :key="kind"
                size="x-small"
                variant="outlined"
                class="ml-1"
              >
                {{ jobName(kind) }}（{{ kind }}）
              </v-chip>
            </div>
          </v-sheet>


        </v-col>
      </v-row>

      <!-- 任务视角：10 个 Job 的说明、状态与所在环节 -->
      <v-row v-show="viewMode === 'job'" dense>
        <v-col cols="12" md="4">
          <v-sheet border rounded class="pa-2">
            <div class="text-subtitle-2 pa-2">Job 列表（10 个）</div>
            <v-list density="compact" nav>
              <v-list-item
                v-for="job in jobList"
                :key="job.job_kind"
                :active="currentJob?.job_kind === job.job_kind"
                color="primary"
                @click="selectJob(job.job_kind)"
              >
                <v-list-item-title class="text-body-2">{{ job.name }}</v-list-item-title>
                <v-list-item-subtitle class="text-caption">{{ job.job_kind }}</v-list-item-subtitle>
                <template #append>
                  <v-chip size="x-small" :color="jobStateColor(job)" variant="tonal">
                    {{ jobStateText(job) }}
                  </v-chip>
                </template>
              </v-list-item>
            </v-list>
          </v-sheet>
        </v-col>
        <v-col cols="12" md="8">
          <v-sheet v-if="currentJob" border rounded class="pa-3">
            <div class="d-flex align-center flex-wrap mb-1">
              <v-icon icon="mdi-cog-play-outline" size="20" class="mr-1" />
              <span class="text-subtitle-1">{{ currentJob.name }}</span>
              <span class="text-caption text-medium-emphasis ml-2">{{ currentJob.job_kind }}</span>
              <v-spacer />
              <v-chip size="small" :color="jobStateColor(currentJob)" variant="tonal">
                {{ jobStateText(currentJob) }}
              </v-chip>
            </div>
            <div class="d-flex flex-wrap mb-2">
              <v-chip size="x-small" variant="tonal" class="mr-1 mb-1">{{ currentJob.schedule }}</v-chip>
              <v-chip v-if="currentJob.windowed" size="x-small" color="primary" variant="tonal" class="mr-1 mb-1">
                仅低峰窗口
              </v-chip>
            </div>

            <v-progress-linear
              v-if="currentJob.in_process"
              :model-value="percentOf(currentJob.running) >= 0 ? percentOf(currentJob.running) : 0"
              :indeterminate="percentOf(currentJob.running) < 0"
              color="primary"
              height="8"
              rounded
              class="mb-1"
            />
            <div v-if="currentJob.in_process" class="text-caption text-medium-emphasis mb-2">
              执行中：{{ stageName(currentJob.running?.stage) }}
              · 已运行 {{ fmtDuration(currentJob.elapsed_seconds) }}
              · 心跳 {{ fmtDuration(currentJob.heartbeat_age_seconds) }}前
              <template v-if="currentJob.running?.progress_detail"> · {{ currentJob.running.progress_detail }}</template>
            </div>

            <div class="text-body-2 mb-1"><strong>功能：</strong>{{ currentJob.summary }}</div>
            <div class="text-body-2 mb-1"><strong>流程：</strong>{{ currentJob.flow }}</div>
            <div class="text-body-2 mb-1"><strong>工作细节：</strong>{{ currentJob.details }}</div>
            <div class="text-body-2 mb-2"><strong>注意事项：</strong>{{ currentJob.cautions }}</div>

            <v-sheet border rounded class="pa-2 mb-2">
              <div class="text-caption text-medium-emphasis mb-1">最近运行</div>
              <template v-if="currentJob.last">
                <div class="d-flex align-center flex-wrap">
                  <v-chip size="x-small" :color="runStatusColor(currentJob.last.status)" variant="tonal" class="mr-2">
                    {{ runStatusName(currentJob.last.status) }}
                  </v-chip>
                  <span class="text-caption">{{ fmtISO(currentJob.last.gmt_finish) }}</span>
                  <span class="text-caption text-medium-emphasis ml-3">
                    阶段 {{ stageName(currentJob.last.stage) }} · 处理 {{ fmtCount(currentJob.last.processed_rows, 0) }} 行
                  </span>
                </div>
                <div v-if="currentJob.last.error" class="text-caption text-error mt-1">{{ currentJob.last.error }}</div>
              </template>
              <div v-else class="text-caption text-medium-emphasis">暂无运行记录</div>
            </v-sheet>

            <div class="text-subtitle-2 mb-1">在总体链路中的位置</div>
            <div class="d-flex flex-wrap">
              <v-chip
                v-for="step in stepsOfJob(currentJob.job_kind)"
                :key="step.key"
                :color="stateMeta(step.state).color"
                variant="tonal"
                class="mr-1 mb-1"
                @click="selectStep(step.key)"
              >
                {{ step.no }} {{ step.title }} · {{ step.stateText }}
              </v-chip>
            </div>
            <div v-if="stepsOfJob(currentJob.job_kind).length === 0" class="text-caption text-medium-emphasis">
              该任务未直接绑定主链路步骤。
            </div>
          </v-sheet>
        </v-col>
      </v-row>

      <v-row class="mt-2" dense>
        <v-col cols="12" md="8">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">视角说明</div>
            <div class="text-caption text-medium-emphasis">
              <div><strong>总体视角</strong>：按 ①–⑪ 展示主数据流、⑫–⑭ 展示旁路任务；红/黄边框与「链路卡点」列出异常步骤及其错误信息。⑫ 收口后补丁属于本月导出批次的一部分：补丁数据先进入 ⑤ 中转表，再走 ⑥ 导出 → ⑦ 登记 → ⑧ 清理 → ⑨ 上传；批次进度包含 ⑫ 补丁任务。</div>
              <div><strong>任务视角</strong>：按 10 个 Job 查看功能 / 流程 / 工作细节 / 注意事项、当前状态与最近运行，并显示该 Job 在总体链路中的步骤位置（可点击跳转）。</div>
              <div class="mt-1">执行中 = Job 正在运行；心跳超时 = 运行记录超过 {{ fmtDuration(runtime?.stale_after_seconds) }} 未刷新；异常 / 需关注来自最近失败与业务阈值。</div>
            </div>
          </v-sheet>
        </v-col>
        <v-col cols="12" md="4">
          <v-sheet border rounded class="pa-3 h-100">
            <div class="text-subtitle-2 mb-2">数据来源</div>
            <div class="text-caption text-medium-emphasis">
              <div>业务指标：GET /API/V1/Meta/Finv/Quote/DeDup/Status</div>
              <div>Job 阶段/进度/心跳：GET /API/V1/Meta/Finv/Quote/DeDup/Job/Runtime</div>
              <div>低峰窗口：GET/PUT /API/V1/Meta/Finv/Quote/DeDup/Settings</div>
            </div>
          </v-sheet>
        </v-col>
      </v-row>
    </v-card-text>

    <v-dialog v-model="settingsOpen" max-width="480">
      <v-card>
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-clock-edit-outline" class="mr-2" />
          修改低峰窗口
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" @click="settingsOpen = false" />
        </v-card-title>
        <v-card-text>
          <v-alert v-if="settingsError" type="error" density="compact" class="mb-3">{{ settingsError }}</v-alert>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model="windowStart" type="time" step="1" label="开始时间（精确到秒）" density="compact" hide-details />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="windowEnd" type="time" step="1" label="结束时间（精确到秒）" density="compact" hide-details />
            </v-col>
          </v-row>
          <div class="text-caption text-medium-emphasis mt-2">
            精确到秒，支持跨零点（如 22:00:00 ~ 06:00:00）；开始与结束相同表示全天。保存到系统配置 20006，立即生效，无需重启服务。
          </div>
          <div v-if="settings" class="text-caption text-medium-emphasis mt-1">
            当前生效：{{ settings.window_start }} ~ {{ settings.window_end }}（{{ settings.source === 'sysconfig' ? '系统配置 20006' : '内置默认值' }}）
            <template v-if="settings.updated_at"> · 更新于 {{ fmtISO(settings.updated_at) }}</template>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="settingsOpen = false">取消</v-btn>
          <v-btn color="primary" :loading="savingWindow" @click="saveWindow">保存并生效</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="toast" :timeout="4000" :color="toastColor">{{ toastText }}</v-snackbar>
  </v-card>
</template>

<style scoped>
.chain-step {
  cursor: pointer;
}

.chain-running {
  border-color: rgb(var(--v-theme-primary));
  box-shadow: 0 0 0 1px rgb(var(--v-theme-primary));
}

.chain-stale,
.chain-warning {
  border-color: rgb(var(--v-theme-warning));
}

.chain-error {
  border-color: rgb(var(--v-theme-error));
  box-shadow: 0 0 0 1px rgb(var(--v-theme-error));
}

.chain-selected {
  background: rgba(var(--v-theme-primary), 0.06);
}

.chain-connector {
  display: flex;
  align-items: center;
  padding: 4px 0 4px 12px;
  color: rgba(var(--v-theme-on-surface), 0.5);
}

.chain-numbers {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 12px;
}

.chain-problem {
  cursor: pointer;
}

.tooltip-doc {
  white-space: normal;
  text-align: left;
  line-height: 1.5;
}

.chain-svg-wrap {
  width: 100%;
  overflow-x: auto;
}

.chain-svg {
  width: 100%;
  min-width: 700px;
  height: auto;
  display: block;
}

.chain-node {
  cursor: pointer;
}

.chain-node:hover rect {
  filter: brightness(0.98);
}

.chain-node-title {
  font-size: 13px;
  font-weight: 600;
  fill: rgb(var(--v-theme-on-surface));
}

.chain-node-status {
  font-size: 11px;
}

.chain-node-metric {
  font-size: 10px;
  fill: rgba(var(--v-theme-on-surface), 0.7);
}

.chain-node-track {
  fill: rgba(var(--v-theme-on-surface), 0.22);
}

.chain-group > rect:first-child {
  fill: rgba(var(--v-theme-on-surface), 0.03);
  stroke: rgba(var(--v-theme-on-surface), 0.25);
  stroke-dasharray: 6 4;
}

.chain-group-title {
  font-size: 12px;
  font-weight: 600;
  fill: rgb(var(--v-theme-on-surface));
}

.chain-group-meta {
  font-size: 10px;
  fill: rgba(var(--v-theme-on-surface), 0.7);
}

.chain-group-track {
  fill: rgba(var(--v-theme-on-surface), 0.22);
}

.chain-node-schedule {
  font-size: 9.5px;
  fill: rgba(var(--v-theme-on-surface), 0.5);
}

.chain-edge-label {
  font-size: 9.5px;
  fill: rgba(var(--v-theme-on-surface), 0.65);
}
</style>
