<script setup lang="ts">
// 面包屑：由菜单节点链生成（系统 / 二级 / 三级 / 四级叶子）。
// 仅 webcms 视觉下由 App.vue 渲染；分组项点击跳到该分组下第一个叶子。
import { computed } from 'vue'

import { findFirstLeaf, isMenuLeaf } from '../../config/menuConfig'
import type { MenuNode } from '../../config/menuConfig'

const props = defineProps<{
  /** 从系统根到当前叶子的节点链；为空时不渲染 */
  trail: MenuNode[]
}>()

interface CrumbItem {
  title: string
  to?: string
  disabled?: boolean
}

const items = computed<CrumbItem[]>(() => {
  if (props.trail.length === 0) return []
  return props.trail.map((node, index) => {
    const isLast = index === props.trail.length - 1
    const target = isMenuLeaf(node) ? node.path : findFirstLeaf(node)
    return {
      title: node.title,
      to: isLast ? undefined : target,
      disabled: isLast || !target,
    }
  })
})
</script>

<template>
  <v-breadcrumbs
    v-if="items.length > 0"
    class="finv-breadcrumb px-4 pt-2 pb-0"
    density="compact"
    divider="mdi-chevron-right"
    :items="items"
  />
</template>

<style>
.finv-breadcrumb .v-breadcrumbs-item--disabled {
  opacity: 0.85;
}
</style>
