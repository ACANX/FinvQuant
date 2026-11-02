<script setup lang="ts">
// 过时USC映射到新USC：规则维护与执行（cdeu.finv_quote_cdeu_usc_patch_rule，需求 10931）
// 接口前缀：/Meta/Finv/Quote/Cdeu/Usc/Patch
//   GET  /List     查询参数小驼峰 page/pageSize/keyword/status
//   POST /Save     idn=0 新增，否则更新
//   POST /Enable   启用/停用
//   POST /Delete   删除
//   GET  /Preview  试算（只读）
//   POST /Run      手动触发（异步），返回运行快照
//   GET  /Status   运行状态 + 增量日志（sinceSeq）
// 交互与 BRS 手动备份一致：运行中 1s 轮询、空闲 5s，run_id 变化时清空日志重新累积。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 一条补丁规则（wire 字段 snake_case，与库列一致）
interface PatchRuleRow {
  idn: number
  usc_old: string
  usc_new: string
  match_mode: string
  flag_enable: string
  priority: number
  note: string
  gmt_create: string | null
  gmt_update: string | null
}

// 试算结果
interface PatchPreviewItem {
  usc_old: string
  usc_new: string
  rows: number
  conflict_rows: number
  min_ts: number
  max_ts: number
}

// 运行日志
interface PatchLogEntry {
  seq: number
  time: string
  level: string
  message: string
}

// 运行状态快照
interface PatchRunState {
  run_id: string
  trigger: string
  status: string
  started_at: string | null
  finished_at: string | null
  elapsed_ms: number
  rule_count: number
  batches: number
  scanned_rows: number
  moved_rows: number
  conflict_rows: number
  deleted_rows: number
  current_batch: string
  error: string
  logs: PatchLogEntry[]
  last_seq: number
}

const API_PREFIX = '/Meta/Finv/Quote/Cdeu/Usc/Patch'
const POLL_RUNNING_MS = 1000
const POLL_IDLE_MS = 5000

const rows = ref<PatchRuleRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const status = ref('')
const loading = ref(false)
const error = ref('')

const dialog = ref(false)
const editing = ref(false)
const form = ref<PatchRuleRow>(emptyForm())

const previewDialog = ref(false)
const previewRows = ref<PatchPreviewItem[]>([])
const previewTotal = ref(0)
const previewConflict = ref(0)
const previewLoading = ref(false)

const triggering = ref(false)
const autoScroll = ref(true)
let timer: ReturnType<typeof setTimeout> | undefined
let currentRunId = ''

const state = ref<PatchRunState>({
  run_id: '',
  trigger: 'manual',
  status: 'idle',
  started_at: null,
  finished_at: null,
  elapsed_ms: 0,
  rule_count: 0,
  batches: 0,
  scanned_rows: 0,
  moved_rows: 0,
  conflict_rows: 0,
  deleted_rows: 0,
  current_batch: '',
  error: '',
  logs: [],
  last_seq: 0,
})

