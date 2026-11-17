<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiGet, apiPost } from '../../../../../api'

// 回测账户行
interface AccountRow {
  account_id: string
  account_code: string
  account_name: string
  initial_capital: number
  currency_type: string
  commission_rate: number
  slippage_pct: number
  margin_mode: string
  margin_rate: number
  allow_backtest: string
  status: string
  remark: string
  created_by: string
}

const rows = ref<AccountRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')

const dialog = ref(false)
const editing = ref(false)
const form = ref<AccountRow>({
  account_id: '',
  account_code: '',
  account_name: '',
  initial_capital: 100000,
  currency_type: 'USD',
  commission_rate: 0.0003,
  slippage_pct: 0.0001,
  margin_mode: 'FULL',
  margin_rate: 1,
  allow_backtest: '1',
  status: 'ENABLED',
  remark: '',
  created_by: '',
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{ total: number; list: AccountRow[] }>(
      `/Meta/Finv/Quant/Backtest/Account/List?page=${page.value}&pageSize=${pageSize.value}&keyword=${encodeURIComponent(keyword.value)}`,
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
    account_id: '', account_code: '', account_name: '', initial_capital: 100000,
    currency_type: 'USD', commission_rate: 0.0003, slippage_pct: 0.0001,
    margin_mode: 'FULL', margin_rate: 1, allow_backtest: '1', status: 'ENABLED',
    remark: '', created_by: '',
  }
  dialog.value = true
}

async function openEdit(row: AccountRow) {
  editing.value = true
  // 通过 Account/Get 拉取最新详情（评审：Get 端点此前未被前端调用，已登记为已使用）
  try {
    const detail = await apiGet<AccountRow>(`/Meta/Finv/Quant/Backtest/Account/Get?accountId=${row.account_id}`)
    form.value = { ...(detail ?? row) }
  } catch (e) {
    error.value = (e as Error).message
    form.value = { ...row }
  }
  dialog.value = true
}

async function save() {
  error.value = ''
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Account/Save', form.value)
    message.value = '保存成功'
    dialog.value = false
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function toggle(row: AccountRow) {
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Account/Toggle', {
      account_id: row.account_id,
      allow_backtest: row.allow_backtest === '1' ? '0' : '1',
    })
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function remove(row: AccountRow) {
  if (!confirm(`确认删除账户「${row.account_name}」？`)) return
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Account/Delete', { account_id: row.account_id })
    message.value = '删除成功'
    await load()
  } catch (e) {
    error.value = (e as Error).message
  }
}

onMounted(load)
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-account-cog-outline" class="mr-2" color="primary" />
        账户管理（回测账户）
        <v-spacer />
        <v-text-field v-model="keyword" density="compact" label="搜索编码/名称/备注" hide-details
          class="mx-4" style="max-width: 260px" @keyup.enter="load" />
        <v-btn color="primary" variant="tonal" class="mr-2" @click="load">查询</v-btn>
        <v-btn color="primary" @click="openCreate">新增账户</v-btn>
      </v-card-title>

      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" dismissible class="mx-4 mb-2">{{ message }}</v-alert>

      <v-data-table-server v-model:page="page" v-model:items-per-page="pageSize" :headers="[
        { title: '编码', key: 'account_code', width: 130 },
        { title: '名称', key: 'account_name' },
        { title: '初始资金', key: 'initial_capital', width: 130 },
        { title: '币种', key: 'currency_type', width: 80 },
        { title: '手续费率', key: 'commission_rate', width: 110 },
        { title: '滑点', key: 'slippage_pct', width: 100 },
        { title: '保证金模式', key: 'margin_mode', width: 110 },
        { title: '回测开关', key: 'allow_backtest', width: 100 },
        { title: '操作', key: 'actions', width: 180, sortable: false },
      ]" :items="rows" :loading="loading" :items-length="total" item-value="account_id"
        @update:options="load">
        <template #item.initial_capital="{ item }">
          {{ Number(item.initial_capital).toLocaleString() }}
        </template>
        <template #item.commission_rate="{ item }">
          {{ (Number(item.commission_rate) * 100).toFixed(4) }}%
        </template>
        <template #item.slippage_pct="{ item }">
          {{ (Number(item.slippage_pct) * 100).toFixed(4) }}%
        </template>
        <template #item.margin_mode="{ item }">
          <v-chip size="small" :color="item.margin_mode === 'FUTURES' ? 'warning' : 'default'">
            {{ item.margin_mode === 'FUTURES' ? '期货保证金' : '全额' }}
          </v-chip>
        </template>
        <template #item.allow_backtest="{ item }">
          <v-switch :model-value="item.allow_backtest === '1'" density="compact" hide-details
            :color="item.allow_backtest === '1' ? 'green' : 'grey'" @update:model-value="toggle(item)" />
        </template>
        <template #item.actions="{ item }">
          <v-btn size="small" variant="text" @click="openEdit(item)">编辑</v-btn>
          <v-btn size="small" variant="text" color="error" @click="remove(item)">删除</v-btn>
        </template>
      </v-data-table-server>
    </v-card>

    <!-- 新增/编辑对话框 -->
    <v-dialog v-model="dialog" max-width="620">
      <v-card>
        <v-card-title>{{ editing ? '编辑账户' : '新增账户' }}</v-card-title>
        <v-card-text>
          <v-text-field v-model="form.account_code" label="账户编码 *" hint="全局唯一，如 ACCT-GOLD-001" />
          <v-text-field v-model="form.account_name" label="账户名称 *" />
          <v-text-field v-model.number="form.initial_capital" label="初始启动资金 *" type="number" prefix="$" />
          <v-select v-model="form.currency_type" :items="['USD', 'CNY', 'HKD']" label="计价币种" />
          <v-row>
            <v-col cols="6">
              <v-text-field v-model.number="form.commission_rate" label="手续费率（按成交金额比例）" type="number"
                hint="0.0003 = 万分之三" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model.number="form.slippage_pct" label="滑点（按成交价比例）" type="number"
                hint="0.0001 = 万分之一" />
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="6">
              <v-select v-model="form.margin_mode" :items="[
                { title: '全额模式（股票类）', value: 'FULL' },
                { title: '期货保证金（预留）', value: 'FUTURES' },
              ]" label="保证金模式" />
            </v-col>
            <v-col cols="6">
              <v-text-field v-model.number="form.margin_rate" label="保证金比例" type="number" hint="FULL=1" />
            </v-col>
          </v-row>
          <v-switch v-model="form.allow_backtest" true-value="1" false-value="0" label="允许回测（回测开关）"
            color="primary" hide-details />
          <v-text-field v-model="form.remark" label="备注" class="mt-2" />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="dialog = false">取消</v-btn>
          <v-btn color="primary" @click="save">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </v-container>
</template>
