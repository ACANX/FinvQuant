<script setup lang="ts">
// USC 补丁规则维护（cdeu.finv_quote_cdeu_usc_patch_rule，需求 10931）
// 接口前缀：/Meta/Finv/Quote/Cdeu/Usc/Patch
//   GET  /List     查询参数小驼峰 page/pageSize/keyword/status
//   POST /Save     idn=0 新增，否则更新
//   POST /Enable   启用/停用
//   POST /Delete   删除
//   GET  /Preview  试算（只读）
//   POST /Run      手动执行
import { onMounted, ref } from 'vue'

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

// 执行统计
interface PatchStats {
  rule_count: number
  batches: number
  scanned_rows: number
  moved_rows: number
  conflict_rows: number
  deleted_rows: number
  from_ts: number
  to_ts: number
}

const API_PREFIX = '/Meta/Finv/Quote/Cdeu/Usc/Patch'

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

const running = ref(false)
const lastStats = ref<PatchStats | null>(null)

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

async function run() {
  const ok = window.confirm('确认按当前生效规则执行补丁？执行会迁移行情行并删除旧 usc 行。')
  if (!ok) return
  running.value = true
  try {
    lastStats.value = await apiPost<PatchStats>(`${API_PREFIX}/Run`, { batch_size: 5000 })
    const stats = lastStats.value
    showToast(
      `执行完成：迁移 ${stats.moved_rows} 行，冲突 ${stats.conflict_rows} 行，删除 ${stats.deleted_rows} 行`,
      'success',
    )
    void load()
  } catch (err) {
    showToast(`执行失败：${(err as Error).message}`, 'error')
  } finally {
    running.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-swap-horizontal-bold" class="mr-2" />
      USC 补丁规则
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
          <v-btn color="warning" prepend-icon="mdi-play" :loading="running" @click="run">执行</v-btn>
        </v-col>
        <v-col cols="auto" class="ml-auto">
          <v-btn color="success" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-alert
        v-if="lastStats"
        type="info"
        density="compact"
        variant="tonal"
        class="mb-3"
      >
        上次执行：规则 {{ lastStats.rule_count }} 条，批次 {{ lastStats.batches }}，
        扫描 {{ lastStats.scanned_rows }} 行，迁移 {{ lastStats.moved_rows }} 行，
        冲突 {{ lastStats.conflict_rows }} 行，删除 {{ lastStats.deleted_rows }} 行
      </v-alert>

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
