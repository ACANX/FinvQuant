<script setup lang="ts">
// 缺省汇率（需求 10999，11000 增加矩阵模式/中文名称）
// 接口：
//   GET  /Meta/Finv/Quote/Cdeu/Fx/Cer        整表查询（88 行量级，不分页；含 currency_names）
//   POST /Meta/Finv/Quote/Cdeu/Fx/CerSync    手动更新（同步执行，一次 moomoo get-fx-overview）
//
// 数据表：public.finv_fxcer（V300124 创建，主键 usc，一行=一个货币对的最新汇率，只保最新、不建历史表）
// 口径：cer = 1 单位 currency_src 折合的 currency_dest 数量（USDCNH=6.67 即 1 USD = 6.67 CNH）
// 需求 11020 起只落直采行：概览 72 条（9 币种两两组合）、列表 280 条（币种更多），不再派生 CNY 行
// 两种展示：矩阵模式（默认，行=源币种、列=目标币种）/ 表格模式（逐行明细）
// 更新节奏：当前仅支持本页手动触发，定期更新后续实现。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 一行汇率（字段与 public.finv_fxcer 列一致，rate 为文本以保住 NUMERIC(24,10) 精度）
interface FxCERRow {
  usc: string
  name: string
  currency_src: string
  currency_dest: string
  rate: string
  // 涨跌额/涨跌幅（需求 11016）：仅表格模式的 rateList 来源提供，概览来源为空
  change_price?: string
  change_ratio?: string
  ts_quote: number
  source: string
  flag_derive: string
  remark?: string
  gmt_update?: string
}

// 一次手动更新的结果
interface FxCERSyncResult {
  source: string
  pages: number
  rows_receive: number
  derive: number
  insert: number
  update: number
  ts_quote_max: number
  elapsed_ms: number
}

// 表格/矩阵展示模式
type ViewMode = 'matrix' | 'table'

const LIST_API = '/Meta/Finv/Quote/Cdeu/Fx/Cer'
const SYNC_API = '/Meta/Finv/Quote/Cdeu/Fx/CerSync'

// 采集来源（需求 11001）：矩阵模式用概览（9 币种完整矩阵），表格模式用列表（币种更多）
const SOURCE_LABELS: Record<string, string> = {
  overview: '汇率概览',
  rateList: '汇率列表',
}

// 矩阵行列的固定顺序：人民币口径在前（在岸/离岸相邻，便于对照），其余按常用度
// 矩阵行列固定顺序（需求 11003/11004）：主要币种在前（GBP 在 AUD 前、CHF 在 PHP 前），贵金属 XAU/XAG 紧随其后；
// 未列入的币种按字母序追加到末尾（保持原有相对顺序）。
// 注：需求里写的「TWB」按数据中实际存在的 TWD（新台币）处理。
const CURRENCY_ORDER = [
  'USD', 'CNY', 'CNH', 'HKD', 'EUR', 'JPY', 'SGD', 'GBP', 'AUD', 'CAD',
  'NZD', 'CHF', 'PHP', 'IDR', 'ZAR', 'RUB', 'THB', 'TWD', 'XAU', 'XAG',
]

const rows = ref<FxCERRow[]>([])
// 币种中文名（后端 FxCERCurrencyNames 下发；未登记的币种只显示代码）
const currencyNames = ref<Record<string, string>>({})
const loading = ref(false)
const syncing = ref(false)
const error = ref('')
const viewMode = ref<ViewMode>('matrix')

const directCount = computed(() => rows.value.filter((r) => r.flag_derive !== '1').length)
const deriveCount = computed(() => rows.value.filter((r) => r.flag_derive === '1').length)
const tsQuoteMax = computed(() => rows.value.reduce((max, r) => (r.ts_quote > max ? r.ts_quote : max), 0))
const gmtUpdateMax = computed(() => {
  let max = 0
  for (const row of rows.value) {
    if (!row.gmt_update) continue
    const ts = new Date(row.gmt_update).getTime()
    if (Number.isFinite(ts) && ts > max) max = ts
  }
  return max
})
// 超过 1 天没更新就提示（手动触发，没有调度兜底）
const staleHours = computed(() => {
  if (!tsQuoteMax.value) return 0
  return Math.floor((Date.now() - tsQuoteMax.value) / 3600000)
})

