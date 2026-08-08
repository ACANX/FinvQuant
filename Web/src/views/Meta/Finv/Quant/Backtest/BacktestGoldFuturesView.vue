<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiGet, apiPost } from '../../../../../api'
import { fmtDate } from '../../../../../utils'

const router = useRouter()

// 任务状态 → ICON + 颜色（列表"状态"列用图标直观展示，替代文本）
const statusMeta: Record<string, { icon: string; color: string; spin?: boolean }> = {
  PENDING: { icon: 'mdi-clock-outline', color: 'grey' },
  RUNNING: { icon: 'mdi-loading', color: 'primary', spin: true },
  SUCCEEDED: { icon: 'mdi-check-circle', color: 'green' },
  FAILED: { icon: 'mdi-close-circle', color: 'red' },
  CANCELLED: { icon: 'mdi-cancel', color: 'orange' },
}

function statusIcon(s: string): string {
  return statusMeta[s]?.icon ?? 'mdi-help-circle-outline'
}

function statusIconColor(s: string): string {
  return statusMeta[s]?.color ?? 'grey'
}

function statusIconSpin(s: string): boolean {
  return statusMeta[s]?.spin ?? false
}

interface StrategyOption {
  strategy_id: string
  strategy_code: string
  strategy_name: string
  data_period: string
  secu_code: string
}

interface AccountOption {
  account_id: string
  account_code: string
  account_name: string
  initial_capital: number
  currency_type: string
}

interface EnvOption {
  env_id: string
  env_code: string
  env_name: string
  env_type: string
  region: string
  is_default: string
  allow_backtest: string
}

interface RunRow {
  run_id: string
  run_no: number
  strategy_name: string
  account_name: string
  secu_code: string
  period: string
  report_precision: string
  start_date: number
  end_date: number
  status: string
  progress: number
  error_message: string
  created_by: string
}

// 回测任务列表仅展示最近 10 条（不分页）；后续扩展多标的时按 secu_code 过滤
const runs = ref<RunRow[]>([])
const runPage = ref(1)
const runPageSize = ref(10)

const strategies = ref<StrategyOption[]>([])
const accounts = ref<AccountOption[]>([])
const environments = ref<EnvOption[]>([])

const strategyId = ref('')
const accountId = ref('')
const envId = ref('')
const secuCode = ref('GCMain')
const startDate = ref('')
const endDate = ref('')
const period = ref('Min')
const reportPrecision = ref('Day')
const enableBacktest = ref(true)
const initialCapital = ref<number | null>(null)
const useInitialCapital = ref(false)
const maxTradesPerDay = ref<number | null>(null)
const useMaxTrades = ref(false)
const allowedTimes = ref('')

const loading = ref(false)
const submitting = ref(false)
const cancelling = ref('')
const error = ref('')
const message = ref('')

let pollTimer: ReturnType<typeof setInterval> | null = null

function hasActiveRuns(): boolean {
  return runs.value.some((r) => r.status === 'PENDING' || r.status === 'RUNNING')
}

