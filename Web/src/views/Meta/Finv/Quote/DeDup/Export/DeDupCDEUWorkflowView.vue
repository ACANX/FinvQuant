<script setup lang="ts">
// CDEU 全流程拓扑：阶段虚线框展开为 Process 节点，并绘制跨阶段依赖边
// 数据来源：GET /Meta/Finv/Quote/DeDup/CDEU/Overview
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../../api'
import { fmtISO } from '../../../../../../utils'

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
  biz_date: string
  trigger_type: string
  status: string
  started_at: string | null
  finished_at: string | null
  operator: string | null
  error: string | null
}

interface CDEUJob {
  job_code: string
  name: string
  seq_no: number
  status: string
  critical: boolean
  affects_completion: boolean
  // 动态单证券 JOB 扩展字段（is_dynamic=true 时有效）
  is_dynamic?: boolean
  from_current_run?: boolean
  usc?: string
  biz_month?: number
  export_type?: string
  job_category?: string
  file_type?: string | null
  file_version?: number | null
  error?: string | null
}

interface CDEUProcess {
  process_id: number
  process_code: string
  name: string
  seq_no: number
  critical: boolean
  affects_completion: boolean
  status: string
  batch_total: number
  batch_success: number
  batch_failed: number
  jobs: CDEUJob[]
}

interface CDEUPhase {
  phase_id: number
  phase_code: string
  name: string
  seq_no: number
  critical: boolean
  status: string
  total_count: number
  success_count: number
  failed_count: number
  processes: CDEUProcess[]
}

interface CDEUOverview {
  generated_at: string
  workflow: CDEUWorkflow
  run: CDEURun | null
  phases: CDEUPhase[]
}

const STATUS_META: Record<string, { text: string; hex: string; tone: string; icon: string }> = {
  NOT_STARTED: { text: '未运行', hex: '#9e9e9e', tone: 'grey', icon: 'mdi-circle-outline' },
  PENDING: { text: '待执行', hex: '#9e9e9e', tone: 'grey', icon: 'mdi-clock-outline' },
  READY: { text: '就绪', hex: '#0288d1', tone: 'info', icon: 'mdi-play-circle-outline' },
  RUNNING: { text: '执行中', hex: '#1976d2', tone: 'primary', icon: 'mdi-loading' },
  SUCCESS: { text: '成功', hex: '#2e7d32', tone: 'success', icon: 'mdi-check-circle-outline' },
  PARTIAL: { text: '部分完成', hex: '#f9a825', tone: 'warning', icon: 'mdi-alert-circle-outline' },
  FAILED: { text: '异常', hex: '#d32f2f', tone: 'error', icon: 'mdi-close-circle-outline' },
  PAUSED: { text: '已暂停', hex: '#f9a825', tone: 'warning', icon: 'mdi-pause-circle-outline' },
  CANCELLED: { text: '已取消', hex: '#9e9e9e', tone: 'grey', icon: 'mdi-cancel' },
  SKIPPED: { text: '已跳过', hex: '#9e9e9e', tone: 'grey', icon: 'mdi-skip-next-circle-outline' },
}

const EDGE_COLORS: Record<string, string> = {
  success: '#2e7d32',
  primary: '#1976d2',
  error: '#d32f2f',
  warning: '#f9a825',
  info: '#0288d1',
  grey: '#9e9e9e',
}

const NODE_WIDTH_MIN = 196
const NODE_HEIGHT = 80
const COL_GAP = 38
const LANE_GAP = 22
const PHASE_PAD = 16
const PHASE_HEADER = 52
const PHASE_GAP = 72
const START_WIDTH = 104
const OUTER_PAD = 20
const FORK_SPACE = 72
const FORK_SIZE = 10

interface ProcessPlacement {
  process: CDEUProcess
  col: number
  lane: number
}

interface LayoutNode {
  process: CDEUProcess
  phase: CDEUPhase
  x: number
  y: number
  width: number
  height: number
}

interface LayoutPhase {
  phase: CDEUPhase
  x: number
  y: number
  width: number
  height: number
  nodes: LayoutNode[]
  fork?: Box
}

interface Box {
  x: number
  y: number
  width: number
  height: number
}

interface TopoEdge {
  key: string
  path: string
  color: string
  marker: string
  running: boolean
  dashed: boolean
}

const overview = ref<CDEUOverview | null>(null)
const loading = ref(false)
const error = ref('')
const autoRefresh = ref(true)
let refreshTimer: number | undefined
let inFlight = false
let disposed = false

const workflow = computed(() => overview.value?.workflow ?? null)
const run = computed(() => overview.value?.run ?? null)
const phases = computed(() => [...(overview.value?.phases ?? [])].sort((a, b) => a.seq_no - b.seq_no))
const processCount = computed(() => phases.value.reduce((total, phase) => total + phase.processes.length, 0))

function statusInfo(status: string) {
  return STATUS_META[status] ?? STATUS_META.NOT_STARTED
}

// fmtPct 进度数值 → 百分比文本（需求 10748）。数值与进度条宽度同源。
function fmtPct(value: number): string {
  return `${Math.round(value)}%`
}

function phaseProgress(phase: CDEUPhase): number {
  if (!phase.total_count) {
    return phase.status === 'SUCCESS' ? 100 : 0
  }
  return Math.min(100, Math.max(0, (phase.success_count / phase.total_count) * 100))
}

function processProgress(process: CDEUProcess): number {
  const jobs = process.jobs ?? []
  if (!jobs.length) return process.status === 'SUCCESS' ? 100 : 0
  const done = jobs.filter((job) => job.status === 'SUCCESS' || job.status === 'SKIPPED').length
  return Math.round((done / jobs.length) * 100)
}

function processJobText(process: CDEUProcess): string {
  const jobs = process.jobs ?? []
  const dyn = jobs.filter((job) => job.is_dynamic)
  const statics = jobs.filter((job) => !job.is_dynamic)
  const parts: string[] = []
  if (statics.length) {
    const done = statics.filter((job) => job.status === 'SUCCESS' || job.status === 'SKIPPED').length
    parts.push(`${done}/${statics.length} JOB`)
  }
  if (dyn.length) {
    // 动态实例：一个证券一个月一条（UPLOAD 按文件再细分），数量才是真实工作量
    const ok = dyn.filter((job) => job.status === 'SUCCESS' || job.status === 'SKIPPED').length
    parts.push(`${dyn.length} 实例（成功 ${ok}）`)
  }
  return parts.length ? parts.join(' · ') : '无 JOB'
}

// processDynamicTooltip 生成动态实例的悬浮明细（最多展示前 8 条）。
function processDynamicTooltip(process: CDEUProcess): string {
  const dyn = (process.jobs ?? []).filter((job) => job.is_dynamic)
  if (!dyn.length) return ''
  const lines = dyn.slice(0, 8).map((job) => {
    const label = [job.usc, job.biz_month, job.file_type ?? job.export_type ?? ''].filter(Boolean).join(' ')
    return `${label} [${job.status}]`
  })
  if (dyn.length > 8) lines.push(`… 共 ${dyn.length} 个实例`)
  return '\n动态实例：\n' + lines.join('\n')
}

let textMeasureCanvas: HTMLCanvasElement | null = null

function measureTextWidth(text: string, font: string): number {
  if (typeof document === 'undefined') {
    return text.length * (font.includes('monospace') ? 6 : 12)
  }
  if (!textMeasureCanvas) textMeasureCanvas = document.createElement('canvas')
  const context = textMeasureCanvas.getContext('2d')
  if (!context) return text.length * 12
  context.font = font
  return context.measureText(text).width
}

// 流程节点宽度按标题与编码实际长度计算，避免标题被省略号截断。
function processNodeWidth(process: CDEUProcess): number {
  const titleWidth = measureTextWidth(process.name, '700 12px sans-serif') + 64
  const metaWidth = measureTextWidth(process.process_code, '10px monospace') +
    8 + measureTextWidth(processJobText(process), '10px sans-serif') + 24
  return Math.ceil(Math.max(NODE_WIDTH_MIN, titleWidth, metaWidth)) + 4
}