// 矩阵行列币种：固定顺序优先，未登记的新币种按字母序追加
const currencies = computed(() => {
  const found = new Set<string>()
  for (const row of rows.value) {
    found.add(row.currency_src)
    found.add(row.currency_dest)
  }
  const ordered = CURRENCY_ORDER.filter((code) => found.has(code))
  const extras = [...found].filter((code) => !CURRENCY_ORDER.includes(code)).sort()
  return [...ordered, ...extras]
})

// 货币对索引：`src|dest` → 行
const rateIndex = computed(() => {
  const index = new Map<string, FxCERRow>()
  for (const row of rows.value) {
    index.set(`${row.currency_src}|${row.currency_dest}`, row)
  }
  return index
})

function currencyName(code: string): string {
  return currencyNames.value[code] || ''
}

function pairName(row: FxCERRow): string {
  const name = (row.name || '').trim()
  if (name) return name
  const from = currencyName(row.currency_src) || row.currency_src
  const to = currencyName(row.currency_dest) || row.currency_dest
  return `${from}/${to}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await apiGet<{
      total: number
      list: FxCERRow[]
      currency_names?: Record<string, string>
    }>(LIST_API)
    rows.value = data.list ?? []
    currencyNames.value = data.currency_names ?? {}
  } catch (err) {
    error.value = (err as Error).message
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function syncNow() {
  // 矩阵模式 → get-fx-overview（9 币种两两完整矩阵）；表格模式 → get-fx-rate-list（币种更多，补长尾）
  const source = viewMode.value === 'matrix' ? 'overview' : 'rateList'
  syncing.value = true
  error.value = ''
  try {
    // 一次上游请求 + 一次批量 upsert，正常 1–2 s（列表 282 条略久）；给 60 s 上限，避免默认 30 s 超时误判。
    const result = await apiPost<FxCERSyncResult>(`${SYNC_API}?source=${source}`, {}, 60_000)
    const pagesNote = result.pages > 1 ? `（分 ${result.pages} 页）` : ''
    showToast(
      `${SOURCE_LABELS[result.source] ?? result.source}已更新${pagesNote}：收到 ${result.rows_receive} 条（派生 ${result.derive}，已停用）、新增 ${result.insert}、覆盖 ${result.update}`,
      'success',
    )
    await load()
  } catch (err) {
    const message = (err as Error).message
    error.value = message
    showToast(`汇率更新失败：${message}`, 'error')
  } finally {
    syncing.value = false
  }
}

// ---- 展示格式化 ----

function trimZero(value: string): string {
  if (!value) return ''
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}

/** 汇率按量级调精度：≥100 保留 4 位、≥1 保留 6 位、<1 保留 8 位（JPYUSD 这类需要更多小数） */
function rateText(value: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return value
  if (n >= 100) return trimZero(n.toFixed(4))
  if (n >= 1) return trimZero(n.toFixed(6))
  return trimZero(n.toFixed(8))
}

/** 涨跌额（带符号）：按汇率同一量级调精度，缺失显示 — */
function changePriceText(value?: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  if (n === 0) return '0'
  const abs = Math.abs(n)
  if (abs >= 100) return trimZero(n.toFixed(4))
  if (abs >= 1) return trimZero(n.toFixed(6))
  return trimZero(n.toFixed(8))
}

/** 涨跌幅（百分数，带正号保留两位）：落库是数值，展示时补「%」 */
function changeRatioText(value?: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`
}

/** 涨跌配色：红涨绿跌（沿用 A 股习惯），0 或缺失不着色 */
function changeClass(value?: string): string {
  if (!value) return ''
  const n = Number(value)
  if (!Number.isFinite(n) || n === 0) return ''
  return n > 0 ? 'finv-fxcer-up' : 'finv-fxcer-down'
}

