<script setup lang="ts">
// 证券参数同步（需求 11042/11043/11044；11045 拆两个 Tab；11046 实时日志）
// Tab 1「FT证券基础参数同步」接口：
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Status        只读快照（20013 是否配置 + 源表名 + 目标表行数）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Config        保存源表名（写系统配置 20014）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run      异步启动（返回 run_id）
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run      运行时快照（进度 + 逐批实时日志，本页每 800ms 轮询）
// Tab 2「证券指标数据同步」：口径待确认（来源表 / 目标表 / 字段映射 / 增量键），当前为占位。
//
// 数据流：Supabase PostgREST（源表，默认 public.finv_quote_secu_key）
//        → 本地 futu.finv_futu_secu_key（主键 ftmm_symbol，分批增量 upsert）
// 连接配置（PROJECT_ID|APIKEY）在系统配置 20013 里维护，本页只展示是否已配置。
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 同步状态快照（GET Status 的 data）
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

// 一条实时日志
interface RunLog {
  seq: number
  time: string
  level: string
  text: string
  batch?: number
  offset?: number
}

// 异步运行快照（POST/GET Sync/Run 的 data）
interface SyncRun {
  run_id: string
  status: string // RUNNING / DONE / FAILED
  source_table: string
  batch_size: number
  rows_total: number
  rows_receive: number
  insert: number
  update: number
  unchanged: number
  skipped: number
  status_reset: number
  batches: number
  progress: number
  started_at: string
  finished_at?: string
  elapsed_ms: number
  error?: string
  logs: RunLog[]
}

const STATUS_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Status'
const CONFIG_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Config'
const SYNC_RUN_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Sync/Run'

// 批量大小候选（默认 1000；7 万行量级约需 70+ 批）
const BATCH_OPTIONS = [500, 1000, 2000, 5000]
// 源表名默认值（与系统配置 20014 的种子一致）
const DEFAULT_SOURCE_TABLE = 'finv_quote_secu_key'
// 启动请求超时（只启动，不等待同步完成，给 60s 足够）
const START_TIMEOUT_MS = 60 * 1000
// 轮询间隔
const POLL_INTERVAL_MS = 800

// Tab 2 待确认的口径项（占位页展示，便于评审时一次问全）
const INDICATOR_PENDING_ITEMS = [
  '来源：Supabase 表名（已探测到候选 public.finv_quote_secu_meta，10,910 行，含 price/volume/turnover/PE/PB/市值/52 周高低等指标字段）',
  '目标：本地表名与是否需要新建（现有 cdeu.finv_cdeu_secu_realtime_stock 只覆盖部分行情字段）',
  '字段映射：哪些指标列入库、类型与精度（金额/比率分开处理）',
  '增量键：按 ftmm_symbol 还是 stock_id/usc 对齐，以及「变了才更新」的判据列',
]

const tab = ref<'basic' | 'indicator'>('basic')

const status = ref<SecuKeyStatus | null>(null)
const statusLoading = ref(false)
const sourceTable = ref(DEFAULT_SOURCE_TABLE)
const batchSize = ref(1000)
const saving = ref(false)
const starting = ref(false)
const run = ref<SyncRun | null>(null)
const errorMessage = ref('')
const logBox = ref<HTMLElement | null>(null)

// 轮询定时器（组件卸载时清理）
let pollTimer: number | null = null

const configured = computed(() => status.value?.configured === true)
const running = computed(() => run.value?.status === 'RUNNING')
const sourceTableChanged = computed(() => {
  const current = (status.value?.source_table || DEFAULT_SOURCE_TABLE).trim()
  return sourceTable.value.trim() !== current
})
const progressValue = computed(() => {
  const value = run.value?.progress ?? 0
  return value < 0 ? 0 : value > 100 ? 100 : value
})

// 数字千分位（日志/统计里便于读数）
function num(value: number | undefined | null): string {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString('en-US')
}

