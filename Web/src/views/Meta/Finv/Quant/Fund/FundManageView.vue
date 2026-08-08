<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { apiGet } from '../../../../../api'
import { fmtDate } from '../../../../../utils'

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
  cash: number
  position_value: number
  position_qty: number
  profit: number
  roi: number
}

const runs = ref<RunRow[]>([])
const runId = ref('')
const points = ref<EquityPoint[]>([])
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

async function loadCurve() {
  if (!runId.value) return
  loading.value = true
  error.value = ''
  chart?.dispose()
  chart = null
  try {
    const data = await apiGet<{ list: EquityPoint[] }>(
      `/Meta/Finv/Quant/Backtest/Run/Equity?runId=${runId.value}&page=1&pageSize=5000`,
    )
    points.value = data.list ?? []
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
    title: { text: '资金管理 · 现金与总资产曲线', left: 12, textStyle: { fontSize: 13, fontWeight: 600 } },
    tooltip: { trigger: 'axis' },
    legend: { data: ['总资产', '现金'], top: 4, right: 12 },
    grid: { left: 80, right: 24, top: 44, bottom: 30 },
    xAxis: { type: 'category', data: labels, axisLabel: { fontSize: 10 } },
    yAxis: { type: 'value', scale: true },
    series: [
      { name: '总资产', type: 'line', data: points.value.map((p) => p.equity), showSymbol: false, itemStyle: { color: '#1565c0' } },
      { name: '现金', type: 'line', data: points.value.map((p) => p.cash), showSymbol: false, itemStyle: { color: '#90a4ae' }, lineStyle: { type: 'dashed' } },
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
        <v-icon icon="mdi-cash-multiple" class="mr-2" color="primary" />资金管理（回测资金数据）
        <v-spacer />
        <v-select v-model="runId" :items="runs.map((r) => ({
          title: `#${r.run_no} ${r.strategy_name}（${r.secu_code} ${fmtDate(r.start_date)}~${fmtDate(r.end_date)}）`,
          value: r.run_id,
        }))" label="选择回测任务（已成功）" hide-details style="max-width: 520px" class="mr-2"
          @update:model-value="loadCurve" />
        <v-btn color="primary" variant="tonal" @click="loadCurve">加载</v-btn>
      </v-card-title>
      <v-alert v-if="error" type="error" dismissible class="mx-4 mb-2">{{ error }}</v-alert>
      <v-card-text>
        <v-alert type="info" variant="tonal" density="compact" class="mb-3">
          查看回测任务在报告精度下的现金余额与总资产（持仓换算现金）变化；结构化数据可通过
          <code>/API/V1/Meta/Finv/Quant/Backtest/Run/Equity?runId=xxx</code> 获取。
        </v-alert>
        <v-skeleton-loader v-if="loading" type="image" height="320" />
        <div v-else ref="chartEl" style="width: 100%; height: 420px" />
      </v-card-text>
    </v-card>
  </v-container>
</template>
