<script setup lang="ts">
// 顶栏一级系统切换器：下拉选择系统（金融投资 / 内容管理），选中后由上层负责路由跳转与左侧联动。
import { computed } from 'vue'

import type { MenuSystem } from '../../config/menuConfig'

const props = defineProps<{
  systems: MenuSystem[]
  currentKey: string
}>()

const emit = defineEmits<{
  select: [key: string]
}>()

const current = computed<MenuSystem>(
  () => props.systems.find((system) => system.key === props.currentKey) ?? props.systems[0],
)
</script>

<template>
  <v-menu location="bottom start">
    <template #activator="{ props: activatorProps }">
      <v-btn
        v-bind="activatorProps"
        class="finv-system-switcher"
        variant="text"
        :prepend-icon="current.icon"
        append-icon="mdi-menu-down"
        aria-haspopup="menu"
      >
        <!-- 小屏只保留系统图标，避免与平台标题、用户菜单挤压 -->
        <span class="d-none d-sm-inline">{{ current.title }}</span>
      </v-btn>
    </template>

    <v-list density="compact" min-width="220">
      <v-list-item
        v-for="system in systems"
        :key="system.key"
        :prepend-icon="system.icon"
        :title="system.title"
        :subtitle="system.subtitle"
        :active="system.key === currentKey"
        :disabled="system.key === currentKey"
        @click="emit('select', system.key)"
      />
    </v-list>
  </v-menu>
</template>

<style scoped>
/* 与平台标题拉开间距：避免标题与一级系统切换器贴在一起 */
.finv-system-switcher {
  margin-left: 16px;
}

@media (max-width: 900px) {
  .finv-system-switcher {
    margin-left: 8px;
  }
}
</style>
