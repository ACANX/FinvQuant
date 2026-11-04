<script setup lang="ts">
// 待录入证券 → 录入对话框（需求 10996）
//
// 用途：把一只「尚待录入」的证券，按三张表各自的字段登记一遍：
//   1. public.finv_security         证券主数据
//   2. futu.finv_futu_quote_collect 采集配置
//   3. cdeu.finv_quote_cdeu_usc     usc 主数据
//
// 当前阶段约定（需求 10996）：
//   - 只做「字段展示 + 必填校验 + 按钮启用」；
//   - **推断逻辑不实现**：除快照里同名字段的直传（代码/名称/市场/品种类型/币种）外，
//     不做任何推导；其余字段由人工填写；
//   - 「录入」写入接口尚未实现（等推断规则确定后一并接入），点击时给出明确提示。
//
// 必填判定 = 目标表上 `NOT NULL 且无 DEFAULT` 的列，清单见下方常量（与库结构核对于 2026-11-05）。
import { computed, ref, watch } from 'vue'

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
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// ---- 字段定义（必填 = NOT NULL 且无 DEFAULT）----

interface Field {
  key: string
  label: string
  hint?: string
  required?: boolean
  type?: 'text' | 'number'
}

/** public.finv_security：必填 usc / exchange_code / security_type / security_code /
 *  security_name / security_name_cn / currency_type（其余列有默认值或可空） */
const SECURITY_FIELDS: Field[] = [
  { key: 'exchange_code', label: 'exchange_code 交易所代码', required: true, type: 'number', hint: '如 HK=21、SZ=12、SH=11（以 finv_security 既有数据为准）' },
  { key: 'security_type', label: 'security_type 证券类型', required: true, hint: '如 Stock / ETF' },
  { key: 'security_code', label: 'security_code 证券代码', required: true },
  { key: 'security_name', label: 'security_name 证券名称（英文）', required: true },
  { key: 'security_name_cn', label: 'security_name_cn 证券名称（中文）', required: true },
  { key: 'currency_type', label: 'currency_type 币种', required: true },
  { key: 'security_name_full', label: 'security_name_full 全称' },
  { key: 'timezone', label: 'timezone 时区' },
  { key: 'tz', label: 'tz 时区缩写' },
  { key: 'init_date', label: 'init_date 初始化日期', type: 'number', hint: '默认 20000000' },
  { key: 'market_code', label: 'market_code 市场代码', type: 'number', hint: '默认 0' },
  { key: 'flag_enable', label: 'flag_enable 是否启用', hint: "默认 '0'；录入后通常置 '1'" },
  { key: 'remark', label: 'remark 备注' },
]

/** futu.finv_futu_quote_collect：必填 usc / stock_id / futu_symbol / market_type /
 *  market_code / instrument_type / sub_instrument_type / quote_market / type_symbol */
const FUTU_FIELDS: Field[] = [
  { key: 'stock_id', label: 'stock_id 证券编号', required: true, type: 'number', hint: '来自快照，一般直接沿用' },
  { key: 'futu_symbol', label: 'futu_symbol futu 代码', required: true, hint: '如 00700、NVDA.US' },
  { key: 'market_type', label: 'market_type 市场类型', required: true, type: 'number', hint: '1=HK 2=US 4=CN' },
  { key: 'market_code', label: 'market_code 市场代码', required: true, type: 'number' },
  { key: 'instrument_type', label: 'instrument_type 品种类型', required: true, type: 'number', hint: '来自快照 instrument_type' },
  { key: 'sub_instrument_type', label: 'sub_instrument_type 子品种类型', required: true, type: 'number' },
  { key: 'quote_market', label: 'quote_market 行情市场', required: true, hint: '如 HK / US / CN' },
  { key: 'type_symbol', label: 'type_symbol 代码类型符号', required: true },
  { key: 'req_section', label: 'req_section 请求分段' },
  { key: 'flag_enable', label: 'flag_enable 是否启用', hint: "默认 '0'；录入后通常置 '1'" },
  { key: 'remark', label: 'remark 备注' },
]

