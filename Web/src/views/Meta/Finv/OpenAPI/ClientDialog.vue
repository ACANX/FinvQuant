<script setup lang="ts">
// 新增/编辑 OpenAPI 调用方（需求 11169）
//
// 接口：GET  /Meta/Finv/OpenAPI/Client/Page（父页面已加载列表）
//       POST /Meta/Finv/OpenAPI/Client/Save（idn=0 新建，默认停用）
//
// 校验口径：client_code 只允许小写字母/数字/._-；数值型字段为负即非法（0 表示不限）。
import { computed, ref, watch } from 'vue'

import { apiGet, apiPost } from '../../../../api'
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

const props = defineProps<{
  modelValue: boolean
  clientIdn: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'saved'): void
}>()

const BASE = '/Meta/Finv/OpenAPI'

const form = ref({
  client_code: '',
  client_name: '',
  owner_uid: '',
  contact_email: '',
  contract_no: '',
  allow_cidr_text: '',
  rate_limit_rpm: 0,
  quota_day: 0,
  dt_expire: '',
  remark: '',
})
const loading = ref(false)
const saving = ref(false)
const error = ref('')

const isEdit = computed(() => props.clientIdn > 0)
const title = computed(() => (isEdit.value ? '编辑调用方' : '新增调用方'))

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

// 数值字段判空：负数非法（0 = 不限）
const rateError = computed(() =>
  Number(form.value.rate_limit_rpm) < 0 ? '不能为负数（0 表示不限）' : '',
)
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
    owner_uid: '',
    contact_email: '',
    contract_no: '',
    allow_cidr_text: '',
    rate_limit_rpm: 0,
    quota_day: 0,
    dt_expire: '',
    remark: '',
  }
  error.value = ''
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
      dt_expire: (found.dt_expire ?? '').slice(0, 10),
      remark: found.remark ?? '',
    }
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
      dt_expire: form.value.dt_expire ? `${form.value.dt_expire}T00:00:00+08:00` : '',
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
    void loadDetail()
  },
)
</script>

<template>
  <v-dialog v-model="visible" max-width="720" persistent>
    <v-card>
      <v-card-title class="text-subtitle-1 d-flex align-center">
        <v-icon icon="mdi-account-key-outline" class="mr-2" />
        {{ title }}
        <v-chip v-if="!isEdit" size="small" variant="tonal" color="warning" class="ml-2">新建默认停用</v-chip>
      </v-card-title>
      <v-card-subtitle>
        调用方代号用于人工识别；IP 白名单与配额为空/0 表示不限。停用调用方会让其全部凭证立即失效。
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
              v-model="form.owner_uid"
              label="负责人账号"
              placeholder="对 sys_user.uid"
              density="compact"
              variant="outlined"
              style="min-width: 200px; flex: 1"
              :disabled="loading"
            />
            <v-text-field
              v-model="form.contact_email"
              label="联系邮箱"
              density="compact"
              variant="outlined"
              style="min-width: 200px; flex: 1"
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
            <v-text-field
              v-model="form.dt_expire"
              type="date"
              label="授权到期（留空 = 长期）"
              density="compact"
              variant="outlined"
              style="min-width: 180px; flex: 1"
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
