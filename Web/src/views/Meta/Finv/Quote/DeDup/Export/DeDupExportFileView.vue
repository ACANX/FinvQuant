<script setup lang="ts">
// 月归档导出文件与上传：分页查询 cdeu.finv_quote_cdeu_export_file，按上传状态 / 文件类型 / 证券 / 业务月过滤
// 后端接口：GET /Meta/Finv/Quote/DeDup/File/List、POST /Meta/Finv/Quote/DeDup/File/Retry
import { onMounted, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../../api'
import { fmtBytes, fmtISO, fmtMonth, fmtNum } from '../../../../../../utils'

interface FileRow {
  file_id: number
  job_id: number
  usc: string
  biz_month: number
  file_type: string
  version: number
  file_name: string
  file_path: string
  target_file_name: string
  upload_target: string | null
  format: string
  compression: string
  row_count: number
  file_size: number
  checksum: string
  manifest_path: string | null
  seal_date: string | null
  export_status: string
  upload_status: string
  upload_attempt: number
  gmt_next_retry: string | null
  gmt_upload_start: string | null
  gmt_upload_finish: string | null
  remote_checksum_verified: boolean
  network_status: string
  cleanup_status: string
  gmt_create: string
  gmt_update: string
}

interface PageData {
  total: number
  list: FileRow[]
}

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
const FILE_TYPE_NAME: Record<string, string> = {
  BASE: 'BASE 基线',
  PATCH: 'PATCH 补丁',
  MANIFEST: 'MANIFEST 清单',
}
const FILE_TYPE_COLOR: Record<string, string> = {
  BASE: 'primary',
  PATCH: 'warning',
  MANIFEST: 'info',
}
const NETWORK_STATUS_NAME: Record<string, string> = {
  NONE: '未归档',
  ARCHIVED: '已归档',
}
const CLEANUP_STATUS_NAME: Record<string, string> = {
  NONE: '未清理',
  LOCAL_DELETED: '本地已删',
}

const uploadStatusItems = Object.keys(UPLOAD_STATUS_NAME).map((value) => ({
  title: `${value}（${UPLOAD_STATUS_NAME[value]}）`,
  value,
}))
const fileTypeItems = Object.keys(FILE_TYPE_NAME).map((value) => ({
  title: `${value}（${FILE_TYPE_NAME[value]}）`,
  value,
}))

const uploadStatus = ref<string | null>(null)
const fileType = ref<string | null>(null)
const usc = ref('')
const bizMonth = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const rows = ref<FileRow[]>([])
const loading = ref(false)
const error = ref('')
const toast = ref(false)
const toastText = ref('')
const retrying = ref<number | null>(null)

const headers = [
  { title: '文件 ID', key: 'file_id', width: 90, sortable: false },
  { title: '任务 ID', key: 'job_id', width: 90, sortable: false },
  { title: '证券[USC]', key: 'usc', width: 110, sortable: false },
  { title: '业务月份', key: 'biz_month', width: 100, sortable: false },
  { title: '类型', key: 'file_type', width: 130, sortable: false },
  { title: '版本', key: 'version', align: 'end' as const, width: 80, sortable: false },
  { title: '文件名', key: 'file_name', sortable: false },
  { title: '行数', key: 'row_count', align: 'end' as const, width: 100, sortable: false },
  { title: '大小', key: 'file_size', align: 'end' as const, width: 100, sortable: false },
  { title: '上传状态', key: 'upload_status', width: 110, sortable: false },
  { title: '尝试', key: 'upload_attempt', align: 'end' as const, width: 80, sortable: false },
  { title: '上传开始', key: 'gmt_upload_start', width: 160, sortable: false },
  { title: '上传完成', key: 'gmt_upload_finish', width: 160, sortable: false },
  { title: '远端校验', key: 'remote_checksum_verified', width: 100, sortable: false },
  { title: '网盘状态', key: 'network_status', width: 100, sortable: false },
  { title: '清理状态', key: 'cleanup_status', width: 110, sortable: false },
  { title: '下次重试', key: 'gmt_next_retry', width: 160, sortable: false },
  { title: '操作', key: 'actions', width: 120, sortable: false },
]

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
  const uploadText = (uploadStatus.value ?? '').trim()
  const typeText = (fileType.value ?? '').trim()
  const uscText = usc.value.trim()
  if (uploadText) params.set('uploadStatus', uploadText)
  if (typeText) params.set('fileType', typeText)
  if (uscText) params.set('usc', uscText)
  if (month !== null) params.set('bizMonth', String(month))
  try {
    const data = await apiGet<PageData>(`/Meta/Finv/Quote/DeDup/File/List?${params.toString()}`)
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
  uploadStatus.value = null
  fileType.value = null
  usc.value = ''
  bizMonth.value = ''
  search()
}

// 快捷筛选：只查已上传文件（用户关注「已上传文件清单」）
function showUploaded() {
  uploadStatus.value = 'UPLOADED'
  search()
}

async function retry(row: FileRow) {
  retrying.value = row.file_id
  try {
    await apiPost<{ file_id: number }>('/Meta/Finv/Quote/DeDup/File/Retry', { file_id: row.file_id })
    toastText.value = `文件 ${row.file_id} 已重置为 PENDING_UPLOAD`
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
      <v-icon icon="mdi-file-cloud-outline" class="mr-2" />
      月归档导出文件与上传
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-3">
        <v-col cols="12" sm="6" md="2">
          <v-select
            v-model="uploadStatus"
            :items="uploadStatusItems"
            label="上传状态"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="12" sm="6" md="2">
          <v-select
            v-model="fileType"
            :items="fileTypeItems"
            label="文件类型"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="12" sm="6" md="2">
          <v-text-field v-model="usc" label="证券[USC]" placeholder="如 GCMain" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="12" sm="6" md="2">
          <v-text-field v-model="bizMonth" label="业务月份" placeholder="如 202610" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" :loading="loading" @click="search">查询</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="tonal" color="success" prepend-icon="mdi-cloud-check" @click="showUploaded">仅看已上传</v-btn>
        </v-col>
        <v-col cols="12" md="auto" class="ml-md-auto text-medium-emphasis text-caption">
          共 <strong>{{ total }}</strong> 个文件
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
        item-value="file_id"
        density="compact"
        class="text-no-wrap"
      >
        <template #item.biz_month="{ item }">{{ fmtMonth(item.biz_month) }}</template>
        <template #item.file_type="{ item }">
          <v-chip size="small" :color="FILE_TYPE_COLOR[item.file_type] ?? 'grey'" variant="tonal">
            {{ FILE_TYPE_NAME[item.file_type] ?? item.file_type }}
          </v-chip>
        </template>
        <template #item.file_name="{ item }">
          <v-tooltip :text="`本地：${item.file_path}｜远端：${item.upload_target ?? '—'}`" location="top">
            <template #activator="{ props }">
              <span v-bind="props">{{ item.file_name }}</span>
            </template>
          </v-tooltip>
        </template>
        <template #item.row_count="{ item }">{{ fmtNum(item.row_count, 0) }}</template>
        <template #item.file_size="{ item }">{{ fmtBytes(item.file_size) }}</template>
        <template #item.upload_status="{ item }">
          <v-chip size="small" :color="UPLOAD_STATUS_COLOR[item.upload_status] ?? 'grey'" variant="tonal">
            {{ UPLOAD_STATUS_NAME[item.upload_status] ?? item.upload_status }}
          </v-chip>
        </template>
        <template #item.gmt_upload_start="{ item }">{{ fmtISO(item.gmt_upload_start) }}</template>
        <template #item.gmt_upload_finish="{ item }">{{ fmtISO(item.gmt_upload_finish) }}</template>
        <template #item.remote_checksum_verified="{ item }">
          <v-icon
            :icon="item.remote_checksum_verified ? 'mdi-check-circle' : 'mdi-circle-outline'"
            :color="item.remote_checksum_verified ? 'success' : 'grey'"
            size="18"
          />
        </template>
        <template #item.network_status="{ item }">
          <v-chip
            size="small"
            :color="item.network_status === 'ARCHIVED' ? 'success' : 'grey'"
            variant="tonal"
          >
            {{ NETWORK_STATUS_NAME[item.network_status] ?? item.network_status }}
          </v-chip>
        </template>
        <template #item.cleanup_status="{ item }">
          {{ CLEANUP_STATUS_NAME[item.cleanup_status] ?? item.cleanup_status }}
        </template>
        <template #item.gmt_next_retry="{ item }">{{ fmtISO(item.gmt_next_retry) }}</template>
        <template #item.actions="{ item }">
          <v-btn
            size="small"
            variant="text"
            color="primary"
            prepend-icon="mdi-cloud-upload-outline"
            :loading="retrying === item.file_id"
            @click="retry(item)"
          >
            重试上传
          </v-btn>
        </template>
        <template #no-data>
          <div class="py-6 text-medium-emphasis">当前条件下没有导出文件</div>
        </template>
      </v-data-table-server>

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>数据来源：cdeu.finv_quote_cdeu_export_file；接口 GET /API/V1/Meta/Finv/Quote/DeDup/File/List。</div>
          <div class="mt-1">
            「仅看已上传」等价于 uploadStatus=UPLOADED，用于查看已上传文件清单；「重试上传」调用 POST /File/Retry，
            把文件重置为 PENDING_UPLOAD。
          </div>
        </v-card-text>
      </v-card>
    </v-card-text>

    <v-snackbar v-model="toast" :timeout="3000" color="success">{{ toastText }}</v-snackbar>
  </v-card>
</template>
