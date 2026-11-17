<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../api'

// finv_exchange 交易所/市场字典行
interface ExchangeRow {
  exchange_code: number
  exchange_flag: string
  exchange_abbr: string
  exchange_name: string
  exchange_abbr_cn: string
  en_market_type: string
  region: string
  base_currency: string
  ft_list_exchange_code: string | null
  flag_enable: string
}

const rows = ref<ExchangeRow[]>([])
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
const form = ref<ExchangeRow>({
  exchange_code: 0,
  exchange_flag: '',
  exchange_abbr: '',
  exchange_name: '',
  exchange_abbr_cn: '',
  en_market_type: '',
  region: '',
  base_currency: '',
  ft_list_exchange_code: null,
  flag_enable: '1',
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{ total: number; list: ExchangeRow[] }>(
      `/Meta/FinvQuant/Metadata/Exchange/List?page=${page.value}&page_size=${pageSize.value}&keyword=${encodeURIComponent(keyword.value)}`,
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
    exchange_code: 0,
    exchange_flag: '',
    exchange_abbr: '',
    exchange_name: '',
    exchange_abbr_cn: '',
    en_market_type: '',
    region: '',
    base_currency: '',
    ft_list_exchange_code: null,
    flag_enable: '1',
  }
  dialog.value = true
}

function openEdit(row: ExchangeRow) {
  editing.value = true
  form.value = { ...row }
  dialog.value = true
}

async function save() {
  error.value = ''
  message.value = ''
  if (!form.value.exchange_code || form.value.exchange_code <= 0) {
    error.value = '交易所代码必须为正整数'
    return
  }
  if (!form.value.exchange_flag.trim() || !form.value.exchange_abbr.trim()) {
    error.value = '交易所标志与英文缩写不能为空'
    return
  }
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Exchange/Save', {
      exchange_code: form.value.exchange_code,
      exchange_flag: form.value.exchange_flag.trim(),
      exchange_abbr: form.value.exchange_abbr.trim(),
      exchange_name: form.value.exchange_name.trim(),
      exchange_abbr_cn: form.value.exchange_abbr_cn.trim(),
      en_market_type: form.value.en_market_type.trim(),
      region: form.value.region.trim(),
      base_currency: form.value.base_currency.trim(),
      ft_list_exchange_code: form.value.ft_list_exchange_code?.trim() || null,
    })
    message.value = '保存成功'
    dialog.value = false
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function toggle(row: ExchangeRow) {
  error.value = ''
  const next = row.flag_enable === '1' ? '0' : '1'
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Exchange/Toggle', { exchange_code: row.exchange_code, flag_enable: next })
    row.flag_enable = next
    message.value = `已${next === '1' ? '启用' : '禁用'}交易所 ${row.exchange_code}`
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
      <v-icon icon="mdi-office-building" class="mr-2" />
      交易所信息维护
    </v-card-title>
    <v-card-subtitle>
      finv_exchange 交易所/市场字典维护：查询展示、新增、修改、禁用/启用
    </v-card-subtitle>

    <v-card-text>
      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="keyword"
            label="关键字（代码/标志/缩写/名称）"
            placeholder="如: SSE / 上交所 / 11"
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
          { title: '代码', key: 'exchange_code', width: 90 },
          { title: '标志', key: 'exchange_flag', width: 90 },
          { title: '缩写', key: 'exchange_abbr', width: 90 },
          { title: '英文全称', key: 'exchange_name' },
          { title: '中文名称', key: 'exchange_abbr_cn' },
          { title: '市场类型', key: 'en_market_type', width: 110 },
          { title: '地区', key: 'region', width: 80 },
          { title: '货币', key: 'base_currency', width: 80 },
          { title: '状态', key: 'flag_enable', width: 90 },
          { title: '操作', key: 'actions', width: 150, sortable: false },
        ]"
        :items="rows"
        :loading="loading"
        :items-per-page="pageSize"
        item-value="exchange_code"
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
        <v-card-title>{{ editing ? '修改交易所' : '新增交易所' }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model.number="form.exchange_code" label="交易所代码（正整数）" type="number" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.exchange_flag" label="交易所标志（如 CN / SH / NSDQ）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.exchange_abbr" label="英文缩写（如 SSE / SZSE / HKEX）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.exchange_abbr_cn" label="中文名称（如 上交所 / 纳斯达克）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.exchange_name" label="英文全称（如 Shanghai Stock Exchange）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.en_market_type" label="市场类型（证券/期货/外汇等）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.region" label="地区（如 CN / HK / USA）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.base_currency" label="基础货币（如 CNY / USD）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.ft_list_exchange_code" label="FT 行情源列表编码（可选）" density="compact" />
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
