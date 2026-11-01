<script setup lang="ts">
// BRS 大表增量（分片）备份
// 接口：
//   GET  /Meta/BRS/Backup/Finv/Slice/Rule/List           登记列表 + 每张表的分片完成度
//   POST /Meta/BRS/Backup/Finv/Slice/Rule/Save           新增/编辑登记
//   POST /Meta/BRS/Backup/Finv/Slice/Rule/Toggle         启用/停用
//   GET  /Meta/BRS/Backup/Finv/Slice/Plan?ruleId          单个登记的分片明细
//   POST /Meta/BRS/Backup/Finv/Slice/Run/Manual          手动触发一轮（异步）
//   GET  /Meta/BRS/Backup/Finv/Slice/Run/Status?sinceSeq 运行状态与日志
// 与小表全量备份（菜单「小表全量备份」）完全独立：各自一套登记、执行链与运行状态。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'

interface LogEntry {
  seq: number
  time: string
  level: 'info' | 'warn' | 'error' | string
  message: string
}

interface SlicePlanItem {
  key: string
  rows: number
  backuped: boolean
}

interface SlicePlan {
  rule_id: number
  table: string
  total: number
  done: number
  missing: number
  skipped: number
  redo_from: number
  items: SlicePlanItem[]
}

interface SliceRule {
  idn: number
  rdb_type: string
  db: string
  schema_name: string
  table_name: string
  flag_enable: string
  priority: number
  slice_mode: string
  slice_key_sql: string | null
  slice_where_tpl: string
  slice_file_tpl: string
  bak_scope: string
  compression: string
  min_rows: number
  max_slices: number
  redo_slices: number
  flag_upload: string
  remark: string | null
  creator: string | null
  gmt_create: string | null
  gmt_update: string | null
  plan: SlicePlan
}

interface SliceRunState {
  run_id: string
  trigger: string
  status: 'idle' | 'running' | 'succeeded' | 'failed' | string
  started_at: string | null
  finished_at: string | null
  elapsed_ms: number
  current_table: string
  current_slice: string
  total: number
  done: number
  failed: number
  backup_dir: string
  backup_dir_ok: boolean
  backup_dir_error: string
  error: string
  logs: LogEntry[]
  last_seq: number
}

const API_PREFIX = '/Meta/BRS/Backup/Finv/Slice'
const POLL_RUNNING_MS = 1000
const POLL_IDLE_MS = 5000

const rules = ref<SliceRule[]>([])
const loading = ref(false)
const saving = ref(false)
const triggering = ref(false)
const error = ref('')
const message = ref('')
const autoScroll = ref(true)

const dialog = ref(false)
const form = ref<SliceRule>(emptyRule())

// 分片明细（调用 Slice/Plan，展示每个分片的键、行数与是否已备份）
const planDialog = ref(false)
const planLoading = ref(false)
const planDetail = ref<SlicePlan | null>(null)
const planOnlyMissing = ref(false)

const state = ref<SliceRunState>({
  run_id: '',
  trigger: '',
  status: 'idle',
  started_at: null,
  finished_at: null,
  elapsed_ms: 0,
  current_table: '',
  current_slice: '',
  total: 0,
  done: 0,
  failed: 0,
  backup_dir: '',
  backup_dir_ok: true,
  backup_dir_error: '',
  error: '',
  logs: [],
  last_seq: 0,
})

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

const modeItems = [
  { title: '按自然日（date）', value: 'date' },
  { title: '按 ISO 周（week）', value: 'week' },
  { title: '按自然月（month）', value: 'month' },
  { title: '按列值（column）', value: 'column' },
  { title: '自定义 SQL（expr）', value: 'expr' },
]

