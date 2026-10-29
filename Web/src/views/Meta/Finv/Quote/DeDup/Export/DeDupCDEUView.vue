<script setup lang="ts">
// CDEU 工作流控制与监控：Workflow → Phase → Process → JOB → Step 结构化视图
// 数据来源：GET /Meta/Finv/Quote/DeDup/CDEU/Overview
// 手动执行复用既有 POST /Meta/Finv/Quote/DeDup/Job/Run（异步）
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../../api'
import { fmtISO, fmtBytes, fmtNum } from '../../../../../../utils'

interface CDEUWorkflow {
  workflow_id: number
  workflow_code: string
  name: string
  description: string
  version: number
  run_mode: string
  schedule_time: string
  schedule_timezone: string
  schedule_enabled: boolean
  completion_rule: string
}

interface CDEURun {
  run_id: number
  workflow_code: string
  workflow_version: number
  biz_date: string
  trigger_type: string
  status: string
  scheduled_at: string | null
  started_at: string | null
  finished_at: string | null
  completion_rule_snapshot: string | null
  completion_snapshot: string | null
  operator: string | null
  error: string | null
  gmt_create: string
}

interface CDEUJobStep {
  step_id: number
  step_code: string
  name: string
  seq_no: number
  critical: boolean
  affects_completion: boolean
  step_run_id: number | null
  status: string
  attempt: number
  started_at: string | null
  finished_at: string | null
  processed_rows: number
  processed_bytes: number
  error: string | null
}

interface CDEUJob {
  job_id: number
  job_code: string
  name: string
  job_kind: string
  execution_stage: string | null
  seq_no: number
  critical: boolean
  affects_completion: boolean
  trigger_mode: string
  execution_mode: string
  manual_rerun_enabled: boolean
  schedule_kind: string
  schedule_value: string | null
  status: string
  attempt: number
  batch_key: string | null
  started_at: string | null
  finished_at: string | null
  processed_rows: number
  processed_bytes: number
  run_log_id: number | null
  error: string | null
  instances: number
  steps: CDEUJobStep[]
  dynamic_job_id?: number | null
  usc?: string | null
  biz_month?: number | null
  export_type?: string | null
  job_category?: string | null
  file_id?: number | null
  file_type?: string | null
  file_version?: number | null
  is_dynamic?: boolean
}

interface RunLogLine {
  line_id: number
  run_id: number
  level: string
  stage: string | null
  message: string
  progress_percent: number | null
  gmt_create: string
}

interface ManualRunResponse {
  job_kind?: string
  dynamic_job_id?: number
  run_log_id: number
  auto_continue?: boolean
}

interface RunLogDetail {
  run_id: number
  job_kind: string
  job_id: number | null
  usc: string | null
  biz_month: number | null
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
  lines: RunLogLine[]
}

interface CDEUProcess {
  process_id: number
  process_code: string
  name: string
  seq_no: number
  critical: boolean
  affects_completion: boolean
  run_mode: string
  batch_enabled: boolean
  batch_source: string | null
  batch_key_field: string | null
  batch_size: number | null
  max_parallel: number
  failure_policy: string
  status: string
  started_at: string | null
  finished_at: string | null
  batch_total: number
  batch_success: number
  batch_failed: number
  error: string | null
  jobs: CDEUJob[]
}

interface CDEUPhase {
  phase_id: number
  phase_code: string
  name: string
  seq_no: number
  critical: boolean
  affects_completion: boolean
  run_mode: string
  batch_enabled: boolean
  phase_run_id: number | null
  status: string
  started_at: string | null
  finished_at: string | null
  total_count: number
  success_count: number
  failed_count: number
  processes: CDEUProcess[]
}

interface CDEUOverview {
  generated_at: string
  cdeu_mode: string
  cdeu_schedule_enabled: boolean
  workflow: CDEUWorkflow
  run: CDEURun | null
  phases: CDEUPhase[]
  history: CDEURun[]
}

const STATUS_META: Record<string, { text: string; color: string; icon: string }> = {
  NOT_STARTED: { text: '未运行', color: 'grey', icon: 'mdi-circle-outline' },
  PENDING: { text: '待执行', color: 'grey', icon: 'mdi-clock-outline' },
  READY: { text: '就绪', color: 'info', icon: 'mdi-play-circle-outline' },
  RUNNING: { text: '执行中', color: 'primary', icon: 'mdi-loading' },
  SUCCESS: { text: '成功', color: 'success', icon: 'mdi-check-circle-outline' },
  PARTIAL: { text: '部分完成', color: 'warning', icon: 'mdi-alert-circle-outline' },
  FAILED: { text: '异常', color: 'error', icon: 'mdi-close-circle-outline' },
  PAUSED: { text: '已暂停', color: 'warning', icon: 'mdi-pause-circle-outline' },
  CANCELLED: { text: '已取消', color: 'grey', icon: 'mdi-cancel' },
  SKIPPED: { text: '已跳过', color: 'grey', icon: 'mdi-skip-next-circle-outline' },
}

const TRIGGER_NAME: Record<string, string> = {
  SCHEDULE: '定时',
  MANUAL: '手动',
  CONTINUOUS: '连续',
}
const RUN_MODE_NAME: Record<string, string> = {
  AUTO_DAILY: '每日自动',
  MANUAL: '手动',
  BOTH: '自动 + 手动',
  INHERIT: '随工作流',
  CONTINUOUS: '独立循环',
}
const JOB_TRIGGER_NAME: Record<string, string> = {
  SCHEDULED: '定时',
  EMBEDDED: '内嵌',
  MANUAL: '手动',
  DYNAMIC: '动态',
}
const BATCH_SOURCE_NAME: Record<string, string> = {
  EXPORT_JOB_CANDIDATES: '导出候选',
  EXPORT_FILES: '导出文件',
  PATCH_CANDIDATES: '补丁候选',
  USC_LIST: '证券列表',
  CUSTOM_SQL: '自定义 SQL',
}

const overview = ref<CDEUOverview | null>(null)
const loading = ref(false)
const error = ref('')
const autoRefresh = ref(true)
const viewMode = ref<'overall' | 'detail'>('overall')
const selectedPhaseCode = ref('')
const selectedProcessCode = ref('')
const startDialog = ref(false)
const terminateDialog = ref(false)
const terminating = ref(false)
const terminateResult = ref<Record<string, number> | null>(null)
const legendVisible = ref(true)
const processTopologyVisible = ref(true)
const processNameWidth = ref(200)
const processKeyWidth = ref(200)
const jobNameWidth = ref(190)
const jobCodeWidth = ref(200)
const jobKindWidth = ref(220)
const starting = ref(false)
const selectedRunId = ref(0)
const runningKind = ref('')
const snackbar = ref(false)
const snackbarText = ref('')
const snackbarColor = ref<'success' | 'error'>('success')
const logDialog = ref(false)
const logLoading = ref(false)
const logError = ref('')
const logRunID = ref(0)
const logTitle = ref('')
const logFullscreen = ref(false)
const logAutoContinue = ref(false)
const logDetail = ref<RunLogDetail | null>(null)
const runDialog = ref(false)
const runDialogJob = ref<CDEUJob | null>(null)
const autoContinue = ref(false)

let refreshTimer: ReturnType<typeof setInterval> | null = null
let logRefreshTimer: ReturnType<typeof setTimeout> | null = null
let fastRefreshUntil = 0
let lastRefreshAt = 0

const workflow = computed(() => overview.value?.workflow ?? null)
const run = computed(() => overview.value?.run ?? null)
const phases = computed(() => overview.value?.phases ?? [])
const history = computed(() => overview.value?.history ?? [])
const activeStatus = computed(() => run.value?.status ?? 'NOT_STARTED')
const canStart = computed(() => !starting.value && !['RUNNING', 'PENDING', 'READY'].includes(activeStatus.value))
// 只有「运行中」才允许强制终止；终态运行的子级已定型，不需要也不能再收口。
const canTerminate = computed(() => !terminating.value && ['RUNNING', 'PENDING', 'READY'].includes(activeStatus.value))

const phaseStats = computed(() => {
  let total = 0
  let success = 0
  let failed = 0
  let running = 0
  for (const phase of phases.value) {
    if (!phase.affects_completion) continue
    total += 1
    if (phase.status === 'SUCCESS') success += 1
    else if (phase.status === 'FAILED') failed += 1
    else if (phase.status === 'RUNNING') running += 1
  }
  return { total, success, failed, running }
})

const jobStats = computed(() => {
  let total = 0
  let success = 0
  let failed = 0
  let running = 0
  let pending = 0
  for (const phase of phases.value) {
    for (const process of phase.processes) {
      for (const job of process.jobs) {
        total += 1
        if (job.status === 'SUCCESS') success += 1
        else if (job.status === 'FAILED' || job.status === 'PARTIAL') failed += 1
        else if (job.status === 'RUNNING') running += 1
        else pending += 1
      }
    }
  }
  return { total, success, failed, running, pending }
})

const jobStepStats = computed(() => {
  let total = 0
  let success = 0
  let failed = 0
  let running = 0
  for (const phase of phases.value) {
    for (const process of phase.processes) {
      for (const job of process.jobs) {
        for (const step of job.steps) {
          total += 1
          if (step.status === 'SUCCESS') success += 1
          else if (step.status === 'FAILED' || step.status === 'PARTIAL') failed += 1
          else if (step.status === 'RUNNING') running += 1
        }
      }
    }
  }
  return { total, success, failed, running }
})

const overallProgress = computed(() => {
  if (!run.value) return 0
  if (phaseStats.value.total === 0) return 0
  return Math.round((phaseStats.value.success / phaseStats.value.total) * 100)
})

const activePhase = computed(() => {
  const runningPhase = phases.value.find((phase) => phase.status === 'RUNNING')
  if (runningPhase) return runningPhase
  const failedPhase = phases.value.find((phase) => phase.status === 'FAILED')
  if (failedPhase) return failedPhase
  return phases.value.find((phase) => phase.status !== 'SUCCESS' && phase.affects_completion) ?? null
})

const selectedPhase = computed<CDEUPhase | null>(() => {
  return phases.value.find((phase) => phase.phase_code === selectedPhaseCode.value) ?? phases.value[0] ?? null
})

const selectedProcess = computed<CDEUProcess | null>(() => {
  return selectedPhase.value?.processes.find((process) => process.process_code === selectedProcessCode.value) ?? null
})

const processTableMinWidth = computed(() =>
  // 序号54 + 名称 + 标识100 + KEY + 状态110 + JOB总数84 + 成功72 + 失败72
  // + 执行中76 + 批次总数84 + 批次失败84 + 开始时间170 + 耗时110 + 异常120
  54 + processNameWidth.value + processKeyWidth.value + 100 + 110 + 84 + 72 + 72 + 76 + 84 + 84 + 170 + 110 + 120,
)
const jobTableMinWidth = computed(() =>
  jobNameWidth.value + jobCodeWidth.value + jobKindWidth.value +
  // 触发方式90 + 执行次数80 + Step150 + 执行阶段90 + 状态95
  // + 开始时间155 + 耗时110 + 处理量90 + 异常160 + 操作180
  90 + 80 + 150 + 90 + 95 + 155 + 110 + 90 + 160 + 180,
)

const WORKFLOW_NODE_WIDTH = 170
const WORKFLOW_NODE_HEIGHT = 96
const WORKFLOW_TERMINAL_WIDTH = 96
const WORKFLOW_GAP = 48
const WORKFLOW_PADDING = 20
const EDGE_COLORS = [
  { key: 'grey', color: '#9E9E9E' },
  { key: 'blue', color: '#1976D2' },
  { key: 'green', color: '#2E7D32' },
  { key: 'orange', color: '#F9A825' },
  { key: 'red', color: '#D32F2F' },
  { key: 'cyan', color: '#0288D1' },
]

interface WorkflowNode {
  phase: CDEUPhase
  x: number
  y: number
}

interface WorkflowTerminal {
  key: 'start' | 'end'
  label: string
  stateText: string
  status: string
  icon: string
  tooltip: string
  x: number
  y: number
}

interface WorkflowEdge {
  key: string
  path: string
  color: string
  marker: string
  running: boolean
}

interface ProcessTopologyNode {
  process: CDEUProcess
  x: number
  y: number
}

interface ProcessTopologyTerminal {
  key: 'start' | 'end'
  label: string
  name: string
  status: string
  x: number
  y: number
}

const PROCESS_TOPOLOGY_NODE_WIDTH = 180
const PROCESS_TOPOLOGY_NODE_HEIGHT = 64
const PROCESS_TOPOLOGY_TERMINAL_WIDTH = 104
const PROCESS_TOPOLOGY_GAP = 44
const PROCESS_TOPOLOGY_PADDING = 16

const selectedPhaseIndex = computed(() =>
  phases.value.findIndex((phase) => phase.phase_code === selectedPhaseCode.value),
)
const previousPhase = computed<CDEUPhase | null>(() => {
  const index = selectedPhaseIndex.value
  return index > 0 ? phases.value[index - 1] : null
})
const nextPhase = computed<CDEUPhase | null>(() => {
  const index = selectedPhaseIndex.value
  return index >= 0 && index < phases.value.length - 1 ? phases.value[index + 1] : null
})

type ProcessTopologyMode = 'linear' | 'exportParallel' | 'decisionFork'

const processTopologyMode = computed<ProcessTopologyMode>(() => {
  const phase = selectedPhase.value
  const codes = new Set((phase?.processes ?? []).map((process) => process.process_code))
  if (phase?.phase_code === 'PHASE_DECISION' &&
      codes.has('PROCESS_INGEST_STATS') && codes.has('PROCESS_MATURITY') && codes.has('PROCESS_BUDGET')) {
    return 'decisionFork'
  }
  if (phase?.phase_code === 'PHASE_EXPORT' &&
      codes.has('PROCESS_EXPORT_FILE_MON_BASE') && codes.has('PROCESS_RECOVER')) {
    return 'exportParallel'
  }
  return 'linear'
})

