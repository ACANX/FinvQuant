<script setup lang="ts">
// 采集参数（需求 11254）
// 数据表：futu.finv_futu_quote_collect（usc 主键，ftmm_symbol 为生成列）
// 接口：GET  /Meta/Finv/Quote/Secu/Collect/Page   分页查询（左关联 public.finv_security 等）
//      POST /Meta/Finv/Quote/Secu/Collect/Update  单行更新（字段白名单）
//
// 读法：
//   ① 本页查询**已经**把 public.finv_security（+ 交易所 / 市场字典）左关联进来，
//      「名称 / 分类 / 子类 / 计价货币 / 交易所 / 市场」六列来自主数据，不是在采集表里；
//      主数据里没有该证券时这六列显示 —，可在「待采证券录入 / 证券参数补全」补录。
//   ② 可编辑 11 个富途侧参数（富途代码、市场、品种、启用、备注等）；
//      usc（主键）、ftmm_symbol（生成列）、gmt_*（触发器维护）不可改。
import { computed, onMounted, ref } from 'vue'

import { apiGet } from '../../../../../api'
import {
  COLLECT_FLAG_OPTIONS,
  COLLECT_INSTRUMENT_ITEMS,
  COLLECT_MARKET_OPTIONS,
  COLLECT_ORDER_OPTIONS,
  COLLECT_SECURITY_TYPE_OPTIONS,
  collectFlagText,
  collectInstrumentText,
  collectInstrumentTypeText,
  collectMarketTypeText,
} from '../../../../../config/collectParam'
import type { CollectParamRow } from '../../../../../config/collectParam'
import { showToast } from '../../../../../ui/toast'
import CollectParamDialog from './CollectParamDialog.vue'

const PAGE_API = '/Meta/Finv/Quote/Secu/Collect/Page'

const rows = ref<CollectParamRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const keyword = ref('')
const marketLabel = ref('')
const instrument = ref('')
const securityType = ref('')
const flagEnable = ref('')
const onlyMissing = ref(false)
const orderBy = ref('usc')
const order = ref<'asc' | 'desc'>('asc')
const loading = ref(false)
const error = ref('')

const dialogOpen = ref(false)
const editing = ref<CollectParamRow | null>(null)

const MARKET_FILTER_OPTIONS = [
  { title: '全部市场', value: '' },
  ...COLLECT_MARKET_OPTIONS.map((value) => ({ title: value, value })),
]
const INSTRUMENT_FILTER_OPTIONS = [
  { title: '全部品种', value: '' },
  ...COLLECT_INSTRUMENT_ITEMS,
]

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      pageSize: String(pageSize.value),
      orderBy: orderBy.value,
      order: order.value,
    })
    if (keyword.value.trim()) params.set('keyword', keyword.value.trim())
    if (marketLabel.value) params.set('marketLabel', marketLabel.value)
    if (instrument.value) params.set('instrument', instrument.value)
    if (securityType.value) params.set('securityType', securityType.value)
    if (flagEnable.value) params.set('flagEnable', flagEnable.value)
    if (onlyMissing.value) params.set('onlyMissing', '1')
    const data = await apiGet<{ total: number; list: CollectParamRow[] }>(`${PAGE_API}?${params.toString()}`)
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

function search(): void {
  page.value = 1
  void load()
}

function reset(): void {
  keyword.value = ''
  marketLabel.value = ''
  instrument.value = ''
  securityType.value = ''
  flagEnable.value = ''
  onlyMissing.value = false
  orderBy.value = 'usc'
  order.value = 'asc'
  page.value = 1
  pageSize.value = 50
  void load()
}

function openEdit(row: CollectParamRow): void {
  editing.value = row
  dialogOpen.value = true
}

function onSaved(): void {
  showToast('采集参数已保存', 'success')
  void load()
}

onMounted(load)
</script>

