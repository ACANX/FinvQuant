<script setup lang="ts">
// K线数据统计：汇总指标 + 空值完整性 + ECharts 可视化
import * as echarts from 'echarts'
import { useTheme } from 'vuetify'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../api'
import { fmtDate, shiftMonthISO, todayISO } from '../../../../../utils'

interface SecurityOption {
  usc: string
  security_name_cn: string
}

interface NullCounts {
  date: number
  open: number
  high: number
  low: number
  close: number
  last_close: number
  change_price: number
  change_ratio: number
  volume: number
  turnover: number
  provider: number
}

interface StatsSummary {
  rows: number
  usc_count: number
  date_from: number | null
  date_to: number | null
  date_count: number
  volume_sum: string | null
  turnover_sum: string | null
  close_min: string | null
  close_max: string | null
  close_avg: string | null
  null_counts: NullCounts
}

interface DailyStat {
  date: number
  rows: number
  volume_sum: string | null
  turnover_sum: string | null
  close_min: string | null
  close_max: string | null
  close_avg: string | null
}

interface UscStat {
  usc: string
  rows: number
}

interface StatsData {
  summary: StatsSummary
  daily: DailyStat[]
  top_usc: UscStat[]
}

// 默认证券与默认日期范围（最近一个月，按库内最新交易日期回推）
const DEFAULT_USC = 'GCMain'
const usc = ref<string | null>(DEFAULT_USC)
const dateFrom = ref<string | null>(shiftMonthISO(todayISO(), -1))
const dateTo = ref<string | null>(todayISO())
const defaultRange = ref<{ from: string; to: string } | null>(null)
const loading = ref(false)
const error = ref('')
const data = ref<StatsData | null>(null)

const securityOptions = ref<SecurityOption[]>([])
const secuItems = ref<{ title: string; value: string }[]>([])

const countEl = ref<HTMLDivElement | null>(null)
const volumeEl = ref<HTMLDivElement | null>(null)
const turnoverEl = ref<HTMLDivElement | null>(null)
const avgEl = ref<HTMLDivElement | null>(null)
const topEl = ref<HTMLDivElement | null>(null)
let countChart: echarts.ECharts | null = null
let volumeChart: echarts.ECharts | null = null
let turnoverChart: echarts.ECharts | null = null
let avgChart: echarts.ECharts | null = null
let topChart: echarts.ECharts | null = null
// 记录图表绑定的 DOM：条件渲染（错误/空数据切换）后 DOM 会重建，必须重新 init
let countDom: HTMLDivElement | null = null
let volumeDom: HTMLDivElement | null = null
let turnoverDom: HTMLDivElement | null = null
let avgDom: HTMLDivElement | null = null
let topDom: HTMLDivElement | null = null
let observer: ResizeObserver | null = null

const AXIS_COLOR = '#9aa4b2'

// 坐标轴线/分割线颜色随主题变化：深色用浅色半透明，浅色用深色半透明
const theme = useTheme()
function splitColor(): string {
  return theme.global.name.value === 'dark' ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.10)'
}

const summary = computed(() => data.value?.summary ?? null)
const daily = computed(() => data.value?.daily ?? [])
const topUsc = computed(() => data.value?.top_usc ?? [])

// 空值/缺失字段完整性清单（0 为正常，非 0 需关注）
const nullItems = computed(() => {
  const n = summary.value?.null_counts
  if (!n) return []
  return [
    { key: 'date', label: '交易日期', value: n.date },
    { key: 'open', label: '开盘价', value: n.open },
    { key: 'high', label: '最高价', value: n.high },
    { key: 'low', label: '最低价', value: n.low },
    { key: 'close', label: '收盘价', value: n.close },
    { key: 'last_close', label: '前收盘价', value: n.last_close },
    { key: 'change_price', label: '涨跌额', value: n.change_price },
    { key: 'change_ratio', label: '涨跌幅', value: n.change_ratio },
    { key: 'volume', label: '成交量', value: n.volume },
    { key: 'turnover', label: '成交额', value: n.turnover },
    { key: 'provider', label: '数据来源', value: n.provider },
  ]
})

