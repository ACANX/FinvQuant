// 修改密码对话框（需求 10763）
//
// 安全与体验约定：
//   - 三个输入框都支持右侧眼睛图标切换明文/密文
//   - 口令只在本组件内存中停留，提交成功后立即清空
//   - 提交前做本地一致性校验（两次新密码是否一致），强度由服务端裁决
//   - 成功后当前会话保留，其它设备上的登录会被服务端吊销

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { apiPost } from '../../api'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'changed', revokedSessions: number): void
}>()

interface ChangeResult {
  revoked_sessions: number
}

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')

const showCurrent = ref(false)
const showNew = ref(false)
const showConfirm = ref(false)

const submitting = ref(false)
const errorMessage = ref('')

/** 口令强度规则（与服务端 ValidatePasswordStrength 保持一致） */
const STRENGTH_HINT = '至少 8 位，且需包含大写字母、小写字母、数字、符号中的至少 3 类'

/** 输入即校验：长度与类别是否达标，用于给用户即时反馈 */
const strengthOk = computed(() => {
  const pwd = newPassword.value
  if (pwd.length < 8 || pwd.length > 64) return false
  let classes = 0
  if (/[A-Z]/.test(pwd)) classes++
  if (/[a-z]/.test(pwd)) classes++
  if (/[0-9]/.test(pwd)) classes++
  if (/[^A-Za-z0-9]/.test(pwd)) classes++
  return classes >= 3
})

const confirmMismatch = computed(
  () => confirmPassword.value.length > 0 && confirmPassword.value !== newPassword.value,
)

const canSubmit = computed(
  () =>
    currentPassword.value.length > 0 &&
    strengthOk.value &&
    confirmPassword.value === newPassword.value &&
    !submitting.value,
)

function reset(): void {
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  showCurrent.value = false
  showNew.value = false
  showConfirm.value = false
  errorMessage.value = ''
  submitting.value = false
}

// 关闭时清空，避免下次打开残留上次输入
watch(visible, (open) => {
  if (!open) reset()
})

async function onSubmit(): Promise<void> {
  if (!canSubmit.value) return
  errorMessage.value = ''
  submitting.value = true
  try {
    const data = await apiPost<ChangeResult>('/Auth/ChangePassword', {
      current_password: currentPassword.value,
      new_password: newPassword.value,
    })
    const revoked = data?.revoked_sessions ?? 0
    reset()
    visible.value = false
    emit('changed', revoked)
  } catch (e) {
    errorMessage.value = (e as Error).message || '修改密码失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <v-dialog v-model="visible" max-width="460" persistent>
    <v-card>
      <v-card-title class="d-flex align-center">
        <v-icon icon="mdi-lock-reset" class="mr-2" />
        修改密码
      </v-card-title>
      <v-card-subtitle>修改成功后，其它设备上的登录状态将失效</v-card-subtitle>

      <v-card-text>
        <v-text-field
          v-model="currentPassword"
          label="当前密码"
          :type="showCurrent ? 'text' : 'password'"
          prepend-inner-icon="mdi-lock-outline"
          :append-inner-icon="showCurrent ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
          :aria-label="showCurrent ? '隐藏当前密码' : '显示当前密码'"
          autocomplete="current-password"
          variant="outlined"
          density="comfortable"
          class="mb-3"
          :disabled="submitting"
          @click:append-inner="showCurrent = !showCurrent"
        />

        <v-text-field
          v-model="newPassword"
          label="新密码"
          :type="showNew ? 'text' : 'password'"
          prepend-inner-icon="mdi-lock-plus-outline"
          :append-inner-icon="showNew ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
          :aria-label="showNew ? '隐藏新密码' : '显示新密码'"
          autocomplete="new-password"
          variant="outlined"
          density="comfortable"
          :hint="STRENGTH_HINT"
          persistent-hint
          :disabled="submitting"
          @click:append-inner="showNew = !showNew"
        />

        <v-progress-linear
          v-if="newPassword.length > 0"
          :model-value="strengthOk ? 100 : 40"
          :color="strengthOk ? 'success' : 'warning'"
          height="3"
          rounded
          class="mt-1 mb-3"
        />

        <v-text-field
          v-model="confirmPassword"
          label="确认新密码"
          :type="showConfirm ? 'text' : 'password'"
          prepend-inner-icon="mdi-lock-check-outline"
          :append-inner-icon="showConfirm ? 'mdi-eye-off-outline' : 'mdi-eye-outline'"
          :aria-label="showConfirm ? '隐藏确认密码' : '显示确认密码'"
          autocomplete="new-password"
          variant="outlined"
          density="comfortable"
          :error="confirmMismatch"
          :error-messages="confirmMismatch ? '两次输入的新密码不一致' : ''"
          :disabled="submitting"
          @click:append-inner="showConfirm = !showConfirm"
        />

        <v-alert
          v-if="errorMessage"
          type="error"
          variant="tonal"
          density="compact"
          class="mt-3"
          :text="errorMessage"
        />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" :disabled="submitting" @click="visible = false">取消</v-btn>
        <v-btn color="primary" variant="flat" :loading="submitting" :disabled="!canSubmit" @click="onSubmit">
          确认修改
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
