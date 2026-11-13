<script setup lang="ts">
// 采集参数单行编辑对话框（需求 11254）
// 只提交**改动过的字段**（未改的不出现在请求体里），避免把别人的值顺手覆盖。
// 不可编辑：usc（主键）、ftmm_symbol（生成列，PG 拒绝显式赋值）、gmt_*（触发器维护）。
import { computed, ref, watch } from 'vue'

import { apiPost } from '../../../../../api'
import { COLLECT_INSTRUMENT_ITEMS, COLLECT_MARKET_OPTIONS } from '../../../../../config/collectParam'
import type { CollectParamRow } from '../../../../../config/collectParam'
import { showToast } from '../../../../../ui/toast'

const UPDATE_API = '/Meta/Finv/Quote/Secu/Collect/Update'

const props = defineProps<{
  modelValue: boolean
  row: CollectParamRow | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

/** 可编辑表单（数字列用字符串承接 v-text-field，提交前转整数） */
interface CollectForm {
  stock_id: string
  market_label: string
  futu_symbol: string
  market_type: string
  market_code: string
  instrument: string
  instrument_type: string
  sub_instrument_type: string
  req_section: string
  flag_enable: boolean
  remark: string
}

const form = ref<CollectForm>(emptyForm())
const saving = ref(false)
const validationError = ref('')
const conflictHint = ref('')

function emptyForm(): CollectForm {
  return {
    stock_id: '',
    market_label: '',
    futu_symbol: '',
    market_type: '',
    market_code: '',
    instrument: '',
    instrument_type: '',
    sub_instrument_type: '',
    req_section: '',
    flag_enable: false,
    remark: '',
  }
}

/** 打开时按当前行回填 */
watch(
  () => [props.modelValue, props.row] as const,
  ([open, row]) => {
    if (!open || !row) return
    form.value = {
      stock_id: String(row.stock_id ?? ''),
      market_label: row.market_label ?? '',
      futu_symbol: row.futu_symbol ?? '',
      market_type: String(row.market_type ?? ''),
      market_code: String(row.market_code ?? ''),
      instrument: row.instrument ?? '',
      instrument_type: String(row.instrument_type ?? ''),
      sub_instrument_type: String(row.sub_instrument_type ?? ''),
      req_section: row.req_section ?? '',
      flag_enable: row.flag_enable === '1',
      remark: row.remark ?? '',
    }
    validationError.value = ''
    conflictHint.value = ''
  },
  { immediate: true },
)

const metaText = computed(() => {
  const row = props.row
  if (!row) return '—'
  if (!row.security_exists) return '主数据里还没有该证券（去「待采证券录入」或「证券参数补全」补录）'
  const parts = [
    row.security_name || '—',
    row.security_type || '—',
    row.security_sub_type || '—',
    row.currency_type || '—',
    row.exchange_name || '—',
    row.security_market_name || '—',
  ]
  return parts.join(' ｜ ')
})

/** 正整数校验：空或非法返回错误文案 */
function positiveInt(value: string, label: string): string {
  const n = Number(value)
  if (!Number.isInteger(n) || n <= 0) return `${label}必须是正整数`
  return ''
}

/** 组装改动字段：只包含与原始行不同的列 */
function buildFields(): Record<string, string | number> {
  const row = props.row
  const fields: Record<string, string | number> = {}
  if (!row) return fields

  const ints: [keyof CollectForm, string, number][] = [
    ['stock_id', '证券编号', row.stock_id],
    ['market_type', '富途市场类型', row.market_type],
    ['market_code', '富途市场代码', row.market_code],
    ['instrument_type', '富途品种类型', row.instrument_type],
    ['sub_instrument_type', '富途品种子类型', row.sub_instrument_type],
  ]
  for (const [key, label, current] of ints) {
    const raw = String(form.value[key]).trim()
    const parsed = Number(raw)
    if (parsed !== current) {
      const message = positiveInt(raw, label)
      if (message) throw new Error(message)
      fields[key as string] = parsed
    }
  }

  const texts: [keyof CollectForm, string, string][] = [
    ['market_label', '行情市场', row.market_label ?? ''],
    ['futu_symbol', '富途代码', row.futu_symbol ?? ''],
    ['instrument', '品种', row.instrument ?? ''],
    ['req_section', '订阅段', row.req_section ?? ''],
    ['remark', '备注', row.remark ?? ''],
  ]
  for (const [key, label, current] of texts) {
    const value = String(form.value[key]).trim()
    if (value !== current) {
      if (!value && key !== 'req_section' && key !== 'remark') throw new Error(`${label}不能为空`)
      fields[key as string] = value
    }
  }

  const flag = form.value.flag_enable ? '1' : '0'
  if (flag !== (row.flag_enable ?? '')) fields.flag_enable = flag

  return fields
}

async function save(): Promise<void> {
  validationError.value = ''
  conflictHint.value = ''
  const row = props.row
  if (!row) return

  let fields: Record<string, string | number>
  try {
    fields = buildFields()
  } catch (err) {
    validationError.value = (err as Error).message
    return
  }
  if (Object.keys(fields).length === 0) {
    showToast('没有改动，无需保存', 'info')
    return
  }

  saving.value = true
  try {
    await apiPost(UPDATE_API, { usc: row.usc, fields })
    emit('update:modelValue', false)
    emit('saved')
  } catch (err) {
    const message = (err as Error).message
    // 唯一键冲突（409）：保持对话框打开，提示改回去或换值
    if (message.includes('唯一约束') || message.includes('冲突')) {
      conflictHint.value = message
    } else {
      showToast(`保存失败：${message}`, 'error')
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <v-dialog
    :model-value="modelValue"
    max-width="880"
    scrollable
    @update:model-value="emit('update:modelValue', $event)"
  >
    <v-card>
      <v-card-title class="d-flex align-center ga-2">
        <v-icon icon="mdi-pencil" />
        <span>编辑采集参数</span>
        <code class="text-caption text-medium-emphasis">{{ row?.usc || '—' }}</code>
        <v-spacer />
        <v-btn size="small" variant="text" icon="mdi-close" @click="emit('update:modelValue', false)" />
      </v-card-title>
      <v-divider />

      <v-card-text>
        <v-alert type="warning" density="compact" variant="tonal" class="mb-3">
          保存后立即写入 <code>futu.finv_futu_quote_collect</code>；<code>ftmm_symbol</code>（生成列）会自动跟随
          「行情市场 + 富途代码」变化。
        </v-alert>

        <!-- 只读区：主数据元数据 + 身份列 -->
        <div class="finv-collect-readonly mb-4">
          <div class="text-caption text-medium-emphasis mb-1">证券元数据（左关联 public.finv_security）</div>
          <div class="text-body-2">{{ metaText }}</div>
          <div class="text-caption text-medium-emphasis mt-2">
            富途键 <code>{{ row?.ftmm_symbol || '—' }}</code>
            ｜ 更新时间 <code>{{ row?.gmt_update || '—' }}</code>
          </div>
        </div>

        <v-row dense>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.stock_id"
              label="证券编号 stock_id"
              type="number"
              density="compact"
              hide-details
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-combobox
              v-model="form.market_label"
              :items="COLLECT_MARKET_OPTIONS"
              label="行情市场 market_label"
              density="compact"
              hide-details
              :return-object="false"
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.futu_symbol"
              label="富途代码 futu_symbol"
              density="compact"
              hide-details
              required
            />
          </v-col>

          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.market_type"
              label="富途市场类型 market_type"
              type="number"
              density="compact"
              hide-details
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.market_code"
              label="富途市场代码 market_code"
              type="number"
              density="compact"
              hide-details
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-combobox
              v-model="form.instrument"
              :items="COLLECT_INSTRUMENT_ITEMS"
              item-title="title"
              item-value="value"
              label="品种 instrument"
              density="compact"
              hide-details
              :return-object="false"
              required
            />
          </v-col>

          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.instrument_type"
              label="富途品种类型 instrument_type"
              type="number"
              density="compact"
              hide-details
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.sub_instrument_type"
              label="富途品种子类型 sub_instrument_type"
              type="number"
              density="compact"
              hide-details
              required
            />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              v-model="form.req_section"
              label="订阅段 req_section（选填）"
              density="compact"
              hide-details
            />
          </v-col>

          <v-col cols="12" sm="6" md="4">
            <v-switch
              v-model="form.flag_enable"
              color="success"
              density="compact"
              hide-details
              :label="form.flag_enable ? '采集启用' : '采集禁用'"
            />
          </v-col>
          <v-col cols="12" sm="6" md="8">
            <v-text-field
              v-model="form.remark"
              label="备注 remark（选填）"
              density="compact"
              hide-details
            />
          </v-col>
        </v-row>

        <v-alert v-if="validationError" type="error" density="compact" variant="tonal" class="mt-3">
          {{ validationError }}
        </v-alert>
        <v-alert v-if="conflictHint" type="error" density="compact" variant="tonal" class="mt-3">
          {{ conflictHint }}
        </v-alert>
      </v-card-text>

      <v-divider />
      <v-card-actions>
        <span class="text-caption text-medium-emphasis ml-2">只提交改动过的字段</span>
        <v-spacer />
        <v-btn variant="text" @click="emit('update:modelValue', false)">取消</v-btn>
        <v-btn color="primary" :loading="saving" prepend-icon="mdi-content-save" @click="save">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.finv-collect-readonly {
  border-left: 3px solid rgba(var(--v-theme-primary), 0.6);
  padding-left: 10px;
}
</style>
