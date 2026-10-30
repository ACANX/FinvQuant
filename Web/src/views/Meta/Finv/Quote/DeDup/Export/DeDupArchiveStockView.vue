<script setup lang="ts">
// 历史月份存量数据清点（需求 10835，页面路由 /Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat）：按「业务月份 + 证券」统计主表里该月的存量数据，
// 让运维在跑导出流程之前就能确认「这次会导出哪些证券、各多少行、文件大致多大」。
//
// 数据源：
//   · cdeu.finv_quote_secu_kline_min      主表存量（真正会被导出的数据）
//   · cdeu.finv_quote_cdeu_quote_maturity_mon  成熟度（主表被清理后仍能看到该月规模）
//   · cdeu.finv_quote_cdeu_export_job / _export_file  导出状态与已生成文件名
// 接口：GET /Meta/Finv/Quote/DeDup/Export/Stock/Page
import { computed, onMounted, ref, watch } from 'vue'

import { apiGet } from '../../../../../../api'
import { bizMonthOptions, fmtBytes, fmtMonth, fmtNum } from '../../../../../../utils'

interface StockRow {
  usc: string
  biz_month: number
  name: string
  region: string
  market: string
  stock_rows: number
  stock_days: number
  date_from: number
  date_to: number
  expected_rows: number
  actual_rows: number
  maturity_status: string
  unmature_days: number
  export_status: string
  job_id?: number
  file_name: string
  est_bytes: number
}

interface StockPageData {
  biz_month: number
  total: number
  items: StockRow[]
  total_stock_rows: number
  total_secu: number
  total_est_bytes: number
  cleaned_secu: number
}

const MATURITY_NAME: Record<string, string> = {
  IMMATURE: '未成熟',
  PARTIAL: '部分到达',
  COMPLETE: '已完整',
  FROZEN: '已冻结',
}
const MATURITY_COLOR: Record<string, string> = {
  IMMATURE: 'grey',
  PARTIAL: 'warning',
  COMPLETE: 'success',
  FROZEN: 'deep-purple',
}
const EXPORT_COLOR: Record<string, string> = {
  WAITING: 'grey',
  LOADING: 'info',
  LOADED: 'info',
  EXPORTING: 'primary',
  EXPORTED: 'success',
  FAILED: 'error',
  PAUSED: 'warning',
  FROZEN: 'deep-purple',
}

const bizMonthItems = computed(() => bizMonthOptions())
const bizMonth = ref(String((new Date().getFullYear()) * 100 + new Date().getMonth() + 1))
const uscKeyword = ref('')
const page = ref(1)
const pageSize = ref(500)
const loading = ref(false)
const error = ref('')
const data = ref<StockPageData | null>(null)

const pageCount = computed(() => {
  if (!data.value) return 1
  return Math.max(1, Math.ceil(data.value.total / pageSize.value))
})

// 提示语：该月主表已无数据但成熟度有记录，说明是"已导出并被清理"，不是数据缺失。
const cleanedHint = computed(() => {
  if (!data.value) return ''
  if (data.value.total_stock_rows > 0 || data.value.cleaned_secu === 0) return ''
  return `该业务月主表存量已为 0，${data.value.cleaned_secu} 个证券的记录仍在成熟度表中——通常表示该月已导出完成并被清理（PURGE_EXPORTED_MONTH），不是数据缺失。`
})

async function load() {
  loading.value = true
  error.value = ''
  const params = new URLSearchParams({
    bizMonth: bizMonth.value,
    page: String(page.value),
    pageSize: String(pageSize.value),
  })
  if (uscKeyword.value.trim()) params.set('usc', uscKeyword.value.trim())
  try {
    data.value = await apiGet<StockPageData>(`/Meta/Finv/Quote/DeDup/Export/Stock/Page?${params.toString()}`)
  } catch (e) {
    data.value = null
    error.value = e instanceof Error ? e.message : '查询历史月份存量数据清点失败'
  } finally {
    loading.value = false
  }
}

watch([bizMonth, pageSize], () => {
  page.value = 1
  void load()
})
watch(page, () => void load())
onMounted(() => void load())
</script>

