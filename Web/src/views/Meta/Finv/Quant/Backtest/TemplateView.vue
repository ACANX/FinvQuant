<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiGet, apiPost } from '../../../../../api'

// 模板行
interface TemplateRow {
  template_id: string
  template_code: string
  template_name: string
  template_type: string
  content: Record<string, unknown>
  user_id: string
  is_builtin: string
  status: string
  description: string
}

const tmpls = ref<TemplateRow[]>([])
const tmplTotal = ref(0)
const tmplPage = ref(1)
const tmplPageSize = ref(10)
const tmplTypeFilter = ref('')
const tmplLoading = ref(false)

const error = ref('')
const message = ref('')

const tmplDialog = ref(false)
const tmplEditing = ref(false)
const tmplForm = ref<TemplateRow>(emptyTmpl())
const tmplContentText = ref('')

function emptyTmpl(): TemplateRow {
  return { template_id: '', template_code: '', template_name: '', template_type: 'STRATEGY', content: {}, user_id: 'default', is_builtin: '0', status: 'ENABLED', description: '' }
}

async function loadTemplates() {
  tmplLoading.value = true
  try {
    const q = new URLSearchParams({ page: String(tmplPage.value), pageSize: String(tmplPageSize.value) })
    if (tmplTypeFilter.value) q.set('templateType', tmplTypeFilter.value)
    const data = await apiGet<{ total: number; list: TemplateRow[] }>(`/Meta/Finv/Quant/Backtest/Template/List?${q.toString()}`)
    tmpls.value = data.list ?? []
    tmplTotal.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    tmplLoading.value = false
  }
}

function openTmplCreate() {
  tmplEditing.value = false
  tmplForm.value = emptyTmpl()
  tmplContentText.value = '{\n  \n}'
  tmplDialog.value = true
}

async function openTmplEdit(row: TemplateRow) {
  tmplEditing.value = true
  // 通过 Template/Get 拉取最新详情（评审：Get 端点此前未被前端调用）
  try {
    const detail = await apiGet<TemplateRow>(`/Meta/Finv/Quant/Backtest/Template/Get?templateId=${row.template_id}`)
    tmplForm.value = { ...(detail ?? row), content: JSON.parse(JSON.stringify((detail ?? row).content ?? {})) }
  } catch (e) {
    error.value = (e as Error).message
    tmplForm.value = { ...row, content: JSON.parse(JSON.stringify(row.content ?? {})) }
  }
  tmplContentText.value = JSON.stringify(tmplForm.value.content ?? {}, null, 2)
  tmplDialog.value = true
}

async function saveTmpl() {
  try {
    tmplForm.value.content = JSON.parse(tmplContentText.value)
  } catch (e) {
    error.value = '模板内容 JSON 格式错误：' + (e as Error).message
    return
  }
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Template/Save', tmplForm.value)
    message.value = '模板保存成功'
    tmplDialog.value = false
    await loadTemplates()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function removeTmpl(row: TemplateRow) {
  if (!confirm(`确认删除模板「${row.template_name}」？`)) return
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Template/Delete', { template_id: row.template_id })
    message.value = '模板删除成功'
    await loadTemplates()
  } catch (e) {
    error.value = (e as Error).message
  }
}

function tmplTypeName(t: string): string {
  return { STRATEGY: '策略模板', ACCOUNT: '账户模板', ENVIRONMENT: '环境模板' }[t] ?? t
}

onMounted(() => {
  loadTemplates()
})
</script>

<template>
  <v-container fluid>
    <v-alert v-if="error" type="error" dismissible class="mb-3">{{ error }}</v-alert>
    <v-alert v-if="message" type="success" dismissible class="mb-3">{{ message }}</v-alert>

    <!-- 模板管理 -->
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-content-copy" class="mr-2" color="primary" />
        模板管理（策略 / 账户 / 环境模板，相同部分复用、差异部分自定义）
        <v-spacer />
        <v-select v-model="tmplTypeFilter" :items="[
          { title: '全部类型', value: '' },
          { title: '策略模板', value: 'STRATEGY' },
          { title: '账户模板', value: 'ACCOUNT' },
          { title: '环境模板', value: 'ENVIRONMENT' },
        ]" density="compact" hide-details style="max-width: 160px" class="mr-2" @update:model-value="loadTemplates" />
        <v-btn size="small" variant="tonal" class="mr-2" @click="loadTemplates">刷新</v-btn>
        <v-btn color="primary" @click="openTmplCreate">新建模板</v-btn>
      </v-card-title>
      <v-card-text class="pt-0">
        <v-data-table-server v-model:page="tmplPage" v-model:items-per-page="tmplPageSize" :headers="[
          { title: '编码', key: 'template_code', width: 150 },
          { title: '名称', key: 'template_name' },
          { title: '类型', key: 'template_type', width: 110 },
          { title: '内置', key: 'is_builtin', width: 70 },
          { title: '说明', key: 'description' },
          { title: '操作', key: 'actions', width: 140, sortable: false },
        ]" :items="tmpls" :loading="tmplLoading" :items-length="tmplTotal" item-value="template_id" @update:options="loadTemplates">
          <template #item.template_type="{ item }">
            <v-chip size="small" variant="tonal" color="primary">{{ tmplTypeName(item.template_type) }}</v-chip>
          </template>
          <template #item.is_builtin="{ item }">
            <v-chip v-if="item.is_builtin === '1'" size="x-small" color="blue">内置</v-chip>
            <span v-else>-</span>
          </template>
          <template #item.actions="{ item }">
            <v-btn size="small" variant="text" @click="openTmplEdit(item)">编辑</v-btn>
            <v-btn size="small" variant="text" color="error" :disabled="item.is_builtin === '1'" @click="removeTmpl(item)">删除</v-btn>
          </template>
        </v-data-table-server>
      </v-card-text>
    </v-card>

    <!-- 模板编辑对话框 -->
    <v-dialog v-model="tmplDialog" max-width="760">
      <v-card>
        <v-card-title>{{ tmplEditing ? '编辑模板' : '新建模板' }}</v-card-title>
        <v-card-text>
          <v-row>
            <v-col cols="6"><v-text-field v-model="tmplForm.template_code" label="模板编码 *" hint="如 TPL-STRAT-DUALMA" /></v-col>
            <v-col cols="6"><v-text-field v-model="tmplForm.template_name" label="模板名称 *" /></v-col>
          </v-row>
          <v-row>
            <v-col cols="6">
              <v-select v-model="tmplForm.template_type" :items="[
                { title: '策略模板', value: 'STRATEGY' },
                { title: '账户模板', value: 'ACCOUNT' },
                { title: '环境模板', value: 'ENVIRONMENT' },
              ]" label="模板类型" />
            </v-col>
            <v-col cols="6"><v-text-field v-model="tmplForm.description" label="说明" /></v-col>
          </v-row>
          <v-textarea v-model="tmplContentText" label="模板内容（JSON）" rows="14"
            style="font-family: monospace" spellcheck="false" />
          <v-alert type="info" variant="tonal" density="compact">
            模板按类型复用相同部分（策略定义/账户配置/环境配置），差异部分在创建时自定义。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="tmplDialog = false">取消</v-btn>
          <v-btn color="primary" @click="saveTmpl">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>
