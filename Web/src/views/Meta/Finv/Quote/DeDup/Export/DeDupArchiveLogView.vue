<script setup lang="ts">
// 归档日志与补丁：Job 运行日志、上传审计（每次尝试）、补丁任务（可重试）
// 接口：RunLog/Page、UploadLog/Page、Patch/Page、Patch/Retry
import { onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../../api'
import { fmtBytes, fmtISO, fmtMonth, fmtNum } from '../../../../../../utils'

interface PageData<T> {
  total: number
  list: T[]
}

interface JobDoc {
  job_kind: string
  name: string
  summary: string
  flow: string
  details: string
  cautions: string
  schedule: string
}

interface RunLogRow {
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
}

interface UploadLogRow {
  log_id: number
  file_id: number
  attempt: number
  target_path: string
  gmt_start: string
  gmt_finish: string | null
  status: string
  uploaded_bytes: number | null
  error: string | null
}

interface PatchRow {
  patch_id: number
  job_id: number
  usc: string
  biz_month: number
  patch_level: string
  patch_version: number
  source_rows: number
  patch_rows: number
  reason: string
  status: string
  patch_file_id: number | null
  gmt_create: string
  gmt_finish: string | null
  last_error: string | null
}

const RUN_STATUS_COLOR: Record<string, string> = { SUCCESS: 'success', FAILED: 'error', PAUSED: 'warning' }
const PATCH_STATUS_COLOR: Record<string, string> = { PENDING: 'grey', RUNNING: 'primary', DONE: 'success', FAILED: 'error' }
const PATCH_LEVEL_NAME: Record<string, string> = { L1_HOT: 'L1 热', L2_WARM: 'L2 温', L3_COLD: 'L3 冷' }

const tab = ref('runlog')
const error = ref('')
const toast = ref(false)
const toastText = ref('')

// ---- 运行日志 ----
const runLogs = ref<PageData<RunLogRow>>({ total: 0, list: [] })
const runPage = ref(1)
const runPageSize = ref(20)
const runKind = ref('')
const runStatus = ref('')
const runKeyword = ref('')
const runFrom = ref('')
const runTo = ref('')
const runLoading = ref(false)

// ---- 上传审计 ----
const uploadLogs = ref<PageData<UploadLogRow>>({ total: 0, list: [] })
const uploadPage = ref(1)
const uploadPageSize = ref(20)
const uploadFileID = ref('')
const uploadStatus = ref('')
const uploadKeyword = ref('')
const uploadLoading = ref(false)

// ---- 补丁任务 ----
const patches = ref<PageData<PatchRow>>({ total: 0, list: [] })
const patchPage = ref(1)
const patchPageSize = ref(20)
const patchUsc = ref('')
const patchStatus = ref('')
const patchLoading = ref(false)
const retrying = ref<number | null>(null)

// Job 中文名与说明（来自 /Status 的 jobs，加载一次缓存）
const jobDocs = ref<Map<string, JobDoc>>(new Map())
async function loadJobDocs() {
  try {
    const data = await apiGet<{ jobs: JobDoc[] }>('/Meta/Finv/Quote/DeDup/Status')
    jobDocs.value = new Map(data.jobs.map((job) => [job.job_kind, job]))
  } catch {
    // 名称/说明加载失败时仅显示 Job 标识
  }
}
function jobDoc(kind: string): JobDoc | undefined {
  return jobDocs.value.get(kind)
}
function jobName(kind: string): string {
  return jobDoc(kind)?.name || kind
}

const runHeaders = [
  { title: 'Run ID', key: 'run_id', width: 90, sortable: false },
  { title: 'Job 类型', key: 'job_kind', width: 220, sortable: false },
  { title: '任务 / 证券', key: 'target', width: 150, sortable: false },
  { title: '月份', key: 'biz_month', width: 100, sortable: false },
  { title: '开始时间', key: 'gmt_start', width: 165, sortable: false },
  { title: '结束时间', key: 'gmt_finish', width: 165, sortable: false },
  { title: '状态', key: 'status', width: 100, sortable: false },
  { title: '处理行数', key: 'processed_rows', align: 'end' as const, width: 110, sortable: false },
  { title: '处理字节', key: 'processed_bytes', align: 'end' as const, width: 110, sortable: false },
  { title: '错误 / 详情', key: 'error', sortable: false },
]
const uploadHeaders = [
  { title: 'Log ID', key: 'log_id', width: 90, sortable: false },
  { title: '文件 ID', key: 'file_id', width: 100, sortable: false },
  { title: '尝试', key: 'attempt', align: 'end' as const, width: 80, sortable: false },
  { title: '目标路径', key: 'target_path', sortable: false },
  { title: '开始时间', key: 'gmt_start', width: 165, sortable: false },
  { title: '结束时间', key: 'gmt_finish', width: 165, sortable: false },
  { title: '状态', key: 'status', width: 110, sortable: false },
  { title: '上传字节', key: 'uploaded_bytes', align: 'end' as const, width: 110, sortable: false },
  { title: '错误', key: 'error', sortable: false },
]
const patchHeaders = [
  { title: '补丁 ID', key: 'patch_id', width: 90, sortable: false },
  { title: '任务 ID', key: 'job_id', width: 90, sortable: false },
  { title: '证券[USC]', key: 'usc', width: 110, sortable: false },
  { title: '业务月份', key: 'biz_month', width: 100, sortable: false },
  { title: '级别', key: 'patch_level', width: 100, sortable: false },
  { title: '版本', key: 'patch_version', align: 'end' as const, width: 80, sortable: false },
  { title: '源行数', key: 'source_rows', align: 'end' as const, width: 100, sortable: false },
  { title: '补丁行数', key: 'patch_rows', align: 'end' as const, width: 100, sortable: false },
  { title: '原因', key: 'reason', width: 130, sortable: false },
  { title: '状态', key: 'status', width: 100, sortable: false },
  { title: '创建时间', key: 'gmt_create', width: 165, sortable: false },
  { title: '完成时间', key: 'gmt_finish', width: 165, sortable: false },
  { title: '错误', key: 'last_error', sortable: false },
  { title: '操作', key: 'actions', width: 110, sortable: false },
]

async function loadRunLogs() {
  runLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(runPage.value), pageSize: String(runPageSize.value) })
  if (runKind.value.trim()) params.set('jobKind', runKind.value.trim())
  if (runStatus.value.trim()) params.set('status', runStatus.value.trim())
  if (runKeyword.value.trim()) params.set('keyword', runKeyword.value.trim())
  if (runFrom.value) params.set('dateFrom', runFrom.value)
  if (runTo.value) params.set('dateTo', runTo.value)
  try {
    runLogs.value = await apiGet<PageData<RunLogRow>>(`/Meta/Finv/Quote/DeDup/RunLog/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    runLoading.value = false
  }
}

async function loadUploadLogs() {
  uploadLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(uploadPage.value), pageSize: String(uploadPageSize.value) })
  if (uploadFileID.value.trim()) params.set('fileId', uploadFileID.value.trim())
  if (uploadStatus.value.trim()) params.set('status', uploadStatus.value.trim())
  if (uploadKeyword.value.trim()) params.set('keyword', uploadKeyword.value.trim())
  try {
    uploadLogs.value = await apiGet<PageData<UploadLogRow>>(`/Meta/Finv/Quote/DeDup/UploadLog/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    uploadLoading.value = false
  }
}