function dateParam(value: string | null): string | null {
  if (!value) return null
  const compact = value.replace(/-/g, '')
  return /^\d{8}$/.test(compact) ? compact : null
}

// 数值展示（默认不使用千分位）
function fmtInt(value: number | null | undefined): string {
  if (value === null || value === undefined || Number.isNaN(value)) return '—'
  return String(Number(value))
}

// 按指定小数位四舍五入后输出纯数字文本（无千分位）
function plain(value: number, digits: number): string {
  return String(Number(value.toFixed(digits)))
}

// 大数字紧凑展示：万 / 亿
function fmtCompact(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const number = Number(value)
  if (!Number.isFinite(number)) return '—'
  const abs = Math.abs(number)
  if (abs >= 1e12) return `${plain(number / 1e12, 2)} 万亿`
  if (abs >= 1e8) return `${plain(number / 1e8, 2)} 亿`
  if (abs >= 1e4) return `${plain(number / 1e4, 2)} 万`
  return plain(number, 2)
}

function fmtPrice(value: string | number | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—'
  const number = Number(value)
  if (!Number.isFinite(number)) return '—'
  return String(number)
}

// 日期轴：MM-DD（悬停显示完整日期）
function shortDate(date: number): string {
  const text = String(date)
  return text.length === 8 ? `${text.slice(4, 6)}-${text.slice(6, 8)}` : text
}

function onUscInput(value: unknown) {
  usc.value = typeof value === 'string' ? value : ((value as { value?: string } | null)?.value ?? '')
}

async function load() {
  loading.value = true
  error.value = ''
  const from = dateParam(dateFrom.value)
  const to = dateParam(dateTo.value)
  if (from && to && from > to) {
    error.value = '开始日期不能晚于结束日期'
    loading.value = false
    return
  }
  const params = new URLSearchParams()
  const uscText = (usc.value ?? '').trim()
  if (uscText) params.set('usc', uscText)
  if (from) params.set('dateFrom', from)
  if (to) params.set('dateTo', to)
  try {
    data.value = await apiGet<StatsData>(
      `/Meta/Finv/Quote/Cdeu/Min/Stats?${params.toString()}`,
    )
    await nextTick()
    renderCharts()
  } catch (e) {
    data.value = null
    error.value = (e as Error).message
    disposeCharts()
  } finally {
    loading.value = false
  }
}

// 默认日期范围：以库内最新交易日期为结束日、往前 1 个月；查询失败时回退到本机今天
async function resolveDefaultRange(): Promise<{ from: string; to: string }> {
  try {
    const bounds = await apiGet<{ date_to: number | null }>(
      `/Meta/Finv/Quote/Cdeu/Min/Bounds?usc=${DEFAULT_USC}`,
    )
    const end = bounds.date_to ? fmtDate(bounds.date_to) : todayISO()
    return { from: shiftMonthISO(end, -1), to: end }
  } catch {
    const end = todayISO()
    return { from: shiftMonthISO(end, -1), to: end }
  }
}

function reset() {
  usc.value = DEFAULT_USC
  const range = defaultRange.value ?? { from: shiftMonthISO(todayISO(), -1), to: todayISO() }
  dateFrom.value = range.from
  dateTo.value = range.to
  load()
}

// 释放全部图表实例（容器被条件渲染销毁时调用）
function disposeCharts() {
  for (const chart of [countChart, volumeChart, turnoverChart, avgChart, topChart]) chart?.dispose()
  countChart = null
  volumeChart = null
  turnoverChart = null
  avgChart = null
  topChart = null
  countDom = null
  volumeDom = null
  turnoverDom = null
  avgDom = null
  topDom = null
}

