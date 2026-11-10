<script setup lang="ts">
// 证券参数补全（需求 11161）
//
// 把原来「待采证券录入」页表格里「录入」按钮的能力独立成一页，并按目标表拆成两个 Tab：
//   security → public.finv_security（规范证券参数）
//   collect  → futu.finv_futu_quote_collect（采集参数）
//
// 接口：
//   GET  /Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete/Pending?target=&keyword=&marketLabel=&orderBy=&order=&page=&pageSize=
//   GET  /Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete/Prefill?target=&stockId=
//   POST /Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../api'
import SecuParamCompleteDialog from './SecuParamCompleteDialog.vue'

type Target = 'security' | 'collect'

// 列表行（wire 字段 snake_case，数值列为文本，保住 NUMERIC(24,6) 精度）
interface SecuParamRow {
  stock_id: number
  usc: string
  ftmm_symbol: string
  name: string
  stock_code: string
  market_label: string
  instrument_type: string
  currency: string
  market_value: string
  dt_update: number
}

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete'

// 两个 Tab 的元信息（表名 / 说明 / 标题都在这里，模板里不写散字段）
const TARGETS: Record<Target, { tab: string; table: string; cardIcon: string; cardTitle: string }> = {
  security: {
    tab: '规范证券参数',
    table: 'public.finv_security',
    cardIcon: 'mdi-shield-key-outline',
    cardTitle: '规范证券参数补全（public.finv_security）',
  },
  collect: {
    tab: '采集参数',
    table: 'futu.finv_futu_quote_collect',
    cardIcon: 'mdi-server-network',
    cardTitle: '采集参数补全（futu.finv_futu_quote_collect）',
  },
}

const target = ref<Target>('security')
const rows = ref<SecuParamRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const marketLabel = ref('')
const orderBy = ref('market_value')
const order = ref<'desc' | 'asc'>('desc')
const loading = ref(false)
const error = ref('')

const ORDER_OPTIONS = [
  { title: '总市值', value: 'market_value' },
  { title: '成交额', value: 'trade_trunover' },
  { title: '涨跌幅', value: 'change_ratio' },
  { title: '证券编号', value: 'stock_id' },
  { title: '更新时间', value: 'dt_update' },
]
const MARKET_OPTIONS = [
  { title: '全部市场', value: '' },
  { title: 'HK', value: 'HK' },
  { title: 'SH', value: 'SH' },
  { title: 'SZ', value: 'SZ' },
  { title: 'BJ', value: 'BJ' },
  { title: 'US', value: 'US' },
]

// 富途 instrument_type 字典（口径取自 futu.finv_futu_quote_collect 实际数据）
const INSTRUMENT_TYPE_LABELS: Record<string, string> = {
  '3': '股票',
  '4': 'ETF',
  '6': '指数',
  '7': '板块',
  '9': '期货',
  '11': '加密货币',
  '12': '股票',
}

function instrumentTypeText(value: string | null | undefined): string {
  const key = String(value ?? '').trim()
  if (!key) return '—'
  return INSTRUMENT_TYPE_LABELS[key] ?? key
}

/** 补全将写入的 usc：快照 usc 优先，为空时后端回落到派生键（= ftmm_symbol） */
function targetUSC(row: SecuParamRow): string {
  return String(row.usc ?? '').trim() || String(row.ftmm_symbol ?? '').trim()
}

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const current = computed(() => TARGETS[target.value])

async function load() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      target: target.value,
      page: String(page.value),
      pageSize: String(pageSize.value),
      orderBy: orderBy.value,
      order: order.value,
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (marketLabel.value) params.set('marketLabel', marketLabel.value)
    const data = await apiGet<{
      total: number
      page: number
      pageSize: number
      list: SecuParamRow[]
    }>(`${API}/Pending?${params.toString()}`)
    rows.value = data.list ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = (err as Error).message
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function reset() {
  keyword.value = ''
  marketLabel.value = ''
  orderBy.value = 'market_value'
  order.value = 'desc'
  search()
}

// 切换 Tab：条件保留，页码回到第 1 页重新查
watch(target, () => {
  page.value = 1
  void load()
})

// ---- 补全对话框 ----

const dialogOpen = ref(false)
const dialogRow = ref<SecuParamRow | null>(null)

function openComplete(row: SecuParamRow) {
  dialogRow.value = row
  dialogOpen.value = true
}

// ---- 展示格式化 ----

function trimZero(value: string): string {
  if (!value) return ''
  return value.includes('.') ? value.replace(/0+$/, '').replace(/\.$/, '') : value
}