<template>
  <v-card>
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-database-eye-outline" class="mr-2" />
      历史月份存量数据清点
      <v-spacer />
      <v-chip size="small" variant="tonal" color="info">跑流程前核对导出范围与数量</v-chip>
    </v-card-title>

    <v-card-text>
      <v-alert v-if="error" type="error" density="compact" class="mb-3" closable @click:close="error = ''">
        {{ error }}
      </v-alert>

      <v-row dense align="center" class="mb-2">
        <v-col cols="12" sm="4" md="2">
          <v-select v-model="bizMonth" :items="bizMonthItems" label="业务月份" density="compact" hide-details />
        </v-col>
        <v-col cols="12" sm="4" md="3">
          <v-text-field
            v-model="uscKeyword"
            label="证券代码（模糊）"
            density="compact"
            hide-details
            clearable
            @keyup.enter="page = 1; load()"
          />
        </v-col>
        <v-col cols="auto">
          <v-btn color="primary" prepend-icon="mdi-magnify" :loading="loading" @click="page = 1; load()">查询</v-btn>
        </v-col>
        <v-col cols="auto">
          <v-select
            v-model="pageSize"
            :items="[100, 200, 500, 1000]"
            label="每页"
            density="compact"
            hide-details
            style="max-width: 110px"
          />
        </v-col>
      </v-row>

      <v-row dense class="mb-2">
        <v-col cols="6" sm="3">
          <v-card variant="tonal" color="primary">
            <v-card-text class="py-2">
              <div class="text-caption">证券数</div>
              <div class="text-h6">{{ fmtNum(data?.total_secu ?? 0) }}</div>
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="6" sm="3">
          <v-card variant="tonal" color="success">
            <v-card-text class="py-2">
              <div class="text-caption">主表存量行数</div>
              <div class="text-h6">{{ fmtNum(data?.total_stock_rows ?? 0) }}</div>
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="6" sm="3">
          <v-card variant="tonal" color="deep-purple">
            <v-card-text class="py-2">
              <div class="text-caption">预计导出总大小</div>
              <div class="text-h6">{{ fmtBytes(data?.total_est_bytes ?? 0) }}</div>
            </v-card-text>
          </v-card>
        </v-col>
        <v-col cols="6" sm="3">
          <v-card variant="tonal" color="grey">
            <v-card-text class="py-2">
              <div class="text-caption">主表已清理（已导出）</div>
              <div class="text-h6">{{ fmtNum(data?.cleaned_secu ?? 0) }}</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-alert v-if="cleanedHint" type="info" density="compact" class="mb-3">{{ cleanedHint }}</v-alert>

      <!-- 不设 height / fixed-header：表格按行数自然展开，只由页面外层滚动，
           避免「页面滚动条 + 表格滚动条」双层嵌套。 -->
      <v-table density="compact">
        <thead>
          <tr>
            <th>证券</th>
            <th>名称</th>
            <th>区域/市场</th>
            <th class="text-right">主表存量行数</th>
            <th class="text-right">交易日</th>
            <th>日期范围</th>
            <th class="text-right">应有/实际行数</th>
            <th>成熟度</th>
            <th>导出状态</th>
            <th>文件名（已生成或预计）</th>
            <th class="text-right">预计大小</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in data?.items ?? []" :key="row.usc">
            <td class="font-weight-medium">{{ row.usc }}</td>
            <td>{{ row.name || '-' }}</td>
            <td>{{ row.region || '-' }} / {{ row.market || '-' }}</td>
            <td class="text-right">
              <span v-if="row.stock_rows > 0">{{ fmtNum(row.stock_rows) }}</span>
              <span v-else class="text-medium-emphasis">0（已清理）</span>
            </td>
            <td class="text-right">{{ row.stock_days || '-' }}</td>
            <td>
              <span v-if="row.date_from">{{ row.date_from }} ~ {{ row.date_to }}</span>
              <span v-else class="text-medium-emphasis">-</span>
            </td>
            <td class="text-right">{{ fmtNum(row.expected_rows) }} / {{ fmtNum(row.actual_rows) }}</td>
            <td>
              <v-chip
                v-if="row.maturity_status"
                size="x-small"
                :color="MATURITY_COLOR[row.maturity_status] ?? 'grey'"
                variant="tonal"
              >
                {{ MATURITY_NAME[row.maturity_status] ?? row.maturity_status }}
                <span v-if="row.unmature_days > 0">（{{ row.unmature_days }} 天未达标）</span>
              </v-chip>
              <span v-else class="text-medium-emphasis">无记录</span>
            </td>
            <td>
              <v-chip
                v-if="row.export_status"
                size="x-small"
                :color="EXPORT_COLOR[row.export_status] ?? 'grey'"
                variant="tonal"
              >
                {{ row.export_status }}<span v-if="row.job_id"> #{{ row.job_id }}</span>
              </v-chip>
              <span v-else class="text-medium-emphasis">未导出</span>
            </td>
            <td class="text-caption" style="word-break: break-all">{{ row.file_name || '-' }}</td>
            <td class="text-right">{{ fmtBytes(row.est_bytes) }}</td>
          </tr>
          <tr v-if="!loading && (data?.items?.length ?? 0) === 0">
            <td colspan="11" class="text-center text-medium-emphasis py-6">
              {{ fmtMonth(Number(bizMonth)) }} 在主表与成熟度表中都没有证券记录
            </td>
          </tr>
        </tbody>
      </v-table>

      <v-row dense align="center" class="mt-2">
        <v-col cols="auto" class="text-caption text-medium-emphasis">
          共 {{ fmtNum(data?.total_secu ?? 0) }} 个证券；预计大小按主表样本行均值估算，仅作范围参考
        </v-col>
        <v-spacer />
        <v-col cols="auto">
          <v-pagination v-model="page" :length="pageCount" density="compact" total-visible="7" />
        </v-col>
      </v-row>
    </v-card-text>
  </v-card>
</template>
