<script setup lang="ts">
// BRS 备份登记维护（brs.brs_bak_rdb_table）：查询、新增、编辑、启用/禁用
// 接口：GET /Meta/BRS/Backup/Finv/Table/List（查询参数小驼峰 page/pageSize/keyword/flagEnable）
//       POST /Meta/BRS/Backup/Finv/Table/Save（idn=0 新增，否则更新）
//       POST /Meta/BRS/Backup/Finv/Table/Toggle（切换 flag_enable）
import { onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 一条备份登记（wire 字段 snake_case，与库列一致）
interface BackupTableRow {
  idn: number
  rdb_type: string
  db: string
  schema_name: string
  table_name: string
  flag_enable: string
  priority: number
  bak_scope: string
  bak_method: string
  compression: string
  flag_upload: string
  where_clause: string | null
  remark: string | null
  creator: string | null
  gmt_create: string | null
  gmt_update: string | null
}

const API_PREFIX = '/Meta/BRS/Backup/Finv/Table'

const rows = ref<BackupTableRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const flagEnable = ref('')
const loading = ref(false)
const error = ref('')

// 新增/编辑对话框
const dialog = ref(false)
const editing = ref(false)
const form = ref<BackupTableRow>(emptyForm())

function emptyForm(): BackupTableRow {
  return {
    idn: 0,
    rdb_type: 'postgresql',
    db: 'finv',
    schema_name: '',
    table_name: '',
    flag_enable: '1',
    priority: 100,
    bak_scope: 'both',
    bak_method: 'pg_dump',
    compression: 'zstd',
    flag_upload: '1',
    where_clause: null,
    remark: null,
    creator: null,
    gmt_create: null,
    gmt_update: null,
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const query = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (keyword.value.trim()) {
      query.set('keyword', keyword.value.trim())
    }
    if (flagEnable.value) {
      query.set('flagEnable', flagEnable.value)
    }
    const data = await apiGet<{ total: number; list: BackupTableRow[] }>(`${API_PREFIX}/List?${query.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = false
  form.value = emptyForm()
  dialog.value = true
}

function openEdit(row: BackupTableRow) {
  editing.value = true
  form.value = { ...row }
  dialog.value = true
}

async function save() {
  if (!form.value.db.trim() || !form.value.schema_name.trim() || !form.value.table_name.trim()) {
    showToast('数据库、schema 与表名不能为空', 'error')
    return
  }
  if (!Number.isInteger(form.value.priority) || form.value.priority < 0) {
    showToast('优先级必须为非负整数', 'error')
    return
  }
  try {
    await apiPost(`${API_PREFIX}/Save`, {
      idn: form.value.idn,
      rdb_type: form.value.rdb_type.trim() || 'postgresql',
      db: form.value.db.trim(),
      schema_name: form.value.schema_name.trim(),
      table_name: form.value.table_name.trim(),
      flag_enable: form.value.flag_enable,
      priority: form.value.priority,
      bak_scope: form.value.bak_scope,
      bak_method: form.value.bak_method,
      compression: form.value.compression,
      flag_upload: form.value.flag_upload,
      where_clause: form.value.where_clause?.trim() || null,
      remark: form.value.remark?.trim() || null,
    })
    showToast('保存成功')
    dialog.value = false
    await load()
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

async function toggle(row: BackupTableRow) {
  const next = row.flag_enable === '1' ? '0' : '1'
  try {
    await apiPost(`${API_PREFIX}/Toggle`, { idn: row.idn, flag_enable: next })
    row.flag_enable = next
    showToast(`已${next === '1' ? '启用' : '禁用'} ${row.db}.${row.schema_name}.${row.table_name}`)
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

function search() {
  page.value = 1
  load()
}

function styleTime(v: string | null): string {
  if (!v) return '-'
  return v.replace('T', ' ').slice(0, 19)
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-table-cog" class="mr-2" />
      备份登记维护
    </v-card-title>
    <v-card-subtitle>
      brs.brs_bak_rdb_table 备份范围登记：查询展示、新增、修改、启用/禁用（同一数据库内 schema+表唯一，优先级越大越先备份）
    </v-card-subtitle>

    <v-card-text>
      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="5">
          <v-text-field
            v-model="keyword"
            label="关键字（schema / 表名 / 备注）"
            placeholder="如: sys / finv_security"
            density="compact"
            hide-details
            clearable
            @keyup.enter="search"
          />
        </v-col>
        <v-col cols="12" sm="3">
          <v-select
            v-model="flagEnable"
            :items="[
              { title: '全部状态', value: '' },
              { title: '仅启用', value: '1' },
              { title: '仅禁用', value: '0' },
            ]"
            item-title="title"
            item-value="value"
            label="启用状态"
            density="compact"
            hide-details
            @update:model-value="search"
          />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" @click="search">查询</v-btn>
        </v-col>
        <v-col cols="auto" class="ml-auto">
          <v-btn color="success" prepend-icon="mdi-plus" @click="openCreate">新增</v-btn>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" class="mb-3" density="compact">{{ error }}</v-alert>

      <v-data-table
        :headers="[
          { title: '优先级', key: 'priority', width: 90 },
          { title: '数据库', key: 'db', width: 100 },
          { title: 'schema', key: 'schema_name', width: 120 },
          { title: '表名', key: 'table_name' },
          { title: '范围', key: 'bak_scope', width: 90 },
          { title: '方式', key: 'bak_method', width: 100 },
          { title: '压缩', key: 'compression', width: 90 },
          { title: '状态', key: 'flag_enable', width: 90 },
          { title: '上传', key: 'flag_upload', width: 80 },
          { title: '备注', key: 'remark' },
          { title: '修改时间', key: 'gmt_update', width: 160 },
          { title: '操作', key: 'actions', width: 150, sortable: false },
        ]"
        :items="rows"
        :loading="loading"
        :items-per-page="pageSize"
        item-value="idn"
        density="compact"
      >
        <template #item.flag_enable="{ item }">
          <v-chip :color="item.flag_enable === '1' ? 'success' : 'grey'" size="small">
            {{ item.flag_enable === '1' ? '启用' : '禁用' }}
          </v-chip>
        </template>
        <template #item.flag_upload="{ item }">
          <span>{{ item.flag_upload === '1' ? '是' : '否' }}</span>
        </template>
        <template #item.remark="{ item }">
          <span>{{ item.remark || '-' }}</span>
        </template>
        <template #item.gmt_update="{ item }">
          <span>{{ styleTime(item.gmt_update) }}</span>
        </template>
        <template #item.actions="{ item }">
          <v-btn size="x-small" variant="text" prepend-icon="mdi-pencil" @click="openEdit(item)">修改</v-btn>
          <v-btn
            size="x-small"
            variant="text"
            :prepend-icon="item.flag_enable === '1' ? 'mdi-toggle-switch-off' : 'mdi-toggle-switch'"
            @click="toggle(item)"
          >
            {{ item.flag_enable === '1' ? '禁用' : '启用' }}
          </v-btn>
        </template>
        <template #no-data>
          <div class="py-4 text-medium-emphasis">暂无备份登记</div>
        </template>
      </v-data-table>

      <v-pagination
        v-model="page"
        :length="Math.max(1, Math.ceil(total / pageSize))"
        :total-visible="7"
        density="compact"
        class="mt-3"
        @update:model-value="load"
      />
      <div class="text-caption text-medium-emphasis mt-2">共 {{ total }} 条登记</div>
    </v-card-text>

    <!-- 新增 / 编辑对话框 -->
    <v-dialog v-model="dialog" max-width="680">
      <v-card>
        <v-card-title>{{ editing ? '修改备份登记' : '新增备份登记' }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model="form.db" label="数据库名（如 finv）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.schema_name" label="schema 名（如 sys / cdeu）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.table_name" label="表名（如 sys_user）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field
                v-model.number="form.priority"
                label="优先级（越大越先，值唯一）"
                type="number"
                density="compact"
              />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.rdb_type" label="数据库类型（首期 postgresql）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-select
                v-model="form.bak_scope"
                :items="[
                  { title: '结构+数据（both）', value: 'both' },
                  { title: '仅结构（schema）', value: 'schema' },
                  { title: '仅数据（data）', value: 'data' },
                ]"
                item-title="title"
                item-value="value"
                label="备份范围"
                density="compact"
              />
            </v-col>
            <v-col cols="4">
              <v-select
                v-model="form.bak_method"
                :items="[
                  { title: 'pg_dump', value: 'pg_dump' },
                  { title: 'copy', value: 'copy' },
                ]"
                item-title="title"
                item-value="value"
                label="备份方式"
                density="compact"
              />
            </v-col>
            <v-col cols="4">
              <v-select
                v-model="form.compression"
                :items="[
                  { title: 'zstd', value: 'zstd' },
                  { title: 'gzip', value: 'gzip' },
                  { title: 'none', value: 'none' },
                ]"
                item-title="title"
                item-value="value"
                label="压缩"
                density="compact"
              />
            </v-col>
            <v-col cols="6">
              <v-switch
                :model-value="form.flag_enable === '1'"
                label="启用备份"
                color="success"
                density="compact"
                hide-details
                @update:model-value="form.flag_enable = $event ? '1' : '0'"
              />
            </v-col>
            <v-col cols="6">
              <v-switch
                :model-value="form.flag_upload === '1'"
                label="上传对象存储"
                color="primary"
                density="compact"
                hide-details
                @update:model-value="form.flag_upload = $event ? '1' : '0'"
              />
            </v-col>
            <v-col cols="12">
              <v-text-field
                v-model="form.where_clause"
                label="备份条件 where（预留，首期 pg_dump 不生效）"
                density="compact"
              />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.remark" label="备注" density="compact" />
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-card>
</template>
