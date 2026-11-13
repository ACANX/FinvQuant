<script setup lang="ts">
// 日K线数据同步：选 usc → 拉取 moomoo 全量日K → upsert 到 cdeu.finv_quote_secu_kline_day（需求 10952）
// 接口前缀：/Meta/Finv/Quote/Cdeu/DayKline
//   GET  /UscList  futu 采集表中已启用（flag_enable='1'）的 usc 候选
//   POST /Sync     拉取并落库（请求体 {usc, from, to}，from/to 为 yyyyMMdd，0=不限）
//   GET  /Stat     查询某 usc 的日K存量（行数 / 日期范围 / 来源）
// 日志：服务端前缀 [cdeu][daykline]，线上排查用
//   docker logs <容器> 2>&1 | grep '\[cdeu\]\[daykline\]'
//
// 展示约定（需求 10987）：图标用 @mdi/font 7.4.47 中确实存在的 mdi-candle
// （mdi-chart-candlestick 在该字体版本中不存在，会渲染成空白）；指标用卡片、
// 明细与排障信息分层展示，同步过程给出耗时提示。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 候选 usc（来自 futu.finv_futu_quote_collect）；title/subtitle 供下拉显示
interface UscOption {
  usc: string
  futu_symbol: string
  market_label: string
  instrument: string
  title: string
  subtitle: string
}

// 同步结果（wire 字段 snake_case）
interface DayKlineSyncResult {
  usc: string
  timezone: string
  total: number
  in_range: number
  insert: number
  update: number
  date_from: number
  date_to: number
  provider: string
  params: {
    usc: string
    stock_id: number
    futu_symbol: string
    market_type: number
    market_code: number
    instrument_type: number
    sub_instrument_type: number
    market_label: string
    instrument: string
  }
}

// 存量统计
interface DayKlineStat {
  usc: string
  rows: number
  date_from: number
  date_to: number
  providers: string
}

const API_PREFIX = '/Meta/Finv/Quote/Cdeu/DayKline'
// 拉全量历史 + upsert 可能超过默认 30s，这里单独放宽到 180s（与后端同步耗时匹配）。
const SYNC_TIMEOUT_MS = 180000

const usc = ref('GCMain')
const from = ref('')
const to = ref('')
const syncing = ref(false)
const statLoading = ref(false)
const error = ref('')
const options = ref<UscOption[]>([])
const result = ref<DayKlineSyncResult | null>(null)
const stat = ref<DayKlineStat | null>(null)
// 原始响应（排障用：可整体复制给后端定位）
const rawResponse = ref('')
const lastSyncAt = ref('')
// 同步计时（秒），同步中每秒刷新，便于判断长耗时是否正常
const elapsed = ref(0)
let elapsedTimer: number | null = null

const canSync = computed(() => usc.value.trim().length > 0 && !syncing.value)

/** 本次同步结果的关键指标（卡片展示） */
const syncSummary = computed(() => {
  if (!result.value) return []
  return [
    { label: '接口返回根数', value: result.value.total, color: 'primary' },
    { label: '区间内根数', value: result.value.in_range, color: 'info' },
    { label: '新增', value: result.value.insert, color: 'success' },
    { label: '更新', value: result.value.update, color: 'warning' },
  ]
})

/** 存量统计的关键指标 */
const statSummary = computed(() => {
  if (!stat.value) return []
  return [
    { label: '存量行数', value: stat.value.rows, color: 'primary' },
    { label: '起始日', value: stat.value.date_from || '—', color: 'info' },
    { label: '结束日', value: stat.value.date_to || '—', color: 'info' },
  ]
})

function startElapsed() {
  elapsed.value = 0
  stopElapsed()
  elapsedTimer = window.setInterval(() => {
    elapsed.value += 1
  }, 1000)
}

function stopElapsed() {
  if (elapsedTimer !== null) {
    window.clearInterval(elapsedTimer)
    elapsedTimer = null
  }
}

function parseDate(text: string): number {
  const value = text.trim()
  if (!value) return 0
  const n = Number(value)
  if (!Number.isInteger(n) || n < 19000101 || n > 22001231) return -1
  return n
}

async function loadOptions() {
  try {
    const data = await apiGet<{
      total: number
      list: { usc: string; futu_symbol: string; market_label: string; instrument: string }[]
    }>(`${API_PREFIX}/UscList?limit=500`)
    options.value = (data.list ?? []).map((item) => ({
      ...item,
      title: item.usc,
      subtitle: `${item.futu_symbol} · ${item.market_label} · ${item.instrument}`,
    }))
  } catch (err) {
    // 候选加载失败不阻塞手输 usc
    error.value = `候选 usc 加载失败：${(err as Error).message}`
  }
}