interface ColumnMetrics {
  widths: number[]
  offsets: number[]
  baseWidth: number
  contentWidth: number
}

// 按列计算宽度：同列取最宽节点；居中节点（竖版预算/决策/批次等）单独计宽。
function buildColumnMetrics(
  placements: Array<{ col: number; process: CDEUProcess; centered?: boolean }>,
  columnGap: number,
): ColumnMetrics {
  const widths: number[] = []
  let centeredMax = 0
  for (const item of placements) {
    const nodeWidth = processNodeWidth(item.process)
    if (item.centered) {
      centeredMax = Math.max(centeredMax, nodeWidth)
      continue
    }
    const column = Math.round(item.col)
    widths[column] = Math.max(widths[column] ?? 0, nodeWidth)
  }
  const offsets: number[] = []
  let baseWidth = 0
  for (let index = 0; index < widths.length; index += 1) {
    offsets[index] = baseWidth
    baseWidth += (widths[index] ?? 0) + (index < widths.length - 1 ? columnGap : 0)
  }
  return {
    widths,
    offsets,
    baseWidth,
    contentWidth: Math.max(baseWidth, centeredMax),
  }
}

function placementsForPhase(phase: CDEUPhase): ProcessPlacement[] {
  const sorted = [...(phase.processes ?? [])].sort((a, b) => a.seq_no - b.seq_no)
  if (!sorted.length) return []
  const byCode = new Map(sorted.map((process) => [process.process_code, process]))

  if (phase.phase_code === 'PHASE_DECISION' &&
      byCode.has('PROCESS_INGEST_STATS') &&
      byCode.has('PROCESS_MATURITY') &&
      byCode.has('PROCESS_BUDGET') &&
      byCode.has('PROCESS_EXPORT_DECISION')) {
    return [
      { process: byCode.get('PROCESS_INGEST_STATS')!, col: 0, lane: 0 },
      { process: byCode.get('PROCESS_MATURITY')!, col: 0, lane: 1 },
      { process: byCode.get('PROCESS_BUDGET')!, col: 1, lane: 0 },
      { process: byCode.get('PROCESS_EXPORT_DECISION')!, col: 2, lane: 0 },
    ]
  }

  if (phase.phase_code === 'PHASE_EXPORT' &&
      byCode.has('PROCESS_EXPORT_BATCH') &&
      byCode.has('PROCESS_EXPORT_FILE_MON_BASE') &&
      byCode.has('PROCESS_EXPORT_FILE_MON_INCR') &&
      byCode.has('PROCESS_RECOVER')) {
    return [
      { process: byCode.get('PROCESS_EXPORT_BATCH')!, col: 0, lane: 0 },
      { process: byCode.get('PROCESS_EXPORT_FILE_MON_BASE')!, col: 1, lane: 0 },
      { process: byCode.get('PROCESS_RECOVER')!, col: 1, lane: 1 },
      { process: byCode.get('PROCESS_EXPORT_FILE_MON_INCR')!, col: 2, lane: 0 },
    ]
  }

  return sorted.map((process, index) => ({ process, col: index, lane: 0 }))
}

const phaseLayouts = computed<LayoutPhase[]>(() => {
  const layouts: LayoutPhase[] = []
  const placementsByPhase = phases.value.map((phase) => ({
    phase,
    placements: placementsForPhase(phase),
  }))
  const maxLaneCount = Math.max(1, ...placementsByPhase.map(({ placements }) =>
    placements.reduce((max, item) => Math.max(max, item.lane + 1), 1),
  ))
  // 所有阶段共用同一高度；宽度按该阶段实际列数（以及入口分叉空间）自适应。
  const phaseHeight = PHASE_HEADER + PHASE_PAD * 2 +
    maxLaneCount * NODE_HEIGHT + (maxLaneCount - 1) * LANE_GAP
  let nextX = OUTER_PAD + START_WIDTH + PHASE_GAP

  for (const { phase, placements } of placementsByPhase) {
    const metrics = buildColumnMetrics(placements, COL_GAP)
    const hasInternalFork = phase.phase_code === 'PHASE_DECISION' &&
      placements.some((item) => item.process.process_code === 'PROCESS_INGEST_STATS') &&
      placements.some((item) => item.process.process_code === 'PROCESS_MATURITY')
    const forkSpace = hasInternalFork ? FORK_SPACE : 0
    const width = PHASE_PAD * 2 + forkSpace + metrics.contentWidth
    const nodeY = OUTER_PAD + PHASE_HEADER + PHASE_PAD
    const forkY = nodeY + NODE_HEIGHT + LANE_GAP / 2 - FORK_SIZE / 2
    const layout: LayoutPhase = {
      phase,
      x: nextX,
      y: OUTER_PAD,
      width,
      height: phaseHeight,
      nodes: placements.map((item) => ({
        process: item.process,
        phase,
        x: nextX + PHASE_PAD + forkSpace + metrics.offsets[item.col],
        y: nodeY + item.lane * (NODE_HEIGHT + LANE_GAP),
        width: processNodeWidth(item.process),
        height: NODE_HEIGHT,
      })),
      fork: hasInternalFork
        ? { x: nextX + PHASE_PAD + 24, y: forkY, width: FORK_SIZE, height: FORK_SIZE }
        : undefined,
    }
    layouts.push(layout)
    nextX += width + PHASE_GAP
  }
  return layouts
})

const phaseBottom = computed(() =>
  phaseLayouts.value.reduce((max, layout) => Math.max(max, layout.y + layout.height), OUTER_PAD),
)
const legendTop = computed(() => phaseBottom.value + 14)
const canvasHeight = computed(() => legendTop.value + 42 + OUTER_PAD)

const startNode = computed<Box>(() => {
  const first = phaseLayouts.value[0]
  return {
    x: OUTER_PAD,
    y: first ? first.y + PHASE_HEADER + PHASE_PAD : OUTER_PAD,
    width: START_WIDTH,
    height: NODE_HEIGHT,
  }
})

const endNode = computed<Box>(() => {
  const last = phaseLayouts.value[phaseLayouts.value.length - 1]
  return {
    x: last ? last.x + last.width + PHASE_GAP : OUTER_PAD + START_WIDTH + PHASE_GAP,
    y: last ? last.y + PHASE_HEADER + PHASE_PAD : OUTER_PAD,
    width: START_WIDTH,
    height: NODE_HEIGHT,
  }
})

const canvasWidth = computed(() => Math.max(endNode.value.x + START_WIDTH + OUTER_PAD, 900))

function findNode(layout: LayoutPhase, processCode: string): LayoutNode | undefined {
  return layout.nodes.find((node) => node.process.process_code === processCode)
}

function entryNodes(layout: LayoutPhase): LayoutNode[] {
  if (layout.phase.phase_code === 'PHASE_DECISION') {
    const stats = findNode(layout, 'PROCESS_INGEST_STATS')
    const maturity = findNode(layout, 'PROCESS_MATURITY')
    if (stats && maturity) return [stats, maturity]
  }
  return layout.nodes.length ? [layout.nodes[0]] : []
}

function exitNode(layout: LayoutPhase): LayoutNode | undefined {
  if (layout.phase.phase_code === 'PHASE_EXPORT') {
    const incr = findNode(layout, 'PROCESS_EXPORT_FILE_MON_INCR')
    if (incr) return incr
  }
  return layout.nodes[layout.nodes.length - 1]
}

function edgePath(from: Box, to: Box): string {
  const sourceX = from.x + from.width
  const sourceY = from.y + from.height / 2
  const targetX = to.x - 8
  const targetY = to.y + to.height / 2
  if (Math.abs(sourceY - targetY) < 2) {
    return `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`
  }
  const midX = sourceX + Math.max(18, (to.x - sourceX) / 2)
  return `M ${sourceX} ${sourceY} L ${midX} ${sourceY} L ${midX} ${targetY} L ${targetX} ${targetY}`
}

