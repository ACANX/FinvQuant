<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'

import { apiGet } from '../../../../../api'

interface QuoteBar {
  date: number
  time: number
  open: string | null
  high: string | null
  low: string | null
  close: string | null
  volume: number | null
  turnover: string | null
  change: string | null
  change_pct: string | null
  remark: string | null
}

// 证券下拉选项（usc 为 key，security_name_cn 为展示值，来自 finv_security 字典）
interface SecurityOption {
  usc: string
  security_name_cn: string
}

// 富途牛牛风格配色：红涨绿跌 + 均线黄/蓝/紫/青
const COLOR_UP = '#ef232a'
const COLOR_DOWN = '#14b143'
// 均线周期与配色（MA5 / MA10 / MA20 / MA60 / MA120 / MA250）
const MA_CONFIGS: { n: number; color: string }[] = [
  { n: 5, color: '#f6c343' },   // MA5  黄
  { n: 10, color: '#3b8ff7' },  // MA10 蓝
  { n: 20, color: '#c56cf0' },  // MA20 紫
  { n: 60, color: '#2fb28a' },  // MA60 青
  { n: 120, color: '#f49f0a' }, // MA120 橙
  { n: 250, color: '#e05cb4' }, // MA250 粉
]
// 布林带三条线名称与配色：上轨 / 中轨 / 下轨
const BOLL_SERIES = [
  { name: 'BOLL上轨', color: '#ff8c69' },
  { name: 'BOLL中轨', color: '#ffffff' },
  { name: 'BOLL下轨', color: '#7ec8e3' },
]

const secuCode = ref('GCMain') // 证券代码（usc key），默认 GCMain
const secuName = ref('') // 证券名称（security_name_cn），选中字典项后回填，便于确认
const dateInput = ref<string | null>('2026-05-12') // 日历选择的日期（yyyy-MM-dd），默认 2026-05-12
const dateMenu = ref(false)
const period = ref('Min')
const days = ref(1) // 1 日 / 5 日
const loading = ref(false)
const error = ref('')
const total = ref(0)
const chartEl = ref<HTMLDivElement | null>(null)

// 各序列显隐状态（仅随用户手动点击图例变化，查询切换不重置）。
// 默认：K线显示；均线全部隐藏；布林带全部隐藏。见 renderChart 的 legend.selected。
const seriesVisible = reactive<Record<string, boolean>>({
  K线: true,
  ...Object.fromEntries(MA_CONFIGS.map((c) => [`MA${c.n}`, false])),
  ...Object.fromEntries(BOLL_SERIES.map((b) => [b.name, false])),
})
const groupToggles = reactive<Record<string, boolean>>({ 均线: false, 布林: false }) // 组总开关状态
const maNames = MA_CONFIGS.map((c) => `MA${c.n}`)
const bollNames = BOLL_SERIES.map((b) => b.name)

// 最近一次查询渲染的 K 线数据，供图例点击后按当前显隐状态重绘（保留用户 dataZoom 缩放区间）
let lastBars: QuoteBar[] = []

// 组内各成员是否全部显示
function groupAllShown(names: string[]): boolean {
  return names.every((n) => seriesVisible[n])
}

// 图表自适应：容器尺寸变化（导航栏折叠/展开、窗口缩放）时调用 chart.resize()
let resizeObserver: ResizeObserver | null = null
function resizeChart() {
  chart?.resize()
}

// 查询结果提示（snackbar 吐司，3 秒自动消失，替代常驻 alert）
const toast = ref<{ text: string; visible: boolean }>({ text: '', visible: false })
function showToast(text: string) {
  toast.value = { text, visible: true }
}

let chart: echarts.ECharts | null = null

// 本地时区当天（不使用 toISOString 避免 UTC 偏移跨日）
function localTodayISO(): string {
  const d = new Date()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}
const todayISO = localTodayISO()