async function loadStat() {
  if (!usc.value.trim()) return
  statLoading.value = true
  try {
    stat.value = await apiGet<DayKlineStat>(
      `${API_PREFIX}/Stat?usc=${encodeURIComponent(usc.value.trim())}`,
    )
  } catch (err) {
    stat.value = null
    error.value = `存量查询失败：${(err as Error).message}`
  } finally {
    statLoading.value = false
  }
}

async function sync() {
  const target = usc.value.trim()
  if (!target) {
    showToast('请先选择或输入 usc', 'warning')
    return
  }
  const fromValue = parseDate(from.value)
  const toValue = parseDate(to.value)
  if (fromValue < 0 || toValue < 0) {
    showToast('起止日期格式应为 yyyyMMdd（可留空）', 'warning')
    return
  }
  if (fromValue > 0 && toValue > 0 && fromValue > toValue) {
    showToast('起始日期不能大于结束日期', 'warning')
    return
  }
  syncing.value = true
  error.value = ''
  rawResponse.value = ''
  startElapsed()
  try {
    const data = await apiPost<DayKlineSyncResult>(
      `${API_PREFIX}/Sync`,
      { usc: target, from: fromValue, to: toValue },
      SYNC_TIMEOUT_MS,
    )
    result.value = data
    rawResponse.value = JSON.stringify(data, null, 2)
    lastSyncAt.value = new Date().toLocaleString('zh-CN', { hour12: false })
    showToast(
      `同步完成：拉取 ${data.total} 根，区间内 ${data.in_range}，新增 ${data.insert}，更新 ${data.update}`,
      'success',
    )
    await loadStat()
  } catch (err) {
    error.value = `同步失败：${(err as Error).message}`
    showToast(error.value, 'error')
  } finally {
    syncing.value = false
    stopElapsed()
  }
}

function resetForm() {
  from.value = ''
  to.value = ''
  result.value = null
  rawResponse.value = ''
  lastSyncAt.value = ''
  error.value = ''
}

onMounted(async () => {
  await loadOptions()
  await loadStat()
})