// 容器变化时销毁旧实例，返回可用的新实例
function chartFor(
  el: HTMLDivElement | null,
  current: echarts.ECharts | null,
  currentDom: HTMLDivElement | null,
): { chart: echarts.ECharts | null; dom: HTMLDivElement | null } {
  if (!el) {
    current?.dispose()
    return { chart: null, dom: null }
  }
  if (current && currentDom !== el) {
    current.dispose()
    current = null
  }
  return { chart: current ?? echarts.init(el), dom: el }
}

// 三张每日图表共用绘图区网格与时间轴配置，保证左右边界、刻度与标签完全对齐
const CHART_GRID = { left: 76, right: 24, top: 36, bottom: 36 }

function timeLabelInterval(): number {
  const size = daily.value.length
  if (size <= 12) return 0
  return Math.ceil(size / 12) - 1
}

function dailyXAxis() {
  return {
    type: 'category' as const,
    boundaryGap: true,
    data: daily.value.map((item) => shortDate(item.date)),
    axisLabel: { color: AXIS_COLOR, hideOverlap: true, interval: timeLabelInterval() },
    axisLine: { lineStyle: { color: splitColor() } },
    axisTick: { alignWithLabel: true },
  }
}

function dailyTooltip(title: string, format: (value: number | null) => string) {
  return {
    trigger: 'axis' as const,
    formatter: (params: unknown) => {
      const list = params as { axisValue: string; value: number | null }[]
      if (!list.length) return ''
      const index = daily.value.findIndex((item) => shortDate(item.date) === list[0].axisValue)
      const fullDate = index >= 0 ? fmtDate(daily.value[index].date) : list[0].axisValue
      return `${fullDate}<br/>${title}：${format(list[0].value)}`
    },
  }
}