const edges = computed<TopoEdge[]>(() => {
  if (!phaseLayouts.value.length) return []
  const result: TopoEdge[] = []

  const appendPath = (key: string, path: string, status: string, dashed = false): void => {
    const info = statusInfo(status)
    result.push({
      key,
      path,
      color: info.hex,
      marker: `fulltopo-arrow-${info.tone}`,
      running: status === 'RUNNING',
      dashed,
    })
  }
  const appendEdge = (key: string, from: Box, to: Box, status: string, dashed = false): void => {
    appendPath(key, edgePath(from, to), status, dashed)
  }
  const append = (key: string, from: Box, to: LayoutNode, dashed = false): void => {
    appendEdge(key, from, to, to.process.status, dashed)
  }
  const appendForkEdge = (key: string, fork: Box, to: LayoutNode): void => {
    const sourceX = fork.x + fork.width / 2
    const sourceY = fork.y + fork.height / 2
    // 分叉后留出足够空间，避免箭头与 Process 节点入口重叠。
    const targetX = to.x - 16
    const targetY = to.y + to.height / 2
    appendPath(key, `M ${sourceX} ${sourceY} L ${sourceX} ${targetY} L ${targetX} ${targetY}`, to.process.status)
  }
  const appendForkEntry = (key: string, from: Box, fork: Box, phase: LayoutPhase): void => {
    const sourceX = from.x + from.width
    const sourceY = from.y + from.height / 2
    const targetX = fork.x - 10
    const targetY = fork.y + fork.height / 2
    const insideX = phase.x + 8
    if (Math.abs(sourceY - targetY) < 2) {
      appendPath(key, `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`, phase.phase.status)
      return
    }
    // 跨阶段箭头：进入虚线框 -> 框内竖向换行 -> 横向进入分叉点，形成完整 Z 字折线。
    appendPath(
      key,
      `M ${sourceX} ${sourceY} L ${insideX} ${sourceY} L ${insideX} ${targetY} L ${targetX} ${targetY}`,
      phase.phase.status,
    )
  }

  for (const layout of phaseLayouts.value) {
    const phaseCode = layout.phase.phase_code
    if (phaseCode === 'PHASE_DECISION') {
      const stats = findNode(layout, 'PROCESS_INGEST_STATS')
      const maturity = findNode(layout, 'PROCESS_MATURITY')
      const budget = findNode(layout, 'PROCESS_BUDGET')
      const decision = findNode(layout, 'PROCESS_EXPORT_DECISION')
      if (layout.fork && stats) {
        appendForkEdge(`edge-${layout.phase.phase_code}-fork-stats`, layout.fork, stats)
      }
      if (layout.fork && maturity) {
        appendForkEdge(`edge-${layout.phase.phase_code}-fork-maturity`, layout.fork, maturity)
      }
      if (stats && budget) append(`edge-${layout.phase.phase_code}-stats-budget`, stats, budget)
      if (maturity && budget) append(`edge-${layout.phase.phase_code}-maturity-budget`, maturity, budget)
      if (budget && decision) append(`edge-${layout.phase.phase_code}-budget-decision`, budget, decision)
    } else if (phaseCode === 'PHASE_EXPORT') {
      const batch = findNode(layout, 'PROCESS_EXPORT_BATCH')
      const base = findNode(layout, 'PROCESS_EXPORT_FILE_MON_BASE')
      const recover = findNode(layout, 'PROCESS_RECOVER')
      const incr = findNode(layout, 'PROCESS_EXPORT_FILE_MON_INCR')
      if (batch && base) append(`edge-${layout.phase.phase_code}-batch-base`, batch, base)
      if (batch && recover) append(`edge-${layout.phase.phase_code}-batch-recover`, batch, recover, true)
      if (base && incr) append(`edge-${layout.phase.phase_code}-base-incr`, base, incr)
    } else {
      for (let index = 0; index < layout.nodes.length - 1; index += 1) {
        append(`edge-${layout.phase.phase_code}-${index}`, layout.nodes[index], layout.nodes[index + 1])
      }
    }
  }

  const firstLayout = phaseLayouts.value[0]
  if (firstLayout.fork) {
    appendForkEntry('edge-start-fork', startNode.value, firstLayout.fork, firstLayout)
  } else {
    for (const node of entryNodes(firstLayout)) {
      append(`edge-start-${node.process.process_code}`, startNode.value, node)
    }
  }

  for (let index = 0; index < phaseLayouts.value.length - 1; index += 1) {
    const current = phaseLayouts.value[index]
    const next = phaseLayouts.value[index + 1]
    const exit = exitNode(current)
    if (!exit) continue
    if (next.fork) {
      // 跨阶段只连到阶段内部的入口分叉点，真正的分叉线留在虚线框内。
      appendForkEntry(`edge-phase-${current.phase.phase_code}-${next.phase.phase_code}-fork`, exit, next.fork, next)
      continue
    }
    for (const node of entryNodes(next)) {
      append(`edge-phase-${current.phase.phase_code}-${next.phase.phase_code}-${node.process.process_code}`, exit, node)
    }
  }

  const lastLayout = phaseLayouts.value[phaseLayouts.value.length - 1]
  const lastExit = exitNode(lastLayout)
  if (lastExit) {
    const info = statusInfo(lastExit.process.status)
    result.push({
      key: 'edge-end',
      path: edgePath(lastExit, endNode.value),
      color: info.hex,
      marker: `fulltopo-arrow-${info.tone}`,
      running: false,
      dashed: false,
    })
  }
  return result
})

function phaseStatusText(phase: CDEUPhase): string {
  return statusInfo(phase.status).text
}

// ---------------------------------------------------------------------------
// 竖版拓扑：阶段自上而下堆叠，阶段内 Process 也自上而下执行。
// 所有阶段框共用最大高度，宽度按该阶段列数自适应，便于和横版对比。
// ---------------------------------------------------------------------------
const V_PHASE_TITLE_WIDTH = 156
const V_PHASE_PAD = 16
const V_PHASE_GAP = 64
const V_ROW_GAP = 26
const V_BRANCH_ROW_GAP = 44
const V_BRANCH_FORK_SPACE = 80
const V_COL_GAP = 38
const V_SIDE_GUTTER = 32

interface VPlacement {
  process: CDEUProcess
  col: number
  row: number
  centered?: boolean
}

function verticalPlacementsForPhase(phase: CDEUPhase): VPlacement[] {
  const sorted = [...(phase.processes ?? [])].sort((a, b) => a.seq_no - b.seq_no)
  if (!sorted.length) return []
  const byCode = new Map(sorted.map((process) => [process.process_code, process]))

  if (phase.phase_code === 'PHASE_DECISION' &&
      byCode.has('PROCESS_INGEST_STATS') &&
      byCode.has('PROCESS_MATURITY') &&
      byCode.has('PROCESS_BUDGET') &&
      byCode.has('PROCESS_EXPORT_DECISION')) {
    return [
      { process: byCode.get('PROCESS_INGEST_STATS')!, col: 0, row: 0 },
      { process: byCode.get('PROCESS_MATURITY')!, col: 1, row: 0 },
      { process: byCode.get('PROCESS_BUDGET')!, col: 0.5, row: 1, centered: true },
      { process: byCode.get('PROCESS_EXPORT_DECISION')!, col: 0.5, row: 2, centered: true },
    ]
  }

  if (phase.phase_code === 'PHASE_EXPORT' &&
      byCode.has('PROCESS_EXPORT_BATCH') &&
      byCode.has('PROCESS_EXPORT_FILE_MON_BASE') &&
      byCode.has('PROCESS_EXPORT_FILE_MON_INCR') &&
      byCode.has('PROCESS_RECOVER')) {
    return [
      { process: byCode.get('PROCESS_EXPORT_BATCH')!, col: 0.5, row: 0, centered: true },
      { process: byCode.get('PROCESS_EXPORT_FILE_MON_BASE')!, col: 0, row: 1 },
      { process: byCode.get('PROCESS_RECOVER')!, col: 1, row: 1 },
      { process: byCode.get('PROCESS_EXPORT_FILE_MON_INCR')!, col: 0.5, row: 2, centered: true },
    ]
  }

  return sorted.map((process, index) => ({ process, col: 0, row: index }))
}