onBeforeUnmount(stopElapsed)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-candle" class="mr-2" />
      日K线数据同步
      <v-spacer />
      <v-chip v-if="lastSyncAt" color="success" variant="tonal" size="small">
        最近同步 {{ lastSyncAt }}
      </v-chip>
    </v-card-title>
    <v-card-subtitle class="pb-2">
      从 moomoo 网页行情接口拉取全量日K，按 (date, usc) upsert 到
      cdeu.finv_quote_secu_kline_day（请求参数取自 futu.finv_futu_quote_collect）
    </v-card-subtitle>

    <v-card-text>
      <!-- 操作栏：选 usc + 日期区间 + 动作按钮（单行，窄屏自动换行） -->
      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-combobox
          v-model="usc"
          :items="options"
          item-title="title"
          item-value="usc"
          label="usc（可手输）"
          density="compact"
          hide-details
          clearable
          :menu-props="{ maxHeight: 360 }"
          style="min-width: 240px; max-width: 360px"
        />
        <v-text-field
          v-model="from"
          label="起始日 yyyyMMdd"
          placeholder="留空=不限"
          density="compact"
          hide-details
          clearable
          style="max-width: 150px"
        />
        <v-text-field
          v-model="to"
          label="结束日 yyyyMMdd"
          placeholder="留空=不限"
          density="compact"
          hide-details
          clearable
          style="max-width: 150px"
        />
        <v-btn
          color="primary"
          prepend-icon="mdi-cloud-download"
          :loading="syncing"
          :disabled="!canSync"
          @click="sync"
        >
          同步日K
        </v-btn>
        <v-btn
          variant="text"
          prepend-icon="mdi-database-eye-outline"
          :loading="statLoading"
          @click="loadStat"
        >
          查询存量
        </v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="resetForm">重置</v-btn>
      </div>

      <!-- 同步进度提示（全量日K可能耗时数十秒） -->
      <v-alert v-if="syncing" type="info" density="compact" variant="tonal" class="mb-2">
        正在拉取 moomoo 全量日K并写入数据库，已耗时 <strong>{{ elapsed }}</strong> 秒；
        数据量大的 usc 可能需要数十秒，请勿关闭页面。
      </v-alert>
      <v-progress-linear v-if="syncing" indeterminate color="primary" class="mb-3" height="4" rounded />

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-row dense>
        <!-- 本次同步结果 -->
        <v-col cols="12" md="6">
          <v-card variant="outlined" class="h-100">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              本次同步结果
              <v-chip v-if="result" size="x-small" variant="tonal" color="primary" class="ml-2">
                {{ result.usc }}
              </v-chip>
            </v-card-title>
            <v-card-text>
              <div v-if="!result" class="text-medium-emphasis">尚未执行同步</div>
              <template v-else>
                <div class="d-flex flex-wrap ga-2 mb-3">
                  <v-sheet
                    v-for="item in syncSummary"
                    :key="item.label"
                    class="pa-3 rounded stat-card"
                    color="surface-variant"
                  >
                    <div class="text-caption text-medium-emphasis">{{ item.label }}</div>
                    <div class="text-h6" :class="`text-${item.color}`">{{ item.value }}</div>
                  </v-sheet>
                </div>
                <v-table density="compact" class="text-caption">
                  <tbody>
                    <tr>
                      <td class="text-medium-emphasis text-no-wrap">usc / 时区</td>
                      <td>{{ result.usc }} / {{ result.timezone }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis text-no-wrap">写入日期范围</td>
                      <td>{{ result.date_from || '—' }} ~ {{ result.date_to || '—' }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis text-no-wrap">provider</td>
                      <td>{{ result.provider }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </template>
            </v-card-text>
          </v-card>
        </v-col>

        <!-- 存量统计 -->
        <v-col cols="12" md="6">
          <v-card variant="outlined" class="h-100">
            <v-card-title class="text-subtitle-1 d-flex align-center">
              存量统计
              <span class="text-caption text-medium-emphasis ml-2">cdeu.finv_quote_secu_kline_day</span>
            </v-card-title>
            <v-card-text>
              <div v-if="!stat" class="text-medium-emphasis">
                点击「查询存量」按当前 usc 加载
              </div>
              <template v-else>
                <div class="d-flex flex-wrap ga-2 mb-3">
                  <v-sheet
                    v-for="item in statSummary"
                    :key="item.label"
                    class="pa-3 rounded stat-card"
                    color="surface-variant"
                  >
                    <div class="text-caption text-medium-emphasis">{{ item.label }}</div>
                    <div class="text-h6" :class="`text-${item.color}`">{{ item.value }}</div>
                  </v-sheet>
                </div>
                <v-table density="compact" class="text-caption">
                  <tbody>
                    <tr>
                      <td class="text-medium-emphasis text-no-wrap">usc</td>
                      <td>{{ stat.usc }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis text-no-wrap">provider</td>
                      <td>{{ stat.providers || '—' }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </template>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <!-- 排障信息：请求参数透传 / 原始响应 / 日志命令（默认收起，避免占版面） -->
      <v-expansion-panels class="mt-3" variant="accordion">
        <v-expansion-panel title="排障信息（请求参数 / 原始响应 / 日志命令）">
          <v-expansion-panel-text>
            <div class="text-subtitle-2 mb-1">请求参数（透传给 moomoo）</div>
            <v-table v-if="result" density="compact" class="text-caption mb-3">
              <tbody>
                <tr>
                  <td class="text-medium-emphasis">market_label / instrument</td>
                  <td>{{ result.params.market_label }} / {{ result.params.instrument }}</td>
                </tr>
                <tr>
                  <td class="text-medium-emphasis">stock_id / futu_symbol</td>
                  <td>{{ result.params.stock_id }} / {{ result.params.futu_symbol }}</td>
                </tr>
                <tr>
                  <td class="text-medium-emphasis">market_type / market_code</td>
                  <td>{{ result.params.market_type }} / {{ result.params.market_code }}</td>
                </tr>
                <tr>
                  <td class="text-medium-emphasis">instrument_type / sub_instrument_type</td>
                  <td>{{ result.params.instrument_type }} / {{ result.params.sub_instrument_type }}</td>
                </tr>
              </tbody>
            </v-table>
            <div v-else class="text-medium-emphasis mb-3">同步一次后可查看</div>

            <div class="text-subtitle-2 mb-1">日志命令</div>
            <pre class="text-caption pa-2 rounded bg-surface-variant">docker logs &lt;容器名&gt; 2&gt;&amp;1 | grep '[cdeu][daykline]'</pre>

            <div v-if="rawResponse" class="text-subtitle-2 mb-1 mt-3">原始响应</div>
            <pre v-if="rawResponse" class="text-caption pa-2 rounded bg-surface-variant">{{ rawResponse }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.stat-card {
  min-width: 112px;
}
</style>