async function loadOptions() {
  try {
    const [s, a, e] = await Promise.all([
      apiGet<{ list: StrategyOption[] }>('/Meta/Finv/Quant/Backtest/Strategy/List?page=1&pageSize=100&allowBacktest=1'),
      apiGet<{ list: AccountOption[] }>('/Meta/Finv/Quant/Backtest/Account/List?page=1&pageSize=100&allowBacktest=1'),
      apiGet<{ list: EnvOption[] }>('/Meta/Finv/Quant/Backtest/Environment/List?page=1&pageSize=100&envType=BACKTEST'),
    ])
    strategies.value = s.list ?? []
    accounts.value = a.list ?? []
    environments.value = (e.list ?? []).filter((x) => x.allow_backtest === '1')
    // 默认选中 GCMain 相关策略与首个账户、默认环境
    if (!strategyId.value) {
      const gc = strategies.value.find((x) => x.secu_code === 'GCMain')
      strategyId.value = gc?.strategy_id ?? strategies.value[0]?.strategy_id ?? ''
      if (gc) period.value = gc.data_period || 'Min'
    }
    if (!accountId.value) accountId.value = accounts.value[0]?.account_id ?? ''
    if (!envId.value) {
      envId.value = environments.value.find((x) => x.is_default === '1')?.env_id ?? environments.value[0]?.env_id ?? ''
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadRuns() {
  loading.value = true
  try {
    // 仅查询最近 10 条（page=1&pageSize=10，按 run_no 倒序）
    const data = await apiGet<{ total: number; list: RunRow[] }>(
      `/Meta/Finv/Quant/Backtest/Run/List?page=1&pageSize=10&secuCode=${encodeURIComponent(secuCode.value)}`,
    )
    runs.value = data.list ?? []
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
  syncPolling()
}

// 轮询：存在 PENDING/RUNNING 任务时每 5s 自动刷新（评审：原实现仅手动刷新）
function syncPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (hasActiveRuns()) {
    pollTimer = setInterval(async () => {
      try {
        const data = await apiGet<{ list: RunRow[] }>(
          `/Meta/Finv/Quant/Backtest/Run/List?page=1&pageSize=10&secuCode=${encodeURIComponent(secuCode.value)}`,
        )
        runs.value = data.list ?? []
        if (!hasActiveRuns()) syncPolling() // 全部结束后停止轮询
      } catch {
        // 轮询失败静默，等待下一次
      }
    }, 5000)
  }
}

function toDateInt(v: string): number {
  return Number(v.replaceAll('-', '')) || 0
}

async function startBacktest() {
  error.value = ''
  message.value = ''
  if (!strategyId.value) {
    error.value = '请先选择策略（可在「策略管理」中创建）'
    return
  }
  if (!accountId.value) {
    error.value = '请先选择账户（可在「账户管理」中创建）'
    return
  }
  submitting.value = true
  try {
    const options: Record<string, unknown> = { enable_backtest: enableBacktest.value }
    if (useInitialCapital.value && initialCapital.value) options.initial_capital = initialCapital.value
    if (useMaxTrades.value && maxTradesPerDay.value) options.max_trades_per_day = maxTradesPerDay.value
    if (allowedTimes.value.trim()) {
      options.allowed_times = allowedTimes.value.split(/[,，\s]+/).filter(Boolean)
    }
    const run = await apiPost<RunRow>('/Meta/Finv/Quant/Backtest/Run/Create', {
      strategy_id: strategyId.value,
      account_id: accountId.value,
      env_id: envId.value || undefined,
      secu_code: secuCode.value || undefined,
      start_date: toDateInt(startDate.value) || undefined,
      end_date: toDateInt(endDate.value) || undefined,
      period: period.value,
      report_precision: reportPrecision.value,
      options,
    })
    message.value = `回测任务 #${run.run_no} 已创建并启动（${run.status}），执行中请稍后刷新查看报告`
    await loadRuns()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    submitting.value = false
  }
}

async function cancelRun(run: RunRow) {
  if (!confirm(`确认取消回测任务 #${run.run_no}？`)) return
  cancelling.value = run.run_id
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Run/Cancel', { run_id: run.run_id })
    message.value = `任务 #${run.run_no} 取消请求已受理`
    await loadRuns()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    cancelling.value = ''
  }
}

function viewReport(run: RunRow) {
  router.push({ path: '/Meta/Finv/Quant/Backtest/Analysis/Report', query: { runId: run.run_id } })
}

// 回测区间默认值：结束日期 = 上个月第一天；开始日期 = 结束日期往前推一年（12 个月前同日）
function initDefaultDates() {
  const now = new Date()
  // 上个月第一天（如当前 2026-08-07 → 2026-07-01）
  const end = new Date(now.getFullYear(), now.getMonth() - 1, 1)
  // 开始日期 = 结束日期往前 12 个月（如 2026-07-01 → 2025-07-01）
  const start = new Date(end.getFullYear(), end.getMonth() - 12, 1)
  const fmt = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`
  endDate.value = fmt(end)
  startDate.value = fmt(start)
}

onMounted(async () => {
  initDefaultDates()
  await loadOptions()
  await loadRuns()
})

onBeforeUnmount(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<style scoped>
/* RUNNING 状态图标旋转动画 */
.spin {
  animation: finv-icon-spin 1.2s linear infinite;
}
@keyframes finv-icon-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>

<template>
  <v-container fluid>
    <!-- 操作引导条：用图标分步骤引导用户完成回测 -->
    <v-card variant="tonal" color="primary" class="mb-3">
      <v-card-text class="py-3 d-flex flex-wrap align-center ga-4">
        <v-icon icon="mdi-gold" color="primary" />
        <div class="text-body-2 font-weight-medium">
          黄金期货回测：按下面 4 步快速验证策略
        </div>
        <v-divider vertical class="mx-2" />
        <div class="d-flex flex-column ga-1">
          <span class="text-caption text-medium-emphasis">① 选择策略 · ② 选择账户 · ③ 配置条件 · ④ 启动回测</span>
          <span class="text-caption text-medium-emphasis">
            任务完成后点击「查看报告」查看收益分析 · 引擎亦支持 ETF / 股票 / 期货等任意已导入行情
          </span>
        </div>
      </v-card-text>
    </v-card>

    <v-row>
      <!-- 回测条件配置（上） -->
      <v-col cols="12">
        <v-card>
          <v-card-title class="d-flex align-center">
            <v-icon icon="mdi-tune-variant" class="mr-2" color="primary" />回测条件配置
          </v-card-title>
          <v-card-text>
            <v-select v-model="strategyId" :items="strategies.map((s) => ({
              title: `${s.strategy_code} ${s.strategy_name}（${s.secu_code || '通用'}）`,
              value: s.strategy_id,
            }))" label="交易策略 *" hint="策略在「策略管理」中维护（结构化定义）" class="mb-2" @update:model-value="(v: string) => {
              const s = strategies.find((x) => x.strategy_id === v)
              if (s) { if (s.secu_code) secuCode = s.secu_code; if (s.data_period) period = s.data_period }
            }" />
            <v-select v-model="accountId" :items="accounts.map((a) => ({
              title: `${a.account_code} ${a.account_name}（${a.initial_capital.toLocaleString()} ${a.currency_type}）`,
              value: a.account_id,
            }))" label="回测账户 *" hint="初始资金/手续费/滑点在「账户管理」中维护" class="mb-2" />

            <v-select v-model="envId" :items="environments.map((e) => ({
              title: `${e.env_code} ${e.env_name}（${e.region || '通用'}）${e.is_default === '1' ? ' · 默认' : ''}`,
              value: e.env_id,
            }))" label="回测环境" hint="交易时段/规则/成本自适应，在「环境与模板管理」中维护" class="mb-2" />

            <v-text-field v-model="secuCode" label="回测标的证券代码" hint="如 GCMain（黄金期货主连）/ 518880 / NVDA" class="mb-2" />

            <v-row>
              <v-col cols="6">
                <v-text-field v-model="startDate" label="开始日期" type="date" hint="默认：结束日期前推一年" />
              </v-col>
              <v-col cols="6">
                <v-text-field v-model="endDate" label="结束日期" type="date" hint="默认：上个月第一天" />
              </v-col>
            </v-row>

            <v-row>
              <v-col cols="6">
                <v-select v-model="period" :items="[
                  { title: '分钟（Min）', value: 'Min' },
                  { title: '小时（Hour）', value: 'Hour' },
                  { title: '日（Day）', value: 'Day' },
                ]" label="回测数据周期" />
              </v-col>
              <v-col cols="6">
                <v-select v-model="reportPrecision" :items="[
                  { title: '日（Day）', value: 'Day' },
                  { title: '小时（Hour）', value: 'Hour' },
                  { title: '分钟（Min）', value: 'Min' },
                ]" label="报告时间精度" hint="曲线数据粒度" />
              </v-col>
            </v-row>

            <v-divider class="my-3" />
            <v-card-subtitle class="pa-0 mb-2">限制条件（可选覆盖）</v-card-subtitle>
            <v-row align="center">
              <v-col cols="1">
                <!-- 开关规范样式：开启绿色 / 关闭灰色（与账户管理表格回测开关一致） -->
                <v-switch v-model="useInitialCapital" density="compact" hide-details
                  :color="useInitialCapital ? 'green' : 'grey'" />
              </v-col>
              <v-col cols="11">
                <v-text-field v-model.number="initialCapital" label="初始资金覆盖（覆盖账户初始资金）" type="number"
                  :disabled="!useInitialCapital" />
              </v-col>
            </v-row>
            <v-row align="center">
              <v-col cols="1">
                <v-switch v-model="useMaxTrades" density="compact" hide-details
                  :color="useMaxTrades ? 'green' : 'grey'" />
              </v-col>
              <v-col cols="11">
                <v-text-field v-model.number="maxTradesPerDay" label="每日最大成交笔数限制" type="number"
                  :disabled="!useMaxTrades" />
              </v-col>
            </v-row>
            <v-text-field v-model="allowedTimes" label="限定交易时间点（hhmmss，逗号分隔，空=不限）"
              hint="如 09:30:00,14:00:00 填 093000,140000" />

            <v-divider class="my-3" />
            <v-row align="center">
              <v-col cols="auto">
                <v-switch v-model="enableBacktest" density="compact" hide-details
                  :color="enableBacktest ? 'green' : 'grey'"
                  :label="enableBacktest ? '回测开关：已开启' : '回测开关：已关闭'" />
              </v-col>
              <v-spacer />
              <v-btn color="primary" size="large" :loading="submitting" :disabled="!enableBacktest"
                prepend-icon="mdi-play" @click="startBacktest">
                启动回测
              </v-btn>
            </v-row>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- 回测任务列表（下，仅展示最近 10 条） -->
      <v-col cols="12">
        <v-card>
          <v-card-title class="d-flex align-center">
            <v-icon icon="mdi-history" class="mr-2" color="primary" />回测任务（{{ secuCode }} · 最近 10 条）
            <v-spacer />
            <v-btn size="small" variant="tonal" prepend-icon="mdi-refresh" @click="loadRuns">刷新</v-btn>
          </v-card-title>
          <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
          <v-alert v-if="message" type="success" dismissible class="mx-4 mb-2">{{ message }}</v-alert>

          <v-data-table-server v-model:page="runPage" v-model:items-per-page="runPageSize"
            :headers="[
              { title: '任务号', key: 'run_no', width: 80 },
              { title: '策略', key: 'strategy_name' },
              { title: '账户', key: 'account_name', width: 130 },
              { title: '区间', key: 'range', width: 190 },
              { title: '周期', key: 'period', width: 70 },
              { title: '精度', key: 'report_precision', width: 70 },
              { title: '状态', key: 'status', width: 90 },
              { title: '进度', key: 'progress', width: 130 },
              { title: '操作', key: 'actions', width: 150, sortable: false },
            ]" :items="runs" :loading="loading" item-value="run_id"
            hide-default-footer :items-length="runs.length" @update:options="loadRuns">
            <template #item.range="{ item }">
              {{ fmtDate(item.start_date) }} ~ {{ fmtDate(item.end_date) }}
            </template>
            <template #item.status="{ item }">
              <v-tooltip location="top">
                <template #activator="{ props }">
                  <v-icon :icon="statusIcon(item.status)" :color="statusIconColor(item.status)"
                    :class="statusIconSpin(item.status) ? 'spin' : ''" v-bind="props" />
                </template>
                <span>{{ item.status }}</span>
              </v-tooltip>
              <div v-if="item.error_message" class="text-caption text-error mt-1">{{ item.error_message }}</div>
            </template>
            <template #item.progress="{ item }">
              <v-progress-linear v-if="item.status === 'RUNNING' || item.status === 'PENDING'"
                :model-value="item.progress" color="primary" height="8" rounded class="mt-2" />
              <span v-else class="text-body-2">{{ item.progress }}%</span>
            </template>
            <template #item.actions="{ item }">
              <v-btn v-if="item.status === 'RUNNING' || item.status === 'PENDING'" size="small" variant="text"
                color="warning" :loading="cancelling === item.run_id" @click="cancelRun(item)">取消</v-btn>
              <v-btn size="small" variant="text" color="primary" :disabled="item.status !== 'SUCCEEDED'"
                @click="viewReport(item)">查看报告</v-btn>
            </template>
          </v-data-table-server>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>
