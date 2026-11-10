<script setup lang="ts">
// 待录入证券 →「入队」对话框（需求 11153 / 11154 / 11155 / 11156）
//
// 三步走，人工确认：
//   ① 探测：POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Probe
//      —— 按当前键字段查键表，返回「存在 / 不存在」与现有行现值，**不写库**；
//   ② 核对：写入键表的字段全部可手工修改（主键 ftmm_symbol 由 market + code 派生，只读）；
//      只要 market / code 被改动，探测结果即失效，提交按钮保持禁用，必须先重新探测；
//   ③ 提交：不存在 → 「确认入队」（INSERT）；存在 → 「更新键」（UPDATE 非主键字段）。
//      服务端在写入前还会再验证一次存在性（需求 11155）。
//
// 入队语义：键表是下游采集与 USC 映射的输入，因此不做一键全自动。
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

interface PendingSecuRow {
  stock_id: number
  name: string
  stock_code: string
  market_type: string
  market_label: string
  instrument_type: string
}

const props = defineProps<{
  modelValue: boolean
  row: PendingSecuRow | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  done: []
}>()

const router = useRouter()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// ---- 数据结构（与后端 PendingSecuKeyFields / Probe / WriteResult 一致）----

interface KeyFields {
  ftmm_symbol: string
  market: string
  code: string
  stock_id: number
  instrument: string
  status: string
  flag_enable: number
  priority: number
  usc: string
}

interface ProbeResult {
  stock_id: number
  planned: KeyFields
  existed: boolean
  current: KeyFields | null
  hint: string
}

interface WriteResult {
  ftmm_symbol: string
  action: string
  affected: number
  existed: boolean
  hint: string
}

/** instrument 词表与键表一致；下拉用「中文[值]」（DictFilterSpec） */
const INSTRUMENT_OPTIONS = [
  { title: '股票[stock]', value: 'stock' },
  { title: 'ETF[etf]', value: 'etf' },
  { title: '指数[index]', value: 'index' },
  { title: '板块[sector]', value: 'sector' },
  { title: '期货[futures]', value: 'futures' },
  { title: '加密货币[crypto]', value: 'crypto' },
  { title: '外汇[forex]', value: 'forex' },
  { title: '大盘[market]', value: 'market' },
]

const STATUS_OPTIONS = [
  { title: '待处理[PENDING]', value: 'PENDING' },
  { title: '已入队[QUEUED]', value: 'QUEUED' },
  { title: '已完成[DONE]', value: 'DONE' },
  { title: '失败[FAILED]', value: 'FAILED' },
  { title: '跳过[SKIP]', value: 'SKIP' },
]

const FLAG_OPTIONS = [
  { title: '启用[1]', value: 1 },
  { title: '停用[0]', value: 0 },
]

const STATUS_TEXT: Record<string, string> = {
  PENDING: '待处理',
  QUEUED: '已入队',
  DONE: '已完成',
  FAILED: '失败',
  SKIP: '跳过',
}

// ---- 表单状态（写入键表的字段，除主键外均可改）----

const loading = ref(false)
const probing = ref(false)
const submitting = ref(false)
const error = ref('')

const market = ref('')
const code = ref('')
const stockID = ref('')
const instrument = ref('')
const status = ref('PENDING')
const flagEnable = ref(1)
const priority = ref('10')

/** 最近一次探测结果（null = 还没探测过） */
const probe = ref<ProbeResult | null>(null)
/** 探测时用的主键；与当前派生主键不一致说明键被改过，探测失效 */
const probedKey = ref('')

/** 主键由 market + code 派生（英文字母大写），只读展示；改了 market/code 就自动变 */
const derivedKey = computed(() => {
  const m = market.value.trim().toUpperCase()
  const c = code.value.trim()
  if (!m || !c) return ''
  return `${m}.${c}`.toUpperCase()
})

/** 键字段改动后探测失效（需求 11155：每次提交前都必须基于最新键的探测结果） */
const probeStale = computed(
  () => probe.value !== null && probedKey.value !== '' && probedKey.value !== derivedKey.value,
)

const probedExisted = computed(() => !probeStale.value && (probe.value?.existed ?? false))
const probedAbsent = computed(() => !probeStale.value && probe.value !== null && !probe.value.existed)

