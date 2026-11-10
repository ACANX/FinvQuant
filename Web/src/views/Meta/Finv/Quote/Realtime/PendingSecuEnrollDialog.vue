<script setup lang="ts">
// 待录入证券 →「录入证券元数据」对话框（需求 10996 落地 / 11147 自动化）
//
// 数据来源：GET /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Prefill?stockId=
//   —— 服务端已把「能关联到的」全部算好并标注来源（快照 / 字典 / 参考同市场 / 缺省 / 待填），
//      这里只负责展示 + 允许改，不再要求人工从零填写。
// 提交：POST /Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enroll
//   —— 单事务写 public.finv_security + futu.finv_futu_quote_collect，并把 usc 回写快照表，
//      该行随后从待录入清单消失；两张表按主键 ON CONFLICT DO NOTHING（幂等、不覆盖人工数据）。
//
// 需求 11147 起 **`cdeu.finv_quote_cdeu_usc` 不再录入、也不再展示**：
// 该表数据由运行时采集链路动态生成，人工补录没有意义。
import { computed, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../../api'
import { showToast } from '../../../../../ui/toast'

interface PendingSecuRow {
  stock_id: number
  name: string
  stock_code: string
  market_type: string
  market_label: string
  instrument_type: string
  currency: string
}

const props = defineProps<{
  modelValue: boolean
  row: PendingSecuRow | null
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  done: []
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// ---- 预填数据结构（与后端 PendingSecuPrefill 一致）----

interface PrefillField {
  key: string
  label: string
  value: string
  source: string
  required: boolean
  note: string
}

interface PendingSecuPrefill {
  stock_id: number
  usc: string
  name: string
  security: PrefillField[]
  collect: PrefillField[]
  warnings: string[]
}

/** 来源标记 → 中文（下拉/表格列口径按 DictColumnSpec：只显示中文） */
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
const prefill = ref<PendingSecuPrefill | null>(null)
const securityForm = ref<Record<string, string>>({})
const collectForm = ref<Record<string, string>>({})
const remark = ref('')

function fillForm(fields: PrefillField[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const field of fields) out[field.key] = field.value ?? ''
  return out
}

/** 打开对话框时拉预填：失败给出可见错误，不静默 */
async function loadPrefill() {
  prefill.value = null
  error.value = ''
  remark.value = ''
  securityForm.value = {}
  collectForm.value = {}
  const row = props.row
  if (!row?.stock_id) return

  loading.value = true
  try {
    const data = await apiGet<PendingSecuPrefill>(
      `/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Prefill?stockId=${row.stock_id}`,
    )
    prefill.value = data
    securityForm.value = fillForm(data.security ?? [])
    collectForm.value = fillForm(data.collect ?? [])
  } catch (err) {
    error.value = `预填失败：${(err as Error).message}`
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.modelValue, props.row?.stock_id] as const,
  ([open]) => {
    if (open) void loadPrefill()
  },
  { immediate: true },
)

// ---- 必填校验 ----

const missing = computed<string[]>(() => {
  const out: string[] = []
  for (const field of prefill.value?.security ?? []) {
    if (field.required && !String(securityForm.value[field.key] ?? '').trim()) {
      out.push(`finv_security.${field.key}`)
    }
  }
  for (const field of prefill.value?.collect ?? []) {
    if (field.required && !String(collectForm.value[field.key] ?? '').trim()) {
      out.push(`finv_futu_quote_collect.${field.key}`)
    }
  }
  return out
})

const canSubmit = computed(
  () => !!prefill.value && !loading.value && !submitting.value && missing.value.length === 0,
)

const missingTip = computed(() =>
  canSubmit.value
    ? '预填项已齐全，点「录入」写入并回写快照 usc'
    : `还有 ${missing.value.length} 项必填未填：${missing.value
        .slice(0, 3)
        .join('、')}${missing.value.length > 3 ? ' …' : ''}`,
)

/** 待人工确认的字段数（来源为缺省/待填），用于标题角标提示「核对即可」 */
const pendingCheck = computed(
  () =>
    [...(prefill.value?.security ?? []), ...(prefill.value?.collect ?? [])].filter(
      (f) => f.source === 'default' || f.source === 'manual' || f.source === 'reference',
    ).length,
)

// ---- 提交 ----

function num(value: string | undefined): number {
  const parsed = Number(String(value ?? '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}

function str(value: string | undefined): string {
  return String(value ?? '').trim()
}

interface EnrollResult {
  usc: string
  security_created: boolean
  collect_created: boolean
  snapshot_updated: boolean
  remark: string
}

async function submit() {
  if (!canSubmit.value || !props.row) return
  submitting.value = true
  error.value = ''
  try {
    const result = await apiPost<EnrollResult>(
      '/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending/Enroll',
      {
        stock_id: props.row.stock_id,
        remark: remark.value.trim(),
        security: {
          exchange_code: num(securityForm.value.exchange_code),
          security_type: str(securityForm.value.security_type),
          security_code: str(securityForm.value.security_code),
          security_name: str(securityForm.value.security_name),
          security_name_cn: str(securityForm.value.security_name_cn),
          security_name_full: str(securityForm.value.security_name_full),
          currency_type: str(securityForm.value.currency_type),
          timezone: str(securityForm.value.timezone),
          tz: str(securityForm.value.tz),
        },
        collect: {
          futu_symbol: str(collectForm.value.futu_symbol),
          market_type: num(collectForm.value.market_type),
          market_code: num(collectForm.value.market_code),
          instrument_type: num(collectForm.value.instrument_type),
          sub_instrument_type: num(collectForm.value.sub_instrument_type),
          market_label: str(collectForm.value.market_label),
          instrument: str(collectForm.value.instrument),
          req_section: str(collectForm.value.req_section),
          flag_enable: str(collectForm.value.flag_enable) || '0',
        },
      },
      60000,
    )
    const parts = [
      result.security_created ? '已建证券主数据' : '证券主数据已存在（未覆盖）',
      result.collect_created ? '已建采集配置' : '采集配置已存在（未覆盖）',
      result.snapshot_updated ? '已回写快照 usc' : '快照 usc 已有值（未改）',
    ]
    showToast(`补录完成 ${result.usc}：${parts.join('、')}`, 'success')
    emit('done')
    close()
  } catch (err) {
    error.value = `补录失败：${(err as Error).message}`
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
  <v-dialog v-model="visible" max-width="1120" scrollable>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-database-plus-outline" class="mr-2" />
        录入证券元数据
        <v-chip v-if="row" size="small" variant="tonal" color="primary" class="ml-3">
          {{ row.stock_code }} {{ row.name }}
        </v-chip>
        <v-spacer />
        <v-chip v-if="pendingCheck > 0" size="small" variant="tonal" color="warning">
          待核对 {{ pendingCheck }} 项
        </v-chip>
      </v-card-title>
      <v-card-subtitle class="pb-2">
        能关联到的一律已<strong>自动填好</strong>（右上角标签标出来源：快照 / 字典 / 参考 / 缺省），
        只需核对后点「录入」。写入 <code>public.finv_security</code> +
        <code>futu.finv_futu_quote_collect</code>，并回写快照 <code>usc</code>，该行随即离开待录入清单。
        <code>cdeu.finv_quote_cdeu_usc</code> <strong>不再录入</strong>（该表数据由运行时采集链路动态生成）。
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
            <span>市场 <strong>{{ row.market_label || '—' }}</strong>（market_type={{ row.market_type }}）</span>
            <span>品种类型 <strong>{{ row.instrument_type || '—' }}</strong></span>
            <span>币种 <strong>{{ row.currency || '—' }}</strong></span>
          </div>
        </v-sheet>

        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3">
          {{ error }}
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

        <!-- 派生键（只读，服务端重新推导，前端改了也不会采信） -->
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            :model-value="prefill?.usc ?? ''"
            label="usc 统一证券代码（派生，只读）"
            hint="由「市场标签.代码」派生并统一大写；两张表共用，服务端重新推导，不受前端影响"
            persistent-hint
            density="compact"
            readonly
            style="min-width: 320px; max-width: 420px"
          />
          <v-chip size="small" color="success" variant="tonal" class="mt-3">自动</v-chip>
        </div>

        <v-expansion-panels :model-value="[0, 1]" multiple variant="accordion">
          <!-- 1. public.finv_security -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-shield-key-outline" class="mr-2" size="18" />
              public.finv_security（证券主数据）
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-row dense>
                <v-col v-for="field in prefill?.security ?? []" :key="field.key" cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="securityForm[field.key]"
                    :label="field.label + (field.required ? ' *' : '')"
                    :hint="field.note"
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
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 2. futu.finv_futu_quote_collect -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-server-network" class="mr-2" size="18" />
              futu.finv_futu_quote_collect（采集配置）
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-row dense>
                <v-col v-for="field in prefill?.collect ?? []" :key="field.key" cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="collectForm[field.key]"
                    :label="field.label + (field.required ? ' *' : '')"
                    :hint="field.note"
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
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>

        <v-text-field
          v-model="remark"
          label="人工备注（选填）"
          hint="服务端会在两张表的 remark 前加 [AUTO-FILL] 标记与时间，便于后续追踪是补录程序写入的"
          persistent-hint
          density="compact"
          class="mt-3"
        />

        <v-alert v-if="missing.length" type="warning" density="compact" variant="tonal" class="mt-3">
          必填未齐（{{ missing.length }} 项）：{{ missing.join('、') }}
        </v-alert>
      </v-card-text>

      <v-divider />

      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">{{ missingTip }}</span>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="close">取消</v-btn>
        <v-btn
          color="primary"
          prepend-icon="mdi-check-decagram"
          :disabled="!canSubmit"
          :loading="submitting"
          @click="submit"
        >
          录入
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
