<script setup lang="ts">
// 证券参数补全 →「补全」确认对话框（需求 11161）
//
// 数据来源：GET /Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete/Prefill?target=&stockId=
//   —— 服务端把「能关联到的」全部算好并标注来源，本组件只展示目标表那一组字段。
// 提交：POST /Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete
//   —— 只写 target 指定的那一张表（public.finv_security 或 futu.finv_futu_quote_collect），
//      按主键 ON CONFLICT (usc) DO NOTHING（幂等、不覆盖人工数据）。
//
// 两步走：点列表里的「补全」只是打开本对话框，人工核对后才写库（需求 11154 的既定做法）。
import { computed, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

type Target = 'security' | 'collect'

interface SecuParamRow {
  stock_id: number
  usc: string
  ftmm_symbol: string
  name: string
  stock_code: string
  market_label: string
  instrument_type: string
  currency: string
}

const props = defineProps<{
  modelValue: boolean
  target: Target
  row: SecuParamRow | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  done: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const API = '/Meta/Finv/Quote/Cdeu/Realtime/Secu/Param/Complete'

const TARGET_META: Record<Target, { title: string; table: string; icon: string }> = {
  security: {
    title: '补全规范证券参数',
    table: 'public.finv_security',
    icon: 'mdi-shield-key-outline',
  },
  collect: {
    title: '补全采集参数',
    table: 'futu.finv_futu_quote_collect',
    icon: 'mdi-server-network',
  },
}

// ---- 预填数据结构（与后端 SecuParamPrefill 一致）----

interface PrefillField {
  key: string
  label: string
  value: string
  source: string
  required: boolean
  note: string
}

interface SecuParamPrefill {
  target: Target
  table: string
  stock_id: number
  usc: string
  name: string
  exists: boolean
  fields: PrefillField[]
  warnings: string[]
}

/** 来源标记 → 中文（DictColumnSpec：表格列只显示中文） */
const SOURCE_TEXT: Record<string, string> = {
  snapshot: '快照',
  dictionary: '字典',
  reference: '参考',
  default: '缺省',
  manual: '待填',
}
const SOURCE_COLOR: Record<string, string> = {
  snapshot: 'success',
  dictionary: 'info',
  reference: 'warning',
  default: 'secondary',
  manual: 'error',
}

function sourceText(source: string): string {
  return SOURCE_TEXT[source] ?? source
}
function sourceColor(source: string): string {
  return SOURCE_COLOR[source] ?? 'secondary'
}

const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const prefill = ref<SecuParamPrefill | null>(null)
const form = ref<Record<string, string>>({})
const remark = ref('')

/** 不进输入框的字段：usc 是主键（由服务端派生并显示在顶部），stock_id 同理 */
const HIDDEN_FIELDS = new Set(['usc', 'stock_id'])

const visibleFields = computed(() =>
  (prefill.value?.fields ?? []).filter((field) => !HIDDEN_FIELDS.has(field.key)),
)

const meta = computed(() => TARGET_META[props.target])

async function loadPrefill() {
  prefill.value = null
  error.value = ''
  remark.value = ''
  form.value = {}
  const row = props.row
  if (!row?.stock_id) return

  loading.value = true
  try {
    const query = new URLSearchParams({
      target: props.target,
      stockId: String(row.stock_id),
    })
    const data = await apiGet<SecuParamPrefill>(`${API}/Prefill?${query.toString()}`)
    prefill.value = data
    const next: Record<string, string> = {}
    for (const field of data.fields ?? []) next[field.key] = field.value ?? ''
    form.value = next
  } catch (err) {
    error.value = `预填失败：${(err as Error).message}`
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.row?.stock_id, props.target] as const,
  ([open]) => {
    if (open) void loadPrefill()
  },
  { immediate: true },
)

// ---- 校验 ----

/**
 * 数值型字段：这类字段「有值但等于 0」同样算未填 ——
 * 后端要求 exchange_code / market_type / market_code / instrument_type 必须 > 0，
 * 只判空串会让「0」蒙混过关，点「写入」才被后端拒（需求 11164 的踩坑点）。
 */
const NUMERIC_KEYS = new Set([
  'exchange_code',
  'market_type',
  'market_code',
  'instrument_type',
  'sub_instrument_type',
])

function isBlankField(field: { key: string }, value: string | undefined): boolean {
  const text = String(value ?? '').trim()
  if (text === '') return true
  if (!NUMERIC_KEYS.has(field.key)) return false
  const parsed = Number(text)
  return !Number.isFinite(parsed) || parsed <= 0
}

const missing = computed<string[]>(() =>
  visibleFields.value
    .filter((field) => field.required && isBlankField(field, form.value[field.key]))
    .map((field) => field.key),
)

/** 单字段的校验提示（模板里直接绑到 v-text-field 上，不必等提交才红） */
function fieldError(field: { key: string, required: boolean }): string {
  if (!field.required) return ''
  const value = form.value[field.key]
  if (!isBlankField(field, value)) return ''
  if (NUMERIC_KEYS.has(field.key) && String(value ?? '').trim() !== '') {
    return `${field.key} 必须大于 0`
  }
  return `${field.key} 必填`
}

/** 待人工核对的字段数（来源为缺省 / 待填 / 参考） */
const pendingCheck = computed(
  () => visibleFields.value.filter((f) => ['default', 'manual', 'reference'].includes(f.source)).length,
)

const canSubmit = computed(
  () =>
    !!prefill.value &&
    !prefill.value.exists &&
    !loading.value &&
    !submitting.value &&
    missing.value.length === 0,
)

const submitTip = computed(() => {
  if (prefill.value?.exists) return `该 usc 已在 ${meta.value.table} 里存在，不重复写入`
  if (loading.value) return '正在读取预填…'
  if (missing.value.length > 0) {
    const labels = missing.value
      .map((key) => visibleFields.value.find((field) => field.key === key)?.label ?? key)
      .slice(0, 3)
    return `还有 ${missing.value.length} 项必填未填（数值项须大于 0）：${labels.join('、')}${
      missing.value.length > 3 ? ' …' : ''
    }`
  }
  return '核对无误后点「写入」，只写当前这一张表'
})

// ---- 提交 ----

function num(value: string | undefined): number {
  const parsed = Number(String(value ?? '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}

function str(value: string | undefined): string {
  return String(value ?? '').trim()
}

interface CompleteResult {
  target: Target
  usc: string
  created: boolean
  remark: string
}

async function submit() {
  if (!canSubmit.value || !props.row) return
  submitting.value = true
  error.value = ''

  // 只填目标表那一组，另一组留空（后端只读目标组）
  const body: Record<string, unknown> = {
    target: props.target,
    stock_id: props.row.stock_id,
    remark: remark.value.trim(),
    security: {},
    collect: {},
  }
  if (props.target === 'security') {
    body.security = {
      exchange_code: num(form.value.exchange_code),
      security_type: str(form.value.security_type),
      security_code: str(form.value.security_code),
      security_name: str(form.value.security_name),
      security_name_cn: str(form.value.security_name_cn),
      security_name_full: str(form.value.security_name_full),
      currency_type: str(form.value.currency_type),
      timezone: str(form.value.timezone),
      tz: str(form.value.tz),
    }
  } else {
    body.collect = {
      futu_symbol: str(form.value.futu_symbol),
      market_type: num(form.value.market_type),
      market_code: num(form.value.market_code),
      instrument_type: num(form.value.instrument_type),
      sub_instrument_type: num(form.value.sub_instrument_type),
      market_label: str(form.value.market_label),
      instrument: str(form.value.instrument),
      req_section: str(form.value.req_section),
      flag_enable: str(form.value.flag_enable) || '0',
    }
  }

  try {
    const result = await apiPost<CompleteResult>(API, body, 60000)
    const message = result.created
      ? `已写入 ${meta.value.table}：${result.usc}`
      : `${meta.value.table} 里已有 ${result.usc}，未改动`
    showToast(message, result.created ? 'success' : 'warning')
    emit('done')
    close()
  } catch (err) {
    error.value = `补全失败：${(err as Error).message}`
    showToast(error.value, 'error')
  } finally {
    submitting.value = false
  }
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
        <v-icon :icon="meta.icon" class="mr-2" />
        {{ meta.title }}
        <v-chip v-if="row" size="small" variant="tonal" color="primary" class="ml-3">
          {{ row.stock_code }} {{ row.name }}
        </v-chip>
        <v-spacer />
        <v-chip v-if="pendingCheck > 0" size="small" variant="tonal" color="warning">
          待核对 {{ pendingCheck }} 项
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        目标表 <code>{{ meta.table }}</code>；字段已按<strong>快照 / 字典 / 参考同市场</strong>自动预填
        （右上角标签标出来源），核对后点「写入」。已存在的 <code>usc</code> 不会覆盖。
      </v-card-subtitle>

      <v-divider />

      <v-card-text>
        <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />

        <!-- 证券摘要（快照只读） -->
        <v-sheet v-if="row" class="pa-3 rounded mb-3" color="surface-variant">
          <div class="d-flex flex-wrap ga-4 text-caption">
            <span>证券编号 <strong>{{ row.stock_id }}</strong></span>
            <span>代码 <strong>{{ row.stock_code || '—' }}</strong></span>
            <span>名称 <strong>{{ row.name || '—' }}</strong></span>
            <span>市场 <strong>{{ row.market_label || '—' }}</strong></span>
            <span>品种类型 <strong>{{ row.instrument_type || '—' }}</strong></span>
            <span>币种 <strong>{{ row.currency || '—' }}</strong></span>
          </div>
        </v-sheet>

        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
          {{ error }}
        </v-alert>

        <v-alert
          v-if="prefill?.exists"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-3"
        >
          <code>{{ prefill.usc }}</code> 已经在 <code>{{ meta.table }}</code> 里存在，
          本次不再写入（不做覆盖）。如需修改请到对应的维护页面处理。
        </v-alert>

        <v-alert
          v-for="(warn, i) in prefill?.warnings ?? []"
          :key="i"
          type="warning"
          density="compact"
          variant="tonal"
          class="mb-2"
        >
          {{ warn }}
        </v-alert>

        <!-- 写入键（只读，服务端重新派生，前端改了也不采信） -->
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            :model-value="prefill?.usc ?? ''"
            label="usc 统一证券代码（主键，只读）"
            hint="快照 usc 优先，为空时用派生键 upper(市场标签.代码)；服务端重新推导，不受前端影响"
            persistent-hint
            density="compact"
            readonly
            style="min-width: 320px; max-width: 460px"
          />
          <v-chip size="small" color="success" variant="tonal" class="mt-3">自动</v-chip>
        </div>

        <v-row dense>
          <v-col v-for="field in visibleFields" :key="field.key" cols="12" sm="6" md="4">
            <v-text-field
              v-model="form[field.key]"
              :label="field.label + (field.required ? ' *' : '')"
              :hint="field.note"
              :error="!!fieldError(field)"
              :error-messages="fieldError(field) ? [fieldError(field)] : []"
              persistent-hint
              density="compact"
              hide-details="auto"
            >
              <template #append-inner>
                <v-chip size="x-small" variant="tonal" :color="sourceColor(field.source)">
                  {{ sourceText(field.source) }}
                </v-chip>
              </template>
            </v-text-field>
          </v-col>
        </v-row>

        <v-text-field
          v-model="remark"
          label="人工备注（选填）"
          hint="服务端会在 remark 前加 [AUTO-FILL] 标记与时间，便于追踪是补全程序写入的"
          persistent-hint
          density="compact"
          class="mt-4"
        />

        <v-alert v-if="missing.length" type="warning" density="compact" variant="tonal" class="mt-3">
          必填未齐（{{ missing.length }} 项）：
          {{ missing.map((key) => visibleFields.find((f) => f.key === key)?.label ?? key).join('、') }}
          <br />
          数值项必须<strong>大于 0</strong>（<code>market_code</code> 等在无参考行时需人工填，
          留空或填 0 后端都会拒绝）
        </v-alert>
      </v-card-text>

      <v-divider />

      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">{{ submitTip }}</span>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="close">取消</v-btn>
        <v-btn
          color="primary"
          prepend-icon="mdi-check-decagram"
          :disabled="!canSubmit"
          :loading="submitting"
          @click="submit"
        >
          写入
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