/** 按钮可用条件：探测过、键没变、必填齐全、没在提交 */
const fieldsReady = computed(
  () =>
    market.value.trim() !== '' &&
    code.value.trim() !== '' &&
    instrument.value !== '' &&
    stockID.value.trim() !== '' &&
    priority.value.trim() !== '',
)
const canInsert = computed(
  () => probedAbsent.value && fieldsReady.value && !submitting.value && !probing.value,
)
const canUpdate = computed(
  () => probedExisted.value && fieldsReady.value && !submitting.value && !probing.value,
)

const actionHint = computed(() => {
  if (loading.value) return '正在探测…'
  if (!probe.value) return '先点「探测」确认该键是否存在'
  if (probeStale.value) return '键字段已改动，请重新「探测」后再提交'
  if (!fieldsReady.value) return '有必填字段为空，无法提交'
  return probe.value.existed ? '键已存在：只能「更新键」，不能重复入队' : '键不存在：可以「确认入队」'
})

function fillFields(planned: KeyFields) {
  market.value = planned.market ?? ''
  code.value = planned.code ?? ''
  stockID.value = String(planned.stock_id ?? '')
  instrument.value = planned.instrument ?? ''
  status.value = planned.status || 'PENDING'
  flagEnable.value = planned.flag_enable ?? 1
  priority.value = String(planned.priority ?? 10)
}

function buildFields(): KeyFields {
  return {
    ftmm_symbol: derivedKey.value,
    market: market.value.trim(),
    code: code.value.trim(),
    stock_id: Number(stockID.value.trim() || 0),
    instrument: instrument.value,
    status: status.value,
    flag_enable: flagEnable.value,
    priority: Number(priority.value.trim() || 0),
    usc: '',
  }
}

/**
 * 探测键是否存在。
 * 首次探测（initial=true）会把服务端派生的默认值灌进表单；
 * 之后的探测只更新「存在性 / 现值」，不覆盖人工改过的 instrument/status/priority。
 */
async function runProbe(initial = false) {
  const row = props.row
  if (!row?.stock_id) return
  probing.value = true
  error.value = ''
  try {
    const data = await apiPost<ProbeResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Probe',
      { stock_id: row.stock_id, market: market.value.trim(), code: code.value.trim() },
      60000,
    )
    probe.value = data
    probedKey.value = data.planned.ftmm_symbol
    if (initial) {
      fillFields(data.planned)
      probedKey.value = data.planned.ftmm_symbol
    }
  } catch (err) {
    error.value = `探测失败：${(err as Error).message}`
    probe.value = null
    probedKey.value = ''
  } finally {
    probing.value = false
  }
}

/** 打开对话框：清空后自动探测一次，拿到默认字段与存在性 */
async function reset() {
  probe.value = null
  probedKey.value = ''
  error.value = ''
  market.value = ''
  code.value = ''
  stockID.value = ''
  instrument.value = ''
  status.value = 'PENDING'
  flagEnable.value = 1
  priority.value = '10'
  if (!props.row?.stock_id) return
  loading.value = true
  try {
    await runProbe(true)
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.row?.stock_id] as const,
  ([open]) => {
    if (open) void reset()
  },
  { immediate: true },
)

// ---- 提交 ----

async function submitInsert() {
  if (!canInsert.value) return
  submitting.value = true
  error.value = ''
  try {
    const result = await apiPost<WriteResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue',
      { fields: buildFields() },
      60000,
    )
    if (result.existed) {
      showToast(`${result.ftmm_symbol} 已存在，未重复入队；请改用「更新键」`, 'warning')
      await runProbe()
      return
    }
    showToast(`已入队 ${result.ftmm_symbol}`, 'success')
    emit('done')
    close()
  } catch (err) {
    error.value = `入队失败：${(err as Error).message}`
    showToast(error.value, 'error')
  } finally {
    submitting.value = false
  }
}

async function submitUpdate() {
  if (!canUpdate.value) return
  submitting.value = true
  error.value = ''
  try {
    const result = await apiPost<WriteResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enqueue/Update',
      { fields: buildFields() },
      60000,
    )
    if (result.affected === 0) {
      showToast(`${result.ftmm_symbol} 不存在，未更新；请改用「确认入队」`, 'warning')
      await runProbe()
      return
    }
    showToast(result.hint || `已更新键 ${result.ftmm_symbol}`, 'success')
    emit('done')
    close()
  } catch (err) {
    error.value = `更新键失败：${(err as Error).message}`
    showToast(error.value, 'error')
  } finally {
    submitting.value = false
  }
}

