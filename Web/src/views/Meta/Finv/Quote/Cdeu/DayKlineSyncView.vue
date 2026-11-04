<script setup lang="ts">
// 日K线数据同步：选 usc → 拉取 moomoo 全量日K → upsert 到 cdeu.finv_quote_secu_kline_day（需求 10952）
// 接口前缀：/Meta/Finv/Quote/Cdeu/DayKline
//   GET  /UscList  futu 采集表中已启用（flag_enable='1'）的 usc 候选
//   POST /Sync     拉取并落库（请求体 {usc, from, to}，from/to 为 yyyyMMdd，0=不限）
//   GET  /Stat     查询某 usc 的日K存量（行数 / 日期范围 / 来源）
// 日志：服务端前缀 [cdeu][daykline]，线上排查用
//   docker logs <容器> 2>&1 | grep '\[cdeu\]\[daykline\]'
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 候选 usc（来自 futu.finv_futu_quote_collect）；title/subtitle 供下拉显示
interface UscOption {
  usc: string
  futu_symbol: string
  quote_market: string
  type_symbol: string
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
    quote_market: string
    type_symbol: string
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

const canSync = computed(() => usc.value.trim().length > 0 && !syncing.value)

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
      list: { usc: string; futu_symbol: string; quote_market: string; type_symbol: string }[]
    }>(`${API_PREFIX}/UscList?limit=500`)
    options.value = (data.list ?? []).map((item) => ({
      ...item,
      title: item.usc,
      subtitle: `${item.futu_symbol} · ${item.quote_market} · ${item.type_symbol}`,
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
  }
}

onMounted(async () => {
  await loadOptions()
  await loadStat()
})
</script>

<template>
  <v-card>
    <v-card-title>
      <v-icon icon="mdi-chart-candlestick" class="mr-2" />
      日K线数据同步
    </v-card-title>
    <v-card-subtitle>
      从 moomoo 网页行情接口拉取全量日K，按 (date, usc) upsert 到
      cdeu.finv_quote_secu_kline_day（请求参数取自 futu.finv_futu_quote_collect）
    </v-card-subtitle>

    <v-card-text>
      <v-alert type="info" density="compact" variant="tonal" class="mb-3">
        服务端日志前缀 <code>[cdeu][daykline]</code>，抓取命令：
        <code>docker logs 容器名 2&gt;&amp;1 | grep '\[cdeu\]\[daykline\]'</code>
      </v-alert>

      <v-row align="center" dense class="mb-3">
        <v-col cols="12" sm="4">
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
          />
        </v-col>
        <v-col cols="6" sm="2">
          <v-text-field
            v-model="from"
            label="起始日 yyyyMMdd"
            placeholder="留空=不限"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="6" sm="2">
          <v-text-field
            v-model="to"
            label="结束日 yyyyMMdd"
            placeholder="留空=不限"
            density="compact"
            hide-details
            clearable
          />
        </v-col>
        <v-col cols="auto">
          <v-btn
            color="primary"
            prepend-icon="mdi-cloud-download"
            :loading="syncing"
            :disabled="!canSync"
            @click="sync"
          >
            同步日K
          </v-btn>
        </v-col>
        <v-col cols="auto">
          <v-btn
            color="secondary"
            prepend-icon="mdi-database-eye-outline"
            :loading="statLoading"
            @click="loadStat"
          >
            查询存量
          </v-btn>
        </v-col>
      </v-row>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-row dense>
        <v-col cols="12" md="6">
          <v-card variant="outlined">
            <v-card-title class="text-subtitle-1">本次同步结果</v-card-title>
            <v-card-text>
              <div v-if="!result" class="text-medium-emphasis">尚未执行同步</div>
              <template v-else>
                <v-table density="compact">
                  <tbody>
                    <tr>
                      <td class="text-medium-emphasis">usc / 时区</td>
                      <td>{{ result.usc }} / {{ result.timezone }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis">接口返回根数</td>
                      <td>{{ result.total }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis">区间内 / 新增 / 更新</td>
                      <td>{{ result.in_range }} / {{ result.insert }} / {{ result.update }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis">日期范围</td>
                      <td>{{ result.date_from || '-' }} ~ {{ result.date_to || '-' }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis">provider</td>
                      <td>{{ result.provider }}</td>
                    </tr>
                    <tr>
                      <td class="text-medium-emphasis">请求参数（透传）</td>
                      <td class="text-caption">
                        stockId={{ result.params.stock_id }} · marketType={{ result.params.market_type }} ·
                        marketCode={{ result.params.market_code }} · instrumentType={{ result.params.instrument_type }} ·
                        subInstrumentType={{ result.params.sub_instrument_type }}
                      </td>
                    </tr>
                    <tr v-if="lastSyncAt">
                      <td class="text-medium-emphasis">完成时间</td>
                      <td>{{ lastSyncAt }}</td>
                    </tr>
                  </tbody>
                </v-table>
              </template>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="6">
          <v-card variant="outlined">
            <v-card-title class="text-subtitle-1">存量统计（cdeu.finv_quote_secu_kline_day）</v-card-title>
            <v-card-text>
              <div v-if="!stat" class="text-medium-emphasis">点击「查询存量」加载</div>
              <v-table v-else density="compact">
                <tbody>
                  <tr>
                    <td class="text-medium-emphasis">usc</td>
                    <td>{{ stat.usc }}</td>
                  </tr>
                  <tr>
                    <td class="text-medium-emphasis">行数</td>
                    <td>{{ stat.rows }}</td>
                  </tr>
                  <tr>
                    <td class="text-medium-emphasis">日期范围</td>
                    <td>{{ stat.date_from || '-' }} ~ {{ stat.date_to || '-' }}</td>
                  </tr>
                  <tr>
                    <td class="text-medium-emphasis">provider</td>
                    <td>{{ stat.providers || '-' }}</td>
                  </tr>
                </tbody>
              </v-table>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-expansion-panels v-if="rawResponse" class="mt-3">
        <v-expansion-panel title="原始响应（复制给后端排障）">
          <v-expansion-panel-text>
            <pre class="text-caption">{{ rawResponse }}</pre>
          </v-expansion-panel-text>
        </v-expansion-panel>
      </v-expansion-panels>
    </v-card-text>
  </v-card>
</template>
