<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiGet, apiPost } from '../../../../../api'

// 环境行
interface EnvRow {
  env_id: string
  env_code: string
  env_name: string
  env_type: string
  region: string
  market_code: number
  config: EnvConfig
  user_id: string
  is_default: string
  allow_backtest: string
  status: string
  description: string
}

interface EnvConfig {
  trading_sessions: { start: string; end: string }[]
  trading_rules: { t_plus: number; tick_size: number; contract_multiplier: number; limit_up_pct: number; limit_down_pct: number }
  cost: { commission_rate: number; slippage_pct: number }
  fill_mode: string
  currency: string
  preferences: Record<string, unknown>
}

const envs = ref<EnvRow[]>([])
const envTotal = ref(0)
const envPage = ref(1)
const envPageSize = ref(10)
const envTypeFilter = ref('')
const envLoading = ref(false)

// 吐司提示（v-snackbar）：成功/错误消息短暂展示后自动消失，替代内嵌 v-alert
const toast = ref<{ text: string; type: 'success' | 'error'; visible: boolean }>({ text: '', type: 'success', visible: false })
function showToast(text: string, type: 'success' | 'error' = 'success') {
  toast.value = { text, type, visible: true }
}

const envDialog = ref(false)
const envEditing = ref(false)
const envForm = ref<EnvRow>(emptyEnv())
const envConfigText = ref('')

function emptyEnv(): EnvRow {
  return {
    env_id: '', env_code: '', env_name: '', env_type: 'BACKTEST', region: '', market_code: 0,
    config: {
      trading_sessions: [{ start: '093000', end: '150000' }],
      trading_rules: { t_plus: 0, tick_size: 0.01, contract_multiplier: 1, limit_up_pct: 0, limit_down_pct: 0 },
      cost: { commission_rate: 0.0003, slippage_pct: 0.0001 },
      fill_mode: 'NEXT_BAR_OPEN', currency: 'USD', preferences: {},
    },
    user_id: 'default', is_default: '0', allow_backtest: '1', status: 'ENABLED', description: '',
  }
}

async function loadEnvs() {
  envLoading.value = true
  try {
    const q = new URLSearchParams({ page: String(envPage.value), pageSize: String(envPageSize.value) })
    if (envTypeFilter.value) q.set('envType', envTypeFilter.value)
    const data = await apiGet<{ total: number; list: EnvRow[] }>(`/Meta/Finv/Quant/Backtest/Environment/List?${q.toString()}`)
    envs.value = data.list ?? []
    envTotal.value = data.total ?? 0
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    envLoading.value = false
  }
}

function openEnvCreate() {
  envEditing.value = false
  envForm.value = emptyEnv()
  envConfigText.value = JSON.stringify(envForm.value.config, null, 2)
  envDialog.value = true
}

async function openEnvEdit(row: EnvRow) {
  envEditing.value = true
  // 通过 Environment/Get 拉取最新详情（评审：Get 端点此前未被前端调用）
  try {
    const detail = await apiGet<EnvRow>(`/Meta/Finv/Quant/Backtest/Environment/Get?envId=${row.env_id}`)
    envForm.value = { ...(detail ?? row), config: JSON.parse(JSON.stringify((detail ?? row).config ?? {})) }
  } catch (e) {
    showToast((e as Error).message, 'error')
    envForm.value = { ...row, config: JSON.parse(JSON.stringify(row.config ?? {})) }
  }
  envConfigText.value = JSON.stringify(envForm.value.config, null, 2)
  envDialog.value = true
}

