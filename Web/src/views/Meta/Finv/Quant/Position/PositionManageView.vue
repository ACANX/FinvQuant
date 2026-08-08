<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { apiGet } from '../../../../../api'
import { fmtDate, fmtTime } from '../../../../../utils'

interface RunRow {
  run_id: string
  run_no: number
  strategy_name: string
  secu_code: string
  period: string
  start_date: number
  end_date: number
  status: string
}

interface EquityPoint {
  seq: number
  ts: number
  date: number
  time: number
  equity: number
  position_value: number
  position_qty: number
}

interface TradeRow {
  trade_id: number
  date: number
  time: number
  action: string
  price: number
  qty: number
  amount: number
  signal: string
}

const runs = ref<RunRow[]>([])
const runId = ref('')
const points = ref<EquityPoint[]>([])
const trades = ref<TradeRow[]>([])
const loading = ref(false)
const error = ref('')
let chart: echarts.ECharts | null = null
const chartEl = ref<HTMLDivElement | null>(null)


async function loadRuns() {
  try {
    const data = await apiGet<{ list: RunRow[] }>(
      '/Meta/Finv/Quant/Backtest/Run/List?page=1&pageSize=100&status=SUCCEEDED',
    )
    runs.value = data.list ?? []
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function loadPosition() {
  if (!runId.value) return
  loading.value = true
  error.value = ''
  chart?.dispose()
  chart = null
  try {
    const [eq, tr] = await Promise.all([
      apiGet<{ list: EquityPoint[] }>(`/Meta/Finv/Quant/Backtest/Run/Equity?runId=${runId.value}&page=1&pageSize=5000`),
      apiGet<{ list: TradeRow[] }>(`/Meta/Finv/Quant/Backtest/Run/Trades?runId=${runId.value}&page=1&pageSize=500`),
    ])
    points.value = eq.list ?? []
    trades.value = tr.list ?? []
    await nextTick()
    render()
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

function render() {
  if (!chartEl.value || !points.value.length) return
  chart = echarts.init(chartEl.value)
  const labels = points.value.map((p) => {
    const t = String(p.time).padStart(6, '0')
    return p.time ? `${fmtDate(p.date)} ${t.slice(0, 2)}:${t.slice(2, 4)}` : fmtDate(p.date)
  })
  chart.setOption({
    title: { text: '持仓管理 · 持仓数量与持仓市值曲线', left: 12, textStyle: { fontSize: 13, fontWeight: 600 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['持仓市值', '持仓数量'], top: 4, right: 12 },
    grid: { left: 80, right: 24, top: 44, bottom: 30 },
    xAxis: { type: 'category', data: labels, axisLabel: { fontSize: 10 } },
    yAxis: [
      { type: 'value', scale: true, name: '市值' },
      { type: 'value', scale: true, name: '数量' },
    ],
    series: [
      { name: '持仓市值', type: 'line', data: points.value.map((p) => p.position_value), showSymbol: false, itemStyle: { color: '#6a1b9a' }, areaStyle: { opacity: 0.06 } },
      { name: '持仓数量', type: 'line', yAxisIndex: 1, data: points.value.map((p) => p.position_qty), showSymbol: false, itemStyle: { color: '#fb8c00' }, lineStyle: { type: 'dashed' } },
    ],
  })
}

function resize() {
  chart?.resize()
}

onMounted(async () => {
  await loadRuns()
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart?.dispose()
})
</script>

<template>
  <v-container fluid>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-briefcase-variant-outline" class="mr-2" color="primary" />持仓管理（回测持仓数据）
        <v-spacer />
        <v-select v-model="runId" :items="runs.map((r) => ({
          title: `#${r.run_no} ${r.strategy_name}（${r.secu_code} ${fmtDate(r.start_date)}~${fmtDate(r.end_date)}）`,
          value: r.run_id,
        }))" label="选择回测任务（已成功）" hide-details style="max-width: 520px" class="mr-2"
          @update:model-value="loadPosition" />
        <v-btn color="primary" variant="tonal" @click="loadPosition">加载</v-btn>
      </v-card-title>
      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-card-text>
        <v-skeleton-loader v-if="loading" type="image" height="320" />
        <div v-else ref="chartEl" style="width: 100%; height: 380px" />

        <v-divider class="my-4" />
        <v-card-title class="text-subtitle-1 pa-0 mb-2">
          <v-icon icon="mdi-swap-horizontal" class="mr-1" color="primary" />开平仓记录（最多 500 条）
        </v-card-title>
        <v-table density="compact">
          <thead>
            <tr>
              <th>时间</th><th>方向</th><th>价格</th><th>数量</th><th>金额</th><th>信号</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in trades" :key="t.trade_id">
              <td>{{ fmtDate(t.date) }} {{ fmtTime(t.time) }}</td>
              <td>
                <v-chip size="x-small" :color="t.action === 'BUY' ? 'red' : 'green'">
                  {{ t.action === 'BUY' ? '买入' : '卖出' }}
                </v-chip>
              </td>
              <td>{{ Number(t.price).toFixed(2) }}</td>
              <td>{{ Number(t.qty).toFixed(4) }}</td>
              <td>{{ Number(t.amount).toLocaleString('en-US', { maximumFractionDigits: 2 }) }}</td>
              <td>{{ t.signal }}</td>
            </tr>
            <tr v-if="!trades.length">
              <td colspan="6" class="text-center text-medium-emphasis py-4">暂无成交记录</td>
            </tr>
          </tbody>
        </v-table>
      </v-card-text>
    </v-card>
  </v-container>
</template>