// Vuetify 4 的 v-date-picker 选中后 model 值可能是 Date 对象（随 locale 显示为英文），
// 统一规范化为 yyyy-MM-dd 纯日期字符串（无时间），保证展示与传参格式稳定
function normalizeDate(v: unknown): string | null {
  if (v === null || v === undefined || v === '') return null
  if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return v
  const d = new Date(v as string)
  if (Number.isNaN(d.getTime())) return null
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

function onDatePicked(v: unknown) {
  dateInput.value = normalizeDate(v)
  dateMenu.value = false
}

// 提交给后端的交易日期（yyyymmdd 纯数字，8 位）
const date = computed(() => (dateInput.value ? dateInput.value.replace(/-/g, '') : ''))

// 证券下拉选项（usc:security_name_cn 作为展示文本，value 为 usc）
const securityOptions = ref<SecurityOption[]>([])
const secuItems = computed(() =>
  securityOptions.value.map((o) => ({
    title: `${o.usc}:${o.security_name_cn}`,
    value: o.usc,
  })),
)

// 证券代码输入变化：若命中字典项则回填证券名称，否则清空
function onSecuCodeInput(val: unknown) {
  // v-combobox 可能返回字符串或对象 {title,value}，统一取字符串值对比
  const s = typeof val === 'string' ? val : (val as { value?: string } | null | undefined)?.value ?? ''
  const hit = securityOptions.value.find((o) => o.usc === s)
  secuName.value = hit ? hit.security_name_cn : ''
}

// 查询条件变化（证券/日期/天数/周期）自动触发查询，无需手动点“查询”
watch([secuCode, dateInput, days, period], () => {
  query()
})

onMounted(async () => {
  // 加载证券下拉字典（finv_security.usc + security_name_cn）
  try {
    const data = await apiGet<{ list: SecurityOption[] }>('/Meta/FinvQuant/Metadata/Security/Options')
    securityOptions.value = data.list ?? []
  } catch {
    // 字典加载失败不阻塞查询（仍可手动输入证券代码）
    securityOptions.value = []
  }
  // 图表自适应：监听容器尺寸变化（导航栏折叠/展开、窗口缩放）
  if (chartEl.value && 'ResizeObserver' in window) {
    resizeObserver = new ResizeObserver(() => resizeChart())
    resizeObserver.observe(chartEl.value)
  }
  window.addEventListener('resize', resizeChart)
  // 打开页面即按默认证券/日期自动查询一次（GCMain · 2026-05-12）
  query()
})

onBeforeUnmount(() => {
  chart?.dispose()
  chart = null
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  window.removeEventListener('resize', resizeChart)
})

function formatTime(time: number): string {
  const s = String(time).padStart(6, '0')
  return `${s.slice(0, 2)}:${s.slice(2, 4)}:${s.slice(4, 6)}`
}

// 本地时区信息（如 UTC+8 / GMT+08:00），用于 tooltip 第一行日期时间的时区标注
function timezoneLabel(): string {
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || 'local'
  const offMin = -new Date().getTimezoneOffset()
  const sign = offMin >= 0 ? '+' : '-'
  const abs = Math.abs(offMin)
  const hh = String(Math.floor(abs / 60)).padStart(2, '0')
  const mm = String(abs % 60).padStart(2, '0')
  return `${tz} (UTC${sign}${hh}:${mm})`
}
const TZ_LABEL = timezoneLabel() // 模块级常量：一次计算，避免频繁调用

// 日期格式化为 yyyy-MM-dd（输入为 8 位数字 yyyymmdd）
function formatDateDash(d: string | number | null | undefined): string {
  const s = String(d ?? '')
  if (!/^\d{8}$/.test(s)) return '—'
  return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
}

// 万分位分组（中文读数习惯，每 4 位一组）：12345678 -> 1234,5678；保留小数
function formatGroup4(v: number | null | undefined): string {
  if (v === null || v === undefined || !Number.isFinite(v)) return '—'
  const s = String(v)
  const neg = s.startsWith('-') ? '-' : ''
  const body = neg ? s.slice(1) : s
  const [int, dec] = body.split('.')
  const grouped = int.replace(/\B(?=(\d{4})+(?!\d))/g, ',')
  return dec !== undefined ? neg + grouped + '.' + dec : neg + grouped
}

function formatAxisLabel(bar: QuoteBar): string {
  const hm = formatTime(bar.time).slice(0, 5)
  if (days.value > 1 && bar.date) {
    const d = String(bar.date)
    return `${d.slice(4, 6)}/${d.slice(6, 8)} ${hm}`
  }
  return hm
}

// 判断某根 bar 的时间是否为整点刻度（time 的分钟部分为 00）
function isHourMark(time: number): boolean {
  return time % 10000 === 0
}

// x 轴刻度显示策略：
//   - 1 日模式：每个整点（0/1/2...23）显示时刻（如 08:00），其余隐藏；
//   - 5 日模式：只在 0 点显示日期（MM/DD），4/8/12/16/20 点显示时刻刻度，其余隐藏
function xAxisLabelFor(bar: QuoteBar): string {
  if (!isHourMark(bar.time)) return ''
  const hour = Math.floor(bar.time / 10000)
  if (days.value > 1) {
    if (hour === 0) {
      const d = String(bar.date)
      return `${d.slice(4, 6)}/${d.slice(6, 8)}`
    }
    if ([4, 8, 12, 16, 20].includes(hour)) return String(hour).padStart(2, '0') + ':00'
    return ''
  }
  return String(hour).padStart(2, '0') + ':00'
}

function toNum(v: string | null | undefined): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isNaN(n) ? null : n
}