function emptyRule(): SliceRule {
  return {
    idn: 0,
    rdb_type: 'postgresql',
    db: 'finv',
    schema_name: 'cdeu',
    table_name: '',
    flag_enable: '1',
    priority: 500,
    slice_mode: 'date',
    slice_key_sql: null,
    slice_where_tpl: 'date = {value}',
    slice_file_tpl: '{key}',
    bak_scope: 'data',
    compression: 'gzip',
    min_rows: 1000,
    max_slices: 5000,
    redo_slices: 2,
    flag_upload: '1',
    remark: null,
    creator: null,
    gmt_create: null,
    gmt_update: null,
    plan: { rule_id: 0, table: '', total: 0, done: 0, missing: 0, skipped: 0, redo_from: 0, items: [] },
  }
}

async function loadRules() {
  loading.value = true
  try {
    const data = await apiGet<{ list: SliceRule[] }>(`${API_PREFIX}/Rule/List`)
    rules.value = data.list ?? []
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function openCreate() {
  form.value = emptyRule()
  dialog.value = true
}

function openEdit(row: SliceRule) {
  form.value = { ...row, slice_key_sql: row.slice_key_sql ?? null }
  dialog.value = true
}

async function saveRule() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    await apiPost(`${API_PREFIX}/Rule/Save`, form.value)
    dialog.value = false
    message.value = '保存成功'
    await loadRules()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

async function openPlan(row: SliceRule) {
  planDialog.value = true
  planLoading.value = true
  planDetail.value = null
  planOnlyMissing.value = false
  try {
    planDetail.value = await apiGet<SlicePlan>(`${API_PREFIX}/Plan?ruleId=${row.idn}`)
  } catch (e) {
    error.value = (e as Error).message
    planDialog.value = false
  } finally {
    planLoading.value = false
  }
}

const planItems = computed(() => {
  const items = planDetail.value?.items ?? []
  return planOnlyMissing.value ? items.filter((item) => !item.backuped) : items
})

async function toggleRule(row: SliceRule) {
  error.value = ''
  try {
    await apiPost(`${API_PREFIX}/Rule/Toggle`, { idn: row.idn, enable: row.flag_enable !== '1' })
    await loadRules()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function fetchStatus(sinceSeq = 0) {
  const suffix = sinceSeq > 0 ? `?sinceSeq=${sinceSeq}` : ''
  const data = await apiGet<SliceRunState>(`${API_PREFIX}/Run/Status${suffix}`)
  if (data.run_id !== currentRunId) {
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
    const box = document.getElementById('brs-slice-log')
    if (box) box.scrollTop = box.scrollHeight
  })
}

async function poll() {
  try {
    await fetchStatus(state.value.last_seq)
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    schedule()
  }
}

function schedule() {
  window.clearTimeout(timer)
  timer = window.setTimeout(poll, isRunning.value ? POLL_RUNNING_MS : POLL_IDLE_MS)
}

async function triggerManual() {
  error.value = ''
  message.value = ''
  triggering.value = true
  try {
    const data = await apiPost<SliceRunState>(`${API_PREFIX}/Run/Manual`, {})
    currentRunId = ''
    state.value = { ...data, logs: [...data.logs] }
    message.value = '已开始大表分片备份'
    await fetchStatus(0)
    schedule()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    triggering.value = false
  }
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

function modeText(v: string): string {
  const hit = modeItems.find((item) => item.value === v)
  return hit ? hit.title : v
}

onMounted(async () => {
  await loadRules()
  try {
    await fetchStatus(0)
  } catch (e) {
    error.value = (e as Error).message
  }
  schedule()
})

onBeforeUnmount(() => window.clearTimeout(timer))
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-table-large" class="mr-2" />
      大表增量备份
    </v-card-title>
    <v-card-subtitle>
      面向体量大、按天/周/月增长的表：按分片维度切分成多个文件备份并上传，同一分片键重跑覆盖；
      已存在的分片自动跳过，只补缺失与最近 N 个分片。与小表的全量备份相互独立。
    </v-card-subtitle>

    <v-card-text>
      <v-alert v-if="state.backup_dir_ok === false" type="error" class="mb-3" density="compact">
        备份目录不可用：{{ state.backup_dir_error }}（当前 <code>{{ state.backup_dir }}</code>）；
        请确认目录存在、位于可写文件系统且运行用户可写（Native 由 Install/Upgrade 预建，容器检查 volume 挂载）
      </v-alert>
      <v-alert v-if="error" type="error" class="mb-3" density="compact">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" class="mb-3" density="compact">{{ message }}</v-alert>

      <div class="d-flex align-center flex-wrap ga-2 mb-3">
        <v-btn color="primary" prepend-icon="mdi-plus" @click="openCreate">新增登记</v-btn>
        <v-btn color="secondary" prepend-icon="mdi-refresh" :loading="loading" @click="loadRules">刷新</v-btn>
        <v-btn
          color="primary"
          variant="outlined"
          prepend-icon="mdi-database-arrow-up"
          :loading="triggering"
          :disabled="isRunning"
          @click="triggerManual"
        >
          立即备份
        </v-btn>
        <v-chip :color="statusColor" size="small">{{ statusText }}</v-chip>
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
          <span class="text-caption">{{ state.done }} / {{ state.total }} 个分片（{{ progress }}%）</span>
        </template>
      </v-progress-linear>

      <v-row v-if="isRunning || state.run_id" dense class="text-body-2 mb-2">
        <v-col cols="12" sm="6" md="3"><span class="text-medium-emphasis">运行 ID：</span>{{ state.run_id || '-' }}</v-col>
        <v-col cols="12" sm="6" md="3"><span class="text-medium-emphasis">开始：</span>{{ formatTime(state.started_at) }}</v-col>
        <v-col cols="12" sm="6" md="3"><span class="text-medium-emphasis">耗时：</span>{{ state.elapsed_ms > 0 ? (state.elapsed_ms / 1000).toFixed(1) + 's' : '-' }}</v-col>
        <v-col cols="12" sm="6" md="3"><span class="text-medium-emphasis">失败分片：</span>{{ state.failed }}</v-col>
        <v-col cols="12" sm="6" md="3">
          <span class="text-medium-emphasis">备份目录：</span>
          <code>{{ state.backup_dir || '-' }}</code>
          <v-icon v-if="state.backup_dir_ok" icon="mdi-check-circle" color="success" size="x-small" class="ml-1" />
          <v-icon v-else icon="mdi-alert-circle" color="error" size="x-small" class="ml-1" />
        </v-col>
        <v-col v-if="state.current_table" cols="12">
          <span class="text-medium-emphasis">当前：</span>
          <code>{{ state.current_table }} / {{ state.current_slice }}</code>
        </v-col>
        <v-col v-if="state.error" cols="12" class="text-error">错误：{{ state.error }}</v-col>
      </v-row>

      <v-table density="compact" class="text-body-2">
        <thead>
          <tr>
            <th>表</th>
            <th>分片维度</th>
            <th>条件模板</th>
            <th class="text-center">分片</th>
            <th class="text-center">已备份</th>
            <th class="text-center">待备份</th>
            <th class="text-center">启用</th>
            <th class="text-center">上传</th>
            <th class="text-center">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="rules.length === 0">
            <td colspan="9" class="text-medium-emphasis">暂无登记（先新增一条，或由迁移 V300088 写入的 K 线种子）</td>
          </tr>
          <tr v-for="row in rules" :key="row.idn">
            <td><code>{{ row.db }}.{{ row.schema_name }}.{{ row.table_name }}</code></td>
            <td>{{ modeText(row.slice_mode) }}</td>
            <td><code>{{ row.slice_where_tpl }}</code></td>
            <td class="text-center">{{ row.plan.total }}</td>
            <td class="text-center">{{ row.plan.done }}</td>
            <td class="text-center" :class="row.plan.missing > 0 ? 'text-warning' : ''">{{ row.plan.missing }}</td>
            <td class="text-center">
              <v-chip size="x-small" :color="row.flag_enable === '1' ? 'success' : 'grey'">
                {{ row.flag_enable === '1' ? '启用' : '停用' }}
              </v-chip>
            </td>
            <td class="text-center">{{ row.flag_upload === '1' ? '是' : '否' }}</td>
            <td class="text-center">
              <v-btn size="x-small" variant="text" @click="openPlan(row)">分片明细</v-btn>
              <v-btn size="x-small" variant="text" @click="openEdit(row)">编辑</v-btn>
              <v-btn size="x-small" variant="text" @click="toggleRule(row)">{{ row.flag_enable === '1' ? '停用' : '启用' }}</v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>

      <div class="text-subtitle-2 mt-4 mb-1">运行日志</div>
      <div id="brs-slice-log" class="log-box">
        <div v-if="state.logs.length === 0" class="text-medium-emphasis">暂无日志（尚未执行过备份）</div>
        <div v-for="e in state.logs" :key="e.seq" :class="logClass(e.level)">
          <span class="log-time">{{ logTime(e.time) }}</span>
          <span class="log-level">[{{ e.level }}]</span>
          {{ e.message }}
        </div>
      </div>
    </v-card-text>
  </v-card>

  <!-- 登记编辑 -->
  <v-dialog v-model="dialog" max-width="720" persistent>
    <v-card>
      <v-card-title>{{ form.idn ? '编辑大表分片备份登记' : '新增大表分片备份登记' }}</v-card-title>
      <v-card-text>
        <v-row dense>
          <v-col cols="12" sm="4">
            <v-text-field v-model="form.db" label="库名" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model="form.schema_name" label="schema" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model="form.table_name" label="表名" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="6">
            <v-select v-model="form.slice_mode" :items="modeItems" item-title="title" item-value="value" label="分片维度" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="3">
            <v-text-field v-model.number="form.priority" label="优先级" type="number" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="3">
            <v-select
              v-model="form.compression"
              :items="[{ title: 'gzip（默认）', value: 'gzip' }, { title: '不压缩', value: 'none' }]"
              item-title="title"
              item-value="value"
              label="压缩"
              density="compact"
              variant="outlined"
            />
          </v-col>
          <v-col cols="12">
            <v-textarea
              v-model="form.slice_where_tpl"
              label="where 模板（占位 {value} / {start} / {end}）"
              rows="2"
              density="compact"
              variant="outlined"
              hint="按日：date = {value}；按周/月建议写成区间：date &gt;= {start} AND date &lt;= {end}"
              persistent-hint
            />
          </v-col>
          <v-col cols="12">
            <v-textarea
              v-model="form.slice_key_sql"
              label="分片枚举 SQL（可空；须返回 key/start/end/rows 四列）"
              rows="3"
              density="compact"
              variant="outlined"
              hint="留空则按分片维度使用内置模板；column / expr 必填"
              persistent-hint
            />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model="form.slice_file_tpl" label="文件名模板" density="compact" variant="outlined" hint="默认 {key}" persistent-hint />
          </v-col>
          <v-col cols="12" sm="4">
            <v-select
              v-model="form.bak_scope"
              :items="[{ title: 'data（仅数据，结构由迁移创建）', value: 'data' }]"
              item-title="title"
              item-value="value"
              label="备份范围"
              density="compact"
              variant="outlined"
            />
          </v-col>
          <v-col cols="12" sm="4">
            <v-select v-model="form.flag_upload" :items="[{ title: '上传', value: '1' }, { title: '仅本地', value: '0' }]" item-title="title" item-value="value" label="对象存储" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model.number="form.min_rows" label="分片最小行数" type="number" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model.number="form.max_slices" label="分片数上限" type="number" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="4">
            <v-text-field v-model.number="form.redo_slices" label="末尾重做分片数" type="number" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="6">
            <v-select v-model="form.flag_enable" :items="[{ title: '启用', value: '1' }, { title: '停用', value: '0' }]" item-title="title" item-value="value" label="启用状态" density="compact" variant="outlined" />
          </v-col>
          <v-col cols="12" sm="6">
            <v-text-field v-model="form.remark" label="备注" density="compact" variant="outlined" />
          </v-col>
        </v-row>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dialog = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" @click="saveRule">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- 分片明细 -->
  <v-dialog v-model="planDialog" max-width="760" scrollable>
    <v-card>
      <v-card-title class="d-flex align-center">
        <span>分片明细<template v-if="planDetail">：{{ planDetail.table }}</template></span>
        <v-spacer />
        <v-switch v-model="planOnlyMissing" label="仅看待备份" density="compact" hide-details class="mr-2" />
      </v-card-title>
      <v-card-text>
        <div v-if="planLoading" class="text-medium-emphasis">加载中…</div>
        <template v-else-if="planDetail">
          <v-row dense class="text-body-2 mb-2">
            <v-col cols="6" sm="3"><span class="text-medium-emphasis">分片总数：</span>{{ planDetail.total }}</v-col>
            <v-col cols="6" sm="3"><span class="text-medium-emphasis">已备份：</span>{{ planDetail.done }}</v-col>
            <v-col cols="6" sm="3"><span class="text-medium-emphasis">待备份：</span>{{ planDetail.missing }}</v-col>
            <v-col cols="6" sm="3"><span class="text-medium-emphasis">跳过（低于 min_rows）：</span>{{ planDetail.skipped }}</v-col>
          </v-row>
          <v-table density="compact" class="text-body-2" height="360">
            <thead>
              <tr>
                <th>分片键</th>
                <th class="text-right">行数</th>
                <th class="text-center">状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="planItems.length === 0">
                <td colspan="3" class="text-medium-emphasis">无匹配分片</td>
              </tr>
              <tr v-for="item in planItems" :key="item.key">
                <td><code>{{ item.key }}</code></td>
                <td class="text-right">{{ item.rows }}</td>
                <td class="text-center">
                  <v-chip size="x-small" :color="item.backuped ? 'success' : 'warning'">
                    {{ item.backuped ? '已备份' : '待备份' }}
                  </v-chip>
                </td>
              </tr>
            </tbody>
          </v-table>
          <div class="text-caption text-medium-emphasis mt-2">
            明细最多返回 200 条；末尾 {{ Math.max(planDetail.total - planDetail.redo_from, 0) }} 个分片属重做窗口，每轮会重新备份。
          </div>
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="planDialog = false">关闭</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-card class="mt-4" variant="flat">
    <v-card-title class="text-subtitle-2">配置说明</v-card-title>
    <v-card-text class="text-body-2 text-medium-emphasis">
      <v-row dense>
        <v-col cols="4" sm="3">切分依据</v-col>
        <v-col cols="8" sm="9">按登记的分片维度切分：日/周/月按 <code>date</code> 字段，也可按其他列值或自定义 SQL 条件</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">增量方式</v-col>
        <v-col cols="8" sm="9">分片键从库中枚举，产物已存在（本地或对象存储）即跳过；末尾 <code>redo_slices</code> 个分片每轮重做</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">产物路径</v-col>
        <v-col cols="8" sm="9">本地 <code>&lt;备份目录&gt;/Incr/&lt;库&gt;.&lt;schema&gt;.&lt;表&gt;/&lt;分片键&gt;.mvsv.gz</code>；远端 <code>&lt;prefix&gt;/&lt;环境&gt;/&lt;库&gt;.&lt;schema&gt;.&lt;表&gt;/&lt;分片键&gt;.mvsv.gz</code>（MVSV-2，gzip 压缩）</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">恢复顺序</v-col>
        <v-col cols="8" sm="9">先在目标库建好表结构（由迁移创建），再用 <code>qiom import &lt;分片&gt;.mvsv.gz</code> 逐片导入（与 qiom export 同一格式）</v-col>
      </v-row>
      <v-row dense>
        <v-col cols="4" sm="3">自动调度</v-col>
        <v-col cols="8" sm="9">本期只支持手动触发；定时需另行配置（验收后再定）</v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.log-box {
  max-height: 360px;
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