/** 去「USC 映射规则」菜单补 USC（只由用户点按钮触发跳转） */
function goUscMapping() {
  close()
  void router.push('/Meta/Finv/Quote/Realtime/UscMapping')
}

function close() {
  visible.value = false
  error.value = ''
}
</script>

<template>
  <v-dialog v-model="visible" max-width="960" scrollable>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-tray-arrow-up" class="mr-2" />
        入队 / 更新键
        <v-chip v-if="row" size="small" variant="tonal" color="primary" class="ml-3">
          {{ row.stock_code }} {{ row.name }}
        </v-chip>
        <v-spacer />
        <v-chip
          v-if="probeStale"
          size="small"
          variant="tonal"
          color="warning"
        >
          键已改动，需重新探测
        </v-chip>
        <v-chip
          v-else-if="probe"
          size="small"
          variant="tonal"
          :color="probe.existed ? 'warning' : 'success'"
        >
          {{ probe.existed ? '键已存在' : '键不存在' }}
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        「探测」确认 <code>futu.finv_futu_secu_key</code> 里是否已有该键：
        <strong>不存在</strong> → 「确认入队」插入；<strong>已存在</strong> → 「更新键」改它的非主键字段。
        键字段（<code>market</code> / <code>code</code>）改动后探测失效，按钮会禁用，必须先重新探测。
      </v-card-subtitle>

      <v-divider />

      <v-card-text>
        <v-progress-linear v-if="loading || probing" indeterminate color="primary" class="mb-3" />

        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
          {{ error }}
        </v-alert>

        <!-- 快照摘要 -->
        <v-sheet v-if="row" class="pa-3 rounded mb-3" color="surface-variant">
          <div class="d-flex flex-wrap ga-4 text-caption">
            <span>证券编号 <strong>{{ row.stock_id }}</strong></span>
            <span>代码 <strong>{{ row.stock_code || '—' }}</strong></span>
            <span>名称 <strong>{{ row.name || '—' }}</strong></span>
            <span>市场 <strong>{{ row.market_label || '—' }}</strong></span>
            <span>instrument_type <strong>{{ row.instrument_type || '—' }}</strong></span>
            <span>market_type <strong>{{ row.market_type || '—' }}</strong></span>
          </div>
        </v-sheet>

        <!-- 写入键表的字段：一个字段一个控件（除主键外均可改） -->
        <div class="text-subtitle-2 mb-1">写入键表的字段（除主键外均可手工修改）</div>
        <v-row dense>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              :model-value="derivedKey"
              label="ftmm_symbol（主键，派生只读）"
              hint="= upper(market + '.' + code)，随下面两个字段自动变化"
              persistent-hint
              density="compact"
              readonly
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="market" label="market 市场标签" hint="如 US / HK / SH，自动转大写" persistent-hint density="compact" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="code" label="code 代码" hint="富途代码口径，字母不重复转换" persistent-hint density="compact" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="stockID" label="stock_id 证券编号" hint="默认取快照证券编号" persistent-hint density="compact" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              v-model="instrument"
              :items="INSTRUMENT_OPTIONS"
              item-title="title"
              item-value="value"
              :return-object="false"
              label="instrument 品种"
              hint="词表与键表一致（板块是 sector，采集表才是 sectors）"
              persistent-hint
              density="compact"
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              v-model="status"
              :items="STATUS_OPTIONS"
              item-title="title"
              item-value="value"
              :return-object="false"
              label="status 状态"
              hint="入队默认 PENDING"
              persistent-hint
              density="compact"
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              v-model="flagEnable"
              :items="FLAG_OPTIONS"
              item-title="title"
              item-value="value"
              :return-object="false"
              label="flag_enable 启用标志"
              hint="入队默认启用（1）"
              persistent-hint
              density="compact"
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="priority" label="priority 优先级" hint="1..1000000，入队默认 10" persistent-hint density="compact" />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              :model-value="probe?.current?.usc || (probeStale ? '（重新探测后显示）' : '（入队后由规则解析）')"
              label="usc（只读，不由本对话框写入）"
              hint="由「USC 映射规则」解析，本页可用「手动补录USC」同步"
              persistent-hint
              density="compact"
              readonly
            />
          </v-col>
        </v-row>

        <div class="d-flex flex-wrap align-center ga-2 my-3">
          <v-btn
            color="primary"
            variant="tonal"
            prepend-icon="mdi-radar"
            :loading="probing"
            :disabled="loading || submitting || !market.trim() || !code.trim()"
            @click="runProbe()"
          >
            探测
          </v-btn>
          <span class="text-caption text-medium-emphasis">{{ actionHint }}</span>
        </div>

        <v-alert
          v-if="probe && !probeStale"
          :type="probe.existed ? 'warning' : 'success'"
          density="compact"
          variant="tonal"
          class="mb-2"
        >
          {{ probe.hint }}
        </v-alert>
        <v-alert v-else-if="probeStale" type="warning" density="compact" variant="tonal" class="mb-2">
          键字段改动后主键变为 <code>{{ derivedKey || '（空）' }}</code>，上次探测（<code>{{ probedKey }}</code>）已失效，请重新「探测」。
        </v-alert>

        <!-- 键表现状：一个字段一行 -->
        <template v-if="probe?.current && !probeStale">
          <div class="text-subtitle-2 mb-1">键表现有记录（只读）</div>
          <v-table density="compact" class="mb-2 finv-enqueue-fields">
            <thead>
              <tr>
                <th class="text-no-wrap">字段</th>
                <th class="text-no-wrap">现值</th>
                <th class="text-no-wrap">说明</th>
              </tr>
            </thead>
            <tbody>
              <tr><td><code>ftmm_symbol</code></td><td>{{ probe.current.ftmm_symbol }}</td><td>主键</td></tr>
              <tr><td><code>market</code></td><td>{{ probe.current.market }}</td><td>市场标签</td></tr>
              <tr><td><code>code</code></td><td>{{ probe.current.code }}</td><td>富途代码</td></tr>
              <tr><td><code>stock_id</code></td><td>{{ probe.current.stock_id || '—' }}</td><td>证券编号（0/NULL 视为未解析）</td></tr>
              <tr><td><code>instrument</code></td><td>{{ probe.current.instrument }}</td><td>品种</td></tr>
              <tr><td><code>status</code></td><td>{{ STATUS_TEXT[probe.current.status] ?? probe.current.status }}</td><td>{{ probe.current.status }}</td></tr>
              <tr><td><code>flag_enable</code></td><td>{{ probe.current.flag_enable }}</td><td>{{ probe.current.flag_enable === 1 ? '启用' : '停用' }}</td></tr>
              <tr><td><code>priority</code></td><td>{{ probe.current.priority }}</td><td>优先级</td></tr>
              <tr><td><code>usc</code></td><td>{{ probe.current.usc || '—' }}</td><td>解析结果</td></tr>
            </tbody>
          </v-table>
        </template>
      </v-card-text>

      <v-divider />

      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">{{ actionHint }}</span>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="close">取消</v-btn>
        <v-btn
          v-if="probe && !probeStale && probe.existed"
          color="primary"
          variant="tonal"
          prepend-icon="mdi-transit-connection-variant"
          class="mr-2"
          @click="goUscMapping"
        >
          去「USC 映射规则」
        </v-btn>
        <v-btn
          v-if="probe && !probeStale && probe.existed"
          color="primary"
          prepend-icon="mdi-content-save-edit-outline"
          :disabled="!canUpdate"
          :loading="submitting"
          @click="submitUpdate"
        >
          更新键
        </v-btn>
        <v-btn
          v-else
          color="primary"
          prepend-icon="mdi-tray-arrow-up"
          :disabled="!canInsert"
          :loading="submitting"
          @click="submitInsert"
        >
          确认入队
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
/* 字段现状表：字段名列不换行，说明列允许折行 */
.finv-enqueue-fields :deep(td),
.finv-enqueue-fields :deep(th) {
  padding-inline: 12px;
}

.finv-enqueue-fields :deep(td:nth-child(1)) {
  white-space: nowrap;
}
</style>