async function loadPatches() {
  patchLoading.value = true
  error.value = ''
  const params = new URLSearchParams({ page: String(patchPage.value), pageSize: String(patchPageSize.value) })
  if (patchUsc.value.trim()) params.set('usc', patchUsc.value.trim())
  if (patchStatus.value.trim()) params.set('status', patchStatus.value.trim())
  try {
    patches.value = await apiGet<PageData<PatchRow>>(`/Meta/Finv/Quote/DeDup/Patch/Page?${params.toString()}`)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    patchLoading.value = false
  }
}

async function retryPatch(row: PatchRow) {
  retrying.value = row.patch_id
  error.value = ''
  try {
    await apiPost<{ patch_id: number }>('/Meta/Finv/Quote/DeDup/Patch/Retry', { patch_id: row.patch_id })
    toastText.value = `补丁 ${row.patch_id} 已重置为 PENDING`
    toast.value = true
    await loadPatches()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    retrying.value = null
  }
}

watch([runPage, runPageSize], () => {
  if (tab.value === 'runlog') loadRunLogs()
})
watch([uploadPage, uploadPageSize], () => {
  if (tab.value === 'uploadlog') loadUploadLogs()
})
watch([patchPage, patchPageSize], () => {
  if (tab.value === 'patch') loadPatches()
})

