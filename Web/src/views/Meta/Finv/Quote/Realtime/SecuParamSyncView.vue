<script setup lang="ts">
// 证券参数同步（需求 11042/11043/11044）
// 接口：
//   GET  /Meta/Finv/Quote/Cdeu/SecuKey/Status   只读快照（20013 是否配置 + 源表名 + 目标表行数）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Config   保存源表名（写系统配置 20014）
//   POST /Meta/Finv/Quote/Cdeu/SecuKey/Sync     手动触发同步（同步执行，返回逐批日志）
//
// 数据流：Supabase PostgREST（源表，默认 public.finv_quote_secu_key）
//        → 本地 futu.finv_futu_secu_key（主键 ftmm_symbol，分批增量 upsert）
// 连接配置（PROJECT_ID|APIKEY）在系统配置 20013 里维护，本页只展示是否已配置。
import { computed, onMounted, ref } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

// 同步状态快照（GET Status 的 data）
interface SecuKeyStatus {
  configured: boolean
  project_id: string
  api_key_masked: string
  config_error?: string
  source_table: string
  source_table_raw?: string
  target_table: string
  target_rows: number
  target_last_update?: string
  batch_size_default: number
}

// 单批日志
interface BatchLog {
  batch: number
  offset: number
  fetched: number
  insert: number
  update: number
  skipped: number
  elapsed_ms: number
}

// 一次同步的结果（POST Sync 的 data）
interface SyncResult {
  source_table: string
  rows_total: number
  batches: number
  rows_receive: number
  insert: number
  update: number
  unchanged: number
  skipped: number
  status_reset: number
  elapsed_ms: number
  logs: BatchLog[]
}

const STATUS_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Status'
const CONFIG_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Config'
const SYNC_API = '/Meta/Finv/Quote/Cdeu/SecuKey/Sync'

// 批量大小候选（默认 1000；7 万行量级约需 70+ 批）
const BATCH_OPTIONS = [500, 1000, 2000, 5000]
// 源表名默认值（与系统配置 20014 的种子一致）
const DEFAULT_SOURCE_TABLE = 'finv_quote_secu_key'
// 同步请求超时：7 万行量级约 1~3 分钟，给足 15 分钟余量
const SYNC_TIMEOUT_MS = 15 * 60 * 1000

const status = ref<SecuKeyStatus | null>(null)
const statusLoading = ref(false)
const sourceTable = ref(DEFAULT_SOURCE_TABLE)
const batchSize = ref(1000)
const saving = ref(false)
const syncing = ref(false)
const result = ref<SyncResult | null>(null)
const errorMessage = ref('')

const configured = computed(() => status.value?.configured === true)
const sourceTableChanged = computed(() => {
  const current = (status.value?.source_table || DEFAULT_SOURCE_TABLE).trim()
  return sourceTable.value.trim() !== current
})

// 数字千分位（日志/统计里便于读数）
function num(value: number | undefined | null): string {
  if (value === undefined || value === null) return '—'
  return value.toLocaleString('en-US')
}