// 滑动窗口均线（窗口内存在缺失值则输出 null，避免断点被错误填充）
function computeMA(closes: (number | null)[], n: number): (number | null)[] {
  const out: (number | null)[] = []
  let sum = 0
  let valid = 0
  for (let i = 0; i < closes.length; i++) {
    const v = closes[i]
    if (v !== null) {
      sum += v
      valid++
    }
    if (i >= n) {
      const drop = closes[i - n]
      if (drop !== null) {
        sum -= drop
        valid--
      }
    }
    out.push(valid >= n ? sum / n : null)
  }
  return out
}

// 布林带（BOLL）：中轨 = MA(n)，上/下轨 = 中轨 ± k × 标准差（滑动窗口）
// 返回 [上轨, 中轨, 下轨] 三条序列；窗口内存在缺失值则输出 null
function computeBOLL(closes: (number | null)[], n: number, k: number): [(number | null)[], (number | null)[], (number | null)[]] {
  const upper: (number | null)[] = []
  const mid: (number | null)[] = []
  const lower: (number | null)[] = []
  for (let i = 0; i < closes.length; i++) {
    // 取窗口内有效值
    const start = Math.max(0, i - n + 1)
    const window: number[] = []
    for (let j = start; j <= i; j++) {
      const v = closes[j]
      if (v !== null) window.push(v)
    }
    if (window.length < n) {
      upper.push(null)
      mid.push(null)
      lower.push(null)
      continue
    }
    let sum = 0
    for (const v of window) sum += v
    const mean = sum / window.length
    let variance = 0
    for (const v of window) variance += (v - mean) * (v - mean)
    const std = Math.sqrt(variance / window.length)
    mid.push(mean)
    upper.push(mean + k * std)
    lower.push(mean - k * std)
  }
  return [upper, mid, lower]
}

// 三行图例配置：第一行 K线；第二行 均线总开关 + MA5/10/20/60/120/250；
// 第三行 布林总开关 + 上/中/下轨。总开关选中态由 groupToggles 驱动，成员由 seriesVisible 驱动。
function renderLegendOption() {
  return [
    {
      top: 4,
      left: 108,
      itemWidth: 14,
      itemHeight: 8,
      icon: 'roundRect',
      textStyle: { fontSize: 11, color: '#ccc' },
      data: ['K线'],
      selected: { K线: seriesVisible.K线 },
    },
    {
      top: 20,
      left: 108,
      itemWidth: 14,
      itemHeight: 8,
      icon: 'roundRect',
      textStyle: { fontSize: 11, color: '#ccc' },
      data: ['均线', ...maNames],
      selected: {
        均线: groupToggles.均线,
        ...Object.fromEntries(maNames.map((n) => [n, seriesVisible[n]])),
      },
    },
    {
      top: 36,
      left: 108,
      itemWidth: 14,
      itemHeight: 8,
      icon: 'roundRect',
      textStyle: { fontSize: 11, color: '#ccc' },
      data: ['布林', ...bollNames],
      selected: {
        布林: groupToggles.布林,
        ...Object.fromEntries(bollNames.map((n) => [n, seriesVisible[n]])),
      },
    },
  ]
}

