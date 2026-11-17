<script setup lang="ts">
// 行情采集 · 状态总览（需求 11283）
//
// 一个只读汇总页：把「行情采集」分组下各采集链路的**配置**与**最近一次运行结果**放在同一页，
// 省得逐 Tab 点进去看。数据全部来自既有接口，不新增后端端点：
//   GET /Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run（runId 省略 = 最近一次）
//   → data.configs（证券列表采集配置：stock / etf）、data.futures_configs（期货 8 个分类）、
//     data.run（内存中最近一次运行：状态 / 耗时 / 逐市场结果 / 错误）
//   GET /Meta/Finv/Quote/Secu/Collect/Stats（需求 11285）
//   → data.tables[]（关键表当前行数：指标表 / 采集参数表），表名由服务端白名单固定
//
// 注意：运行记录只存在服务进程内存里（重启即清空），配置来自数据库，因此
// 「配置有值、运行状态 IDLE」是正常现象（见页面底部「说明」卡片）。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

const RUN_API = '/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run'
// 表规模统计（需求 11285）：白名单两张表的当前行数
const STATS_API = '/Meta/Finv/Quote/Secu/Collect/Stats'

// 证券列表采集配置（cdeu.finv_cdeu_secu_realtime_sync_config 一行）
interface SyncConfig {
  instrument_symbol: string
  market_type: string
  count_plan: number | null
  count_exists: number
  total_available: number
}

// 期货分类采集配置（cdeu.finv_quote_cdeu_future_sync_config 一行，含中文名）
interface FuturesConfig {
  market_type: string
  futures_type: string
  futures_type_name: string
  count_plan: number | null
  count_exists: number
  total_available: number
}

// 单个市场 / 分类的运行结果
interface RunResult {
  instrument_symbol: string
  market_type: string
  futures_type?: string
  futures_type_name?: string
  pages: number
  rows_receive: number
  insert: number
  update: number
  count_exists: number
  total_available: number
  elapsed_ms: number
  error?: string
}

// 最近一次运行
interface RunInfo {
  run_id: string
  status: string
  started_at?: string
  finished_at?: string
  elapsed_ms: number
  results: RunResult[]
  error?: string
}

// 后端在「无运行记录」时返回扁平结构（run_id/status/results），有记录时返回 { run, configs, … }；
// 这里统一按「两种形态都能读」处理。
interface RunPayload {
  run?: RunInfo
  run_id?: string
  status?: string
  elapsed_ms?: number
  started_at?: string
  finished_at?: string
  results?: RunResult[]
  configs?: SyncConfig[]
  futures_configs?: FuturesConfig[]
  error?: string
  futures_error?: string
}

// 表规模统计（服务端白名单固定表名，前端只负责展示）
interface TableStat {
  table: string
  rows: number
  exists: boolean
}

interface StatsPayload {
  tables?: TableStat[]
  rows_total?: number
  elapsed_ms?: number
}

const loading = ref(false)
const stats = ref<TableStat[]>([])
const statsError = ref('')
const configs = ref<SyncConfig[]>([])
const futuresConfigs = ref<FuturesConfig[]>([])
const run = ref<RunInfo | null>(null)
const configError = ref('')

// 品种 / 市场 / 分类显示名
const SYMBOL_LABELS: Record<string, string> = {
  stock: '股票',
  etf: 'ETF',
  bsestock: '北交所股票',
  cnetf: '沪深京ETF',
}
const MARKET_LABELS: Record<string, string> = { '1': 'HK 港股', '2': 'US 美股', '4': 'CN A 股', '0': '聚合口径' }

function symbolLabel(value: string): string {
  return SYMBOL_LABELS[value] ?? value
}

function marketLabel(value: string): string {
  return MARKET_LABELS[value] ?? value
}

/** 行数千分位（70,171）：表格与指标卡统一用它，避免各写一份 */
function formatRows(value: number | undefined): string {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString('zh-CN')
}

