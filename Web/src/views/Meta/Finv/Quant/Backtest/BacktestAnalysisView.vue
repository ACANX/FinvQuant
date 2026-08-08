<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { apiGet, apiPost } from '../../../../../api'
import { fmtDate, fmtNum, fmtPct } from '../../../../../utils'

// 任务状态 → ICON + 颜色（列表"状态"列用图标直观展示，替代文本，与黄金回测验证页一致）
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

const route = useRoute()
const router = useRouter()

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
  report: Report | null
}

interface Report {
  secu_code: string
  period: string
  report_precision: string
  start_date: number
  end_date: number
  bar_count: number
  initial_capital: number
  final_equity: number
  total_profit: number
  total_return_pct: number
  annual_return_pct: number
  max_drawdown_pct: number
  sharpe_ratio: number
  volatility_pct: number
  trade_count: number
  event_stats: EventStats | null
}

interface EventStats {
  trigger_count: number
  filled_count: number
  rejected_count: number
  expired_count: number
  pending_count: number
  reject_reasons: Record<string, number>
}

const runs = ref<RunRow[]>([])
const runTotal = ref(0)
const page = ref(1)
const pageSize = ref(10)
const statusFilter = ref('')
const secuFilter = ref('')
const keyword = ref('')
const loading = ref(false)
const error = ref('')
const message = ref('')
const cancelling = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null

// K 线周期 / 报告精度可读化（与报告页保持一致）
function periodName(p: string | undefined): string {
  return { Min: '分钟', Hour: '小时', Day: '日线' }[p ?? ''] ?? (p ?? '-')
}

async function loadRuns() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
    })
    if (statusFilter.value) params.set('status', statusFilter.value)
    if (secuFilter.value) params.set('secuCode', secuFilter.value)
    if (keyword.value) params.set('keyword', keyword.value)
    const data = await apiGet<{ total: number; list: RunRow[] }>(`/Meta/Finv/Quant/Backtest/Run/List?${params.toString()}`)
    runs.value = data.list ?? []
    runTotal.value = data.total ?? 0
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
  syncPolling()
}

// 轮询：存在执行中/待执行任务时每 5s 自动刷新列表与进度
function syncPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (runs.value.some((r) => r.status === 'PENDING' || r.status === 'RUNNING')) {
    pollTimer = setInterval(async () => {
      try {
        const params = new URLSearchParams({ page: String(page.value), pageSize: String(pageSize.value) })
        if (statusFilter.value) params.set('status', statusFilter.value)
        if (secuFilter.value) params.set('secuCode', secuFilter.value)
        if (keyword.value) params.set('keyword', keyword.value)
        const data = await apiGet<{ total: number; list: RunRow[] }>(`/Meta/Finv/Quant/Backtest/Run/List?${params.toString()}`)
        runs.value = data.list ?? []
        runTotal.value = data.total ?? 0
        if (!runs.value.some((r) => r.status === 'PENDING' || r.status === 'RUNNING')) syncPolling()
      } catch {
        // 轮询失败静默，等待下一次
      }
    }, 5000)
  }
}

async function cancelRun(run: RunRow) {
  if (!confirm(`确认取消回测任务 #${run.run_no}？`)) return
  cancelling.value = run.run_id
  try {
    await apiPost('/Meta/Finv/Quant/Backtest/Run/Cancel', { run_id: run.run_id })
    await loadRuns()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    cancelling.value = ''
  }
}

// 提交删除任务（异步执行）：删除任务及其全部明细，策略/账户/环境/模板保留
async function deleteRun(run: RunRow) {
  const msg = `确认删除回测任务 #${run.run_no}？\n将删除该任务的净值曲线/成交/资金流水/持仓/事件追踪等全部记录（不可恢复）。\n策略、账户、环境、模板保留不受影响。`
  if (!confirm(msg)) return
  try {
    const res = await apiPost<{ del_task_id: string }>('/Meta/Finv/Quant/Backtest/Run/Delete', { run_id: run.run_id })
    message.value = `删除任务已提交（${res.del_task_id}），后台异步执行中`
    // 跳转到删除任务管理页查看进度与日志
    router.push({ path: '/Meta/Finv/Quant/Backtest/RunDeleteTasks' })
  } catch (e) {
    error.value = (e as Error).message
  }
}

// 点击 SUCCEEDED 任务 → 跳转独立报告页
function openRun(run: RunRow) {
  if (run.status !== 'SUCCEEDED') return
  router.push({ path: '/Meta/Finv/Quant/Backtest/Analysis/Report', query: { runId: run.run_id } })
}

onMounted(async () => {
  await loadRuns()
  // 深链 ?runId=：直接跳转报告页
  const q = route.query.runId as string | undefined
  if (q) {
    router.replace({ path: '/Meta/Finv/Quant/Backtest/Analysis/Report', query: { runId: q } })
  }
})

