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

// 关闭由 showToast 的独立计时器或关闭按钮触发；这里只把 v-snackbar 的关闭动作转成出队。
function onVisibleChange(value: boolean): void {
  if (!value) dismissToast()
}
</script>

<template>
  <v-snackbar
    :key="current?.id ?? 0"
    :model-value="Boolean(current)"
    :timeout="-1"
    :color="current?.type ?? 'success'"
    location="top"
    rounded="lg"
    elevation="8"
    class="finv-toast"
    @update:model-value="onVisibleChange"
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