const processTopologyHasParallel = computed(() => processTopologyMode.value !== 'linear')
const processTopologyBranchCodes = computed<string[]>(() => {
  if (processTopologyMode.value === 'decisionFork') return ['PROCESS_MATURITY']
  if (processTopologyMode.value === 'exportParallel') return ['PROCESS_RECOVER']
  return []
})

const processTopologyNodes = computed<ProcessTopologyNode[]>(() => {
  const processes = selectedPhase.value?.processes ?? []
  const startX = PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_TERMINAL_WIDTH + PROCESS_TOPOLOGY_GAP
  const branchCodes = new Set(processTopologyBranchCodes.value)
  const mainProcesses = processes.filter((process) => !branchCodes.has(process.process_code))
  const branchProcesses = processes.filter((process) => branchCodes.has(process.process_code))
  const mainNodes = mainProcesses.map((process, index) => ({
    process,
    x: startX + index * (PROCESS_TOPOLOGY_NODE_WIDTH + PROCESS_TOPOLOGY_GAP),
    y: PROCESS_TOPOLOGY_PADDING,
  }))
  const branchX = processTopologyMode.value === 'decisionFork'
    ? (mainNodes[0]?.x ?? startX)
    : (mainNodes[1]?.x ?? mainNodes[0]?.x ?? startX)
  const branchY = PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_NODE_HEIGHT + 24
  const branchNodes = branchProcesses.map((process, index) => ({
    process,
    x: branchX + index * (PROCESS_TOPOLOGY_NODE_WIDTH + PROCESS_TOPOLOGY_GAP),
    y: branchY,
  }))
  return [...mainNodes, ...branchNodes]
})

const processTopologyEndX = computed(() => {
  const nodes = processTopologyNodes.value
  if (!nodes.length) return PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_TERMINAL_WIDTH + PROCESS_TOPOLOGY_GAP
  const right = Math.max(...nodes.map((node) => node.x + PROCESS_TOPOLOGY_NODE_WIDTH))
  return right + PROCESS_TOPOLOGY_GAP
})
const processTopologyWidth = computed(() =>
  Math.max(processTopologyEndX.value + PROCESS_TOPOLOGY_TERMINAL_WIDTH + PROCESS_TOPOLOGY_PADDING, 360),
)
const processTopologyHeight = computed(() => processTopologyHasParallel.value
  ? PROCESS_TOPOLOGY_PADDING * 2 + PROCESS_TOPOLOGY_NODE_HEIGHT * 2 + 24
  : PROCESS_TOPOLOGY_PADDING * 2 + PROCESS_TOPOLOGY_NODE_HEIGHT,
)

const processTopologyStart = computed<ProcessTopologyTerminal>(() => {
  const previous = previousPhase.value
  return {
    key: 'start',
    label: previous ? '上一阶段' : '开始',
    name: previous ? previous.name : '工作流起点',
    status: previous?.status ?? (run.value ? 'SUCCESS' : 'PENDING'),
    x: PROCESS_TOPOLOGY_PADDING,
    y: PROCESS_TOPOLOGY_PADDING,
  }
})

const processTopologyEnd = computed<ProcessTopologyTerminal>(() => {
  const next = nextPhase.value
  return {
    key: 'end',
    label: next ? '下一阶段' : '结束',
    name: next ? next.name : '工作流终点',
    status: next?.status ?? selectedPhase.value?.status ?? 'PENDING',
    x: processTopologyEndX.value,
    y: PROCESS_TOPOLOGY_PADDING,
  }
})

const processTopologyTerminals = computed<ProcessTopologyTerminal[]>(() => [
  processTopologyStart.value,
  processTopologyEnd.value,
])

const processTopologyEdges = computed<WorkflowEdge[]>(() => {
  const nodes = processTopologyNodes.value
  if (!nodes.length) return []
  const branchCodes = new Set(processTopologyBranchCodes.value)
  const mainProcesses = nodes.filter((node) => !branchCodes.has(node.process.process_code))
  const branchNodes = nodes.filter((node) => branchCodes.has(node.process.process_code))
  const mainCenterY = PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_NODE_HEIGHT / 2
  const branchCenterY = PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_NODE_HEIGHT + 24 +
    PROCESS_TOPOLOGY_NODE_HEIGHT / 2
  const start = processTopologyStart.value
  const end = processTopologyEnd.value
  const edges: WorkflowEdge[] = []
  const first = mainProcesses[0]
  if (!first) return edges
  const firstEdge = workflowEdgeByStatus(first.process.status)
  edges.push({
    key: 'topology-start',
    path: `M ${start.x + PROCESS_TOPOLOGY_TERMINAL_WIDTH} ${mainCenterY} L ${first.x - 8} ${mainCenterY}`,
    color: firstEdge.color,
    marker: firstEdge.marker,
    running: first.process.status === 'RUNNING',
  })
  for (let index = 0; index < mainProcesses.length - 1; index += 1) {
    const from = mainProcesses[index]
    const to = mainProcesses[index + 1]
    const edge = workflowEdgeByStatus(to.process.status)
    edges.push({
      key: `topology-${from.process.process_code}-${to.process.process_code}`,
      path: `M ${from.x + PROCESS_TOPOLOGY_NODE_WIDTH} ${mainCenterY} L ${to.x - 8} ${mainCenterY}`,
      color: edge.color,
      marker: edge.marker,
      running: to.process.status === 'RUNNING',
    })
  }
  if (processTopologyMode.value === 'decisionFork') {
    // 入口统计与行情成熟度并行：两条支路汇入预算计算。
    const budget = mainProcesses[1]
    for (const branch of branchNodes) {
      const branchEdge = workflowEdgeByStatus(branch.process.status)
      const startEdgeX = start.x + PROCESS_TOPOLOGY_TERMINAL_WIDTH + 12
      edges.push({
        key: `topology-start-branch-${branch.process.process_code}`,
        path: `M ${start.x + PROCESS_TOPOLOGY_TERMINAL_WIDTH} ${mainCenterY} L ${startEdgeX} ${mainCenterY} L ${startEdgeX} ${branchCenterY} L ${branch.x - 8} ${branchCenterY}`,
        color: branchEdge.color,
        marker: branchEdge.marker,
        running: branch.process.status === 'RUNNING',
      })
      if (budget) {
        edges.push({
          key: `topology-branch-join-${branch.process.process_code}`,
          path: `M ${branch.x + PROCESS_TOPOLOGY_NODE_WIDTH} ${branchCenterY} L ${budget.x - 8} ${branchCenterY} L ${budget.x - 8} ${mainCenterY} L ${budget.x} ${mainCenterY}`,
          color: workflowEdgeByStatus(budget.process.status).color,
          marker: workflowEdgeByStatus(budget.process.status).marker,
          running: budget.process.status === 'RUNNING',
        })
      }
    }
  } else if (processTopologyMode.value === 'exportParallel') {
    // PROCESS_RECOVER 与 BASE 并行：从批次节点分出下行支路，再汇入下一阶段。
    for (const branch of branchNodes) {
      const branchEdge = workflowEdgeByStatus(branch.process.status)
      const branchX = first.x + PROCESS_TOPOLOGY_NODE_WIDTH / 2
      edges.push({
        key: `topology-branch-${branch.process.process_code}`,
        path: `M ${branchX} ${PROCESS_TOPOLOGY_PADDING + PROCESS_TOPOLOGY_NODE_HEIGHT} L ${branchX} ${branchCenterY} L ${branch.x - 8} ${branchCenterY}`,
        color: branchEdge.color,
        marker: branchEdge.marker,
        running: branch.process.status === 'RUNNING',
      })
      edges.push({
        key: `topology-branch-end-${branch.process.process_code}`,
        path: `M ${branch.x + PROCESS_TOPOLOGY_NODE_WIDTH} ${branchCenterY} L ${end.x - 8} ${branchCenterY} L ${end.x - 8} ${mainCenterY} L ${end.x} ${mainCenterY}`,
        color: workflowEdgeByStatus(end.status).color,
        marker: workflowEdgeByStatus(end.status).marker,
        running: false,
      })
    }
  }
  const last = mainProcesses[mainProcesses.length - 1]
  const endEdge = workflowEdgeByStatus(end.status)
  edges.push({
    key: 'topology-end',
    path: `M ${last.x + PROCESS_TOPOLOGY_NODE_WIDTH} ${mainCenterY} L ${end.x - 8} ${mainCenterY}`,
    color: endEdge.color,
    marker: endEdge.marker,
    running: false,
  })
  return edges
})

const workflowNodes = computed<WorkflowNode[]>(() => {
  const phaseStartX = WORKFLOW_PADDING + WORKFLOW_TERMINAL_WIDTH + WORKFLOW_GAP
  return phases.value.map((phase, index) => ({
    phase,
    x: phaseStartX + index * (WORKFLOW_NODE_WIDTH + WORKFLOW_GAP),
    y: WORKFLOW_PADDING,
  }))
})

const workflowEndX = computed(() =>
  WORKFLOW_PADDING + WORKFLOW_TERMINAL_WIDTH + WORKFLOW_GAP +
  phases.value.length * (WORKFLOW_NODE_WIDTH + WORKFLOW_GAP),
)

const workflowWidth = computed(() =>
  Math.max(workflowEndX.value + WORKFLOW_TERMINAL_WIDTH + WORKFLOW_PADDING, 360),
)
const workflowHeight = WORKFLOW_PADDING * 2 + WORKFLOW_NODE_HEIGHT

const workflowStart = computed<WorkflowTerminal>(() => ({
  key: 'start',
  label: '开始',
  status: run.value ? 'SUCCESS' : 'PENDING',
  stateText: run.value ? '已开始' : '未开始',
  icon: 'mdi-play-circle-outline',
  tooltip: run.value ? `工作流已启动 · #${run.value.run_id} · ${fmtISO(run.value.started_at)}` : '工作流尚未启动',
  x: WORKFLOW_PADDING,
  y: WORKFLOW_PADDING,
}))

const workflowEnd = computed<WorkflowTerminal>(() => {
  const status = run.value?.status ?? 'PENDING'
  let terminalStatus = 'PENDING'
  let stateText = '等待开始'
  if (run.value) {
    if (status === 'SUCCESS') {
      terminalStatus = 'SUCCESS'
      stateText = '已完成'
    } else if (status === 'FAILED') {
      terminalStatus = 'FAILED'
      stateText = '未完成'
    } else if (status === 'PARTIAL') {
      terminalStatus = 'PARTIAL'
      stateText = '部分完成'
    } else if (status === 'PAUSED' || status === 'CANCELLED') {
      terminalStatus = status
      stateText = status === 'PAUSED' ? '已暂停' : '已取消'
    } else {
      stateText = '等待结束'
    }
  }
  return {
    key: 'end',
    label: '结束',
    status: terminalStatus,
    stateText,
    icon: 'mdi-flag-checkered',
    tooltip: run.value
      ? `工作流状态：${statusOf(status).text}${run.value.finished_at ? ` · ${fmtISO(run.value.finished_at)}` : ' · 等待工作流完成'}`
      : '暂无工作流运行',
    x: workflowEndX.value,
    y: WORKFLOW_PADDING,
  }
})

const workflowTerminals = computed<WorkflowTerminal[]>(() => [workflowStart.value, workflowEnd.value])

function workflowEdgeByStatus(status: string): { color: string; marker: string } {
  switch (statusOf(status).color) {
    case 'success': return { color: '#2E7D32', marker: 'cdeu-arrow-green' }
    case 'error': return { color: '#D32F2F', marker: 'cdeu-arrow-red' }
    case 'primary': return { color: '#1976D2', marker: 'cdeu-arrow-blue' }
    case 'warning': return { color: '#F9A825', marker: 'cdeu-arrow-orange' }
    case 'info': return { color: '#0288D1', marker: 'cdeu-arrow-cyan' }
    default: return { color: '#9E9E9E', marker: 'cdeu-arrow-grey' }
  }
}

function workflowEdge(phase: CDEUPhase): { color: string; marker: string } {
  return workflowEdgeByStatus(phase.status)
}

const workflowEdges = computed<WorkflowEdge[]>(() => {
  const edges: WorkflowEdge[] = []
  const phaseNodes = workflowNodes.value
  if (phaseNodes.length > 0) {
    const first = phaseNodes[0]
    const last = phaseNodes[phaseNodes.length - 1]
    const startEdge = workflowEdge(first.phase)
    const endEdge = workflowEdgeByStatus(run.value?.status ?? 'PENDING')
    const centerY = WORKFLOW_PADDING + WORKFLOW_NODE_HEIGHT / 2
    const start = workflowStart.value
    const end = workflowEnd.value
    edges.push({
      key: 'start-phase',
      path: `M ${start.x + WORKFLOW_TERMINAL_WIDTH} ${centerY} L ${first.x - 8} ${centerY}`,
      color: startEdge.color,
      marker: startEdge.marker,
      running: first.phase.status === 'RUNNING',
    })
    edges.push({
      key: 'phase-end',
      path: `M ${last.x + WORKFLOW_NODE_WIDTH} ${centerY} L ${end.x - 8} ${centerY}`,
      color: endEdge.color,
      marker: endEdge.marker,
      running: false,
    })
  }
  for (let index = 0; index < workflowNodes.value.length - 1; index += 1) {
    const from = workflowNodes.value[index]
    const to = workflowNodes.value[index + 1]
    const edge = workflowEdge(to.phase)
    const y = from.y + WORKFLOW_NODE_HEIGHT / 2
    const x1 = from.x + WORKFLOW_NODE_WIDTH
    const x2 = to.x - 8
    edges.push({
      key: `${from.phase.phase_code}-${to.phase.phase_code}`,
      path: `M ${x1} ${y} L ${x2} ${y}`,
      color: edge.color,
      marker: edge.marker,
      running: to.phase.status === 'RUNNING',
    })
  }
  return edges
})