onBeforeUnmount(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-chart-timeline-variant" class="mr-2" color="primary" />
        回测任务列表
        <v-chip size="small" class="ml-2">共 {{ runTotal }} 个任务</v-chip>
        <v-spacer />
        <v-btn size="small" variant="tonal" prepend-icon="mdi-refresh" @click="loadRuns">刷新</v-btn>
      </v-card-title>

      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-alert v-if="message" type="success" dismissible class="mx-4 mb-2">{{ message }}</v-alert>

      <v-card-text class="pt-0">
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-select v-model="statusFilter" :items="[
              { title: '全部状态', value: '' },
              { title: '待执行', value: 'PENDING' },
              { title: '执行中', value: 'RUNNING' },
              { title: '成功', value: 'SUCCEEDED' },
              { title: '失败', value: 'FAILED' },
              { title: '已取消', value: 'CANCELLED' },
            ]" density="compact" hide-details label="状态" @update:model-value="loadRuns" />
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-text-field v-model="secuFilter" label="标的过滤" density="compact" hide-details
              @keyup.enter="loadRuns" @blur="loadRuns" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="keyword" label="关键字过滤（策略/账户/任务号）" density="compact" hide-details
              clearable @keyup.enter="loadRuns" @blur="loadRuns" />
          </v-col>
          <v-col cols="12" sm="6" md="2" class="d-flex align-center">
            <v-btn color="primary" variant="tonal" prepend-icon="mdi-filter" @click="loadRuns">查询</v-btn>
          </v-col>
        </v-row>
      </v-card-text>

      <v-data-table-server v-model:page="page" v-model:items-per-page="pageSize" :loading="loading"
        :headers="[
          { title: '任务号', key: 'run_no', width: 90 },
          { title: '策略', key: 'strategy_name' },
          { title: '账户', key: 'account_name', width: 140 },
          { title: '标的', key: 'secu_code', width: 100 },
          { title: '区间', key: 'range', width: 110 },
          { title: '周期', key: 'period', width: 80 },
          { title: '精度', key: 'report_precision', width: 80 },
          { title: '状态', key: 'status', width: 70 },
          { title: '进度', key: 'progress', width: 130 },
          { title: '报告摘要', key: 'summary', width: 150 },
          { title: '操作', key: 'actions', width: 132, sortable: false },
        ]" :items="runs" :items-length="runTotal" item-value="run_id"
        @update:options="loadRuns">
        <template #item.range="{ item }">
          <!-- 区间三行展示：开始日期 / ~ / 结束日期，避免单行占用过宽 -->
          <div class="d-flex flex-column align-center" style="line-height: 1.25">
            <span class="text-caption">{{ fmtDate(item.start_date) }}</span>
            <span class="text-caption text-medium-emphasis">~</span>
            <span class="text-caption">{{ fmtDate(item.end_date) }}</span>
          </div>
        </template>
        <template #item.period="{ item }">
          <v-chip size="x-small">{{ periodName(item.period) }}</v-chip>
        </template>
        <template #item.report_precision="{ item }">
          <v-chip size="x-small" variant="tonal">{{ periodName(item.report_precision) }}</v-chip>
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
        <template #item.summary="{ item }">
          <template v-if="item.status === 'SUCCEEDED' && item.report">
            <div class="text-caption text-truncate" style="line-height: 1.4">
              期末 <span class="font-weight-medium">{{ fmtNum(item.report.final_equity) }}</span>
            </div>
            <div class="text-caption text-truncate" style="line-height: 1.4"
              :class="item.report.total_return_pct >= 0 ? 'text-success' : 'text-error'">
              收益率 {{ fmtPct(item.report.total_return_pct) }}
            </div>
            <div class="text-caption text-truncate" style="line-height: 1.4">
              {{ item.report.trade_count }} 笔成交
            </div>
          </template>
          <span v-else class="text-caption text-medium-emphasis">-</span>
        </template>
        <template #item.actions="{ item }">
          <!-- d-flex 默认 flex-wrap:nowrap，强制操作图标水平一行，避免列窄时被挤成上下排列 -->
          <div class="d-flex align-center flex-nowrap ga-1">
            <v-btn v-if="item.status === 'PENDING' || item.status === 'RUNNING'" size="small" variant="text"
              color="warning" :loading="cancelling === item.run_id" title="取消任务"
              @click="cancelRun(item)">
              <v-icon icon="mdi-stop-circle-outline" size="18" />
            </v-btn>
            <!-- 查看报告：仅 SUCCEEDED 可点，ICON + hover 提示节省列宽 -->
            <v-tooltip location="top">
              <template #activator="{ props }">
                <v-btn size="small" variant="text" :disabled="item.status !== 'SUCCEEDED'"
                  :class="item.status === 'SUCCEEDED' ? '' : 'opacity-40'" v-bind="props"
                  @click="openRun(item)">
                  <v-icon icon="mdi-file-chart-outline" size="18" color="primary" />
                </v-btn>
              </template>
              <span>点击查看报告</span>
            </v-tooltip>
            <!-- 删除任务：仅已结束任务可删（SUCCEEDED/FAILED/CANCELLED） -->
            <v-btn size="small" variant="text" color="error"
              :disabled="item.status === 'PENDING' || item.status === 'RUNNING'"
              title="删除任务及其全部明细（异步）"
              @click="deleteRun(item)">
              <v-icon icon="mdi-delete-outline" size="18" />
            </v-btn>
          </div>
        </template>
      </v-data-table-server>
      <v-pagination v-model="page" :length="Math.max(1, Math.ceil(runTotal / pageSize))" density="compact"
        class="mt-2" @update:model-value="loadRuns" />
    </v-card>
  </v-container>
</template>

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