function verticalPlacementWidth(placements: VPlacement[]): number {
  const metrics = buildColumnMetrics(placements, V_COL_GAP)
  return V_PHASE_TITLE_WIDTH + V_PHASE_PAD * 2 + metrics.contentWidth
}

const verticalMaxWidth = computed(() => {
  const widths = phases.value.map((phase) => verticalPlacementWidth(verticalPlacementsForPhase(phase)))
  return Math.max(1, ...widths)
})

const verticalPhaseLayouts = computed<LayoutPhase[]>(() => {
  const layouts: LayoutPhase[] = []
  const phaseData = phases.value.map((phase) => {
    const placements = verticalPlacementsForPhase(phase)
    const maxRow = placements.reduce((max, item) => Math.max(max, item.row), 0)
    const hasFork = phase.phase_code === 'PHASE_DECISION' &&
      placements.some((item) => item.process.process_code === 'PROCESS_INGEST_STATS') &&
      placements.some((item) => item.process.process_code === 'PROCESS_MATURITY')
    const metrics = buildColumnMetrics(placements, V_COL_GAP)
    return {
      phase,
      placements,
      metrics,
      maxRow: placements.length ? maxRow : 0,
      width: V_PHASE_TITLE_WIDTH + V_PHASE_PAD * 2 + metrics.contentWidth,
      hasFork,
    }
  })
  const maxWidth = Math.max(1, ...phaseData.map((item) => item.width))
  let nextY = OUTER_PAD + NODE_HEIGHT + V_PHASE_GAP

  for (const item of phaseData) {
    const rows = item.maxRow + 1
    const hasBranchLayout = item.hasFork || item.phase.phase_code === 'PHASE_EXPORT'
    const forkSpace = item.hasFork ? V_BRANCH_FORK_SPACE : 0
    const rowGap = hasBranchLayout ? V_BRANCH_ROW_GAP : V_ROW_GAP
    const phaseHeight = Math.max(132, V_PHASE_PAD * 2 + forkSpace +
      rows * NODE_HEIGHT + (rows - 1) * rowGap)
    // 竖版所有阶段框统一使用最大宽度；内容较少的阶段在框内水平居中。
    const x = OUTER_PAD + V_SIDE_GUTTER
    const nodeBaseY = nextY + V_PHASE_PAD + forkSpace
    const contentWidth = maxWidth - V_PHASE_TITLE_WIDTH - V_PHASE_PAD * 2
    const itemContentWidth = item.metrics.contentWidth
    const contentOffset = Math.max(0, (contentWidth - itemContentWidth) / 2)
    const contentStartX = x + V_PHASE_TITLE_WIDTH + V_PHASE_PAD + contentOffset
    const forkX = x + V_PHASE_TITLE_WIDTH + V_PHASE_PAD + (contentWidth - FORK_SIZE) / 2
    const layout: LayoutPhase = {
      phase: item.phase,
      x,
      y: nextY,
      width: maxWidth,
      height: phaseHeight,
      nodes: item.placements.map((placement) => {
        const nodeWidth = processNodeWidth(placement.process)
        const nodeX = placement.centered
          ? contentStartX + (itemContentWidth - nodeWidth) / 2
          : contentStartX + item.metrics.offsets[placement.col]
        return {
          process: placement.process,
          phase: item.phase,
          x: nodeX,
          y: nodeBaseY + placement.row * (NODE_HEIGHT + rowGap),
          width: nodeWidth,
          height: NODE_HEIGHT,
        }
      }),
      fork: item.hasFork
        ? {
            x: forkX,
            y: nextY + V_PHASE_PAD + (forkSpace - FORK_SIZE) / 2,
            width: FORK_SIZE,
            height: FORK_SIZE,
          }
        : undefined,
    }
    layouts.push(layout)
    nextY += phaseHeight + V_PHASE_GAP
  }
  return layouts
})

const verticalStartNode = computed<Box>(() => ({
  x: OUTER_PAD + V_SIDE_GUTTER + verticalMaxWidth.value / 2 - START_WIDTH / 2,
  y: OUTER_PAD,
  width: START_WIDTH,
  height: NODE_HEIGHT,
}))

const verticalEndNode = computed<Box>(() => {
  const last = verticalPhaseLayouts.value[verticalPhaseLayouts.value.length - 1]
  return {
    x: OUTER_PAD + V_SIDE_GUTTER + verticalMaxWidth.value / 2 - START_WIDTH / 2,
    y: last ? last.y + last.height + V_PHASE_GAP : OUTER_PAD + NODE_HEIGHT + V_PHASE_GAP,
    width: START_WIDTH,
    height: NODE_HEIGHT,
  }
})

const verticalCanvasWidth = computed(() =>
  Math.max(OUTER_PAD * 2 + V_SIDE_GUTTER * 2 + verticalMaxWidth.value, 420),
)
const verticalLegendTop = computed(() => verticalEndNode.value.y + NODE_HEIGHT + 16)
const verticalCanvasHeight = computed(() => verticalLegendTop.value + 42 + OUTER_PAD)

function verticalFindNode(layout: LayoutPhase, processCode: string): LayoutNode | undefined {
  return layout.nodes.find((node) => node.process.process_code === processCode)
}

function verticalEntryBox(layout: LayoutPhase): Box | undefined {
  if (layout.fork) return layout.fork
  return layout.nodes[0]
}

function verticalExitNode(layout: LayoutPhase): LayoutNode | undefined {
  if (layout.phase.phase_code === 'PHASE_EXPORT') {
    const incr = verticalFindNode(layout, 'PROCESS_EXPORT_FILE_MON_INCR')
    if (incr) return incr
  }
  return layout.nodes[layout.nodes.length - 1]
}

function verticalPath(from: Box, to: Box): string {
  const sourceX = from.x + from.width / 2
  const sourceY = from.y + from.height
  const targetX = to.x + to.width / 2
  const targetY = to.y - 8
  if (Math.abs(sourceX - targetX) < 2) {
    return `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`
  }
  const midY = sourceY + Math.max(18, (to.y - sourceY) / 2)
  return `M ${sourceX} ${sourceY} L ${sourceX} ${midY} L ${targetX} ${midY} L ${targetX} ${targetY}`
}

function verticalForkPath(from: Box, to: Box): string {
  const sourceX = from.x + from.width / 2
  const sourceY = from.y + from.height
  const targetX = to.x + to.width / 2
  const targetY = to.y - 8
  const midY = sourceY + (targetY - sourceY) / 2
  // 入口分叉的水平折线段在常规中点基础上再上移 10px。
  const routeY = Math.max(sourceY + 2, midY - 10)
  return `M ${sourceX} ${sourceY} L ${sourceX} ${routeY} L ${targetX} ${routeY} L ${targetX} ${targetY}`
}

function verticalCrossPath(from: Box, to: Box): string {
  const sourceX = from.x + from.width / 2
  const sourceY = from.y + from.height
  const targetX = to.x + to.width / 2
  const targetY = to.y - 8
  if (Math.abs(sourceX - targetX) < 2) {
    return `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`
  }
  const midY = sourceY + Math.max(18, (to.y - sourceY) / 2)
  return `M ${sourceX} ${sourceY} L ${sourceX} ${midY} L ${targetX} ${midY} L ${targetX} ${targetY}`
}