function futuresTypeLabel(item: FuturesConfig): string {
  const name = (item.futures_type_name ?? '').trim()
  return name || `分类 ${item.futures_type}`
}

/** 运行状态文案与配色（与「行情快照同步」页同一套口径） */
const statusText = computed(() => {
  const status = run.value?.status
  if (!status) return '暂无运行记录'
  return { RUNNING: '运行中', SUCCESS: '成功', PARTIAL: '部分成功', FAILED: '失败', IDLE: '暂无运行记录' }[status] ?? status
})

const statusColor = computed(() => {
  const status = run.value?.status
  if (status === 'SUCCESS') return 'success'
  if (status === 'RUNNING') return 'info'
  if (status === 'PARTIAL') return 'warning'
  if (status === 'FAILED') return 'error'
  return 'grey'
})

/** 结果行标题：期货按分类，其余按「品种 / 市场」 */
function resultLabel(item: RunResult): string {
  if (item.futures_type) {
    const name = (item.futures_type_name ?? '').trim()
    return `期货 · ${name || `分类 ${item.futures_type}`}`
  }
  return `${symbolLabel(item.instrument_symbol)} · ${marketLabel(item.market_type)}`
}

/** 证券列表配置按品种分组展示（stock / etf 各若干市场） */
const secuConfigs = computed(() =>
  [...configs.value].sort((a, b) =>
    `${a.instrument_symbol}-${a.market_type}`.localeCompare(`${b.instrument_symbol}-${b.market_type}`),
  ),
)

const totals = computed(() => {
  const results = run.value?.results ?? []
  return results.reduce(
    (acc, item) => {
      acc.receive += item.rows_receive || 0
      acc.insert += item.insert || 0
      acc.update += item.update || 0
      acc.failed += item.error ? 1 : 0
      return acc
    },
    { receive: 0, insert: 0, update: 0, failed: 0 },
  )
})