function renderCharts() {
  const dailyData = daily.value
  const topData = topUsc.value

  // 1) 每日记录数（柱）
  {
    const target = chartFor(countEl.value, countChart, countDom)
    countChart = target.chart
    countDom = target.dom
  }
  if (countChart) {
    countChart.setOption(
      {
        backgroundColor: 'transparent',
        tooltip: dailyTooltip('记录数', (value) => (value === null ? '—' : String(Number(value)))),
        grid: CHART_GRID,
        xAxis: dailyXAxis(),
        yAxis: {
          type: 'value',
          name: '记录数',
          nameTextStyle: { color: AXIS_COLOR },
          axisLabel: { color: AXIS_COLOR, formatter: (value: number) => String(value) },
          splitLine: { lineStyle: { color: splitColor() } },
        },
        series: [
          {
            name: '记录数',
            type: 'bar',
            data: dailyData.map((item) => item.rows),
            itemStyle: { color: '#3b8ff7' },
            barMaxWidth: 24,
          },
        ],
      },
      true,
    )
  }

  // 2) 每日成交量（柱）
  {
    const target = chartFor(volumeEl.value, volumeChart, volumeDom)
    volumeChart = target.chart
    volumeDom = target.dom
  }
  if (volumeChart) {
    volumeChart.setOption(
      {
        backgroundColor: 'transparent',
        tooltip: dailyTooltip('成交量', (value) => (value === null ? '—' : fmtCompact(value))),
        grid: CHART_GRID,
        xAxis: dailyXAxis(),
        yAxis: {
          type: 'value',
          name: '成交量',
          nameTextStyle: { color: AXIS_COLOR },
          axisLabel: { color: AXIS_COLOR, formatter: (value: number) => fmtCompact(value) },
          splitLine: { lineStyle: { color: splitColor() } },
        },
        series: [
          {
            name: '成交量',
            type: 'bar',
            data: dailyData.map((item) => (item.volume_sum === null ? null : Number(item.volume_sum))),
            itemStyle: { color: '#f6c343' },
            barMaxWidth: 24,
          },
        ],
      },
      true,
    )
  }

  // 3) 每日成交额（柱）
  {
    const target = chartFor(turnoverEl.value, turnoverChart, turnoverDom)
    turnoverChart = target.chart
    turnoverDom = target.dom
  }
  if (turnoverChart) {
    turnoverChart.setOption(
      {
        backgroundColor: 'transparent',
        tooltip: dailyTooltip('成交额', (value) => (value === null ? '—' : fmtCompact(value))),
        grid: CHART_GRID,
        xAxis: dailyXAxis(),
        yAxis: {
          type: 'value',
          name: '成交额',
          nameTextStyle: { color: AXIS_COLOR },
          axisLabel: { color: AXIS_COLOR, formatter: (value: number) => fmtCompact(value) },
          splitLine: { lineStyle: { color: splitColor() } },
        },
        series: [
          {
            name: '成交额',
            type: 'bar',
            data: dailyData.map((item) => (item.turnover_sum === null ? null : Number(item.turnover_sum))),
            itemStyle: { color: '#c56cf0' },
            barMaxWidth: 24,
          },
        ],
      },
      true,
    )
  }

  // 4) 每日收盘价均值（折线）
  {
    const target = chartFor(avgEl.value, avgChart, avgDom)
    avgChart = target.chart
    avgDom = target.dom
  }
  if (avgChart) {
    avgChart.setOption(
      {
        backgroundColor: 'transparent',
        tooltip: dailyTooltip('收盘价均值', (value) => (value === null ? '—' : fmtPrice(value))),
        grid: CHART_GRID,
        xAxis: dailyXAxis(),
        yAxis: {
          type: 'value',
          scale: true,
          name: '收盘价均值',
          nameTextStyle: { color: AXIS_COLOR },
          axisLabel: { color: AXIS_COLOR, formatter: (value: number) => String(value) },
          splitLine: { lineStyle: { color: splitColor() } },
        },
        series: [
          {
            name: '收盘价均值',
            type: 'line',
            smooth: true,
            showSymbol: dailyData.length <= 60,
            data: dailyData.map((item) => (item.close_avg === null ? null : Number(item.close_avg))),
            lineStyle: { color: '#2fb28a' },
            itemStyle: { color: '#2fb28a' },
            areaStyle: { color: 'rgba(47,178,138,0.12)' },
          },
        ],
      },
      true,
    )
  }

  // 5) 证券行数 Top 20（横向柱）
  {
    const target = chartFor(topEl.value, topChart, topDom)
    topChart = target.chart
    topDom = target.dom
  }
  if (topChart) {
    const reversed = [...topData].reverse()
    topChart.setOption(
      {
        backgroundColor: 'transparent',
        tooltip: {
          trigger: 'axis',
          axisPointer: { type: 'shadow' },
          formatter: (params: unknown) => {
            const list = params as { name: string; value: number }[]
            if (!list.length) return ''
            return `${list[0].name}<br/>记录数：${String(Number(list[0].value))}`
          },
        },
        grid: { left: 16, right: 48, top: 16, bottom: 8, containLabel: true },
        xAxis: {
          type: 'value',
          axisLabel: { color: AXIS_COLOR, formatter: (value: number) => fmtCompact(value) },
          splitLine: { lineStyle: { color: splitColor() } },
        },
        yAxis: {
          type: 'category',
          data: reversed.map((item) => item.usc),
          axisLabel: { color: AXIS_COLOR },
          axisLine: { lineStyle: { color: splitColor() } },
        },
        series: [
          {
            name: '记录数',
            type: 'bar',
            data: reversed.map((item) => item.rows),
            itemStyle: { color: '#c56cf0' },
            barMaxWidth: 18,
            label: { show: true, position: 'right', color: AXIS_COLOR, formatter: (p: { value: number }) => fmtCompact(p.value) },
          },
        ],
      },
      true,
    )
  }

  // 条件渲染后重新绑定尺寸监听
  if (observer) {
    observer.disconnect()
    for (const el of [countEl.value, volumeEl.value, turnoverEl.value, avgEl.value, topEl.value]) {
      if (el) observer.observe(el)
    }
  }
  for (const chart of [countChart, volumeChart, turnoverChart, avgChart, topChart]) chart?.resize()
}