const verticalEdges = computed<TopoEdge[]>(() => {
  if (!verticalPhaseLayouts.value.length) return []
  const result: TopoEdge[] = []

  const appendPath = (key: string, path: string, status: string, dashed = false): void => {
    const info = statusInfo(status)
    result.push({
      key,
      path,
      color: info.hex,
      marker: `fulltopo-v-arrow-${info.tone}`,
      running: status === 'RUNNING',
      dashed,
    })
  }
  const appendNode = (key: string, from: Box, to: LayoutNode, dashed = false): void => {
    appendPath(key, verticalPath(from, to), to.process.status, dashed)
  }
  const appendCross = (key: string, from: Box, to: Box, status: string): void => {
    appendPath(key, verticalCrossPath(from, to), status)
  }

  for (const layout of verticalPhaseLayouts.value) {
    const phaseCode = layout.phase.phase_code
    if (phaseCode === 'PHASE_DECISION') {
      const stats = verticalFindNode(layout, 'PROCESS_INGEST_STATS')
      const maturity = verticalFindNode(layout, 'PROCESS_MATURITY')
      const budget = verticalFindNode(layout, 'PROCESS_BUDGET')
      const decision = verticalFindNode(layout, 'PROCESS_EXPORT_DECISION')
      if (layout.fork && stats) {
        appendPath(`v-${phaseCode}-fork-stats`, verticalForkPath(layout.fork, stats), stats.process.status)
      }
      if (layout.fork && maturity) {
        appendPath(`v-${phaseCode}-fork-maturity`, verticalForkPath(layout.fork, maturity), maturity.process.status)
      }
      if (stats && budget) appendNode(`v-${phaseCode}-stats-budget`, stats, budget)
      if (maturity && budget) appendNode(`v-${phaseCode}-maturity-budget`, maturity, budget)
      if (budget && decision) appendNode(`v-${phaseCode}-budget-decision`, budget, decision)
    } else if (phaseCode === 'PHASE_EXPORT') {
      const batch = verticalFindNode(layout, 'PROCESS_EXPORT_BATCH')
      const base = verticalFindNode(layout, 'PROCESS_EXPORT_FILE_MON_BASE')
      const recover = verticalFindNode(layout, 'PROCESS_RECOVER')
      const incr = verticalFindNode(layout, 'PROCESS_EXPORT_FILE_MON_INCR')
      if (batch && base) appendNode(`v-${phaseCode}-batch-base`, batch, base)
      if (batch && recover) appendNode(`v-${phaseCode}-batch-recover`, batch, recover, true)
      if (base && incr) appendNode(`v-${phaseCode}-base-incr`, base, incr)
    } else {
      for (let index = 0; index < layout.nodes.length - 1; index += 1) {
        appendNode(`v-${phaseCode}-${index}`, layout.nodes[index], layout.nodes[index + 1])
      }
    }
  }

  const first = verticalPhaseLayouts.value[0]
  const firstEntry = verticalEntryBox(first)
  if (firstEntry) {
    appendCross('v-edge-start', verticalStartNode.value, firstEntry, first.phase.status)
  }

  for (let index = 0; index < verticalPhaseLayouts.value.length - 1; index += 1) {
    const current = verticalPhaseLayouts.value[index]
    const next = verticalPhaseLayouts.value[index + 1]
    const exit = verticalExitNode(current)
    const entry = verticalEntryBox(next)
    if (!exit || !entry) continue
    appendCross(
      `v-edge-${current.phase.phase_code}-${next.phase.phase_code}`,
      exit,
      entry,
      next.phase.status,
    )
  }

  const last = verticalPhaseLayouts.value[verticalPhaseLayouts.value.length - 1]
  const lastExit = verticalExitNode(last)
  if (lastExit) {
    appendPath('v-edge-end', verticalPath(lastExit, verticalEndNode.value), lastExit.process.status)
  }
  return result
})

function isActiveRun(status: string | undefined): boolean {
  return status === 'RUNNING' || status === 'PENDING' || status === 'READY'
}

// 执行中 2 秒同步一次，空闲时 10 秒；避免工作流执行期间页面停留在静态快照。
const refreshInterval = computed(() => isActiveRun(run.value?.status) ? 2000 : 10000)

function scheduleRefresh(): void {
  if (disposed) return
  if (refreshTimer) {
    window.clearTimeout(refreshTimer)
    refreshTimer = undefined
  }
  if (!autoRefresh.value) return
  refreshTimer = window.setTimeout(() => { void load() }, refreshInterval.value)
}

async function load(): Promise<void> {
  if (inFlight) return
  inFlight = true
  loading.value = true
  try {
    overview.value = await apiGet<CDEUOverview>(
      `/Meta/Finv/Quote/DeDup/CDEU/Overview?historyLimit=1&_t=${Date.now()}`,
    )
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载 CDEU 全流程拓扑失败'
  } finally {
    inFlight = false
    loading.value = false
    scheduleRefresh()
  }
}

function refreshIfVisible(): void {
  if (autoRefresh.value && document.visibilityState === 'visible') void load()
}

watch(autoRefresh, (enabled) => {
  if (enabled) void load()
  else if (refreshTimer) {
    window.clearTimeout(refreshTimer)
    refreshTimer = undefined
  }
})

onMounted(() => {
  void load()
  document.addEventListener('visibilitychange', refreshIfVisible)
  window.addEventListener('focus', refreshIfVisible)
})

onBeforeUnmount(() => {
  disposed = true
  if (refreshTimer) window.clearTimeout(refreshTimer)
  document.removeEventListener('visibilitychange', refreshIfVisible)
  window.removeEventListener('focus', refreshIfVisible)
})
</script>