const isRunning = computed(() => state.value.status === 'running')
const statusText = computed(() => {
  switch (state.value.status) {
    case 'running':
      return '执行中'
    case 'succeeded':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return '未执行'
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

function emptyForm(): PatchRuleRow {
  return {
    idn: 0,
    usc_old: '',
    usc_new: '',
    match_mode: 'exact',
    flag_enable: '0',
    priority: 100,
    note: '',
    gmt_create: null,
    gmt_update: null,
  }
}

function toText(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

function logTime(value: string): string {
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('zh-CN', { hour12: false })
}

function logClass(level: string): string {
  if (level === 'error') return 'log-error'
  if (level === 'warn') return 'log-warn'
  return 'log-info'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (status.value) params.set('status', status.value)
    const data = await apiGet<{ total: number; list: PatchRuleRow[] }>(
      `${API_PREFIX}/List?${params.toString()}`,
    )
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function openCreate() {
  editing.value = false
  form.value = emptyForm()
  dialog.value = true
}

function openEdit(row: PatchRuleRow) {
  editing.value = true
  form.value = { ...row }
  dialog.value = true
}

async function save() {
  if (!form.value.usc_old.trim() || !form.value.usc_new.trim()) {
    showToast('usc_old 与 usc_new 不能为空', 'warning')
    return
  }
  try {
    await apiPost(`${API_PREFIX}/Save`, form.value)
    showToast(editing.value ? '修改成功' : '新增成功', 'success')
    dialog.value = false
    void load()
  } catch (err) {
    showToast(`保存失败：${(err as Error).message}`, 'error')
  }
}

async function toggle(row: PatchRuleRow) {
  const enable = row.flag_enable !== '1'
  try {
    await apiPost(`${API_PREFIX}/Enable`, { idn: row.idn, enable })
    showToast(enable ? '已启用' : '已停用', 'success')
    void load()
  } catch (err) {
    showToast(`操作失败：${(err as Error).message}`, 'error')
  }
}

async function remove(row: PatchRuleRow) {
  const ok = window.confirm(`确认删除规则 ${row.usc_old} → ${row.usc_new}？建议优先停用。`)
  if (!ok) return
  try {
    await apiPost(`${API_PREFIX}/Delete`, { idn: row.idn })
    showToast('删除成功', 'success')
    void load()
  } catch (err) {
    showToast(`删除失败：${(err as Error).message}`, 'error')
  }
}

async function preview() {
  previewLoading.value = true
  try {
    const data = await apiGet<{
      list: PatchPreviewItem[]
      total_rows: number
      total_conflict: number
    }>(`${API_PREFIX}/Preview`)
    previewRows.value = data.list ?? []
    previewTotal.value = data.total_rows ?? 0
    previewConflict.value = data.total_conflict ?? 0
    previewDialog.value = true
  } catch (err) {
    showToast(`试算失败：${(err as Error).message}`, 'error')
  } finally {
    previewLoading.value = false
  }
}

function scrollToBottom() {
  if (!autoScroll.value) return
  requestAnimationFrame(() => {
    const box = document.getElementById('usc-patch-log')
    if (box) box.scrollTop = box.scrollHeight
  })
}

async function fetchStatus(sinceSeq = 0) {
  const suffix = sinceSeq > 0 ? `?sinceSeq=${sinceSeq}` : ''
  const data = await apiGet<PatchRunState>(`${API_PREFIX}/Status${suffix}`)
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

function schedule() {
  window.clearTimeout(timer)
  timer = window.setTimeout(poll, isRunning.value ? POLL_RUNNING_MS : POLL_IDLE_MS)
}

async function poll() {
  try {
    await fetchStatus(state.value.last_seq)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    schedule()
  }
}

async function triggerRun() {
  const ok = window.confirm('确认按当前生效规则执行补丁？执行会迁移行情行并删除旧 usc 行。')
  if (!ok) return
  triggering.value = true
  try {
    const data = await apiPost<PatchRunState>(`${API_PREFIX}/Run`, {})
    currentRunId = ''
    state.value = { ...data, logs: [...data.logs] }
    showToast(`已开始执行（run_id=${data.run_id}）`, 'success')
    await fetchStatus(0)
    schedule()
    void load()
  } catch (e) {
    showToast(`触发失败：${(e as Error).message}`, 'error')
  } finally {
    triggering.value = false
  }
}

onMounted(async () => {
  await load()
  try {
    await fetchStatus(0)
  } catch {
    // 首次拉取失败不阻塞页面
  }
  schedule()
})

onBeforeUnmount(() => {
  window.clearTimeout(timer)
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-swap-horizontal-bold" class="mr-2" />
      过时USC映射到新USC
    </v-card-title>
    <v-card-subtitle>
      cdeu.finv_quote_cdeu_usc_patch_rule：把过时 usc 的行情迁移到最新定义的 usc（主键冲突按 gmt_update 较新者胜）
    </v-card-subtitle>

    <v-card-text>
      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="4">
          <v-text-field
            v-model="keyword"
            label="关键字（旧 usc / 新 usc / 备注）"
            placeholder="如: SH000688"
            density="compact"
            hide-details
            clearable
            @keyup.enter="search"
          />
        </v-col>
        <v-col cols="12" sm="3">
          <v-select
            v-model="status"
            :items="[
              { title: '全部状态', value: '' },
              { title: '仅生效', value: '1' },
              { title: '仅停用', value: '0' },
            ]"
            item-title="title"
            item-value="value"
            label="生效状态"
            density="compact"
            hide-details
            @update:model-value="search"
          />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" @click="search">查询</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn color="secondary" prepend-icon="mdi-eye-outline" :loading="previewLoading" @click="preview">
            试算
          </v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn
            color="warning"
            prepend-icon="mdi-play"
            :loading="triggering || isRunning"
            :disabled="isRunning"
            @click="triggerRun"
          >
            执行
          </v-btn>
        </v-col>
        <v-col cols="auto" class="ml-auto">
          <v-btn color="success" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <!-- 运行状态与日志（与 BRS 手动备份一致：状态卡 + 逐批日志） -->
      <v-card variant="outlined" class="mb-4">
        <v-card-text class="pb-2">
          <v-row dense align="center">
            <v-col cols="auto">
              <v-chip :color="statusColor" size="small" variant="flat">{{ statusText }}</v-chip>
            </v-col>
            <v-col cols="auto" class="text-caption">
              run_id：{{ state.run_id || '-' }}
            </v-col>
            <v-col cols="auto" class="text-caption">
              触发：{{ state.trigger === 'scheduled' ? '定时' : '手动' }}
            </v-col>
            <v-col cols="auto" class="text-caption">
              耗时：{{ (state.elapsed_ms / 1000).toFixed(1) }} s
            </v-col>
            <v-col cols="auto" class="ml-auto">
              <v-checkbox v-model="autoScroll" label="自动滚动" density="compact" hide-details />
            </v-col>
          </v-row>
          <v-row dense class="mt-1">
            <v-col cols="auto" class="text-caption">生效规则：{{ state.rule_count }}</v-col>
            <v-col cols="auto" class="text-caption">批次：{{ state.batches }}</v-col>
            <v-col cols="auto" class="text-caption">扫描：{{ state.scanned_rows }}</v-col>
            <v-col cols="auto" class="text-caption">迁移：{{ state.moved_rows }}</v-col>
            <v-col cols="auto" class="text-caption">冲突：{{ state.conflict_rows }}</v-col>
            <v-col cols="auto" class="text-caption">删除：{{ state.deleted_rows }}</v-col>
            <v-col v-if="state.current_batch" cols="auto" class="text-caption">
              当前批 ts：{{ state.current_batch }}
            </v-col>
          </v-row>
          <v-alert v-if="state.error" type="error" density="compact" variant="tonal" class="mt-2">
            {{ state.error }}
          </v-alert>
        </v-card-text>
        <v-divider />
        <v-card-text class="pa-0">
          <div id="usc-patch-log" class="log-box">
            <div v-if="state.logs.length === 0" class="text-medium-emphasis">暂无日志（尚未执行过补丁）</div>
            <div v-for="e in state.logs" :key="e.seq" :class="logClass(e.level)">
              <span class="log-time">{{ logTime(e.time) }}</span>
              <span class="log-level">[{{ e.level }}]</span>
              <span>{{ e.message }}</span>
            </div>
          </div>
        </v-card-text>
      </v-card>

      <v-data-table
        :headers="[
          { title: '序号', key: 'idn', width: 80 },
          { title: '旧 USC', key: 'usc_old' },
          { title: '新 USC', key: 'usc_new' },
          { title: '匹配', key: 'match_mode', width: 100 },
          { title: '优先级', key: 'priority', width: 90 },
          { title: '生效', key: 'flag_enable', width: 90 },
          { title: '备注', key: 'note' },
          { title: '更新时间', key: 'gmt_update', width: 170 },
          { title: '操作', key: 'actions', sortable: false, width: 220 },
        ]"
        :items="rows"
        :loading="loading"
        item-value="idn"
        density="compact"
      >
        <template #[`item.flag_enable`]="{ item }">
          <v-chip :color="toText(item.flag_enable) === '1' ? 'success' : 'default'" size="small" variant="tonal">
            {{ toText(item.flag_enable) === '1' ? '生效' : '停用' }}
          </v-chip>
        </template>
        <template #[`item.match_mode`]="{ item }">
          {{ toText(item.match_mode) === 'prefix' ? '前缀' : '精确' }}
        </template>
        <template #[`item.note`]="{ item }">
          <span class="text-caption">{{ toText(item.note) }}</span>
        </template>
        <template #[`item.actions`]="{ item }">
          <v-btn size="small" variant="text" @click="openEdit(item)">编辑</v-btn>
          <v-btn size="small" variant="text" @click="toggle(item)">
            {{ toText(item.flag_enable) === '1' ? '停用' : '启用' }}
          </v-btn>
          <v-btn size="small" variant="text" color="error" @click="remove(item)">删除</v-btn>
        </template>
      </v-data-table>

      <v-row class="mt-2" align="center" dense>
        <v-col cols="auto">
          <span class="text-caption">共 {{ total }} 条</span>
        </v-col>
        <v-col cols="auto" class="ml-auto">
          <v-pagination
            v-model="page"
            :length="Math.max(1, Math.ceil(total / pageSize))"
            :total-visible="5"
            density="compact"
            @update:model-value="load"
          />
        </v-col>
      </v-row>
    </v-card-text>

    <v-dialog v-model="dialog" max-width="640">
      <v-card>
        <v-card-title>{{ editing ? '编辑规则' : '新增规则' }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="form.usc_old" label="旧 USC（过时）" density="compact" class="mb-2" />
          <v-text-field v-model="form.usc_new" label="新 USC（目标）" density="compact" class="mb-2" />
          <v-select
            v-model="form.match_mode"
            :items="[
              { title: '精确匹配', value: 'exact' },
              { title: '前缀匹配', value: 'prefix' },
            ]"
            item-title="title"
            item-value="value"
            label="匹配方式"
            density="compact"
            class="mb-2"
          />
          <v-text-field v-model.number="form.priority" label="优先级（数值小的先匹配）" type="number" density="compact" class="mb-2" />
          <v-select
            v-model="form.flag_enable"
            :items="[
              { title: '停用', value: '0' },
              { title: '生效', value: '1' },
            ]"
            item-title="title"
            item-value="value"
            label="生效状态"
            density="compact"
            class="mb-2"
          />
          <v-text-field v-model="form.note" label="备注（规则依据）" density="compact" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="previewDialog" max-width="760">
      <v-card>
        <v-card-title>试算结果（只读，不写库）</v-card-title>
        <v-card-subtitle>
          生效规则命中 {{ previewTotal }} 行，其中 {{ previewConflict }} 行会与目标 usc 冲突（按较新者胜覆盖）
        </v-card-subtitle>
        <v-card-text>
          <v-data-table
            :headers="[
              { title: '旧 USC', key: 'usc_old' },
              { title: '新 USC', key: 'usc_new' },
              { title: '命中行数', key: 'rows' },
              { title: '冲突行数', key: 'conflict_rows' },
            ]"
            :items="previewRows"
            density="compact"
            item-value="usc_old"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="previewDialog = false">关闭</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>
</template>

<style scoped>
.log-box {
  height: 220px;
  overflow-y: auto;
  padding: 8px 12px;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
}
.log-time {
  color: #888;
  margin-right: 8px;
}
.log-level {
  margin-right: 8px;
}
.log-info {
  color: inherit;
}
.log-warn {
  color: #d9822b;
}
.log-error {
  color: #d33;
}
</style>