function resizeCharts() {
  // 条件渲染后重新绑定尺寸监听
  if (observer) {
    observer.disconnect()
    for (const el of [countEl.value, volumeEl.value, turnoverEl.value, avgEl.value, topEl.value]) {
      if (el) observer.observe(el)
    }
  }
  for (const chart of [countChart, volumeChart, turnoverChart, avgChart, topChart]) chart?.resize()
}

// 主题切换后重绘（分割线/轴线颜色随主题变化）
watch(
  () => theme.global.name.value,
  () => {
    renderCharts()
    resizeCharts()
  },
)

onMounted(async () => {
  try {
    const options = await apiGet<{ list: SecurityOption[] }>('/Meta/Finv/Quant/Metadata/Security/Options')
    securityOptions.value = options.list ?? []
    secuItems.value = securityOptions.value.map((item) => ({
      title: `${item.usc}:${item.security_name_cn}`,
      value: item.usc,
    }))
  } catch {
    securityOptions.value = []
    secuItems.value = []
  }
  if ('ResizeObserver' in window) {
    observer = new ResizeObserver(() => resizeCharts())
    for (const el of [countEl.value, volumeEl.value, turnoverEl.value, avgEl.value, topEl.value]) {
      if (el) observer.observe(el)
    }
  }
  window.addEventListener('resize', resizeCharts)
  const range = await resolveDefaultRange()
  defaultRange.value = range
  dateFrom.value = range.from
  dateTo.value = range.to
  await load()
})