function compact(value: string): string {
  if (!value) return '—'
  const n = Number(value)
  if (!Number.isFinite(n)) return value
  const abs = Math.abs(n)
  if (abs >= 1e12) return `${trimZero((n / 1e12).toFixed(2))} 万亿`
  if (abs >= 1e8) return `${trimZero((n / 1e8).toFixed(2))} 亿`
  if (abs >= 1e4) return `${trimZero((n / 1e4).toFixed(2))} 万`
  return trimZero(n.toFixed(2))
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

onMounted(load)
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-table-cog" class="mr-2" />
      证券参数补全
      <v-spacer />
      <v-chip size="small" variant="tonal" color="info">{{ current.table }}</v-chip>
    </v-card-title>
    <v-card-subtitle class="pb-2">
      快照里有、目标主数据表里<strong>没有</strong>的证券；点「补全」只写<strong>当前 Tab 那一张表</strong>
      （按主键 <code>usc</code> 插入，已存在则不改动）
    </v-card-subtitle>

    <v-card-text>
      <v-tabs v-model="target" density="compact" class="mb-3">
        <v-tab value="security">
          <v-icon icon="mdi-shield-key-outline" start />
          规范证券参数
        </v-tab>
        <v-tab value="collect">
          <v-icon icon="mdi-server-network" start />
          采集参数
        </v-tab>
      </v-tabs>

      <div class="d-flex flex-wrap align-center ga-2 mb-3">
        <v-text-field
          v-model="keyword"
          label="关键字（代码 / 名称 / usc）"
          placeholder="如: PLD / 安博"
          density="compact"
          hide-details
          clearable
          style="min-width: 220px; max-width: 300px"
          @keyup.enter="search"
        />
        <v-select
          v-model="marketLabel"
          :items="MARKET_OPTIONS"
          item-title="title"
          item-value="value"
          label="市场"
          density="compact"
          hide-details
          style="max-width: 130px"
          @update:model-value="search"
        />
        <v-select
          v-model="orderBy"
          :items="ORDER_OPTIONS"
          item-title="title"
          item-value="value"
          label="排序字段"
          density="compact"
          hide-details
          style="max-width: 150px"
          @update:model-value="search"
        />
        <v-select
          v-model="order"
          :items="[
            { title: '降序', value: 'desc' },
            { title: '升序', value: 'asc' },
          ]"
          item-title="title"
          item-value="value"
          label="方向"
          density="compact"
          hide-details
          style="max-width: 110px"
          @update:model-value="search"
        />
        <v-btn color="primary" prepend-icon="mdi-magnify" @click="search">查询</v-btn>
        <v-btn variant="text" prepend-icon="mdi-refresh" :loading="loading" @click="load">刷新</v-btn>
        <v-btn variant="text" prepend-icon="mdi-restore" @click="reset">重置</v-btn>
      </div>

      <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
        {{ error }}
      </v-alert>

      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-2" />

      <v-table density="compact" class="text-caption finv-complete-table">
        <thead>
          <tr>
            <th class="text-no-wrap">
              <v-tooltip
                text="补全将写入的主键 usc：快照 usc 有值就用它，为空时用派生键 upper(市场标签.代码)"
                location="bottom"
                max-width="360"
              >
                <template #activator="{ props }">
                  <span v-bind="props" class="d-inline-flex align-center">
                    usc（写入键）
                    <v-icon icon="mdi-information-outline" size="14" class="ml-1 text-medium-emphasis" />
                  </span>
                </template>
              </v-tooltip>
            </th>
            <th class="text-no-wrap">证券编号</th>
            <th class="text-no-wrap">代码</th>
            <th class="text-no-wrap">名称</th>
            <th class="text-no-wrap">市场</th>
            <th class="text-no-wrap">品种类型</th>
            <th class="text-no-wrap">币种</th>
            <th class="text-no-wrap text-right">总市值</th>
            <th class="text-no-wrap">快照时间</th>
            <th class="text-no-wrap">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.stock_id">
            <td class="text-no-wrap">
              <code>{{ targetUSC(row) || '—' }}</code>
              <v-icon
                v-if="!String(row.usc ?? '').trim()"
                icon="mdi-asterisk"
                size="12"
                class="ml-1 text-medium-emphasis"
              />
            </td>
            <td class="text-no-wrap">{{ row.stock_id }}</td>
            <td class="text-no-wrap">{{ row.stock_code || '—' }}</td>
            <td class="text-no-wrap">{{ row.name || '—' }}</td>
            <td class="text-no-wrap">{{ row.market_label || '—' }}</td>
            <td class="text-no-wrap">{{ instrumentTypeText(row.instrument_type) }}</td>
            <td class="text-no-wrap">{{ row.currency || '—' }}</td>
            <td class="text-right num">{{ compact(row.market_value) }}</td>
            <td class="text-no-wrap">{{ timeText(row.dt_update) }}</td>
            <td class="text-no-wrap">
              <v-tooltip
                :text="`把该证券写进 ${current.table}（按 usc 插入，已存在不改动）；点击只打开确认对话框，核对后才写库`"
                location="top"
                max-width="360"
              >
                <template #activator="{ props }">
                  <v-btn
                    v-bind="props"
                    size="x-small"
                    variant="tonal"
                    color="primary"
                    prepend-icon="mdi-table-plus"
                    @click="openComplete(row)"
                  >
                    补全
                  </v-btn>
                </template>
              </v-tooltip>
            </td>
          </tr>
          <tr v-if="!rows.length && !loading">
            <td colspan="10" class="text-center text-medium-emphasis py-6">
              没有待补全的证券（{{ current.table }} 里都已有对应记录）
            </td>
          </tr>
        </tbody>
      </v-table>

      <SecuParamCompleteDialog
        v-model="dialogOpen"
        :target="target"
        :row="dialogRow"
        @done="load"
      />

      <div class="d-flex flex-wrap align-center justify-space-between ga-2 mt-3">
        <span class="text-caption text-medium-emphasis">
          共 {{ total }} 条{{ total ? `，第 ${page} / ${pageCount} 页` : '' }}
        </span>
        <div class="d-flex align-center ga-2">
          <span class="text-caption text-medium-emphasis">每页</span>
          <v-select
            v-model="pageSize"
            :items="[20, 50, 100, 200]"
            density="compact"
            hide-details
            style="width: 88px"
            @update:model-value="search"
          />
          <span class="text-caption text-medium-emphasis">条</span>
          <v-pagination
            v-model="page"
            :length="pageCount"
            :total-visible="7"
            density="compact"
            @update:model-value="load"
          />
        </div>
      </div>
    </v-card-text>

    <v-divider />

    <!-- 说明（ExplainCardSpec：一个页面只有一个说明卡片，放在最底部） -->
    <v-card-text>
      <div class="finv-secukey-note">
        <div class="finv-secukey-note-title">说明</div>
        <div class="text-body-2 finv-secukey-note-body">
          <b>① 列表口径</b>：源表 <code>cdeu.finv_cdeu_secu_realtime_stock</code> 的每一行（一行一证券），
          判断目标表里「有没有该证券」——
          <br />
          · <b>规范证券参数</b>：<code>public.finv_security.usc</code>；
          <br />
          · <b>采集参数</b>：<code>futu.finv_futu_quote_collect.usc</code>；
          <br />
          匹配时同时接受<strong>目标键</strong>（快照 <code>usc</code>，为空时用派生键
          <code>upper(市场标签.代码)</code>）与<strong>派生键</strong>两种形态，
          只要有一种已存在就不算缺（避免补出与存量互不相认、还撞唯一约束的重复行）。
          <br /><br />
          <b>② 补全只写一张表</b>：当前 Tab 决定写哪张 —— 两个 Tab 各自独立，补完规范证券参数不会顺带写采集参数，
          反之亦然。写入按主键 <code>usc</code> 做 <code>ON CONFLICT DO NOTHING</code>：
          已存在时不覆盖人工数据，弹窗里也会直接提示「已存在」并禁用提交。
          <br /><br />
          <b>③ 两步走</b>：点「补全」只打开对话框，字段由服务端预填并标注来源
          （<span class="text-success">快照</span> / <span class="text-info">字典</span> /
          <span class="text-warning">参考同市场</span> / <span class="text-medium-emphasis">缺省</span> /
          <span class="text-error">待填</span>），人工核对后再点「写入」，不做一键全自动。
          <br /><br />
          <b>④ 与其它菜单的关系</b>：<code>usc</code> 本身由「待采证券录入」页负责
          （「入队」登记键 /「手动补录USC」回写快照）；本页只负责按已有的 <code>usc</code> 把主数据表补齐。
          <code>remark</code> 一律写 <code>[AUTO-FILL]</code> 前缀 + 时间，可按此前缀溯源本页写入的行。
          <br /><br />
          <b>⑤ 品种类型字典</b>：<code>3/12</code> 股票、<code>4</code> ETF、<code>6</code> 指数、
          <code>7</code> 板块、<code>9</code> 期货、<code>11</code> 加密货币；未登记的值原样显示，空值显示 <code>—</code>。
        </div>
      </div>
    </v-card-text>
  </v-card>
</template>

<style scoped>
.finv-complete-table :deep(th),
.finv-complete-table :deep(td) {
  white-space: nowrap;
  padding-inline: 12px;
}

.finv-complete-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

.finv-complete-table :deep(td.num) {
  min-width: 96px;
}
</style>