const selectedHistoryLabel = computed(() => {
  if (!selectedRunId.value) return '最新运行'
  const found = history.value.find((item) => item.run_id === selectedRunId.value)
  return found ? `#${found.run_id} · ${found.biz_date}` : `#${selectedRunId.value}`
})

const latestRun = computed<CDEURun | null>(() => history.value[0] ?? null)
const previousRuns = computed<CDEURun[]>(() => history.value.slice(1))

function statusOf(status: string | null | undefined) {
  return STATUS_META[status ?? 'NOT_STARTED'] ?? STATUS_META.NOT_STARTED
}

function phaseJobStats(phase: CDEUPhase) {
  let total = 0
  let success = 0
  let failed = 0
  let running = 0
  for (const process of phase.processes) {
    for (const job of process.jobs) {
      total += 1
      if (job.status === 'SUCCESS') success += 1
      else if (job.status === 'FAILED' || job.status === 'PARTIAL') failed += 1
      else if (job.status === 'RUNNING') running += 1
    }
  }
  return { total, success, failed, running }
}

function phaseProcessSummary(phase: CDEUPhase): string {
  return phase.processes.map((process) => process.name).join(' / ') || '无 Process'
}

function hasRun(jobs: CDEUJob[]): boolean {
  return jobs.some((job) => ['SUCCESS', 'RUNNING', 'FAILED', 'PARTIAL', 'PAUSED'].includes(job.status))
}

// 旁路阶段/Process 不参与完成判定，工作流结束后仍可能周期执行；避免显示成「待执行」。
function phaseStatus(phase: CDEUPhase) {
  const base = statusOf(phase.status)
  if (!phase.affects_completion && phase.status === 'PENDING' &&
      phase.processes.some((process) => hasRun(process.jobs))) {
    return { text: '旁路运行', color: 'info', icon: 'mdi-call-split' }
  }
  return base
}

function processStatus(process: CDEUProcess) {
  const base = statusOf(process.status)
  if (!process.affects_completion && process.status === 'PENDING' && hasRun(process.jobs)) {
    return { text: '旁路运行', color: 'info', icon: 'mdi-call-split' }
  }
  return base
}

function syncSelectedPhase(): void {
  const list = phases.value
  if (!list.length) {
    selectedPhaseCode.value = ''
    return
  }
  if (list.some((phase) => phase.phase_code === selectedPhaseCode.value)) return
  const active = list.find((phase) => phase.status === 'RUNNING')
    ?? list.find((phase) => phase.status === 'FAILED')
    ?? list.find((phase) => phase.affects_completion && phase.status !== 'SUCCESS')
    ?? list.find((phase) => phase.processes.some((process) => process.jobs.length > 0))
    ?? list[0]
  selectedPhaseCode.value = active.phase_code
}

function syncSelectedProcess(): void {
  const processes = selectedPhase.value?.processes ?? []
  if (!processes.length) {
    selectedProcessCode.value = ''
    return
  }
  if (!processes.some((process) => process.process_code === selectedProcessCode.value)) {
    selectedProcessCode.value = (processes.find((process) => process.jobs.length > 0) ?? processes[0]).process_code
  }
}

function selectPhase(phase: CDEUPhase): void {
  selectedPhaseCode.value = phase.phase_code
  selectedProcessCode.value = (phase.processes.find((process) => process.jobs.length > 0) ?? phase.processes[0])?.process_code ?? ''
  viewMode.value = 'overall'
}

function selectProcess(process: CDEUProcess): void {
  selectedProcessCode.value = process.process_code
}

function processJobStats(process: CDEUProcess) {
  let total = 0
  let success = 0
  let failed = 0
  let running = 0
  for (const job of process.jobs) {
    total += 1
    if (job.status === 'SUCCESS') success += 1
    else if (job.status === 'FAILED' || job.status === 'PARTIAL') failed += 1
    else if (job.status === 'RUNNING') running += 1
  }
  return { total, success, failed, running }
}