/** cdeu.finv_quote_cdeu_usc：必填只有 usc，其余可空 */
const USC_FIELDS: Field[] = [
  { key: 'category', label: 'category 分类' },
  { key: 'region', label: 'region 地区' },
  { key: 'market', label: 'market 市场', hint: '如 HK / SZ / SH / US' },
  { key: 'provider', label: 'provider 数据源', hint: '如 FTMM' },
  { key: 'name', label: 'name 名称' },
  { key: 'timezone', label: 'timezone 时区' },
  { key: 'currency', label: 'currency 币种' },
  { key: 'day_expect_rows', label: 'day_expect_rows 每日预期条数', type: 'number' },
  { key: 'date_list', label: 'date_list 上市日期', hint: 'yyyy-MM-dd' },
  { key: 'date_delist', label: 'date_delist 退市日期', hint: 'yyyy-MM-dd' },
  { key: 'priority_rank', label: 'priority_rank 优先级', type: 'number' },
  { key: 'sla_days', label: 'sla_days SLA 天数', type: 'number' },
  { key: 'max_wait_days', label: 'max_wait_days 最大等待天数', type: 'number' },
]

// ---- 表单状态 ----

/** usc 三张表共用，单独放在顶部填写（为空则三张表都无法录入） */
const usc = ref('')
const securityForm = ref<Record<string, string>>({})
const futuForm = ref<Record<string, string>>({})
const uscForm = ref<Record<string, string>>({})

function blank(fields: Field[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const field of fields) out[field.key] = ''
  return out
}

/** 用快照里「同名字段」直传（不做任何推断），其余留空由人工填 */
function resetByRow(row: PendingSecuRow | null) {
  usc.value = ''
  securityForm.value = blank(SECURITY_FIELDS)
  futuForm.value = blank(FUTU_FIELDS)
  uscForm.value = blank(USC_FIELDS)
  if (!row) return

  securityForm.value.security_code = row.stock_code ?? ''
  securityForm.value.security_name = row.name ?? ''
  securityForm.value.security_name_cn = row.name ?? ''
  securityForm.value.currency_type = row.currency ?? ''

  futuForm.value.stock_id = row.stock_id ? String(row.stock_id) : ''
  futuForm.value.market_type = row.market_type ?? ''
  futuForm.value.instrument_type = row.instrument_type ?? ''
  futuForm.value.quote_market = row.market_label ?? ''

  uscForm.value.market = row.market_label ?? ''
  uscForm.value.name = row.name ?? ''
  uscForm.value.currency = row.currency ?? ''
}

watch(
  () => [props.modelValue, props.row] as const,
  ([open]) => {
    if (open) resetByRow(props.row)
  },
  { immediate: true },
)

// ---- 必填校验 ----

/** 缺失的必填项（中文标签），为空表示可以提交 */
const missing = computed<string[]>(() => {
  const out: string[] = []
  if (!usc.value.trim()) out.push('usc（三表共用）')
  for (const field of SECURITY_FIELDS) {
    if (field.required && !String(securityForm.value[field.key] ?? '').trim()) {
      out.push(`finv_security.${field.key}`)
    }
  }
  for (const field of FUTU_FIELDS) {
    if (field.required && !String(futuForm.value[field.key] ?? '').trim()) {
      out.push(`finv_futu_quote_collect.${field.key}`)
    }
  }
  return out
})

const canSubmit = computed(() => missing.value.length === 0)

const missingTip = computed(() =>
  canSubmit.value ? '必填项已齐全，可提交录入' : `还有 ${missing.value.length} 项必填未填：${missing.value.slice(0, 3).join('、')}${missing.value.length > 3 ? ' …' : ''}`,
)

/** 各表必填项汇总（给分区标题的角标用） */
const securityMissing = computed(() =>
  SECURITY_FIELDS.filter((f) => f.required && !String(securityForm.value[f.key] ?? '').trim()).length,
)
const futuMissing = computed(() =>
  FUTU_FIELDS.filter((f) => f.required && !String(futuForm.value[f.key] ?? '').trim()).length,
)
const uscMissing = computed(() => (usc.value.trim() ? 0 : 1))

// ---- 提交（写入接口待接入）----

const submitted = ref('')

function submit() {
  if (!canSubmit.value) return
  // 需求 10996 当前阶段：只做字段登记与校验，写入三张表的接口等推断规则确定后实现。
  submitted.value =
    `字段已齐全，但写入三张表的接口尚未接入（需求 10996 规划中）。` +
    `届时将依次写入 public.finv_security → futu.finv_futu_quote_collect → cdeu.finv_quote_cdeu_usc。`
}