onMounted(async () => {
  await Promise.all([loadRunLogs(), loadUploadLogs(), loadPatches(), loadJobDocs()])
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-text-box-search-outline" class="mr-2" />
      归档日志与补丁
    </v-card-title>

    <v-card-text>
      <v-tabs v-model="tab" density="compact" class="mb-3">
        <v-tab value="runlog">运行日志</v-tab>
        <v-tab value="uploadlog">上传审计</v-tab>
        <v-tab value="patch">补丁任务</v-tab>
      </v-tabs>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-window v-model="tab">
        <!-- 运行日志 -->
        <v-window-item value="runlog">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="runKind" label="Job 类型" placeholder="如 UPLOAD_PENDING_EXPORT_FILES" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="runStatus" label="状态" placeholder="SUCCESS / FAILED / PAUSED" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="runKeyword" label="错误关键字" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="runFrom" type="date" label="起始日期" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="runTo" type="date" label="结束日期" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="runLoading" @click="loadRunLogs">查询</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ runLogs.total }}</strong> 条
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="runPage"
            v-model:items-per-page="runPageSize"
            :headers="runHeaders"
            :items="runLogs.list"
            :items-length="runLogs.total"
            :loading="runLoading"
            item-value="run_id"
            density="compact"
            class="text-no-wrap"
          >
            <template #item.job_kind="{ item }">
              <v-tooltip location="top" max-width="480" open-delay="150">
                <template #activator="{ props }">
                  <div v-bind="props">
                    <div class="text-body-2">{{ jobName(item.job_kind) }}</div>
                    <div class="text-caption text-medium-emphasis">{{ item.job_kind }}</div>
                  </div>
                </template>
                <div class="tooltip-doc text-caption">
                  <div class="font-weight-bold mb-1">{{ jobName(item.job_kind) }}（{{ item.job_kind }}）</div>
                  <div v-if="jobDoc(item.job_kind)?.schedule" class="mb-1">调度：{{ jobDoc(item.job_kind)?.schedule }}</div>
                  <div class="mb-1">功能：{{ jobDoc(item.job_kind)?.summary }}</div>
                  <div class="mb-1">流程：{{ jobDoc(item.job_kind)?.flow }}</div>
                  <div class="mb-1">工作细节：{{ jobDoc(item.job_kind)?.details }}</div>
                  <div>注意事项：{{ jobDoc(item.job_kind)?.cautions }}</div>
                </div>
              </v-tooltip>
            </template>
            <template #item.target="{ item }">
              <span v-if="item.job_id">任务 {{ item.job_id }}</span>
              <span v-else>{{ item.usc ?? '—' }}</span>
            </template>
            <template #item.biz_month="{ item }">{{ item.biz_month === null ? '—' : fmtMonth(item.biz_month) }}</template>
            <template #item.gmt_start="{ item }">{{ fmtISO(item.gmt_start) }}</template>
            <template #item.gmt_finish="{ item }">{{ fmtISO(item.gmt_finish) }}</template>
            <template #item.status="{ item }">
              <v-chip size="small" variant="tonal" :color="RUN_STATUS_COLOR[item.status ?? ''] ?? 'grey'">
                {{ item.status ?? '—' }}
              </v-chip>
            </template>
            <template #item.processed_rows="{ item }">{{ item.processed_rows === null ? '—' : fmtNum(item.processed_rows, 0) }}</template>
            <template #item.processed_bytes="{ item }">{{ item.processed_bytes === null ? '—' : fmtBytes(item.processed_bytes) }}</template>
            <template #item.error="{ item }">
              <span :class="item.error ? 'text-error' : 'text-medium-emphasis'">{{ item.error ?? '—' }}</span>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无运行日志</div></template>
          </v-data-table-server>
        </v-window-item>

        <!-- 上传审计 -->
        <v-window-item value="uploadlog">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="uploadFileID" label="文件 ID" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="uploadStatus" label="状态" placeholder="SUCCESS / FAILED" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="3">
              <v-text-field v-model="uploadKeyword" label="错误关键字" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="uploadLoading" @click="loadUploadLogs">查询</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ uploadLogs.total }}</strong> 条
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="uploadPage"
            v-model:items-per-page="uploadPageSize"
            :headers="uploadHeaders"
            :items="uploadLogs.list"
            :items-length="uploadLogs.total"
            :loading="uploadLoading"
            item-value="log_id"
            density="compact"
            class="text-no-wrap"
          >
            <template #item.gmt_start="{ item }">{{ fmtISO(item.gmt_start) }}</template>
            <template #item.gmt_finish="{ item }">{{ fmtISO(item.gmt_finish) }}</template>
            <template #item.status="{ item }">
              <v-chip size="small" variant="tonal" :color="RUN_STATUS_COLOR[item.status] ?? 'grey'">{{ item.status }}</v-chip>
            </template>
            <template #item.uploaded_bytes="{ item }">{{ item.uploaded_bytes === null ? '—' : fmtBytes(item.uploaded_bytes) }}</template>
            <template #item.error="{ item }">
              <span :class="item.error ? 'text-error' : 'text-medium-emphasis'">{{ item.error ?? '—' }}</span>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无上传审计记录</div></template>
          </v-data-table-server>
        </v-window-item>

        <!-- 补丁任务 -->
        <v-window-item value="patch">
          <v-row dense align="center" class="mb-3">
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="patchUsc" label="证券[USC]" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="12" sm="4" md="2">
              <v-text-field v-model="patchStatus" label="状态" placeholder="PENDING / RUNNING / DONE / FAILED" density="compact" hide-details clearable />
            </v-col>
            <v-col cols="auto">
              <v-btn color="primary" prepend-icon="mdi-magnify" :loading="patchLoading" @click="loadPatches">查询</v-btn>
            </v-col>
            <v-col cols="12" md="auto" class="ml-md-auto text-caption text-medium-emphasis">
              共 <strong>{{ patches.total }}</strong> 条
            </v-col>
          </v-row>
          <v-data-table-server
            v-model:page="patchPage"
            v-model:items-per-page="patchPageSize"
            :headers="patchHeaders"
            :items="patches.list"
            :items-length="patches.total"
            :loading="patchLoading"
            item-value="patch_id"
            density="compact"
            class="text-no-wrap"
          >
            <template #item.biz_month="{ item }">{{ fmtMonth(item.biz_month) }}</template>
            <template #item.patch_level="{ item }">{{ PATCH_LEVEL_NAME[item.patch_level] ?? item.patch_level }}</template>
            <template #item.source_rows="{ item }">{{ fmtNum(item.source_rows, 0) }}</template>
            <template #item.patch_rows="{ item }">{{ fmtNum(item.patch_rows, 0) }}</template>
            <template #item.status="{ item }">
              <v-chip size="small" variant="tonal" :color="PATCH_STATUS_COLOR[item.status] ?? 'grey'">{{ item.status }}</v-chip>
            </template>
            <template #item.gmt_create="{ item }">{{ fmtISO(item.gmt_create) }}</template>
            <template #item.gmt_finish="{ item }">{{ fmtISO(item.gmt_finish) }}</template>
            <template #item.last_error="{ item }">
              <span :class="item.last_error ? 'text-error' : 'text-medium-emphasis'">{{ item.last_error ?? '—' }}</span>
            </template>
            <template #item.actions="{ item }">
              <v-btn
                size="small"
                variant="text"
                color="primary"
                prepend-icon="mdi-refresh"
                :loading="retrying === item.patch_id"
                @click="retryPatch(item)"
              >
                重试
              </v-btn>
            </template>
            <template #no-data><div class="py-6 text-medium-emphasis">暂无补丁任务</div></template>
          </v-data-table-server>
        </v-window-item>
      </v-window>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>数据来源：finv_dedup_job_run_log_export_file、finv_dedup_export_file_upload_log、finv_dedup_export_file_patch（均只读，补丁可重试）。</div>
          <div class="mt-1">管理员在其他页面的编辑操作会以 job_kind=MANUAL_ADMIN_EDIT 记录在「运行日志」中，便于追溯。</div>
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
