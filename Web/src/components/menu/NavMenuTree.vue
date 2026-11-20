<script setup lang="ts">
// 左侧菜单树容器：维护展开集合（按系统分桶持久化），并随路由自动展开祖先链。
import { ref, watch } from 'vue'

import NavMenuTreeNode from './NavMenuTreeNode.vue'
import { findMenuTrail, trailToOpenKeys } from '../../config/menuConfig'
import type { MenuSystem } from '../../config/menuConfig'
import { NAV_OPEN_KEYS_STORAGE_KEY, readStorage, writeStorage } from '../../config/uiConfig'

const props = defineProps<{
  /** 当前系统（一级菜单），其 children 为二~四级菜单 */
  system: MenuSystem
  /** 当前路由路径 */
  activePath: string
}>()

const emit = defineEmits<{
  navigate: []
}>()

/** 当前系统已展开的分组键集合 */
const openKeys = ref<Set<string>>(new Set())

/** 当前路由的祖先链展开键（不含叶子） */
function ancestorKeys(path: string): string[] {
  return trailToOpenKeys(findMenuTrail(path))
}

/** 读取指定系统在 localStorage 中的展开集合 */
function loadStoredKeys(systemKey: string): Set<string> {
  const raw = readStorage(NAV_OPEN_KEYS_STORAGE_KEY)
  if (!raw) return new Set()
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const list = parsed?.[systemKey]
    if (!Array.isArray(list)) return new Set()
    return new Set(list.filter((item): item is string => typeof item === 'string'))
  } catch {
    // 脏数据直接忽略，回退到激活链
    return new Set()
  }
}

/** 写入指定系统的展开集合（保留其它系统的桶） */
function persistKeys(systemKey: string, keys: Set<string>): void {
  const raw = readStorage(NAV_OPEN_KEYS_STORAGE_KEY)
  let all: Record<string, string[]> = {}
  if (raw) {
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>
      for (const [key, value] of Object.entries(parsed ?? {})) {
        if (Array.isArray(value)) all[key] = value.filter((item): item is string => typeof item === 'string')
      }
    } catch {
      all = {}
    }
  }
  all[systemKey] = [...keys]
  writeStorage(NAV_OPEN_KEYS_STORAGE_KEY, JSON.stringify(all))
}

/** 合并展开键（不收起用户手动展开的其它分支） */
function mergeOpenKeys(keys: string[]): void {
  const next = new Set(openKeys.value)
  let changed = false
  for (const key of keys) {
    if (!next.has(key)) {
      next.add(key)
      changed = true
    }
  }
  if (changed) openKeys.value = next
}

/** 切换系统时重置为该系统的持久化状态 + 当前激活链 */
function resetForSystem(): void {
  openKeys.value = new Set([...loadStoredKeys(props.system.key), ...ancestorKeys(props.activePath)])
}

// 系统切换：立即初始化；切换后按新系统重新计算
watch(() => props.system.key, resetForSystem, { immediate: true })

// 路由变化：仅追加激活链展开，不收起其它分支
watch(() => props.activePath, (path) => mergeOpenKeys(ancestorKeys(path)))

/** 点击分组：切换展开并持久化 */
function onToggle(key: string): void {
  const next = new Set(openKeys.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  openKeys.value = next
  persistKeys(props.system.key, next)
}
</script>

<template>
  <v-list nav density="comfortable" class="finv-nav-tree">
    <NavMenuTreeNode
      v-for="node in system.children"
      :key="node.key"
      :node="node"
      :depth="0"
      :parent-key="system.key"
      :open-keys="openKeys"
      :active-path="activePath"
      @toggle="onToggle"
      @navigate="emit('navigate')"
    />
  </v-list>
</template>

<style>
.finv-nav-tree {
  background: transparent !important;
  padding-top: 4px;
  padding-bottom: 8px;
}
</style>
