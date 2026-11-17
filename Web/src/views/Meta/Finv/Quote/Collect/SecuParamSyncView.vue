<script setup lang="ts">
// 证券参数同步（需求 11042–11055）
// Tab 1「FT证券基础参数同步」：
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Status          只读快照（20013 状态 + 源表名 + 目标表行数）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Config          保存源表名（写系统配置 20014）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run        异步启动（返回 run_id）
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run        运行时快照（进度 + 逐批实时日志）
// Tab 2「FT证券指标数据同步」（需求 11050，来源表固定 public.finv_quote_secu_meta）：
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status          两张目标表行数
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run        异步启动
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run        运行时快照
//
// Tab 3「本地 usc 回推上游」（需求 11084 重构，方向相反：本地 → 上游，回推 usc + priority 两列，需求 11142）：
//   源 futu.finv_futu_secu_key.usc/priority → 目标 public.finv_quote_secu_key.usc/priority（按 ftmm_symbol）
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Status         本地键表行数 / usc 有值行数
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run       异步启动
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run       运行时快照
//
// 三个 Tab 都是「异步启动 + 800ms 轮询 + 独立滚动日志区」，实现共享 useAsyncRun。
import { computed, nextTick, onBeforeUnmount, onMounted, ref, type ComputedRef, type Ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// ---------- 公共类型 ----------
interface RunLog {
  seq: number
  time: string
  level: string
  text: string
  batch?: number
  offset?: number
}

interface RunBase {
  run_id: string
  status: string // RUNNING / DONE / FAILED
  source_table: string
  batch_size: number
  rows_total: number
  rows_receive: number
  batches: number
  progress: number
  started_at: string
  finished_at?: string
  elapsed_ms: number
  error?: string
  logs: RunLog[]
}

// Tab 1 运行快照（基础参数同步）
interface SyncRun extends RunBase {
  insert: number
  update: number
  unchanged: number
  skipped: number
  status_reset: number
}

// Tab 1 状态快照
interface SecuKeyStatus {
  configured: boolean
  project_id: string
  api_key_masked: string
  config_error?: string
  source_table: string
  source_table_raw?: string
  target_table: string
  target_rows: number
  target_last_update?: string
  batch_size_default: number
}

// Tab 2 运行快照（指标数据同步）
interface IndicatorRun extends RunBase {
  kind: string
  ind_insert: number
  ind_update: number
  ind_unchanged: number
  qc_insert: number
  qc_update: number
  qc_skip: number
}

// Tab 2 状态快照
interface IndicatorStatus {
  configured: boolean
  project_id: string
  api_key_masked: string
  config_error?: string
  source_table: string
  targets: { table: string; rows: number }[]
  batch_size_default: number
}

// Tab 3 运行快照（本地 usc 回推上游：写 public.finv_quote_secu_key 的 usc + priority 两列）
interface BackfillRun extends RunBase {
  kind: string
  key_table: string
  key_column: string
  key_key: string
  local_table: string
  rows_done: number
  fill: number
  overwrite: number
  keep: number
  missing: number
  // 本次回推写的列（需求 11163）
  fields?: FieldSelection
  // 上游缺记录 → 已插入（upsert，需求 11157）
  insert: number
  // 上游缺记录但本地缺 instrument/market/code → 未插入
  insert_skipped: number
  local_empty_usc: number
  // Tab 3 的运行时间字段按去 -ed 口径命名（Tab 1/2 仍为 started_at/elapsed_ms，后续统一）
  start_at: string
  finish_at?: string
  duration_ms: number
}

// Tab 3 状态快照
interface BackfillStatus {
  configured: boolean
  project_id: string
  api_key_masked: string
  config_error?: string
  key_table: string
  key_column: string
  key_key: string
  local_table: string
  local_rows: number
  local_with_usc: number
  local_empty_usc: number
  batch_size_default: number
}

const STATUS_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Status'
const CONFIG_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Config'
const SYNC_RUN_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run'
const IND_STATUS_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Status'
const IND_RUN_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Indicator/Sync/Run'
const BACKFILL_STATUS_API = '/Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Status'
const BACKFILL_RUN_API = '/Meta/Finv/Quote/Cdeu/SecuKey/UscBackfill/Sync/Run'

const BATCH_OPTIONS = [500, 1000, 2000, 5000]
const DEFAULT_SOURCE_TABLE = 'finv_quote_secu_key'
const START_TIMEOUT_MS = 60 * 1000
const POLL_INTERVAL_MS = 800

// Tab 3 的批量档位更小：逐行走 PATCH，批量大只会让单批耗时变长、日志更粗
const BACKFILL_BATCH_OPTIONS = [200, 500, 1000, 2000]

const tab = ref<'basic' | 'indicator' | 'backfill'>('basic')

// ---------- 通用工具 ----------
function num(value: number | undefined | null): string {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString('en-US')
}

function timeText(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}:${pad(date.getSeconds())}`
}

function clockText(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--:--:--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function elapsedText(ms: number | undefined): string {
  if (!ms || ms < 0) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

// 进度百分比裁剪到 0~100
function clampProgress(value: number | undefined): number {
  const v = value ?? 0
  return v < 0 ? 0 : v > 100 ? 100 : v
}

// ---------- 异步运行轮询（两个 Tab 共用） ----------
interface AsyncRunHandle<T extends RunBase> {
  run: Ref<T | null>
  running: ComputedRef<boolean>
  starting: Ref<boolean>
  error: Ref<string>
  start: (label: string) => Promise<void>
  stop: () => void
}

function useAsyncRun<T extends RunBase>(
  api: string,
  buildBody: () => Record<string, unknown>,
  logBox: Ref<HTMLElement | null>,
): AsyncRunHandle<T> {
  const run = ref<T | null>(null) as Ref<T | null>
  const starting = ref(false)
  const error = ref('')
  let timer: number | null = null

  const running = computed(() => run.value?.status === 'RUNNING')

  function stop() {
    if (timer !== null) {
      window.clearInterval(timer)
      timer = null
    }
  }

  async function scrollToBottom() {
    await nextTick()
    const box = logBox.value
    if (box) box.scrollTop = box.scrollHeight
  }

  async function start(label: string) {
    starting.value = true
    error.value = ''
    stop()
    try {
      const snapshot = await apiPost<T>(api, buildBody(), START_TIMEOUT_MS)
      run.value = snapshot
      await scrollToBottom()
      timer = window.setInterval(async () => {
        try {
          const next = await apiGet<T>(`${api}?runId=${encodeURIComponent(snapshot.run_id)}`)
          run.value = next
          await scrollToBottom()
          if (next.status !== 'RUNNING') {
            stop()
            starting.value = false
            if (next.status === 'DONE') {
              showToast(`${label}完成：共 ${num(next.rows_receive)} 行 / ${next.batches} 批`, 'success')
            } else {
              showToast(`${label}失败：${next.error || '未知错误'}`, 'error')
            }
          }
        } catch (err) {
          stop()
          starting.value = false
          error.value = errorText(err)
          showToast(error.value, 'error')
        }
      }, POLL_INTERVAL_MS)
    } catch (err) {
      starting.value = false
      error.value = errorText(err)
      showToast(error.value, 'error')
    }
  }

  return { run, running, starting, error, start, stop }
}

// ---------- Tab 1：FT证券基础参数同步 ----------
const status = ref<SecuKeyStatus | null>(null)
const statusLoading = ref(false)
const sourceTable = ref(DEFAULT_SOURCE_TABLE)
const batchSize = ref(1000)
const saving = ref(false)
const logBoxBasic = ref<HTMLElement | null>(null)

const configured = computed(() => status.value?.configured === true)
const sourceTableChanged = computed(() => {
  const current = (status.value?.source_table || DEFAULT_SOURCE_TABLE).trim()
  return sourceTable.value.trim() !== current
})

const basic = useAsyncRun<SyncRun>(
  SYNC_RUN_API,
  () => ({ source_table: sourceTable.value.trim(), batch_size: batchSize.value }),
  logBoxBasic,
)

async function loadStatus() {
  statusLoading.value = true
  try {
    const data = await apiGet<SecuKeyStatus>(STATUS_API)
    status.value = data
    sourceTable.value = data.source_table || DEFAULT_SOURCE_TABLE
    if (data.batch_size_default > 0) {
      batchSize.value = BATCH_OPTIONS.includes(data.batch_size_default) ? data.batch_size_default : 1000
    }
  } catch (err) {
    showToast(errorText(err), 'error')
  } finally {
    statusLoading.value = false
  }
}

async function saveSourceTable() {
  saving.value = true
  try {
    const data = await apiPost<{ source_table: string }>(CONFIG_API, { source_table: sourceTable.value.trim() })
    sourceTable.value = data.source_table
    showToast(`源表名已保存：${data.source_table}`, 'success')
    await loadStatus()
  } catch (err) {
    showToast(errorText(err), 'error')
  } finally {
    saving.value = false
  }
}

async function runBasicSync() {
  if (!configured.value) {
    showToast('系统配置 20013（Supabase 连接配置）未配置，请先填写 {PROJECT_ID}|{APIKEY}', 'warning')
    return
  }
  await basic.start('基础参数同步')
  await loadStatus()
}

// ---------- Tab 2：FT证券指标数据同步 ----------
const indStatus = ref<IndicatorStatus | null>(null)
const indStatusLoading = ref(false)
const indBatchSize = ref(1000)
const logBoxIndicator = ref<HTMLElement | null>(null)

const indConfigured = computed(() => indStatus.value?.configured === true)

const ind = useAsyncRun<IndicatorRun>(
  IND_RUN_API,
  () => ({ batch_size: indBatchSize.value }),
  logBoxIndicator,
)

async function loadIndicatorStatus() {
  indStatusLoading.value = true
  try {
    const data = await apiGet<IndicatorStatus>(IND_STATUS_API)
    indStatus.value = data
    if (data.batch_size_default > 0) {
      indBatchSize.value = BATCH_OPTIONS.includes(data.batch_size_default) ? data.batch_size_default : 1000
    }
  } catch (err) {
    showToast(errorText(err), 'error')
  } finally {
    indStatusLoading.value = false
  }
}

async function runIndicatorSync() {
  if (!indConfigured.value) {
    showToast('系统配置 20013（Supabase 连接配置）未配置，请先填写 {PROJECT_ID}|{APIKEY}', 'warning')
    return
  }
  await ind.start('指标数据同步')
  await loadIndicatorStatus()
}

// ---------- Tab 3：本地 usc 回推上游 ----------
/** 本次回推要写上游哪些列（需求 11163）；默认两个都勾 */
interface FieldSelection {
  usc: boolean
  priority: boolean
}

const bfStatus = ref<BackfillStatus | null>(null)
const bfStatusLoading = ref(false)
const bfBatchSize = ref(500)
// 字段勾选（需求 11163）：默认 usc + priority 都写；上游 priority 是人工设定时可只勾 usc
const bfFields = ref<FieldSelection>({ usc: true, priority: true })
const bfFieldsInvalid = computed(() => !bfFields.value.usc && !bfFields.value.priority)
/** 字段选择 → 「usc+priority」可读文本（空选择显示「未选列」） */
function fieldsText(fields: FieldSelection | undefined | null): string {
  if (!fields) return '—'
  const picked: string[] = []
  if (fields.usc) picked.push('usc')
  if (fields.priority) picked.push('priority')
  return picked.length ? picked.join('+') : '未选列'
}

/** 本次将写哪些列（按钮与参数区展示用） */
const bfFieldsText = computed(() => fieldsText(bfFields.value))
const logBoxBackfill = ref<HTMLElement | null>(null)

const bfConfigured = computed(() => bfStatus.value?.configured === true)

const bf = useAsyncRun<BackfillRun>(
  BACKFILL_RUN_API,
  () => ({
    batch_size: bfBatchSize.value,
    // 需求 11163：带上字段选择，后端只写勾选的列
    fields: { usc: bfFields.value.usc, priority: bfFields.value.priority },
  }),
  logBoxBackfill,
)

async function loadBackfillStatus() {
  bfStatusLoading.value = true
  try {
    const data = await apiGet<BackfillStatus>(BACKFILL_STATUS_API)
    bfStatus.value = data
    if (data.batch_size_default > 0) {
      bfBatchSize.value = BACKFILL_BATCH_OPTIONS.includes(data.batch_size_default) ? data.batch_size_default : 500
    }
  } catch (err) {
    showToast(errorText(err), 'error')
  } finally {
    bfStatusLoading.value = false
  }
}

async function runBackfill() {
  if (!bfConfigured.value) {
    showToast('系统配置 20013（Supabase 连接配置）未配置，请先填写 {PROJECT_ID}|{APIKEY}', 'warning')
    return
  }
  if (bfFieldsInvalid.value) {
    showToast('同步字段至少要勾选一个（usc 或 priority）', 'warning')
    return
  }
  await bf.start('本地 usc 回推上游')
  await loadBackfillStatus()
}

onMounted(async () => {
  await loadStatus()
  await loadIndicatorStatus()
  await loadBackfillStatus()
})

onBeforeUnmount(() => {
  basic.stop()
  ind.stop()
  bf.stop()
})
</script>

<template>
  <div>
    <div class="d-flex align-center mb-2">
      <v-icon icon="mdi-database-sync-outline" class="mr-2" />
      <span class="text-h6">证券参数同步</span>
      <v-spacer />
      <v-btn
        variant="text"
        size="small"
        prepend-icon="mdi-refresh"
        :loading="statusLoading || indStatusLoading || bfStatusLoading"
        @click="loadStatus(); loadIndicatorStatus(); loadBackfillStatus()"
      >
        刷新状态
      </v-btn>
    </div>
    <div class="text-caption text-medium-emphasis mb-2">
      Tab 1/2 从 Supabase 上游按主键分批增量同步到本地；Tab 3 反向把本地
      <code>futu.finv_futu_secu_key.usc</code> / <code>.priority</code> 回推上游
      <code>public.finv_quote_secu_key.usc</code> / <code>.priority</code>
      （按 <code>ftmm_symbol</code> 关联，两列一起写）。两列都已一致的行不改写。
    </div>

    <v-tabs v-model="tab" density="comfortable" color="primary">
      <v-tab value="basic">
        <v-icon icon="mdi-key-chain" class="mr-2" />
        FT证券基础参数同步
      </v-tab>
      <v-tab value="indicator">
        <v-icon icon="mdi-chart-box-outline" class="mr-2" />
        FT证券指标数据同步
      </v-tab>
      <v-tab value="backfill">
        <v-icon icon="mdi-upload-outline" class="mr-2" />
        本地 usc 回推上游
      </v-tab>
    </v-tabs>

    <!-- ============ Tab 1：FT证券基础参数同步 ============ -->
    <div v-if="tab === 'basic'">
      <v-card class="mt-3">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-cloud-check-outline" class="mr-2" />
          连接与目标表
        </v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">Supabase 连接（系统配置 20013）</div>
              <div class="d-flex align-center flex-wrap ga-2 mt-1">
                <v-chip v-if="configured" size="small" color="success" variant="tonal" prepend-icon="mdi-check-circle-outline">
                  已配置
                </v-chip>
                <v-chip v-else size="small" color="warning" variant="tonal" prepend-icon="mdi-alert-outline">
                  未配置
                </v-chip>
                <code v-if="configured" class="text-body-2">{{ status?.project_id }}</code>
                <span v-if="configured" class="text-caption text-medium-emphasis">
                  APIKEY {{ status?.api_key_masked }}（已脱敏）
                </span>
              </div>
              <div v-if="status?.config_error" class="text-caption text-warning mt-1">{{ status.config_error }}</div>
            </v-col>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">本地目标表</div>
              <div class="d-flex align-center flex-wrap ga-2 mt-1">
                <code class="text-body-2">{{ status?.target_table }}</code>
                <v-chip size="small" variant="tonal" color="primary">共 {{ num(status?.target_rows) }} 行</v-chip>
              </div>
              <div class="text-caption text-medium-emphasis mt-1">
                最近变更：{{ timeText(status?.target_last_update) }}
              </div>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>

      <v-card class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-tune-variant" class="mr-2" />
          同步参数
        </v-card-title>
        <v-card-text>
          <v-row dense align="start">
            <v-col cols="12" md="5">
              <v-text-field
                v-model="sourceTable"
                label="Supabase 源表名"
                density="compact"
                variant="outlined"
                hide-details
                :placeholder="DEFAULT_SOURCE_TABLE"
                hint="默认 finv_quote_secu_key；可写 schema.table 两段，只允许字母/数字/下划线"
                persistent-hint
              />
            </v-col>
            <v-col cols="12" md="3">
              <v-select
                v-model="batchSize"
                :items="BATCH_OPTIONS"
                label="每批行数"
                density="compact"
                variant="outlined"
                hide-details
              />
            </v-col>
            <v-col cols="12" md="4" class="d-flex align-center ga-2">
              <v-btn
                variant="tonal"
                color="primary"
                prepend-icon="mdi-content-save-outline"
                :loading="saving"
                :disabled="basic.running.value || !sourceTableChanged"
                @click="saveSourceTable"
              >
                保存源表名
              </v-btn>
              <v-btn
                color="primary"
                prepend-icon="mdi-play-circle-outline"
                :loading="basic.starting.value && basic.running.value"
                :disabled="!configured || basic.running.value"
                @click="runBasicSync"
              >
                {{ basic.running.value ? '同步中…' : '立即同步' }}
              </v-btn>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>

      <v-alert v-if="basic.error.value" type="error" variant="tonal" class="mt-4" density="compact">
        {{ basic.error.value }}
      </v-alert>

      <v-card v-if="basic.run.value" class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-progress-clock" class="mr-2" />
          同步进度与日志
          <v-spacer />
          <v-chip
            size="small"
            variant="tonal"
            :color="basic.run.value.status === 'RUNNING' ? 'info' : basic.run.value.status === 'DONE' ? 'success' : 'error'"
          >
            {{ basic.run.value.status === 'RUNNING' ? '运行中' : basic.run.value.status === 'DONE' ? '已完成' : '失败' }}
          </v-chip>
        </v-card-title>
        <v-card-text>
          <div class="d-flex flex-wrap ga-2 mb-2">
            <v-chip size="small" variant="tonal" color="info">拉取 {{ num(basic.run.value.rows_receive) }}</v-chip>
            <v-chip size="small" variant="tonal" color="success">新增 {{ num(basic.run.value.insert) }}</v-chip>
            <v-chip size="small" variant="tonal" color="primary">更新 {{ num(basic.run.value.update) }}</v-chip>
            <v-chip size="small" variant="tonal">未变 {{ num(basic.run.value.unchanged) }}</v-chip>
            <v-chip v-if="basic.run.value.skipped" size="small" variant="tonal" color="warning">
              跳过 {{ num(basic.run.value.skipped) }}
            </v-chip>
            <v-chip v-if="basic.run.value.status_reset" size="small" variant="tonal" color="warning">
              status 归一 {{ num(basic.run.value.status_reset) }}
            </v-chip>
            <v-chip size="small" variant="text">
              {{ basic.run.value.batches }} 批 · {{ elapsedText(basic.run.value.elapsed_ms) }}
            </v-chip>
          </div>

          <v-progress-linear
            :model-value="clampProgress(basic.run.value.progress)"
            :indeterminate="basic.running.value && clampProgress(basic.run.value.progress) === 0"
            height="8"
            rounded
            class="mb-1"
          />
          <div class="text-caption text-medium-emphasis mb-2">
            进度 {{ clampProgress(basic.run.value.progress).toFixed(1) }}%（{{ num(basic.run.value.rows_receive) }} /
            {{ num(basic.run.value.rows_total) }} 行）· 源表 <code>{{ basic.run.value.source_table || sourceTable }}</code>
            · run_id <code>{{ basic.run.value.run_id }}</code>
          </div>

          <div ref="logBoxBasic" class="finv-secukey-logbox">
            <div
              v-for="log in basic.run.value.logs"
              :key="log.seq"
              class="finv-secukey-logline"
              :class="{ 'finv-secukey-logline-error': log.level === 'ERROR' }"
            >
              <span class="finv-secukey-logtime">{{ clockText(log.time) }}</span>
              <span class="finv-secukey-logtext">{{ log.text }}</span>
            </div>
            <div v-if="basic.running.value" class="finv-secukey-logline finv-secukey-logline-wait">
              <span class="finv-secukey-logtime">……</span>
              <span class="finv-secukey-logtext">等待下一批…</span>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </div>

    <!-- ============ Tab 2：FT证券指标数据同步 ============ -->
    <div v-else-if="tab === 'indicator'">
      <v-card class="mt-3">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-cloud-check-outline" class="mr-2" />
          连接与目标表
        </v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">Supabase 连接（系统配置 20013）</div>
              <div class="d-flex align-center flex-wrap ga-2 mt-1">
                <v-chip
                  v-if="indConfigured"
                  size="small"
                  color="success"
                  variant="tonal"
                  prepend-icon="mdi-check-circle-outline"
                >
                  已配置
                </v-chip>
                <v-chip v-else size="small" color="warning" variant="tonal" prepend-icon="mdi-alert-outline">
                  未配置
                </v-chip>
                <code v-if="indConfigured" class="text-body-2">{{ indStatus?.project_id }}</code>
                <span v-if="indConfigured" class="text-caption text-medium-emphasis">
                  APIKEY {{ indStatus?.api_key_masked }}（已脱敏）
                </span>
              </div>
              <div v-if="indStatus?.config_error" class="text-caption text-warning mt-1">{{ indStatus.config_error }}</div>
            </v-col>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">上游源表（固定）</div>
              <code class="text-body-2">{{ indStatus?.source_table || 'public.finv_quote_secu_meta' }}</code>
            </v-col>
          </v-row>

          <v-divider class="my-3" />

          <div class="text-caption text-medium-emphasis mb-1">本地目标表（一次同步写两张）</div>
          <div class="d-flex flex-wrap ga-2">
            <v-chip
              v-for="item in indStatus?.targets || []"
              :key="item.table"
              size="small"
              variant="tonal"
              color="primary"
            >
              <code class="mr-1">{{ item.table }}</code> {{ num(item.rows) }} 行
            </v-chip>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-tune-variant" class="mr-2" />
          同步参数
        </v-card-title>
        <v-card-text>
          <v-row dense align="start">
            <v-col cols="12" md="3">
              <v-select
                v-model="indBatchSize"
                :items="BATCH_OPTIONS"
                label="每批行数"
                density="compact"
                variant="outlined"
                hide-details
              />
            </v-col>
            <v-col cols="12" md="9" class="d-flex align-center">
              <v-btn
                color="primary"
                prepend-icon="mdi-play-circle-outline"
                :loading="ind.starting.value && ind.running.value"
                :disabled="!indConfigured || ind.running.value"
                @click="runIndicatorSync"
              >
                {{ ind.running.value ? '同步中…' : '立即同步' }}
              </v-btn>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>

      <v-alert v-if="ind.error.value" type="error" variant="tonal" class="mt-4" density="compact">
        {{ ind.error.value }}
      </v-alert>

      <v-card v-if="ind.run.value" class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-progress-clock" class="mr-2" />
          同步进度与日志
          <v-spacer />
          <v-chip
            size="small"
            variant="tonal"
            :color="ind.run.value.status === 'RUNNING' ? 'info' : ind.run.value.status === 'DONE' ? 'success' : 'error'"
          >
            {{ ind.run.value.status === 'RUNNING' ? '运行中' : ind.run.value.status === 'DONE' ? '已完成' : '失败' }}
          </v-chip>
        </v-card-title>
        <v-card-text>
          <div class="d-flex flex-wrap ga-2 mb-2">
            <v-chip size="small" variant="tonal" color="info">拉取 {{ num(ind.run.value.rows_receive) }}</v-chip>
            <v-chip size="small" variant="tonal" color="success">指标 新增 {{ num(ind.run.value.ind_insert) }}</v-chip>
            <v-chip size="small" variant="tonal" color="primary">指标 更新 {{ num(ind.run.value.ind_update) }}</v-chip>
            <v-chip size="small" variant="tonal">指标 未变 {{ num(ind.run.value.ind_unchanged) }}</v-chip>
            <v-chip size="small" variant="tonal" color="success">
              采集属性 新增 {{ num(ind.run.value.qc_insert) }}
            </v-chip>
            <v-chip size="small" variant="tonal" color="primary">
              采集属性 更新 {{ num(ind.run.value.qc_update) }}
            </v-chip>
            <v-chip v-if="ind.run.value.qc_skip" size="small" variant="tonal" color="warning">
              采集属性 跳过 {{ num(ind.run.value.qc_skip) }}
            </v-chip>
            <v-chip size="small" variant="text">
              {{ ind.run.value.batches }} 批 · {{ elapsedText(ind.run.value.elapsed_ms) }}
            </v-chip>
          </div>

          <v-progress-linear
            :model-value="clampProgress(ind.run.value.progress)"
            :indeterminate="ind.running.value && clampProgress(ind.run.value.progress) === 0"
            height="8"
            rounded
            class="mb-1"
          />
          <div class="text-caption text-medium-emphasis mb-2">
            进度 {{ clampProgress(ind.run.value.progress).toFixed(1) }}%（{{ num(ind.run.value.rows_receive) }} /
            {{ num(ind.run.value.rows_total) }} 行）· 源表 <code>{{ ind.run.value.source_table }}</code>
            · run_id <code>{{ ind.run.value.run_id }}</code>
          </div>

          <div ref="logBoxIndicator" class="finv-secukey-logbox">
            <div
              v-for="log in ind.run.value.logs"
              :key="log.seq"
              class="finv-secukey-logline"
              :class="{ 'finv-secukey-logline-error': log.level === 'ERROR' }"
            >
              <span class="finv-secukey-logtime">{{ clockText(log.time) }}</span>
              <span class="finv-secukey-logtext">{{ log.text }}</span>
            </div>
            <div v-if="ind.running.value" class="finv-secukey-logline finv-secukey-logline-wait">
              <span class="finv-secukey-logtime">……</span>
              <span class="finv-secukey-logtext">等待下一批…</span>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </div>

    <!-- ============ Tab 3：本地 usc 回推上游 ============ -->
    <div v-else>
      <v-card class="mt-3">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-cloud-upload-outline" class="mr-2" />
          连接与目标表
        </v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">Supabase 连接（系统配置 20013）</div>
              <div class="d-flex align-center flex-wrap ga-2 mt-1">
                <v-chip
                  v-if="bfConfigured"
                  size="small"
                  color="success"
                  variant="tonal"
                  prepend-icon="mdi-check-circle-outline"
                >
                  已配置
                </v-chip>
                <v-chip v-else size="small" color="warning" variant="tonal" prepend-icon="mdi-alert-outline">
                  未配置
                </v-chip>
                <code v-if="bfConfigured" class="text-body-2">{{ bfStatus?.project_id }}</code>
                <span v-if="bfConfigured" class="text-caption text-medium-emphasis">
                  APIKEY {{ bfStatus?.api_key_masked }}（已脱敏）
                </span>
              </div>
              <div v-if="bfStatus?.config_error" class="text-caption text-warning mt-1">{{ bfStatus.config_error }}</div>
            </v-col>
            <v-col cols="12" md="6">
              <div class="text-caption text-medium-emphasis">本地来源</div>
              <div class="d-flex align-center flex-wrap ga-2 mt-1">
                <code class="text-body-2">{{ bfStatus?.local_table || 'futu.finv_futu_secu_key' }}</code>
                <v-chip size="small" variant="tonal" color="primary">共 {{ num(bfStatus?.local_rows) }} 行</v-chip>
                <v-chip size="small" variant="tonal" color="info">
                  usc 有值 {{ num(bfStatus?.local_with_usc) }} 行
                </v-chip>
                <v-chip v-if="bfStatus?.local_empty_usc" size="small" variant="text" color="warning">
                  usc 为空跳过 {{ num(bfStatus?.local_empty_usc) }} 行
                </v-chip>
              </div>
            </v-col>
          </v-row>

          <v-divider class="my-3" />

          <div class="text-caption text-medium-emphasis mb-1">
            上游可回推的列（本次写哪几列由下方「同步字段」决定）
          </div>
          <div class="d-flex align-center flex-wrap ga-2">
            <v-chip size="small" variant="tonal" color="secondary">
              <code class="mr-1">{{ bfStatus?.key_table || 'public.finv_quote_secu_key' }}</code>
              .{{ bfStatus?.key_column || 'usc' }}
            </v-chip>
            <v-chip size="small" variant="tonal" color="secondary">
              <code class="mr-1">{{ bfStatus?.key_table || 'public.finv_quote_secu_key' }}</code>
              .priority
            </v-chip>
            <span class="text-caption text-medium-emphasis">
              按 <code>{{ bfStatus?.key_key || 'ftmm_symbol' }}</code> 与本地关联（双边主键，一对一）；
              勾选的列在同一行一次写入，未勾选的列不发、不改
            </span>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-tune-variant" class="mr-2" />
          回推参数
        </v-card-title>
        <v-card-text>
          <v-row dense align="start">
            <v-col cols="12" md="3">
              <v-select
                v-model="bfBatchSize"
                :items="BACKFILL_BATCH_OPTIONS"
                label="每批 ftmm_symbol 数"
                density="compact"
                variant="outlined"
                hide-details
              />
            </v-col>
            <v-col cols="12" md="9">
              <div class="text-caption text-medium-emphasis mb-1">
                同步字段（本次只写勾选的列，未勾的列上游保持原值）
              </div>
              <div class="d-flex flex-wrap align-center ga-4">
                <v-checkbox
                  v-model="bfFields.usc"
                  label="usc（统一证券代码）"
                  density="compact"
                  hide-details
                  :disabled="bf.running.value"
                />
                <v-checkbox
                  v-model="bfFields.priority"
                  label="priority（优先级）"
                  density="compact"
                  hide-details
                  :disabled="bf.running.value"
                />
                <v-btn
                  color="primary"
                  prepend-icon="mdi-play-circle-outline"
                  :loading="bf.starting.value && bf.running.value"
                  :disabled="!bfConfigured || bf.running.value || bfFieldsInvalid"
                  @click="runBackfill"
                >
                  {{ bf.running.value ? '回推中…' : '立即回推' }}
                </v-btn>
                <v-chip size="small" variant="tonal" :color="bfFieldsInvalid ? 'error' : 'secondary'">
                  将写 {{ bfFieldsText }}
                </v-chip>
              </div>
              <div v-if="bfFieldsInvalid" class="text-caption text-error mt-1">
                至少勾选一个字段才能回推（取消 priority 可保留上游人工设定的优先级）
              </div>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>

      <v-alert v-if="bf.error.value" type="error" variant="tonal" class="mt-4" density="compact">
        {{ bf.error.value }}
      </v-alert>

      <v-card v-if="bf.run.value" class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-progress-clock" class="mr-2" />
          回推进度与日志
          <v-spacer />
          <v-chip
            size="small"
            variant="tonal"
            :color="bf.run.value.status === 'RUNNING' ? 'info' : bf.run.value.status === 'DONE' ? 'success' : 'error'"
          >
            {{ bf.run.value.status === 'RUNNING' ? '运行中' : bf.run.value.status === 'DONE' ? '已完成' : '失败' }}
          </v-chip>
        </v-card-title>
        <v-card-text>
          <div class="d-flex flex-wrap ga-2 mb-2">
            <v-chip size="small" variant="tonal" color="info">
              处理 {{ num(bf.run.value.rows_done) }} / {{ num(bf.run.value.rows_total) }}
            </v-chip>
            <v-chip size="small" variant="tonal" color="success">填入 {{ num(bf.run.value.fill) }}</v-chip>
            <v-chip size="small" variant="tonal" color="primary">覆盖 {{ num(bf.run.value.overwrite) }}</v-chip>
            <v-chip size="small" variant="tonal">未变 {{ num(bf.run.value.keep) }}</v-chip>
            <v-chip v-if="bf.run.value.insert" size="small" variant="tonal" color="success">
              上游新增 {{ num(bf.run.value.insert) }}
            </v-chip>
            <v-chip v-if="bf.run.value.insert_skipped" size="small" variant="tonal" color="warning">
              无法插入 {{ num(bf.run.value.insert_skipped) }}
            </v-chip>
            <v-chip size="small" variant="tonal" color="secondary">
              字段 {{ fieldsText(bf.run.value.fields) }}
            </v-chip>
            <v-chip size="small" variant="text">
              {{ bf.run.value.batches }} 批 · {{ elapsedText(bf.run.value.duration_ms) }}
            </v-chip>
          </div>

          <v-progress-linear
            :model-value="clampProgress(bf.run.value.progress)"
            :indeterminate="bf.running.value && clampProgress(bf.run.value.progress) === 0"
            height="8"
            rounded
            class="mb-1"
          />
          <div class="text-caption text-medium-emphasis mb-2">
            进度 {{ clampProgress(bf.run.value.progress).toFixed(1) }}%（{{ num(bf.run.value.rows_done) }} /
            {{ num(bf.run.value.rows_total) }} 个 <code>{{ bf.run.value.key_key || 'ftmm_symbol' }}</code>）
            · run_id <code>{{ bf.run.value.run_id }}</code>
          </div>

          <div ref="logBoxBackfill" class="finv-secukey-logbox">
            <div
              v-for="log in bf.run.value.logs"
              :key="log.seq"
              class="finv-secukey-logline"
              :class="{ 'finv-secukey-logline-error': log.level === 'ERROR' }"
            >
              <span class="finv-secukey-logtime">{{ clockText(log.time) }}</span>
              <span class="finv-secukey-logtext">{{ log.text }}</span>
            </div>
            <div v-if="bf.running.value" class="finv-secukey-logline finv-secukey-logline-wait">
              <span class="finv-secukey-logtime">……</span>
              <span class="finv-secukey-logtext">等待下一批…</span>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </div>


    <!-- 说明卡片（ExplainCardSpec：页面最底部唯一「说明」卡片） -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" />
        说明
      </v-card-title>
      <v-card-text>
        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">FT证券基础参数同步</div>
          <div class="text-body-2 finv-secukey-note-body">
            源 <code>public.finv_quote_secu_key</code>（表名可改，见「同步参数」）→ 目标
            <code>futu.finv_futu_secu_key</code>（主键 <code>ftmm_symbol</code>）。同步列：<code>instrument</code>、
            <code>market</code>、<code>code</code>、<code>flag_enable</code>（源 <code>enable</code>）、<code>priority</code>、
            <code>status</code>、<code>url</code>、<code>stock_id</code>、<code>usc</code>——上游补齐这两列后它们不再是本地专有列，
            现在也参与同步；但三者（含 <code>url</code>）都按 <b>COALESCE(源, 本地)</b> 取值，<b>上游为 NULL 时保留本地值、不会清空</b>。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">FT证券指标数据同步</div>
          <div class="text-body-2 finv-secukey-note-body">
            源 <code>public.finv_quote_secu_meta</code>（固定）的每条记录<b>一次写两张表</b>（均为 upsert 增量）：
            <br />
            ① <b>upsert 增量同步到 <code>cdeu.finv_quote_secu_indicator</code></b>：132 列同名同步
            （<code>dt_update</code> 由触发器维护，不参与写入与比对），<b>通过 <code>usc</code> 主键关联</b>；
            <br />
            ② <b>upsert 增量同步到 <code>futu.finv_futu_quote_collect</code></b>：身份字段同名同步
            （<code>stock_id</code>、<code>market_type</code>、<code>market_code</code>、<code>instrument_type</code>、
            <code>sub_instrument_type</code>、<code>market_label</code>、<code>instrument</code>；
            <code>futu_symbol</code> 由 <code>public.finv_quote_secu_meta.stock_code</code>（缺失时用
            <code>ftmm_symbol</code>）去掉市场前缀派生；<code>flag_enable</code> 保持表默认值不覆盖），
            <b>通过 <code>usc</code> 主键关联</b>；
            <br />
            两表主键均为 <code>usc</code>，<b>源记录 <code>usc</code> 为空时用 <code>ftmm_symbol</code> 占位</b>。
            采集属性表有唯一约束 <code>(market_code, futu_symbol)</code>：若该组合已被别的 <code>usc</code> 占用，该行跳过并计入「跳过」。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">本地 usc 回推上游（方向相反：本地 → 上游，回推 usc + priority 两列）</div>
          <div class="text-body-2 finv-secukey-note-body">
            源 <code>futu.finv_futu_secu_key.usc</code>（USC 映射规则 <code>futu.finv_futu_usc_map</code> 的产出，唯一权威）→
            目标 <code>public.finv_quote_secu_key.usc</code> 与 <code>.priority</code>，
            <b>按 <code>ftmm_symbol</code> 关联</b>（本地主键 = 上游主键，一对一），
            <b>勾选的列同一行一次写入</b>（需求 11142 起 priority 跟随 usc 一起同步；11163 起可只勾一列）。<br />
            <b>同步字段可配置</b>（需求 11163）：「回推参数」里勾 <code>usc</code> / <code>priority</code>，默认两个都勾。
            只勾 <code>usc</code> 时上游 <code>priority</code> <b>完全不参与比对、也不会被写</b> ——
            上游若有<b>人工设定</b>的优先级（例如 <code>BJ.899050</code> 上游 29 / 本地 26），不会被这次回推改掉；
            反之只勾 <code>priority</code> 时上游 <code>usc</code> 保持原值。两个都不勾会被拒（400），不算「同步过了」。<br />
            口径不是「只写 NULL 行」：<b>勾选的列都一致</b> → 「未变」且不发请求（幂等，重跑收敛）；
            勾选的列任一为空 → 「填入」；都不为空但有差异 → 「覆盖」；
            <b>上游没有该 <code>ftmm_symbol</code> → 插入新记录</b>（需求 11157，记「上游新增」）：
            按本地行的 <code>instrument</code> / <code>market</code> / <code>code</code> / <code>usc</code> /
            <code>priority</code> / <code>stock_id</code> / <code>status</code> / <code>flag_enable</code>（上游列名 <code>enable</code>）组一行，
            以 <code>on_conflict=ftmm_symbol</code> + <code>resolution=merge-duplicates</code> 的方式 upsert，
            <code>note</code> 列写 <code>[AUTO-FILL]</code> 标记便于溯源；本地缺 <code>instrument</code> / <code>market</code> / <code>code</code>
            的行无法插入，计入「无法插入」。本地 <code>usc</code> 为空的行直接跳过。<br />
            本 Tab 会<b>写入线上 Supabase</b>：已有行走 PATCH（每行校验「受影响行数 = 1」），缺失行走批量 upsert；被 RLS 拒绝会直接报错。
            历史逻辑（写 <code>public.finv_quote_secu_meta.usc</code>、回写 <code>stock_id</code>、按 <code>stock_id</code> 定值/歧义裁决）
            已随需求 11084 整体删除。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">增量口径与进度</div>
          <div class="text-body-2 finv-secukey-note-body">
            Tab 1/2 是分批拉取（按主键 / <code>ftmm_symbol</code> 升序），Tab 3 是分批按 <code>ftmm_symbol</code> 升序：主键不存在则插入；已存在且字段与源不同才更新；
            完全一致的行不改写。点「立即同步」后任务在后台运行，页面每 800 ms 轮询一次，日志逐批实时追加在
            「同步进度与日志」区域，无需等结束；同一时刻每个 Tab 只允许一个运行。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">配置与安全</div>
          <div class="text-body-2 finv-secukey-note-body">
            连接配置在系统配置 <strong>20013</strong>，格式 <code>{PROJECT_ID}|{APIKEY}</code>（本页只展示项目 ID 与脱敏后的
            key 前缀）；Tab 1 的源表名存 <strong>20014</strong>（默认 <code>finv_quote_secu_key</code>）。
            源 <code>status</code> 为空或不在 <code>PENDING/QUEUED/DONE/FAILED/SKIP</code> 之内时统一落 <code>PENDING</code>。
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
/* 实时日志区：等宽、限定高度、内容超出滚动（页面本身不因日志变长而抖动） */
.finv-secukey-logbox {
  max-height: 320px;
  overflow-y: auto;
  padding: 8px 10px;
  border-radius: 6px;
  background-color: rgba(var(--v-theme-on-surface), 0.04);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.78rem;
  line-height: 1.5;
}

.finv-secukey-logline {
  display: flex;
  gap: 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.finv-secukey-logtime {
  flex: 0 0 auto;
  opacity: 0.6;
}

.finv-secukey-logtext {
  flex: 1 1 auto;
}

.finv-secukey-logline-error .finv-secukey-logtext {
  color: rgb(var(--v-theme-error));
}

.finv-secukey-logline-wait .finv-secukey-logtext {
  opacity: 0.55;
}

/* 说明卡片：每段带标题 + 左侧色条（与缺省汇率页保持一致的视觉语言） */
.finv-secukey-note {
  padding-left: 10px;
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
}

.finv-secukey-note-title {
  font-size: 0.85rem;
  font-weight: 600;
  margin-bottom: 2px;
}

.finv-secukey-note-body {
  opacity: 0.85;
}
</style>