/** 拉取汇总数据（配置 + 最近一次运行） */
async function load(): Promise<void> {
  loading.value = true
  configError.value = ''
  statsError.value = ''
  try {
    // 两个请求互不依赖：统计失败不影响配置与运行结果展示（反之亦然）
    const [runResult, statsResult] = await Promise.allSettled([
      apiGet<RunPayload>(RUN_API),
      apiGet<StatsPayload>(STATS_API),
    ])
    if (statsResult.status === 'fulfilled') {
      stats.value = statsResult.value?.tables ?? []
    } else {
      statsError.value = (statsResult.reason as Error)?.message ?? '未知错误'
      stats.value = []
    }
    if (runResult.status === 'rejected') throw runResult.reason
    const payload = runResult.value
    configs.value = payload?.configs ?? []
    futuresConfigs.value = payload?.futures_configs ?? []
    // 两种响应形态归一：有 run 用它，没有就把扁平字段组装成 RunInfo（run_id 为空表示无记录）
    run.value = payload?.run ?? {
      run_id: payload?.run_id ?? '',
      status: payload?.status ?? 'IDLE',
      started_at: payload?.started_at,
      finished_at: payload?.finished_at,
      elapsed_ms: payload?.elapsed_ms ?? 0,
      results: payload?.results ?? [],
    }
    const err = payload?.error ? String(payload.error) : ''
    if (err) {
      configError.value = err
      showToast(`读取采集配置失败：${err}`, 'error')
    }
  } catch (error) {
    showToast(`读取状态失败：${(error as Error).message}`, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-view-dashboard-outline" class="mr-2" />
      行情采集状态总览
      <v-spacer />
      <v-chip :color="statusColor" variant="flat" size="small" class="mr-2">{{ statusText }}</v-chip>
      <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
    </v-card-title>

    <v-card-text>
      <!-- 最近一次运行摘要 -->
      <div class="d-flex flex-wrap align-center ga-4 mb-3 text-caption">
        <span>运行编号：<code>{{ run?.run_id || '—' }}</code></span>
        <span>开始：{{ run?.started_at || '—' }}</span>
        <span>结束：{{ run?.finished_at || '—' }}</span>
        <span>耗时：{{ run?.elapsed_ms ?? 0 }} ms</span>
        <span>收到：{{ totals.receive }}（新增 {{ totals.insert }} / 覆盖 {{ totals.update }}）</span>
        <span v-if="totals.failed" class="text-error">失败市场/分类：{{ totals.failed }}</span>
      </div>

      <v-alert v-if="configError" type="error" density="compact" variant="tonal" class="mb-3">
        读取采集配置失败：{{ configError }}
      </v-alert>

      <!-- 表规模指标（需求 11285）：指标表与采集参数表的当前行数 -->
      <div class="text-subtitle-2 mb-1">表规模指标（当前行数）</div>
      <div class="d-flex flex-wrap align-center ga-3 mb-4">
        <v-card
          v-for="item in stats"
          :key="item.table"
          variant="tonal"
          color="primary"
          class="px-4 py-2"
          min-width="260"
        >
          <div class="text-caption text-medium-emphasis">{{ item.table }}</div>
          <div class="text-h6">
            {{ item.exists ? formatRows(item.rows) : '—' }}
            <span class="text-caption text-medium-emphasis">行</span>
          </div>
        </v-card>
        <span v-if="statsError" class="text-caption text-error">统计失败：{{ statsError }}</span>
        <span v-else-if="!stats.length" class="text-caption text-medium-emphasis">暂无统计数据</span>
      </div>

      <!-- 证券列表采集配置（stock / etf） -->
      <div class="text-subtitle-2 mb-1">证券列表采集配置</div>
      <v-table density="compact" class="text-caption mb-4">
        <thead>
          <tr>
            <th class="text-no-wrap">品种</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-right">计划数量</th>
            <th class="text-right">市场证券总数</th>
            <th class="text-right">本轮可采集</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in secuConfigs" :key="`${item.instrument_symbol}-${item.market_type}`">
            <td class="text-no-wrap">{{ symbolLabel(item.instrument_symbol) }}</td>
            <td class="text-no-wrap">{{ marketLabel(item.market_type) }}</td>
            <td class="text-right">{{ item.count_plan ?? '全量' }}</td>
            <td class="text-right">{{ item.count_exists || '—' }}</td>
            <td class="text-right">{{ item.total_available || '—' }}</td>
          </tr>
          <tr v-if="!secuConfigs.length">
            <td colspan="5" class="text-center text-medium-emphasis py-4">
              配置表暂无启用行（cdeu.finv_cdeu_secu_realtime_sync_config）
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 期货分类配置 -->
      <div class="text-subtitle-2 mb-1">期货分类采集配置</div>
      <v-table density="compact" class="text-caption mb-4">
        <thead>
          <tr>
            <th class="text-no-wrap">分类</th>
            <th class="text-no-wrap">分类名称</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-right">计划数量</th>
            <th class="text-right">分类合约总数</th>
            <th class="text-right">本轮可采集</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in futuresConfigs" :key="`${item.market_type}-${item.futures_type}`">
            <td class="text-no-wrap">{{ item.futures_type }}</td>
            <td class="text-no-wrap">{{ futuresTypeLabel(item) }}</td>
            <td class="text-no-wrap">{{ item.market_type }}</td>
            <td class="text-right">{{ item.count_plan ?? '全量' }}</td>
            <td class="text-right">{{ item.count_exists || '—' }}</td>
            <td class="text-right">{{ item.total_available || '—' }}</td>
          </tr>
          <tr v-if="!futuresConfigs.length">
            <td colspan="6" class="text-center text-medium-emphasis py-4">
              配置表暂无启用行（cdeu.finv_quote_cdeu_future_sync_config）
            </td>
          </tr>
        </tbody>
      </v-table>

      <!-- 最近一次运行结果 -->
      <div class="text-subtitle-2 mb-1">最近一次运行结果</div>
      <v-table density="compact" class="text-caption">
        <thead>
          <tr>
            <th class="text-no-wrap">品种 / 分类</th>
            <th class="text-right">页数</th>
            <th class="text-right">收到</th>
            <th class="text-right">新增</th>
            <th class="text-right">覆盖</th>
            <th class="text-right">接口总数</th>
            <th class="text-right">本轮可采</th>
            <th class="text-no-wrap">结果</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, index) in run?.results ?? []" :key="`${resultLabel(item)}-${index}`">
            <td class="text-no-wrap">{{ resultLabel(item) }}</td>
            <td class="text-right">{{ item.pages }}</td>
            <td class="text-right">{{ item.rows_receive }}</td>
            <td class="text-right">{{ item.insert }}</td>
            <td class="text-right">{{ item.update }}</td>
            <td class="text-right">{{ item.count_exists }}</td>
            <td class="text-right">{{ item.total_available }}</td>
            <td class="text-no-wrap">
              <span v-if="item.error" class="text-error">{{ item.error }}</span>
              <span v-else class="text-success">成功</span>
            </td>
          </tr>
          <tr v-if="!(run?.results ?? []).length">
            <td colspan="8" class="text-center text-medium-emphasis py-4">
              暂无运行记录：到「行情快照同步」页点「开始同步」后这里会出现逐市场/分类结果（进程重启会清空该记录）
            </td>
          </tr>
        </tbody>
      </v-table>

      <div v-if="run?.error" class="text-caption text-error mt-2">运行错误：{{ run.error }}</div>
    </v-card-text>
  </v-card>

  <!-- 说明卡片：页面最底部、唯一、标题固定为「说明」（Docs/DevSpec/ExplainCardSpec.md） -->
  <v-card class="mt-4">
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-information-outline" class="mr-2" />
      说明
    </v-card-title>
    <v-card-text>
      <div class="finv-note">
        <div class="finv-note-title">数据来源</div>
        <div class="text-body-2 finv-note-body">
          两张配置表：证券列表（<code>cdeu.finv_cdeu_secu_realtime_sync_config</code>，一行 = 一个「市场 + 品种」）
          与期货分类（<code>cdeu.finv_quote_cdeu_future_sync_config</code>，一行 = 一个「市场 + 分类」，共 8 类）；
          运行结果取自<strong>服务进程内存</strong>里最近一次采集（同一份数据供「行情快照同步」页轮询）。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">读法</div>
        <div class="text-body-2 finv-note-body">
          「市场证券总数」/「分类合约总数」是接口 <code>pagination.total</code> 的回写值，
          「本轮可采集」= <code>min(count_plan, 总数)</code>；两者都只在采集成功后刷新，未采过的显示 <code>—</code>。
          下表的「接口总数」与「本轮可采」是<strong>最近一次运行的现场值</strong>，可与上表对照看配置是否被正确执行。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">表规模指标</div>
        <div class="text-body-2 finv-note-body">
          <code>cdeu.finv_quote_secu_indicator</code>（证券指标表，一行一证券）与
          <code>futu.finv_futu_quote_collect</code>（富途采集参数镜像表，一行一证券）的<strong>当前行数</strong>，
          来自 <code>GET /Meta/Finv/Quote/Secu/Collect/Stats</code>；表名由服务端白名单固定，不接受请求拼表名。
          两张表行数应基本相当，差值可用来判断采集覆盖是否有缺口；表不存在时该项显示 <code>—</code>。
        </div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">注意事项</div>
        <div class="text-body-2 finv-note-body">
          运行记录<strong>不落库</strong>：服务重启后本页只显示配置，运行结果回到「暂无运行记录」。
          本页只读，不触发任何采集；要采数据请到「行情快照同步」页选品种后点「开始同步」。
          同一时间只允许一个采集任务在跑，重复启动会被拒绝并提示当前 <code>run_id</code>。
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
/* 说明卡片分节（Docs/DevSpec/ExplainCardSpec.md §4 统一 token） */
.finv-note {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}

.finv-note-title {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgba(var(--v-theme-primary), 1);
  margin-bottom: 2px;
}

.finv-note-body {
  opacity: 0.85;
  line-height: 1.6;
}
</style>