function close() {
  visible.value = false
  submitted.value = ''
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
      </v-card-title>
      <v-card-subtitle class="pb-2">
        依次写入 public.finv_security → futu.finv_futu_quote_collect → cdeu.finv_quote_cdeu_usc。
        带 <span class="text-error">*</span> 为目标表上 NOT NULL 且无默认值的字段，必须填写。
      </v-card-subtitle>

      <v-divider />

      <v-card-text>
        <!-- 证券摘要（只读） -->
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

        <!-- 三表共用 usc -->
        <div class="d-flex flex-wrap align-center ga-2 mb-3">
          <v-text-field
            v-model="usc"
            label="usc 统一证券代码 *"
            hint="三张表共用；建议与证券代码一致（推断规则待定，当前请手工确认）"
            persistent-hint
            density="compact"
            style="min-width: 280px; max-width: 360px"
          />
          <v-chip size="small" :color="uscMissing ? 'warning' : 'success'" variant="tonal" class="mt-3">
            {{ uscMissing ? '必填' : '已填' }}
          </v-chip>
        </div>

        <v-expansion-panels :model-value="[0, 1, 2]" multiple variant="accordion">
          <!-- 1. public.finv_security -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-shield-key-outline" class="mr-2" size="18" />
              public.finv_security（证券主数据）
              <v-chip size="x-small" class="ml-2" :color="securityMissing ? 'warning' : 'success'" variant="tonal">
                {{ securityMissing ? `缺 ${securityMissing} 项必填` : '必填已齐' }}
              </v-chip>
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-row dense>
                <v-col v-for="field in SECURITY_FIELDS" :key="field.key" cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="securityForm[field.key]"
                    :label="field.label + (field.required ? ' *' : '')"
                    :hint="field.hint"
                    :type="field.type === 'number' ? 'number' : 'text'"
                    :persistent-hint="!!field.hint"
                    density="compact"
                    hide-details="auto"
                  />
                </v-col>
              </v-row>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 2. futu.finv_futu_quote_collect -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-server-network" class="mr-2" size="18" />
              futu.finv_futu_quote_collect（采集配置）
              <v-chip size="x-small" class="ml-2" :color="futuMissing ? 'warning' : 'success'" variant="tonal">
                {{ futuMissing ? `缺 ${futuMissing} 项必填` : '必填已齐' }}
              </v-chip>
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-row dense>
                <v-col v-for="field in FUTU_FIELDS" :key="field.key" cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="futuForm[field.key]"
                    :label="field.label + (field.required ? ' *' : '')"
                    :hint="field.hint"
                    :type="field.type === 'number' ? 'number' : 'text'"
                    :persistent-hint="!!field.hint"
                    density="compact"
                    hide-details="auto"
                  />
                </v-col>
              </v-row>
            </v-expansion-panel-text>
          </v-expansion-panel>

          <!-- 3. cdeu.finv_quote_cdeu_usc -->
          <v-expansion-panel>
            <v-expansion-panel-title>
              <v-icon icon="mdi-database-cog-outline" class="mr-2" size="18" />
              cdeu.finv_quote_cdeu_usc（usc 主数据）
              <v-chip size="x-small" class="ml-2" :color="uscMissing ? 'warning' : 'success'" variant="tonal">
                {{ uscMissing ? '缺 usc' : '必填已齐' }}
              </v-chip>
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-row dense>
                <v-col v-for="field in USC_FIELDS" :key="field.key" cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="uscForm[field.key]"
                    :label="field.label + (field.required ? ' *' : '')"
                    :hint="field.hint"
                    :type="field.type === 'number' ? 'number' : 'text'"
                    :persistent-hint="!!field.hint"
                    density="compact"
                    hide-details="auto"
                  />
                </v-col>
              </v-row>
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>

        <v-alert v-if="missing.length" type="warning" density="compact" variant="tonal" class="mt-3">
          必填未齐（{{ missing.length }} 项）：{{ missing.join('、') }}
        </v-alert>
        <v-alert v-if="submitted" type="info" density="compact" variant="tonal" class="mt-3">
          {{ submitted }}
        </v-alert>
      </v-card-text>

      <v-divider />

      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">{{ missingTip }}</span>
        <v-spacer />
        <v-btn variant="text" @click="close">取消</v-btn>
        <v-tooltip :text="missingTip" location="top" :disabled="canSubmit">
          <template #activator="{ props: tipProps }">
            <span v-bind="tipProps">
              <v-btn color="primary" prepend-icon="mdi-check-decagram" :disabled="!canSubmit" @click="submit">
                录入
              </v-btn>
            </span>
          </template>
        </v-tooltip>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