async function query() {
  error.value = ''
  // v-combobox 可能返回字符串（自由输入）或对象（从字典下拉选中 {title,value}），
  // 统一归一化为纯字符串 usc 代码，避免 .trim() 对非字符串报错
  const secuRaw = secuCode.value
  const secuStr = typeof secuRaw === 'string' ? secuRaw : (secuRaw as { value?: string } | null | undefined)?.value ?? ''
  if (!secuStr.trim()) {
    error.value = '证券代码不能为空'
    return
  }
  if (!/^\d{8}$/.test(date.value)) {
    error.value = '请通过日历选择交易日期'
    return
  }

  loading.value = true
  try {
    // 按日期 + N 日回溯查询（服务端转为 ts 范围返回全部记录，不分页）
    // URL 查询参数遵循小驼峰规范（ApiSpec §3）
    const params = new URLSearchParams({
      secuCode: secuStr.trim(),
      date: date.value,
      period: period.value,
      days: String(days.value),
    })
    // 选中字典项时回传证券名称（security_name_cn），便于服务端与前端确认证券
    if (secuName.value.trim()) {
      params.set('secuName', secuName.value.trim())
    }
    const response = await fetch(`/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery?${params}`)
    const body = await response.json()
    if (body.code !== 0) {
      error.value = body.message || '查询失败'
      return
    }
    const data = body.data ?? {}
    const bars: QuoteBar[] = data.bars ?? []
    total.value = data.total ?? bars.length
    const rangeText =
      days.value > 1 && bars.length
        ? `${String(bars[0].date).slice(4)}-${String(bars[bars.length - 1].date).slice(4)}`
        : date.value
    const namePart = secuName.value.trim() ? `（${secuName.value.trim()}）` : ''
    showToast(`${data.secu_code ?? '?'}${namePart} ${rangeText} ${data.period ?? 'Min'} · ${days.value}日 · 共 ${total.value} 根`)
    await nextTick()
    if (bars.length) {
      renderChart(bars)
    } else {
      chart?.clear()
    }
  } catch {
    error.value = '网络错误：无法连接服务端'
  } finally {
    loading.value = false
  }
}