<template>
  <div>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-tune-variant" class="mr-2" />
        采集参数
        <v-chip class="ml-3" size="small" variant="tonal">futu.finv_futu_quote_collect</v-chip>
      </v-card-title>

      <v-card-text>
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            v-model="keyword"
            label="关键字（证券编码 / 富途键 / 富途代码 / 名称）"
            placeholder="如: 00700 / HK.00700 / 腾讯"
            density="compact"
            hide-details
            clearable
            style="min-width: 280px; max-width: 360px"
            @keyup.enter="search"
          />
          <v-select
            v-model="marketLabel"
            :items="MARKET_FILTER_OPTIONS"
            item-title="title"
            item-value="value"
            label="行情市场"
            density="compact"
            hide-details
            style="max-width: 140px"
            @update:model-value="search"
          />
          <v-select
            v-model="instrument"
            :items="INSTRUMENT_FILTER_OPTIONS"
            item-title="title"
            item-value="value"
            label="品种"
            density="compact"
            hide-details
            style="max-width: 150px"
            @update:model-value="search"
          />
          <v-select
            v-model="securityType"
            :items="COLLECT_SECURITY_TYPE_OPTIONS"
            item-title="title"
            item-value="value"
            label="证券分类"
            density="compact"
            hide-details
            style="max-width: 150px"
            @update:model-value="search"
          />
          <v-select
            v-model="flagEnable"
            :items="COLLECT_FLAG_OPTIONS"
            item-title="title"
            item-value="value"
            label="采集状态"
            density="compact"
            hide-details
            style="max-width: 130px"
            @update:model-value="search"
          />
          <v-checkbox
            v-model="onlyMissing"
            label="只看主数据缺失"
            density="compact"
            hide-details
            class="finv-collect-check"
            @update:model-value="search"
          />
          <v-select
            v-model="orderBy"
            :items="COLLECT_ORDER_OPTIONS"
            item-title="title"
            item-value="value"
            label="排序字段"
            density="compact"
            hide-details
            style="max-width: 140px"
            @update:model-value="search"
          />
          <v-select
            v-model="order"
            :items="[
              { title: '升序', value: 'asc' },
              { title: '降序', value: 'desc' },
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

        <div class="finv-collect-table-wrap">
          <v-table density="compact" class="text-caption finv-collect-table">
            <thead>
              <tr>
                <th class="text-no-wrap">证券编码</th>
                <th class="text-no-wrap">富途键</th>
                <th class="text-no-wrap">名称</th>
                <th class="text-no-wrap">分类</th>
                <th class="text-no-wrap">子类</th>
                <th class="text-no-wrap">计价货币</th>
                <th class="text-no-wrap">交易所</th>
                <th class="text-no-wrap">市场</th>
                <th class="text-no-wrap">富途代码</th>
                <th class="text-no-wrap">行情市场</th>
                <th class="text-no-wrap">市场类型</th>
                <th class="text-no-wrap text-right">市场代码</th>
                <th class="text-no-wrap">品种</th>
                <th class="text-no-wrap">品种类型</th>
                <th class="text-no-wrap text-right">子类型</th>
                <th class="text-no-wrap">订阅段</th>
                <th class="text-no-wrap">采集</th>
                <th class="text-no-wrap">备注</th>
                <th class="text-no-wrap">更新时间</th>
                <th class="text-no-wrap text-center">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="row.usc">
                <td class="text-no-wrap">{{ row.usc || '—' }}</td>
                <td class="text-no-wrap">{{ row.ftmm_symbol || '—' }}</td>
                <td class="text-no-wrap">
                  <span v-if="!row.security_exists" class="text-medium-emphasis">—</span>
                  <span v-else>{{ row.security_name || '—' }}</span>
                </td>
                <td class="text-no-wrap">{{ row.security_type || '—' }}</td>
                <td class="text-no-wrap">{{ row.security_sub_type || '—' }}</td>
                <td class="text-no-wrap">{{ row.currency_type || '—' }}</td>
                <td class="text-no-wrap">{{ row.exchange_name || '—' }}</td>
                <td class="text-no-wrap">{{ row.security_market_name || '—' }}</td>
                <td class="text-no-wrap">{{ row.futu_symbol || '—' }}</td>
                <td class="text-no-wrap">{{ row.market_label || '—' }}</td>
                <td class="text-no-wrap">{{ collectMarketTypeText(row.market_type) }}</td>
                <td class="text-no-wrap text-right">{{ row.market_code || '—' }}</td>
                <td class="text-no-wrap">{{ collectInstrumentText(row.instrument) }}</td>
                <td class="text-no-wrap">{{ collectInstrumentTypeText(row.instrument_type) }}</td>
                <td class="text-no-wrap text-right">{{ row.sub_instrument_type || '—' }}</td>
                <td class="text-no-wrap">{{ row.req_section || '—' }}</td>
                <td class="text-no-wrap">
                  <v-chip :color="row.flag_enable === '1' ? 'success' : undefined" size="x-small" variant="tonal">
                    {{ collectFlagText(row.flag_enable) }}
                  </v-chip>
                </td>
                <td class="text-no-wrap">{{ row.remark || '—' }}</td>
                <td class="text-no-wrap">{{ row.gmt_update || '—' }}</td>
                <td class="text-no-wrap text-center">
                  <v-btn
                    size="x-small"
                    variant="tonal"
                    prepend-icon="mdi-pencil"
                    title="编辑该证券的采集参数"
                    @click="openEdit(row)"
                  >
                    编辑
                  </v-btn>
                </td>
              </tr>
              <tr v-if="!rows.length && !loading">
                <td colspan="20" class="text-center text-medium-emphasis py-6">暂无数据</td>
              </tr>
            </tbody>
          </v-table>
        </div>

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
    </v-card>

    <CollectParamDialog v-model="dialogOpen" :row="editing" @saved="onSaved" />

    <!-- 说明卡片：页面最底部、唯一（ExplainCardSpec） -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" />
        说明
      </v-card-title>
      <v-card-text>
        <div class="finv-collect-note">
          <div class="finv-collect-note-title">表口径</div>
          <div class="text-body-2 finv-collect-note-body">
            <code>futu.finv_futu_quote_collect</code>：一行 = 一个证券的富途采集参数（主键 <code>usc</code>）。
            本页只改参数，不新增、不删除行——这张表是采集链路的镜像，删行等于停采。
            写入由「FT证券指标数据同步」（指标表 → 采集参数表）与「待采证券录入 / 证券参数补全」负责。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-collect-note">
          <div class="finv-collect-note-title">六列证券元数据的来源</div>
          <div class="text-body-2 finv-collect-note-body">
            「名称 / 分类 / 子类 / 计价货币 / 交易所 / 市场」<strong>不在采集表里</strong>，查询时左关联
            <code>public.finv_security</code>（<code>c.usc = s.usc</code>）取 <code>security_name_cn</code> /
            <code>security_type</code> / <code>type_secu_sub</code> / <code>currency_type</code>，
            再左关联 <code>public.finv_exchange</code> / <code>public.finv_market</code> 取交易所与市场显示名。
            这六列全显示 <code>—</code> 的行表示主数据里还没有该证券，去「待采证券录入」或「证券参数补全」补录。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-collect-note">
          <div class="finv-collect-note-title">字段与编码口径</div>
          <div class="text-body-2 finv-collect-note-body">
            <code>ftmm_symbol</code> 是生成列（<code>market_label || '.' || futu_symbol</code>），自动跟随两者变化，不能手改；
            <code>usc</code> 是主键，也不可改（要换证券请走「入队」）。
            <br />
            <code>market_label</code>：行情市场标识（HK / SH / SZ / BJ / US / FX / CC）。
            <code>market_type</code>：富途市场类型（1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密）。
            <code>market_code</code>：富途市场代码（1 / 2 / 3 / 10 / 30 / 31 / 35 / 120 / 360 …，<strong>不是</strong> <code>finv_market.market_code</code>）。
            <code>instrument</code>：富途证券类型（stock / etfs / index / currency / crypto / futures / sectors）。
            <code>instrument_type</code>：富途品种类型（3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密）。
            <code>sub_instrument_type</code>：富途品种子类型（如 3002 / 4002 / 6001）。
            <code>req_section</code>：富途行情订阅段，通常为空。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-collect-note">
          <div class="finv-collect-note-title">注意事项</div>
          <div class="text-body-2 finv-collect-note-body">
            <code>(market_code, futu_symbol)</code> 有唯一约束：把某行的「市场代码 / 富途代码」改成与别的证券相同会
            <strong>被拒绝</strong>（提示冲突），不会覆盖别人的行。
            <code>flag_enable</code> 是采集开关：<code>1</code> 启用（采集端会抓）、<code>0</code> 禁用。
            更新时间 <code>gmt_update</code> 由数据库触发器维护，展示按北京时间。
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
/* 列较多：单元格不换行，列宽随内容撑开，超出视口整体横向滚动 */
.finv-collect-table-wrap {
  overflow-x: auto;
}

.finv-collect-table :deep(th),
.finv-collect-table :deep(td) {
  white-space: nowrap;
  padding-inline: 12px;
}

.finv-collect-table :deep(th) {
  background: rgb(var(--v-theme-surface));
}

.finv-collect-check {
  max-width: 160px;
}

/* 说明卡片分节（ExplainCardSpec §4 统一 token） */
.finv-collect-note {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}

.finv-collect-note-title {
  font-size: 0.875rem;
  font-weight: 500;
  color: rgb(var(--v-theme-primary));
  margin-bottom: 2px;
}

.finv-collect-note-body {
  opacity: 0.85;
  line-height: 1.6;
}
</style>