async function saveEnv() {
  try {
    envForm.value.config = JSON.parse(envConfigText.value)
  } catch (e) {
    showToast('环境配置 JSON 格式错误：' + (e as Error).message, 'error')
    return
  }
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Environment/Save', envForm.value)
    showToast('环境保存成功')
    envDialog.value = false
    await loadEnvs()
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

async function toggleEnv(row: EnvRow) {
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Environment/Toggle', { env_id: row.env_id, allow_backtest: row.allow_backtest === '1' ? '0' : '1' })
    await loadEnvs()
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

async function removeEnv(row: EnvRow) {
  if (!confirm(`确认删除环境「${row.env_name}」？`)) return
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Environment/Delete', { env_id: row.env_id })
    showToast('环境删除成功')
    await loadEnvs()
  } catch (e) {
    showToast((e as Error).message, 'error')
  }
}

function envTypeName(t: string): string {
  return { BACKTEST: '回测', PAPER: '模拟盘', SIMULATION: '仿真', LIVE: '实盘' }[t] ?? t
}

function envTypeColor(t: string): string {
  return { BACKTEST: 'primary', PAPER: 'teal', SIMULATION: 'purple', LIVE: 'red' }[t] ?? 'grey'
}

onMounted(() => {
  loadEnvs()
})
</script>

<template>
  <v-container fluid>
    <!-- 环境管理 -->
    <v-card class="mb-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-application-cog-outline" class="mr-2" color="primary" />
        环境管理
        <v-chip size="small" class="ml-2" variant="tonal">回测 / 模拟盘 / 仿真 / 实盘 × 地区市场</v-chip>
        <v-spacer />
        <v-select v-model="envTypeFilter" :items="[
          { title: '全部类型', value: '' },
          { title: '回测', value: 'BACKTEST' },
          { title: '模拟盘', value: 'PAPER' },
          { title: '仿真', value: 'SIMULATION' },
          { title: '实盘', value: 'LIVE' },
        ]" density="compact" hide-details style="max-width: 160px" class="mr-2" @update:model-value="loadEnvs" />
        <v-btn size="small" variant="tonal" class="mr-2" @click="loadEnvs">刷新</v-btn>
        <v-btn color="primary" @click="openEnvCreate">新建环境</v-btn>
      </v-card-title>
      <v-card-text class="pt-0">
        <v-data-table-server v-model:page="envPage" v-model:items-per-page="envPageSize" :headers="[
          { title: '编码', key: 'env_code', width: 150 },
          { title: '名称', key: 'env_name' },
          { title: '类型', key: 'env_type', width: 90 },
          { title: '地区', key: 'region', width: 70 },
          { title: '交易时段', key: 'sessions', width: 170 },
          { title: '最小变动价位', key: 'tick', width: 110 },
          { title: '默认', key: 'is_default', width: 70 },
          { title: '回测开关', key: 'allow_backtest', width: 100 },
          { title: '操作', key: 'actions', width: 170, sortable: false },
        ]" :items="envs" :loading="envLoading" :items-length="envTotal" item-value="env_id" @update:options="loadEnvs">
          <template #item.env_type="{ item }">
            <v-chip size="small" :color="envTypeColor(item.env_type)">{{ envTypeName(item.env_type) }}</v-chip>
          </template>
          <template #item.sessions="{ item }">
            <span class="text-caption">{{ (item.config?.trading_sessions ?? []).map((s: { start: string; end: string }) => `${s.start.slice(0, 2)}:${s.start.slice(2, 4)}-${s.end.slice(0, 2)}:${s.end.slice(2, 4)}`).join(' / ') || '-' }}</span>
          </template>
          <template #item.tick="{ item }">
            {{ item.config?.trading_rules?.tick_size ?? '-' }}
          </template>
          <template #item.is_default="{ item }">
            <v-chip v-if="item.is_default === '1'" size="x-small" color="primary">默认</v-chip>
            <span v-else>-</span>
          </template>
          <template #item.allow_backtest="{ item }">
            <v-switch :model-value="item.allow_backtest === '1'" density="compact" hide-details
              :color="item.allow_backtest === '1' ? 'green' : 'grey'" @update:model-value="toggleEnv(item)" />
          </template>
          <template #item.actions="{ item }">
            <v-btn size="small" variant="text" @click="openEnvEdit(item)">编辑</v-btn>
            <v-btn size="small" variant="text" color="error" @click="removeEnv(item)">删除</v-btn>
          </template>
        </v-data-table-server>
      </v-card-text>
    </v-card>

    <!-- 环境编辑对话框 -->
    <v-dialog v-model="envDialog" max-width="760">
      <v-card>
        <v-card-title>{{ envEditing ? '编辑环境' : '新建环境' }}</v-card-title>
        <v-card-text>
          <v-row>
            <v-col cols="6"><v-text-field v-model="envForm.env_code" label="环境编码 *" hint="如 ENV-BT-COMEX-GC" /></v-col>
            <v-col cols="6"><v-text-field v-model="envForm.env_name" label="环境名称 *" /></v-col>
          </v-row>
          <v-row>
            <v-col cols="4">
              <v-select v-model="envForm.env_type" :items="[
                { title: '回测', value: 'BACKTEST' }, { title: '模拟盘', value: 'PAPER' },
                { title: '仿真', value: 'SIMULATION' }, { title: '实盘', value: 'LIVE' },
              ]" label="环境类型" />
            </v-col>
            <v-col cols="4"><v-text-field v-model="envForm.region" label="地区" hint="CN/US/HK..." /></v-col>
            <v-col cols="4">
              <v-switch v-model="envForm.is_default" true-value="1" false-value="0" label="设为默认环境" hide-details class="mt-3" />
            </v-col>
          </v-row>
          <v-text-field v-model="envForm.description" label="说明" />
          <v-textarea v-model="envConfigText" label="环境配置（JSON：trading_sessions / trading_rules / cost / fill_mode / currency / preferences）"
            rows="12" style="font-family: monospace" spellcheck="false" />
          <v-alert type="info" variant="tonal" density="compact">
            成本覆盖链：环境 &gt; 任务 &gt; 策略 &gt; 账户；trading_sessions 控制交易时段（hhmmss），
            trading_rules.tick_size（最小变动价位）控制价格最小变动单位，引擎自动对齐。
          </v-alert>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="envDialog = false">取消</v-btn>
          <v-btn color="primary" @click="saveEnv">保存</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- 环境能力介绍（表格下方独立区域） -->
    <v-card variant="outlined" class="mt-4">
      <v-card-title class="text-subtitle-1 d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" color="primary" />
        环境配置说明
      </v-card-title>
      <v-card-text>
        <v-row>
          <v-col cols="12" md="6">
            <div class="d-flex mb-2">
              <v-icon icon="mdi-tune-variant" color="primary" class="mr-2" />
              <div>
                <div class="text-body-2 font-weight-medium">环境配置驱动引擎自适应</div>
                <div class="text-caption text-medium-emphasis">
                  交易时段 / 交易规则 / 成本基准 / 撮合模式 / 币种 / 地区偏好等配置，
                  驱动回测引擎自适应不同市场（黄金期货 / ETF / 股票等）。
                </div>
              </div>
            </div>
            <div class="d-flex">
              <v-icon icon="mdi-link-variant" color="primary" class="mr-2" />
              <div>
                <div class="text-body-2 font-weight-medium">成本覆盖链</div>
                <div class="text-caption text-medium-emphasis">
                  环境 &gt; 任务 &gt; 策略 &gt; 账户：环境成本基准优先级最高，引擎按此解析。
                </div>
              </div>
            </div>
          </v-col>
          <v-col cols="12" md="6">
            <div class="d-flex">
              <v-icon icon="mdi-camera-sync-outline" color="primary" class="mr-2" />
              <div>
                <div class="text-body-2 font-weight-medium">动态切换与快照</div>
                <div class="text-caption text-medium-emphasis">
                  回测任务创建时可动态选择环境，任务保存环境快照（environment_snapshot），
                  保证历史结果可复现。
                </div>
              </div>
            </div>
            <v-divider class="my-3" />
            <div class="d-flex">
              <v-icon icon="mdi-database-clock-outline" color="primary" class="mr-2" />
              <div>
                <div class="text-body-2 font-weight-medium">环境类型</div>
                <div class="text-caption text-medium-emphasis">
                  回测（BACKTEST）/ 模拟盘（PAPER）/ 仿真（SIMULATION）/ 实盘（LIVE）四类，
                  覆盖从策略验证到实盘的全链路。
                </div>
              </div>
            </div>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <!-- 吐司提示：保存/删除成功等短暂展示后自动消失 -->
    <v-snackbar v-model="toast.visible" :timeout="3000" :color="toast.type === 'success' ? 'success' : 'error'"
      location="top" rounded="lg">
      <v-icon :icon="toast.type === 'success' ? 'mdi-check-circle' : 'mdi-alert-circle'" class="mr-2" />
      {{ toast.text }}
      <template #actions>
        <v-btn variant="text" icon="mdi-close" @click="toast.visible = false" />
      </template>
    </v-snackbar>
  </v-container>
</template>