function timeText(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  if (Number.isNaN(d.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes(),
  )}:${pad(d.getSeconds())}`
}

function updateText(value?: string): string {
  if (!value) return '—'
  const ts = new Date(value).getTime()
  return Number.isFinite(ts) ? timeText(ts) : value
}

/**
 * 矩阵悬浮定位（需求 11015）：记录当前悬浮单元格的行/列币种，
 * 用于让「行首」与「列首」单元格同步高亮（纯 CSS 无法选中列首）。
 */
const hoverSrc = ref('')
const hoverDest = ref('')

function setHover(src: string, dest: string) {
  hoverSrc.value = src
  hoverDest.value = dest
}

function clearHover() {
  hoverSrc.value = ''
  hoverDest.value = ''
}

/** 矩阵单元：对角线、缺失货币对都返回 undefined */
function cell(src: string, dest: string): FxCERRow | undefined {
  if (src === dest) return undefined
  return rateIndex.value.get(`${src}|${dest}`)
}

/**
 * 单元格悬浮提示（需求 11014，11018 增加涨跌额/涨跌幅）：返回逐行文本，由模板按行渲染。
 * v-tooltip 的 text 属性是纯文本，其中的换行符会被折叠，故改为「每行一个 div」。
 * 提示内容统一用提示框默认文字色，不按涨跌着色（需求 11019）。
 */
function cellTooltipLines(src: string, dest: string): string[] {
  if (src === dest) return [`${src} 自身，不构成货币对`]
  const row = cell(src, dest)
  const label = `${src}${currencyName(src) ? '（' + currencyName(src) + '）' : ''} → ${dest}${
    currencyName(dest) ? '（' + currencyName(dest) + '）' : ''
  }`
  if (!row) {
    return [`${label}：无该货币对`, '上游只提供 9 币种两两组合，且不提供 CNH↔CNY']
  }
  const kind = row.flag_derive === '1' ? '派生（历史行，当前采集已停用）' : '直采'

  // 涨跌额/涨跌幅：只有表格模式来源（get-fx-rate-list）采过才有值，
  // 矩阵模式来源不带这两个字段，此时显示 — 并注明来源，避免被误读成 0（需求 11018）。
  const hasChange = Boolean(row.change_price) || Boolean(row.change_ratio)
  const changeLine = hasChange
    ? `涨跌额 ${changePriceText(row.change_price)} · 涨跌幅 ${changeRatioText(row.change_ratio)}`
    : '涨跌额 — · 涨跌幅 —（表格模式采集后才有）'

  return [
    label,
    `1 ${src} = ${rateText(row.rate)} ${dest}`,
    changeLine,
    `报价时间 ${timeText(row.ts_quote)}`,
    `${kind} · ${row.source}`,
  ]
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center flex-wrap ga-2">
      <v-icon icon="mdi-cash-sync" class="mr-2" />
      缺省汇率
      <v-spacer />
      <v-btn-toggle v-model="viewMode" density="compact" variant="outlined" mandatory divided>
        <v-btn value="matrix" prepend-icon="mdi-grid" size="small">矩阵模式</v-btn>
        <v-btn value="table" prepend-icon="mdi-table" size="small">表格模式</v-btn>
      </v-btn-toggle>
      <v-tooltip
        :text="
          viewMode === 'matrix'
            ? '矩阵模式：调用 get-fx-overview（9 币种两两完整矩阵；需求 11020 起不再派生 CNY）'
            : '表格模式：调用 get-fx-rate-list（listId=1、pageSize=500，币种更多，补齐概览缺失的货币对）'
        "
        location="bottom"
      >
        <template #activator="{ props }">
          <v-btn
            v-bind="props"
            color="primary"
            prepend-icon="mdi-cloud-download-outline"
            :loading="syncing"
            class="ml-2"
            @click="syncNow"
          >
            立即更新
          </v-btn>
        </template>
      </v-tooltip>
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
    </v-card-title>
    <v-card-text>
      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-alert
        v-else-if="!loading && rows.length === 0"
        type="warning"
        density="compact"
        variant="tonal"
        class="mb-3"
      >
        表为空：还没有成功更新过汇率，请点击右上角「立即更新」。
      </v-alert>

      <v-alert
        v-else-if="staleHours >= 24"
        type="warning"
        density="compact"
        variant="tonal"
        class="mb-3"
      >
        最新报价已是 {{ staleHours }} 小时前，建议重新更新（暂无定期更新任务）。
      </v-alert>

      <div class="d-flex flex-wrap ga-2 mb-3">
        <v-chip color="primary" variant="tonal" prepend-icon="mdi-database-arrow-up-outline">
          直采 {{ directCount }} 条
        </v-chip>
        <v-chip color="secondary" variant="tonal" prepend-icon="mdi-source-branch">
          派生 {{ deriveCount }} 条（已停用）
        </v-chip>
        <v-chip variant="tonal" prepend-icon="mdi-clock-outline">
          最新报价 {{ timeText(tsQuoteMax) }}
        </v-chip>
        <v-chip variant="tonal" prepend-icon="mdi-update">
          最近更新 {{ timeText(gmtUpdateMax) }}
        </v-chip>
      </div>

      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />

      <!-- 矩阵模式（默认）：行=源币种，列=目标币种，单元格=1 源币种折合的目标币种数量 -->
      <template v-if="viewMode === 'matrix'">
        <div class="finv-fxcer-scroll" @mouseleave="clearHover()">
        <v-table density="compact" class="text-caption finv-fxcer-matrix">
          <thead>
            <tr>
              <!-- 角单元格（需求 11012）：对角线拆两区，右上三角 = 列方向「目标」，左下三角 = 行方向「源」 -->
              <th class="text-no-wrap finv-fxcer-corner-th">
                <v-tooltip text="行 = 源币种，列 = 目标币种" location="top" open-delay="200">
                  <template #activator="{ props }">
                    <div v-bind="props" class="finv-fxcer-corner">
                      <svg
                        class="finv-fxcer-corner-line"
                        viewBox="0 0 100 100"
                        preserveAspectRatio="none"
                        aria-hidden="true"
                      >
                        <line
                          x1="0"
                          y1="0"
                          x2="100"
                          y2="100"
                          stroke="currentColor"
                          stroke-width="1"
                          vector-effect="non-scaling-stroke"
                        />
                      </svg>
                      <span class="finv-fxcer-corner-dest">目标</span>
                      <span class="finv-fxcer-corner-src">源</span>
                    </div>
                  </template>
                </v-tooltip>
              </th>
              <th
                v-for="dest in currencies"
                :key="`h-${dest}`"
                class="text-no-wrap text-center"
                :class="{ 'finv-fxcer-axis-hover': hoverDest === dest }"
                @mouseenter="setHover('', dest)"
              >
                <div class="font-weight-medium">{{ dest }}</div>
                <div class="text-caption font-weight-regular finv-fxcer-name">
                  {{ currencyName(dest) || '—' }}
                </div>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="src in currencies" :key="`r-${src}`">
              <th
                class="text-no-wrap"
                :class="{ 'finv-fxcer-axis-hover': hoverSrc === src }"
                @mouseenter="setHover(src, '')"
              >
                <span class="font-weight-medium">{{ src }}</span>
                <span class="text-caption font-weight-regular finv-fxcer-name ml-1">
                  {{ currencyName(src) || '—' }}
                </span>
              </th>
              <td
                v-for="dest in currencies"
                :key="`c-${src}-${dest}`"
                class="text-center text-no-wrap"
                :class="{ 'finv-fxcer-self': src === dest, 'finv-fxcer-derived': cell(src, dest)?.flag_derive === '1' }"
                @mouseenter="setHover(src, dest)"
                @mouseleave="clearHover()"
              >
                <v-tooltip location="top" open-delay="200">
                  <template #activator="{ props }">
                    <span
                      v-bind="props"
                      class="font-monospace"
                      :class="changeClass(cell(src, dest)?.change_ratio)"
                    >
                      {{ cell(src, dest) ? rateText(cell(src, dest)!.rate) : '—' }}
                      <sup v-if="cell(src, dest)?.flag_derive === '1'">*</sup>
                    </span>
                  </template>
                  <div class="finv-fxcer-tip">
                    <div v-for="(line, i) in cellTooltipLines(src, dest)" :key="i">{{ line }}</div>
                  </div>
                </v-tooltip>
              </td>
            </tr>
          </tbody>
        </v-table>
        </div>
      </template>

      <!-- 表格模式：逐行明细，币种代码后附中文名称，避免误读 -->
      <div v-else class="finv-fxcer-scroll">
      <v-table density="compact" class="text-caption finv-fxcer-table">
        <thead>
          <tr>
            <th class="text-no-wrap">代码</th>
            <th class="text-no-wrap">币种名称</th>
            <th class="text-no-wrap">源币种</th>
            <th class="text-no-wrap">目标币种</th>
            <th class="text-no-wrap text-right">缺省汇率</th>
            <th class="text-no-wrap text-right">涨跌额</th>
            <th class="text-no-wrap text-right">涨跌幅</th>
            <th class="text-no-wrap">类型</th>
            <th class="text-no-wrap">来源</th>
            <th class="text-no-wrap">备注</th>
            <th class="text-no-wrap">报价时间</th>
            <th class="text-no-wrap">更新时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.usc">
            <td class="text-no-wrap font-monospace">{{ row.usc }}</td>
            <td class="text-no-wrap">{{ pairName(row) }}</td>
            <td class="text-no-wrap font-weight-medium">{{ row.currency_src }}</td>
            <td class="text-no-wrap font-weight-medium">{{ row.currency_dest }}</td>
            <td class="text-no-wrap text-right font-monospace">{{ rateText(row.rate) }}</td>
            <td class="text-no-wrap text-right font-monospace" :class="changeClass(row.change_price)">
              {{ changePriceText(row.change_price) }}
            </td>
            <td class="text-no-wrap text-right font-monospace" :class="changeClass(row.change_ratio)">
              {{ changeRatioText(row.change_ratio) }}
            </td>
            <td class="text-no-wrap">
              <v-chip v-if="row.flag_derive === '1'" size="x-small" color="secondary" variant="tonal">
                派生
              </v-chip>
              <v-chip v-else size="x-small" color="primary" variant="tonal">直采</v-chip>
            </td>
            <td class="text-no-wrap"><code class="text-caption">{{ row.source }}</code></td>
            <td class="finv-fxcer-remark">{{ row.remark || '—' }}</td>
            <td class="text-no-wrap">{{ timeText(row.ts_quote) }}</td>
            <td class="text-no-wrap">{{ updateText(row.gmt_update) }}</td>
          </tr>
        </tbody>
      </v-table>
      </div>
    </v-card-text>
  </v-card>

  <!-- 说明卡片（需求 11005 平移，11006 加标题与分节样式）：三段说明统一放在表格下方 -->
  <v-card class="mt-4">
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-information-outline" class="mr-2" />
      说明
    </v-card-title>
    <v-card-text>
      <div class="finv-fxcer-note">
        <div class="finv-fxcer-note-title">表口径</div>
        <div class="text-body-2 finv-fxcer-note-body">
          <code>public.finv_fxcer</code>：一行 = 一个货币对的最新汇率（<code>usc</code> 为主键，<code>name</code> 为中文名），
          <code>rate</code> = 1 单位源币种折合的目标币种数量；只保最新、不建历史表。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-fxcer-note">
        <div class="finv-fxcer-note-title">数据源与互补</div>
        <div class="text-body-2 finv-fxcer-note-body">
          数据源 moomoo <code>get-fx-overview</code>：一次拉取 72 个货币对（9 币种两两组合，正反两向都落库）。
          该接口只提供离岸人民币 <code>CNH</code>；<code>CNY</code> 相关货币对由表格模式的
          <code>get-fx-rate-list</code> 直采补齐，<strong>不再按 CNH 同价派生</strong>（派生值不等于在岸真实价，
          会污染换算与回测口径）。<strong>人民币（CNY）与离岸人民币（CNH）价格存在基差</strong>，换算时不要混用。
        </div>
        <div class="text-body-2 finv-fxcer-note-body mt-2">
          两个模式各自触发对应来源的更新：矩阵模式用概览（完整矩阵）、表格模式用汇率列表（币种更多），
          两者写入同一张表、互不删除，可相互补齐缺失的货币对。
        </div>
      </div>

      <template v-if="viewMode === 'matrix'">
        <v-divider class="my-3" />
        <div class="finv-fxcer-note">
          <div class="finv-fxcer-note-title">矩阵读法</div>
          <div class="text-caption finv-fxcer-name">
            单元格 = <strong>1 行首币种 = N 列首币种</strong>（例：行 USD、列 CNH 的 6.67333 表示 1 USD = 6.67333 CNH）。
            <code>*</code> 为历史派生行标记（当前采集不再产生派生行）；<code>—</code> 为无该货币对
            （对角线、以及上游不提供的 CNH↔CNY）。鼠标悬浮可看币种对、汇率、涨跌额/涨跌幅、
            报价时间与来源。
            汇率数值按涨跌幅着色：<span class="finv-fxcer-up">涨为红色</span>、
            <span class="finv-fxcer-down">跌为绿色</span>，没有涨跌数据（矩阵模式采集不带涨跌字段）时按默认色显示。
          </div>
        </div>
      </template>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.finv-fxcer-table,
.finv-fxcer-matrix {
  font-variant-numeric: tabular-nums;
}

.finv-fxcer-name {
  opacity: 0.7;
}

/* 说明卡片：每段带标题 + 左侧色条，正文保持常规字号，段落之间用 v-divider 分隔 */
.finv-fxcer-note {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}

.finv-fxcer-note-title {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgba(var(--v-theme-primary), 1);
  margin-bottom: 2px;
}

.finv-fxcer-note-body {
  opacity: 0.85;
  line-height: 1.6;
}

/* 矩阵单元格（需求 11013/11014）：直角矩形、相邻单元格共用实线边框，行列都有分割线。
   选择器带上 .v-table__wrapper 以压过 Vuetify 自带的单元格 padding / border 规则。 */
.finv-fxcer-matrix :deep(.v-table__wrapper > table) {
  border-collapse: collapse;
  border-spacing: 0;
}

.finv-fxcer-matrix :deep(.v-table__wrapper > table th),
.finv-fxcer-matrix :deep(.v-table__wrapper > table td) {
  border: 1px solid rgba(var(--v-theme-on-surface), 0.18);
  padding: 4px 10px;
  background-color: rgba(var(--v-theme-surface-variant), 0.30);
  transition:
    background-color 0.15s ease,
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    color 0.15s ease;
}

/* 表头行、行首列：底色更深，像卡片的标题栏 */
.finv-fxcer-matrix :deep(.v-table__wrapper > table thead th),
.finv-fxcer-matrix :deep(.v-table__wrapper > table tbody th) {
  background-color: rgba(var(--v-theme-surface-variant), 0.70);
  font-weight: 500;
}

/* 悬浮：变亮 + 主色描边 + 抬升阴影；离开时同样有过渡，不会瞬切 */
.finv-fxcer-matrix :deep(.v-table__wrapper > table tbody td:hover) {
  background-color: rgba(var(--v-theme-primary), 0.16);
  border-color: rgba(var(--v-theme-primary), 0.55);
  box-shadow: 0 2px 10px rgba(var(--v-theme-on-surface), 0.20);
  color: rgb(var(--v-theme-primary));
  font-weight: 500;
  position: relative;
  z-index: 1;
}

/* 悬浮时行首与列首同步高亮：明确「源币种（行）/ 目标币种（列）」两个维度 */
.finv-fxcer-matrix :deep(.v-table__wrapper > table th.finv-fxcer-axis-hover) {
  background-color: rgba(var(--v-theme-primary), 0.10);
  border-color: rgba(var(--v-theme-primary), 0.35);
  color: rgb(var(--v-theme-primary));
  font-weight: 600;
}

/* 币种补齐后矩阵可能到 30+ 列，横向滚动避免撑破卡片 */
.finv-fxcer-scroll {
  overflow-x: auto;
}

.finv-fxcer-remark {
  max-width: 420px;
  white-space: normal;
  word-break: break-word;
}

/* 涨跌配色：红涨绿跌（需求 11016/11017），色值与「证券待录入」表格保持同一套 */
.finv-fxcer-up {
  color: #ff0000;
}

.finv-fxcer-down {
  color: #00e676;
}

.finv-fxcer-self {
  opacity: 0.35;
}

/* 悬浮提示：逐行展示，行距紧凑，数字等宽便于核对 */
.finv-fxcer-tip {
  line-height: 1.5;
  font-variant-numeric: tabular-nums;
}

.finv-fxcer-derived {
  background-color: rgba(var(--v-theme-secondary), 0.16);
}

/* 矩阵角单元格：左上角连到右下角的对角线，把单元格拆成「目标」（右上三角）与「源」（左下三角） */
.finv-fxcer-corner-th {
  vertical-align: middle;
}

.finv-fxcer-corner {
  position: relative;
  width: 66px;
  height: 40px;
}

.finv-fxcer-corner-line {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0.5;
}

.finv-fxcer-corner-dest,
.finv-fxcer-corner-src {
  position: absolute;
  font-size: 0.7rem;
  font-weight: 400;
  line-height: 1;
  opacity: 0.85;
}

.finv-fxcer-corner-dest {
  top: 4px;
  right: 5px;
}

.finv-fxcer-corner-src {
  left: 5px;
  bottom: 4px;
}
</style>