// RFC3339 → 本地时间（空的用 —）
function timeText(value?: string): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}:${pad(date.getSeconds())}`
}

// 毫秒 → 1.2s 形式
function elapsedText(ms: number | undefined): string {
  if (!ms || ms < 0) return '—'
  if (ms < 1000) return `${ms} ms`
  return `${(ms / 1000).toFixed(1)} s`
}

// 读取状态快照
async function loadStatus() {
  statusLoading.value = true
  try {
    const data = await apiGet<SecuKeyStatus>(STATUS_API)
    status.value = data
    sourceTable.value = data.source_table || DEFAULT_SOURCE_TABLE
    if (data.batch_size_default > 0) {
      const allowed = BATCH_OPTIONS.includes(data.batch_size_default)
      batchSize.value = allowed ? data.batch_size_default : 1000
    }
    errorMessage.value = ''
  } catch (err) {
    errorMessage.value = err instanceof Error ? err.message : String(err)
    showToast(errorMessage.value, 'error')
  } finally {
    statusLoading.value = false
  }
}

// 保存源表名（写系统配置 20014）
async function saveSourceTable() {
  saving.value = true
  try {
    const data = await apiPost<{ source_table: string }>(CONFIG_API, {
      source_table: sourceTable.value.trim(),
    })
    sourceTable.value = data.source_table
    showToast(`源表名已保存：${data.source_table}`, 'success')
    await loadStatus()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    errorMessage.value = message
    showToast(message, 'error')
  } finally {
    saving.value = false
  }
}

// 手动触发同步（同步执行，按钮期间禁用）
async function runSync() {
  if (!configured.value) {
    showToast('系统配置 20013（Supabase 连接配置）未配置，请先填写 {PROJECT_ID}|{APIKEY}', 'warning')
    return
  }
  syncing.value = true
  errorMessage.value = ''
  result.value = null
  try {
    const data = await apiPost<SyncResult>(
      SYNC_API,
      { source_table: sourceTable.value.trim(), batch_size: batchSize.value },
      SYNC_TIMEOUT_MS,
    )
    result.value = data
    showToast(
      `同步完成：新增 ${num(data.insert)}、更新 ${num(data.update)}、未变 ${num(data.unchanged)}`,
      'success',
    )
    await loadStatus()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    errorMessage.value = message
    showToast(message, 'error')
  } finally {
    syncing.value = false
  }
}

onMounted(loadStatus)
</script>

<template>
  <div>
    <div class="d-flex align-center mb-2">
      <v-icon icon="mdi-database-sync-outline" class="mr-2" />
      <span class="text-h6">证券参数同步</span>
      <v-spacer />
      <v-btn
        variant="text"
        size="small"
        prepend-icon="mdi-refresh"
        :loading="statusLoading"
        @click="loadStatus"
      >
        刷新状态
      </v-btn>
    </div>
    <div class="text-caption text-medium-emphasis mb-4">
      Supabase 证券键表（默认 <code>public.finv_quote_secu_key</code>）→ 本地
      <code>futu.finv_futu_secu_key</code>：按 <code>ftmm_symbol</code> 分批增量写入（缺则插入、变了才更新）。
    </div>

    <!-- 同步状态 -->
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-cloud-check-outline" class="mr-2" />
        连接与目标表
      </v-card-title>
      <v-card-text>
        <v-row dense>
          <v-col cols="12" md="6">
            <div class="text-caption text-medium-emphasis">Supabase 连接（系统配置 20013）</div>
            <div class="d-flex align-center flex-wrap ga-2 mt-1">
              <v-chip v-if="configured" size="small" color="success" variant="tonal" prepend-icon="mdi-check-circle-outline">
                已配置
              </v-chip>
              <v-chip v-else size="small" color="warning" variant="tonal" prepend-icon="mdi-alert-outline">
                未配置
              </v-chip>
              <code v-if="configured" class="text-body-2">{{ status?.project_id }}</code>
              <span v-if="configured" class="text-caption text-medium-emphasis">
                APIKEY {{ status?.api_key_masked }}（已脱敏）
              </span>
            </div>
            <div v-if="status?.config_error" class="text-caption text-warning mt-1">
              {{ status.config_error }}
            </div>
          </v-col>
          <v-col cols="12" md="6">
            <div class="text-caption text-medium-emphasis">本地目标表</div>
            <div class="d-flex align-center flex-wrap ga-2 mt-1">
              <code class="text-body-2">{{ status?.target_table }}</code>
              <v-chip size="small" variant="tonal" color="primary">共 {{ num(status?.target_rows) }} 行</v-chip>
            </div>
            <div class="text-caption text-medium-emphasis mt-1">
              最近变更：{{ timeText(status?.target_last_update) }}
            </div>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>

    <!-- 同步参数 -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-tune-variant" class="mr-2" />
        同步参数
      </v-card-title>
      <v-card-text>
        <v-row dense align="start">
          <v-col cols="12" md="5">
            <v-text-field
              v-model="sourceTable"
              label="Supabase 源表名"
              density="compact"
              variant="outlined"
              hide-details
              :placeholder="DEFAULT_SOURCE_TABLE"
              hint="默认 finv_quote_secu_key；可写 schema.table 两段，只允许字母/数字/下划线"
              persistent-hint
            />
          </v-col>
          <v-col cols="12" md="3">
            <v-select
              v-model="batchSize"
              :items="BATCH_OPTIONS"
              label="每批行数"
              density="compact"
              variant="outlined"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="4" class="d-flex align-center ga-2">
            <v-btn
              variant="tonal"
              color="primary"
              prepend-icon="mdi-content-save-outline"
              :loading="saving"
              :disabled="syncing || !sourceTableChanged"
              @click="saveSourceTable"
            >
              保存源表名
            </v-btn>
            <v-btn
              color="primary"
              prepend-icon="mdi-cloud-download-outline"
              :loading="syncing"
              :disabled="!configured"
              @click="runSync"
            >
              立即同步
            </v-btn>
          </v-col>
        </v-row>
        <div v-if="syncing" class="text-caption text-medium-emphasis mt-3">
          正在同步：按批拉取并写入，7 万行量级约需 1~3 分钟，请勿关闭页面。
        </div>
      </v-card-text>
    </v-card>

    <!-- 错误提示 -->
    <v-alert v-if="errorMessage" type="error" variant="tonal" class="mt-4" density="compact">
      {{ errorMessage }}
    </v-alert>

    <!-- 同步结果 -->
    <v-card v-if="result" class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-chart-timeline-variant-shimmer" class="mr-2" />
        同步结果
        <v-spacer />
        <span class="text-caption text-medium-emphasis">
          源表 <code>{{ result.source_table }}</code> · 共 {{ num(result.rows_total) }} 行 · {{ result.batches }} 批 ·
          {{ elapsedText(result.elapsed_ms) }}
        </span>
      </v-card-title>
      <v-card-text>
        <div class="d-flex flex-wrap ga-2 mb-3">
          <v-chip size="small" variant="tonal" color="info">拉取 {{ num(result.rows_receive) }}</v-chip>
          <v-chip size="small" variant="tonal" color="success">新增 {{ num(result.insert) }}</v-chip>
          <v-chip size="small" variant="tonal" color="primary">更新 {{ num(result.update) }}</v-chip>
          <v-chip size="small" variant="tonal">未变 {{ num(result.unchanged) }}</v-chip>
          <v-chip v-if="result.skipped" size="small" variant="tonal" color="warning">跳过 {{ num(result.skipped) }}</v-chip>
          <v-chip v-if="result.status_reset" size="small" variant="tonal" color="warning">
            status 归一 {{ num(result.status_reset) }}
          </v-chip>
        </div>
        <v-table density="compact" class="finv-secukey-log">
          <thead>
            <tr>
              <th class="text-right">批次</th>
              <th class="text-right">offset</th>
              <th class="text-right">拉取</th>
              <th class="text-right">新增</th>
              <th class="text-right">更新</th>
              <th class="text-right">跳过</th>
              <th class="text-right">耗时</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in result.logs" :key="log.batch">
              <td class="text-right">{{ log.batch }}</td>
              <td class="text-right">{{ num(log.offset) }}</td>
              <td class="text-right">{{ num(log.fetched) }}</td>
              <td class="text-right">{{ num(log.insert) }}</td>
              <td class="text-right">{{ num(log.update) }}</td>
              <td class="text-right">{{ num(log.skipped) }}</td>
              <td class="text-right">{{ elapsedText(log.elapsed_ms) }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-card-text>
    </v-card>

    <!-- 说明卡片（ExplainCardSpec：静态说明统一放页面最底部唯一「说明」卡片） -->
    <v-card class="mt-4">
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-information-outline" class="mr-2" />
        说明
      </v-card-title>
      <v-card-text>
        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">表口径</div>
          <div class="text-body-2 finv-secukey-note-body">
            源：Supabase <code>public.finv_quote_secu_key</code>（表名可改，见「同步参数」）；
            目标：<code>futu.finv_futu_secu_key</code>（主键 <code>ftmm_symbol</code>）。
            同步的列：<code>instrument</code>、<code>market</code>、<code>code</code>、<code>flag_enable</code>（源
            <code>enable</code>）、<code>priority</code>、<code>status</code>、<code>url</code>；
            本地专有列 <code>stock_id</code> / <code>usc</code> 不参与同步，也不会被清空。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">增量口径</div>
          <div class="text-body-2 finv-secukey-note-body">
            按 <code>ftmm_symbol</code> 升序分批拉取：主键不存在则插入；已存在且上述字段与源不同才更新；
            完全一致的行不改写（不会让 <code>gmt_update</code> 前进）。因此「新增 + 更新」才是本次真正写入的行数，
            「未变」是比对后跳过的行数。同步只读源表，不删除本地行，也不改动历史数据。
          </div>
        </div>

        <v-divider class="my-3" />

        <div class="finv-secukey-note">
          <div class="finv-secukey-note-title">配置与安全</div>
          <div class="text-body-2 finv-secukey-note-body">
            连接配置在系统配置 <strong>20013</strong>，格式 <code>{PROJECT_ID}|{APIKEY}</code>（本页只展示项目 ID 与脱敏后的
            key 前缀，不回显完整 key）；源表名存系统配置 <strong>20014</strong>（默认
            <code>finv_quote_secu_key</code>），在本页改完需点「保存源表名」才生效。
            源 <code>status</code> 为空或不在 <code>PENDING/QUEUED/DONE/FAILED/SKIP</code> 之内时统一落
            <code>PENDING</code>（计入「status 归一」）。
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<style scoped>
.finv-secukey-log,
.finv-secukey-note-body code {
  font-variant-numeric: tabular-nums;
}

/* 说明卡片：每段带标题 + 左侧色条（与缺省汇率页保持一致的视觉语言） */
.finv-secukey-note {
  padding-left: 10px;
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
}

.finv-secukey-note-title {
  font-size: 0.85rem;
  font-weight: 600;
  margin-bottom: 2px;
}

.finv-secukey-note-body {
  opacity: 0.85;
}
</style>
