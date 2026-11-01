<script setup lang="ts">
// 全局轻提示容器（单例，挂在 App.vue 根部）。
//
// 行为规范（Docs/DevSpec/UiSpec.md）：
//   - 悬浮在视口顶部居中，**不占布局空间**，不顶动页面内容；
//   - 默认 5 秒后自动消失，可点右侧关闭按钮立即关闭；
//   - 多条消息按 FIFO 逐条展示，不叠在一起。
import { computed } from 'vue'

import { currentToast, dismissToast, type ToastType } from '../ui/toast'

const iconOf: Record<ToastType, string> = {
  success: 'mdi-check-circle-outline',
  error: 'mdi-alert-circle-outline',
  info: 'mdi-information-outline',
  warning: 'mdi-alert-outline',
}

const current = computed(() => currentToast.value)
const visible = computed({
  get: () => Boolean(current.value),
  set: (value: boolean) => {
    if (!value) dismissToast()
  },
})

function onTimeout(): void {
  dismissToast()
}
</script>

<template>
  <v-snackbar
    v-model="visible"
    :timeout="current?.timeout ?? 5000"
    :color="current?.type ?? 'success'"
    location="top"
    rounded="lg"
    elevation="8"
    class="finv-toast"
    @timeout="onTimeout"
  >
    <div class="d-flex align-center">
      <v-icon :icon="iconOf[current?.type ?? 'success']" class="mr-2" />
      <span class="finv-toast-text">{{ current?.text }}</span>
    </div>
    <template #actions>
      <v-btn variant="text" icon="mdi-close" size="small" aria-label="关闭提示" @click="dismissToast()" />
    </template>
  </v-snackbar>
</template>

<style scoped>
/* 长文案折行，不撑破屏幕；短文案保持一行 */
.finv-toast-text {
  white-space: pre-wrap;
  word-break: break-word;
  max-width: min(70vw, 720px);
}
</style>