function formatDuration(start: string | null, finish: string | null, millisecondPrecision = false): string {
  if (!start) return '-'
  const from = new Date(start).getTime()
  const to = finish ? new Date(finish).getTime() : Date.now()
  if (Number.isNaN(from) || Number.isNaN(to) || to < from) return '-'
  const durationMs = to - from
  // 耗时列：10 秒以内展示毫秒精度，超过 10 秒沿用秒 / 分 / 小时粒度。
  if (millisecondPrecision && durationMs <= 10_000) {
    if (durationMs < 1000) return `${durationMs}ms`
    return `${(durationMs / 1000).toFixed(3)}s`
  }
  const seconds = Math.floor(durationMs / 1000)
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const rest = seconds % 60
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${rest}s`
  return `${rest}s`
}

function progressPercent(success: number, total: number): number {
  if (total <= 0) return 0
  return Math.min(100, Math.round((success / total) * 100))
}

// fmtPct 把进度数值渲染成百分比文本（需求 10748）。
// 数值直接来自 phaseProgress / processProgress，与进度条宽度同源，不二次计算。
function fmtPct(value: number): string {
  return `${Math.round(value)}%`
}

function phaseProgress(phase: CDEUPhase): number {
  if (phase.total_count <= 0) return phase.status === 'SUCCESS' ? 100 : 0
  return progressPercent(phase.success_count, phase.total_count)
}

function processProgress(process: CDEUProcess): number {
  if (process.batch_total <= 0) return process.status === 'SUCCESS' ? 100 : 0
  return progressPercent(process.batch_success, process.batch_total)
}

function jobStepSummary(job: CDEUJob): string {
  if (!job.steps.length) return '无 Step'
  return job.steps.map((step) => `${step.step_code}:${statusOf(step.status).text}`).join(' / ')
}

// JOB 清单只展示当前执行到的 Step：优先 RUNNING，其次已完成的最后一个 Step。
function currentJobStep(job: CDEUJob): CDEUJobStep | null {
  if (!job.steps.length) return null
  const steps = [...job.steps].sort((a, b) => a.seq_no - b.seq_no)
  const running = steps.find((step) => step.status === 'RUNNING')
  if (running) return running
  const reached = steps.filter((step) => step.status !== 'PENDING' && step.status !== 'NOT_STARTED')
  if (reached.length) return reached[reached.length - 1]
  return steps[0]
}

// processStartedText 流程开始时间（需求 10747）：没有开始时间说明它的子级从未执行，
// 显示「未执行」而不是一个空横线，避免与「刚创建还没到点」混淆。
function processStartedText(process: CDEUProcess): string {
  if (process.started_at) return fmtISO(process.started_at)
  return process.status === 'PENDING' ? '-' : '未执行（无子级执行记录）'
}

function jobStartedText(job: CDEUJob): string {
  if (job.started_at) return fmtISO(job.started_at)
  return job.status === 'SUCCESS' ? '空阶段（未执行）' : '-'
}

function runModeText(mode: string): string {
  return RUN_MODE_NAME[mode] ?? mode
}

// cdeuModeText 系统配置 20005 的 CDEU 工作流运行模式（与 workflow.run_mode 不是同一个概念）。
function cdeuModeText(mode: string | undefined | null): string {
  return mode === 'Auto' ? '自动' : '手动'
}

function completionMode(rule: string | undefined): string {
  if (!rule) return '-'
  try {
    const parsed = JSON.parse(rule) as { mode?: string }
    return parsed.mode ?? rule
  } catch {
    return rule
  }
}

const COMPLETION_MODE_NAME: Record<string, string> = {
  ALL: '全部完成',
  CRITICAL_ALL: '关键项全部完成',
  AT_LEAST: '至少 N 项完成',
  ANY_OF: '任意一项完成',
}

function completionModeText(rule: string | undefined): string {
  const mode = completionMode(rule)
  return COMPLETION_MODE_NAME[mode] ?? mode
}

function triggerText(trigger: string): string {
  return TRIGGER_NAME[trigger] ?? trigger
}

function batchSourceText(source: string | null): string {
  if (!source) return ''
  return BATCH_SOURCE_NAME[source] ?? source
}

function showRunButton(job: CDEUJob): boolean {
  if (job.is_dynamic) {
    return job.job_category === 'EXPORT' &&
      ['PENDING', 'FAILED', 'PAUSED', 'SKIPPED', 'PARTIAL'].includes(job.status)
  }
  // 异常态一律允许手动重跑：不能因「内嵌步骤」或「未开启手动重跑」而把按钮藏起来，
  // 否则 JOB 失败后用户没有任何重试入口（run 59 反馈）。
  const isAbnormal = ['FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED'].includes(job.status)
  if (isAbnormal) return true
  if (!job.manual_rerun_enabled) return false
  // 5–8 为同一个 RUN_DUE_EXPORT_JOBS 的内嵌阶段，正常态下只保留一个入口。
  if (job.execution_mode === 'EMBEDDED') return job.job_code === 'JOB_EXPORT_LOAD'
  return true
}

function jobRunKey(job: CDEUJob): string {
  return job.is_dynamic ? `dynamic:${job.dynamic_job_id ?? job.job_id}` : job.job_kind
}

function runButtonText(job: CDEUJob): string {
  if (job.is_dynamic) return '手动执行'
  return job.execution_mode === 'EMBEDDED' ? '运行导出 JOB' : '立即运行'
}

async function load(runId = selectedRunId.value): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const query = runId > 0 ? `?runId=${runId}&historyLimit=30` : '?historyLimit=30'
    overview.value = await apiGet<CDEUOverview>(`/Meta/Finv/Quote/DeDup/CDEU/Overview${query}`)
    syncSelectedPhase()
    syncSelectedProcess()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载 CDEU 工作流失败'
    if (runId > 0) {
      selectedRunId.value = 0
      try {
        overview.value = await apiGet<CDEUOverview>('/Meta/Finv/Quote/DeDup/CDEU/Overview?historyLimit=30')
        syncSelectedPhase()
    syncSelectedProcess()
        error.value = '指定运行不存在，已回到最新运行'
      } catch {
        // 保留原始错误
      }
    }
  } finally {
    loading.value = false
  }
}

async function selectRun(item: CDEURun): Promise<void> {
  selectedRunId.value = item.run_id
  await load(item.run_id)
}

async function loadLatest(): Promise<void> {
  selectedRunId.value = 0
  await load()
}

function runJob(job: CDEUJob): void {
  if (runningKind.value) return
  if (selectedRunId.value > 0) {
    snackbarColor.value = 'error'
    snackbarText.value = '历史运行只读，请回到最新运行后再触发 JOB'
    snackbar.value = true
    return
  }
  // 默认单 JOB 执行；勾选后才续跑同流程后续 JOB。
  autoContinue.value = false
  runDialogJob.value = job
  runDialog.value = true
}

async function confirmRunJob(): Promise<void> {
  const job = runDialogJob.value
  if (!job || runningKind.value) return
  runningKind.value = jobRunKey(job)
  try {
    let response: ManualRunResponse
    if (job.is_dynamic) {
      response = await apiPost<ManualRunResponse>('/Meta/Finv/Quote/DeDup/Job/Dynamic/Run', {
        dynamic_job_id: job.dynamic_job_id,
        auto_continue: autoContinue.value,
      })
    } else {
      response = await apiPost<ManualRunResponse>('/Meta/Finv/Quote/DeDup/Job/Run', {
        job_kind: job.job_kind,
        auto_continue: autoContinue.value,
      })
    }
    runDialog.value = false
    snackbarColor.value = 'success'
    snackbarText.value = autoContinue.value
      ? `已开始 ${job.name}，将自动续跑后续 JOB`
      : `已开始 ${job.name}`
    snackbar.value = true
    fastRefreshUntil = Date.now() + (autoContinue.value ? 5 * 60_000 : 30_000)
    if (response.run_log_id) {
      await openJobLogByID(job, response.run_log_id, autoContinue.value)
    }
    window.setTimeout(() => { void load() }, 1500)
  } catch (e) {
    snackbarColor.value = 'error'
    snackbarText.value = e instanceof Error ? e.message : '触发失败'
    snackbar.value = true
  } finally {
    runningKind.value = ''
  }
}

function isFinishedStatus(status: string | null | undefined): boolean {
  return ['SUCCESS', 'FAILED', 'PARTIAL', 'PAUSED', 'CANCELLED', 'SKIPPED'].includes(status ?? '')
}

function stopLogRefresh(): void {
  if (logRefreshTimer) {
    window.clearTimeout(logRefreshTimer)
    logRefreshTimer = null
  }
}

function scheduleLogRefresh(): void {
  stopLogRefresh()
  if (!logDialog.value || !logDetail.value || isFinishedStatus(logDetail.value.status)) return
  logRefreshTimer = window.setTimeout(async () => {
    await loadJobLog()
    scheduleLogRefresh()
  }, 2000)
}

async function loadJobLog(): Promise<void> {
  const runID = logRunID.value
  if (!runID) return
  logLoading.value = true
  try {
    logDetail.value = await apiGet<RunLogDetail>(`/Meta/Finv/Quote/DeDup/RunLog/Detail?runId=${runID}`)
    logError.value = ''
  } catch (e) {
    logError.value = e instanceof Error ? e.message : '加载 Job 执行日志失败'
  } finally {
    logLoading.value = false
  }
}

async function openJobLog(job: CDEUJob): Promise<void> {
  if (!job.run_log_id) return
  await openJobLogByID(job, job.run_log_id, false)
}

async function openJobLogByID(job: CDEUJob, runID: number, autoRun = false): Promise<void> {
  logRunID.value = runID
  logTitle.value = job.name || 'JOB 执行日志'
  logAutoContinue.value = autoRun
  logDetail.value = null
  logError.value = ''
  logDialog.value = true
  await loadJobLog()
  scheduleLogRefresh()
}

function closeJobLog(): void {
  stopLogRefresh()
  logDialog.value = false
  logRunID.value = 0
  logTitle.value = ''
  logAutoContinue.value = false
  logDetail.value = null
  logFullscreen.value = false
}

async function startWorkflow(): Promise<void> {
  if (starting.value) return
  starting.value = true
  try {
    await apiPost('/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run', { operator: 'web' })
    startDialog.value = false
    selectedRunId.value = 0
    snackbarColor.value = 'success'
    snackbarText.value = 'CDEU 工作流已启动，后台按阶段顺序执行'
    snackbar.value = true
    window.setTimeout(() => { void load() }, 1200)
  } catch (e) {
    snackbarColor.value = 'error'
    snackbarText.value = e instanceof Error ? e.message : '启动工作流失败'
    snackbar.value = true
  } finally {
    starting.value = false
  }
}

// terminateWorkflow 强制终止当前运行：后端把该 run 下所有 PENDING/READY/RUNNING 的
// 阶段、流程、JOB、步骤统一置为 CANCELLED，并释放残留的任务锁。
async function terminateWorkflow(): Promise<void> {
  if (terminating.value) return
  terminating.value = true
  try {
    const res = await apiPost<Record<string, number>>(
      '/Meta/Finv/Quote/DeDup/CDEU/Workflow/Terminate',
      { run_id: run.value?.run_id ?? 0, operator: 'web' },
    )
    terminateResult.value = res as unknown as Record<string, number>
    terminateDialog.value = false
    snackbarColor.value = 'success'
    snackbarText.value = '已强制终止，相关阶段/流程/JOB 已置为终态'
    snackbar.value = true
    window.setTimeout(() => { void load() }, 800)
  } catch (e) {
    snackbarColor.value = 'error'
    snackbarText.value = e instanceof Error ? e.message : '终止工作流失败'
    snackbar.value = true
  } finally {
    terminating.value = false
  }
}

type ResizeColumn = 'processName' | 'processKey' | 'jobName' | 'jobCode' | 'jobKind'

function startColumnResize(column: ResizeColumn, event: MouseEvent): void {
  const targets = {
    processName: processNameWidth,
    processKey: processKeyWidth,
    jobName: jobNameWidth,
    jobCode: jobCodeWidth,
    jobKind: jobKindWidth,
  }
  const target = targets[column]
  const startX = event.clientX
  const startWidth = target.value
  const onMove = (move: MouseEvent) => {
    target.value = Math.max(100, Math.min(620, startWidth + move.clientX - startX))
  }
  const onUp = () => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function startTimer(): void {
  if (refreshTimer) window.clearInterval(refreshTimer)
  refreshTimer = window.setInterval(() => {
    if (!autoRefresh.value || loading.value) return
    const now = Date.now()
    // 手动执行 / 自动续跑期间使用 1 秒刷新，便于观察后续 JOB；空闲时保持 10 秒。
    if (now >= fastRefreshUntil && now - lastRefreshAt < 10000) return
    lastRefreshAt = now
    void load()
  }, 1000)
}

onMounted(() => {
  void load()
  startTimer()
})

onBeforeUnmount(() => {
  if (refreshTimer) window.clearInterval(refreshTimer)
  stopLogRefresh()
})
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center justify-space-between ga-3 mb-4">
      <div>
        <div class="d-flex align-center ga-2">
          <v-icon icon="mdi-sitemap-outline" color="primary" size="30" />
          <h2 class="text-h5 font-weight-bold mb-0">CDEU 工作流控制与监控</h2>
          <v-chip v-if="workflow" size="small" color="primary" variant="tonal">
            {{ workflow.workflow_code }} v{{ workflow.version }}
          </v-chip>
        </div>
        <div v-if="workflow" class="workflow-meta-row">
          <v-chip class="workflow-meta-chip" size="small" color="deep-purple" variant="tonal" prepend-icon="mdi-sitemap-outline">
            {{ workflow.name }}
          </v-chip>
          <v-chip
            class="workflow-meta-chip"
            size="small"
            :color="overview?.cdeu_mode === 'Auto' ? 'success' : 'grey'"
            variant="tonal"
            :prepend-icon="overview?.cdeu_mode === 'Auto' ? 'mdi-robot-outline' : 'mdi-hand-back-right-outline'"
            :title="`系统配置 20005：${cdeuModeText(overview?.cdeu_mode)}（${overview?.cdeu_mode === 'Auto' ? '按定时任务自动触发' : '仅手动触发，调度器不自动跑'}）`"
          >
            <span class="workflow-meta-label">运行模式</span>
            <span class="workflow-meta-value">{{ cdeuModeText(overview?.cdeu_mode) }}</span>
          </v-chip>
          <!-- 每日调度只在自动模式下有意义：20005=Manual 时调度器根本不读这个时间，
               展示出来会让运维误以为「到点会自动跑」（需求 10746）。 -->
          <v-chip
            v-if="overview?.cdeu_mode === 'Auto'"
            class="workflow-meta-chip"
            size="small"
            :color="workflow.schedule_enabled ? 'info' : 'grey'"
            variant="tonal"
            prepend-icon="mdi-clock-outline"
          >
            <span class="workflow-meta-label">每日调度</span>
            <span class="workflow-meta-value">{{ workflow.schedule_enabled ? (workflow.schedule_time || '-') : '未启用' }}</span>
            <span v-if="workflow.schedule_enabled" class="workflow-meta-sub">{{ workflow.schedule_timezone }}</span>
          </v-chip>
          <v-chip class="workflow-meta-chip" size="small" color="success" variant="tonal" prepend-icon="mdi-check-decagram-outline">
            <span class="workflow-meta-label">完成条件</span>
            <span class="workflow-meta-value">{{ completionModeText(workflow.completion_rule) }}</span>
          </v-chip>
          <v-chip class="workflow-meta-chip" size="small" color="warning" variant="tonal" prepend-icon="mdi-shield-alert-outline">
            <span class="workflow-meta-label">失败策略</span>
            <span class="workflow-meta-value">继续执行</span>
          </v-chip>
          <v-tooltip location="bottom" max-width="420" open-delay="200">
            <template #activator="{ props }">
              <v-btn v-bind="props" icon="mdi-information-outline" size="x-small" variant="text" color="medium-emphasis" aria-label="工作流定义详情" />
            </template>
            <div class="phase-tooltip">
              <div class="font-weight-bold mb-1">{{ workflow.name }} · {{ workflow.workflow_code }} v{{ workflow.version }}</div>
              <div>完成条件：{{ workflow.completion_rule }}</div>
              <div>业务时区：{{ workflow.schedule_timezone }}</div>
              <div>定时触发：{{ overview?.cdeu_mode === 'Auto' ? `启用（每日 ${workflow.schedule_time || '-'}）` : '未启用（20005=Manual，仅手动触发）' }}</div>
              <div>批处理失败策略：CONTINUE</div>
              <div>最近查询：{{ fmtISO(overview?.generated_at) }}</div>
              <div class="mt-1">5–8 为 RUN_DUE_EXPORT_JOBS 的内嵌阶段，当前按钮触发完整导出 JOB，单阶段重跑待实现。</div>
            </div>
          </v-tooltip>
        </div>
        <div v-else class="text-body-2 text-medium-emphasis mt-1">加载工作流定义中…</div>
      </div>
      <div class="d-flex flex-wrap align-center ga-2">
        <v-btn
          variant="tonal"
          color="primary"
          prepend-icon="mdi-graph-outline"
          href="/Meta/Finv/Quote/Cdeu/CdeuWorkflow"
          target="_blank"
          rel="noopener noreferrer"
          title="在新窗口打开全流程拓扑"
        >
          全流程拓扑
        </v-btn>
        <v-btn color="primary" prepend-icon="mdi-play-circle" :loading="starting" :disabled="!canStart" @click="startDialog = true">
          启动工作流
        </v-btn>
        <v-btn
          color="error"
          variant="tonal"
          prepend-icon="mdi-stop-circle-outline"
          :loading="terminating"
          :disabled="!canTerminate"
          title="强制终止当前运行：把所有未终态的阶段/流程/JOB/步骤置为已取消"
          @click="terminateDialog = true"
        >
          强制终止
        </v-btn>
        <v-switch v-model="autoRefresh" color="primary" density="compact" hide-details label="自动刷新" />
        <v-btn variant="tonal" prepend-icon="mdi-refresh" :loading="loading" :title="overview ? `最近查询 ${fmtISO(overview.generated_at)}` : ''" @click="load()">刷新</v-btn>
        <v-btn v-if="selectedRunId" variant="text" prepend-icon="mdi-history" @click="loadLatest()">回到最新</v-btn>
      </div>
    </div>

    <v-alert v-if="error" type="warning" variant="tonal" class="mb-4" closable @click:close="error = ''">
      {{ error }}
    </v-alert>

    <v-alert v-if="selectedRunId" type="info" variant="tonal" density="compact" class="mb-4">
      正在查看历史运行 {{ selectedHistoryLabel }}，JOB 触发已禁用；点击「回到最新」恢复。
    </v-alert>

    <v-row class="mb-1" dense>
      <v-col cols="12" sm="6" lg="3">
        <v-card variant="tonal" color="primary" height="100%">
          <v-card-text>
            <div class="d-flex align-center justify-space-between">
              <span class="text-caption text-medium-emphasis">工作流状态</span>
              <v-icon :icon="statusOf(activeStatus).icon" :color="statusOf(activeStatus).color" :class="{ 'spin-icon': activeStatus === 'RUNNING' }" />
            </div>
            <div class="d-flex align-center ga-2 mt-1">
              <span class="text-h5 font-weight-bold">{{ statusOf(activeStatus).text }}</span>
              <v-chip v-if="run" size="x-small" :color="statusOf(run.status).color" variant="flat">{{ triggerText(run.trigger_type) }}</v-chip>
            </div>
            <div class="text-caption text-medium-emphasis mt-1">
              {{ run ? `业务日 ${run.biz_date} · 运行 #${run.run_id}` : '尚未产生运行实例' }}
            </div>
          </v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" sm="6" lg="3">
        <v-card variant="tonal" height="100%">
          <v-card-text>
            <div class="text-caption text-medium-emphasis">阶段进度（参与完成判定）</div>
            <div class="d-flex align-baseline justify-space-between mt-1">
              <span class="text-h5 font-weight-bold">{{ phaseStats.success }} / {{ phaseStats.total }}</span>
              <span class="text-caption text-medium-emphasis">失败 {{ phaseStats.failed }} · 执行中 {{ phaseStats.running }}</span>
            </div>
            <v-progress-linear class="mt-3" :model-value="overallProgress" color="primary" height="8" rounded />
          </v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" sm="6" lg="3">
        <v-card variant="tonal" color="success" height="100%">
          <v-card-text>
            <div class="text-caption text-medium-emphasis">JOB 进度（{{ jobStats.total }} 个 JOB / {{ jobStepStats.total }} 个 Step）</div>
            <div class="d-flex align-baseline justify-space-between mt-1">
              <span class="text-h5 font-weight-bold">{{ jobStats.success }} / {{ jobStats.total }}</span>
              <span class="text-caption text-medium-emphasis">失败 {{ jobStats.failed }} · 待执行 {{ jobStats.pending }}</span>
            </div>
            <v-progress-linear class="mt-3" :model-value="progressPercent(jobStats.success, jobStats.total)" color="success" height="8" rounded />
          </v-card-text>
        </v-card>
      </v-col>
      <v-col cols="12" sm="6" lg="3">
        <v-card variant="tonal" height="100%">
          <v-card-text>
            <div class="text-caption text-medium-emphasis">运行时长</div>
            <div class="text-h5 font-weight-bold mt-1">{{ run ? formatDuration(run.started_at, run.finished_at) : '-' }}</div>
            <div class="text-caption text-medium-emphasis mt-1">
              {{ run?.started_at ? `开始 ${fmtISO(run.started_at)}` : '等待首次触发' }}
            </div>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>

    <v-card class="mt-4" variant="outlined">
      <v-card-title class="d-flex align-center justify-space-between py-3">
        <div class="d-flex align-center ga-2">
          <v-icon icon="mdi-transit-connection-variant" color="primary" />
          <span class="text-subtitle-1 font-weight-bold">CDEU工作流</span>
          <v-chip v-if="activePhase" size="x-small" variant="tonal" color="primary">当前：{{ activePhase.name }}</v-chip>
        </div>
        <div class="d-flex flex-wrap align-center ga-3">
          <v-switch v-model="processTopologyVisible" color="primary" density="compact" hide-details label="流程拓扑" />
          <v-switch v-model="legendVisible" color="primary" density="compact" hide-details label="图例" />
          <v-btn-toggle v-model="viewMode" density="compact" variant="outlined" divided mandatory>
            <v-btn value="overall" size="small" prepend-icon="mdi-view-dashboard-outline">总体视图</v-btn>
            <v-btn value="detail" size="small" prepend-icon="mdi-sitemap-outline">阶段详情</v-btn>
          </v-btn-toggle>
          <v-menu location="bottom end" :close-on-content-click="true">
            <template #activator="{ props }">
              <v-btn
                v-bind="props"
                variant="outlined"
                size="small"
                prepend-icon="mdi-history"
                append-icon="mdi-chevron-down"
                class="run-select-btn"
              >
                {{ selectedHistoryLabel }}
              </v-btn>
            </template>
            <v-list density="compact" min-width="360" max-height="460" class="overflow-y-auto">
              <v-list-subheader>运行历史</v-list-subheader>
              <v-list-item :active="selectedRunId === 0" @click="loadLatest">
                <template #prepend>
                  <v-icon icon="mdi-update" color="primary" size="20" />
                </template>
                <v-list-item-title class="font-weight-medium">最新运行</v-list-item-title>
                <v-list-item-subtitle>
                  {{ latestRun ? `#${latestRun.run_id} · ${latestRun.biz_date} · ${triggerText(latestRun.trigger_type)}` : '暂无运行' }}
                </v-list-item-subtitle>
                <template #append>
                  <v-chip v-if="latestRun" size="x-small" :color="statusOf(latestRun.status).color" variant="tonal">
                    {{ statusOf(latestRun.status).text }}
                  </v-chip>
                </template>
              </v-list-item>
              <v-divider />
              <v-list-item
                v-for="item in previousRuns"
                :key="item.run_id"
                :active="selectedRunId === item.run_id"
                @click="selectRun(item)"
              >
                <template #prepend>
                  <v-icon :icon="statusOf(item.status).icon" :color="statusOf(item.status).color" size="20" :class="{ 'spin-icon': item.status === 'RUNNING' }" />
                </template>
                <v-list-item-title>
                  <span class="font-weight-medium">#{{ item.run_id }}</span>
                  <span class="text-caption text-medium-emphasis ml-2">{{ item.biz_date }}</span>
                </v-list-item-title>
                <v-list-item-subtitle>
                  {{ triggerText(item.trigger_type) }} · {{ formatDuration(item.started_at, item.finished_at) }}
                </v-list-item-subtitle>
                <template #append>
                  <v-chip size="x-small" :color="statusOf(item.status).color" variant="tonal">{{ statusOf(item.status).text }}</v-chip>
                </template>
              </v-list-item>
              <v-list-item v-if="!previousRuns.length">
                <v-list-item-title class="text-caption text-medium-emphasis">暂无更早运行</v-list-item-title>
              </v-list-item>
            </v-list>
          </v-menu>
        </div>
      </v-card-title>
      <v-divider />
      <v-card-text class="py-4">
        <div class="workflow-scroll">
          <div class="workflow-canvas" :style="{ width: `${workflowWidth}px`, height: `${workflowHeight}px` }">
            <svg
              class="workflow-arrows"
              :width="workflowWidth"
              :height="workflowHeight"
              :viewBox="`0 0 ${workflowWidth} ${workflowHeight}`"
            >
              <defs>
                <marker
                  v-for="color in EDGE_COLORS"
                  :id="`cdeu-arrow-${color.key}`"
                  :key="color.key"
                  markerWidth="5"
                  markerHeight="5"
                  refX="4"
                  refY="2.5"
                  orient="auto"
                  markerUnits="strokeWidth"
                >
                  <path d="M 0 0 L 5 2.5 L 0 5 z" :fill="color.color" />
                </marker>
              </defs>
              <path
                v-for="edge in workflowEdges"
                :key="edge.key"
                :d="edge.path"
                :stroke="edge.color"
                :marker-end="`url(#${edge.marker})`"
                :class="{ 'workflow-edge-running': edge.running }"
                fill="none"
                stroke-width="2"
                stroke-linecap="round"
              />
            </svg>
            <div
              v-for="terminal in workflowTerminals"
              :key="terminal.key"
              class="workflow-terminal-wrap"
              :style="{ left: `${terminal.x}px`, top: `${terminal.y}px`, width: `${WORKFLOW_TERMINAL_WIDTH}px`, height: `${WORKFLOW_NODE_HEIGHT}px` }"
            >
              <v-tooltip location="top" max-width="360" open-delay="200">
                <template #activator="{ props }">
                  <div v-bind="props" class="workflow-terminal" :class="`status-${statusOf(terminal.status).color}`">
                    <v-icon :icon="terminal.icon" :color="statusOf(terminal.status).color" size="22" />
                    <div class="workflow-terminal-text">
                      <span class="workflow-terminal-label">{{ terminal.label }}</span>
                      <span class="workflow-terminal-state">{{ terminal.stateText }}</span>
                    </div>
                  </div>
                </template>
                <div class="phase-tooltip">{{ terminal.tooltip }}</div>
              </v-tooltip>
            </div>
            <div
              v-for="node in workflowNodes"
              :key="node.phase.phase_code"
              class="workflow-node-wrap"
              :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${WORKFLOW_NODE_WIDTH}px`, height: `${WORKFLOW_NODE_HEIGHT}px` }"
            >
              <v-tooltip location="top" max-width="440" open-delay="200">
                <template #activator="{ props }">
                  <div
                    v-bind="props"
                    class="workflow-node"
                    :class="[`status-${phaseStatus(node.phase).color}`, { 'phase-selected': selectedPhaseCode === node.phase.phase_code, 'workflow-node-running': node.phase.status === 'RUNNING' }]"
                    @click="selectPhase(node.phase)"
                  >
                    <div class="workflow-node-head">
                      <span class="workflow-node-seq">{{ node.phase.seq_no }}</span>
                      <span class="workflow-node-title">{{ node.phase.name }}</span>
                      <v-icon :icon="phaseStatus(node.phase).icon" :color="phaseStatus(node.phase).color" size="17" :class="{ 'spin-icon': node.phase.status === 'RUNNING' }" />
                    </div>
                    <div class="workflow-node-status">
                      <span class="running-dot" v-if="node.phase.status === 'RUNNING'" />
                      <span>{{ phaseStatus(node.phase).text }}</span>
                      <span v-if="node.phase.affects_completion">{{ node.phase.success_count }}/{{ node.phase.total_count }}</span>
                      <v-chip v-else size="x-small" variant="text" color="grey">旁路</v-chip>
                    </div>
                    <div class="workflow-node-foot">
                      <span>JOB {{ phaseJobStats(node.phase).success }}/{{ phaseJobStats(node.phase).total }}</span>
                      <span>{{ formatDuration(node.phase.started_at, node.phase.finished_at) }}</span>
                    </div>
                    <div class="workflow-node-progress">
                      <v-progress-linear
                        :model-value="phaseProgress(node.phase)"
                        :color="phaseStatus(node.phase).color"
                        height="4"
                        rounded
                        class="workflow-node-progress-bar"
                      />
                      <span class="workflow-node-percent">{{ fmtPct(phaseProgress(node.phase)) }}</span>
                    </div>
                  </div>
                </template>
                <div class="phase-tooltip">
                  <div class="font-weight-bold mb-1">{{ node.phase.name }} · {{ node.phase.phase_code }}</div>
                  <div>状态：{{ phaseStatus(node.phase).text }} · {{ runModeText(node.phase.run_mode) }}</div>
                  <div>Process：{{ node.phase.success_count }}/{{ node.phase.total_count }}（失败 {{ node.phase.failed_count }}）</div>
                  <div>JOB：{{ phaseJobStats(node.phase).success }}/{{ phaseJobStats(node.phase).total }}（失败 {{ phaseJobStats(node.phase).failed }}，执行中 {{ phaseJobStats(node.phase).running }}）</div>
                  <div>开始：{{ fmtISO(node.phase.started_at) }} · 耗时：{{ formatDuration(node.phase.started_at, node.phase.finished_at) }}</div>
                  <div v-if="node.phase.processes.length">Process：{{ phaseProcessSummary(node.phase) }}</div>
                  <div v-if="!node.phase.affects_completion">旁路阶段，不参与工作流完成判定</div>
                </div>
              </v-tooltip>
            </div>
          </div>
        </div>
        <div v-if="selectedPhase && processTopologyVisible" class="process-topology">
          <div class="process-topology-head">
            <div class="d-flex align-center ga-2">
              <v-icon icon="mdi-family-tree" color="primary" size="18" />
              <span class="text-subtitle-2 font-weight-bold">{{ selectedPhase.name }} · Process 拓扑</span>
              <v-chip size="x-small" variant="tonal" color="primary">{{ selectedPhase.processes.length }} 个 Process</v-chip>
            </div>
            <span class="text-caption text-medium-emphasis">上一阶段输出 → 本阶段 Process → 下一阶段输入</span>
          </div>
          <div v-if="selectedPhase.processes.length" class="process-topology-scroll">
            <div
              class="process-topology-canvas"
              :style="{ width: `${processTopologyWidth}px`, height: `${processTopologyHeight}px` }"
            >
              <svg
                class="process-topology-arrows"
                :width="processTopologyWidth"
                :height="processTopologyHeight"
                :viewBox="`0 0 ${processTopologyWidth} ${processTopologyHeight}`"
              >
                <path
                  v-for="edge in processTopologyEdges"
                  :key="edge.key"
                  :d="edge.path"
                  :stroke="edge.color"
                  :marker-end="`url(#${edge.marker})`"
                  :class="{ 'workflow-edge-running': edge.running }"
                  fill="none"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
              <div
                v-for="terminal in processTopologyTerminals"
                :key="terminal.key"
                class="process-topology-terminal-wrap"
                :style="{ left: `${terminal.x}px`, top: `${terminal.y}px`, width: `${PROCESS_TOPOLOGY_TERMINAL_WIDTH}px`, height: `${PROCESS_TOPOLOGY_NODE_HEIGHT}px` }"
              >
                <div class="process-topology-terminal" :class="`status-${statusOf(terminal.status).color}`">
                  <span class="process-topology-terminal-label">{{ terminal.label }}</span>
                  <span class="process-topology-terminal-name">{{ terminal.name }}</span>
                </div>
              </div>
              <div
                v-for="node in processTopologyNodes"
                :key="node.process.process_code"
                class="process-topology-node-wrap"
                :style="{ left: `${node.x}px`, top: `${node.y}px`, width: `${PROCESS_TOPOLOGY_NODE_WIDTH}px`, height: `${PROCESS_TOPOLOGY_NODE_HEIGHT}px` }"
              >
                <div
                  class="process-topology-node"
                  :class="[`status-${processStatus(node.process).color}`, { 'process-selected': selectedProcessCode === node.process.process_code, 'workflow-node-running': node.process.status === 'RUNNING' }]"
                  @click="selectProcess(node.process)"
                >
                  <div class="process-topology-node-head">
                    <span class="process-topology-seq">{{ node.process.seq_no }}</span>
                    <span class="process-topology-name">{{ node.process.name }}</span>
                    <v-icon :icon="processStatus(node.process).icon" :color="processStatus(node.process).color" size="15" :class="{ 'spin-icon': node.process.status === 'RUNNING' }" />
                  </div>
                  <div class="process-topology-node-meta">
                    <span>{{ processStatus(node.process).text }}</span>
                    <span>JOB {{ processJobStats(node.process).success }}/{{ processJobStats(node.process).total }}</span>
                  </div>
                  <div class="workflow-node-progress">
                    <v-progress-linear
                      :model-value="processProgress(node.process)"
                      :color="processStatus(node.process).color"
                      height="3"
                      rounded
                      class="workflow-node-progress-bar"
                    />
                    <span class="workflow-node-percent">{{ fmtPct(processProgress(node.process)) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div v-else class="text-caption text-medium-emphasis py-3">该阶段暂无 Process，仅保留上下阶段连接关系。</div>
        </div>
        <v-expand-transition>
          <div v-if="legendVisible" class="workflow-legend">
            <span class="text-caption font-weight-bold text-medium-emphasis mr-1">图例</span>
            <v-chip
              v-for="meta in Object.values(STATUS_META)"
              :key="meta.text"
              size="small"
              :color="meta.color"
              variant="tonal"
            >
              <v-icon :icon="meta.icon" start size="14" :class="{ 'spin-icon': meta.text === '执行中' }" />{{ meta.text }}
            </v-chip>
          </div>
        </v-expand-transition>
      </v-card-text>
    </v-card>

    <v-row class="mt-4">
      <v-col cols="12">
        <v-card v-if="viewMode === 'overall'" variant="outlined" class="mb-4">
          <v-card-title class="d-flex flex-wrap align-center justify-space-between ga-2 py-3">
            <div class="d-flex align-center ga-2">
              <v-icon icon="mdi-view-dashboard-outline" color="primary" />
              <span class="text-subtitle-1 font-weight-bold">流程概览</span>
              <v-chip v-if="selectedPhase" size="small" color="primary" variant="tonal">
                {{ selectedPhase.seq_no }} · {{ selectedPhase.name }}
              </v-chip>
            </div>
          </v-card-title>
          <v-divider />
          <v-table v-if="selectedPhase" class="process-table" density="compact" hover :style="{ minWidth: `${processTableMinWidth}px` }">
            <thead>
              <tr>
                <th style="width: 54px">序号</th>
                <th :style="{ width: `${processNameWidth}px` }">
                  Process 名称
                  <span class="col-resizer" @mousedown.stop.prevent="startColumnResize('processName', $event)" />
                </th>
                <th style="width: 100px">标识</th>
                <th :style="{ width: `${processKeyWidth}px`, whiteSpace: 'nowrap' }">
                  Process KEY
                  <span class="col-resizer" @mousedown.stop.prevent="startColumnResize('processKey', $event)" />
                </th>
                <th style="width: 110px">状态</th>
                <th style="width: 84px">JOB总数</th>
                <th style="width: 72px">成功</th>
                <th style="width: 72px">失败</th>
                <th style="width: 76px">执行中</th>
                <th style="width: 84px">批次总数</th>
                <th style="width: 84px">批次失败</th>
                <th style="width: 170px">开始时间</th>
                <th style="width: 110px">耗时</th>
                <th>异常</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="process in selectedPhase.processes"
                :key="process.process_code"
                class="process-row-clickable"
                :class="{ 'process-row-selected': selectedProcessCode === process.process_code, 'process-row-running': process.status === 'RUNNING' }"
                @click="selectProcess(process)"
              >
                <td>{{ process.seq_no }}</td>
                <td>
                  <div class="font-weight-medium">
                    {{ process.name }}
                    <span v-if="process.status === 'RUNNING'" class="running-dot" title="执行中" />
                  </div>
                </td>
                <td>
                  <v-chip v-if="process.critical" size="x-small" color="error" variant="text">关键</v-chip>
                  <v-chip v-if="!process.affects_completion" size="x-small" color="grey" variant="text">旁路</v-chip>
                  <span v-if="!process.critical && process.affects_completion" class="text-caption text-medium-emphasis">-</span>
                </td>
                <td class="text-caption nowrap-cell">
                  {{ process.process_code }}
                  <div v-if="process.batch_enabled" class="text-medium-emphasis">{{ batchSourceText(process.batch_source) }}</div>
                </td>
                <td>
                  <v-chip size="small" :color="processStatus(process).color" variant="tonal">
                    <v-icon :icon="processStatus(process).icon" start size="14" :class="{ 'spin-icon': process.status === 'RUNNING' }" />{{ processStatus(process).text }}
                  </v-chip>
                </td>
                <td class="text-center">{{ processJobStats(process).total }}</td>
                <td class="text-center">
                  <span v-if="processJobStats(process).success" class="text-success font-weight-medium">{{ processJobStats(process).success }}</span>
                  <span v-else class="text-medium-emphasis">-</span>
                </td>
                <td class="text-center">
                  <span v-if="processJobStats(process).failed" class="text-error font-weight-medium">{{ processJobStats(process).failed }}</span>
                  <span v-else class="text-medium-emphasis">-</span>
                </td>
                <td class="text-center">
                  <span v-if="processJobStats(process).running" class="text-primary font-weight-medium">{{ processJobStats(process).running }}</span>
                  <span v-else class="text-medium-emphasis">-</span>
                </td>
                <td class="text-center">
                  <span v-if="process.batch_total">{{ process.batch_total }}</span>
                  <span v-else class="text-medium-emphasis">-</span>
                </td>
                <td class="text-center">
                  <span v-if="process.batch_failed" class="text-error font-weight-medium">{{ process.batch_failed }}</span>
                  <span v-else class="text-medium-emphasis">-</span>
                </td>
                <td class="text-caption">{{ processStartedText(process) }}</td>
                <td class="text-caption">{{ formatDuration(process.started_at, process.finished_at, true) }}</td>
                <td class="text-caption text-error error-cell" :title="process.error || ''">{{ process.error || '-' }}</td>
              </tr>
              <tr v-if="!selectedPhase.processes.length">
                <td colspan="14" class="text-caption text-medium-emphasis">该阶段暂无 Process 定义</td>
              </tr>
            </tbody>
          </v-table>
          <template v-if="selectedProcess">
            <v-divider />
            <div class="d-flex flex-wrap align-center justify-space-between ga-2 px-4 py-2">
              <div class="d-flex align-center ga-2">
                <v-icon icon="mdi-briefcase-outline" color="primary" size="18" />
                <span class="text-subtitle-2 font-weight-bold">JOB 清单 · {{ selectedProcess.name }}</span>
                <v-chip size="x-small" variant="tonal" color="primary">{{ selectedProcess.jobs.length }} 个 JOB</v-chip>
                <v-chip v-if="!selectedProcess.affects_completion" size="x-small" color="grey" variant="text">旁路 Process</v-chip>
              </div>
              <div class="d-flex align-center ga-2">
                <span class="text-caption text-medium-emphasis">列较多，可横向滚动查看右侧列</span>
                <span class="text-caption text-medium-emphasis">{{ selectedProcess.process_code }}</span>
              </div>
            </div>
            <div
              v-if="selectedProcess.jobs.length"
              class="job-table-scroll"
              :style="{ '--job-table-min-width': `${jobTableMinWidth}px` }"
            >
              <v-table
                class="job-table"
                density="compact"
                hover
                :style="{ minWidth: `${jobTableMinWidth}px` }"
              >
              <thead>
                <tr>
                  <th :style="{ width: `${jobNameWidth}px` }">
                    JOB 名称
                    <span class="col-resizer" @mousedown.stop.prevent="startColumnResize('jobName', $event)" />
                  </th>
                  <th style="width: 110px; white-space: nowrap">证券</th>
                  <th style="width: 90px; white-space: nowrap">年月</th>
                  <th style="width: 80px; white-space: nowrap">类型</th>
                  <th :style="{ width: `${jobCodeWidth}px` }">
                    JOB 编码
                    <span class="col-resizer" @mousedown.stop.prevent="startColumnResize('jobCode', $event)" />
                  </th>
                  <th :style="{ width: `${jobKindWidth}px` }">
                    调度标识
                    <span class="col-resizer" @mousedown.stop.prevent="startColumnResize('jobKind', $event)" />
                  </th>
                  <th style="width: 90px; white-space: nowrap">触发方式</th>
                  <th style="width: 80px; white-space: nowrap">执行次数</th>
                  <th style="width: 150px; white-space: nowrap">Step</th>
                  <th class="stage-text" style="width: 90px; white-space: nowrap">执行阶段</th>
                  <th style="width: 95px; white-space: nowrap">状态</th>
                  <th style="width: 155px; white-space: nowrap">开始时间</th>
                  <th style="width: 110px; white-space: nowrap">耗时</th>
                  <th style="width: 90px; white-space: nowrap">处理量</th>
                  <th style="width: 160px; white-space: nowrap">异常</th>
                  <th style="width: 180px; white-space: nowrap">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="job in selectedProcess.jobs" :key="job.is_dynamic ? `dyn-${job.dynamic_job_id}` : job.job_id">
                  <td class="font-weight-medium">{{ job.name }}</td>
                  <td class="text-caption nowrap-cell">{{ job.usc || '-' }}</td>
                  <td class="text-caption nowrap-cell">{{ job.biz_month || '-' }}</td>
                  <td class="text-caption nowrap-cell">{{ job.export_type || job.file_type || '-' }}</td>
                  <td class="text-caption nowrap-cell">{{ job.job_code }}</td>
                  <td class="text-caption nowrap-cell">{{ job.job_kind }}</td>
                  <td class="text-caption nowrap-cell">{{ JOB_TRIGGER_NAME[job.trigger_mode] ?? job.trigger_mode }}</td>
                  <td class="text-caption nowrap-cell">{{ job.instances || '-' }}</td>
                  <td class="text-caption nowrap-cell" :title="jobStepSummary(job)">
                    <v-chip
                      v-if="currentJobStep(job)"
                      size="x-small"
                      :color="statusOf(currentJobStep(job)!.status).color"
                      variant="tonal"
                    >
                      {{ currentJobStep(job)!.seq_no }}/{{ job.steps.length }} · {{ currentJobStep(job)!.step_code }}
                    </v-chip>
                    <span v-else>-</span>
                  </td>
                  <td>
                    <v-chip v-if="job.execution_stage" size="x-small" color="deep-purple" variant="tonal" class="stage-text">{{ job.execution_stage }}</v-chip>
                    <span v-else class="text-caption stage-text">{{ job.execution_mode }}</span>
                  </td>
                  <td>
                    <v-chip size="small" :color="statusOf(job.status).color" variant="tonal">
                      <v-icon :icon="statusOf(job.status).icon" start size="14" :class="{ 'spin-icon': job.status === 'RUNNING' }" />{{ statusOf(job.status).text }}
                    </v-chip>
                  </td>
                  <td class="text-caption nowrap-cell">{{ jobStartedText(job) }}</td>
                  <td class="text-caption nowrap-cell">{{ formatDuration(job.started_at, job.finished_at, true) }}</td>
                  <td class="text-caption">
                    <div v-if="job.processed_rows">行 {{ fmtNum(job.processed_rows, 0) }}</div>
                    <div v-if="job.processed_bytes">{{ fmtBytes(job.processed_bytes) }}</div>
                    <span v-if="!job.processed_rows && !job.processed_bytes">-</span>
                  </td>
                  <td class="text-caption text-error error-cell" :title="job.error || ''">{{ job.error || '-' }}</td>
                  <td>
                    <div class="d-flex flex-wrap align-center ga-1">
                      <v-btn
                        v-if="job.run_log_id"
                        size="x-small"
                        variant="text"
                        color="info"
                        prepend-icon="mdi-text-box-search-outline"
                        title="查看该 JOB 的详细执行日志"
                        @click.stop="openJobLog(job)"
                      >
                        执行日志
                      </v-btn>
                      <v-btn
                        v-if="showRunButton(job)"
                        size="x-small"
                        variant="tonal"
                        color="primary"
                        prepend-icon="mdi-play"
                        :loading="runningKind === jobRunKey(job)"
                        :disabled="Boolean(runningKind) || selectedRunId > 0"
                        @click.stop="runJob(job)"
                      >
                        {{ runButtonText(job) }}
                      </v-btn>
                      <span v-if="!job.run_log_id && !showRunButton(job)" class="text-caption text-medium-emphasis">-</span>
                    </div>
                  </td>
                </tr>
                </tbody>
              </v-table>
            </div>
            <v-card-text v-else class="text-caption text-medium-emphasis">该 Process 暂无 JOB 定义</v-card-text>
          </template>
          <v-card-text v-else-if="selectedPhase" class="text-caption text-medium-emphasis">
            点击上方 Process 查看该 Process 定义的 JOB 清单
          </v-card-text>
          <v-card-text v-else class="text-caption text-medium-emphasis">暂无阶段定义</v-card-text>
        </v-card>
        <template v-if="viewMode === 'detail'">
          <v-card
            v-for="phase in phases"
            :key="phase.phase_code"
            class="mb-4 phase-card"
            :class="{ 'phase-card-running': phase.status === 'RUNNING' }"
            variant="outlined"
          >
          <v-card-title class="d-flex flex-wrap align-center justify-space-between ga-2 py-3">
            <div class="d-flex align-center ga-2">
              <v-avatar size="26" :color="phaseStatus(phase).color" variant="tonal">
                <span class="text-caption font-weight-bold">{{ phase.seq_no }}</span>
              </v-avatar>
              <div>
                <div class="text-subtitle-1 font-weight-bold">{{ phase.name }}</div>
                <div class="text-caption text-medium-emphasis">{{ phase.phase_code }} · {{ runModeText(phase.run_mode) }}</div>
              </div>
              <v-chip size="small" :color="phaseStatus(phase).color" variant="flat">{{ phaseStatus(phase).text }}</v-chip>
              <v-chip v-if="!phase.affects_completion" size="x-small" color="grey" variant="tonal">不参与完成判定</v-chip>
            </div>
            <div class="d-flex align-center ga-3">
              <div class="text-caption text-medium-emphasis">
                {{ phase.success_count }}/{{ phase.total_count }} Process · 失败 {{ phase.failed_count }}
              </div>
              <div class="d-flex align-center ga-2" style="min-width: 168px">
                <v-progress-linear :model-value="phaseProgress(phase)" :color="phaseStatus(phase).color" height="7" rounded class="flex-grow-1" />
                <span class="progress-percent">{{ fmtPct(phaseProgress(phase)) }}</span>
              </div>
            </div>
          </v-card-title>
          <v-divider />
          <v-card-text class="pa-0">
            <div v-for="process in phase.processes" :key="process.process_code" class="process-block">
              <div
                class="d-flex flex-wrap align-center justify-space-between ga-2 px-4 py-3 process-header"
                :class="{ 'process-header-running': process.status === 'RUNNING' }"
              >
                <div class="d-flex align-center ga-2">
                  <v-icon :icon="processStatus(process).icon" :color="processStatus(process).color" size="20" :class="{ 'spin-icon': process.status === 'RUNNING' }" />
                  <div>
                    <div class="font-weight-medium">
                      {{ process.name }}
                      <span v-if="process.status === 'RUNNING'" class="running-dot" title="执行中" />
                      <v-chip v-if="process.critical" size="x-small" color="error" variant="text">关键</v-chip>
                      <v-chip v-if="!process.affects_completion" size="x-small" color="grey" variant="text">旁路</v-chip>
                    </div>
                    <div class="text-caption text-medium-emphasis">
                      {{ process.process_code }}
                      <span v-if="process.batch_enabled"> · 批处理 {{ batchSourceText(process.batch_source) }}</span>
                      <span v-if="process.failure_policy === 'CONTINUE'"> · 失败继续</span>
                    </div>
                  </div>
                </div>
                <div class="d-flex align-center ga-3">
                  <span class="text-caption text-medium-emphasis">
                    {{ process.batch_success }}/{{ process.batch_total }} 批次
                    <span v-if="process.batch_failed"> · 失败 {{ process.batch_failed }}</span>
                  </span>
                  <v-chip size="small" :color="processStatus(process).color" variant="tonal">{{ processStatus(process).text }}</v-chip>
                  <div class="d-flex align-center ga-2" style="width: 128px">
                    <v-progress-linear :model-value="processProgress(process)" :color="processStatus(process).color" height="6" rounded class="flex-grow-1" />
                    <span class="progress-percent">{{ fmtPct(processProgress(process)) }}</span>
                  </div>
                </div>
              </div>
              <div v-for="job in process.jobs" :key="job.is_dynamic ? `dyn-${job.dynamic_job_id}` : job.job_id" class="job-row px-4 py-2">
                <div class="d-flex flex-wrap align-center justify-space-between ga-2">
                  <div class="d-flex align-center ga-2">
                    <v-icon :icon="statusOf(job.status).icon" :color="statusOf(job.status).color" size="18" :class="{ 'spin-icon': job.status === 'RUNNING' }" />
                    <span class="font-weight-medium">{{ job.name }}</span>
                    <v-chip v-if="job.execution_stage" size="x-small" color="deep-purple" variant="tonal" class="stage-text">{{ job.execution_stage }}</v-chip>
                    <v-chip v-if="job.trigger_mode === 'EMBEDDED'" size="x-small" color="blue-grey" variant="text">内嵌</v-chip>
                    <span class="text-caption text-medium-emphasis">{{ job.job_kind }}</span>
                    <span v-if="job.is_dynamic" class="text-caption text-medium-emphasis">
                      {{ job.usc }} · {{ job.biz_month }} · {{ job.export_type || job.file_type }}
                    </span>
                  </div>
                  <div class="d-flex flex-wrap align-center ga-2">
                    <v-chip size="x-small" :color="statusOf(job.status).color" variant="flat">{{ statusOf(job.status).text }}</v-chip>
                    <span class="text-caption text-medium-emphasis">
                      {{ job.attempt > 0 ? `第 ${job.attempt} 次` : '未运行' }}
                    </span>
                    <span class="text-caption text-medium-emphasis">{{ formatDuration(job.started_at, job.finished_at, true) }}</span>
                    <v-btn
                      v-if="job.run_log_id"
                      size="x-small"
                      variant="text"
                      color="info"
                      prepend-icon="mdi-text-box-search-outline"
                      @click="openJobLog(job)"
                    >
                      执行日志
                    </v-btn>
                    <v-btn
                      v-if="showRunButton(job)"
                      size="x-small"
                      variant="tonal"
                      color="primary"
                      prepend-icon="mdi-play"
                      :loading="runningKind === jobRunKey(job)"
                      :disabled="Boolean(runningKind) || selectedRunId > 0"
                      @click="runJob(job)"
                    >
                      {{ runButtonText(job) }}
                    </v-btn>
                  </div>
                </div>
                <div class="d-flex flex-wrap ga-4 mt-1 text-caption text-medium-emphasis job-meta">
                  <span>开始 {{ jobStartedText(job) }}</span>
                  <span>结束 {{ fmtISO(job.finished_at) }}</span>
                  <span v-if="job.processed_rows">行 {{ fmtNum(job.processed_rows, 0) }}</span>
                  <span v-if="job.processed_bytes">字节 {{ fmtBytes(job.processed_bytes) }}</span>
                  <span v-if="job.batch_key">批次 {{ job.batch_key }}</span>
                  <span v-if="job.run_log_id">日志 #{{ job.run_log_id }}</span>
                  <span v-if="job.error" class="text-error">异常：{{ job.error }}</span>
                </div>
                <div v-if="job.steps.length" class="job-step-list">
                  <span class="text-caption font-weight-bold text-medium-emphasis mr-2">Step</span>
                  <v-chip
                    v-for="step in job.steps"
                    :key="step.step_id"
                    size="x-small"
                    :color="statusOf(step.status).color"
                    variant="tonal"
                    class="mr-1 mb-1"
                    :title="`${step.name} · ${step.step_code}${step.error ? ` · ${step.error}` : ''}`"
                  >
                    <v-icon :icon="statusOf(step.status).icon" start size="12" :class="{ 'spin-icon': step.status === 'RUNNING' }" />
                    {{ step.name }} · {{ step.step_code }} · {{ statusOf(step.status).text }}
                  </v-chip>
                </div>
              </div>
            </div>
            <div v-if="!phase.processes.length" class="px-4 py-4 text-caption text-medium-emphasis">该阶段暂无 Process 定义</div>
          </v-card-text>
          </v-card>
        </template>
      </v-col>

    </v-row>

    <v-dialog
      v-model="runDialog"
      max-width="580"
      persistent
      @keydown.esc="runDialog = false"
    >
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon icon="mdi-play-circle" color="primary" />
          手动执行 JOB
        </v-card-title>
        <v-divider />
        <v-card-text>
          <div class="d-flex flex-wrap align-center ga-2 mb-3">
            <span class="font-weight-medium">{{ runDialogJob?.name || '-' }}</span>
            <v-chip v-if="runDialogJob?.job_code" size="x-small" color="deep-purple" variant="tonal">{{ runDialogJob.job_code }}</v-chip>
            <v-chip v-if="runDialogJob?.is_dynamic" size="x-small" color="info" variant="tonal">动态 JOB</v-chip>
          </div>
          <v-checkbox
            v-model="autoContinue"
            label="自动续跑同流程下的后续 JOB"
            color="primary"
            density="compact"
            hide-details
          />
          <div class="text-caption text-medium-emphasis mt-1">
            默认仅执行当前 JOB；勾选后，当前 JOB 完成后按工作流顺序继续执行其后的 JOB。
          </div>
          <v-alert v-if="runDialogJob?.execution_mode === 'EMBEDDED'" class="mt-3" type="info" variant="tonal" density="compact">
            该入口会完整执行导出 JOB 的 LOAD → EXPORT → REGISTER → CLEANUP 内嵌阶段。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" :disabled="Boolean(runningKind)" @click="runDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="Boolean(runningKind)" @click="confirmRunJob">
            {{ autoContinue ? '执行并续跑' : '仅执行当前 JOB' }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog
      v-model="logDialog"
      :fullscreen="logFullscreen"
      :max-width="logFullscreen ? undefined : 1080"
      scrollable
      @keydown.esc="closeJobLog"
    >
      <v-card :class="{ 'job-log-card-fullscreen': logFullscreen }">
        <v-card-title class="d-flex flex-wrap align-center ga-2">
          <v-icon icon="mdi-text-box-search-outline" color="info" />
          <span class="text-truncate">{{ logTitle || 'JOB 执行日志' }}</span>
          <v-chip size="x-small" color="info" variant="tonal">日志 #{{ logRunID }}</v-chip>
          <v-chip v-if="logAutoContinue" size="x-small" color="success" variant="tonal" prepend-icon="mdi-playlist-play">自动续跑</v-chip>
          <v-chip v-if="logDetail?.job_kind" size="x-small" color="deep-purple" variant="tonal">{{ logDetail.job_kind }}</v-chip>
          <v-spacer />
          <v-btn
            :icon="logFullscreen ? 'mdi-arrow-collapse' : 'mdi-arrow-expand'"
            size="small"
            variant="text"
            :title="logFullscreen ? '退出全屏' : '全屏显示'"
            @click="logFullscreen = !logFullscreen"
          />
          <v-btn icon="mdi-refresh" size="small" variant="text" :loading="logLoading" title="刷新日志" @click="loadJobLog()" />
          <v-btn icon="mdi-close" size="small" variant="text" title="关闭" @click="closeJobLog" />
        </v-card-title>
        <v-divider />
        <v-card-subtitle v-if="logDetail" class="job-log-summary">
          <div class="d-flex flex-wrap align-center ga-2">
            <v-chip size="x-small" :color="statusOf(logDetail.status || 'PENDING').color" variant="tonal">
              {{ statusOf(logDetail.status || 'PENDING').text }}
            </v-chip>
            <span v-if="logDetail.stage">阶段 <b>{{ logDetail.stage }}</b></span>
            <span v-if="logDetail.progress_percent !== null && logDetail.progress_percent >= 0">进度 <b>{{ Math.round(logDetail.progress_percent) }}%</b></span>
            <span v-if="logDetail.usc">{{ logDetail.usc }} · {{ logDetail.biz_month }}</span>
            <span>开始 {{ fmtISO(logDetail.gmt_start) }}</span>
            <span>结束 {{ fmtISO(logDetail.gmt_finish) }}</span>
          </div>
          <div v-if="logDetail.error" class="text-error mt-1">异常：{{ logDetail.error }}</div>
          <div v-else-if="logDetail.progress_detail" class="mt-1">最近明细：{{ logDetail.progress_detail }}</div>
        </v-card-subtitle>
        <v-divider v-if="logDetail" />
        <v-card-text class="job-log-body" :class="{ 'job-log-body-fullscreen': logFullscreen }">
          <v-progress-linear v-if="logLoading && !logDetail" indeterminate color="info" />
          <v-alert v-if="logError" type="error" variant="tonal" density="compact" class="mb-2">{{ logError }}</v-alert>
          <div v-if="logDetail && !logDetail.lines.length" class="text-medium-emphasis py-4">
            该次运行暂无详细日志行（历史记录只保留了汇总信息）。
            <div v-if="logDetail.progress_detail" class="mt-2 font-monospace">最近明细：{{ logDetail.progress_detail }}</div>
          </div>
          <div v-else class="job-log-lines">
            <div v-for="line in logDetail?.lines || []" :key="line.line_id" class="job-log-line">
              <span class="job-log-time">{{ fmtISO(line.gmt_create) }}</span>
              <v-chip
                size="x-small"
                :color="line.level === 'ERROR' ? 'error' : line.level === 'WARN' ? 'warning' : 'info'"
                variant="tonal"
                class="job-log-level"
              >{{ line.level }}</v-chip>
              <span v-if="line.stage" class="job-log-stage">{{ line.stage }}</span>
              <span class="job-log-message">{{ line.message }}</span>
              <span v-if="line.progress_percent !== null && line.progress_percent >= 0" class="job-log-percent">{{ Math.round(line.progress_percent) }}%</span>
            </div>
          </div>
        </v-card-text>
        <v-card-actions>
          <span class="text-caption text-medium-emphasis">
            <span v-if="logAutoContinue && logDetail && isFinishedStatus(logDetail.status)">
              当前 JOB 已完成，后续 JOB 正在后台自动续跑；关闭弹窗可在 JOB 清单查看进度
            </span>
            <span v-else-if="logAutoContinue">已开启自动续跑，当前 JOB 结束后将继续执行后续 JOB</span>
            <span v-else-if="logDetail && !isFinishedStatus(logDetail.status)">执行中，日志每 2 秒自动刷新</span>
            <span v-else>共 {{ logDetail?.lines.length || 0 }} 行日志</span>
          </span>
          <v-spacer />
          <v-btn variant="text" @click="closeJobLog">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="startDialog" max-width="560" persistent>
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon icon="mdi-play-circle" color="primary" />
          启动 CDEU 工作流
        </v-card-title>
        <v-divider />
        <v-card-text class="text-body-2">
          <div>将新建一次 MANUAL 运行，并按 Phase → Process → JOB → Step 顺序执行：</div>
          <div class="mt-2 font-weight-medium">历史行情收集 → 历史行情去重 → 导出决策 → 月度行情导出（含补丁）→ 月度行情上传 → 归档复核</div>
          <v-alert class="mt-3" type="warning" variant="tonal" density="compact">
            导出 JOB 仍受低峰窗口限制；窗口外会快速跳过，其他阶段继续执行。已有运行中的工作流时不能重复启动。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" :disabled="starting" @click="startDialog = false">取消</v-btn>
          <v-btn color="primary" :loading="starting" @click="startWorkflow">确认启动</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="terminateDialog" max-width="560" persistent>
      <v-card>
        <v-card-title class="d-flex align-center ga-2">
          <v-icon icon="mdi-stop-circle-outline" color="error" />
          强制终止 CDEU 工作流
        </v-card-title>
        <v-divider />
        <v-card-text class="text-body-2">
          <div>
            将终止运行
            <span class="font-weight-bold">#{{ run?.run_id ?? '-' }}</span>
            （当前状态 {{ statusOf(run?.status ?? '').text || '-' }}），并把该运行下：
          </div>
          <ul class="mt-2 ms-4">
            <li>所有未终态的阶段（Phase）置为「已取消」</li>
            <li>所有未终态的流程（Process）置为「已取消」</li>
            <li>所有未终态的 JOB 及其步骤置为「已取消」</li>
            <li>建立流程的动态 JOB 实例一并置为「已取消」</li>
            <li>释放残留的导出任务锁，避免挡住后续启动</li>
          </ul>
          <v-alert class="mt-3" type="warning" variant="tonal" density="compact">
            已成功的记录不会被改写；但处于运行中的 JOB 会被立即中断，其未完成的导出/上传需要重新执行。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" :disabled="terminating" @click="terminateDialog = false">取消</v-btn>
          <v-btn color="error" :loading="terminating" @click="terminateWorkflow">确认终止</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar" :color="snackbarColor" timeout="4000">
      {{ snackbarText }}
    </v-snackbar>
  </div>
</template>

<style scoped>
.workflow-meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
}
.workflow-meta-chip {
  font-size: 12px;
}
.workflow-meta-chip :deep(.v-chip__content) {
  gap: 4px;
}
.workflow-meta-label {
  opacity: 0.62;
}
.workflow-meta-value {
  font-weight: 600;
}
.workflow-meta-sub {
  margin-left: 2px;
  font-size: 11px;
  opacity: 0.55;
}
.process-topology {
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px dashed rgba(128, 128, 128, 0.28);
}
.process-topology-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.process-topology-scroll {
  overflow-x: auto;
  padding-bottom: 4px;
}
.process-topology-canvas {
  position: relative;
  min-width: 100%;
  border-radius: 10px;
  background-color: rgba(128, 128, 128, 0.035);
  background-image: radial-gradient(rgba(128, 128, 128, 0.16) 1px, transparent 1px);
  background-size: 14px 14px;
}
.process-topology-arrows {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.process-topology-terminal-wrap,
.process-topology-node-wrap {
  position: absolute;
}
.process-topology-terminal {
  box-sizing: border-box;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  padding: 0 6px;
  border: 1px solid #9e9e9e;
  border-radius: 6px;
  background: rgb(var(--v-theme-surface));
  text-align: center;
}
.process-topology-terminal-label {
  font-size: 10px;
  font-weight: 700;
  color: rgba(var(--v-theme-on-surface), 0.62);
}
.process-topology-terminal-name {
  font-size: 11px;
  font-weight: 600;
  max-width: 92px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.process-topology-terminal.status-success { border-color: #2e7d32; }
.process-topology-terminal.status-error { border-color: #d32f2f; }
.process-topology-terminal.status-warning { border-color: #f9a825; }
.process-topology-terminal.status-primary { border-color: #1976d2; }
.process-topology-terminal.status-info { border-color: #0288d1; }
.process-topology-node {
  box-sizing: border-box;
  height: 100%;
  padding: 7px 9px 6px;
  border: 1px dashed rgba(128, 128, 128, 0.62);
  border-top-width: 3px;
  border-left-width: 1px;
  border-radius: 5px;
  background: rgba(128, 128, 128, 0.055);
  box-shadow: none;
  cursor: pointer;
  transition: transform 0.16s ease, box-shadow 0.16s ease, background 0.16s ease;
}
.process-topology-node:hover {
  transform: translateY(-1px);
  background: rgba(128, 128, 128, 0.10);
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.14);
}
.process-topology-node.status-primary { border-top-color: #1976d2; }
.process-topology-node.status-success { border-top-color: #2e7d32; }
.process-topology-node.status-error { border-top-color: #d32f2f; }
.process-topology-node.status-warning { border-top-color: #f9a825; }
.process-topology-node.status-info { border-top-color: #0288d1; }
.process-topology-node.status-grey { border-top-color: #9e9e9e; }
.process-topology-node.process-selected {
  outline: 2px solid #1976d2;
  outline-offset: 1px;
  background: rgba(25, 118, 210, 0.10);
}
.process-topology-node-head {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}
.process-topology-seq {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 17px;
  height: 17px;
  border-radius: 50%;
  background: rgba(25, 118, 210, 0.16);
  color: #1976d2;
  font-size: 10px;
  font-weight: 700;
}
.process-topology-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 600;
}
/* 进度条 + 右侧百分比（需求 10748）：百分比与进度条宽度同源，便于一眼读出完成度 */
.workflow-node-progress {
  display: flex;
  align-items: center;
  gap: 6px;
}
.workflow-node-progress-bar {
  flex: 1 1 auto;
}
.workflow-node-percent,
.progress-percent {
  flex: none;
  min-width: 34px;
  text-align: right;
  font-size: 10px;
  font-variant-numeric: tabular-nums;
  color: rgba(128, 128, 128, 0.92);
}
.progress-percent {
  font-size: 11px;
}

.process-topology-node-meta {
  display: flex;
  justify-content: space-between;
  gap: 6px;
  margin: 5px 0 4px;
  font-size: 10px;
  color: rgba(var(--v-theme-on-surface), 0.62);
}
.workflow-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed rgba(128, 128, 128, 0.28);
}
.workflow-scroll {
  overflow-x: auto;
  padding-bottom: 4px;
}
.workflow-canvas {
  position: relative;
  min-width: 100%;
  border-radius: 12px;
  background-color: rgba(128, 128, 128, 0.045);
  background-image: radial-gradient(rgba(128, 128, 128, 0.20) 1px, transparent 1px);
  background-size: 16px 16px;
}
.workflow-arrows {
  position: absolute;
  inset: 0;
  pointer-events: none;
}
.workflow-node-wrap,
.workflow-terminal-wrap {
  position: absolute;
}
.workflow-terminal {
  box-sizing: border-box;
  height: 100%;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 8px;
  border: 2px solid #9e9e9e;
  border-radius: 8px;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.14);
  cursor: default;
}
.workflow-terminal-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 1px;
  min-width: 0;
  line-height: 1.2;
}
.workflow-terminal-label {
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}
.workflow-terminal-state {
  font-size: 10px;
  color: rgba(var(--v-theme-on-surface), 0.62);
  white-space: nowrap;
}
.workflow-terminal.status-success { border-color: #2e7d32; }
.workflow-terminal.status-error { border-color: #d32f2f; }
.workflow-terminal.status-warning { border-color: #f9a825; }
.workflow-terminal.status-grey { border-color: #9e9e9e; }
.workflow-terminal.status-info { border-color: #0288d1; }
.workflow-node {
  box-sizing: border-box;
  height: 100%;
  border: 1px solid rgba(128, 128, 128, 0.40);
  border-left-width: 5px;
  border-radius: 12px;
  padding: 9px 12px 8px;
  background: linear-gradient(180deg, rgba(128, 128, 128, 0.055), rgb(var(--v-theme-surface)) 38%);
  box-shadow: 0 5px 16px rgba(0, 0, 0, 0.18);
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease;
}
.workflow-node:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.22);
}
.workflow-node.phase-selected {
  outline: 2px solid #1976d2;
  outline-offset: 2px;
  background: rgba(25, 118, 210, 0.10);
}
.workflow-node.workflow-node-running {
  border-color: #1976d2;
  animation: workflowRunning 1.8s ease-in-out infinite;
}
.workflow-edge-running {
  stroke-dasharray: 10 6;
  animation: workflowEdgeFlow 0.9s linear infinite;
}
.running-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: rgb(0, 229, 255);
  box-shadow: 0 0 0 0 rgba(25, 118, 210, 0.7);
  animation: runningDot 1.4s ease-out infinite;
  vertical-align: middle;
}
@keyframes workflowRunning {
  0%, 100% {
    box-shadow: 0 0 0 1px rgba(25, 118, 210, 0.35), 0 8px 22px rgba(25, 118, 210, 0.18);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(25, 118, 210, 0.10), 0 10px 28px rgba(25, 118, 210, 0.42);
  }
}
@keyframes workflowEdgeFlow {
  to { stroke-dashoffset: -16; }
}
@keyframes runningDot {
  0% { box-shadow: 0 0 0 0 rgba(25, 118, 210, 0.65); }
  70% { box-shadow: 0 0 0 7px rgba(25, 118, 210, 0); }
  100% { box-shadow: 0 0 0 0 rgba(25, 118, 210, 0); }
}
.workflow-node-head {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}
.workflow-node-seq {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(25, 118, 210, 0.16);
  color: #1976d2;
  font-size: 11px;
  font-weight: 700;
}
.workflow-node-title {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 600;
}
.workflow-node-status {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 7px;
  font-size: 12px;
}
.workflow-node-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: 8px 0 6px;
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 11px;
}
.status-primary { border-left-color: #1976d2; }
.status-success { border-left-color: #2e7d32; }
.status-error { border-left-color: #d32f2f; }
.status-warning { border-left-color: #f9a825; }
.status-grey { border-left-color: #9e9e9e; }
.status-info { border-left-color: #0288d1; }
.phase-tooltip {
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.8;
}
.status-primary { border-left-color: #1976d2; }
.status-success { border-left-color: #2e7d32; }
.status-error { border-left-color: #d32f2f; }
.status-warning { border-left-color: #f9a825; }
.status-grey { border-left-color: #9e9e9e; }
.status-info { border-left-color: #0288d1; }
.process-block + .process-block {
  border-top: 1px solid rgba(0, 0, 0, 0.08);
}
/* 运行中 Phase 卡片的青色跑马灯。
   动效主体用 ::after 推一层斜向流光带，卡片内容抬到 z-index:1 之上不被遮住。 */
.phase-card {
  position: relative;
  overflow: hidden;
  isolation: isolate;
}
.phase-card > * {
  position: relative;
  z-index: 1;
}
.phase-card-running {
  border-color: rgba(0, 229, 255, 0.75) !important;
  animation: cdeuPhasePulse 1.8s ease-in-out infinite;
}
.phase-card-running::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: linear-gradient(
    115deg,
    transparent 32%,
    rgba(0, 229, 255, 0.14) 46%,
    rgba(0, 229, 255, 0.38) 50%,
    rgba(0, 229, 255, 0.14) 54%,
    transparent 68%
  );
  background-size: 220% 100%;
  animation: cdeuMarquee 2.2s linear infinite;
}
@keyframes cdeuMarquee {
  0% { background-position: 160% 0; }
  100% { background-position: -160% 0; }
}
@keyframes cdeuPhasePulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 229, 255, 0); }
  50% { box-shadow: 0 0 0 3px rgba(0, 229, 255, 0.28); }
}
.process-header {
  background: rgba(0, 0, 0, 0.018);
  position: relative;
  overflow: hidden;
  isolation: isolate;
}
.process-header > * {
  position: relative;
  z-index: 1;
}
.process-header-running {
  background: linear-gradient(90deg, rgba(0, 229, 255, 0.20), rgba(0, 229, 255, 0.02));
  box-shadow: inset 3px 0 0 rgb(0, 229, 255);
}
/* 运行中 Process 头部的青色跑马灯 */
.process-header-running::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: linear-gradient(
    115deg,
    transparent 32%,
    rgba(0, 229, 255, 0.18) 46%,
    rgba(0, 229, 255, 0.46) 50%,
    rgba(0, 229, 255, 0.18) 54%,
    transparent 68%
  );
  background-size: 220% 100%;
  animation: cdeuMarquee 2.2s linear infinite;
}
.job-row + .job-row {
  border-top: 1px dashed rgba(0, 0, 0, 0.09);
}
.job-row:last-child {
  padding-bottom: 12px !important;
}
.job-meta {
  padding-left: 26px;
}
.job-log-summary {
  font-size: 12px;
  padding-bottom: 8px;
}
.job-log-body {
  max-height: 62vh;
  overflow: auto;
}
.job-log-body-fullscreen {
  max-height: calc(100vh - 220px);
}
.job-log-card-fullscreen {
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.job-log-card-fullscreen .job-log-body {
  flex: 1 1 auto;
  min-height: 0;
}
.job-log-lines {
  display: flex;
  flex-direction: column;
  gap: 1px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.job-log-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 5px 6px;
  border-radius: 4px;
}
.job-log-line:nth-child(odd) {
  background: rgba(128, 128, 128, 0.045);
}
.job-log-time {
  flex: 0 0 142px;
  color: rgba(128, 128, 128, 0.95);
  font-size: 11px;
  white-space: nowrap;
}
.job-log-level {
  flex: 0 0 auto;
}
.job-log-stage {
  flex: 0 0 76px;
  color: rgb(var(--v-theme-primary));
  font-weight: 600;
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.job-log-message {
  flex: 1;
  min-width: 0;
  white-space: pre-wrap;
  word-break: break-word;
}
.job-log-percent {
  flex: 0 0 44px;
  text-align: right;
  color: rgba(128, 128, 128, 0.95);
  font-size: 11px;
}
.process-row-clickable {
  cursor: pointer;
}
.process-row-selected {
  background: rgba(25, 118, 210, 0.10);
}
.job-step-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  padding-left: 26px;
}
.nowrap-cell {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.job-code-line {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.job-table-scroll {
  width: 100%;
  overflow-x: scroll;
  overflow-y: hidden;
  padding-bottom: 4px;
  scrollbar-width: thin;
  scrollbar-color: rgba(128, 128, 128, 0.55) rgba(128, 128, 128, 0.12);
}
.job-table-scroll::-webkit-scrollbar {
  height: 14px;
  background: rgba(128, 128, 128, 0.12);
}
.job-table-scroll::-webkit-scrollbar-track {
  border-radius: 7px;
  background: rgba(128, 128, 128, 0.14);
}
.job-table-scroll::-webkit-scrollbar-thumb {
  border-radius: 7px;
  background: rgba(128, 128, 128, 0.55);
}
.process-table :deep(table),
.job-table :deep(table) {
  table-layout: fixed;
  width: 100%;
}
.job-table :deep(table) {
  min-width: var(--job-table-min-width);
}
.process-table :deep(th),
.job-table :deep(th) {
  position: relative;
  overflow: visible;
}
.process-table :deep(td),
.job-table :deep(td) {
  overflow: hidden;
}
.col-resizer {
  position: absolute;
  top: 0;
  right: -3px;
  z-index: 2;
  width: 8px;
  height: 100%;
  cursor: col-resize;
  touch-action: none;
}
.col-resizer::after {
  content: '';
  position: absolute;
  top: 20%;
  left: 3px;
  width: 1px;
  height: 60%;
  background: rgba(128, 128, 128, 0.45);
}
.col-resizer:hover::after {
  width: 2px;
  background: rgb(0, 229, 255);
}
.process-row-selected td:first-child {
  box-shadow: inset 3px 0 0 rgb(0, 229, 255);
}
.process-row-running {
  animation: stepRunningBg 1.8s ease-in-out infinite;
}
@keyframes stepRunningBg {
  0%, 100% { background: rgba(0, 229, 255, 0.05); }
  50% { background: rgba(0, 229, 255, 0.16); }
}
@media (prefers-reduced-motion: reduce) {
  .spin-icon { animation: none; }
  .workflow-node.workflow-node-running,
  .workflow-edge-running,
  .running-dot,
  .process-row-running,
  .phase-card-running,
  .process-header-running {
    animation: none;
  }
  /* 关闭跑马灯，只留静态青色描边，保留「正在跑」的视觉信息 */
  .phase-card-running::after,
  .process-header-running::after {
    animation: none;
    background: rgba(0, 229, 255, 0.10);
  }
}
.spin-icon {
  animation: spinIcon 1s linear infinite;
}
@keyframes spinIcon {
  to { transform: rotate(360deg); }
}
@media (prefers-reduced-motion: reduce) {
  .spin-icon { animation: none; }
}
.stage-text {
  color: rgb(251, 140, 0) !important;
}
</style>
