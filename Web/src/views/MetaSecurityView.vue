<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../api'

// finv_security 证券代码行
interface SecurityRow {
  usc: string
  exchange_code: number
  market_code: number
  security_type: string
  security_code: string
  security_name: string
  security_name_cn: string
  security_name_full: string | null
  currency_type: string
  init_date: number
  timezone: string | null
  tz: string | null
  flag_enable: string
}

// finv_exchange 交易所下拉选项（交易所信息维护字典）
interface ExchangeOption {
  exchange_code: number
  exchange_abbr: string
  exchange_name: string
  exchange_abbr_cn: string
  flag_enable: string
}

// finv_market 市场下拉选项（交易所下设市场字典）
interface MarketOption {
  market_code: number
  market_flag: string
  market_name: string
  flag_enable: string
}

const rows = ref<SecurityRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')

// 下拉选项：交易所（finv_exchange）/ 市场（finv_market），仅启用记录
const exchangeOptions = ref<ExchangeOption[]>([])
const marketOptions = ref<MarketOption[]>([])
const exchangeItems = computed(() =>
  exchangeOptions.value.map((e) => ({
    title: `${e.exchange_code} ${e.exchange_abbr}（${e.exchange_name}）`,
    value: e.exchange_code,
  })),
)
const marketItems = computed(() =>
  marketOptions.value.map((m) => ({
    title: `${m.market_code} ${m.market_flag}（${m.market_name}）`,
    value: m.market_code,
  })),
)

