<script setup lang="ts">
// BRS 手动备份与实时日志
// 接口：POST /Meta/BRS/Backup/Finv/Run/Manual          手动触发一轮备份（异步，返回运行快照）
//       GET  /Meta/BRS/Backup/Finv/Run/Status?sinceSeq 查询状态；sinceSeq 为增量拉取起点
// 轮询：运行中 1s 一次，空闲 5s 一次；run_id 变化时清空日志重新累积。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

interface LogEntry {
  seq: number
  time: string
  level: 'info' | 'warn' | 'error' | string
  message: string
}

interface RunState {
  run_id: string
  trigger: string
  status: 'idle' | 'running' | 'succeeded' | 'failed' | string
  started_at: string | null
  finished_at: string | null
  elapsed_ms: number
  total: number
  done: number
  failed: number
  current_table: string
  backup_dir: string
  disk_path: string
  disk_free_bytes: number
  disk_total_bytes: number
  disk_min_free_bytes: number
  disk_low: boolean
  disk_error: string
  error: string
  logs: LogEntry[]
  last_seq: number
}

const API_PREFIX = '/Meta/BRS/Backup/Finv/Run'
const POLL_RUNNING_MS = 1000
const POLL_IDLE_MS = 5000

const state = ref<RunState>({
  run_id: '',
  trigger: '',
  status: 'idle',
  started_at: null,
  finished_at: null,
  elapsed_ms: 0,
  total: 0,
  done: 0,
  failed: 0,
  current_table: '',
  backup_dir: '',
  disk_path: '',
  disk_free_bytes: 0,
  disk_total_bytes: 0,
  disk_min_free_bytes: 0,
  disk_low: false,
  disk_error: '',
  error: '',
  logs: [],
  last_seq: 0,
})

const loading = ref(false)
const triggering = ref(false)
const autoScroll = ref(true)

let timer: number | undefined
let currentRunId = ''

const isRunning = computed(() => state.value.status === 'running')
const progress = computed(() => {
  if (state.value.total <= 0) return 0
  return Math.min(100, Math.round((state.value.done / state.value.total) * 100))
})
const statusText = computed(() => {
  switch (state.value.status) {
    case 'running':
      return '运行中'
    case 'succeeded':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return '空闲'
  }
})
const statusColor = computed(() => {
  switch (state.value.status) {
    case 'running':
      return 'primary'
    case 'succeeded':
      return 'success'
    case 'failed':
      return 'error'
    default:
      return 'grey'
  }
})
const triggerText = computed(() => (state.value.trigger === 'manual' ? '手动' : state.value.trigger === 'auto' ? '定时' : '-'))

async function fetchStatus(sinceSeq = 0) {
  const suffix = sinceSeq > 0 ? `?sinceSeq=${sinceSeq}` : ''
  const data = await apiGet<RunState>(`${API_PREFIX}/Status${suffix}`)
  if (data.run_id !== currentRunId) {
    // 新的一轮运行：清空旧日志重新累积
    currentRunId = data.run_id
    state.value = { ...data, logs: [...data.logs] }
  } else if (sinceSeq > 0) {
    state.value = { ...data, logs: [...state.value.logs, ...data.logs] }
  } else {
    state.value = { ...data, logs: [...data.logs] }
  }
  scrollToBottom()
}

function scrollToBottom() {
  if (!autoScroll.value) return
  requestAnimationFrame(() => {
    const box = document.getElementById('brs-run-log')
    if (box) box.scrollTop = box.scrollHeight
  })
}

async function poll() {
  try {
    await fetchStatus(state.value.last_seq)
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    schedule()
  }
}

function schedule() {
  window.clearTimeout(timer)
  timer = window.setTimeout(poll, isRunning.value ? POLL_RUNNING_MS : POLL_IDLE_MS)
}

async function triggerManual() {
  triggering.value = true
  try {
    const data = await apiPost<RunState>(`${API_PREFIX}/Manual`, {})
    currentRunId = ''
    state.value = { ...data, logs: [...data.logs] }
    showToast('已开始手动备份')
    await fetchStatus(0)
    schedule()
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    triggering.value = false
  }
}

