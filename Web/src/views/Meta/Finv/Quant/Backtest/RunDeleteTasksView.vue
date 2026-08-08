<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { apiGet, apiPost } from '../../../../../api'
import { fmtDate, statusColor } from '../../../../../utils'

interface DelTaskRow {
  del_task_id: string
  run_id: string
  status: string
  progress: number
  error_message: string
  deleted_counts: Record<string, number>
  created_by: string
  gmt_update: string
}

interface DelLogRow {
  log_id: number
  del_task_id: string
  run_id: string
  seq: number
  action: string
  detail: string
  created_at: string
}

interface RunArchiveRow {
  archive_id: string
  run_id: string
  run_no: number
  strategy_id: string
  strategy_name: string
  account_id: string
  account_name: string
  secu_code: string
  period: string
  start_date: number
  end_date: number
  status: string
  error_message: string
  report_json: string
  deleted_at: string
  deleted_by: string
  del_task_id: string
}

const rows = ref<DelTaskRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const statusFilter = ref('')
const runIdFilter = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')

const logDialog = ref(false)
const logLoading = ref(false)
const logs = ref<DelLogRow[]>([])
const currentTask = ref<DelTaskRow | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

// 已删除任务归档（"曾经存在的证明"）
const archives = ref<RunArchiveRow[]>([])
const archiveTotal = ref(0)
const archivePage = ref(1)
const archivePageSize = ref(10)
const archiveLoading = ref(false)
const archiveDialog = ref(false)
const archiveRunId = ref('')

async function loadArchives() {
  archiveLoading.value = true
  try {
    const params = new URLSearchParams({ page: String(archivePage.value), pageSize: String(archivePageSize.value) })
    if (archiveRunId.value) params.set('runId', archiveRunId.value)
    const data = await apiGet<{ total: number; list: RunArchiveRow[] }>(`/Meta/Finv/Quant/Backtest/Run/DeleteTask/Archives?${params.toString()}`)
    archives.value = data.list ?? []
    archiveTotal.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    archiveLoading.value = false
  }
}

function openArchives() {
  archiveDialog.value = true
  archivePage.value = 1
  archiveRunId.value = ''
  loadArchives()
}

function delStatusColor(s: string): string {
  return { PENDING: 'grey', RUNNING: 'primary', SUCCEEDED: 'success', FAILED: 'error' }[s] ?? 'grey'
}

function delStatusName(s: string): string {
  return { PENDING: '待执行', RUNNING: '执行中', SUCCEEDED: '成功', FAILED: '失败' }[s] ?? s
}

function delActionName(a: string): string {
  return {
    TASK_CREATED: '创建任务',
    DELETING_TABLE: '删除明细表',
    TASK_SUCCEEDED: '删除完成',
    TASK_FAILED: '删除失败',
  }[a] ?? a
}

