<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../api'

// finv_market 交易市场行
interface MarketRow {
  market_code: number
  market_flag: string
  market_abbr: string
  market_name: string
  en_security_type: string
  base_currency: string
  flag_enable: string
}

const rows = ref<MarketRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')

// 新增/编辑对话框
const dialog = ref(false)
const editing = ref(false)
const form = ref<MarketRow>({
  market_code: 0,
  market_flag: '',
  market_abbr: '',
  market_name: '',
  en_security_type: '',
  base_currency: '',
  flag_enable: '1',
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{ total: number; list: MarketRow[] }>(
      `/Meta/FinvQuant/Metadata/Market/List?page=${page.value}&page_size=${pageSize.value}&keyword=${encodeURIComponent(keyword.value)}`,
    )
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
  form.value = {
    market_code: 0,
    market_flag: '',
    market_abbr: '',
    market_name: '',
    en_security_type: '',
    base_currency: '',
    flag_enable: '1',
  }
  dialog.value = true
}

function openEdit(row: MarketRow) {
  editing.value = true
  form.value = { ...row }
  dialog.value = true
}

async function save() {
  error.value = ''
  message.value = ''
  if (!form.value.market_code || form.value.market_code <= 0) {
    error.value = '市场代码必须为正整数'
    return
  }
  if (!form.value.market_flag.trim()) {
    error.value = '市场标识不能为空'
    return
  }
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Market/Save', {
      market_code: form.value.market_code,
      market_flag: form.value.market_flag.trim(),
      market_abbr: form.value.market_abbr.trim(),
      market_name: form.value.market_name.trim(),
      en_security_type: form.value.en_security_type.trim(),
      base_currency: form.value.base_currency.trim(),
    })
    message.value = '保存成功'
    dialog.value = false
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function toggle(row: MarketRow) {
  error.value = ''
  const next = row.flag_enable === '1' ? '0' : '1'
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Market/Toggle', { market_code: row.market_code, flag_enable: next })
    row.flag_enable = next
    message.value = `已${next === '1' ? '启用' : '禁用'}市场 ${row.market_code}`
  } catch (e) {
    error.value = (e as Error).message
  }
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-chart-areaspline" class="mr-2" />
      交易所下设市场信息维护
    </v-card-title>
    <v-card-subtitle>
      finv_market 交易市场字典维护：查询展示、新增、修改、禁用/启用
    </v-card-subtitle>

    <v-card-text>
      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="keyword"
            label="关键字（代码/标识/名称/证券类型）"
            placeholder="如: SSE-A / 上交所 / 1110"
            density="compact"
            hide-details
            clearable
            @keyup.enter="search"
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
      <v-alert v-if="message" type="success" class="mb-3" density="compact">{{ message }}</v-alert>

      <v-data-table
        :headers="[
          { title: '市场代码', key: 'market_code', width: 100 },
          { title: '市场标识', key: 'market_flag', width: 110 },
          { title: '简码', key: 'market_abbr', width: 90 },
          { title: '市场名称', key: 'market_name' },
          { title: '证券类型', key: 'en_security_type', width: 120 },
          { title: '货币', key: 'base_currency', width: 80 },
          { title: '状态', key: 'flag_enable', width: 90 },
          { title: '操作', key: 'actions', width: 150, sortable: false },
        ]"
        :items="rows"
        :loading="loading"
        :items-per-page="pageSize"
        item-value="market_code"
        density="compact"
      >
        <template #item.flag_enable="{ item }">
          <v-chip :color="item.flag_enable === '1' ? 'success' : 'grey'" size="small">
            {{ item.flag_enable === '1' ? '启用' : '禁用' }}
          </v-chip>
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
      </v-data-table>

      <v-pagination
        v-model="page"
        :length="Math.max(1, Math.ceil(total / pageSize))"
        :total-visible="7"
        density="compact"
        class="mt-3"
        @update:model-value="load"
      />
    </v-card-text>

    <!-- 新增 / 编辑对话框 -->
    <v-dialog v-model="dialog" max-width="640">
      <v-card>
        <v-card-title>{{ editing ? '修改市场' : '新增市场' }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model.number="form.market_code" label="市场代码（正整数）" type="number" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.market_flag" label="市场标识（如 SSE-A / HKEX-H / US-I）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.market_abbr" label="交易所简码（如 SSE / HKEX / US）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.market_name" label="市场名称（如 上交所 A 股）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.en_security_type" label="证券类型编码（如 1110 / 1210 / 1310）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.base_currency" label="基础货币（如 CNY / USD / HKD）" density="compact" />
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