function renderChart(bars: QuoteBar[], preserveZoom = false) {
  if (!chartEl.value) return
  lastBars = bars
  // 图例点击触发的重绘（preserveZoom）保留用户缩放区间；新查询则重置为全量
  const zoomRaw = preserveZoom ? ((chart?.getOption().dataZoom ?? []) as Array<{ start?: number; end?: number }>) : []
  const prevZoom = zoomRaw[0]?.start != null ? { start: zoomRaw[0].start, end: zoomRaw[0].end } : null
  if (!chart) {
    chart = echarts.init(chartEl.value)
    // 图例点击：同步用户手动调整到 seriesVisible / groupToggles，查询切换时保持
    chart.on('legendselectchanged', (e: unknown) => {
      const ev = e as { name?: string; selected?: Record<string, boolean> }
      const sel = ev.selected ?? {}
      const toggledName = ev.name ?? ''
      const updateName = (n: string) => {
        if (n in seriesVisible && typeof sel[n] === 'boolean') seriesVisible[n] = sel[n]
      }
      if (toggledName === '均线') {
        // 均线总开关：占位 series 使其成为真实 series，点击可正常翻转选中态；一键全部显示/隐藏，成员跟随
        const next = sel['均线'] === true
        maNames.forEach((n) => (seriesVisible[n] = next))
        groupToggles.均线 = next
      } else if (toggledName === '布林') {
        // 布林总开关：占位 series 保证可翻转；一键全部显示/隐藏，成员跟随
        const next = sel['布林'] === true
        bollNames.forEach((n) => (seriesVisible[n] = next))
        groupToggles.布林 = next
      } else {
        // 普通成员：只切换该成员；若成员全开/全关则联动总开关状态
        updateName(toggledName)
        if (maNames.includes(toggledName)) groupToggles.均线 = groupAllShown(maNames)
        else if (bollNames.includes(toggledName)) groupToggles.布林 = groupAllShown(bollNames)
      }
      // 重绘：series/legend 按当前显隐状态重建（notMerge），保留 dataZoom 缩放区间。
      // 不能只 setOption 更新 legend——ECharts 在 dispatchAction 点击流程后不会重算 series 显隐
      renderChart(lastBars, true)
    })
  }

  const labels = bars.map((b) => formatAxisLabel(b))
  const candleData = bars.map((b) => [
    toNum(b.open) ?? 0,
    toNum(b.close) ?? 0,
    toNum(b.low) ?? 0,
    toNum(b.high) ?? 0,
  ])
  const volumes = bars.map((b) => b.volume ?? 0)
  const closes = bars.map((b) => toNum(b.close))
  // 均线序列（MA5/10/20/60/120/250），显隐由 seriesVisible 控制
  const maSeries = MA_CONFIGS.map((c) => ({
    name: `MA${c.n}`,
    type: 'line' as const,
    data: computeMA(closes, c.n),
    smooth: true,
    showSymbol: false,
    connectNulls: false,
    // 顶层 color 与 lineStyle.color 同源：图例 icon 与图中线颜色必须一致
    color: c.color,
    lineStyle: { width: 1.1, color: c.color },
    emphasis: { disabled: true },
    z: 3,
  }))
  // 布林带三条线（20 周期，2 倍标准差）：上轨 / 中轨 / 下轨
  const [bollUp, bollMid, bollLow] = computeBOLL(closes, 20, 2)
  // 顶层 color 与 lineStyle.color 同源：图例 icon 与图中线颜色必须一致
  const bollSeries = [
    { name: BOLL_SERIES[0].name, type: 'line' as const, data: bollUp, smooth: true, showSymbol: false, connectNulls: false, color: BOLL_SERIES[0].color, lineStyle: { width: 1, color: BOLL_SERIES[0].color }, emphasis: { disabled: true }, z: 2 },
    { name: BOLL_SERIES[1].name, type: 'line' as const, data: bollMid, smooth: true, showSymbol: false, connectNulls: false, color: BOLL_SERIES[1].color, lineStyle: { width: 1, color: BOLL_SERIES[1].color }, emphasis: { disabled: true }, z: 2 },
    { name: BOLL_SERIES[2].name, type: 'line' as const, data: bollLow, smooth: true, showSymbol: false, connectNulls: false, color: BOLL_SERIES[2].color, lineStyle: { width: 1, color: BOLL_SERIES[2].color }, emphasis: { disabled: true }, z: 2 },
  ]

  chart.setOption(
    {
      animation: false,
      backgroundColor: '#1e1e1e',
      axisPointer: { link: [{ xAxisIndex: 'all' }] },
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross', label: { backgroundColor: '#3a3a3a' } },
        backgroundColor: '#2d2d2d',
        borderColor: '#444444',
        textStyle: { color: '#eee', fontSize: 12 },
        formatter: (params: unknown) => {
          const list = params as Array<{ dataIndex: number; seriesName: string; value: number | number[] | null }>
          if (!list?.length) return ''
          const index = list[0].dataIndex
          const bar = bars[index]
          if (!bar) return ''
          const price = (v: string | null) => (v !== null && v !== undefined ? Number(v).toFixed(4) : '—')
          const up = (toNum(bar.close) ?? 0) >= (toNum(bar.open) ?? 0)
          const upColor = up ? COLOR_UP : COLOR_DOWN
          // 两列布局：第一列指标名，第二列值左对齐
          const row = (label: string, valueHtml: string) =>
            `<tr><td style="padding:1px 14px 1px 0">${label}</td><td style="text-align:left">${valueHtml}</td></tr>`
          const rows: string[] = [
            // 第一行：yyyy-MM-dd HH:mm:ss（跨两列加粗标题）
            `<tr><td colspan="2" style="padding-bottom:2px"><b>${formatDateDash(bar.date)} ${formatTime(bar.time)}</b></td></tr>`,
            // 第二行：时区标注单独成行（灰色小号），避免与日期时间同行撑宽悬浮框
            `<tr><td colspan="2" style="padding-bottom:4px;color:#999;font-size:11px">${TZ_LABEL}</td></tr>`,
            row('开盘', price(bar.open)),
            row('收盘', `<span style="color:${upColor}">${price(bar.close)}</span>`),
            row('最高', price(bar.high)),
            row('最低', price(bar.low)),
            // 涨跌额不再重复展示括号中的涨跌幅（下方已有独立涨跌幅行）
            row('涨跌额', bar.change ? `<span style="color:${upColor}">${bar.change}</span>` : '—'),
            row('涨跌幅', bar.change_pct ? `<span style="color:${upColor}">${Number(bar.change_pct).toFixed(2)}%</span>` : '—'),
          ]
          if (bar.volume !== null && bar.volume !== undefined) rows.push(row('成交量', formatGroup4(Number(bar.volume))))
          if (bar.turnover) rows.push(row('成交额', formatGroup4(Number(bar.turnover))))
          // 均线：每个指标单独一行展示
          MA_CONFIGS.forEach((c) => {
            const v = computeMA(closes, c.n)[index]
            if (v !== null) rows.push(row(`MA${c.n}`, v.toFixed(4)))
          })
          // 布林带三条线值：每个指标单独一行展示
          const bollVals = [
            { label: BOLL_SERIES[0].name, v: bollUp[index] },
            { label: BOLL_SERIES[1].name, v: bollMid[index] },
            { label: BOLL_SERIES[2].name, v: bollLow[index] },
          ]
          bollVals.forEach((x) => {
            if (x.v !== null) rows.push(row(x.label, x.v.toFixed(4)))
          })
          return `<table style="border-collapse:collapse">${rows.join('')}</table>`
        },
      },
      // 三行图例：K线 / 均线(总开关+6条) / 布林(总开关+3轨)。总开关与成员显隐联动。
      legend: renderLegendOption(),
      grid: [
        { left: 64, right: 8, top: 58, height: '55%' },
        { left: 64, right: 8, top: '72%', height: '17%' },
      ],
      xAxis: [
        {
          type: 'category',
          data: labels,
          boundaryGap: true,
          axisLine: { lineStyle: { color: '#3a3a3a' } },
          axisTick: { show: false },
          // 整点刻度：1 日模式标整点，5 日模式 0 点标日期 + 4/8/12/16/20 点标时刻
          axisLabel: {
            color: '#aaa',
            fontSize: 11,
            hideOverlap: true,
            formatter: (val: string) => {
              const idx = labels.indexOf(val)
              if (idx < 0 || idx >= bars.length) return ''
              return xAxisLabelFor(bars[idx])
            },
          },
        },
        {
          type: 'category',
          data: labels,
          gridIndex: 1,
          axisLine: { lineStyle: { color: '#3a3a3a' } },
          axisTick: { show: false },
          axisLabel: { show: false },
        },
      ],
      yAxis: [
        {
          scale: true,
          position: 'left',
          // 价格轴左侧展示，去除千分位（直接数值格式）
          axisLabel: { color: '#aaa', fontSize: 11, formatter: (v: number) => String(v) },
          axisLine: { show: false },
          axisTick: { show: false },
          splitLine: { lineStyle: { color: '#333333' } },
        },
        {
          gridIndex: 1,
          position: 'right',
          axisLabel: { show: false },
          axisLine: { show: false },
          axisTick: { show: false },
          splitLine: { show: false },
        },
      ],
      dataZoom: [{ type: 'inside', xAxisIndex: [0, 1], start: 0, end: 100 }],
      series: [
        // 占位 series（隐藏且无数据）：让「均线」「布林」成为真实 series，
        // ECharts 才会渲染这两个总开关图例项，点击才能正常翻转其选中态
        { name: '均线', type: 'line', data: [], silent: true, tooltip: { show: false }, legendHoverLink: false, itemStyle: { color: '#aaa' }, emphasis: { disabled: true }, z: -10 },
        { name: '布林', type: 'line', data: [], silent: true, tooltip: { show: false }, legendHoverLink: false, itemStyle: { color: '#aaa' }, emphasis: { disabled: true }, z: -10 },
        {
          name: 'K线',
          type: 'candlestick',
          data: candleData,
          itemStyle: {
            color: COLOR_UP,
            color0: COLOR_DOWN,
            borderColor: COLOR_UP,
            borderColor0: COLOR_DOWN,
          },
        },
        ...maSeries,
        ...bollSeries,
        {
          name: 'K线', // 与 K 线图同名：图例点击「K线」同时控制 K 线与成交量显隐
          type: 'bar',
          xAxisIndex: 1,
          yAxisIndex: 1,
          data: volumes,
          barWidth: '60%',
          itemStyle: {
            color: (param: unknown) => {
              const p = param as { dataIndex: number }
              const bar = bars[p.dataIndex]
              const up = (toNum(bar?.close) ?? 0) >= (toNum(bar?.open) ?? 0)
              return up ? COLOR_UP : COLOR_DOWN
            },
          },
        },
      ],
    },
    true,
  )
  // 图例点击重绘时保留用户缩放区间（dataZoom 不因显隐切换重置）
  if (prevZoom && chart) chart.setOption({ dataZoom: [{ start: prevZoom.start, end: prevZoom.end }] })
}
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-chart-line" class="mr-2" />
      历史行情查询
    </v-card-title>
    <v-card-subtitle>
      按证券代码 + 交易日查询分钟级 K 线；支持 1 日 / 5 日切换与分页翻页；红涨绿跌，悬停查看详情
    </v-card-subtitle>

    <v-card-text>
      <v-form @submit.prevent="query">
        <v-row align="center" dense>
          <v-col cols="12" sm="3">
            <!-- 证券代码：下拉字典（usc:security_name_cn）+ 手动输入，默认 NVDA -->
            <v-combobox
              v-model="secuCode"
              :items="secuItems"
              label="证券代码"
              placeholder="如: NVDA"
              density="compact"
              hide-details
              clearable
              @update:model-value="onSecuCodeInput"
            />
          </v-col>
          <v-col cols="12" sm="3">
            <v-menu v-model="dateMenu" :close-on-content-click="false" transition="scale-transition">
              <template #activator="{ props }">
                <v-text-field
                  v-model="dateInput"
                  label="交易日期"
                  placeholder="点击日历选择"
                  readonly
                  density="compact"
                  hide-details
                  prepend-inner-icon="mdi-calendar"
                  v-bind="props"
                />
              </template>
              <v-date-picker
                v-model="dateInput"
                :max="todayISO"
                show-adjacent-months
                @update:model-value="onDatePicked"
              />
            </v-menu>
          </v-col>
          <v-col cols="12" sm="2">
            <v-select
              v-model="period"
              label="周期"
              :items="[{ title: '1分（Min）', value: 'Min' }]"
              density="compact"
              hide-details
            />
          </v-col>
          <v-col cols="12" sm="2">
            <v-btn-toggle v-model="days" mandatory density="comfortable" variant="outlined" divided>
              <v-btn :value="1" size="small">1日</v-btn>
              <v-btn :value="5" size="small">5日</v-btn>
            </v-btn-toggle>
          </v-col>
          <v-col cols="12" sm="2" class="text-right">
            <v-btn type="submit" color="primary" :loading="loading" prepend-icon="mdi-magnify">
              查询
            </v-btn>
          </v-col>
        </v-row>
      </v-form>

      <v-alert v-if="error" type="error" class="mt-3" density="compact">{{ error }}</v-alert>
    </v-card-text>

    <!-- 图表占满卡片可用宽度，左右不留空隙 -->
    <div ref="chartEl" class="w-100" style="width: 100%; height: 560px" />

    <v-card-text class="pt-0">
      <v-row align="center" justify="space-between" dense>
        <v-col cols="auto" class="text-body-2 text-medium-emphasis">
          共 {{ total }} 根 · 滚轮/拖拽平移查看
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>

  <!-- 查询结果提示：3 秒自动消失 -->
  <v-snackbar v-model="toast.visible" :timeout="3000" color="info" location="top" rounded="lg">
    <v-icon icon="mdi-information-outline" class="mr-2" />
    {{ toast.text }}
    <template #actions>
      <v-btn variant="text" icon="mdi-close" @click="toast.visible = false" />
    </template>
  </v-snackbar>
</template>