function fmtDateTime(ts: string | undefined | null): string {
  if (!ts) return '-'
  return ts.replace('T', ' ').slice(0, 19)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
    if (statusFilter.value) params.set('status', statusFilter.value)
    if (runIdFilter.value) params.set('runId', runIdFilter.value)
    const data = await apiGet<{ total: number; list: DelTaskRow[] }>(`/Meta/Finv/Quant/Backtest/Run/DeleteTask/List?${params.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
  syncPolling()
}

function syncPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (rows.value.some((r) => r.status === 'PENDING' || r.status === 'RUNNING')) {
    pollTimer = setInterval(async () => {
      try {
        const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
        if (statusFilter.value) params.set('status', statusFilter.value)
        if (runIdFilter.value) params.set('runId', runIdFilter.value)
        const data = await apiGet<{ total: number; list: DelTaskRow[] }>(`/Meta/Finv/Quant/Backtest/Run/DeleteTask/List?${params.toString()}`)
        rows.value = data.list ?? []
        total.value = data.total ?? 0
        if (!rows.value.some((r) => r.status === 'PENDING' || r.status === 'RUNNING')) syncPolling()
      } catch {
        // 静默
      }
    }, 3000)
  }
}

async function viewLogs(task: DelTaskRow) {
  currentTask.value = task
  logDialog.value = true
  logLoading.value = true
  logs.value = []
  try {
    const data = await apiGet<{ list: DelLogRow[] }>(`/Meta/Finv/Quant/Backtest/Run/DeleteTask/Logs?delTaskId=${task.del_task_id}`)
    logs.value = data.list ?? []
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    logLoading.value = false
  }
}

async function retry(task: DelTaskRow) {
  if (!confirm(`确认重试删除任务 ${task.del_task_id.slice(0, 8)}...？将重新提交删除该回测任务。`)) return
  try {
    const res = await apiPost<{ del_task_id: string }>('/Meta/Finv/Quant/Backtest/Run/DeleteTask/Retry', { del_task_id: task.del_task_id })
    message.value = `已重新提交删除任务（${res.del_task_id}）`
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

onMounted(async () => {
  await load()
})

onBeforeUnmount(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-delete-clock-outline" class="mr-2" color="error" />
        回测任务删除管理（异步批处理 + 审计留痕）
        <v-chip size="small" class="ml-2">共 {{ total }} 个删除任务</v-chip>
        <v-spacer />
        <v-btn size="small" variant="tonal" prepend-icon="mdi-archive-search-outline" class="mr-2"
          @click="openArchives">已删除任务归档</v-btn>
        <v-btn size="small" variant="tonal" prepend-icon="mdi-refresh" @click="load">刷新</v-btn>
      </v-card-title>

      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" dismissible class="mx-4 mb-2">{{ message }}</v-alert>

      <v-card-text class="pt-0">
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-select v-model="statusFilter" :items="[
              { title: '全部状态', value: '' },
              { title: '待执行', value: 'PENDING' },
              { title: '执行中', value: 'RUNNING' },
              { title: '成功', value: 'SUCCEEDED' },
              { title: '失败', value: 'FAILED' },
            ]" density="compact" hide-details label="状态" @update:model-value="load" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="runIdFilter" label="回测任务ID过滤" density="compact" hide-details
              @keyup.enter="load" @blur="load" />
          </v-col>
          <v-col cols="12" sm="6" md="2" class="d-flex align-center">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-filter" @click="load">查询</v-btn>
          </v-col>
        </v-row>
      </v-card-text>

      <v-data-table-server v-model:page="page" v-model:items-per-page="pageSize" :loading="loading"
        :headers="[
          { title: '删除任务ID', key: 'del_task_id', width: 120 },
          { title: '回测任务ID', key: 'run_id', width: 120 },
          { title: '状态', key: 'status', width: 90 },
          { title: '进度', key: 'progress', width: 130 },
          { title: '各表删除行数', key: 'deleted_counts' },
          { title: '错误信息', key: 'error_message', width: 180 },
          { title: '提交人', key: 'created_by', width: 90 },
          { title: '更新时间', key: 'gmt_update', width: 150 },
          { title: '操作', key: 'actions', width: 130, sortable: false },
        ]" :items="rows" :items-length="total" item-value="del_task_id"
        @update:options="load">
        <template #item.del_task_id="{ item }">
          <span class="text-caption font-mono">{{ item.del_task_id.slice(0, 8) }}…</span>
        </template>
        <template #item.run_id="{ item }">
          <span class="text-caption font-mono">{{ item.run_id.slice(0, 8) }}…</span>
        </template>
        <template #item.status="{ item }">
          <v-chip size="small" :color="delStatusColor(item.status)">{{ delStatusName(item.status) }}</v-chip>
        </template>
        <template #item.progress="{ item }">
          <v-progress-linear v-if="item.status === 'RUNNING' || item.status === 'PENDING'"
            :model-value="item.progress" color="primary" height="8" rounded class="mt-2" />
          <span v-else class="text-body-2">{{ item.progress }}%</span>
        </template>
        <template #item.deleted_counts="{ item }">
          <span v-if="item.deleted_counts && Object.keys(item.deleted_counts).length" class="text-caption">
            <v-chip v-for="(cnt, tbl) in item.deleted_counts" :key="tbl" size="x-small" class="mr-1 mb-1">
              {{ tbl.split('_').pop() }}: {{ cnt }}
            </v-chip>
          </span>
          <span v-else class="text-caption text-medium-emphasis">-</span>
        </template>
        <template #item.error_message="{ item }">
          <span v-if="item.error_message" class="text-caption text-error">{{ item.error_message }}</span>
          <span v-else class="text-caption text-medium-emphasis">-</span>
        </template>
        <template #item.gmt_update="{ item }">
          <span class="text-caption">{{ fmtDateTime(item.gmt_update) }}</span>
        </template>
        <template #item.actions="{ item }">
          <v-btn size="small" variant="text" color="primary" @click="viewLogs(item)">日志</v-btn>
          <v-btn v-if="item.status === 'FAILED'" size="small" variant="text" color="warning" @click="retry(item)">重试</v-btn>
        </template>
      </v-data-table-server>
      <v-pagination v-model="page" :length="Math.max(1, Math.ceil(total / pageSize))" density="compact"
        class="mt-2" @update:model-value="load" />
    </v-card>

    <!-- 审计日志弹窗 -->
    <v-dialog v-model="logDialog" max-width="720">
      <v-card>
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-text-box-search-outline" class="mr-2" color="primary" />
          删除任务审计日志
          <v-chip v-if="currentTask" size="small" class="ml-2"
            :color="delStatusColor(currentTask.status)">{{ delStatusName(currentTask.status) }}</v-chip>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" @click="logDialog = false" />
        </v-card-title>
        <v-card-subtitle v-if="currentTask" class="text-caption">
          删除任务 {{ currentTask.del_task_id }} · 回测任务 {{ currentTask.run_id }} · 提交人 {{ currentTask.created_by }}
        </v-card-subtitle>
        <v-card-text :loading="logLoading">
          <v-timeline density="compact" side="end">
            <v-timeline-item v-for="l in logs" :key="l.log_id" size="small"
              :color="l.action === 'TASK_FAILED' ? 'error' : l.action === 'TASK_SUCCEEDED' ? 'success' : 'primary'">
              <div class="text-body-2">
                <v-chip size="x-small" :color="l.action === 'TASK_FAILED' ? 'error' : l.action === 'TASK_SUCCEEDED' ? 'success' : 'primary'" variant="tonal" class="mr-2">
                  {{ delActionName(l.action) }}
                </v-chip>
                <span class="text-caption text-medium-emphasis">{{ fmtDateTime(l.created_at) }}</span>
              </div>
              <div class="text-caption mt-1">{{ l.detail || '-' }}</div>
            </v-timeline-item>
          </v-timeline>
          <div v-if="!logs.length" class="text-center text-medium-emphasis py-4">暂无日志</div>
        </v-card-text>
      </v-card>
    </v-dialog>

    <!-- 已删除任务归档弹窗（"曾经存在的证明"） -->
    <v-dialog v-model="archiveDialog" max-width="1000">
      <v-card>
        <v-card-title class="d-flex align-center">
          <v-icon icon="mdi-archive-outline" class="mr-2" color="orange" />
          已删除任务归档（失败/已结束任务执行记录留痕）
          <v-chip size="small" class="ml-2">共 {{ archiveTotal }} 条</v-chip>
          <v-spacer />
          <v-btn icon="mdi-close" variant="text" @click="archiveDialog = false" />
        </v-card-title>
        <v-card-subtitle class="text-caption">
          任务被删除后，其元信息（任务号/策略/账户/标的/状态/错误信息等）归档于此，作为曾经存在的证明。
        </v-card-subtitle>
        <v-card-text>
          <v-text-field v-model="archiveRunId" label="按回测任务ID过滤" density="compact" hide-details
            class="mb-2" @keyup.enter="archivePage = 1; loadArchives()" @blur="archivePage = 1; loadArchives()" />
          <v-data-table-server v-model:page="archivePage" v-model:items-per-page="archivePageSize"
            :loading="archiveLoading" :headers="[
              { title: '任务号', key: 'run_no', width: 80 },
              { title: '原任务ID', key: 'run_id', width: 120 },
              { title: '策略', key: 'strategy_name' },
              { title: '账户', key: 'account_name', width: 120 },
              { title: '标的', key: 'secu_code', width: 90 },
              { title: '区间', key: 'range', width: 180 },
              { title: '删除前状态', key: 'status', width: 110 },
              { title: '错误信息', key: 'error_message', width: 160 },
              { title: '删除时间', key: 'deleted_at', width: 150 },
            ]" :items="archives" :items-length="archiveTotal" item-value="archive_id"
            @update:options="loadArchives">
            <template #item.run_id="{ item }">
              <span class="text-caption font-mono">{{ item.run_id.slice(0, 8) }}…</span>
            </template>
            <template #item.range="{ item }">
              <span class="text-caption">{{ fmtDate(item.start_date) }} ~ {{ fmtDate(item.end_date) }}</span>
            </template>
            <template #item.status="{ item }">
              <v-chip size="small" :color="statusColor(item.status)">{{ item.status }}</v-chip>
            </template>
            <template #item.error_message="{ item }">
              <span v-if="item.error_message" class="text-caption text-error">{{ item.error_message }}</span>
              <span v-else class="text-caption text-medium-emphasis">-</span>
            </template>
            <template #item.deleted_at="{ item }">
              <span class="text-caption">{{ fmtDateTime(item.deleted_at) }}</span>
            </template>
          </v-data-table-server>
          <v-pagination v-model="archivePage" :length="Math.max(1, Math.ceil(archiveTotal / archivePageSize))"
            density="compact" class="mt-2" @update:model-value="loadArchives" />
        </v-card-text>
      </v-card>
    </v-dialog>
  </v-container>
</template>