function formatBytes(v: number): string {
  if (!v || v <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = v
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index++
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function formatTime(v: string | null): string {
  if (!v) return '-'
  return v.replace('T', ' ').slice(0, 19)
}

function logTime(v: string): string {
  if (!v) return ''
  return v.replace('T', ' ').slice(11, 19)
}

function logClass(level: string): string {
  if (level === 'error') return 'log-error'
  if (level === 'warn') return 'log-warn'
  return 'log-info'
}

onMounted(async () => {
  loading.value = true
  try {
    await fetchStatus(0)
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    loading.value = false
    schedule()
  }
})

onBeforeUnmount(() => window.clearTimeout(timer))
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-database-export" class="mr-2" />
      小表全量备份
    </v-card-title>
    <v-card-subtitle>
      手动触发一轮小表全量备份（与定时任务共用同一执行链），实时查看进度与运行日志；备份范围取「备份登记维护」中启用的表。大表按分片增量备份见「大表增量备份」。
    </v-card-subtitle>

    <v-card-text>
      <v-alert v-if="state.disk_low" type="warning" class="mb-3" density="compact">
        磁盘剩余空间偏低：<b>{{ formatBytes(state.disk_free_bytes) }}</b> / 总 {{ formatBytes(state.disk_total_bytes) }}，
        低于阈值 {{ formatBytes(state.disk_min_free_bytes) }}（{{ state.disk_path }}）。
        请清理旧备份或扩容（本地保留天数见 <code>FINV_BRS_LOCAL_KEEP_DAYS</code>）。
      </v-alert>
      <v-alert v-if="state.disk_error" type="info" class="mb-3" density="compact">{{ state.disk_error }}</v-alert>

      <div class="d-flex align-center flex-wrap ga-2 mb-3">
        <v-btn
          color="primary"
          prepend-icon="mdi-database-export"
          :loading="triggering"
          :disabled="isRunning"
          @click="triggerManual"
        >
          立即备份
        </v-btn>
        <v-chip :color="statusColor" size="small">{{ statusText }}</v-chip>
        <v-chip v-if="state.trigger" size="small" variant="tonal">来源：{{ triggerText }}</v-chip>
        <v-switch v-model="autoScroll" label="日志自动滚动" density="compact" hide-details class="ml-4" />
      </div>

      <v-progress-linear
        v-if="isRunning || state.total > 0"
        :model-value="progress"
        :color="state.status === 'failed' ? 'error' : progress === 100 ? 'success' : 'primary'"
        height="18"
        rounded
        class="mb-2"
      >
        <template #default>
          <span class="text-caption">{{ state.done }} / {{ state.total }} 张表（{{ progress }}%）</span>
        </template>
      </v-progress-linear>

      <v-row dense class="text-body-2">
        <v-col cols="12" sm="6" md="3">
          <span class="text-medium-emphasis">运行 ID：</span>{{ state.run_id || '-' }}
        </v-col>
        <v-col cols="12" sm="6" md="3">
          <span class="text-medium-emphasis">开始：</span>{{ formatTime(state.started_at) }}
        </v-col>
        <v-col cols="12" sm="6" md="3">
          <span class="text-medium-emphasis">结束：</span>{{ formatTime(state.finished_at) }}
        </v-col>
        <v-col cols="12" sm="6" md="3">
          <span class="text-medium-emphasis">耗时：</span>
          {{ state.elapsed_ms > 0 ? (state.elapsed_ms / 1000).toFixed(1) + 's' : '-' }}
        </v-col>
        <v-col v-if="state.current_table" cols="12">
          <span class="text-medium-emphasis">当前表：</span><code>{{ state.current_table }}</code>
        </v-col>
        <v-col v-if="state.failed > 0" cols="12" class="text-error">
          失败张数：{{ state.failed }}
        </v-col>
        <v-col v-if="state.error" cols="12" class="text-error">错误：{{ state.error }}</v-col>
        <v-col cols="12"><span class="text-medium-emphasis">备份目录：</span>{{ state.backup_dir || '-' }}</v-col>
        <v-col cols="12">
          <span class="text-medium-emphasis">磁盘剩余：</span>
          <span :class="state.disk_low ? 'text-warning' : ''">{{ formatBytes(state.disk_free_bytes) }}</span>
          <span class="text-medium-emphasis"> / 总 {{ formatBytes(state.disk_total_bytes) }}（阈值 {{ formatBytes(state.disk_min_free_bytes) }}）</span>
        </v-col>
      </v-row>

      <div class="text-subtitle-2 mt-4 mb-1">运行日志</div>
      <div id="brs-run-log" class="log-box">
        <div v-if="state.logs.length === 0" class="text-medium-emphasis">暂无日志（尚未执行过备份）</div>
        <div v-for="e in state.logs" :key="e.seq" :class="logClass(e.level)">
          <span class="log-time">{{ logTime(e.time) }}</span>
          <span class="log-level">[{{ e.level }}]</span>
          {{ e.message }}
        </div>
      </div>
    </v-card-text>
  </v-card>

  <!-- 配置说明：标签/内容两列对齐，无冒号 -->
  <v-card class="mt-4" variant="flat">
    <v-card-title class="text-subtitle-2">配置说明</v-card-title>
    <v-card-text class="text-body-2 text-medium-emphasis">
      <v-row dense>
        <v-col cols="4" sm="3">触发方式</v-col>
        <v-col cols="8" sm="9">手动点击「立即备份」；或由「备份调度配置」的 39001 cron 定时触发</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">并发限制</v-col>
        <v-col cols="8" sm="9">同一时刻只允许一个运行，已有任务在跑时再次触发返回冲突提示</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">进度与日志</v-col>
        <v-col cols="8" sm="9">运行中每 1 秒增量拉取一次（<code>sinceSeq</code>），空闲时每 5 秒；日志最多保留 1000 条</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">状态存储</v-col>
        <v-col cols="8" sm="9">运行状态与日志仅保存在服务内存，服务重启后清空；备份产物仍落磁盘与对象存储</v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.log-box {
  max-height: 420px;
  overflow-y: auto;
  padding: 8px 10px;
  border-radius: 4px;
  background: rgba(var(--v-theme-surface-variant), 0.4);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.78rem;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-time {
  color: rgb(var(--v-theme-on-surface));
  opacity: 0.55;
  margin-right: 6px;
}
.log-level {
  margin-right: 6px;
}
.log-info {
  color: rgb(var(--v-theme-on-surface));
}
.log-warn {
  color: rgb(var(--v-theme-warning));
}
.log-error {
  color: rgb(var(--v-theme-error));
}
</style>