<template>
  <div>
    <div class="d-flex flex-wrap align-center justify-space-between ga-3 mb-3">
      <div>
        <div class="d-flex align-center ga-2">
          <v-icon icon="mdi-graph-outline" color="primary" size="30" />
          <h2 class="text-h5 font-weight-bold mb-0">CDEU 全流程拓扑</h2>
          <v-chip v-if="workflow" size="small" color="primary" variant="tonal">
            {{ workflow.workflow_code }} v{{ workflow.version }}
          </v-chip>
        </div>
        <div class="text-body-2 text-medium-emphasis mt-1">
          虚线框为阶段范围，框内为 Process；箭头表示执行依赖，跨框箭头表示阶段衔接，画布可横向滚动查看完整链路。
        </div>
      </div>
      <div class="d-flex flex-wrap align-center ga-2">
        <v-chip v-if="phases.length" size="small" color="deep-purple" variant="tonal" prepend-icon="mdi-layers-triple-outline">
          {{ phases.length }} 个阶段 · {{ processCount }} 个流程
        </v-chip>
        <v-chip
          v-if="run"
          size="small"
          :color="statusInfo(run.status).tone"
          variant="tonal"
          :prepend-icon="statusInfo(run.status).icon"
        >
          {{ statusInfo(run.status).text }} · 业务日 {{ run.biz_date }} · 运行 #{{ run.run_id }}
        </v-chip>
        <v-switch v-model="autoRefresh" color="primary" density="compact" hide-details label="自动刷新" />
        <v-btn variant="tonal" prepend-icon="mdi-refresh" :loading="loading" @click="load()">刷新</v-btn>
      </div>
    </div>

    <v-alert v-if="error" type="warning" variant="tonal" class="mb-3" closable @click:close="error = ''">
      {{ error }}
    </v-alert>

    <v-progress-linear v-if="loading && !overview" indeterminate color="primary" class="mb-2" />

    <div v-if="overview" class="fulltopo-shell">
      <div class="fulltopo-scroll">
        <div class="fulltopo-canvas" :style="{ width: `${canvasWidth}px`, height: `${canvasHeight}px` }">
          <div
            v-for="layout in phaseLayouts"
            :key="layout.phase.phase_code"
            class="fulltopo-phase"
            :class="{ 'fulltopo-phase-running': layout.phase.status === 'RUNNING' }"
            :style="{
              left: `${layout.x}px`,
              top: `${layout.y}px`,
              width: `${layout.width}px`,
              height: `${layout.height}px`,
              '--phase-color': statusInfo(layout.phase.status).hex,
            }"
          >
            <div class="fulltopo-phase-header">
              <span class="fulltopo-phase-seq">{{ layout.phase.seq_no }}</span>
              <div class="fulltopo-phase-title">
                <div class="fulltopo-phase-name">{{ layout.phase.name }}</div>
                <div class="fulltopo-phase-code">{{ layout.phase.phase_code }}</div>
              </div>
              <v-chip
                size="x-small"
                :color="statusInfo(layout.phase.status).tone"
                variant="tonal"
                :prepend-icon="statusInfo(layout.phase.status).icon"
              >
                {{ phaseStatusText(layout.phase) }}
              </v-chip>
            </div>
            <div class="fulltopo-phase-progress">
              <span class="fulltopo-phase-progress-track">
                <span :style="{ width: `${phaseProgress(layout.phase)}%`, background: statusInfo(layout.phase.status).hex }" />
              </span>
              <span class="fulltopo-phase-percent">{{ fmtPct(phaseProgress(layout.phase)) }}</span>
            </div>
            <div class="fulltopo-phase-summary">
              {{ layout.phase.success_count }}/{{ layout.phase.total_count }} Process 完成
            </div>
          </div>

          <svg class="fulltopo-edge-layer" :width="canvasWidth" :height="canvasHeight" aria-hidden="true">
            <defs>
              <marker
                v-for="(color, tone) in EDGE_COLORS"
                :id="`fulltopo-arrow-${tone}`"
                :key="tone"
                viewBox="0 0 7 7"
                refX="6"
                refY="3.5"
                markerWidth="7"
                markerHeight="7"
                orient="auto"
              >
                <path d="M 0 0 L 7 3.5 L 0 7 z" :fill="color" />
              </marker>
            </defs>
            <path
              v-for="edge in edges"
              :key="edge.key"
              :d="edge.path"
              fill="none"
              :stroke="edge.color"
              stroke-width="1.6"
              :stroke-dasharray="edge.dashed || edge.running ? '5 4' : undefined"
              :marker-end="`url(#${edge.marker})`"
              :class="{ 'fulltopo-edge-running': edge.running }"
            />
          </svg>

          <div
            v-for="layout in phaseLayouts"
            :key="`nodes-${layout.phase.phase_code}`"
          >
            <div
              v-if="layout.fork"
              class="fulltopo-fork"
              :style="{
                left: `${layout.fork.x}px`,
                top: `${layout.fork.y}px`,
                width: `${layout.fork.width}px`,
                height: `${layout.fork.height}px`,
                '--fork-color': statusInfo(layout.phase.status).hex,
              }"
              title="阶段内并行入口分叉"
            />
            <div
              v-for="node in layout.nodes"
              :key="node.process.process_code"
              class="fulltopo-node"
              :class="{ 'fulltopo-node-running': node.process.status === 'RUNNING' }"
              :style="{
                left: `${node.x}px`,
                top: `${node.y}px`,
                width: `${node.width}px`,
                height: `${node.height}px`,
                '--status-color': statusInfo(node.process.status).hex,
              }"
              :title="`${node.process.name}（${node.process.process_code}）· ${statusInfo(node.process.status).text} · ${processJobText(node.process)}${processDynamicTooltip(node.process)}`"
            >
              <div class="fulltopo-node-head">
                <span class="fulltopo-node-seq">{{ node.process.process_id }}</span>
                <span class="fulltopo-node-name">{{ node.process.name }}</span>
                <v-icon :color="statusInfo(node.process.status).tone" size="14" :icon="statusInfo(node.process.status).icon" />
              </div>
              <div class="fulltopo-node-meta">
                <span class="fulltopo-node-code">{{ node.process.process_code }}</span>
                <span>{{ processJobText(node.process) }}</span>
              </div>
              <div class="fulltopo-node-progress">
                <span class="fulltopo-node-progress-track">
                  <span :style="{ width: `${processProgress(node.process)}%` }" />
                </span>
                <span class="fulltopo-node-percent">{{ fmtPct(processProgress(node.process)) }}</span>
              </div>
            </div>
          </div>

          <div
            class="fulltopo-terminal"
            :style="{ left: `${startNode.x}px`, top: `${startNode.y}px`, width: `${startNode.width}px`, height: `${startNode.height}px` }"
          >
            <v-icon icon="mdi-play-circle-outline" size="18" />
            <span>开始</span>
            <small>工作流入口</small>
          </div>
          <div
            class="fulltopo-terminal"
            :style="{ left: `${endNode.x}px`, top: `${endNode.y}px`, width: `${endNode.width}px`, height: `${endNode.height}px` }"
          >
            <v-icon icon="mdi-flag-checkered" size="18" />
            <span>结束</span>
            <small>工作流出口</small>
          </div>

          <div
            class="fulltopo-legend fulltopo-legend-in-canvas"
            :style="{ left: `${OUTER_PAD}px`, top: `${legendTop}px` }"
          >
            <span class="fulltopo-legend-title">图例</span>
            <span class="fulltopo-legend-item"><i class="fulltopo-legend-phase" />阶段范围</span>
            <span class="fulltopo-legend-item"><i class="fulltopo-legend-process" />Process</span>
            <span v-for="item in [
              STATUS_META.SUCCESS,
              STATUS_META.RUNNING,
              STATUS_META.PARTIAL,
              STATUS_META.FAILED,
              STATUS_META.PENDING,
            ]" :key="item.text" class="fulltopo-legend-item">
              <i class="fulltopo-legend-dot" :style="{ background: item.hex }" />{{ item.text }}
            </span>
            <span class="fulltopo-legend-hint">虚线段表示并行/独立支路</span>
            <span class="fulltopo-legend-time">最近查询 {{ fmtISO(overview.generated_at) }}</span>
          </div>
        </div>
      </div>

    </div>

    <div v-if="overview" class="fulltopo-mode-heading mt-6 mb-2">
      <div class="d-flex align-center ga-2">
        <v-icon icon="mdi-format-vertical-align-center" color="primary" size="22" />
        <span class="text-subtitle-1 font-weight-bold">竖版全流程拓扑</span>
        <v-chip size="x-small" color="info" variant="tonal">对比视图</v-chip>
      </div>
      <span class="text-caption text-medium-emphasis">阶段自上而下排列，框内 Process 纵向执行；状态与横版同步。</span>
    </div>

    <div v-if="overview" class="fulltopo-shell fulltopo-vertical-shell">
      <div class="fulltopo-scroll fulltopo-vertical-scroll">
        <div
          class="fulltopo-canvas"
          :style="{ width: `${verticalCanvasWidth}px`, height: `${verticalCanvasHeight}px` }"
        >
          <div
            v-for="layout in verticalPhaseLayouts"
            :key="`v-phase-${layout.phase.phase_code}`"
            class="fulltopo-phase"
            :class="{ 'fulltopo-phase-running': layout.phase.status === 'RUNNING' }"
            :style="{
              left: `${layout.x}px`,
              top: `${layout.y}px`,
              width: `${layout.width}px`,
              height: `${layout.height}px`,
              '--phase-color': statusInfo(layout.phase.status).hex,
            }"
          >
            <div class="fulltopo-vphase-title">
              <div class="fulltopo-vphase-title-head">
                <span class="fulltopo-phase-seq">{{ layout.phase.seq_no }}</span>
                <v-chip
                  size="x-small"
                  :color="statusInfo(layout.phase.status).tone"
                  variant="tonal"
                  :prepend-icon="statusInfo(layout.phase.status).icon"
                >
                  {{ phaseStatusText(layout.phase) }}
                </v-chip>
              </div>
              <div class="fulltopo-vphase-name">{{ layout.phase.name }}</div>
              <div class="fulltopo-vphase-code">{{ layout.phase.phase_code }}</div>
              <div class="fulltopo-vphase-summary">
                {{ layout.phase.success_count }}/{{ layout.phase.total_count }} Process 完成
              </div>
            </div>
            <div
              class="fulltopo-vphase-divider"
              :style="{ left: `${V_PHASE_TITLE_WIDTH - 3}px` }"
            >
              <span :style="{ height: `${phaseProgress(layout.phase)}%`, background: statusInfo(layout.phase.status).hex }" />
            </div>
            <div class="fulltopo-vphase-percent" :style="{ left: `${V_PHASE_TITLE_WIDTH + 6}px` }">
              {{ fmtPct(phaseProgress(layout.phase)) }}
            </div>
          </div>

          <svg class="fulltopo-edge-layer" :width="verticalCanvasWidth" :height="verticalCanvasHeight" aria-hidden="true">
            <defs>
              <marker
                v-for="(color, tone) in EDGE_COLORS"
                :id="`fulltopo-v-arrow-${tone}`"
                :key="tone"
                viewBox="0 0 7 7"
                refX="6"
                refY="3.5"
                markerWidth="7"
                markerHeight="7"
                orient="auto"
              >
                <path d="M 0 0 L 7 3.5 L 0 7 z" :fill="color" />
              </marker>
            </defs>
            <path
              v-for="edge in verticalEdges"
              :key="edge.key"
              :d="edge.path"
              fill="none"
              :stroke="edge.color"
              stroke-width="1.6"
              :stroke-dasharray="edge.dashed || edge.running ? '5 4' : undefined"
              :marker-end="`url(#${edge.marker})`"
              :class="{ 'fulltopo-edge-running': edge.running }"
            />
          </svg>

          <div
            v-for="layout in verticalPhaseLayouts"
            :key="`v-nodes-${layout.phase.phase_code}`"
          >
            <div
              v-if="layout.fork"
              class="fulltopo-fork"
              :style="{
                left: `${layout.fork.x}px`,
                top: `${layout.fork.y}px`,
                width: `${layout.fork.width}px`,
                height: `${layout.fork.height}px`,
                '--fork-color': statusInfo(layout.phase.status).hex,
              }"
              title="阶段内并行入口分叉"
            />
            <div
              v-for="node in layout.nodes"
              :key="`v-${node.process.process_code}`"
              class="fulltopo-node"
              :class="{ 'fulltopo-node-running': node.process.status === 'RUNNING' }"
              :style="{
                left: `${node.x}px`,
                top: `${node.y}px`,
                width: `${node.width}px`,
                height: `${node.height}px`,
                '--status-color': statusInfo(node.process.status).hex,
              }"
              :title="`${node.process.name}（${node.process.process_code}）· ${statusInfo(node.process.status).text} · ${processJobText(node.process)}`"
            >
              <div class="fulltopo-node-head">
                <span class="fulltopo-node-seq">{{ node.process.process_id }}</span>
                <span class="fulltopo-node-name">{{ node.process.name }}</span>
                <v-icon :color="statusInfo(node.process.status).tone" size="14" :icon="statusInfo(node.process.status).icon" />
              </div>
              <div class="fulltopo-node-meta">
                <span class="fulltopo-node-code">{{ node.process.process_code }}</span>
                <span>{{ processJobText(node.process) }}</span>
              </div>
              <div class="fulltopo-node-progress">
                <span class="fulltopo-node-progress-track">
                  <span :style="{ width: `${processProgress(node.process)}%` }" />
                </span>
                <span class="fulltopo-node-percent">{{ fmtPct(processProgress(node.process)) }}</span>
              </div>
            </div>
          </div>

          <div
            class="fulltopo-terminal"
            :style="{ left: `${verticalStartNode.x}px`, top: `${verticalStartNode.y}px`, width: `${verticalStartNode.width}px`, height: `${verticalStartNode.height}px` }"
          >
            <v-icon icon="mdi-play-circle-outline" size="18" />
            <span>开始</span>
            <small>工作流入口</small>
          </div>
          <div
            class="fulltopo-terminal"
            :style="{ left: `${verticalEndNode.x}px`, top: `${verticalEndNode.y}px`, width: `${verticalEndNode.width}px`, height: `${verticalEndNode.height}px` }"
          >
            <v-icon icon="mdi-flag-checkered" size="18" />
            <span>结束</span>
            <small>工作流出口</small>
          </div>

          <div
            class="fulltopo-legend fulltopo-legend-in-canvas"
            :style="{ left: `${OUTER_PAD}px`, top: `${verticalLegendTop}px` }"
          >
            <span class="fulltopo-legend-title">图例</span>
            <span class="fulltopo-legend-item"><i class="fulltopo-legend-phase" />阶段范围</span>
            <span class="fulltopo-legend-item"><i class="fulltopo-legend-process" />Process</span>
            <span v-for="item in [
              STATUS_META.SUCCESS,
              STATUS_META.RUNNING,
              STATUS_META.PARTIAL,
              STATUS_META.FAILED,
              STATUS_META.PENDING,
            ]" :key="item.text" class="fulltopo-legend-item">
              <i class="fulltopo-legend-dot" :style="{ background: item.hex }" />{{ item.text }}
            </span>
            <span class="fulltopo-legend-hint">虚线段表示并行/独立支路</span>
            <span class="fulltopo-legend-time">最近查询 {{ fmtISO(overview.generated_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <v-alert v-if="!overview && !loading" type="info" variant="tonal">
      暂无 CDEU 工作流定义数据。
    </v-alert>
  </div>
</template>

<style scoped>
.fulltopo-shell {
  border: 1px solid rgba(128, 128, 128, 0.22);
  border-radius: 10px;
  background: rgba(128, 128, 128, 0.025);
  overflow: hidden;
}
.fulltopo-scroll {
  overflow: auto;
  max-height: calc(100vh - 190px);
}
.fulltopo-mode-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 10px;
}
.fulltopo-vertical-shell {
  margin-top: 4px;
}
.fulltopo-vertical-scroll {
  max-height: calc(100vh - 150px);
}
.fulltopo-canvas {
  position: relative;
  min-width: 100%;
  background-image: radial-gradient(rgba(128, 128, 128, 0.18) 0.7px, transparent 0.7px);
  background-size: 18px 18px;
}
.fulltopo-phase {
  position: absolute;
  z-index: 1;
  box-sizing: border-box;
  border: 2px dashed rgba(128, 128, 128, 0.68);
  border-radius: 10px;
  background: linear-gradient(180deg, rgba(128, 128, 128, 0.075), rgba(128, 128, 128, 0.018));
  overflow: hidden;
  isolation: isolate;
}
.fulltopo-phase-running {
  animation: fulltopo-phase-pulse 1.8s ease-in-out infinite;
}
/* 阶段框内的内容抬到跑马灯之上，避免被遮住 */
.fulltopo-phase > * {
  position: relative;
  z-index: 1;
}
/* 运行中阶段的跑马灯：青色斜向流光带，直观提示「当前阶段正在跑」 */
.fulltopo-phase-running::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: linear-gradient(
    115deg,
    transparent 32%,
    rgba(0, 229, 255, 0.16) 46%,
    rgba(0, 229, 255, 0.42) 50%,
    rgba(0, 229, 255, 0.16) 54%,
    transparent 68%
  );
  background-size: 220% 100%;
  animation: fulltopo-marquee 2.2s linear infinite;
}
@keyframes fulltopo-marquee {
  0% { background-position: 160% 0; }
  100% { background-position: -160% 0; }
}
@keyframes fulltopo-phase-pulse {
  0%, 100% {
    border-color: rgba(0, 229, 255, 0.62);
    box-shadow: inset 0 0 0 1px rgba(0, 229, 255, 0.18);
  }
  50% {
    border-color: rgba(0, 229, 255, 1);
    box-shadow: inset 0 0 0 1px rgba(0, 229, 255, 0.42), 0 0 14px rgba(0, 229, 255, 0.28);
  }
}
.fulltopo-phase-header {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 52px;
  padding: 8px 12px 6px 14px;
  border-bottom: 1px dashed rgba(128, 128, 128, 0.28);
}
.fulltopo-phase-seq {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 24px;
  height: 24px;
  border-radius: 6px;
  background: var(--phase-color);
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}
.fulltopo-phase-title {
  min-width: 0;
  flex: 1;
  line-height: 1.15;
}
.fulltopo-phase-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 700;
}
.fulltopo-phase-code {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgba(128, 128, 128, 0.92);
  font-family: monospace;
  font-size: 10px;
}
/* 阶段进度条 + 右侧百分比（需求 10748）：轨道为弹性，百分比固定在右端 */
.fulltopo-phase-progress {
  position: absolute;
  top: 53px;
  left: 14px;
  right: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
  height: 12px;
}
.fulltopo-phase-progress-track {
  flex: 1 1 auto;
  height: 2px;
  overflow: hidden;
  border-radius: 2px;
  background: rgba(128, 128, 128, 0.16);
}
.fulltopo-phase-progress-track > span {
  display: block;
  height: 100%;
  transition: width 0.3s ease;
}
.fulltopo-phase-percent {
  flex: none;
  min-width: 30px;
  text-align: right;
  font-size: 10px;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  color: rgba(128, 128, 128, 0.95);
}
.fulltopo-phase-summary {
  position: absolute;
  right: 12px;
  bottom: 5px;
  opacity: 0.65;
  font-size: 10px;
}
.fulltopo-vphase-title {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 156px;
  padding: 10px 12px;
  overflow: hidden;
  border-right: 1px dashed rgba(128, 128, 128, 0.44);
}
.fulltopo-vphase-title-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.fulltopo-vphase-name {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.25;
  word-break: break-word;
}
.fulltopo-vphase-code {
  overflow: hidden;
  color: rgba(128, 128, 128, 0.92);
  font-family: monospace;
  font-size: 9px;
  line-height: 1.2;
  word-break: break-all;
}
.fulltopo-vphase-divider {
  position: absolute;
  top: 0;
  bottom: 0;
  z-index: 2;
  width: 3px;
  overflow: hidden;
  background: rgba(128, 128, 128, 0.20);
}
/* 竖版阶段：纵向进度条右侧的百分比标签（需求 10748） */
.fulltopo-vphase-percent {
  position: absolute;
  top: 6px;
  z-index: 2;
  font-size: 10px;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  color: rgba(128, 128, 128, 0.95);
}
.fulltopo-vphase-divider span {
  display: block;
  width: 100%;
  transition: height 0.3s ease;
}
.fulltopo-vphase-summary {
  margin-top: auto;
  color: rgba(128, 128, 128, 0.82);
  font-size: 10px;
  line-height: 1.2;
}
.fulltopo-fork {
  position: absolute;
  z-index: 3;
  box-sizing: border-box;
  border: 2px solid var(--fork-color, #90a4ae);
  border-radius: 50%;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 0 0 2px rgba(128, 128, 128, 0.14);
}
.fulltopo-edge-layer {
  position: absolute;
  z-index: 2;
  inset: 0;
  pointer-events: none;
}
.fulltopo-edge-running {
  animation: fulltopo-dash 1.1s linear infinite;
}
@keyframes fulltopo-dash {
  to { stroke-dashoffset: -18; }
}
.fulltopo-node {
  position: absolute;
  z-index: 3;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 8px 10px 7px;
  border: 1px solid rgba(128, 128, 128, 0.42);
  border-left: 4px solid var(--status-color);
  border-radius: 7px;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.14);
  cursor: default;
  overflow: hidden;
  isolation: isolate;
  transition: transform 0.16s ease, box-shadow 0.16s ease;
}
.fulltopo-node > * {
  position: relative;
  z-index: 1;
}
.fulltopo-node:hover {
  transform: translateY(-1px);
  box-shadow: 0 7px 16px rgba(0, 0, 0, 0.20);
}
.fulltopo-node-running {
  animation: fulltopo-node-pulse 1.8s ease-in-out infinite;
}
/* 运行中的 Process 矩形框：青色跑马灯 */
.fulltopo-node-running::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background: linear-gradient(
    115deg,
    transparent 32%,
    rgba(0, 229, 255, 0.20) 46%,
    rgba(0, 229, 255, 0.52) 50%,
    rgba(0, 229, 255, 0.20) 54%,
    transparent 68%
  );
  background-size: 220% 100%;
  animation: fulltopo-marquee 2.2s linear infinite;
}
.fulltopo-node-running .v-icon {
  animation: fulltopo-spin 1s linear infinite;
}
@keyframes fulltopo-node-pulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 229, 255, 0.0), 0 3px 10px rgba(0, 0, 0, 0.14); }
  50% { box-shadow: 0 0 0 3px rgba(0, 229, 255, 0.34), 0 3px 10px rgba(0, 229, 255, 0.30); }
}
@keyframes fulltopo-spin {
  to { transform: rotate(360deg); }
}
.fulltopo-node-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.fulltopo-node-seq {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 18px;
  height: 18px;
  border-radius: 4px;
  background: var(--status-color);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
}
.fulltopo-node-name {
  flex: 1;
  overflow: visible;
  text-overflow: clip;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 700;
}
.fulltopo-node-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 5px;
  color: rgba(128, 128, 128, 0.95);
  font-size: 10px;
}
.fulltopo-node-code {
  min-width: 0;
  overflow: visible;
  text-overflow: clip;
  white-space: nowrap;
  font-family: monospace;
}
/* 节点进度条 + 右侧百分比（需求 10748） */
.fulltopo-node-progress {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: auto;
  height: 12px;
}
.fulltopo-node-progress-track {
  flex: 1 1 auto;
  height: 2px;
  overflow: hidden;
  border-radius: 2px;
  background: rgba(128, 128, 128, 0.18);
}
.fulltopo-node-progress-track > span {
  display: block;
  height: 100%;
  border-radius: 2px;
  background: var(--status-color);
  transition: width 0.3s ease;
}
.fulltopo-node-percent {
  flex: none;
  min-width: 28px;
  text-align: right;
  font-size: 9px;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  color: rgba(128, 128, 128, 0.95);
}
.fulltopo-terminal {
  position: absolute;
  z-index: 3;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  border: 2px solid #9e9e9e;
  border-radius: 6px;
  background: rgb(var(--v-theme-surface));
  color: rgb(var(--v-theme-on-surface));
  font-size: 13px;
  font-weight: 700;
  box-shadow: 0 2px 7px rgba(0, 0, 0, 0.12);
}
.fulltopo-terminal small {
  opacity: 0.65;
  font-size: 9px;
  font-weight: 400;
}
.fulltopo-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  font-size: 12px;
}
.fulltopo-legend-in-canvas {
  position: absolute;
  z-index: 4;
  max-width: calc(100% - 40px);
  padding: 8px 12px;
  border: 1px solid rgba(128, 128, 128, 0.32);
  border-radius: 8px;
  background: rgb(var(--v-theme-surface));
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
}
.fulltopo-legend-title {
  font-weight: 700;
}
.fulltopo-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: rgba(128, 128, 128, 0.95);
}
.fulltopo-legend-phase {
  display: inline-block;
  width: 18px;
  height: 11px;
  border: 1px dashed rgba(128, 128, 128, 0.75);
  border-radius: 3px;
}
.fulltopo-legend-process {
  display: inline-block;
  width: 18px;
  height: 11px;
  border: 1px solid rgba(128, 128, 128, 0.5);
  border-left: 3px solid #2e7d32;
  border-radius: 3px;
  background: rgb(var(--v-theme-surface));
}
.fulltopo-legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.fulltopo-legend-hint,
.fulltopo-legend-time {
  margin-left: auto;
  color: rgba(128, 128, 128, 0.85);
  font-size: 11px;
}
.fulltopo-legend-time {
  margin-left: 0;
}
@media (max-width: 900px) {
  .fulltopo-legend-hint,
  .fulltopo-legend-time {
    margin-left: 0;
  }
}
/* 尊重系统的「减少动画」偏好：关闭跑马灯与脉动，仅保留静态高亮 */
@media (prefers-reduced-motion: reduce) {
  .fulltopo-phase-running {
    animation: none;
    border-color: rgba(0, 229, 255, 0.9);
  }
  .fulltopo-phase-running::after,
  .fulltopo-node-running::after {
    animation: none;
    background: rgba(0, 229, 255, 0.10);
  }
  .fulltopo-node-running {
    animation: none;
    box-shadow: 0 0 0 2px rgba(0, 229, 255, 0.55);
  }
}
</style>
