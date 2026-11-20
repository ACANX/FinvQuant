<script setup lang="ts">
// 可选中复制的文本浮层（需求 11207）。
//
// 为什么不直接用 `v-tooltip`：hover 触发、鼠标一移开就消失，里面的长文本根本没法选中复制——
// 而表格里这些 ⓘ 恰恰是给"要看全文 / 要粘走"用的。
// 这里的做法：**点击图标**打开浮层，浮层里文本可选中、带「复制」按钮，点外部或再点一次关闭。
import { ref } from 'vue'

import { showToast } from '../ui/toast'

const props = defineProps<{
  /** 浮层标题（如「公开标识（完整值）」） */
  title?: string
  /** 纯文本内容；用 lines 时可为空 */
  text?: string
  /** 多行内容（每行一条），与 text 二选一 */
  lines?: string[]
}>()

const open = ref(false)
const copied = ref(false)

async function copyAll() {
  const value = props.lines?.length ? props.lines.join('\n') : (props.text ?? '')
  try {
    await navigator.clipboard.writeText(value)
    copied.value = true
    showToast('已复制到剪贴板', 'success')
    setTimeout(() => { copied.value = false }, 1500)
  } catch {
    showToast('复制失败：浏览器不允许，请手动选中复制', 'warning')
  }
}
</script>

<template>
  <v-menu v-model="open" :close-on-content-click="false" location="bottom start" offset="4">
    <template #activator="{ props: activator }">
      <v-icon
        v-bind="activator"
        icon="mdi-information-outline"
        size="small"
        class="ml-1 text-medium-emphasis text-popover-trigger"
        title="点击查看全文（可选中复制）"
      />
    </template>
    <v-card max-width="560" class="text-popover-card">
      <v-card-title v-if="title" class="text-subtitle-2 py-2 d-flex align-center">
        <span>{{ title }}</span>
        <v-spacer />
        <v-btn
          size="x-small"
          variant="tonal"
          :prepend-icon="copied ? 'mdi-check' : 'mdi-content-copy'"
          @click="copyAll"
        >
          {{ copied ? '已复制' : '复制' }}
        </v-btn>
      </v-card-title>
      <v-divider v-if="title" />
      <v-card-text class="py-2">
        <pre v-if="lines && lines.length" class="text-popover-text"><code v-for="line in lines" :key="line">{{ line }}
</code></pre>
        <pre v-else class="text-popover-text">{{ text }}</pre>
      </v-card-text>
    </v-card>
  </v-menu>
</template>

<style scoped>
.text-popover-trigger {
  cursor: pointer;
}

.text-popover-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  line-height: 1.6;
  /* 关键：允许鼠标选中 + 复制，不能被浮层关闭逻辑吞掉 */
  user-select: text;
  cursor: text;
}
</style>