// RFC3339 → 本地时间（空的用 —）
function timeText(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}:${pad(date.getSeconds())}`
}

// 日志行的时间只显示时分秒
function clockText(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--:--:--'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

// 毫秒 → 1.2s 形式
function elapsedText(ms: number | undefined): string {
  if (!ms || ms < 0) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

// 读取状态快照
async function loadStatus() {
  statusLoading.value = true
  try {
    const data = await apiGet<SecuKeyStatus>(STATUS_API)
    status.value = data
    sourceTable.value = data.source_table || DEFAULT_SOURCE_TABLE
    if (data.batch_size_default > 0) {
      const allowed = BATCH_OPTIONS.includes(data.batch_size_default)
      batchSize.value = allowed ? data.batch_size_default : 1000
    }
    errorMessage.value = ''
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : String(err)
    showToast(errorMessage.value, 'error')
  } finally {
    statusLoading.value = false
  }
}

// 保存源表名（写系统配置 20014）
async function saveSourceTable() {
  saving.value = true
  try {
    const data = await apiPost<{ source_table: string }>(CONFIG_API, {
      source_table: sourceTable.value.trim(),
    })
    sourceTable.value = data.source_table
    showToast(`源表名已保存：${data.source_table}`, 'success')
    await loadStatus()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    errorMessage.value = message
    showToast(message, 'error')
  } finally {
    saving.value = false
  }
}

// 日志区自动滚到底部（同步过程中始终能看到最新一行）
async function scrollLogsToBottom() {
  await nextTick()
  const box = logBox.value
  if (box) box.scrollTop = box.scrollHeight
}

// 应用一次运行快照
async function applyRun(snapshot: SyncRun) {
  run.value = snapshot
  await scrollLogsToBottom()
}

// 轮询运行快照（RUNNING 时每 800ms 一次，结束后停止）
function startPolling(runId: string) {
  stopPolling()
  pollTimer = window.setInterval(async () => {
    try {
      const snapshot = await apiGet<SyncRun>(`${SYNC_RUN_API}?runId=${encodeURIComponent(runId)}`)
      await applyRun(snapshot)
      if (snapshot.status !== 'RUNNING') {
        stopPolling()
        starting.value = false
        if (snapshot.status === 'DONE') {
          showToast(
            `同步完成：新增 ${num(snapshot.insert)}、更新 ${num(snapshot.update)}、未变 ${num(snapshot.unchanged)}`,
            'success',
          )
        } else {
          showToast(`同步失败：${snapshot.error || '未知错误'}`, 'error')
        }
        await loadStatus()
      }
    } catch (err) {
      stopPolling()
      starting.value = false
      const message = err instanceof Error ? err.message : String(err)
      errorMessage.value = message
      showToast(message, 'error')
    }
  }, POLL_INTERVAL_MS)
}

function stopPolling() {
  if (pollTimer !== null) {
    window.clearInterval(pollTimer)
    pollTimer = null
  }
}

// 立即同步：先异步启动（拿 run_id + 首条日志），再轮询实时进度
async function runSync() {
  if (!configured.value) {
    showToast('系统配置 20013（Supabase 连接配置）未配置，请先填写 {PROJECT_ID}|{APIKEY}', 'warning')
    return
  }
  starting.value = true
  errorMessage.value = ''
  stopPolling()
  try {
    const snapshot = await apiPost<SyncRun>(
      SYNC_RUN_API,
      { source_table: sourceTable.value.trim(), batch_size: batchSize.value },
      START_TIMEOUT_MS,
    )
    await applyRun(snapshot)
    startPolling(snapshot.run_id)
  } catch (err) {
    starting.value = false
    const message = err instanceof Error ? err.message : String(err)
    errorMessage.value = message
    showToast(message, 'error')
  }
}

onMounted(loadStatus)
onBeforeUnmount(stopPolling)
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
        :loading="statusLoading"
        @click="loadStatus"
      >
        刷新状态
      </v-btn>
    </div>
    <div class="text-caption text-medium-emphasis mb-2">
      证券参数与指标数据的上游同步入口：按主键分批增量写入本地表，只在上游有变化时更新。
    </div>

    <!-- 两个 Tab（需求 11045）：基础参数同步 / 指标数据同步 -->
    <v-tabs v-model="tab" density="comfortable" color="primary">
      <v-tab value="basic">
        <v-icon icon="mdi-key-chain" class="mr-2" />
        FT证券基础参数同步
      </v-tab>
      <v-tab value="indicator">
        <v-icon icon="mdi-chart-box-outline" class="mr-2" />
        证券指标数据同步
      </v-tab>
    </v-tabs>

    <!-- ============ Tab 1：FT证券基础参数同步 ============ -->
    <div v-if="tab === 'basic'">
      <!-- 同步状态 -->
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
              <div v-if="status?.config_error" class="text-caption text-warning mt-1">
                {{ status.config_error }}
              </div>
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

      <!-- 同步参数 -->
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
                :disabled="running || !sourceTableChanged"
                @click="saveSourceTable"
              >
                保存源表名
              </v-btn>
              <v-btn
                color="primary"
                prepend-icon="mdi-play-circle-outline"
                :loading="starting && running"
                :disabled="!configured || running"
                @click="runSync"
              >
                {{ running ? '同步中…' : '立即同步' }}
              </v-btn>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>

      <!-- 错误提示 -->
      <v-alert v-if="errorMessage" type="error" variant="tonal" class="mt-4" density="compact">
        {{ errorMessage }}
      </v-alert>

      <!-- 运行进度 + 实时日志（需求 11046：单独区域，边跑边看） -->
      <v-card v-if="run" class="mt-4">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-progress-clock" class="mr-2" />
          同步进度与日志
          <v-spacer />
          <v-chip
            size="small"
            variant="tonal"
            :color="run.status === 'RUNNING' ? 'info' : run.status === 'DONE' ? 'success' : 'error'"
          >
            {{ run.status === 'RUNNING' ? '运行中' : run.status === 'DONE' ? '已完成' : '失败' }}
          </v-chip>
        </v-card-title>
        <v-card-text>
          <div class="d-flex flex-wrap ga-2 mb-2">
            <v-chip size="small" variant="tonal" color="info">拉取 {{ num(run.rows_receive) }}</v-chip>
            <v-chip size="small" variant="tonal" color="success">新增 {{ num(run.insert) }}</v-chip>
            <v-chip size="small" variant="tonal" color="primary">更新 {{ num(run.update) }}</v-chip>
            <v-chip size="small" variant="tonal">未变 {{ num(run.unchanged) }}</v-chip>
            <v-chip v-if="run.skipped" size="small" variant="tonal" color="warning">跳过 {{ num(run.skipped) }}</v-chip>
            <v-chip v-if="run.status_reset" size="small" variant="tonal" color="warning">
              status 归一 {{ num(run.status_reset) }}
            </v-chip>
            <v-chip size="small" variant="text">
              {{ run.batches }} 批 · {{ elapsedText(run.elapsed_ms) }}
            </v-chip>
          </div>

          <v-progress-linear
            :model-value="progressValue"
            :indeterminate="running && progressValue === 0"
            height="8"
            rounded
            class="mb-1"
          />
          <div class="text-caption text-medium-emphasis mb-2">
            进度 {{ progressValue.toFixed(1) }}%（{{ num(run.rows_receive) }} / {{ num(run.rows_total) }} 行）·
            源表 <code>{{ run.source_table || sourceTable }}</code> · run_id <code>{{ run.run_id }}</code>
          </div>

          <!-- 实时日志区：单独区域、自动滚到底 -->
          <div ref="logBox" class="finv-secukey-logbox">
            <div
              v-for="log in run.logs"
              :key="log.seq"
              class="finv-secukey-logline"
              :class="{ 'finv-secukey-logline-error': log.level === 'ERROR' }"
            >
              <span class="finv-secukey-logtime">{{ clockText(log.time) }}</span>
              <span class="finv-secukey-logtext">{{ log.text }}</span>
            </div>
            <div v-if="running" class="finv-secukey-logline finv-secukey-logline-wait">
              <span class="finv-secukey-logtime">……</span>
              <span class="finv-secukey-logtext">等待下一批…</span>
            </div>
          </div>
        </v-card-text>
      </v-card>
    </div>

    <!-- ============ Tab 2：证券指标数据同步（口径待确认） ============ -->
    <div v-else>
      <v-card class="mt-3">
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-chart-box-outline" class="mr-2" />
          证券指标数据同步
          <v-spacer />
          <v-chip size="small" variant="tonal" color="warning">待接入</v-chip>
        </v-card-title>
        <v-card-text>
          <div class="text-body-2 mb-3">
            本 Tab 预留给「证券指标数据」的上游同步（如价格、成交量、成交额、PE/PB、市值、52 周高低等指标）。
            <strong>同步口径尚未确认，功能未接入</strong>：按钮置灰，避免误触发写入。
          </div>

          <v-alert type="info" variant="tonal" density="compact" class="mb-3">
            已探测到的上游候选表：<code>public.finv_quote_secu_meta</code>（10,910 行，
            含 <code>price</code>/<code>volume</code>/<code>turnover</code>/<code>pe_ttm</code>/<code>pb_ratio</code>/
            <code>total_market_cap</code>/<code>price_highest_week52</code> 等指标字段，
            主键 <code>ftmm_symbol</code>，另有 <code>stock_id</code>/<code>usc</code>）。
          </v-alert>

          <div class="finv-secukey-note-title mb-1">需要确认的四项</div>
          <ol class="text-body-2 finv-secukey-pending-list">
            <li v-for="item in INDICATOR_PENDING_ITEMS" :key="item">{{ item }}</li>
          </ol>

          <v-btn class="mt-3" color="primary" prepend-icon="mdi-play-circle-outline" disabled>
            立即同步
          </v-btn>
        </v-card-text>
      </v-card>
    </div>

    <!-- 说明卡片（ExplainCardSpec：静态说明统一放页面最底部唯一「说明」卡片） -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" />
        说明
      </v-card-title>
      <v-card-text>
        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">表口径（FT证券基础参数同步）</div>
          <div class="text-body-2 finv-secukey-note-body">
            源：Supabase <code>public.finv_quote_secu_key</code>（表名可改，见「同步参数」）；
            目标：<code>futu.finv_futu_secu_key</code>（主键 <code>ftmm_symbol</code>）。
            同步的列：<code>instrument</code>、<code>market</code>、<code>code</code>、<code>flag_enable</code>（源
            <code>enable</code>）、<code>priority</code>、<code>status</code>、<code>url</code>；
            本地专有列 <code>stock_id</code> / <code>usc</code> 不参与同步，也不会被清空。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">增量口径与进度</div>
          <div class="text-body-2 finv-secukey-note-body">
            按 <code>ftmm_symbol</code> 升序分批拉取：主键不存在则插入；已存在且上述字段与源不同才更新；
            完全一致的行不改写（不会让 <code>gmt_update</code> 前进）。因此「新增 + 更新」才是本次真正写入的行数，
            「未变」是比对后跳过的行数。同步只读源表，不删除本地行，也不改动历史数据。
            点「立即同步」后任务在后台运行，页面每 800 ms 轮询一次进度，<strong>日志逐批实时追加</strong>在
            「同步进度与日志」区域，无需等结束。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">配置与安全</div>
          <div class="text-body-2 finv-secukey-note-body">
            连接配置在系统配置 <strong>20013</strong>，格式 <code>{PROJECT_ID}|{APIKEY}</code>（本页只展示项目 ID 与脱敏后的
            key 前缀，不回显完整 key）；源表名存系统配置 <strong>20014</strong>（默认
            <code>finv_quote_secu_key</code>），在本页改完需点「保存源表名」才生效。
            源 <code>status</code> 为空或不在 <code>PENDING/QUEUED/DONE/FAILED/SKIP</code> 之内时统一落
            <code>PENDING</code>（计入「status 归一」）。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">证券指标数据同步（待接入）</div>
          <div class="text-body-2 finv-secukey-note-body">
            该 Tab 目前只是占位：来源表、目标表（是否新建）、字段映射与增量键四项口径确认后再实现，
            实现方式与「FT证券基础参数同步」一致（分批拉取 + 增量 upsert + 实时日志）。
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

.finv-secukey-pending-list {
  padding-left: 20px;
  opacity: 0.85;
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
