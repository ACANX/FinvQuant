<script setup lang="ts">
// 新增/编辑 OpenAPI 调用方（需求 11169；11174 补默认值）
//
// 接口：GET  /Meta/Finv/Auth/Profile            （取当前登录账号与邮箱做默认值）
//       POST /Meta/Finv/OpenAPI/Client/Save     （idn=0 新建，默认停用）
//
// 默认值口径（需求 11174）：
//   - 负责人账号 = 当前登录账号，只读展示（后端在 uid 为空时也会用它兜底）；
//   - 联系邮箱   = 当前登录账号的资料邮箱，预填后可改；
//   - 授权到期   = 下拉预设（1天/7天/15天/1个月/3个月/半年/1年/18个月/3年/长期），
//                  选中后自动算出具体日期并提交，不再手输日期。
import { computed, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
import { authState } from '../../../../auth'
import { showToast } from '../../../../ui/toast'

interface ClientRow {
  idn: number
  client_code: string
  client_name: string
  owner_uid: string
  contact_email: string
  contract_no: string
  allow_cidr: string[] | null
  rate_limit_rpm: number
  quota_day: number
  dt_expire: string
  flag_enable: string
  remark: string
}

interface Profile {
  uid: string
  nick_name: string
  email: string
}

const props = defineProps<{
  modelValue: boolean
  clientIdn: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

const BASE = '/Meta/Finv/OpenAPI'

// ---------- 授权到期预设 ----------
//
// value 是「天数或月数」的语义标记，真正落库的是算出来的日期（YYYY-MM-DD）。
const EXPIRE_PRESETS = [
  { title: '长期（不过期）', value: 'never' },
  { title: '1 天', value: 'd1' },
  { title: '7 天', value: 'd7' },
  { title: '15 天', value: 'd15' },
  { title: '1 个月', value: 'm1' },
  { title: '3 个月', value: 'm3' },
  { title: '半年', value: 'm6' },
  { title: '1 年', value: 'm12' },
  { title: '18 个月', value: 'm18' },
  { title: '3 年', value: 'm36' },
  { title: '保持当前到期日', value: 'keep' },
]

const form = ref({
  client_code: '',
  client_name: '',
  owner_uid: '',
  contact_email: '',
  contract_no: '',
  allow_cidr_text: '',
  rate_limit_rpm: 0,
  quota_day: 0,
  remark: '',
})
const expirePreset = ref('never')
const existingExpire = ref('') // 编辑时的原到期日（YYYY-MM-DD），供「保持当前」使用
const loading = ref(false)
const saving = ref(false)
const error = ref('')

const isEdit = computed(() => props.clientIdn > 0)
const title = computed(() => (isEdit.value ? '编辑调用方' : '新增调用方'))

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// 「保持当前到期日」只在编辑且原有到期日时出现
const expireOptions = computed(() =>
  isEdit.value && existingExpire.value
    ? EXPIRE_PRESETS
    : EXPIRE_PRESETS.filter((item) => item.value !== 'keep'),
)

/** 月末裁剪的月份加法：2026-01-31 + 1 个月 → 2026-02-28 */
function addMonths(base: Date, months: number): Date {
  const result = new Date(base.getFullYear(), base.getMonth() + months, base.getDate())
  if (result.getDate() !== base.getDate()) {
    result.setDate(0) // 溢出时回退到上个月最后一天
  }
  return result
}

function formatDate(value: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}`
}

/** 按预设算出到期日（YYYY-MM-DD）；long 返回空串表示长期 */
const expireDate = computed(() => {
  const preset = expirePreset.value
  if (preset === 'keep') return existingExpire.value
  if (preset === 'never') return ''
  const now = new Date()
  const days = Number(preset.slice(1))
  switch (preset[0]) {
    case 'd':
      return formatDate(new Date(now.getFullYear(), now.getMonth(), now.getDate() + days))
    case 'm':
      return formatDate(addMonths(now, days))
    default:
      return ''
  }
})

const expireHint = computed(() =>
  expireDate.value ? `到期日：${expireDate.value}（提交时按该日 00:00 +08:00 生效）` : '长期有效，不会自动过期',
)

// 数值字段判空：负数非法（0 = 不限）
const rateError = computed(() => (Number(form.value.rate_limit_rpm) < 0 ? '不能为负数（0 表示不限）' : ''))
const quotaError = computed(() => (Number(form.value.quota_day) < 0 ? '不能为负数（0 表示不限）' : ''))
const codeError = computed(() => {
  const value = form.value.client_code.trim()
  if (!value) return '不能为空'
  if (!/^[a-z0-9][a-z0-9._-]{1,31}$/.test(value)) return '只允许小写字母/数字/._-，2~32 位'
  return ''
})
const nameError = computed(() => (form.value.client_name.trim() ? '' : '不能为空'))
const canSubmit = computed(
  () => !codeError.value && !nameError.value && !rateError.value && !quotaError.value && !saving.value,
)

function resetForm() {
  form.value = {
    client_code: '',
    client_name: '',
    owner_uid: authState.user?.uid ?? '',
    contact_email: '',
    contract_no: '',
    allow_cidr_text: '',
    rate_limit_rpm: 0,
    quota_day: 0,
    remark: '',
  }
  expirePreset.value = 'never'
  existingExpire.value = ''
  error.value = ''
}

/** 新增时带出当前登录账号与邮箱（需求 11174） */
async function fillLoginDefaults() {
  form.value.owner_uid = authState.user?.uid ?? ''
  try {
    const profile = await apiGet<Profile>('/Meta/Finv/Auth/Profile')
    if (profile.uid) form.value.owner_uid = profile.uid
    if (profile.email) form.value.contact_email = profile.email
  } catch (err) {
    // 取不到资料不阻断录入：负责人已有登录账号兜底，邮箱留空可手填
    showToast(`读取登录用户资料失败（可手填邮箱）：${(err as Error).message}`, 'warning')
  }
}

async function loadDetail() {
  if (props.clientIdn <= 0) return
  loading.value = true
  try {
    // 列表接口按 idn 无法直接取单条，这里用 keyword 精确查代号再筛 idn
    const data = await apiGet<{ total: number; list: ClientRow[] }>(
      `${BASE}/Client/Page?page=1&pageSize=200`,
    )
    const found = (data.list ?? []).find((row: ClientRow) => row.idn === props.clientIdn)
    if (!found) {
      error.value = '调用方不存在（可能已被删除）'
      return
    }
    form.value = {
      client_code: found.client_code,
      client_name: found.client_name,
      owner_uid: found.owner_uid ?? '',
      contact_email: found.contact_email ?? '',
      contract_no: found.contract_no ?? '',
      allow_cidr_text: (found.allow_cidr ?? []).join('\n'),
      rate_limit_rpm: found.rate_limit_rpm,
      quota_day: found.quota_day,
      remark: found.remark ?? '',
    }
    existingExpire.value = (found.dt_expire ?? '').slice(0, 10)
    expirePreset.value = existingExpire.value ? 'keep' : 'never'
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

async function submit() {
  if (!canSubmit.value) return
  saving.value = true
  error.value = ''
  try {
    const cidrs = form.value.allow_cidr_text
      .split(/[\n,;]/)
      .map((item) => item.trim())
      .filter((item) => item !== '')
    const saved = await apiPost<ClientRow>(`${BASE}/Client/Save`, {
      idn: props.clientIdn,
      client_code: form.value.client_code.trim(),
      client_name: form.value.client_name.trim(),
      owner_uid: form.value.owner_uid.trim(),
      contact_email: form.value.contact_email.trim(),
      contract_no: form.value.contract_no.trim(),
      allow_cidr: cidrs,
      rate_limit_rpm: Number(form.value.rate_limit_rpm),
      quota_day: Number(form.value.quota_day),
      dt_expire: expireDate.value ? `${expireDate.value}T00:00:00+08:00` : '',
      remark: form.value.remark.trim(),
    })
    showToast(
      isEdit.value
        ? `调用方「${saved.client_code}」已更新`
        : `调用方「${saved.client_code}」已新增（默认停用，核对后请启用）`,
      'success',
    )
    emit('saved')
    visible.value = false
  } catch (err) {
    error.value = (err as Error).message
    showToast(`保存失败：${(err as Error).message}`, 'error')
  } finally {
    saving.value = false
  }
}

watch(
  () => [props.modelValue, props.clientIdn] as const,
  ([open]) => {
    if (!open) return
    resetForm()
    if (props.clientIdn > 0) {
      void loadDetail()
    } else {
      void fillLoginDefaults()
    }
  },
)
</script>

<template>
  <v-dialog v-model="visible" max-width="760" persistent>
    <v-card>
      <v-card-title class="text-subtitle-1 d-flex align-center">
        <v-icon icon="mdi-account-key-outline" class="mr-2" />
        {{ title }}
        <v-chip v-if="!isEdit" size="small" variant="tonal" color="warning" class="ml-2">新建默认停用</v-chip>
      </v-card-title>
      <v-card-subtitle>
        调用方代号用于人工识别；负责人与联系邮箱已按当前登录账号预填，可按需修改；IP 白名单与配额为空/0 表示不限。
      </v-card-subtitle>

      <v-card-text>
        <v-alert v-if="error" type="error" density="compact" variant="tonal" class="mb-3" :text="error" />
        <div class="d-flex flex-column ga-3">
          <div class="d-flex flex-wrap ga-3">
            <v-text-field
              v-model="form.client_code"
              label="调用方代号 *"
              placeholder="如 cdeu-hook"
              density="compact"
              variant="outlined"
              style="min-width: 240px; flex: 1"
              :error="codeError !== ''"
              :error-messages="codeError"
              :disabled="loading"
            />
            <v-text-field
              v-model="form.client_name"
              label="名称 *"
              placeholder="如 CDEU 采集钩子"
              density="compact"
              variant="outlined"
              style="min-width: 240px; flex: 1"
              :error="nameError !== ''"
              :error-messages="nameError"
              :disabled="loading"
            />
          </div>
          <div class="d-flex flex-wrap ga-3">
            <v-text-field
              :model-value="form.owner_uid"
              label="负责人账号"
              density="compact"
              variant="outlined"
              readonly
              persistent-hint
              hint="默认当前登录账号，取自服务端会话，不可在此修改"
              style="min-width: 220px; flex: 1"
              :disabled="loading"
            />
            <v-text-field
              v-model="form.contact_email"
              label="联系邮箱"
              density="compact"
              variant="outlined"
              persistent-hint
              hint="默认取当前登录账号的资料邮箱，可改"
              style="min-width: 220px; flex: 1"
              :disabled="loading"
            />
            <v-text-field
              v-model="form.contract_no"
              label="合同/授权编号"
              density="compact"
              variant="outlined"
              style="min-width: 180px; flex: 1"
              :disabled="loading"
            />
          </div>
          <div class="d-flex flex-wrap ga-3">
            <v-select
              v-model="expirePreset"
              :items="expireOptions"
              item-title="title"
              item-value="value"
              label="授权到期"
              density="compact"
              variant="outlined"
              persistent-hint
              :hint="expireHint"
              style="min-width: 220px; flex: 1"
              :disabled="loading"
            />
            <v-text-field
              v-model.number="form.rate_limit_rpm"
              type="number"
              label="每分钟上限（0 = 不限）"
              density="compact"
              variant="outlined"
              min="0"
              style="min-width: 180px; flex: 1"
              :error="rateError !== ''"
              :error-messages="rateError"
              :disabled="loading"
            />
            <v-text-field
              v-model.number="form.quota_day"
              type="number"
              label="每日配额（0 = 不限）"
              density="compact"
              variant="outlined"
              min="0"
              style="min-width: 180px; flex: 1"
              :error="quotaError !== ''"
              :error-messages="quotaError"
              :disabled="loading"
            />
          </div>
          <v-textarea
            v-model="form.allow_cidr_text"
            label="IP 白名单（每行一个 IP 或 CIDR，留空 = 不限）"
            placeholder="10.0.0.1&#10;192.168.0.0/16&#10;2001:db8::1"
            density="compact"
            variant="outlined"
            rows="3"
            :disabled="loading"
          />
          <v-textarea
            v-model="form.remark"
            label="备注"
            density="compact"
            variant="outlined"
            rows="2"
            :disabled="loading"
          />
        </div>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" :disabled="saving" @click="visible = false">取消</v-btn>
        <v-btn color="primary" :loading="saving" :disabled="!canSubmit" @click="submit">保存</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