onBeforeUnmount(() => {
  disposeCharts()
  observer?.disconnect()
  observer = null
  window.removeEventListener('resize', resizeCharts)
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-chart-box-outline" class="mr-2" />
      K线数据统计
    </v-card-title>

    <v-card-text>
      <v-row dense align="center" class="mb-3">
        <v-col cols="12" sm="6" md="3">
          <v-combobox
            v-model="usc"
            :items="secuItems"
            item-title="title"
            item-value="value"
            label="证券代码[USC]"
            placeholder="全部证券"
            density="compact"
            hide-details
            clearable
            @update:model-value="onUscInput"
          />
        </v-col>
        <v-col cols="6" sm="3" md="2">
          <v-text-field v-model="dateFrom" type="date" label="开始日期" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="6" sm="3" md="2">
          <v-text-field v-model="dateTo" type="date" label="结束日期" density="compact" hide-details clearable />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-chart-line" :loading="loading" @click="load">统计</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <template v-if="summary">
        <v-row dense>
          <v-col cols="12" sm="6" md="3">
            <v-card variant="tonal" color="primary">
              <v-card-text>
                <div class="text-caption">记录总数</div>
                <div class="text-h5 font-weight-bold">{{ fmtInt(summary.rows) }}</div>
              </v-card-text>
            </v-card>
          </v-col>
          <v-col cols="6" sm="3" md="2">
            <v-card variant="tonal" color="info">
              <v-card-text>
                <div class="text-caption">证券数</div>
                <div class="text-h6 font-weight-bold">{{ fmtInt(summary.usc_count) }}</div>
              </v-card-text>
            </v-card>
          </v-col>
          <v-col cols="6" sm="3" md="2">
            <v-card variant="tonal" color="secondary">
              <v-card-text>
                <div class="text-caption">交易日数</div>
                <div class="text-h6 font-weight-bold">{{ fmtInt(summary.date_count) }}</div>
              </v-card-text>
            </v-card>
          </v-col>
          <v-col cols="12" sm="6" md="2">
            <v-card variant="tonal" color="teal">
              <v-card-text>
                <div class="text-caption">成交量合计</div>
                <div class="text-h6 font-weight-bold">{{ fmtCompact(summary.volume_sum) }}</div>
              </v-card-text>
            </v-card>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-card variant="tonal" color="deep-purple">
              <v-card-text>
                <div class="text-caption">成交额合计</div>
                <div class="text-h6 font-weight-bold">{{ fmtCompact(summary.turnover_sum) }}</div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-text class="d-flex flex-wrap align-center ga-6">
                <div>
                  <div class="text-caption text-medium-emphasis">日期范围</div>
                  <div class="text-body-1">
                    {{ fmtDate(summary.date_from) }} ~ {{ fmtDate(summary.date_to) }}
                  </div>
                </div>
                <div>
                  <div class="text-caption text-medium-emphasis">收盘价区间</div>
                  <div class="text-body-1">{{ fmtPrice(summary.close_min) }} ~ {{ fmtPrice(summary.close_max) }}</div>
                </div>
                <div>
                  <div class="text-caption text-medium-emphasis">收盘价均值</div>
                  <div class="text-body-1">{{ fmtPrice(summary.close_avg) }}</div>
                </div>
                <div>
                  <div class="text-caption text-medium-emphasis">当前过滤</div>
                  <div class="text-body-1">
                    {{ usc ? `证券 ${usc}` : '全部证券' }} · {{ dateFrom || dateTo ? '自定义日期' : '全部日期' }}
                  </div>
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">
                字段完整性（空值行数）
              </v-card-title>
              <v-card-text class="pt-0">
                <div class="d-flex flex-wrap ga-2">
                  <v-chip
                    v-for="item in nullItems"
                    :key="item.key"
                    size="small"
                    :color="item.value === 0 ? 'success' : 'warning'"
                    variant="tonal"
                  >
                    {{ item.label }}：{{ fmtInt(item.value) }}
                  </v-chip>
                </div>
                <div class="text-caption text-medium-emphasis mt-3">
                  空值占比 = 空值行数 / 记录总数；涨跌字段为 0 空表示全部已补全。
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">每日记录数</v-card-title>
              <v-card-text>
                <div ref="countEl" style="height: 320px; width: 100%" />
                <div v-if="daily.length === 0" class="text-medium-emphasis text-caption">
                  当前条件下没有按日期聚合的数据
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">每日成交量</v-card-title>
              <v-card-text>
                <div ref="volumeEl" style="height: 320px; width: 100%" />
                <div v-if="daily.length === 0" class="text-medium-emphasis text-caption">
                  当前条件下没有按日期聚合的数据
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">每日成交额</v-card-title>
              <v-card-text>
                <div ref="turnoverEl" style="height: 320px; width: 100%" />
                <div v-if="daily.length === 0" class="text-medium-emphasis text-caption">
                  当前条件下没有按日期聚合的数据
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">每日收盘价均值</v-card-title>
              <v-card-text>
                <div ref="avgEl" style="height: 320px; width: 100%" />
                <div v-if="daily.length === 0" class="text-medium-emphasis text-caption">
                  当前条件下没有按日期聚合的数据
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <v-row dense class="mt-1">
          <v-col cols="12">
            <v-card variant="outlined">
              <v-card-title class="text-body-2">证券记录数 Top 20</v-card-title>
              <v-card-text>
                <div
                  ref="topEl"
                  :style="{ height: `${Math.max(180, Math.min(520, topUsc.length * 26 + 40))}px`, width: '100%' }"
                />
                <div v-if="topUsc.length === 0" class="text-medium-emphasis text-caption">暂无证券分布数据</div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>
      </template>

      <v-skeleton-loader v-else-if="loading" type="card-avatar, article" />

      <v-card variant="tonal" color="surface-variant" class="mt-4">
        <v-card-text class="text-caption text-medium-emphasis">
          <div class="text-body-2 text-high-emphasis mb-1">页面说明</div>
          <div>cdeu.finv_quote_secu_kline_min 汇总指标、字段完整性与分布可视化；支持按证券代码[USC]与交易日期范围过滤。</div>
          <div class="mt-1">统计口径：汇总、每日趋势与证券分布在同一只读事务内完成；空值占比 = 空值行数 / 记录总数。</div>
        </v-card-text>
      </v-card>
    </v-card-text>
  </v-card>
</template>