// 新增/编辑对话框
const dialog = ref(false)
const editing = ref(false)
const form = ref<SecurityRow>({
  usc: '',
  exchange_code: 0,
  market_code: 0,
  security_type: '',
  security_code: '',
  security_name: '',
  security_name_cn: '',
  security_name_full: null,
  currency_type: '',
  init_date: 20000000,
  timezone: null,
  tz: null,
  flag_enable: '1',
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{ total: number; list: SecurityRow[] }>(
      `/Meta/FinvQuant/Metadata/Security/List?page=${page.value}&page_size=${pageSize.value}&keyword=${encodeURIComponent(keyword.value)}`,
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
    usc: '',
    exchange_code: 0,
    market_code: 0,
    security_type: '',
    security_code: '',
    security_name: '',
    security_name_cn: '',
    security_name_full: null,
    currency_type: '',
    init_date: 20000000,
    timezone: null,
    tz: null,
    flag_enable: '1',
  }
  dialog.value = true
}

function openEdit(row: SecurityRow) {
  editing.value = true
  form.value = { ...row }
  dialog.value = true
}

async function save() {
  error.value = ''
  message.value = ''
  if (!form.value.usc.trim()) {
    error.value = '统一证券代码（usc）不能为空'
    return
  }
  if (!form.value.exchange_code || form.value.exchange_code <= 0) {
    error.value = '交易所代码必须为正整数'
    return
  }
  if (!form.value.security_code.trim() || !form.value.security_name_cn.trim()) {
    error.value = '源证券代码与中文名称不能为空'
    return
  }
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Security/Save', {
      usc: form.value.usc.trim(),
      exchange_code: form.value.exchange_code,
      market_code: form.value.market_code ?? 0,
      security_type: form.value.security_type.trim(),
      security_code: form.value.security_code.trim(),
      security_name: form.value.security_name.trim(),
      security_name_cn: form.value.security_name_cn.trim(),
      security_name_full: form.value.security_name_full?.trim() || null,
      currency_type: form.value.currency_type.trim(),
      init_date: form.value.init_date,
      timezone: form.value.timezone?.trim() || null,
      tz: form.value.tz?.trim() || null,
    })
    message.value = '保存成功'
    dialog.value = false
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function toggle(row: SecurityRow) {
  error.value = ''
  const next = row.flag_enable === '1' ? '0' : '1'
  try {
    await apiPost('/Meta/FinvQuant/Metadata/Security/Toggle', { usc: row.usc, flag_enable: next })
    row.flag_enable = next
    message.value = `已${next === '1' ? '启用' : '禁用'}证券 ${row.usc}`
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadOptions() {
  try {
    const [exData, mkData] = await Promise.all([
      apiGet<{ list: ExchangeOption[] }>('/Meta/FinvQuant/Metadata/Exchange/List?page=1&page_size=500'),
      apiGet<{ list: MarketOption[] }>('/Meta/FinvQuant/Metadata/Market/List?page=1&page_size=500'),
    ])
    // 仅启用（flag_enable='1'）的记录入下拉；保持后端排序（启用优先）
    exchangeOptions.value = (exData.list ?? []).filter((e) => e.flag_enable === '1')
    marketOptions.value = (mkData.list ?? []).filter((m) => m.flag_enable === '1')
  } catch {
    exchangeOptions.value = []
    marketOptions.value = []
  }
}

function search() {
  page.value = 1
  load()
}

onMounted(() => {
  load()
  loadOptions()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-tag-multiple" class="mr-2" />
      规范证券信息维护
    </v-card-title>
    <v-card-subtitle>
      finv_security 证券代码字典维护：查询展示、新增、修改、禁用/启用（usc 为统一证券代码）
    </v-card-subtitle>

    <v-card-text>
      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="6">
          <v-text-field
            v-model="keyword"
            label="关键字（usc / 证券代码 / 证券名称）"
            placeholder="如: GCMain / NVDA / 黄金"
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
          { title: 'usc', key: 'usc', width: 110 },
          { title: '交易所', key: 'exchange_code', width: 80 },
          { title: '市场', key: 'market_code', width: 80 },
          { title: '类型', key: 'security_type', width: 100 },
          { title: '源代码', key: 'security_code', width: 110 },
          { title: '证券名称', key: 'security_name' },
          { title: '中文名称', key: 'security_name_cn', width: 140 },
          { title: '货币', key: 'currency_type', width: 80 },
          { title: '上市日', key: 'init_date', width: 100 },
          { title: '状态', key: 'flag_enable', width: 90 },
          { title: '操作', key: 'actions', width: 150, sortable: false },
        ]"
        :items="rows"
        :loading="loading"
        :items-per-page="pageSize"
        item-value="usc"
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
    <v-dialog v-model="dialog" max-width="680">
      <v-card>
        <v-card-title>{{ editing ? '修改证券' : '新增证券' }}</v-card-title>
        <v-card-text>
          <v-row dense>
            <v-col cols="6">
              <v-text-field v-model="form.usc" label="usc（统一证券代码，全局唯一）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_code" label="源证券代码（交易所原始代码）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_name_cn" label="证券名称（中文）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.security_name" label="源证券名称" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.security_name_full" label="证券名称（全称，可选）" density="compact" />
            </v-col>
            <v-col cols="6">
              <v-select
                v-model="form.exchange_code"
                label="交易所代码（基于 finv_exchange 下拉）"
                :items="exchangeItems"
                clearable
                density="compact"
                hint="仅展示启用状态的交易所"
              />
            </v-col>
            <v-col cols="6">
              <v-select
                v-model="form.market_code"
                label="市场代码（基于 finv_market 下拉，缺省 0）"
                :items="marketItems"
                clearable
                density="compact"
                hint="仅展示启用状态的市场"
              />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model="form.security_type" label="证券类型（如 Stock / ETF / Futures）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.currency_type" label="计价货币（对齐 finv_currency）" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model.number="form.init_date" label="上市日期（yyyymmdd）" type="number" density="compact" />
            </v-col>
            <v-col cols="4">
              <v-text-field v-model="form.timezone" label="时区偏移（如 -04:00）" density="compact" />
            </v-col>
            <v-col cols="12">
              <v-text-field v-model="form.tz" label="时区标识（如 America/New_York）" density="compact" />
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
